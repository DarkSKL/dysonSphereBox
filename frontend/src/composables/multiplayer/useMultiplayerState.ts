// =============== 联机页面状态 ===============
// 说明：以模块级 ref 共享 Go 推送的 multiplayer:state 投影，并编排页面的连接、大厅订阅与房间操作。
// 使用：联机页面调用 useMultiplayerState；挂载与激活时恢复投影、建立控制链路并订阅大厅，停用时取消大厅订阅。
// 边界：状态只来自 Go 投影，前端不轮询、不自行拼装房间；离开页面不断开控制链路（避免切换页面即离房）。

import {computed, onActivated, onBeforeUnmount, onDeactivated, onMounted, ref} from 'vue'
import * as api from '../../services/multiplayer'
import {codeText, type MultiplayerActionResult, type MultiplayerState, type PortDecl, type RoomResolveResult} from '../../types/multiplayer'

const state = ref<MultiplayerState | null>(null)
const lastAction = ref<{ok: boolean; text: string} | null>(null)
let listenerCount = 0
let disposeListener: (() => void) | null = null

function applyState(next: MultiplayerState | null) {
  if (!next) return
  if (state.value && next.stateRevision <= state.value.stateRevision) return
  state.value = next
}

async function restore() {
  applyState(await api.getMultiplayerState())
}

// 说明：统一记录意图结果，失败时按 code 映射文案。
function report(result: MultiplayerActionResult, successText = ''): boolean {
  const ok = result.code === 0
  if (!ok) lastAction.value = {ok, text: codeText(result.code, result.msg)}
  else if (successText) lastAction.value = {ok, text: successText}
  return ok
}

export function useMultiplayerState() {
  const resolved = ref<RoomResolveResult | null>(null)

  const online = computed(() => state.value?.connection.state === 'online')
  const room = computed(() => state.value?.room ?? null)
  const isHost = computed(() => room.value?.role === 'host')
  const pending = computed(() => new Set(state.value?.pending ?? []))

  // 说明：silent 用于页面挂载/激活时的自动连接，未登录等结果由页面常驻提示展示，不重复弹出消息。
  async function connect(silent = false) {
    const result = await api.connectMultiplayer()
    if (!silent) report(result)
    await restore()
  }

  async function disconnect() {
    report(await api.disconnectMultiplayer())
  }

  async function activateLobby() {
    if (!online.value) return
    await api.refreshLobby()
    await api.watchLobby(true)
  }

  async function create(form: {name: string; visibility: string; password: string; maxMembers: number; ports: PortDecl[]}) {
    return report(await api.createRoom(form), '房间已创建')
  }

  async function resolve(roomId: string, inviteCode: string) {
    resolved.value = null
    const result = await api.resolveRoom(roomId, inviteCode)
    if (report(result)) resolved.value = result.data as RoomResolveResult
  }

  async function join(roomId: string, inviteCode: string, password: string) {
    return report(await api.joinRoom(roomId, inviteCode, password), '已加入房间')
  }

  const invite = async () => report(await api.inviteRoom(), '已生成邀请码')
  const leave = async () => report(await api.leaveRoom())
  const close = async () => report(await api.closeRoom())
  const kick = async (userId: number) => report(await api.kickMember(userId))
  const retry = async () => report(await api.retryLink())
  const refreshLobby = async (keyword = '') => report(await api.refreshLobby(keyword))

  // 说明：多个组件共用同一份投影，只注册一次事件监听；最后一个使用者卸载时释放。
  onMounted(async () => {
    if (listenerCount++ === 0) disposeListener = api.onMultiplayerState(applyState)
    await restore()
    await connect(true)
    await activateLobby()
  })
  onActivated(async () => {
    await restore()
    await connect(true)
    await activateLobby()
  })
  onDeactivated(() => {
    if (online.value) void api.watchLobby(false)
  })
  onBeforeUnmount(() => {
    if (online.value) void api.watchLobby(false)
    if (--listenerCount === 0 && disposeListener) {
      disposeListener()
      disposeListener = null
    }
  })

  return {
    state, lastAction, resolved, online, room, isHost, pending,
    connect, disconnect, activateLobby, refreshLobby, create, resolve, join, invite, leave, close, kick, retry,
  }
}
