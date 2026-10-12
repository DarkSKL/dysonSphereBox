package link

// 文件职责：把数据链路的每次直连 / 打洞结果（含目标地址列表、实际连上的地址与逐地址错误）写入盒子系统日志。
// 使用：service/multiplayer 成员链路每次直连或指定中继拨号结束时构造 DialRecord 并调用 RecordDial。
// 边界：只写本地日志，不上报服务器；日志写入失败静默忽略，不影响链路状态机。

import (
	"fmt"
	"strings"

	systemModel "DysonSphereBox/constant/logging"
	"DysonSphereBox/helper/multiplayer/p2p"
	systemHelper "DysonSphereBox/helper/system"
)

// 拨号记录的角色取值。
const (
	// DialRoleMember 职责：成员拨房主；使用：DialRecord.Role；边界：默认值。
	DialRoleMember = "member"
	// DialRoleHost 职责：房主侧诊断角色；使用：DialRecord.Role；边界：v2 不再 WS 回拨。
	DialRoleHost = "host"
)

// DialRecord 是一次拨号的结果。
// 职责：携带角色、路径、轮次、对端、目标地址、实际地址与错误；使用：盒子日志与成员 links[].detail；边界：Round 直连为 0；协调失败等未真正拨号的记录 Targets 为空。
type DialRecord struct {
	// Role 职责：member / host；使用：区分谁在拨号；边界：空视为 member。
	Role string
	// Path 职责：direct / punch；使用：区分直连与打洞；边界：小写。
	Path string
	// Round 职责：打洞轮次；使用：定位第几轮；边界：直连为 0。
	Round int
	// PeerID 职责：对端临时 PeerID（成员侧为房主，房主侧为成员）；使用：排障；边界：字符串形式。
	PeerID string
	// PeerUserID 职责：对端 userId；使用：房主侧区分成员；边界：未知为 0。
	PeerUserID int64
	// RoomID 职责：房间；使用：排障；边界：可为空。
	RoomID string
	// OK 职责：是否连通；使用：日志等级与文本；边界：无。
	OK bool
	// Err 职责：顶层错误或失败说明；使用：日志与 detail；边界：成功为空。
	Err string
	// Targets 职责：目标地址（带私网/公网归类）；使用：判断地址是否可达；边界：可为空。
	Targets []string
	// Connected 职责：实际连上的地址（带归类）；使用：成功时展示；边界：失败为空。
	Connected string
	// AddrErrors 职责：逐地址结果行；使用：定位哪个地址为什么失败；边界：成功为空。
	AddrErrors []string
}

// ApplyReport 用拨号诊断结果填写 OK、错误、目标地址、实际地址与逐地址错误。
func (record *DialRecord) ApplyReport(report p2p.DialReport) {
	record.OK = report.Err == nil
	record.Err = report.TopError()
	record.Targets = report.TargetTexts()
	record.Connected = report.ConnectedText()
	record.AddrErrors = report.AddrLines()
}

// Text 返回面向用户的说明：第一行结论，其后缩进列出目标地址与逐地址错误。
func (record DialRecord) Text() string {
	action := "直连"
	if record.Path == "relay" {
		action = "指定中继连接"
	}
	if record.Path == "punch" {
		action = fmt.Sprintf("打洞第 %d 轮", record.Round)
		if record.Round == 0 {
			// 历史日志兼容：旧版本第 0 轮表示未执行拨号，v2 不生成此路径。
			action = "打洞"
		}
	}
	peer := fmt.Sprintf("（房主 %s）", record.PeerID)
	if record.Role == DialRoleHost {
		action = "回拨成员" + action
		peer = fmt.Sprintf("（成员 %d %s）", record.PeerUserID, record.PeerID)
	}
	var builder strings.Builder
	if record.OK {
		builder.WriteString(action + "成功" + peer + "：连上 " + record.Connected)
	} else {
		builder.WriteString(action + "失败" + peer + "：" + record.Err)
	}
	if len(record.Targets) > 0 {
		builder.WriteString("\n  目标地址：" + strings.Join(record.Targets, ", "))
	}
	for _, line := range record.AddrErrors {
		builder.WriteString("\n  · " + line)
	}
	return builder.String()
}

// RecordDial 记录一次拨号结果。
// 职责：失败写 warn（event dialFailure），成功写 info（event dialSuccess），namespace multiplayer、module link；content 为 Text，metadata 为结构化字段；
// 使用：成员链路与房主回拨；边界：静默写入，调用方可放到独立 goroutine。
func RecordDial(record DialRecord) {
	level, event := systemModel.Warn, "dialFailure"
	if record.OK {
		// 单次成功拨号是连接建立过程的一部分；后续断线与最终链路状态负责反馈结果。
		return
	}
	role := record.Role
	if role == "" {
		role = DialRoleMember
	}
	systemHelper.RecordLogSilently(systemHelper.LogEntry{
		Namespace: "multiplayer.link." + event,
		Source:    "backend.link",
		Reason:    "联机链路状态机执行直连或指定中继后的实际拨号结果",
		Handler:   "System",
		Level:     level,
		Action:    record.Text(),
		Error:     strings.Join(append([]string{record.Err}, record.AddrErrors...), "\n"),
		Data: map[string]interface{}{
			"role":       role,
			"path":       record.Path,
			"round":      record.Round,
			"peerId":     record.PeerID,
			"peerUserId": record.PeerUserID,
			"roomId":     record.RoomID,
			"ok":         record.OK,
			"targets":    record.Targets,
			"connected":  record.Connected,
			"addrErrors": record.AddrErrors,
		},
	})
}
