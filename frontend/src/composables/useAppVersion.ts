import {ref} from 'vue'
import {GetAppVersion} from '../../wailsjs/go/system/UpdateService'

const appVersion = ref('0.0.0-dev')
let appVersionPromise: Promise<void> | null = null

// =============== 应用版本状态 ===============
// 说明：缓存后端构建版本，供布局和关于页展示。
// 边界：后端调用失败时保留开发版兜底文案，不阻塞页面渲染。
export const useAppVersion = () => {
  const loadAppVersion = async (): Promise<void> => {
    if (!appVersionPromise) {
      appVersionPromise = GetAppVersion()
        .then((version) => {
          appVersion.value = version || '0.0.0-dev'
        })
        .catch(() => {
          appVersion.value = '0.0.0-dev'
        })
    }

    await appVersionPromise
  }

  return {
    appVersion,
    loadAppVersion,
  }
}
