// =============== 联机页面状态 ===============
// 说明：共享 Go 投影，编排页面激活、连接恢复与大厅请求；不在前端重建房间或持有账号 token。
// 逻辑流：恢复投影 => 仅离线时自动连接 => 在线后同步大厅 => 停用时取消订阅，保留房间连接。
import {computed, onActivated, onBeforeUnmount, onDeactivated, onMounted, reactive, ref, watch} from 'vue'
import {accountIsAuthenticated} from '../../services/account'
import * as api from '../../services/multiplayer'
import {codeText, type LobbyQuery, type MultiplayerActionResult, type MultiplayerState, type RoomCreateForm, type RoomResolveResult} from '../../types/multiplayer'

const state = ref<MultiplayerState | null>(null)
const lastAction = ref<{ok: boolean; text: string} | null>(null)
let listenerCount = 0
let disposeListener: (() => void) | null = null

function applyState(next: MultiplayerState | null): void {
  if (!next || (state.value && next.stateRevision <= state.value.stateRevision)) return
  state.value = next
}

async function restore(): Promise<void> {
  applyState(await api.getMultiplayerState())
}

// 说明：用户操作反馈使用统一码表，自动恢复不重复弹出错误。
function report(result: MultiplayerActionResult, successText = ''): boolean {
  const ok = result.code === 0
  if (!ok) lastAction.value = {ok, text: codeText(result.code, result.msg)}
  else if (successText) lastAction.value = {ok, text: successText}
  return ok
}

