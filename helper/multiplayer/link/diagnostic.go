package link

// 文件职责：复制并裁剪异步诊断附件；使用：Tracer 入队前；边界：不修改调用者对象，裁剪必须标记且 JSON 不超过协议预算。

import (
	"encoding/json"

	mpAPI "DysonSphereBox/apiDefinition/multiplayer"
)

// boundDiagnostic 职责：复制并裁剪异步附件；使用：Trace 入队前；边界：总 JSON 最多8192字节，裁剪显式标记，调用者对象不变。
func boundDiagnostic(raw *mpAPI.TraceDiagnostic) *mpAPI.TraceDiagnostic {
	if raw == nil {
		return nil
	}
	d := *raw
	scalar := func(value string, max int) string {
		if len(value) > max {
			d.Truncated = true
		}
		return truncateUTF8(value, max)
	}
	d.Event = scalar(d.Event, 64)
	d.Transport = scalar(d.Transport, 16)
	d.LocalPeerID = scalar(d.LocalPeerID, 128)
	d.RemotePeerID = scalar(d.RemotePeerID, 128)
	d.ObservedAddr = scalar(d.ObservedAddr, 256)
	d.ConnectedAddr = scalar(d.ConnectedAddr, 256)
	groups := []*[]string{&d.RemoteAddrs, &d.Targets, &d.ListenAddrs, &d.HostAddrs}
	for _, group := range groups {
		values := *group
		*group = append([]string{}, values...)
		if len(*group) > 32 {
			*group = (*group)[:32]
			d.Truncated = true
		}
		for i, value := range *group {
			if len(value) > 256 {
				d.Truncated = true
			}
			(*group)[i] = truncateUTF8(value, 256)
		}
	}
	d.Errors = append([]mpAPI.TraceAddrError{}, raw.Errors...)
	if len(d.Errors) > 32 {
		d.Errors = d.Errors[:32]
		d.Truncated = true
	}
	for i := range d.Errors {
		if len(d.Errors[i].Error) > 512 || len(d.Errors[i].Addr) > 256 {
			d.Truncated = true
		}
		d.Errors[i].Addr = truncateUTF8(d.Errors[i].Addr, 256)
		d.Errors[i].Error = truncateUTF8(d.Errors[i].Error, 512)
	}
	for {
		encoded, err := json.Marshal(d)
		if err != nil {
			return &mpAPI.TraceDiagnostic{Event: d.Event, Truncated: true}
		}
		if len(encoded) <= 8192 {
			return &d
		}
		d.Truncated = true
		if len(d.Errors) > 0 {
			d.Errors = d.Errors[:len(d.Errors)-1]
			continue
		}
		removed := false
		for _, group := range []*[]string{&d.HostAddrs, &d.ListenAddrs, &d.Targets, &d.RemoteAddrs} {
			if len(*group) > 0 {
				*group = (*group)[:len(*group)-1]
				removed = true
				break
			}
		}
		if !removed {
			return &mpAPI.TraceDiagnostic{Event: d.Event, Truncated: true}
		}
	}
}
