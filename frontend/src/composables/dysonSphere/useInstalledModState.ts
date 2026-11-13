import {useDialog, useMessage} from 'naive-ui'
import {computed, onActivated, onBeforeUnmount, onDeactivated, onMounted, reactive, ref} from 'vue'
import {
  CancelRemotePackageInstall, ContinueRemotePackageInstall, ListInstalledPackages,
  ListInstallProgressSnapshots, UninstallRemotePackage,
} from '../../../wailsjs/go/modDependence/InstallerService'
import {EventsOn} from '../../../wailsjs/runtime/runtime'
import type {contract, updates} from '../../../wailsjs/go/models'
import {checkInstalledModUpdates, updateInstalledMod} from '../../services/mod-updates'
import type {InstalledModUpdateTask} from '../../types/mod-update'
import type {InstallProgressPayload} from '../../types/mod'
import {formatDownloadSpeed, formatErrorMessage} from '../../utils/format'

// =============== 更新任务的页面间恢复 ===============
// 说明：旧卡片映射新任务 key；组件重建后保留映射，由后端快照补齐进度，不自动重放安装。
const updateTasks = reactive<Record<string, InstalledModUpdateTask>>({})
const packageIdentity = (key: string): string => key.split(':').slice(0, 3).join(':').toLowerCase()

