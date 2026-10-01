import {useMessage} from 'naive-ui'
import {computed, ref} from 'vue'
import {ListInstalledPackages, UninstallRemotePackage} from '../../../wailsjs/go/modDependence/InstallerService'
import {contract} from '../../../wailsjs/go/models'
import {formatErrorMessage} from '../../utils/format'

// =============== 市场安装模组状态 ===============
// 说明：维护资源页“模组市场”分区的已安装包列表、详情选中与卸载状态。
// 边界：只读取 V2 安装记录并调用安装器卸载；安装、冲突处理和进度恢复仍在模组页完成。
export const useInstalledModState = () => {
  const message = useMessage()
  const installedPackages = ref<contract.InstalledPackageSummary[]>([])
  const isLoadingInstalledPackages = ref(false)
  const uninstallingPackageKeys = ref<Record<string, boolean>>({})
  const selectedInstalledPackageKey = ref('')

  const selectedInstalledPackage = computed(() => (
    installedPackages.value.find((pack) => pack.packageKey === selectedInstalledPackageKey.value) ?? null
  ))

  // 说明：读取失败时保留旧列表，避免一次瞬时错误清空页面。
  const loadInstalledPackages = async (): Promise<void> => {
    isLoadingInstalledPackages.value = true
    try {
      installedPackages.value = await ListInstalledPackages()
    } catch (error) {
      message.error(`读取已安装模组失败：${formatErrorMessage(error)}`)
    } finally {
      isLoadingInstalledPackages.value = false
    }
  }

  const openInstalledPackage = (packageKey: string): void => {
    selectedInstalledPackageKey.value = packageKey
  }

  const closeInstalledPackage = (): void => {
    selectedInstalledPackageKey.value = ''
  }

  // 说明：卸载按安装记录清理未共享文件；失败时提示并继续抛出，让确认弹窗保持打开。
  // 边界：安装器串行保护忙碌时同样以错误返回，由用户稍后重试。
  const uninstallInstalledPackage = async (packageKey: string): Promise<void> => {
    if (uninstallingPackageKeys.value[packageKey]) {
      return
    }
    uninstallingPackageKeys.value[packageKey] = true
    try {
      const result = await UninstallRemotePackage(packageKey)
      message.success(result.message || '模组已卸载')
      if (selectedInstalledPackageKey.value === packageKey) {
        closeInstalledPackage()
      }
      await loadInstalledPackages()
    } catch (error) {
      message.error(`模组卸载失败：${formatErrorMessage(error)}`)
      throw error
    } finally {
      uninstallingPackageKeys.value[packageKey] = false
    }
  }

  return {
    installedPackages,
    isLoadingInstalledPackages,
    uninstallingPackageKeys,
    selectedInstalledPackage,
    loadInstalledPackages,
    openInstalledPackage,
    closeInstalledPackage,
    uninstallInstalledPackage,
  }
}
