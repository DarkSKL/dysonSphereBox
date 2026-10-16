package multiplayer

// 文件职责：应用服务端管理策略与玩家通知；使用：控制器接收动态事件；边界：不提供管理接口，不读取客户端配置。

import (
	mpAPI "DysonSphereBox/apiDefinition/multiplayer"
	mpConstant "DysonSphereBox/constant/multiplayer"
	"encoding/json"
	"slices"
)

// NoticeView 职责：保存离房和管理原因；使用：Wails 投影和缓存页面恢复；边界：纯文本显示，不触发自动加入。
type NoticeView struct {
	// Code 职责：管理原因码；使用：中文码表；边界：0 时只展示 Message。
	Code int `json:"code"`
	// Message 职责：安全原因说明；使用：补充默认文案；边界：不渲染 HTML。
	Message string `json:"message"`
}

// applyPolicy 职责：串行替换已提交策略和原生预约；使用：socket 事件循环；边界：不持控制器锁等待网络，过期或已结束会话不应用。
func (c *controller) applyPolicy(sess *session, frame *mpAPI.ActiveFrame) (int, string) {
	var data mpAPI.SessionPolicyEvent
	if json.Unmarshal(frame.Data, &data) != nil || data.Policy == nil {
		return mpConstant.CodeApplyFailed, "bad policy"
	}
	if data.Result != 0 {
		return 0, ""
	}
	c.mu.Lock()
	if c.sess != sess || sess.terminate != 0 || (sess.info.Policy != nil && (data.Policy.Revision < sess.info.Policy.Revision ||
		(*data.Policy == *sess.info.Policy && data.Capabilities == sess.info.Capabilities && slices.Equal(data.Relays, sess.info.Relays)))) {
		c.mu.Unlock()
		return 0, ""
	}
	relayChanged := data.Capabilities != sess.info.Capabilities || !slices.Equal(data.Relays, sess.info.Relays)
	relayGame := false
	if c.room != nil {
		if c.room.link != nil {
			relayGame = c.room.link.current().Path == mpConstant.PathRelay
		}
		for _, member := range c.room.view.Snapshot.Members {
			if member.Link != nil && member.Link.Path == mpConstant.PathRelay {
				relayGame = true
			}
		}
	}
	c.mu.Unlock()
	// 事件队列串行执行，ConfigureRelay 等待旧 worker 退出，旧预约不能污染新策略。
	var err error
	if relayChanged {
		err = sess.host.ConfigureRelay(data.Capabilities, data.Relays, nil)
	}
	c.mu.Lock()
	if c.sess != sess || sess.terminate != 0 {
		c.mu.Unlock()
		return 0, ""
	}
	sess.info.Policy, sess.info.Capabilities, sess.info.Relays = data.Policy, data.Capabilities, data.Relays
	if relayChanged && relayGame {
		c.notice = &NoticeView{Message: "服务器已更新中继策略，原中继游戏连接已结束，正在尝试重新连接。"}
	}
	if data.Policy.MaxMembers > 0 {
		sess.info.Limits.MaxMembers = data.Policy.MaxMembers
	}
	sess.report.Addrs = sess.host.FilterRelayAddrs(sess.report.Addrs, sess.host.ID())
	if c.room != nil && c.room.endpoint != nil && c.room.link != nil {
		endpoint := *c.room.endpoint
		endpoint.Addrs = sess.host.FilterRelayAddrs(endpoint.Addrs, c.room.link.hostID)
		c.room.endpoint = &endpoint
		c.room.link.updateEndpoint(endpoint)
	}
	if c.room != nil && c.room.link != nil {
		c.room.link.wakeUp()
	}
	c.mu.Unlock()
	sess.host.RunReservations()
	c.emit()
	if err != nil {
		return mpConstant.CodeApplyFailed, err.Error()
	}
	return 0, ""
}
