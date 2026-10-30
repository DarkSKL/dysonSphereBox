// =============== 账号接口契约 ===============
// 说明：集中维护账号认证、全局响应式会话状态与统一刷新入口，字段以 auth-account-api.md 为准。
// 使用：账号界面和需要账号数据的页面读取本模块导出的响应式状态，并通过 refreshAccountSession 统一刷新。
// 边界：前端只调用 Wails 绑定的 AccountService，由客户端后端经全局 HTTP 客户端请求应用服务器；
// token 由后端保存，不进入前端存储。未运行在 Wails 桌面环境时绑定不存在，调用直接返回可读错误。

import {computed, ref} from 'vue'
import {
  ChangePassword,
  GetCaptcha,
  GetSessionState,
  Login,
  Logout,
  Register,
  SetSelfInfo,
  SetSelfSetting,
  SyncSession,
  UploadAvatar,
} from '../../wailsjs/go/system/AccountService'
import {system} from '../../wailsjs/go/models'
import defaultAvatarAssetUrl from '../../../resources/images/avatar_dsq.png?url'
import {applyServerAddressFromSession, loadServerAddress} from './server'

export type CaptchaInfo = system.AccountCaptcha
export type AccountUser = system.AccountUser
export type AccountSession = system.AccountSessionState

// defaultAvatarUrl 是账号无头像、头像解析失败或图片加载失败时统一使用的默认头像。
// 使用：快捷栏、账号弹窗和联机座位只在该地址为空或加载失败时渲染它。
// 边界：资源来自项目根 resources/images，由 Vite 打包为静态地址，不依赖账号服务器。
export const defaultAvatarUrl = defaultAvatarAssetUrl

// 账号会话阶段。
// 说明：后端返回 initializing、anonymous、pending、authenticated、passwordRequired、expired、offline；
// 前端额外使用 idle 与 error 表示尚未开始同步或绑定调用失败。
export type AccountStatus = 'idle' | 'syncing' | 'initializing' | 'anonymous' | 'pending' | 'authenticated' | 'passwordRequired' | 'expired' | 'offline' | 'error'

const accountStatuses = new Set<AccountStatus>([
  'initializing', 'anonymous', 'pending', 'authenticated', 'passwordRequired', 'expired', 'offline',
])

interface AccountBridgeWindow extends Window {
  go?: {
    system?: {
      AccountService?: Record<string, unknown>
    }
  }
}

// 旧版本曾把 token 存在 WebView localStorage，现改由后端持有，这里清理残留
const legacyTokenStorageKey = 'dsbox:account-token'
try {
  localStorage.removeItem(legacyTokenStorageKey)
} catch {
  // WebView 存储不可用时无需清理
}

// =============== 全局会话状态 ===============
// 说明：账号用户、会话快照和同步阶段只在本模块维护，所有页面通过导出的响应式引用读取，避免各自缓存导致状态分裂。
// 边界：页面不得直接调用 GetSessionState 或自行拼装登录态；需要最新资料时统一调用 refreshAccountSession。
const session = ref<AccountSession | null>(null)
const currentUser = ref<AccountUser | null>(null)
const sessionStatus = ref<AccountStatus>('idle')
const lastError = ref('')
const bootstrapDone = ref(false)
const avatarUploading = ref(false)
let stateSequence = 0
let sessionRevision = 0
let userRevision = 0

// getAccountSessionRevision 返回会话身份代次。
// 说明：登录、退出、切服或身份变化时递增，用于丢弃迟到响应；不代表同步次数。
export const getAccountSessionRevision = (): number => sessionRevision