// =============== 已安装市场模组与更新 ===============
// 说明：本地列表先显示，版本检查在后台完成；更新复用现有安装器，不先卸载旧版。
export const useInstalledModState = (onUpdated?: () => Promise<void>) => {
  const message = useMessage()
  const dialog = useDialog()
  const installedPackages = ref<contract.InstalledPackageSummary[]>([])
  const isLoadingInstalledPackages = ref(false)
  const uninstallingPackageKeys = ref<Record<string, boolean>>({})
  const selectedInstalledPackageKey = ref('')
  const packageUpdates = ref<Record<string, updates.PackageUpdate>>({})
  const isCheckingUpdates = ref(false)
  const hasActiveOperation = computed(() => Object.values(updateTasks).some((task) => (
    task.running || task.requesting || Boolean(task.conflict)
  )) || Object.values(uninstallingPackageKeys.value).some(Boolean))
  const selectedInstalledPackage = computed(() => (
    installedPackages.value.find((pack) => pack.packageKey === selectedInstalledPackageKey.value) ?? null
  ))
  let active = true
  let disposed = false
  let loading: Promise<void> | null = null
  let checking: Promise<void> | null = null
  let lastCheckedFingerprint = ''
  let lastCheckedAt = 0
  let reloadTimer: number | undefined
  let stopProgress: (() => void) | undefined
  const fingerprint = (): string => installedPackages.value.map((pack) => pack.packageKey).sort().join('|')

  // =============== 安装事件映射 ===============
  // 说明：按 targetKey 匹配更新；页面重载后由同一来源和模组身份恢复旧卡片映射。
  const applyProgress = (payload: InstallProgressPayload): void => {
    if (!payload?.packageKey || disposed) return
    let task = Object.values(updateTasks).find((item) => item.targetKey === payload.packageKey)
    if (!task && (payload.state === 'running' || payload.state === 'conflict')) {
      const pack = installedPackages.value.find((item) => item.source === 'thunderstore'
        && item.packageKey !== payload.packageKey && packageIdentity(item.packageKey) === packageIdentity(payload.packageKey))
      if (!pack) return
      updateTasks[pack.packageKey] = {
        packageKey: pack.packageKey, targetKey: payload.packageKey,
        running: false, requesting: false, cancelRequested: false,
        percent: 0, status: '', downloadSpeed: '', failureMessage: '',
      }
      task = updateTasks[pack.packageKey]
    }
    if (!task) return
    task.percent = Math.max(0, Math.min(100, Number.isFinite(payload.percent) ? payload.percent : 0))
    task.status = payload.status || payload.message || '正在更新'
    task.downloadSpeed = payload.stage === 'download' ? formatDownloadSpeed(payload.bytesPerSecond || 0) : ''
    task.running = !payload.state || payload.state === 'running'
    task.conflict = payload.state === 'conflict' ? payload.conflict : undefined
    task.failureMessage = payload.state === 'failed' || payload.state === 'conflict'
      ? payload.errorMessage || payload.conflict?.message || payload.message || '更新未完成' : ''
    if (task.cancelRequested && payload.state === 'failed') {
      task.failureMessage = ''
      task.status = '已取消更新'
    }
    if (payload.state === 'done' || payload.state === 'installed' || payload.state === 'uninstalled') {
      task.failureMessage = ''
      if (reloadTimer !== undefined) window.clearTimeout(reloadTimer)
      reloadTimer = window.setTimeout(() => { if (!disposed) void loadInstalledPackages() }, 100)
    }
  }

  // 说明：快照恢复只读，不依据远端最新版本改写正在运行的 targetKey。
  const restoreUpdateSnapshots = async (): Promise<void> => {
    try {
      const snapshots = await ListInstallProgressSnapshots()
      if (!disposed) snapshots.forEach((snapshot) => applyProgress(snapshot as InstallProgressPayload))
    } catch { /* 本地列表继续展示，下一次进入或实时事件再次补齐进度。 */ }
  }

  // =============== 后台检查更新 ===============
  // 说明：普通进入复用来源缓存和短期检查结果；手动检查绕过来源缓存，失败保留既有徽标。
  const checkUpdates = (forceRefresh = false, showFeedback = false): Promise<void> => {
    if (disposed) return Promise.resolve()
    if (checking) return forceRefresh ? checking.then(() => checkUpdates(true, showFeedback)) : checking
    const current = fingerprint()
    if (!forceRefresh && current === lastCheckedFingerprint && Date.now() - lastCheckedAt < 300_000) return Promise.resolve()
    if (!installedPackages.value.some((pack) => pack.source === 'thunderstore')) {
      packageUpdates.value = {}
      return Promise.resolve()
    }
    isCheckingUpdates.value = true
    checking = (async (): Promise<void> => {
      try {
        const results = await checkInstalledModUpdates(forceRefresh)
        if (disposed || fingerprint() !== current) return
        packageUpdates.value = Object.fromEntries(results.map((item) => [item.packageKey, item]))
        lastCheckedFingerprint = current
        lastCheckedAt = Date.now()
        if (showFeedback && active) message.success(results.length ? `发现 ${results.length} 个模组可更新` : '市场模组已是最新版本')
      } catch (error) {
        if (showFeedback && active && !disposed) message.error(`检查模组更新失败：${formatErrorMessage(error)}`)
      } finally {
        isCheckingUpdates.value = false
        checking = null
        if (!disposed && fingerprint() !== current) queueMicrotask(() => { if (!disposed) void checkUpdates() })
      }
    })()
    return checking
  }

  // 说明：本地记录先返回，不等待远端检查；成功后新 key 替换旧卡片，失败保留旧版。
  const loadInstalledPackages = (checkForUpdates = true): Promise<void> => {
    if (loading) return loading
    isLoadingInstalledPackages.value = true
    loading = (async (): Promise<void> => {
      try {
        const list = await ListInstalledPackages()
        if (disposed) return
        installedPackages.value = list
        const keys = new Set(list.map((pack) => pack.packageKey))
        packageUpdates.value = Object.fromEntries(Object.entries(packageUpdates.value).filter(([key]) => keys.has(key)))
        for (const [key, task] of Object.entries(updateTasks)) {
          if (!keys.has(key) && !task.requesting && !task.running && !task.conflict) delete updateTasks[key]
        }
        await restoreUpdateSnapshots()
        if (checkForUpdates && !hasActiveOperation.value) void checkUpdates()
      } catch (error) {
        if (active && !disposed) message.error(`读取已安装模组失败：${formatErrorMessage(error)}`)
      } finally {
        isLoadingInstalledPackages.value = false
        loading = null
      }
    })()
    return loading
  }

  const openInstalledPackage = (key: string): void => { selectedInstalledPackageKey.value = key }
  const closeInstalledPackage = (): void => { selectedInstalledPackageKey.value = '' }

  // =============== 更新执行与取消 ===============
  // 说明：点击徽标只启动一次升级；先建立旧卡片到新任务的映射，再调用后端。
  const executeUpdate = async (key: string, continuation = false): Promise<void> => {
    if (disposed || (!continuation && hasActiveOperation.value)) return
    const update = packageUpdates.value[key]
    const previous = updateTasks[key]
    const targetKey = continuation ? previous?.targetKey : update?.latestPackage.key
    if (!targetKey) return
    updateTasks[key] = {
      packageKey: key, targetKey, running: true, requesting: true, cancelRequested: false,
      percent: continuation ? previous?.percent ?? 0 : 0,
      status: continuation ? '正在继续更新' : '准备更新', downloadSpeed: '', failureMessage: '',
    }
    const task = updateTasks[key]
    try {
      const result = continuation ? await ContinueRemotePackageInstall(targetKey, 'overwrite') : await updateInstalledMod(key, targetKey)
      task.running = false
      task.percent = 100
      task.status = '更新完成'
      if (selectedInstalledPackageKey.value === key) selectedInstalledPackageKey.value = result.packageKey
      delete packageUpdates.value[key]
      if (!disposed) {
        await loadInstalledPackages()
        await onUpdated?.()
        if (active) message.success('模组更新完成')
      }
    } catch (error) {
      task.running = false
      task.downloadSpeed = ''
      if (task.cancelRequested) {
        task.failureMessage = ''
        task.status = '已取消更新'
        if (active && !disposed) message.warning('已取消模组更新')
      } else {
        const snapshots = await ListInstallProgressSnapshots().catch(() => [])
        if (!disposed) snapshots.forEach((snapshot) => applyProgress(snapshot as InstallProgressPayload))
        if (!task.conflict) {
          task.failureMessage = formatErrorMessage(error)
          task.status = '更新失败'
        }
        if (active && !disposed) message.error(`模组更新未完成：${formatErrorMessage(error)}`)
      }
    } finally {
      task.running = false
      task.requesting = false
      task.downloadSpeed = ''
      if (!disposed) {
        await loadInstalledPackages()
        if (!hasActiveOperation.value) void checkUpdates()
      }
    }
  }

  const updateInstalledPackage = async (key: string): Promise<void> => executeUpdate(key)
  const cancelPackageUpdate = async (key: string): Promise<void> => {
    const task = updateTasks[key]
    if (!task || !task.running || task.cancelRequested) return
    task.cancelRequested = true
    task.status = '正在取消更新'
    try { await CancelRemotePackageInstall(task.targetKey) }
    catch (error) {
      task.cancelRequested = false
      if (task.running && active && !disposed) message.error(`取消更新失败：${formatErrorMessage(error)}`)
    }
  }

  // 说明：异常冲突沿用安装器的明确覆盖选择，正常版本替换按既有策略自动完成。
  const repairPackageUpdate = (key: string): void => {
    const task = updateTasks[key]
    if (!task?.conflict || task.requesting) return
    dialog.warning({
      title: '继续模组更新', content: task.conflict.message || '目标文件存在冲突，是否覆盖并继续更新？',
      positiveText: '覆盖并继续', negativeText: '暂不处理',
      onPositiveClick: () => { void executeUpdate(key, true) },
    })
  }

  // =============== 卸载与生命周期 ===============
  const uninstallInstalledPackage = async (key: string): Promise<void> => {
    if (hasActiveOperation.value) { message.warning('请等待当前模组操作完成'); return }
    uninstallingPackageKeys.value[key] = true
    try {
      const result = await UninstallRemotePackage(key)
      if (active && !disposed) message.success(result.message || '模组已卸载')
      if (selectedInstalledPackageKey.value === key) closeInstalledPackage()
      await loadInstalledPackages()
    } catch (error) {
      if (active && !disposed) message.error(`模组卸载失败：${formatErrorMessage(error)}`)
      throw error
    } finally { uninstallingPackageKeys.value[key] = false }
  }

  onMounted(() => { stopProgress = EventsOn('modDependence:installProgress', (payload: InstallProgressPayload) => applyProgress(payload)) })
  onActivated(() => { active = true })
  onDeactivated(() => { active = false })
  onBeforeUnmount(() => {
    disposed = true
    stopProgress?.()
    if (reloadTimer !== undefined) window.clearTimeout(reloadTimer)
  })

  return {
    installedPackages, isLoadingInstalledPackages, uninstallingPackageKeys, selectedInstalledPackage,
    packageUpdates, isCheckingUpdates, hasActiveOperation, updateTasks,
    loadInstalledPackages, openInstalledPackage, closeInstalledPackage, uninstallInstalledPackage,
    checkUpdates, updateInstalledPackage, cancelPackageUpdate, repairPackageUpdate,
  }
}
