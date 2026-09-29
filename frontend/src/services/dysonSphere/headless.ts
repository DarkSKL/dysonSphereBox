import {
  GetEnvironmentStatus as getEnvironmentStatusBinding,
  PrepareEnvironment as prepareEnvironmentBinding,
  SaveSettings as saveSettingsBinding,
  StartEnvironment as startEnvironmentBinding,
  StopEnvironment as stopEnvironmentBinding,
} from '../../../wailsjs/go/modDependence/HeadlessService'
import {contract} from '../../../wailsjs/go/models'

interface HeadlessBridgeWindow extends Window {
  go?: {
    modDependence?: {
      HeadlessService?: Record<string, unknown>
    }
  }
}

// =============== Wails 开服桥接 ===============
// 说明：集中封装 HeadlessService 调用，避免页面直接读取未注入的 window.go 时抛出低层 TypeError。
// 边界：若当前运行的 Wails 后端尚未绑定 HeadlessService，会返回可读错误，提示重启 dev/应用。
type HeadlessBridgeMethod =
  | 'GetEnvironmentStatus'
  | 'PrepareEnvironment'
  | 'SaveSettings'
  | 'StartEnvironment'
  | 'StopEnvironment'

const assertHeadlessBridgeReady = (methodName: HeadlessBridgeMethod): void => {
  const bridge = (window as HeadlessBridgeWindow).go?.modDependence?.HeadlessService
  if (typeof bridge?.[methodName] !== 'function') {
    throw new Error(`HeadlessService.${methodName} 尚未绑定，请重新启动 Wails 开发服务或应用后再试。`)
  }
}

export const getHeadlessEnvironmentStatus = async (): Promise<contract.HeadlessEnvironmentStatus> => {
  assertHeadlessBridgeReady('GetEnvironmentStatus')
  return getEnvironmentStatusBinding()
}

export const prepareHeadlessEnvironment = async (): Promise<contract.HeadlessPrepareResult> => {
  assertHeadlessBridgeReady('PrepareEnvironment')
  return prepareEnvironmentBinding()
}

export const saveHeadlessSettings = async (hostPort: number, serverArchive: string): Promise<contract.HeadlessSettings> => {
  assertHeadlessBridgeReady('SaveSettings')
  return saveSettingsBinding(hostPort, serverArchive)
}

export const startHeadlessEnvironment = async (): Promise<contract.HeadlessStartResult> => {
  assertHeadlessBridgeReady('StartEnvironment')
  return startEnvironmentBinding()
}

export const stopHeadlessEnvironment = async (): Promise<contract.HeadlessStopResult> => {
  assertHeadlessBridgeReady('StopEnvironment')
  return stopEnvironmentBinding()
}