const authenticated = computed(() => session.value?.authenticated === true)
const needsPasswordChange = computed(() => session.value?.needChangePassword === true)
// 已验证会话才允许公告、翻译等能力使用；仅有 token 或离线缓存不足以调用这些接口。
export const verifiedAccountSession = computed<AccountSession | null>(() => {
  const state = session.value
  return state && state.authenticated && state.verified && !state.needChangePassword && state.subjectId && currentUser.value
    ? state
    : null
})
// 公告、翻译和联机使用同一作用域键，身份或服务器变化时所有依赖方一起失效。
export const accountSessionKey = computed(() => {
  const state = verifiedAccountSession.value
  return state ? [state.serverKey, state.subjectId, state.generation].join('\n') : ''
})
export const accountUser = currentUser
export const accountIsAuthenticated = authenticated
export const accountNeedsPasswordChange = needsPasswordChange
export const accountSessionStatus = computed(() => sessionStatus.value)
export const accountSessionReady = computed(() => bootstrapDone.value)
export const accountLastError = computed(() => lastError.value)
export const accountAvatarUploading = computed(() => avatarUploading.value)

export class AccountApiError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'AccountApiError'
  }
}

// =============== 通用状态处理 ===============
// 说明：把后端返回的段、原始错误统一转换为可展示文本；账号数据只通过 applySessionState 写入全局状态。
const statusOf = (state: AccountSession): AccountStatus => {
  if (state.status && accountStatuses.has(state.status as AccountStatus)) return state.status as AccountStatus
  if (!state.ready) return 'initializing'
  if (!state.authenticated) return 'anonymous'
  if (state.needChangePassword) return 'passwordRequired'
  return state.verified ? 'authenticated' : 'pending'
}

const errorMessage = (error: unknown): string => {
  if (error instanceof Error) return error.message
  const text = String(error ?? '').trim()
  return text || '账号请求失败'
}

const delay = (milliseconds: number): Promise<void> => new Promise((resolve) => setTimeout(resolve, milliseconds))

const hasAccountBridge = (): boolean => (
  Boolean((window as AccountBridgeWindow).go?.system?.AccountService)
)

const stateIdentity = (state: AccountSession | null): string => (
  state?.authenticated ? [state.serverKey, state.subjectId, state.generation].join('\n') : ''
)

function clearLocalSession(): void {
  stateSequence++
  sessionRevision++
  userRevision++
  session.value = null
  currentUser.value = null
  sessionStatus.value = 'anonymous'
  lastError.value = ''
}

// applySessionState 原子更新全局账号状态。
// 说明：身份变化时递增代次并清空旧用户，随后写入新会话、缓存用户、错误提示和服务器地址。
// 边界：只有当前请求代次可以调用；迟到响应由调用方在应用前丢弃。
function applySessionState(state: AccountSession, options: {message?: string; userRevision?: number} = {}): void {
  const previous = session.value
  const previousIdentity = stateIdentity(previous)
  const nextIdentity = stateIdentity(state)
  const identityChanged = nextIdentity !== previousIdentity
  if (identityChanged || Boolean(previous?.authenticated) !== Boolean(state.authenticated)) {
    sessionRevision++
    if (identityChanged) {
      userRevision++
      currentUser.value = null
    }
  }
  // 先校准服务器地址，再发布会话和用户；同步 watcher 立即读取公告作用域时不会拿到旧地址。
  if (state.serverKey) applyServerAddressFromSession(state.serverKey)
  session.value = state
  sessionStatus.value = statusOf(state)
  lastError.value = (options.message ?? '').trim()
  if (!state.authenticated) {
    currentUser.value = null
  } else if (state.user && (identityChanged || options.userRevision === undefined || options.userRevision === userRevision)) {
    // 身份切换时快照用户属于新身份；同身份则用 revision 防止上传头像前发起的同步覆盖新头像。
    currentUser.value = state.user
  }
}

// applyAccountUser 直接应用服务器已确认的用户响应，并让旧同步结果失效。
// 说明：登录与头像上传响应本身已包含完整用户，不需要再发一次资料请求。
// 边界：不改变会话身份；会话已被清除时由调用方负责丢弃响应。
function applyAccountUser(user: AccountUser): void {
  userRevision++
  currentUser.value = user
  if (session.value) session.value.user = user
}

