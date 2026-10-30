// =============== 服务器连接地址 ===============
// 说明：集中维护应用服务（账号等）的服务器基础地址，持久化到 System/ServerAddress 配置。
// 使用：全局设置页读写、测试地址。
// 边界：配置为空表示使用内置默认地址。前端不直接请求服务器，账号等请求经 Wails 绑定进入客户端后端，
// 由后端全局 HTTP 客户端按已保存地址访问服务器；保存地址时后端同步替换该客户端。

import {computed, ref} from 'vue'
import {GetConfig, SetConfig} from '../../wailsjs/go/system/ConfigurationService'
import {ProbeServerAddress} from '../../wailsjs/go/system/ServerService'

const SERVER_CONFIG_GROUP = 'System'
const SERVER_CONFIG_KEY = 'ServerAddress'

// 需与后端 constant/system/server.go 中的 DefaultServerAddress 保持一致
export const DEFAULT_SERVER_ADDRESS = 'https://dysonbox.cn:32888'

export interface ServerProbeResult {
  readonly ok: boolean
  readonly latencyMs: number
  readonly message: string
}

interface ServerBridgeWindow extends Window {
  go?: {
    system?: {
      ServerService?: Record<string, unknown>
    }
  }
}

// 已保存的自定义地址；空字符串表示使用默认地址
const customServerAddress = ref('')
const isServerAddressReady = ref(false)
const effectiveServerAddress = computed(() => customServerAddress.value || DEFAULT_SERVER_ADDRESS)
let loadServerAddressPromise: Promise<boolean> | null = null
let serverAddressLoadSequence = 0

// =============== 地址归一 ===============
// 说明：与后端 utils.NormalizeHTTPBaseURL 保持同一规则，保存前先在前端给出即时错误提示。
// 边界：只接受 http/https 基础地址，拒绝账号信息、查询参数和锚点；返回值不带末尾斜杠。
export function normalizeServerAddress(raw: string): string {
  const value = raw.trim()
  if (!value) {
    throw new Error('地址不能为空')
  }

  let parsed: URL
  try {
    parsed = new URL(value)
  } catch {
    throw new Error('地址格式无法识别')
  }

  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
    throw new Error('地址必须以 http:// 或 https:// 开头')
  }
  if (!parsed.hostname) {
    throw new Error('地址缺少主机名')
  }
  if (parsed.username || parsed.password) {
    throw new Error('地址不能包含账号或密码')
  }
  if (parsed.search || parsed.hash || value.includes('?') || value.includes('#')) {
    throw new Error('地址不能包含查询参数或锚点')
  }

  return `${parsed.protocol}//${parsed.host}${parsed.pathname.replace(/\/+$/, '')}`
}

// =============== 配置读写 ===============
// 说明：首次调用时从数据库加载地址，后续复用同一 Promise；强制刷新用于后端启动完成后重新读取。
// 边界：读取失败时回退默认地址并保留可重试状态，不把失败结果永久缓存；返回值表示本次是否成功读取配置。
export async function loadServerAddress(force = false): Promise<boolean> {
  if (!force && loadServerAddressPromise) return loadServerAddressPromise
  const sequence = ++serverAddressLoadSequence
  const request = GetConfig(SERVER_CONFIG_GROUP, SERVER_CONFIG_KEY)
    .then((stored) => {
      if (sequence === serverAddressLoadSequence) {
        // 后端会话快照已给出当前生效地址时以快照为准；配置读取晚到不能把公告作用域切回默认地址。
        if (!isServerAddressReady.value) customServerAddress.value = stored.trim()
        isServerAddressReady.value = true
      }
      return true
    })
    .catch(() => {
      if (sequence === serverAddressLoadSequence) {
        isServerAddressReady.value = false
        loadServerAddressPromise = null
      }
      return false
    })
  loadServerAddressPromise = request
  return request
}

// applyServerAddressFromSession 使用后端会话中的有效服务器地址校准前端地址。
// 职责：后端已完成会话验证时，保证头像相对路径、公告作用域和设置页展示使用同一服务器。
// 使用：账号统一同步应用会话快照后调用；设置页保存地址后仍以配置写入结果为准。
// 边界：空地址不覆盖当前状态；默认地址归一为空自定义值，避免把默认地址误存为自定义配置。
export function applyServerAddressFromSession(serverKey: string): void {
  const normalized = serverKey.trim().replace(/\/+$/, '')
  if (!normalized) return
  customServerAddress.value = normalized === DEFAULT_SERVER_ADDRESS ? '' : normalized
  isServerAddressReady.value = true
}

// saveServerAddress 保存自定义地址；传入空字符串或默认地址时恢复为默认。
// 返回值表示实际生效地址是否发生变化，调用方据此决定是否重置账号会话。
export async function saveServerAddress(raw: string): Promise<boolean> {
  const trimmed = raw.trim()
  const normalized = trimmed ? normalizeServerAddress(trimmed) : ''
  const nextCustom = normalized === DEFAULT_SERVER_ADDRESS ? '' : normalized
  const previousEffective = effectiveServerAddress.value

  await SetConfig(SERVER_CONFIG_GROUP, SERVER_CONFIG_KEY, nextCustom)
  customServerAddress.value = nextCustom
  isServerAddressReady.value = true
  return effectiveServerAddress.value !== previousEffective
}

// =============== 连接测试 ===============
// 说明：由后端 ServerService 请求候选地址的验证码接口，确认地址可达且返回账号服务的统一响应结构。
// 边界：只读探测，不保存地址、不写入 token；后端发起请求不受跨域限制。
// 未运行在 Wails 桌面环境时绑定不存在，直接返回失败结论而不是抛出底层 TypeError。
export async function probeServerAddress(raw: string): Promise<ServerProbeResult> {
  const address = normalizeServerAddress(raw)
  const bridge = (window as ServerBridgeWindow).go?.system?.ServerService
  if (typeof bridge?.ProbeServerAddress !== 'function') {
    return {ok: false, latencyMs: 0, message: '连接测试需要在戴森球盒子桌面应用中运行（wails dev 或打包应用）。'}
  }

  const result = await ProbeServerAddress(address)
  return {ok: result.ok, latencyMs: result.latencyMs, message: result.message}
}

export const serverAddress = customServerAddress
export const serverAddressReady = isServerAddressReady
export const serverEffectiveAddress = effectiveServerAddress
