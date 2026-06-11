<script setup lang="ts">
import {AlertCircle, CheckCircle2, Database, HelpCircle, RefreshCw, Search} from '@lucide/vue'
import {NButton, NTooltip, createDiscreteApi, useMessage} from 'naive-ui'
import {computed, h, nextTick, onActivated, onBeforeUnmount, onMounted, ref, watch} from 'vue'
import {useRouter} from 'vue-router'
import {Install as InstallBepInEx, SearchAndLoadBepInEx} from '../../wailsjs/go/modDependence/BepInExService'
import {CancelRemotePackageInstall, ContinueRemotePackageInstall, InstallRemotePackage, ListInstalledPackageKeys, ListInstallProgressSnapshots, UninstallRemotePackage} from '../../wailsjs/go/modDependence/InstallerService'
import {GetRemotePackageList, RefreshRemotePackageList} from '../../wailsjs/go/modDependence/MarketplaceService'
import {modDependence, response} from '../../wailsjs/go/models'
import {EventsOn} from '../../wailsjs/runtime/runtime'
import {GetConfig} from '../../wailsjs/go/system/ConfigurationService'
import {CheckDysonSphereExe} from '../../wailsjs/go/system/EnvironmentService'
import {useWorkshopState} from '../composables/useWorkshopState'

definePage({
  meta: {
    layout: 'main',
    keepAlive: true,
    title: 'Mod',
  },
})

const pageSize = 20
const sourceOptions = [
  {label: '国内服务器', value: 'official'},
  {label: 'Thunderstore', value: 'thunderstore'},
  {label: '手动导入', value: 'manual'},
]

const router = useRouter()
const {
  gamePath,
} = useWorkshopState()

const selectedPackKey = ref('')
const selectedSource = ref('official')
const searchKeyword = ref('')
const dialogVisible = ref(false)
const isCheckingGamePath = ref(true)
const isCheckingBepInEx = ref(false)
const isInstallingBepInEx = ref(false)
const canOpenModPage = ref(false)
const isBepInExInstalled = ref(false)
const bepInExVersion = ref('')
const baseURL = ref('')
const packageList = ref<response.RemoteModPackageDTO[]>([])
const currentPage = ref(0)
const totalPackages = ref(0)
const reachedPackageEnd = ref(false)
const isLoadingPackages = ref(false)
const packageLoadError = ref('')
const loadMoreTarget = ref<HTMLElement | null>(null)
const installedPackageKeys = ref<Record<string, boolean>>({})
const installingPackageKeys = ref<Record<string, boolean>>({})
const failedPackageKeys = ref<Record<string, boolean>>({})
const packageProgress = ref<Record<string, number>>({})
const packageProgressStatus = ref<Record<string, string>>({})
const packageDownloadSpeeds = ref<Record<string, string>>({})
const packageFailureMessages = ref<Record<string, string>>({})
const packageConflicts = ref<Record<string, modDependence.InstallConflictDTO | undefined>>({})
const packageOperationVersions = ref<Record<string, number>>({})
const activeConflictDialogKeys = ref<Record<string, string>>({})
const lastHandledConflictDialogKeys = ref<Record<string, string>>({})
const {dialog} = createDiscreteApi(['dialog'])
const message = useMessage()

let loadMoreObserver: IntersectionObserver | null = null
let disposeInstallProgressListener: (() => void) | null = null
let searchRefreshTimer: number | undefined

type InstallConflictAction = 'overwrite' | 'skip' | 'fail'

interface InstallProgressPayload {
  packageKey: string
  stage: string
  status: string
  percent: number
  current: number
  total: number
  bytesPerSecond: number
  message: string
  state: string
  errorMessage: string
  conflict?: modDependence.InstallConflictDTO
}

const selectedPack = computed<response.RemoteModPackageDTO | undefined>(() => {
  return packageList.value.find((pack) => pack.key === selectedPackKey.value)
})
const hasLoadedAllPackages = computed(() => {
  return reachedPackageEnd.value || (
    totalPackages.value > 0 &&
    packageList.value.length >= totalPackages.value
  )
})
const isManualSource = computed(() => selectedSource.value === 'manual')
const canLoadMorePackages = computed(() => {
  return canOpenModPage.value && !isManualSource.value && !isLoadingPackages.value && !hasLoadedAllPackages.value
})
const canInstallPackages = computed(() => {
  return canOpenModPage.value && isBepInExInstalled.value
})
const selectedSourceLabel = computed(() => {
  return sourceOptions.find((item) => item.value === selectedSource.value)?.label || '选择来源'
})
const bepinExStatusTitle = computed(() => {
  if (isCheckingBepInEx.value) {
    return '正在检查 BepInEx'
  }
  if (isBepInExInstalled.value) {
    return 'BepInEx 已就绪'
  }

  return '需要优先安装 BepInEx'
})
const bepinExStatusDescription = computed(() => {
  if (isCheckingBepInEx.value) {
    return '正在确认当前游戏目录下的前置框架状态。'
  }
  if (isBepInExInstalled.value) {
    return `已检测到 BepInEx ${bepInExVersion.value || '未知版本'}，现在可以安装整合包。`
  }

  return '安装 Mod 整合包前需要先安装 BepInEx 前置框架。'
})