export function useMultiplayerState() {
  const resolved = ref<RoomResolveResult | null>(null)
  const active = ref(false)
  const localPending = ref(new Set<string>())
  const query = reactive<LobbyQuery>({keyword: '', page: 1, pageSize: 20, sort: 'createdAt'})
  const lobbyReady = ref(false)
  const lobbyError = ref('')
  const lobbyLoading = ref(false)
  let resolveRevision = 0
  let syncRevision = 0
  let lobbyQueue: Promise<void> = Promise.resolve()

  const online = computed(() => state.value?.connection.state === 'online')
  const admissionEnabled = computed(() => {
    const policy = state.value?.session?.policy
    return online.value && (!policy || (policy.admissionEnabled && !policy.maintenance))
  })
  const room = computed(() => state.value?.room ?? null)
  const isHost = computed(() => room.value?.role === 'host')
  const pending = computed(() => new Set([...state.value?.pending ?? [], ...localPending.value]))
  const roomBusy = computed(() => ['createRoom', 'joinRoom', 'leave', 'close'].some((key) => pending.value.has(key)))
  // 说明：lobby.change 不携带搜索分页上下文，仅默认第一页接收实时增量，避免混入其他房间。
  const liveLobby = computed(() => !query.keyword && query.page === 1 && query.sort === 'createdAt')

  // =============== 意图执行与预览失效 ===============
  // 说明：控制链路未在线时阻止业务意图，仅允许连接；立即锁住重复点击，补足后端 pending 推送前的空隙。
  async function run(name: string, action: () => Promise<MultiplayerActionResult>, successText = '', silent = false): Promise<MultiplayerActionResult | null> {
    if (!online.value && name !== 'connect') return null
    if (pending.value.has(name)) return null
    localPending.value = new Set(localPending.value).add(name)
    try {
      const result = await action()
      if (!silent) report(result, successText)
      return result
    } finally {
      const next = new Set(localPending.value)
      next.delete(name)
      localPending.value = next
    }
  }

  function clearResolved(): void {
    resolveRevision++
    resolved.value = null
  }

  async function connect(silent = false): Promise<void> {
    if (state.value?.connection.state !== 'offline' && state.value) return
    await run('connect', api.connectMultiplayer, '', silent)
    await restore()
  }

  async function create(form: RoomCreateForm): Promise<boolean> {
    if (!admissionEnabled.value || room.value || roomBusy.value) return false
    const result = await run('createRoom', () => api.createRoom(form), '房间已创建')
    return result?.code === 0
  }

  async function resolve(target: string): Promise<void> {
    if (!online.value || room.value || pending.value.has('resolveRoom')) return
    clearResolved()
    const revision = resolveRevision
    const invite = !target.startsWith('r_')
    const result = await run('resolveRoom', () => api.resolveRoom(invite ? '' : target, invite ? target : ''), '', true)
    if (revision !== resolveRevision || !online.value || room.value || !result) return
    if (report(result)) resolved.value = result.data as RoomResolveResult
  }

  async function join(target: string, password: string): Promise<boolean> {
    if (!admissionEnabled.value || room.value || roomBusy.value || !resolved.value) return false
    const invite = !target.startsWith('r_')
    const result = await run('joinRoom', () => api.joinRoom(invite ? '' : target, invite ? target : '', password), '已加入房间')
    if (result?.code === 0) clearResolved()
    else if (result && [20201, 20202, 20203, 20210].includes(result.code)) clearResolved()
    return result?.code === 0
  }

  const invite = () => run('invite', api.inviteRoom, '已生成邀请码')
  const leave = () => roomBusy.value ? Promise.resolve(null) : run('leave', api.leaveRoom)
  const close = () => roomBusy.value ? Promise.resolve(null) : run('close', api.closeRoom)
  const kick = (userId: number) => run('kick', () => api.kickMember(userId))
  const retry = () => run('retry', api.retryLink)

  // =============== 大厅同步 ===============
  // 说明：订阅切换和列表请求串行；换页、筛选、重连后重新读取，不让旧搜索覆盖新搜索。
  function refreshLobby(): Promise<void> {
    const revision = ++syncRevision
    lobbyReady.value = false
    lobbyLoading.value = online.value && active.value && !room.value
    lobbyError.value = ''
    lobbyQueue = lobbyQueue.then(async () => {
      if (revision !== syncRevision || !online.value) return
      const browsing = active.value && !room.value
      const watchOn = browsing && liveLobby.value
      // 后端重连会恢复旧订阅，因此每次同步以当前页面意图为准。
      const watchResult = await api.watchLobby(watchOn)
      if (revision !== syncRevision || !online.value || !browsing) return
      if (watchResult.code !== 0) {
        lobbyError.value = codeText(watchResult.code, watchResult.msg)
      } else {
        const result = await run('lobbyList', () => api.refreshLobby({...query}), '', true)
        if (revision !== syncRevision) return
        if (result && result.code !== 0) lobbyError.value = codeText(result.code, result.msg)
        lobbyReady.value = !!result && result.code === 0
      }
    }).finally(() => {
      if (revision === syncRevision) lobbyLoading.value = false
    })
    return lobbyQueue
  }

  watch(() => [active.value, online.value, room.value?.snapshot.roomId, query.keyword, query.page, query.pageSize, query.sort], () => {
    if (!online.value || room.value) clearResolved()
    void refreshLobby()
  })

  // =============== 缓存页面生命周期 ===============
  // 说明：登录只更新账号共享状态，不会推送联机投影；停留本页时重新读取后端，交给下方投影监听触发连接。
  // 边界：退出登录也恢复投影；缓存页停用时不发请求，重新激活后统一恢复，不在前端伪造登录态。
  watch(accountIsAuthenticated, () => {
    if (active.value) void restore()
  })

  // 说明：首次 mounted/activated 共用激活锁；请求恢复期间切走页面，不再自动连接。
  async function activate(): Promise<void> {
    if (active.value) return
    active.value = true
    await restore()
    if (!active.value) return
    if (state.value?.connection.state === 'offline' && state.value.account.loggedIn && !state.value.connection.lastErrorCode) await connect(true)
  }

  // 说明：控制链路强制保持；页面停留期间登录成功也立即自动连接，不可重连原因导致的离线仍等用户手动重连。
  watch(() => state.value?.account.loggedIn, (loggedIn, wasLoggedIn) => {
    if (active.value && loggedIn && !wasLoggedIn && state.value?.connection.state === 'offline' && !state.value.connection.lastErrorCode) void connect(true)
  })

  onMounted(() => {
    if (listenerCount++ === 0) disposeListener = api.onMultiplayerState(applyState)
    void activate()
  })
  onActivated(() => { void activate() })
  onDeactivated(() => { active.value = false })
  onBeforeUnmount(() => {
    active.value = false
    clearResolved()
    if (--listenerCount === 0 && disposeListener) {
      disposeListener()
      disposeListener = null
    }
  })

  return {
    state, lastAction, resolved, online, admissionEnabled, room, isHost, pending, roomBusy,
    query, liveLobby, lobbyReady, lobbyLoading, lobbyError,
    connect, refreshLobby, clearResolved, create, resolve, join, invite, leave, close, kick, retry,
  }
}
