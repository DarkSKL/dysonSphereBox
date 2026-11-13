import {computed, ref} from 'vue'
import {updater} from '../../../wailsjs/go/models'
import {CheckAppUpdate, UpdateAppSelf} from '../../../wailsjs/go/system/UpdateService'
import {EventsOn} from '../../../wailsjs/runtime/runtime'
import {formatFileSize} from '../../utils/format'

// =============== 更新包下载进度 ===============
// 说明：与后端 system:appUpdateProgress 快照对应；大小未知时只展示已下载状态，不伪造百分比。
export interface AppUpdateProgress {
  readonly stage: 'downloading' | 'verifying'
  readonly downloadedBytes: number
  readonly totalBytes: number
}
const appUpdateProgress = ref<AppUpdateProgress | null>(null)

// 说明：所有更新入口共用字节比例和阶段文案；总量未知时仍展示实际已下载量，不用计时器模拟进度。
// 边界：满进度只表示下载结束，是否更新成功仍以后端调用结果为准。
const appUpdatePercentage = computed(() => {
  const progress = appUpdateProgress.value
  if (progress?.stage === 'verifying') return 100
  if (!progress || progress.totalBytes <= 0) return undefined
  return Math.min(100, Math.max(0, progress.downloadedBytes / progress.totalBytes * 100))
})
const appUpdateLabel = computed(() => {
  const progress = appUpdateProgress.value
  if (!progress) return '准备更新…'
  if (progress.stage === 'verifying') return '校验安装中…'
  return appUpdatePercentage.value === undefined
    ? `已下载 ${formatFileSize(progress.downloadedBytes)}`
    : `下载 ${Math.floor(appUpdatePercentage.value)}%`
})
const appUpdateDetail = computed(() => {
  const progress = appUpdateProgress.value
  if (!progress) return '正在检查版本并连接下载源…'
  const downloaded = formatFileSize(progress.downloadedBytes)
  if (progress.stage === 'verifying') return `已下载 ${downloaded}，正在校验并安装…`
  return progress.totalBytes > 0
    ? `${downloaded} / ${formatFileSize(progress.totalBytes)}`
    : `已下载 ${downloaded}（总大小未知）`
})

interface AppUpdateCheckOptions {
  readonly force?: boolean
}

interface AppUpdateCheckResult {
  readonly result: updater.CheckResult | null
  readonly skipped: boolean
}

const appUpdateInfo = ref<updater.CheckResult | null>(null)
// 最近一次成功取得检查结果的时间，与检查结果同属程序周期缓存
const appUpdateCheckedAt = ref<Date | null>(null)
const isCheckingAppUpdate = ref(false)
const isUpdatingApp = ref(false)
let hasCheckedAppUpdateInProgramSession = false
let appUpdateCheckPromise: Promise<updater.CheckResult> | null = null

// =============== 应用更新运行期状态 ===============
// 说明：缓存本次程序周期内的自动更新检查结果，避免设置页反复进入时重复请求远端。
// 边界：重启应用后模块状态会重新初始化；手动检查会强制刷新远端状态。
export const useAppUpdateState = () => {
  const checkAppUpdate = async (
    options: AppUpdateCheckOptions = {},
  ): Promise<AppUpdateCheckResult> => {
    if (!options.force && hasCheckedAppUpdateInProgramSession) {
      return {
        result: appUpdateInfo.value,
        skipped: true,
      }
    }

    hasCheckedAppUpdateInProgramSession = true

    if (!appUpdateCheckPromise) {
      isCheckingAppUpdate.value = true
      appUpdateCheckPromise = CheckAppUpdate()
        .then((result) => {
          appUpdateInfo.value = result
          appUpdateCheckedAt.value = new Date()
          return result
        })
        .catch((error: unknown) => {
          hasCheckedAppUpdateInProgramSession = false
          throw error
        })
        .finally(() => {
          isCheckingAppUpdate.value = false
          appUpdateCheckPromise = null
        })
    }

    const result = await appUpdateCheckPromise
    return {
      result,
      skipped: false,
    }
  }

  const updateAppSelf = async (): Promise<updater.UpdateResult> => {
    if (isUpdatingApp.value) {
      throw new Error('应用更新正在执行中，请稍候。')
    }

    isUpdatingApp.value = true
    appUpdateProgress.value = null
    // 订阅覆盖整个更新调用，页面切换仍保留模块级进度；成功或失败均只清理本次监听。
    let stopProgress: (() => void) | undefined
    try {
      stopProgress = EventsOn('system:appUpdateProgress', (progress: AppUpdateProgress) => {
        if (!progress || !['downloading', 'verifying'].includes(progress.stage)
          || !Number.isFinite(progress.downloadedBytes) || progress.downloadedBytes < 0
          || !Number.isFinite(progress.totalBytes)) return
        appUpdateProgress.value = progress
      })
      const result = await UpdateAppSelf()
      appUpdateInfo.value = result.check
      return result
    } finally {
      stopProgress?.()
      appUpdateProgress.value = null
      isUpdatingApp.value = false
    }
  }

  return {
    appUpdateInfo,
    appUpdateCheckedAt,
    isCheckingAppUpdate,
    isUpdatingApp,
    appUpdateProgress,
    appUpdatePercentage,
    appUpdateLabel,
    appUpdateDetail,
    checkAppUpdate,
    updateAppSelf,
  }
}