// =============== 整合包列表 ===============
// 说明：维护远端整合包分页数据，滚动到底部时追加下一页。
// 边界：接口失败时保留已加载数据，首屏失败展示重试入口。
const loadPackagePage = async (page: number, replace = false, showFeedback = false): Promise<void> => {
  if (isLoadingPackages.value) {
    return
  }
  if (isManualSource.value) {
    packageLoadError.value = ''
    packageList.value = []
    currentPage.value = 1
    totalPackages.value = 0
    reachedPackageEnd.value = true
    if (showFeedback) {
      message.info('暂时不支持手动导入')
    }
    return
  }

  isLoadingPackages.value = true
  packageLoadError.value = ''
  try {
    const keyword = searchKeyword.value.trim()
    const result = showFeedback && replace
      ? await RefreshRemotePackageList(selectedSource.value, page, pageSize, keyword)
      : await GetRemotePackageList(selectedSource.value, page, pageSize, keyword)
    const nextList = result.list ?? []

    if (replace) {
      packageList.value = nextList
    } else {
      const existingKeys = new Set(packageList.value.map((pack) => pack.key))
      packageList.value = [
        ...packageList.value,
        ...nextList.filter((pack) => !existingKeys.has(pack.key)),
      ]
    }

    currentPage.value = result.page || page
    totalPackages.value = result.total || 0
    reachedPackageEnd.value = (
      (result.total > 0 && packageList.value.length >= result.total) ||
      nextList.length < pageSize
    )
    if (showFeedback) {
      message.success(replace ? '整合包列表已刷新' : '已加载下一页整合包')
    }
    await syncInstalledPackageKeys()
  } catch (error) {
    packageLoadError.value = formatErrorMessage(error)
    if (showFeedback) {
      message.error(`整合包加载失败：${packageLoadError.value}`)
    }
  } finally {
    isLoadingPackages.value = false
  }
}

// =============== 安装状态标记 ===============
// 说明：按 packageKey 维护卡片安装中、失败、进度与文案状态。
// 边界：失败状态会保留在卡片上，直到用户重新安装、卸载或刷新来源列表。
const resetPackageOperationState = (key: string): void => {
  packageProgress.value[key] = 0
  packageProgressStatus.value[key] = ''
  packageDownloadSpeeds.value[key] = ''
  failedPackageKeys.value[key] = false
  packageFailureMessages.value[key] = ''
  packageConflicts.value[key] = undefined
}

const markPackageFailure = (key: string, error: unknown): void => {
  const errorMessage = formatErrorMessage(error)
  installedPackageKeys.value[key] = false
  installingPackageKeys.value[key] = false
  failedPackageKeys.value[key] = true
  packageProgress.value[key] = 100
  packageProgressStatus.value[key] = '安装失败'
  packageDownloadSpeeds.value[key] = ''
  packageFailureMessages.value[key] = errorMessage
}

const nextPackageOperationVersion = (key: string): number => {
  const nextVersion = (packageOperationVersions.value[key] || 0) + 1
  packageOperationVersions.value[key] = nextVersion
  return nextVersion
}

const isCurrentPackageOperation = (key: string, version: number): boolean => {
  return packageOperationVersions.value[key] === version
}

const applyProgressPayload = (payload: InstallProgressPayload): void => {
  if (!payload?.packageKey) {
    return
  }

  const key = payload.packageKey
  const state = payload.state || 'running'
  packageProgress.value[key] = Math.max(0, Math.min(100, payload.percent || 0))
  packageProgressStatus.value[key] = payload.status || payload.message || '处理中'
  packageDownloadSpeeds.value[key] = payload.stage === 'download' ? formatDownloadSpeed(payload.bytesPerSecond || 0) : ''

  if (state === 'failed') {
    installingPackageKeys.value[key] = false
    failedPackageKeys.value[key] = true
    packageDownloadSpeeds.value[key] = ''
    packageConflicts.value[key] = undefined
    packageFailureMessages.value[key] = payload.errorMessage || payload.message || payload.status || '操作失败'
    return
  }

  if (state === 'conflict') {
    installingPackageKeys.value[key] = false
    failedPackageKeys.value[key] = true
    packageDownloadSpeeds.value[key] = ''
    packageConflicts.value[key] = payload.conflict
    packageFailureMessages.value[key] = payload.conflict?.message || payload.errorMessage || payload.message || payload.status || '等待处理安装冲突'
    packageProgressStatus.value[key] = '等待处理冲突'
    return
  }

  if (state === 'done' || state === 'installed') {
    installingPackageKeys.value[key] = false
    failedPackageKeys.value[key] = false
    packageDownloadSpeeds.value[key] = ''
    packageFailureMessages.value[key] = ''
    packageConflicts.value[key] = undefined
    return
  }

  if (state === 'uninstalled') {
    installingPackageKeys.value[key] = false
    installedPackageKeys.value[key] = false
    failedPackageKeys.value[key] = false
    packageDownloadSpeeds.value[key] = ''
    packageFailureMessages.value[key] = ''
    packageConflicts.value[key] = undefined
    return
  }

  installingPackageKeys.value[key] = true
  failedPackageKeys.value[key] = false
  packageConflicts.value[key] = undefined
  packageFailureMessages.value[key] = ''
}

