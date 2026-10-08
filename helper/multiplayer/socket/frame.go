package socket

// 文件职责：联机控制链路帧的编码、解码与服务器主动帧 seqId 连续性检测。
// 使用：Conn 读写循环调用；测试替身服务器（sockettest）复用同一套编解码。
// 边界：只处理帧外层结构，不解释 data 业务含义；按字段是否存在区分主动帧与回复帧，不按取值判断。

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	mpAPI "DysonSphereBox/apiDefinition/multiplayer"
)

// emptyObject 职责：data/exData 缺省值 {}；使用：编码时 data 为空；边界：协议要求 data 永远是对象。
var emptyObject = json.RawMessage("{}")

// ErrBadFrame 表示帧无法解析或缺少必需字段。
// 职责：区分可回复 20001 的帧错误；使用：读循环收到非法帧时回复 20001 且不关闭连接；边界：不代表连接错误。
var ErrBadFrame = errors.New("multiplayer: bad frame")

// Frame 是解码后的帧，Active 与 Reply 恰好一个非空。
// 职责：承载按字段存在判定后的帧；使用：读循环分流主动帧与回复帧；边界：不含原始字节。
type Frame struct {
	// Active 职责：主动帧（服务器事件或客户端请求）；使用：事件队列；边界：与 Reply 互斥。
	Active *mpAPI.ActiveFrame
	// Reply 职责：回复帧；使用：直接交付在途请求；边界：与 Active 互斥。
	Reply *mpAPI.ReplyFrame
}

// EncodeActive 编码一个主动帧。
// 职责：生成 {opcode,data,exData} 文本；使用：客户端发请求、测试替身发事件；边界：data 为 nil 时写 {}。
func EncodeActive(opcode int, data any, exData mpAPI.ActiveExData) ([]byte, error) {
	raw, err := marshalObject(data)
	if err != nil {
		return nil, err
	}
	return json.Marshal(mpAPI.ActiveFrame{Opcode: opcode, Data: raw, ExData: exData})
}

// EncodeReply 编码一个回复帧。
// 职责：生成 {code,msg,data,exData} 文本；使用：客户端回复事件、测试替身回复请求；边界：data 为 nil 时写 {}。
func EncodeReply(code int, msg string, data any, exData mpAPI.ReplyExData) ([]byte, error) {
	raw, err := marshalObject(data)
	if err != nil {
		return nil, err
	}
	return json.Marshal(mpAPI.ReplyFrame{Code: code, Msg: msg, Data: raw, ExData: exData})
}

// DecodeFrame 解码一帧文本。
// 职责：按 opcode/code 字段是否存在判定帧类型并校验必需字段；使用：读循环；边界：两个字段同时出现、缺字段或 data/exData 不是对象时返回 ErrBadFrame。
func DecodeFrame(raw []byte) (Frame, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Frame{}, fmt.Errorf("%w: %v", ErrBadFrame, err)
	}
	_, hasOpcode := fields["opcode"]
	_, hasCode := fields["code"]
	switch {
	case hasOpcode && hasCode:
		return Frame{}, fmt.Errorf("%w: opcode and code both present", ErrBadFrame)
	case hasOpcode:
		if !isObject(fields["data"]) || !isObject(fields["exData"]) {
			return Frame{}, fmt.Errorf("%w: data/exData must be objects", ErrBadFrame)
		}
		var frame mpAPI.ActiveFrame
		if err := json.Unmarshal(raw, &frame); err != nil {
			return Frame{}, fmt.Errorf("%w: %v", ErrBadFrame, err)
		}
		return Frame{Active: &frame}, nil
	case hasCode:
		if !isObject(fields["exData"]) {
			return Frame{}, fmt.Errorf("%w: exData must be object", ErrBadFrame)
		}
		var frame mpAPI.ReplyFrame
		if err := json.Unmarshal(raw, &frame); err != nil {
			return Frame{}, fmt.Errorf("%w: %v", ErrBadFrame, err)
		}
		if !isObject(frame.Data) {
			frame.Data = emptyObject
		}
		return Frame{Reply: &frame}, nil
	default:
		return Frame{}, fmt.Errorf("%w: neither opcode nor code", ErrBadFrame)
	}
}

// ErrSeqGap 表示服务器主动帧 seqId 跳号。
// 职责：标识协议错误；使用：Conn 收到后关闭 socket，控制器按断线退避重连；边界：不可恢复。
var ErrSeqGap = errors.New("multiplayer: server seqId gap")

// SeqTracker 检测对方主动帧 seqId 是否从 1 起严格 +1。
// 职责：保存上一个序号并校验下一个；使用：Conn 读循环收到服务器事件时调用；边界：非并发安全，只在读循环中使用。
type SeqTracker struct {
	// last 职责：上一个已接受的序号；使用：Next 校验；边界：初始为 0。
	last int64
}

// Next 校验并接受下一个序号。
// 职责：判断是否等于 last+1；使用：每个服务器主动帧；边界：不连续时返回 ErrSeqGap 且不更新 last。
func (tracker *SeqTracker) Next(seq int64) error {
	if seq != tracker.last+1 {
		return fmt.Errorf("%w: want %d, got %d", ErrSeqGap, tracker.last+1, seq)
	}
	tracker.last = seq
	return nil
}

// marshalObject 把任意 data 编码为 JSON 对象原文。
// 职责：统一 nil、RawMessage 与结构体的编码；使用：EncodeActive/EncodeReply；边界：编码结果不是对象时返回错误。
func marshalObject(data any) (json.RawMessage, error) {
	switch value := data.(type) {
	case nil:
		return emptyObject, nil
	case json.RawMessage:
		if len(value) == 0 {
			return emptyObject, nil
		}
		if !isObject(value) {
			return nil, errors.New("multiplayer: data must be object")
		}
		return value, nil
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	if !isObject(raw) {
		return nil, errors.New("multiplayer: data must be object")
	}
	return raw, nil
}

// isObject 判断 JSON 原文是否是对象。
// 职责：校验 data/exData 不为 null 且为对象；使用：编解码校验；边界：只看首个非空白字符。
func isObject(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && trimmed[0] == '{'
}
