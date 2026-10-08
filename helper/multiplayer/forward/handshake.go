package forward

// 文件职责：数据链路 stream 握手头与 UDP 报文帧的编解码（方案第 7.6/7.7 节）。
// 使用：房主 handler 读握手、成员开 stream 写握手；UDP 数据阶段读写 u16 长度 + 报文。
// 边界：只处理字节格式，不做授权与端口校验；握手不携带任何凭证。

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"

	mpConstant "DysonSphereBox/constant/multiplayer"
)

// Header 是 stream 握手头。
// 职责：描述端口下标、段内偏移与 UDP 会话 sid；使用：成员写、房主读；边界：TCP 不带 sid。
type Header struct {
	// V 职责：握手版本；使用：房主只接受 1；边界：固定 1。
	V int `json:"v"`
	// Port 职责：端口声明下标；使用：房主查端口表；边界：越界回 BAD_PORT。
	Port int `json:"port"`
	// Offset 职责：段内偏移；使用：目标端口 = 起点 + 偏移；边界：≥ count 回 BAD_PORT。
	Offset int `json:"offset"`
	// SID 职责：UDP 会话标识（16 位十六进制）；使用：房主 (PeerID,sid) 映射与接回；边界：只用于 UDP。
	SID string `json:"sid,omitempty"`
}

// maxHeaderBytes 职责：握手头长度上限；使用：读握手；边界：防止异常长度占用内存。
const maxHeaderBytes = 1024

// WriteHeader 写 u16 长度 + JSON 握手头。
func WriteHeader(writer io.Writer, header Header) error {
	raw, err := json.Marshal(header)
	if err != nil {
		return err
	}
	return writeFrame(writer, raw)
}

// ReadHeader 读握手头。
// 职责：解析并校验长度与版本；使用：房主 handler；边界：版本不为 1 返回错误（房主回 BAD_PORT）。
func ReadHeader(reader io.Reader) (Header, error) {
	raw, err := readFrame(reader, maxHeaderBytes)
	if err != nil {
		return Header{}, err
	}
	var header Header
	if err := json.Unmarshal(raw, &header); err != nil {
		return Header{}, err
	}
	if header.V != mpConstant.HandshakeVersion {
		return header, errors.New("unsupported handshake version")
	}
	return header, nil
}

// WritePacket 写一个 UDP 报文帧。
func WritePacket(writer io.Writer, payload []byte) error { return writeFrame(writer, payload) }

// ReadPacket 读一个 UDP 报文帧。
func ReadPacket(reader io.Reader) ([]byte, error) { return readFrame(reader, mpConstant.MaxUDPPayload) }

// writeFrame 写 u16 大端长度 + 内容（一次写出，避免与其他写交错）。
func writeFrame(writer io.Writer, payload []byte) error {
	if len(payload) > 0xFFFF {
		return errors.New("frame too large")
	}
	buffer := make([]byte, 2+len(payload))
	binary.BigEndian.PutUint16(buffer, uint16(len(payload)))
	copy(buffer[2:], payload)
	_, err := writer.Write(buffer)
	return err
}

// readFrame 读 u16 大端长度 + 内容。
func readFrame(reader io.Reader, limit int) ([]byte, error) {
	var size [2]byte
	if _, err := io.ReadFull(reader, size[:]); err != nil {
		return nil, err
	}
	length := int(binary.BigEndian.Uint16(size[:]))
	if length > limit {
		return nil, errors.New("frame exceeds limit")
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, err
	}
	return payload, nil
}
