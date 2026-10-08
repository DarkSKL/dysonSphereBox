package multiplayer

// 文件职责：定义联机控制链路 WebSocket 的两种帧外层结构与 socket 路径。
// 使用：helper/multiplayer/socket 编解码帧；service/multiplayer 通过 SocketURL 构造连接地址。
// 边界：只描述协议契约，不读写连接、不判定业务结果；字段名严格按协议设计 README 第 1 节。

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// APIURLSocket 是联机控制链路的 WebSocket 路径。
// 职责：描述服务器控制面端点相对路径；使用：SocketURL 拼接服务器地址；边界：router-prefix 为空时即该路径，x-token 走请求头不放 URL。
const APIURLSocket = "/client/multiplayer/v3/socket"

// SocketURL 把应用服务器 HTTP 地址转换为控制链路 WebSocket 地址。
// 职责：http→ws、https→wss 并拼接 APIURLSocket；使用：控制器每次拨号前调用；边界：只接受 http/https/ws/wss，其他协议返回错误。
func SocketURL(baseURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", err
	}
	switch parsed.Scheme {
	case "http", "ws":
		parsed.Scheme = "ws"
	case "https", "wss":
		parsed.Scheme = "wss"
	default:
		return "", fmt.Errorf("不支持的服务器地址协议：%q", parsed.Scheme)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + APIURLSocket
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

// ActiveFrame 是任意一方主动发送的帧（客户端请求、服务器事件）。
// 职责：承载 opcode、data 与 exData；使用：socket 编码请求、解码事件；边界：data 与 exData 永远是对象，不能为 null。
type ActiveFrame struct {
	// Opcode 职责：协议码；使用：分发请求或事件；边界：1xxx 请求、2xxx/3xxx 事件。
	Opcode int `json:"opcode"`
	// Data 职责：请求或事件数据原文；使用：按 opcode 解码为具体结构；边界：为空时编码为 {}。
	Data json.RawMessage `json:"data"`
	// ExData 职责：序号、时间戳与事件关联信息；使用：seqId 检测与回复匹配；边界：事件才有 eventId/causeSeqId。
	ExData ActiveExData `json:"exData"`
}

// ActiveExData 是主动帧的附加数据。
// 职责：携带发送方序号与时间；使用：seqId 跳号检测、时钟偏移估算；边界：客户端请求不发送 eventId/causeSeqId。
type ActiveExData struct {
	// SeqID 职责：发送方主动帧序号，单 socket 内从 1 严格 +1；使用：回复匹配与跳号检测；边界：双方各自计数。
	SeqID int64 `json:"seqId"`
	// Timestamp 职责：发送时 Unix 毫秒；使用：时钟偏移估算；边界：服务器时钟。
	Timestamp int64 `json:"timestamp"`
	// EventID 职责：事件对 ID；使用：日志关联 before/after；边界：只在服务器事件中出现。
	EventID string `json:"eventId,omitempty"`
	// CauseSeqID 职责：触发请求的 seqId；使用：识别由自己请求触发的事件；边界：非触发者为 0。
	CauseSeqID int64 `json:"causeSeqId,omitempty"`
}

// ReplyFrame 是对主动帧的回复帧。
// 职责：承载结果码、提示、回复数据；使用：服务器回复请求、客户端回复事件；边界：回复帧本身不再被回复。
type ReplyFrame struct {
	// Code 职责：结果码；使用：0 成功，非 0 失败；边界：按字段存在而非取值区分帧类型。
	Code int `json:"code"`
	// Msg 职责：提示文案；使用：服务器回复可直接展示；边界：客户端回复只用于日志。
	Msg string `json:"msg"`
	// Data 职责：回复数据原文；使用：只读请求的结果；边界：失败时为 {} 或文档注明的错误详情。
	Data json.RawMessage `json:"data"`
	// ExData 职责：回传所回复帧的 seqId/opcode 与回复时间；使用：匹配在途请求；边界：seqId 指对方序号。
	ExData ReplyExData `json:"exData"`
}

// ReplyExData 是回复帧的附加数据。
// 职责：回传被回复帧的标识；使用：在途请求匹配与时钟偏移估算；边界：无法解析的帧回复时 seqId/opcode 为 0。
type ReplyExData struct {
	// SeqID 职责：被回复主动帧的 seqId；使用：匹配请求；边界：指对方的序号。
	SeqID int64 `json:"seqId"`
	// Opcode 职责：被回复主动帧的 opcode；使用：日志与校验；边界：无法解析时为 0。
	Opcode int `json:"opcode"`
	// Timestamp 职责：回复发送时 Unix 毫秒；使用：时钟偏移估算；边界：发送方时钟。
	Timestamp int64 `json:"timestamp"`
}