// =============== 统一刷新 ===============
// 说明：所有需要最新会话或用户资料的地方调用 refreshAccountSession；同一时刻只执行一个同步，force 请求会在当前同步后补跑。
// 边界：网络失败不清除 token；只有后端返回 expired 才表示登录失效。
// AccountRefreshResult 描述一次统一刷新的结果：ok 表示调用完成，status 才是会话阶段，message 为可展示原因。
export interface AccountRefreshResult {
  readonly ok: boolean
  readonly status: AccountStatus
  readonly message: string
}

let refreshPromise: Promise<AccountRefreshResult> | null = null
let queuedForceRefresh = false

export function refreshAccountSession(options: {force?: boolean; reason?: string} = {}): Promise<AccountRefreshResult> {
  if (options.force) queuedForceRefresh = true
  if (!refreshPromise) {
    refreshPromise = drainRefreshQueue().finally(() => { refreshPromise = null })
  }
  return refreshPromise
}

async function drainRefreshQueue(): Promise<AccountRefreshResult> {
  let result: AccountRefreshResult = {ok: false, status: sessionStatus.value, message: lastError.value}
  do {
    const force = queuedForceRefresh
    queuedForceRefresh = false
    result = await refreshAccountOnce(force)
  } while (queuedForceRefresh)
  return result
}

async function refreshAccountOnce(force: boolean): Promise<AccountRefreshResult> {
  if (!hasAccountBridge()) {
    lastError.value = '账号功能需要在戴森球盒子桌面应用中运行（wails dev 或打包应用）。'
    if (!session.value?.authenticated) sessionStatus.value = 'error'
    return {ok: false, status: sessionStatus.value, message: lastError.value}
  }
  const sequence = ++stateSequence
  const revision = sessionRevision
  const requestUserRevision = userRevision
  if (!session.value) sessionStatus.value = 'syncing'
  try {
    const snapshot = await SyncSession(force)
    if (sequence !== stateSequence || revision !== sessionRevision) {
      return {ok: false, status: sessionStatus.value, message: '账号会话已切换。'}
    }
    applySessionState(snapshot.session, {message: snapshot.message, userRevision: requestUserRevision})
    return {ok: true, status: sessionStatus.value, message: lastError.value}
  } catch (error) {
    const message = errorMessage(error)
    lastError.value = message
    if (!session.value?.authenticated) sessionStatus.value = 'error'
    else if (!currentUser.value) sessionStatus.value = 'offline'
    return {ok: false, status: sessionStatus.value, message}
  }
}

// syncSessionState 只读取后端本地会话快照，不发起远端验证。
// 说明：账号写操作完成后用本地快照同步 401、改密和身份变化；需要用户资料时调用 refreshAccountSession。
// 边界：失败时保留当前状态；旧请求或旧代次结果不得覆盖新状态。
async function syncSessionState(expectedRevision = sessionRevision): Promise<void> {
  if (!hasAccountBridge()) return
  const sequence = ++stateSequence
  const requestUserRevision = userRevision
  try {
    const state = await GetSessionState()
    if (sequence !== stateSequence || expectedRevision !== sessionRevision) return
    applySessionState(state, {userRevision: requestUserRevision})
  } catch {
    // 同步失败时保留当前状态，下一次统一刷新会再次同步
  }
}

// =============== 启动引导 ===============
// 说明：应用启动时等待后端基础设施就绪，加载服务器地址，再完成一次远端会话验证；网络短暂失败时自动重试。
// 边界：引导只执行一次；进入页面或打开账号入口时仍可再次调用 refreshAccountSession 重试。
let bootstrapPromise: Promise<void> | null = null

export function bootstrapAccountSession(): Promise<void> {
  bootstrapPromise ??= runBootstrap()
  return bootstrapPromise
}