const restoreInstallProgressSnapshots = async (): Promise<void> => {
  try {
    const snapshots = await ListInstallProgressSnapshots()
    snapshots.forEach((snapshot) => applyProgressPayload(snapshot as InstallProgressPayload))
  } catch (error) {
    message.warning(`安装进度恢复失败：${formatErrorMessage(error)}`)
  }
}

const applyInstallConflict = (key: string, conflict?: modDependence.InstallConflictDTO): void => {
  if (!conflict) {
    return
  }
  installingPackageKeys.value[key] = false
  failedPackageKeys.value[key] = true
  packageConflicts.value[key] = conflict
  packageProgress.value[key] = 100
  packageProgressStatus.value[key] = '等待处理冲突'
  packageFailureMessages.value[key] = conflict.message || '存在文件版本冲突，需要选择处理方式。'
}

const refreshPackages = async (showFeedback = true): Promise<void> => {
  await loadPackagePage(1, true, showFeedback)
  await nextTick()
  observeLoadMoreTarget()
}

// =============== 整合包搜索 ===============
// 说明：搜索框只改变远端列表查询条件，实际匹配由后端按名称、作者、命名空间和描述过滤。
// 边界：手动导入来源暂不支持数据源，输入变化仍只刷新为空态，不触发导入流程。
const scheduleSearchRefresh = (): void => {
  if (searchRefreshTimer !== undefined) {
    window.clearTimeout(searchRefreshTimer)
  }
  searchRefreshTimer = window.setTimeout(() => {
    void refreshPackages(false)
  }, 300)
}

const loadNextPackagePage = async (): Promise<void> => {
  if (!canLoadMorePackages.value) {
    return
  }

  await loadPackagePage(currentPage.value + 1, false, Boolean(packageLoadError.value))
}

// =============== 本地安装状态 ===============
// 说明：从本地 SQLite 读取已安装整合包 ID，并映射到市场卡片状态。
// 边界：状态读取失败不阻止市场列表展示，只清空本地安装标记并给出轻提示。
const syncInstalledPackageKeys = async (): Promise<void> => {
  try {
    const keys = await ListInstalledPackageKeys()
    installedPackageKeys.value = keys.reduce<Record<string, boolean>>((result, key) => {
      result[key] = true
      return result
    }, {})
  } catch (error) {
    installedPackageKeys.value = {}
    message.warning(`本地安装状态读取失败：${formatErrorMessage(error)}`)
  }
}

// =============== 滚动加载 ===============
// 说明：用 IntersectionObserver 监听列表底部哨兵，进入可视区后请求下一页。
// 边界：页面被 KeepAlive 复用时会重复挂载观察器，因此每次绑定前先断开旧实例。
const observeLoadMoreTarget = (): void => {
  loadMoreObserver?.disconnect()
  if (!loadMoreTarget.value) {
    return
  }

  loadMoreObserver = new IntersectionObserver((entries) => {
    if (entries.some((entry) => entry.isIntersecting)) {
      loadNextPackagePage()
    }
  }, {
    root: null,
    rootMargin: '220px 0px',
    threshold: 0.01,
  })
  loadMoreObserver.observe(loadMoreTarget.value)
}

// =============== 整合包交互 ===============
// 说明：安装与卸载均调用后端安装器，并在完成后同步本地 SQLite 安装状态。
const openPack = (key: string): void => {
  selectedPackKey.value = key
  dialogVisible.value = true
}

const executeUninstall = async (key: string): Promise<void> => {
  installingPackageKeys.value[key] = true
  resetPackageOperationState(key)
  packageProgress.value[key] = 1
  packageProgressStatus.value[key] = '准备卸载'
  try {
    const result = await UninstallRemotePackage(key)
    installedPackageKeys.value[key] = false
    failedPackageKeys.value[key] = false
    packageProgress.value[key] = 100
    packageProgressStatus.value[key] = result.message || '卸载完成'
    message.success(result.message || '整合包已卸载')
    await syncInstalledPackageKeys()
  } catch (error) {
    failedPackageKeys.value[key] = true
    packageFailureMessages.value[key] = formatErrorMessage(error)
    packageProgressStatus.value[key] = '卸载失败'
    packageDownloadSpeeds.value[key] = ''
    packageProgress.value[key] = 100
    message.error(`整合包卸载失败：${formatErrorMessage(error)}`)
  } finally {
    installingPackageKeys.value[key] = false
  }
}

