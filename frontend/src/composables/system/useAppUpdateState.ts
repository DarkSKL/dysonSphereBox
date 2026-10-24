import {ref} from 'vue'
import {updater} from '../../../wailsjs/go/models'
import {CheckAppUpdate, UpdateAppSelf} from '../../../wailsjs/go/system/UpdateService'

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
    try {
      const result = await UpdateAppSelf()
      appUpdateInfo.value = result.check
      return result
    } finally {
      isUpdatingApp.value = false
    }
  }

  return {
    appUpdateInfo,
    appUpdateCheckedAt,
    isCheckingAppUpdate,
    isUpdatingApp,
    checkAppUpdate,
    updateAppSelf,
  }
}

