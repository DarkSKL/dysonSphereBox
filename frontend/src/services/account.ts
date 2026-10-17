// =============== 账号接口契约 ===============
// 说明：集中维护账号认证调用与当前会话状态，字段以 auth-account-api.md 为准。
// 使用：账号界面和需要账号数据的页面调用本模块导出的方法。
// 边界：前端只调用 Wails 绑定的 AccountService，由客户端后端经全局 HTTP 客户端请求应用服务器；
// token 由后端保存，不进入前端存储。未运行在 Wails 桌面环境时绑定不存在，调用直接返回可读错误。

import {computed, ref} from 'vue'
import {
  ChangePassword,
  GetCaptcha,
  GetCurrentUser,
  GetSessionState,
  Login,
  Logout,
  Register,
  SetSelfInfo,
  SetSelfSetting,
  UploadAvatar,
} from '../../wailsjs/go/system/AccountService'
import {system} from '../../wailsjs/go/models'

export type CaptchaInfo = system.AccountCaptcha
export type AccountUser = system.AccountUser

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

const authenticated = ref(false)
const currentUser = ref<AccountUser | null>(null)
const needChangePassword = ref(false)
const avatarUploading = ref(false)
export const verifiedAccountSession = ref<system.AccountSessionState | null>(null)
let stateSequence = 0
let sessionRevision = 0
let userRevision = 0
export const getAccountSessionRevision = (): number => sessionRevision
const isAuthenticated = computed(() => authenticated.value)

export class AccountApiError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'AccountApiError'
  }
}

function clearLocalSession(): void {
  sessionRevision++
  authenticated.value = false
  currentUser.value = null
  needChangePassword.value = false
  verifiedAccountSession.value = null
}

// =============== 后端调用与会话同步 ===============
// 说明：每次调用后从后端同步登录态与强制改密状态，401、续期、改密等状态变化都以后端为准。
// 边界：Wails 绑定以字符串拒绝 Promise，这里统一转换为 AccountApiError。
export async function syncSessionState(expectedRevision = sessionRevision): Promise<void> {
  const sequence = ++stateSequence
  try {
    const state = await GetSessionState()
    if (expectedRevision !== sessionRevision || sequence !== stateSequence) return
    authenticated.value = state.authenticated
    needChangePassword.value = state.needChangePassword
    verifiedAccountSession.value = state.authenticated && !state.needChangePassword && state.subjectId ? state : null
    if (!state.authenticated) {
      currentUser.value = null
    }
  } catch {
    // 同步失败时保留当前状态，下一次调用会再次同步
  }
}

async function callAccount<T>(action: () => Promise<T>): Promise<T> {
  const revision = sessionRevision
  if (!(window as AccountBridgeWindow).go?.system?.AccountService) {
    throw new AccountApiError('账号功能需要在戴森球盒子桌面应用中运行（wails dev 或打包应用）。')
  }

  try {
    const result = await action()
    if (revision !== sessionRevision) throw new AccountApiError('账号会话已切换。')
    return result
  } catch (error) {
    throw new AccountApiError(error instanceof Error ? error.message : String(error || '请求失败'))
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

// loginAccount 登录，token 由后端保存；强制改密标志交由账号界面继续处理。
export async function loginAccount(payload: {
  username: string
  password: string
  captcha?: string
  captchaId?: string
}): Promise<system.AccountLoginResult> {
  clearLocalSession()
  const session = sessionRevision
  const result = await callAccount(() => Login(system.AccountLoginRequest.createFrom(payload)))
  if (session === sessionRevision) currentUser.value = result.user
  return result
}

// =============== 已登录账号接口 ===============
// 说明：统一处理用户信息、资料、个人设置、改密与注销，token 续期和 401 清理由后端完成。
export async function fetchCurrentUser(): Promise<AccountUser> {
  const revision = userRevision
  const session = sessionRevision
  const user = await callAccount(() => GetCurrentUser())
  if (session === sessionRevision && revision === userRevision && authenticated.value) currentUser.value = user
  return user
}

// =============== 头像即时上传 ===============
// 说明：上传状态由模块共享，关闭弹窗也继续完成；仅以服务器返回用户替换全局资料。
// 边界：失败保留当前头像；会话切换后丢弃旧响应，重复调用不会再次上传。
export const accountAvatarUploading = computed(() => avatarUploading.value)

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
    if (session !== sessionRevision || !authenticated.value || user.ID !== userID) {
      throw new AccountApiError('账号会话已变化，请重新打开资料页确认头像。')
    }
    userRevision++
    currentUser.value = user
    return user
  } finally {
    avatarUploading.value = false
  }
}

// updateSelfInfo 部分更新当前用户资料；省略字段保持原值，显式空字符串清除头像和联系方式。
export function updateSelfInfo(payload: {
  nickName?: string
  headerImg?: ''
  phone?: string
  email?: string
}): Promise<void> {
  return callAccount(() => SetSelfInfo(system.AccountSelfInfoRequest.createFrom(payload)))
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
  clearLocalSession()
  window.dispatchEvent(new Event('account:cleared'))
  await callAccount(() => Logout())
}

// restoreAccountSession 启动时同步后端会话，并用已保存 token 获取用户资料；网络异常时保留会话以便稍后重试。
let restorePromise: Promise<void> | undefined
export function restoreAccountSession(): Promise<void> {
  return restorePromise ??= restoreSessionOnce()
}
async function restoreSessionOnce(): Promise<void> {
  if (!(window as AccountBridgeWindow).go?.system?.AccountService) {
    return
  }
  await syncSessionState()
  if (!authenticated.value) {
    return
  }
  try {
    await fetchCurrentUser()
  } catch {
    // 网络不可用时保留会话；401 响应已由后端清理并在同步时反映到前端。
  }
}

// resetAccountSession 清除前端会话状态；服务器地址切换后后端已清除 token，这里同步界面状态。
export function resetAccountSession(): void {
  clearLocalSession()
  window.dispatchEvent(new Event('account:cleared'))
  void syncSessionState()
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

export const accountUser = currentUser
export const accountIsAuthenticated = isAuthenticated
export const accountNeedsPasswordChange = needChangePassword