const handleUninstall = (key: string): void => {
  if (installingPackageKeys.value[key]) {
    return
  }

  if (selectedPackKey.value === key) {
    dialogVisible.value = false
  }

  dialog.warning({
    title: '卸载整合包',
    content: '将移除该整合包中不再被其它整合包引用的 Mod 插件目录，并删除本地安装记录。',
    positiveText: '卸载',
    negativeText: '取消',
    onPositiveClick: () => {
      void executeUninstall(key)
    },
  })
}

const cancelInstall = async (key: string): Promise<void> => {
  if (!installingPackageKeys.value[key]) {
    return
  }

  nextPackageOperationVersion(key)
  packageProgressStatus.value[key] = '正在取消安装'
  try {
    await CancelRemotePackageInstall(key)
    markPackageFailure(key, new Error('安装已取消'))
    message.warning('已取消安装。')
  } catch (error) {
    message.error(`取消安装失败：${formatErrorMessage(error)}`)
  }
}

const resolveConflictAction = async (key: string, action: InstallConflictAction): Promise<void> => {
  const operationVersion = nextPackageOperationVersion(key)
  installingPackageKeys.value[key] = true
  packageProgress.value[key] = 1
  packageProgressStatus.value[key] = action === 'skip' ? '跳过冲突并继续安装' : '处理冲突并继续安装'
  try {
    const result = await ContinueRemotePackageInstall(key, action)
    if (!isCurrentPackageOperation(key, operationVersion)) {
      return
    }
    installedPackageKeys.value[key] = true
    failedPackageKeys.value[key] = false
    packageConflicts.value[key] = undefined
    packageProgress.value[key] = 100
    packageProgressStatus.value[key] = result.message || '安装完成'
    message.success(result.message || '整合包安装完成')
    await syncInstalledPackageKeys()
  } catch (error) {
    if (!isCurrentPackageOperation(key, operationVersion)) {
      return
    }
    await restoreInstallProgressSnapshots()
    const currentConflict = packageConflicts.value[key]
    if (action !== 'fail' && currentConflict) {
      applyInstallConflict(key, currentConflict)
      openConflictDialog(key)
      message.warning('发现新的版本冲突，请继续选择处理方式。')
    } else {
      markPackageFailure(key, error)
    }
    message.error(`整合包安装未完成：${formatErrorMessage(error)}`)
  } finally {
    if (isCurrentPackageOperation(key, operationVersion)) {
      installingPackageKeys.value[key] = false
    }
  }
}

const openConflictDialog = (key: string, force = false): void => {
  const conflict = packageConflicts.value[key]
  if (!conflict) {
    message.warning('未找到需要处理的安装冲突。')
    return
  }
  const dialogKey = buildConflictDialogKey(conflict)
  if (force) {
    activeConflictDialogKeys.value[key] = ''
  }
  if (activeConflictDialogKeys.value[key] === dialogKey) {
    return
  }
  if (!force && lastHandledConflictDialogKeys.value[key] === dialogKey) {
    return
  }
  activeConflictDialogKeys.value[key] = dialogKey

  const renderConflictHelp = (content: string) => h(NTooltip, {
    trigger: 'hover',
  }, {
    trigger: () => h(HelpCircle, {
      class: 'ml-1 inline-block align-[-2px] text-zinc-400',
      size: 14,
      strokeWidth: 1.8,
    }),
    default: () => content,
  })
  const renderConflictRow = (label: string, value: string, help: string) => h('p', {}, [
    h('span', {class: 'font-medium text-zinc-700'}, `${label}：`),
    h('span', {}, value),
    renderConflictHelp(help),
  ])
  const clearActiveConflictDialog = (): void => {
    if (activeConflictDialogKeys.value[key] === dialogKey) {
      activeConflictDialogKeys.value[key] = ''
    }
  }
  let closeConflictDialog: (() => void) | undefined
  const handleConflictDecision = (action: InstallConflictAction): void => {
    lastHandledConflictDialogKeys.value[key] = dialogKey
    closeConflictDialog?.()
    clearActiveConflictDialog()
    void resolveConflictAction(key, action)
  }
  const conflictDialog = dialog.warning({
    title: '处理 Mod 版本冲突',
    content: () => h('div', {class: 'space-y-2 text-sm leading-6'}, [
      renderConflictRow('组件', conflict.componentName || conflict.componentKey, '发生冲突的 Mod 组件。'),
      renderConflictRow(
        '当前已存在版本',
        `${conflict.existingComponentName || conflict.existingPackageName || conflict.existingPackageKey || '本地文件'} ${conflict.existingComponentVersion || '未知版本'}`,
        '游戏目录里已经存在的文件版本，可能来自其它整合包、之前安装的 Mod，或手动放入的本地文件。',
      ),
      renderConflictRow(
        '本次将安装版本',
        `${conflict.componentName || conflict.componentKey} ${conflict.componentVersion || '未知版本'}`,
        '当前整合包准备写入游戏目录的版本。选择覆盖会用这个版本替换当前已存在文件。',
      ),
      h('p', {class: 'break-all text-xs text-zinc-500'}, `路径：${conflict.targetPath}`),
    ]),
    onClose: clearActiveConflictDialog,
    onAfterLeave: clearActiveConflictDialog,
    action: () => [
      h(NButton, {
        size: 'small',
        type: 'primary',
        onClick: () => handleConflictDecision('overwrite'),
      }, {default: () => '覆盖'}),
      h(NButton, {
        size: 'small',
        secondary: true,
        onClick: () => handleConflictDecision('skip'),
      }, {default: () => '跳过'}),
      h(NButton, {
        size: 'small',
        type: 'error',
        secondary: true,
        onClick: () => handleConflictDecision('fail'),
      }, {default: () => '失败'}),
    ],
  })
  closeConflictDialog = () => {
    clearActiveConflictDialog()
    conflictDialog.destroy()
  }
}

