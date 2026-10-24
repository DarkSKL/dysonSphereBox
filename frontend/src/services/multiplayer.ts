// =============== 联机绑定封装 ===============
// 说明：包装 Wails MultiplayerService 意图方法与 multiplayer:state 事件订阅。
// 使用：useMultiplayerState 调用；页面不直接调用生成绑定。
// 边界：前端不连接服务器、不持有 token；未运行在 Wails 桌面环境时返回 20903 而不是抛出 TypeError。

import {
  CloseRoom,
  Connect,
  CreateRoom,
  GamePortPresets,
  GetState,
  InviteRoom,
  JoinRoom,
  KickMember,
  LeaveRoom,
  LobbyList,
  LobbyWatch,
  ResolveRoom,
  RetryLink,
} from '../../wailsjs/go/multiplayer/MultiplayerService'
import {multiplayer} from '../../wailsjs/go/models'
import {EventsOn} from '../../wailsjs/runtime/runtime'
import type {GamePortPreset, LobbyQuery, MultiplayerActionResult, MultiplayerState, RoomCreateForm} from '../types/multiplayer'

export const MULTIPLAYER_STATE_EVENT = 'multiplayer:state'

interface MultiplayerBridgeWindow extends Window {
  go?: {multiplayer?: {MultiplayerService?: Record<string, unknown>}}
}

function bridgeReady(): boolean {
  return typeof (window as MultiplayerBridgeWindow).go?.multiplayer?.MultiplayerService?.Connect === 'function'
}

const notInDesktop: MultiplayerActionResult = {code: 20903, msg: '联机需要在戴森球盒子桌面应用中运行', data: null}

async function call(action: () => Promise<MultiplayerActionResult>): Promise<MultiplayerActionResult> {
  if (!bridgeReady()) return notInDesktop
  try {
    return await action()
  } catch (error) {
    return {code: 20902, msg: error instanceof Error ? error.message : String(error), data: null}
  }
}

export const connectMultiplayer = () => call(Connect)
export const refreshLobby = (query: LobbyQuery) =>
  call(() => LobbyList(multiplayer.LobbyListRequest.createFrom({...query, order: 'desc'})))
export const watchLobby = (on: boolean) => call(() => LobbyWatch(on))
export const createRoom = (request: RoomCreateForm) =>
  call(() => CreateRoom(multiplayer.RoomCreateRequest.createFrom(request)))
export const resolveRoom = (roomId: string, inviteCode: string) =>
  call(() => ResolveRoom(multiplayer.RoomResolveRequest.createFrom({roomId, inviteCode})))
export const joinRoom = (roomId: string, inviteCode: string, password: string) =>
  call(() => JoinRoom(multiplayer.RoomJoinRequest.createFrom({roomId, inviteCode, password})))
export const inviteRoom = () => call(InviteRoom)
export const leaveRoom = () => call(LeaveRoom)
export const closeRoom = () => call(CloseRoom)
export const kickMember = (userId: number) => call(() => KickMember(userId))
export const retryLink = () => call(RetryLink)

// 说明：游戏端口自动识别是本地只读探测，不依赖控制链路；非桌面环境或调用失败时返回空列表。
// 边界：空列表只表示“没有可用游戏项”，手动 TCP/UDP 输入仍然可用。
export async function detectGamePortPresets(): Promise<GamePortPreset[]> {
  if (!bridgeReady()) return []
  try {
    return await GamePortPresets()
  } catch {
    return []
  }
}

export async function getMultiplayerState(): Promise<MultiplayerState | null> {
  if (!bridgeReady()) return null
  try {
    return await GetState()
  } catch {
    return null
  }
}

// 说明：返回取消订阅函数；非桌面环境返回空函数。
export function onMultiplayerState(listener: (state: MultiplayerState) => void): () => void {
  if (!bridgeReady()) return () => {}
  return EventsOn(MULTIPLAYER_STATE_EVENT, listener)
}
