package multiplayer

// 文件职责：暴露联机意图方法给 Wails 前端，并装配生产环境的控制器依赖（服务器地址、账号 token、状态推送）。
// 使用：前端联机页面通过生成绑定调用 Connect/LobbyList/CreateRoom 等方法，并监听 multiplayer:state。
// 边界：只做参数转发与结果返回；token 只在 Go 后端使用，永远不返回给前端；构造时不访问数据库与网络。

import (
	"strings"
	"sync"

	mpAPI "DysonSphereBox/apiDefinition/multiplayer"
	mpConstant "DysonSphereBox/constant/multiplayer"
	"DysonSphereBox/core"
	accountHelper "DysonSphereBox/helper/system/account"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var (
	defaultControllerOnce sync.Once
	defaultControllerRef  *controller
)

// defaultController 返回进程内唯一的生产控制器。
// 职责：延迟创建并注册账号会话清除回调；使用：MultiplayerService 每个方法；边界：首次调用时才读取配置。
func defaultController() *controller {
	defaultControllerOnce.Do(func() {
		defaultControllerRef = newController(options{
			socketURL: func() (string, error) {
				base := ""
				if core.GLOBAL.ServerClient != nil {
					base = core.GLOBAL.ServerClient.BaseURL()
				}
				return mpAPI.SocketURL(base)
			},
			token: accountHelper.Token,
			emit: func(state State) {
				if core.GLOBAL.RuntimeContext != nil {
					runtime.EventsEmit(core.GLOBAL.RuntimeContext, mpConstant.StateEventName, state)
				}
			},
			clientVersion: strings.TrimPrefix(core.Version, "v"),
		})
		// 退出登录、401、切换服务器地址都会清除会话：控制链路必须随之主动断开
		accountHelper.OnSessionCleared(func() { go defaultControllerRef.disconnect() })
	})
	return defaultControllerRef
}

// MultiplayerService 提供联机意图方法。
// 职责：把前端意图转交控制器；使用：联机页面；边界：结果码 0 成功，变更类请求的状态变化经 multiplayer:state 推送。
type MultiplayerService struct{}

// Connect 建立并保持控制链路（幂等）。
// 职责：进入联机页面时调用；使用：前端 onMounted/onActivated；边界：未登录返回 code 7，离开页面不需要断开。
func (service *MultiplayerService) Connect() ActionResult { return defaultController().connect() }

// Disconnect 断开控制链路并停止自动重连。
// 职责：页面“断开”按钮；使用：前端；边界：在房间内时等同于离房/关房。
func (service *MultiplayerService) Disconnect() ActionResult { return defaultController().disconnect() }

// GetState 返回当前状态投影。
// 职责：页面挂载与激活时恢复状态；使用：前端；边界：只读。
func (service *MultiplayerService) GetState() State { return defaultController().getState() }

// LobbyList 查询大厅。
// 职责：lobby.list；使用：大厅刷新；边界：结果同时写入投影 lobby。
func (service *MultiplayerService) LobbyList(request mpAPI.LobbyListRequest) ActionResult {
	return defaultController().lobbyList(request)
}

// LobbyWatch 订阅或取消订阅大厅变化。
// 职责：lobby.watch；使用：大厅区块激活/停用；边界：断线重连后自动恢复订阅。
func (service *MultiplayerService) LobbyWatch(on bool) ActionResult {
	return defaultController().lobbyWatch(on)
}

// CreateRoom 创建房间。
// 职责：room.create；使用：创建表单；边界：等待首次地址上报完成后再发送，房间经 room.create after 进入投影。
func (service *MultiplayerService) CreateRoom(request mpAPI.RoomCreateRequest) ActionResult {
	return defaultController().createRoom(request)
}

// ResolveRoom 预览房间。
// 职责：room.resolve；使用：加入前展示；边界：只读，不占名额。
func (service *MultiplayerService) ResolveRoom(request mpAPI.RoomResolveRequest) ActionResult {
	return defaultController().resolveRoom(request)
}

// JoinRoom 加入房间。
// 职责：room.join；使用：加入表单；边界：成功后本地端口与链路经投影展示。
func (service *MultiplayerService) JoinRoom(request mpAPI.RoomJoinRequest) ActionResult {
	return defaultController().joinRoom(request)
}

// InviteRoom 生成邀请码。
// 职责：room.invite；使用：房主；边界：邀请码经 room.invite after 进入投影。
func (service *MultiplayerService) InviteRoom() ActionResult {
	return defaultController().roomAction("invite", mpConstant.OpRoomInvite)
}

// LeaveRoom 退出房间。
// 职责：room.leave；使用：成员；边界：房主应使用 CloseRoom。
func (service *MultiplayerService) LeaveRoom() ActionResult {
	return defaultController().roomAction("leave", mpConstant.OpRoomLeave)
}

// CloseRoom 关闭房间。
// 职责：room.close；使用：房主；边界：成员调用由服务器拒绝。
func (service *MultiplayerService) CloseRoom() ActionResult {
	return defaultController().roomAction("close", mpConstant.OpRoomClose)
}

// KickMember 踢出成员。
// 职责：room.kick；使用：房主；边界：被踢者不能再次加入本房间。
func (service *MultiplayerService) KickMember(userID int64) ActionResult {
	return defaultController().kick(userID)
}

// RetryLink 手动重试数据链路。
// 职责：唤醒 failed/reconnecting 的成员链路立即重试；使用：成员；边界：房主调用返回 20209。
func (service *MultiplayerService) RetryLink() ActionResult { return defaultController().retryLink() }