async function runBootstrap(): Promise<void> {
  if (sessionStatus.value === 'idle') sessionStatus.value = 'initializing'
  let ready = false
  for (let attempt = 0; attempt < 60; attempt += 1) {
    if (!hasAccountBridge()) {
      await delay(100)
      continue
    }
    try {
      const state = await GetSessionState()
      if (state.ready) {
        applySessionState(state, {userRevision: userRevision})
        ready = true
        break
      }
    } catch {
      // Wails startup 与前端挂载并发，后端基础设施可能尚未初始化，短暂等待后重试
    }
    await delay(150)
  }
  if (!ready) {
    bootstrapPromise = null
    lastError.value = '账号服务初始化超时，可稍后在账号入口重试。'
    if (!session.value?.authenticated) sessionStatus.value = 'error'
    // Wails 桥接存在但后端尚未就绪时保持 ready=false，打开账号入口会再次引导；
    // 纯浏览器预览没有桥接，则标记引导结束，避免每次打开账号入口都重复等待。
    bootstrapDone.value = !hasAccountBridge()
    return
  }
  // 先读取服务器地址，保证公告作用域与头像地址和账号请求使用同一目标；失败时稍后由会话快照补齐。
  await loadServerAddress(true)
  let result = await refreshAccountSession({force: true, reason: 'bootstrap'})
  for (let attempt = 0; attempt < 2 && result.status === 'offline'; attempt += 1) {
    await delay(800)
    result = await refreshAccountSession({force: true, reason: 'bootstrap-retry'})
  }
  bootstrapDone.value = true
}

// =============== 后端调用与会话同步 ===============
// 说明：写操作捕获会话代次，完成后用本地快照同步状态；Wails 绑定以字符串拒绝 Promise，这里统一转换为 AccountApiError。
async function callAccount<T>(action: () => Promise<T>): Promise<T> {
  const revision = sessionRevision
  if (!hasAccountBridge()) {
    throw new AccountApiError('账号功能需要在戴森球盒子桌面应用中运行（wails dev 或打包应用）。')
  }

  try {
    const result = await action()
    if (revision !== sessionRevision) throw new AccountApiError('账号会话已切换。')
    return result
  } catch (error) {
    throw new AccountApiError(errorMessage(error))
  } finally {
    if (revision === sessionRevision) await syncSessionState(revision)
  }
}

// =============== 公开认证接口 ===============
// 说明：验证码、注册和登录不携带 token；验证码按后端的一次性规则在每次提交后刷新。
export function fetchCaptcha(): Promise<CaptchaInfo> {
  return callAccount(() => GetCaptcha())
}

// registerAccount 按文档字段创建账号，注册成功后不会自动登录。
export function registerAccount(payload: {
  username: string
  password: string
  captcha: string
  captchaId: string
}): Promise<AccountUser> {
  return callAccount(() => Register(system.AccountRegisterRequest.createFrom(payload)))
}

// loginAccount 登录，token 由后端保存；登录响应会建立已验证主体与用户缓存，无需再点击一次头像同步。
export async function loginAccount(payload: {
  username: string
  password: string
  captcha?: string
  captchaId?: string
}): Promise<system.AccountLoginResult> {
  clearLocalSession()
  const revision = sessionRevision
  const result = await callAccount(() => Login(system.AccountLoginRequest.createFrom(payload)))
  if (revision === sessionRevision) {
    // 登录响应自带会话快照时直接原子应用，不依赖随后一次本地读取；旧后端缺少该字段时再回退到统一刷新。
    if (result.session) applySessionState(result.session, {userRevision: userRevision})
    if (result.user) applyAccountUser(result.user)
  }
  if (revision === sessionRevision && !accountIsAuthenticated.value) {
    await refreshAccountSession({force: true, reason: 'login'})
  }
  return result
}