const buildConflictDialogKey = (conflict: modDependence.InstallConflictDTO): string => {
  return [
    conflict.componentKey || '',
    conflict.targetPath || '',
    conflict.incomingSha256 || '',
    conflict.existingSha256 || '',
  ].join('|')
}

const handleInstall = async (key: string): Promise<void> => {
  if (installingPackageKeys.value[key]) {
    return
  }
  if (!canInstallPackages.value) {
    message.warning('需要优先安装 BepInEx 后才能安装整合包。')
    return
  }

  const pack = packageList.value.find((item) => item.key === key)
  if (!pack) {
    message.error('未找到要安装的整合包。')
    return
  }

  installingPackageKeys.value[key] = true
  const operationVersion = nextPackageOperationVersion(key)
  resetPackageOperationState(key)
  packageProgress.value[key] = 1
  packageProgressStatus.value[key] = '准备安装'
  try {
    const result = await InstallRemotePackage(pack)
    if (!isCurrentPackageOperation(key, operationVersion)) {
      return
    }
    installedPackageKeys.value[key] = true
    packageProgress.value[key] = 100
    packageProgressStatus.value[key] = result.message || '安装完成'
    isBepInExInstalled.value = result.bepInExStatus.installed
    bepInExVersion.value = result.bepInExStatus.installedVersion
    message.success(result.message || '整合包安装完成')
    await syncInstalledPackageKeys()
  } catch (error) {
    if (!isCurrentPackageOperation(key, operationVersion)) {
      return
    }
    await restoreInstallProgressSnapshots()
    if (packageConflicts.value[key]) {
      openConflictDialog(key)
      message.warning('发现版本冲突，请选择处理方式。')
    } else {
      markPackageFailure(key, error)
      message.error(`整合包安装失败：${formatErrorMessage(error)}`)
    }
  } finally {
    if (isCurrentPackageOperation(key, operationVersion)) {
      installingPackageKeys.value[key] = false
    }
  }
}

// =============== 安装进度事件 ===============
// 说明：后端安装器通过 Wails runtime 事件推送下载、解压、复制与记录阶段进度。
// 边界：事件只更新正在操作的包；安装器最终结果仍以后端方法返回为准。
const bindInstallProgressListener = (): void => {
  disposeInstallProgressListener?.()
  disposeInstallProgressListener = EventsOn('modDependence:installProgress', (payload: InstallProgressPayload) => {
    applyProgressPayload(payload)
  })
}

// =============== 来源切换 ===============
// 说明：切换来源时重置分页与选中状态，统一使用 V2 key 映射安装状态。
const handleSourceChange = async (): Promise<void> => {
  selectedPackKey.value = ''
  dialogVisible.value = false
  packageList.value = []
  failedPackageKeys.value = {}
  packageFailureMessages.value = {}
  packageConflicts.value = {}
  activeConflictDialogKeys.value = {}
  lastHandledConflictDialogKeys.value = {}
  packageOperationVersions.value = {}
  packageProgress.value = {}
  packageProgressStatus.value = {}
  packageDownloadSpeeds.value = {}
  currentPage.value = 0
  totalPackages.value = 0
  reachedPackageEnd.value = false
  await refreshPackages(false)
  await restoreInstallProgressSnapshots()
}

// =============== BepInEx 前置安装 ===============
// 说明：在 Mod 页直接触发前置框架安装，成功后刷新页面安装许可状态。
// 边界：安装失败时保留市场浏览能力，但继续阻止整合包安装。
const installBepInExFromModPage = async (): Promise<void> => {
  if (isInstallingBepInEx.value || isCheckingBepInEx.value) {
    return
  }

  isInstallingBepInEx.value = true
  try {
    const result = await InstallBepInEx()
    isBepInExInstalled.value = result.status.installed
    bepInExVersion.value = result.status.installedVersion

    if (result.status.installed) {
      message.success(result.message || 'BepInEx 已安装')
    } else {
      message.warning('BepInEx 安装未完成，请稍后重试。')
    }
  } catch (error) {
    message.error(`BepInEx 安装失败：${formatErrorMessage(error)}`)
  } finally {
    isInstallingBepInEx.value = false
  }
}

// =============== 封面地址 ===============
// 说明：根据 System/BaseURL 与接口 coverUrl 拼出可直接访问的封面图片地址。
// 边界：任一端为空时返回空字符串；绝对地址直接使用接口值。
const resolveCoverUrl = (coverUrl?: string): string => {
  const trimmedCoverUrl = (coverUrl || '').trim()
  if (!trimmedCoverUrl) {
    return ''
  }
  if (/^https?:\/\//i.test(trimmedCoverUrl)) {
    return trimmedCoverUrl
  }

  const trimmedBaseURL = baseURL.value.trim()
  if (!trimmedBaseURL) {
    return ''
  }

  return `${trimmedBaseURL.replace(/\/+$/, '')}/${trimmedCoverUrl.replace(/^\/+/, '')}`
}

// =============== 游戏与前置框架检查 ===============
// 说明：沿用设置页保存的游戏路径，并在可打开 Mod 页时同步刷新 BepInEx 状态。
const refreshGamePathAvailability = async (): Promise<void> => {
  isCheckingGamePath.value = true
  try {
    baseURL.value = await GetConfig('System', 'BaseURL')
    const configuredPath = await GetConfig('Game', 'StartupFilePath')
    const nextGamePath = configuredPath || gamePath.value
    if (configuredPath) {
      gamePath.value = configuredPath
    }

    canOpenModPage.value = await CheckDysonSphereExe(nextGamePath)
    if (canOpenModPage.value) {
      await refreshBepInExAvailability(nextGamePath)
      await syncInstalledPackageKeys()
      if (packageList.value.length === 0 && !isLoadingPackages.value) {
        await refreshPackages(false)
      }
    } else {
      isBepInExInstalled.value = false
      bepInExVersion.value = ''
    }
  } catch {
    canOpenModPage.value = false
    isBepInExInstalled.value = false
    bepInExVersion.value = ''
  } finally {
    isCheckingGamePath.value = false
  }
}

const refreshBepInExAvailability = async (path: string = gamePath.value): Promise<void> => {
  isCheckingBepInEx.value = true
  try {
    const status = await SearchAndLoadBepInEx(path)
    isBepInExInstalled.value = status.installed
    bepInExVersion.value = status.installedVersion
  } catch {
    isBepInExInstalled.value = false
    bepInExVersion.value = ''
  } finally {
    isCheckingBepInEx.value = false
  }
}

const formatErrorMessage = (error: unknown): string => {
  if (error instanceof Error) {
    return error.message
  }

  return String(error || '未知错误')
}

const formatDownloadSpeed = (bytesPerSecond: number): string => {
  if (!Number.isFinite(bytesPerSecond) || bytesPerSecond <= 0) {
    return ''
  }

  const kbPerSecond = bytesPerSecond / 1024
  if (kbPerSecond < 1024) {
    return `${kbPerSecond.toFixed(kbPerSecond >= 100 ? 0 : 1)} KB/s`
  }

  return `${(kbPerSecond / 1024).toFixed(1)} MB/s`
}

onMounted(async () => {
  bindInstallProgressListener()
  await restoreInstallProgressSnapshots()
  await refreshGamePathAvailability()
  await nextTick()
  observeLoadMoreTarget()
})
onActivated(async () => {
  await restoreInstallProgressSnapshots()
  await refreshGamePathAvailability()
  await nextTick()
  observeLoadMoreTarget()
})
onBeforeUnmount(() => {
  loadMoreObserver?.disconnect()
  disposeInstallProgressListener?.()
  disposeInstallProgressListener = null
  if (searchRefreshTimer !== undefined) {
    window.clearTimeout(searchRefreshTimer)
  }
})

watch(searchKeyword, scheduleSearchRefresh)
</script>