// =============== 头像即时上传 ===============
// 说明：上传状态由模块共享，关闭弹窗也继续完成；服务器返回用户后直接更新全局资料。
// 边界：失败保留当前头像；会话切换后丢弃旧响应，重复调用不会再次上传。
export async function uploadAccountAvatar(file: File): Promise<AccountUser> {
  if (avatarUploading.value) throw new AccountApiError('头像正在上传，请稍候。')
  if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type) || !file.size || file.size > 2 * 1024 * 1024) {
    throw new AccountApiError('请选择不超过 2 MiB 的 JPEG、PNG 或 WebP 图片。')
  }
  avatarUploading.value = true
  const session = sessionRevision
  const userID = currentUser.value?.ID
  try {
    const content = await new Promise<string>((resolve, reject) => {
      const reader = new FileReader()
      reader.onload = () => typeof reader.result === 'string'
        ? resolve(reader.result.slice(reader.result.indexOf(',') + 1))
        : reject(new AccountApiError('读取图片失败，请重新选择图片重试。'))
      reader.onerror = () => reject(new AccountApiError('读取图片失败，请重新选择图片重试。'))
      reader.onabort = () => reject(new AccountApiError('读取图片已取消，请重新选择图片。'))
      reader.readAsDataURL(file)
    })
    if (session !== sessionRevision) throw new AccountApiError('账号会话已切换，请重新选择图片。')
    const user = await callAccount(() => UploadAvatar(file.name, content))
    if (session !== sessionRevision || !accountIsAuthenticated.value || user.ID !== userID) {
      throw new AccountApiError('账号会话已变化，请重新打开资料页确认头像。')
    }
    applyAccountUser(user)
    return user
  } finally {
    avatarUploading.value = false
  }
}

// updateSelfInfo 部分更新当前用户资料，并在保存成功后统一刷新全局用户。
// 说明：省略字段保持原值，显式空字符串清除头像和联系方式；返回最新用户，刷新失败时返回 null。
export async function updateSelfInfo(payload: {
  nickName?: string
  headerImg?: ''
  phone?: string
  email?: string
}): Promise<AccountUser | null> {
  await callAccount(() => SetSelfInfo(system.AccountSelfInfoRequest.createFrom(payload)))
  const result = await refreshAccountSession({force: true, reason: 'profile'})
  return result.ok && currentUser.value ? currentUser.value : null
}

// updateSelfSetting 覆盖保存个人界面设置对象。
export function updateSelfSetting(settings: Record<string, unknown>): Promise<void> {
  return callAccount(() => SetSelfSetting(settings))
}

// changeAccountPassword 使用当前密码修改账户密码。
export function changeAccountPassword(password: string, newPassword: string): Promise<void> {
  return callAccount(() => ChangePassword(system.AccountChangePasswordRequest.createFrom({password, newPassword})))
}

// logoutAccount 请求注销；后端无论请求结果如何都会清除已保存的 token。
export async function logoutAccount(): Promise<void> {
  // 全局响应式状态是唯一订阅源；清空后依赖页面通过 accountSessionKey、accountUser 等自动失效。
  clearLocalSession()
  await callAccount(() => Logout())
}

// resetAccountSession 清除前端会话状态；服务器地址切换后后端已清除 token，这里同步界面状态并重新读取快照。
export function resetAccountSession(): void {
  clearLocalSession()
  void refreshAccountSession({reason: 'server-changed'})
}

// resolveAvatarUrl 把账号头像字段解析为可直接渲染的绝对地址。
// 说明：相对路径属于账号服务器，不能落到 Wails 本地资源根；地址未加载完成时返回空串，由调用方回退到图标或首字母。
// 边界：只接受 http/https，拒绝带账号信息的地址和协议相对地址；不附带 token。
export function resolveAvatarUrl(raw: string | undefined, serverAddress: string, ready: boolean): string {
  const value = raw?.trim() ?? ''
  if (!value || !ready || value.startsWith('//')) return ''
  try {
    const url = new URL(value, `${serverAddress}/`)
    return ['http:', 'https:'].includes(url.protocol) && !url.username && !url.password ? url.href : ''
  } catch {
    return ''
  }
}