<template>
  <section class="mx-auto max-w-6xl px-8 py-8">
    <div class="relative min-h-[30rem]">
      <!-- ~ 整合包工具栏 -->
      <!-- * 说明：同层展示 BepInEx 前置状态、整合包搜索、来源切换与刷新入口 -->
      <!-- * 交互：未安装 BepInEx 时可直接安装；搜索框按关键词刷新第一页，滚动到底部自动请求下一页 -->
      <!-- * 依赖：依赖 BepInExService.Install、MarketplaceService.GetPackageList 与分页状态 -->
      <!-- * 边界：游戏路径无效时整体虚化并阻止列表交互 -->
      <div
        class="mb-4 flex flex-col gap-3 md:flex-row md:items-center md:justify-between"
        :class="(isCheckingGamePath || !canOpenModPage) ? 'pointer-events-none select-none blur-sm' : ''"
      >
        <div
          v-if="canOpenModPage"
          class="flex h-[34px] min-w-0 flex-1 items-center justify-between gap-4 border px-3"
          :class="isBepInExInstalled ? 'border-emerald-200 bg-emerald-50 text-emerald-800' : 'border-amber-200 bg-amber-50 text-amber-800'"
        >
          <div class="flex h-full min-w-0 flex-1 items-center gap-2">
            <CheckCircle2
              v-if="isBepInExInstalled"
              :size="16"
              :stroke-width="1.9"
              class="shrink-0 text-emerald-700"
            />
            <AlertCircle
              v-else
              :size="16"
              :stroke-width="1.9"
              class="shrink-0 text-amber-700"
            />
            <p class="shrink-0 text-sm font-semibold leading-none">{{ bepinExStatusTitle }}</p>
            <p class="ml-auto min-w-0 truncate text-right text-xs leading-none opacity-80">{{ bepinExStatusDescription }}</p>
          </div>
          <n-button
            v-if="!isBepInExInstalled"
            class="shrink-0"
            type="primary"
            :loading="isInstallingBepInEx"
            :disabled="isCheckingBepInEx"
            @click="installBepInExFromModPage"
          >
            安装 BepInEx
          </n-button>
        </div>
        <!-- ~ 整合包搜索 -->
        <!-- * 说明：在 BepInEx 状态和渠道来源之间提供关键词搜索 -->
        <!-- * 交互：输入后自动刷新列表，清空后恢复当前来源的默认分页 -->
        <!-- * 依赖：依赖 searchKeyword 与 MarketplaceService 的 keyword 参数 -->
        <!-- * 边界：桌面宽度固定占工具栏 30%，不参与左右组件的空间分配 -->
        <div v-if="canOpenModPage" class="w-full md:basis-[30%] md:max-w-[30%] md:shrink-0">
          <n-input
            v-model:value="searchKeyword"
            class="h-[34px]"
            size="small"
            clearable
            placeholder="搜索 Mod"
            :disabled="isManualSource"
          >
            <template #prefix>
              <Search :size="15" :stroke-width="1.8" class="text-zinc-400" />
            </template>
          </n-input>
        </div>
        <div class="flex shrink-0 items-center gap-2 self-end md:self-auto">
          <n-popselect
            v-model:value="selectedSource"
            :options="sourceOptions"
            trigger="click"
            @update:value="handleSourceChange"
          >
            <n-button class="h-[34px] shrink-0" secondary>
              <template #icon>
                <Database :size="16" :stroke-width="1.8" />
              </template>
              {{ selectedSourceLabel }}
            </n-button>
          </n-popselect>
          <n-button
            class="h-[34px] shrink-0"
            secondary
            :loading="isLoadingPackages"
            :disabled="isManualSource"
            @click="() => refreshPackages()"
          >
            <template #icon>
              <RefreshCw :size="16" :stroke-width="1.8" />
            </template>
            刷新
          </n-button>
        </div>
      </div>

      <!-- ~ 整合包列表 -->
      <!-- * 说明：以卡片网格展示后端分页返回的 Mod 整合包 -->
      <!-- * 交互：点击卡片打开详情，底部哨兵进入视口后加载下一页 -->
      <!-- * 依赖：依赖 packageList、installedPackageKeys、installingPackageKeys 与后端安装器状态 -->
      <!-- * 边界：首屏加载、空列表和请求失败都有独立反馈，不遮挡游戏路径检查层 -->
      <div
        class="mb-8 grid grid-cols-2 gap-4 md:grid-cols-4"
        :class="(isCheckingGamePath || !canOpenModPage) ? 'pointer-events-none select-none blur-sm' : ''"
      >
        <ModPackCard
          v-for="pack in packageList"
          :key="pack.key"
          :package-key="pack.key"
          :title="pack.displayName || '未命名整合包'"
          :description="pack.description || '暂无整合包说明。'"
          :image-url="resolveCoverUrl(pack.coverUrl)"
          :file-count="pack.dependencyCount || pack.dependencies?.length || 0"
          :download-count="pack.downloadCount || 0"
          :author="pack.author || ''"
          :installed="installedPackageKeys[pack.key] || false"
          :installing="installingPackageKeys[pack.key] || false"
          :failed="failedPackageKeys[pack.key] || false"
          :conflicted="Boolean(packageConflicts[pack.key])"
          :progress="packageProgress[pack.key] || 0"
          :progress-status="packageProgressStatus[pack.key] || ''"
          :download-speed="packageDownloadSpeeds[pack.key] || ''"
          :failure-message="packageFailureMessages[pack.key] || ''"
          @open="openPack"
          @uninstall="handleUninstall"
          @repair="(key) => openConflictDialog(key, true)"
          @cancel="cancelInstall"
        />

        <template v-if="isLoadingPackages && packageList.length === 0">
          <div
            v-for="index in 8"
            :key="`skeleton-${index}`"
            class="aspect-square animate-pulse border border-zinc-200 bg-white"
          >
            <div class="h-24 border-b border-zinc-100 p-3">
              <div class="h-4 w-2/3 bg-zinc-100"></div>
              <div class="mt-3 h-3 w-full bg-zinc-100"></div>
              <div class="mt-2 h-3 w-4/5 bg-zinc-100"></div>
            </div>
            <div class="grid h-[calc(100%-6rem)] place-items-center bg-zinc-50">
              <div class="h-20 w-20 border border-zinc-200 bg-white"></div>
            </div>
          </div>
        </template>
      </div>

      <!-- ~ 列表状态 -->
      <!-- * 说明：承载空态、错误、加载更多与底部触发哨兵 -->
      <!-- * 交互：失败时允许重试；哨兵被观察到时自动加载下一页 -->
      <!-- * 依赖：依赖 packageLoadError、isLoadingPackages、hasLoadedAllPackages -->
      <!-- * 边界：已加载数据失败时只提示底部错误，不清空现有列表 -->
      <div
        class="pb-8"
        :class="(isCheckingGamePath || !canOpenModPage) ? 'pointer-events-none select-none blur-sm' : ''"
      >
        <div
          v-if="packageLoadError && packageList.length === 0"
          class="flex items-center justify-between gap-4 border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
        >
          <div class="flex min-w-0 items-center gap-2">
            <AlertCircle :size="16" :stroke-width="1.8" class="shrink-0" />
            <span class="truncate">{{ packageLoadError }}</span>
          </div>
          <n-button size="small" type="error" secondary @click="() => refreshPackages()">重试</n-button>
        </div>
        <div
          v-else-if="!isLoadingPackages && packageList.length === 0"
          class="border border-zinc-200 bg-white px-4 py-8 text-center text-sm text-zinc-500"
        >
          {{ isManualSource ? '暂时不支持手动导入' : '暂无可用整合包。' }}
        </div>
        <div v-else class="text-center text-xs text-zinc-500">
          <span v-if="isLoadingPackages">正在加载整合包...</span>
          <span v-else-if="packageLoadError">{{ packageLoadError }}</span>
          <span v-else-if="hasLoadedAllPackages">已经到底了。</span>
          <span v-else>继续向下滚动加载更多。</span>
        </div>
        <div ref="loadMoreTarget" class="h-8"></div>
      </div>

<!--      <p-->
<!--        class="text-xs text-zinc-500"-->
<!--        :class="(isCheckingGamePath || !canOpenModPage) ? 'pointer-events-none select-none blur-sm' : ''"-->
<!--      >-->
<!--        <span v-if="isCheckingBepInEx">正在检查 BepInEx 前置框架。</span>-->
<!--        <span v-else-if="isBepInExInstalled">已检测到 BepInEx {{ bepInExVersion || '未知版本' }}，可安装整合包。</span>-->
<!--        <span v-else>安装整合包前需要先安装 BepInEx 前置框架。</span>-->
<!--      </p>-->

      <!-- ~ 游戏路径遮罩 -->
      <!-- * 说明：游戏路径未准备好时阻止 Mod 市场操作 -->
      <!-- * 交互：路径缺失时提供跳转设置页入口 -->
      <!-- * 依赖：依赖 CheckDysonSphereExe 与 settings 页保存的 Game/StartupFilePath -->
      <!-- * 边界：检查失败按不可用处理，避免在错误目录下继续安装 -->
      <div v-if="isCheckingGamePath || !canOpenModPage" class="absolute inset-0 z-10 grid place-items-center bg-white/65 backdrop-blur-sm">
        <div class="max-w-md px-6 text-center">
          <h1 class="text-xl font-semibold text-zinc-950">{{ isCheckingGamePath ? '正在检测游戏路径' : '无法开启 Mod' }}</h1>
          <p class="mt-3 text-sm leading-6 text-zinc-500">
            {{ isCheckingGamePath ? '正在确认 DSPGAME.exe 是否存在。' : '未检测到 DSPGAME.exe，请正确选择《戴森球计划》游戏路径。' }}
          </p>
          <n-button v-if="!isCheckingGamePath" class="mt-6" type="primary" @click="router.push('/settings')">前往设置</n-button>
        </div>
      </div>
    </div>

    <!-- ~ 整合包详情弹窗 -->
    <!-- * 说明：展示当前选中整合包的版本、作者、来源与依赖明细 -->
    <!-- * 交互：点击安装后由后端安装器自动处理 BepInEx、下载、解压与本地记录 -->
    <!-- * 依赖：依赖 selectedPack、installedPackageKeys 与 installingPackageKeys -->
    <!-- * 边界：未选中整合包时弹窗内容为空，不触发安装事件 -->
    <ModPackDialog
      v-model:show="dialogVisible"
      :pack="selectedPack"
      :installed="selectedPack ? installedPackageKeys[selectedPack.key] || false : false"
      :installing="selectedPack ? installingPackageKeys[selectedPack.key] || false : false"
      :can-install="canInstallPackages"
      install-disabled-text="需要优先安装 BepInEx 后才能安装整合包。"
      @install="handleInstall"
      @uninstall="handleUninstall"
    />
  </section>
</template>
