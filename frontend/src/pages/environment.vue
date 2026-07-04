<script setup lang="ts">
import {
  Archive,
  ChevronRight,
  Clock,
  FolderOpen,
  Layers,
  Package,
  PencilRuler,
  Search,
} from '@lucide/vue'
import type {DropdownOption} from 'naive-ui'
import {useMessage} from 'naive-ui'
import {computed, nextTick, onBeforeUnmount, onMounted, ref, watch} from 'vue'
import {useEnvironmentState} from '../composables/useEnvironmentState'
import {environment} from '../../wailsjs/go/models'
import {OnFileDrop, OnFileDropOff} from '../../wailsjs/runtime/runtime'
import BlueprintViewerPanel from '../components/blueprint/viewer-panel.vue'

definePage({
  meta: {
    layout: 'main',
    keepAlive: true,
    title: '资源',
  },
})

const {
  activeTab,
  mods,
  selectedModDetail,
  selectedBlueprintDetail,
  blueprintPage,
  saves,
  modSearch,
  blueprintSearch,
  saveSearch,
  currentBlueprintParentId,
  isLoadingMods,
  isLoadingModDetail,
  isLoadingBlueprintDetail,
  isLoadingBlueprints,
  isLoadingSaves,
  isRunningInitialSync,
  operatingEnvResourceIds,
  enabledModCount,
  disabledModCount,
  enabledBlueprintCount,
  saveCount,
  recoverInterruptedOperations,
  loadOverview,
  loadMods,
  loadModDetail,
  closeModDetail,
  loadBlueprintDetail,
  closeBlueprintDetail,
  loadBlueprints,
  loadSaves,
  enterBlueprintFolder,
  backToBlueprintRoot,
  toggleMod,
  importMod,
  selectAndImportModFiles,
  selectAndImportModDirectory,
  deleteMod,
  toggleBlueprint,
  createBlueprintFolder,
  selectAndImportBlueprintFiles,
  quickImportBlueprint,
  renameBlueprint,
  moveBlueprint,
  deleteBlueprint,
  selectAndExportBlueprint,
  renameSave,
  selectAndImportSaves,
  selectAndExportSave,
  deleteSave,
  updateClientMirrorFromGame,
  restoreGameFromClientMirror,
  unlinkSyncRecord,
  openModLibraryDir,
  openModRealDir,
  openBlueprintLibraryDir,
  openBlueprintRealDir,
  openSaveLibraryDir,
  openSaveDir,
  runInitialSync,
  syncEnvironmentResources,
  selectAndSetEnvironmentRootDirectory,
} = useEnvironmentState()

const message = useMessage()

// =============== 标签与状态展示 ===============
// 说明：维护环境页顶部标签和状态语义，供模板复用统一文案与颜色。
const tabs = [
  {id: 'mods', label: '模组', icon: Package},
  {id: 'blueprints', label: '蓝图', icon: Layers},
  {id: 'saves', label: '存档', icon: Archive},
] as const

const blueprintResources = computed(() => blueprintPage.value?.items ?? [])
const blueprintFolders = computed(() => blueprintResources.value.filter((blueprintResource) => blueprintResource.type === 'folder'))
const blueprintFiles = computed(() => blueprintResources.value.filter((blueprintResource) => blueprintResource.type === 'blueprint'))
const totalBlueprintFileCount = computed(() => {
  const totalFromPage = blueprintPage.value?.totalBlueprints ?? 0
  const folderCount = blueprintPage.value?.totalFolders ?? 0
  const totalWithoutFolders = Math.max(0, totalFromPage - folderCount)
  return Math.max(enabledBlueprintCount.value, blueprintFiles.value.length, totalWithoutFolders)
})
const activeTabSummary = computed(() => {
  if (activeTab.value === 'mods') {
    return `${enabledModCount.value} 启用 · ${disabledModCount.value} 禁用 · ${mods.value.length} 总计`
  }
  if (activeTab.value === 'blueprints') {
    return `${enabledBlueprintCount.value}/${totalBlueprintFileCount.value} 已启用 · ${blueprintPage.value?.totalFolders || 0} 文件夹`
  }
  return `${saveCount.value} 个存档`
})
const draggedBlueprintResourceId = ref('')
const blueprintDropTargetId = ref<string | null>(null)
const isDroppingBlueprint = ref(false)
const isModZipDragActive = ref(false)
const modZipDragDepth = ref(0)
const isImportingDroppedModZip = ref(false)
let environmentSearchTimer: ReturnType<typeof window.setTimeout> | null = null
onMounted(async () => {
  OnFileDrop((_, __, paths) => {
    void importDroppedModZipPaths(paths)
  }, false)
  await recoverInterruptedOperations()
  await loadOverview()
  await loadMods()
})

onBeforeUnmount(() => {
  OnFileDropOff()
})

watch(modSearch, () => {
  if (activeTab.value === 'mods') {
    scheduleEnvironmentSearch(loadMods)
  }
})

watch(blueprintSearch, () => {
  if (activeTab.value === 'blueprints') {
    scheduleEnvironmentSearch(loadBlueprints)
  }
})

watch(saveSearch, () => {
  if (activeTab.value === 'saves') {
    scheduleEnvironmentSearch(loadSaves)
  }
})

const scheduleEnvironmentSearch = (loader: () => Promise<void>): void => {
  if (environmentSearchTimer) {
    window.clearTimeout(environmentSearchTimer)
  }
  environmentSearchTimer = window.setTimeout(() => {
    void loader()
  }, 180)
}

// =============== 数据格式化 ===============
// 说明：把后端字节和时间数据转为紧凑 UI 文案。
// 边界：Wails time.Time 在前端可能是对象或空值，无法解析时返回占位文案。
const formatSize = (sizeBytes: number): string => {
  if (!Number.isFinite(sizeBytes) || sizeBytes <= 0) {
    return '0 KB'
  }
  if (sizeBytes >= 1024 * 1024) {
    return `${(sizeBytes / 1024 / 1024).toFixed(1)} MB`
  }
  return `${Math.max(1, Math.round(sizeBytes / 1024))} KB`
}

const formatDate = (value: unknown): string => {
  if (!value) {
    return '未知时间'
  }
  const date = new Date(String(value))
  if (Number.isNaN(date.getTime())) {
    return '未知时间'
  }
  return date.toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  })
}

const stateLabel = (state: string): string => {
  if (state === 'enabled') {
    return '已启用'
  }
  if (state === 'disabled') {
    return '已禁用'
  }
  if (state === 'conflict') {
    return '冲突'
  }
  if (state === 'missing') {
    return '不可用'
  }
  return '默认'
}

// =============== 模组压缩包拖入 ===============
// 说明：支持把本地 zip 压缩包拖入模组子页面，复用后端 ImportMod 解析和导入规则。
// 边界：真实文件路径只信任 Wails OnFileDrop 回调；DOM drop 只负责清理拖入蒙版，非 zip 路径不会提交给后端。

const hasDraggedFile = (event: DragEvent): boolean => {
  return Array.from(event.dataTransfer?.types ?? []).includes('Files')
}

const modZipPathsOnly = (paths: string[]): string[] => {
  return paths.filter((path) => path.toLowerCase().endsWith('.zip'))
}

const importDroppedModZipPaths = async (paths: string[]): Promise<void> => {
  if (activeTab.value !== 'mods') {
    return
  }
  if (isImportingDroppedModZip.value) {
    return
  }
  const zipPaths = modZipPathsOnly(paths)
  if (zipPaths.length === 0) {
    return
  }
  isImportingDroppedModZip.value = true
  try {
    await importMod(zipPaths)
  } finally {
    window.setTimeout(() => {
      isImportingDroppedModZip.value = false
    }, 300)
  }
}

const markModZipDrag = (event: DragEvent): void => {
  if (activeTab.value !== 'mods' || !hasDraggedFile(event)) {
    return
  }
  event.preventDefault()
  modZipDragDepth.value += 1
  isModZipDragActive.value = true
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = 'copy'
  }
}

const keepModZipDrag = (event: DragEvent): void => {
  if (activeTab.value !== 'mods' || !hasDraggedFile(event)) {
    return
  }
  event.preventDefault()
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = 'copy'
  }
}

const clearModZipDrag = (event: DragEvent): void => {
  if (activeTab.value !== 'mods' || !hasDraggedFile(event)) {
    return
  }
  event.preventDefault()
  modZipDragDepth.value = Math.max(0, modZipDragDepth.value - 1)
  if (modZipDragDepth.value === 0) {
    isModZipDragActive.value = false
  }
}

const dropModZipFiles = async (event: DragEvent): Promise<void> => {
  if (activeTab.value !== 'mods') {
    return
  }
  event.preventDefault()
  modZipDragDepth.value = 0
  isModZipDragActive.value = false
}

// =============== 蓝图拖拽移动 ===============
// 说明：维护蓝图卡片拖起、拖入文件夹和拖回当前目录的移动交互。
// 边界：页面只传递资源 ID 和目标父级 ID；循环移动、启用迁移和文件失败由后端校验并返回错误。
const startBlueprintDrag = (event: DragEvent, blueprintResource: environment.EnvironmentBlueprintItem): void => {
  draggedBlueprintResourceId.value = blueprintResource.id
  blueprintDropTargetId.value = null
  event.dataTransfer?.setData('text/plain', blueprintResource.id)
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
  }
}

const clearBlueprintDragState = (): void => {
  draggedBlueprintResourceId.value = ''
  blueprintDropTargetId.value = null
}

const markBlueprintDropTarget = (event: DragEvent, targetParentId: string): void => {
  if (!draggedBlueprintResourceId.value) {
    return
  }
  event.preventDefault()
  event.stopPropagation()
  blueprintDropTargetId.value = targetParentId
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = 'move'
  }
}

const clearBlueprintDropTarget = (event: DragEvent): void => {
  event.stopPropagation()
  blueprintDropTargetId.value = null
}

const dropBlueprintToParent = async (event: DragEvent, targetParentId: string): Promise<void> => {
  event.preventDefault()
  event.stopPropagation()
  if (isDroppingBlueprint.value) {
    return
  }
  if (!draggedBlueprintResourceId.value) {
    return
  }
  const draggingResourceId = event.dataTransfer?.getData('text/plain') || draggedBlueprintResourceId.value
  if (!draggingResourceId) {
    return
  }
  if (targetParentId === currentBlueprintParentId.value) {
    clearBlueprintDragState()
    return
  }
  isDroppingBlueprint.value = true
  draggedBlueprintResourceId.value = ''
  blueprintDropTargetId.value = null
  try {
    await moveBlueprint(draggingResourceId, targetParentId)
  } finally {
    isDroppingBlueprint.value = false
  }
}

// =============== 弹窗与右键模型 ===============
// 说明：集中维护环境页输入弹窗、确认弹窗和右键菜单目标，保持模组、蓝图、存档交互一致。
// 边界：页面只收集用户意图并调用 composable；真实文件写入、校验和回滚仍由后端服务负责。
type EnvContextMenuKind = 'mod-space' | 'mod' | 'blueprint-space' | 'blueprint-folder' | 'blueprint-file' | 'save-space' | 'save'
type EnvContextMenuAction =
  | 'mod-import'
  | 'mod-import-zip'
  | 'mod-sync'
  | 'mod-detail'
  | 'mod-open-dir'
  | 'mod-open-real-dir'
  | 'mod-update-client'
  | 'mod-restore-game'
  | 'mod-unlink-sync'
  | 'mod-delete'
  | 'blueprint-new-folder'
  | 'blueprint-import'
  | 'blueprint-quick-import'
  | 'blueprint-sync'
  | 'blueprint-detail'
  | 'blueprint-open-dir'
  | 'blueprint-open-real-dir'
  | 'blueprint-select-dir'
  | 'blueprint-enter-folder'
  | 'blueprint-rename'
  | 'blueprint-export'
  | 'blueprint-update-client'
  | 'blueprint-restore-game'
  | 'blueprint-unlink-sync'
  | 'blueprint-delete'
  | 'save-import'
  | 'save-sync'
  | 'save-open-library-dir'
  | 'save-select-dir'
  | 'save-rename'
  | 'save-export'
  | 'save-delete'
  | 'save-open-dir'

interface EnvContextMenuTarget {
  kind: EnvContextMenuKind
  mod?: environment.EnvironmentModItem
  blueprint?: environment.EnvironmentBlueprintItem
  save?: environment.EnvironmentSaveItem
}

interface EnvTextDialogState {
  visible: boolean
  title: string
  label: string
  value: string
  placeholder: string
  positiveText: string
  isSubmitting: boolean
  submit: ((value: string) => Promise<void>) | null
}

interface EnvQuickImportDialogState {
  visible: boolean
  name: string
  code: string
  isSubmitting: boolean
}

interface EnvConfirmDialogState {
  visible: boolean
  title: string
  content: string
  positiveText: string
  isDanger: boolean
  isSubmitting: boolean
  submit: (() => Promise<void>) | null
}

const envContextMenu = ref<{
  visible: boolean
  x: number
  y: number
  target: EnvContextMenuTarget | null
}>({
  visible: false,
  x: 0,
  y: 0,
  target: null,
})

const envTextDialog = ref<EnvTextDialogState>({
  visible: false,
  title: '',
  label: '',
  value: '',
  placeholder: '',
  positiveText: '保存',
  isSubmitting: false,
  submit: null,
})

const envQuickImportDialog = ref<EnvQuickImportDialogState>({
  visible: false,
  name: '',
  code: '',
  isSubmitting: false,
})

const envConfirmDialog = ref<EnvConfirmDialogState>({
  visible: false,
  title: '',
  content: '',
  positiveText: '确认',
  isDanger: false,
  isSubmitting: false,
  submit: null,
})

const selectedModDetailVisible = computed({
  get: () => selectedModDetail.value !== null,
  set: (visible: boolean) => {
    if (!visible) {
      closeModDetail()
    }
  },
})

const selectedBlueprintDetailVisible = computed({
  get: () => selectedBlueprintDetail.value !== null || isLoadingBlueprintDetail.value,
  set: (visible: boolean) => {
    if (!visible) {
      closeBlueprintDetail()
    }
  },
})
const selectedBlueprintDetailName = computed(() => selectedBlueprintDetail.value?.item.name || '未选择蓝图')
const selectedBlueprintBuildingCount = computed(() => (
  selectedBlueprintDetail.value?.parsed.summary.buildingCount ?? 0
))

const conflictMenuOptions = (prefix: 'mod' | 'blueprint'): DropdownOption[] => [
  {type: 'divider', key: `${prefix}-conflict-divider`},
  {label: '用游戏侧更新客户端镜像', key: `${prefix}-update-client`},
  {label: '用客户端镜像恢复游戏侧', key: `${prefix}-restore-game`},
  {label: '解除同步记录', key: `${prefix}-unlink-sync`},
]

const modMaintenanceMenuOptions = (): DropdownOption[] => [
  {type: 'divider', key: 'mod-state-divider'},
  {label: '同步模组', key: 'mod-sync'},
  {type: 'divider', key: 'mod-directory-divider'},
  {label: '打开资源库目录', key: 'mod-open-dir'},
  {label: '打开真实资源目录', key: 'mod-open-real-dir'},
]

const blueprintMaintenanceMenuOptions = (): DropdownOption[] => [
  {type: 'divider', key: 'blueprint-state-divider'},
  {label: '同步蓝图', key: 'blueprint-sync'},
  {type: 'divider', key: 'blueprint-directory-divider'},
  {label: '打开资源库目录', key: 'blueprint-open-dir'},
  {label: '打开真实资源目录', key: 'blueprint-open-real-dir'},
  {label: '选择蓝图目录', key: 'blueprint-select-dir'},
]

const saveMaintenanceMenuOptions = (): DropdownOption[] => [
  {type: 'divider', key: 'save-state-divider'},
  {label: '同步存档', key: 'save-sync'},
  {type: 'divider', key: 'save-directory-divider'},
  {label: '打开资源库目录', key: 'save-open-library-dir'},
  {label: '打开真实资源目录', key: 'save-open-dir'},
  {label: '选择存档目录', key: 'save-select-dir'},
]

const envContextMenuOptions = computed<DropdownOption[]>(() => {
  const target = envContextMenu.value.target
  if (!target) {
    return []
  }

  if (target.kind === 'mod-space') {
    return [
      {label: '导入 zip 压缩包', key: 'mod-import-zip'},
      {label: '导入模组目录', key: 'mod-import'},
      ...modMaintenanceMenuOptions(),
    ]
  }

  if (target.kind === 'mod') {
    return [
      {label: '查看详情', key: 'mod-detail'},
      ...(shouldShowSyncRepairMenu(target.mod?.state) ? conflictMenuOptions('mod') : []),
      ...modMaintenanceMenuOptions(),
      {type: 'divider', key: 'mod-danger-divider'},
      {label: '删除模组', key: 'mod-delete'},
    ]
  }

  if (target.kind === 'blueprint-space') {
    return [
      {label: '新建文件夹', key: 'blueprint-new-folder'},
      {label: '导入文件', key: 'blueprint-import'},
      {label: '快速导入', key: 'blueprint-quick-import'},
      ...blueprintMaintenanceMenuOptions(),
    ]
  }

  if (target.kind === 'blueprint-folder') {
    return [
      {label: '进入文件夹', key: 'blueprint-enter-folder'},
      {label: '重命名', key: 'blueprint-rename'},
      ...blueprintMaintenanceMenuOptions(),
      {type: 'divider', key: 'blueprint-folder-danger-divider'},
      {label: '删除文件夹', key: 'blueprint-delete'},
    ]
  }

  if (target.kind === 'blueprint-file') {
    return [
      {label: '查看详情', key: 'blueprint-detail'},
      {label: '重命名', key: 'blueprint-rename'},
      {label: '导出蓝图', key: 'blueprint-export'},
      ...(shouldShowSyncRepairMenu(target.blueprint?.state) ? conflictMenuOptions('blueprint') : []),
      ...blueprintMaintenanceMenuOptions(),
      {type: 'divider', key: 'blueprint-file-danger-divider'},
      {label: '删除蓝图', key: 'blueprint-delete'},
    ]
  }

  if (target.kind === 'save-space') {
    return [
      {label: '导入存档', key: 'save-import'},
      ...saveMaintenanceMenuOptions(),
    ]
  }

  return [
    {label: '重命名', key: 'save-rename'},
    {label: '导出存档', key: 'save-export'},
    ...saveMaintenanceMenuOptions(),
    {type: 'divider', key: 'save-danger-divider'},
    {label: '删除存档', key: 'save-delete'},
  ]
})

const isEnvResourceOperating = (resourceId: string): boolean => operatingEnvResourceIds.value.includes(resourceId)
const canToggleMod = (mod: environment.EnvironmentModItem): boolean => mod.supported && !isEnvResourceOperating(mod.id)
const canToggleBlueprint = (blueprint: environment.EnvironmentBlueprintItem): boolean => !isEnvResourceOperating(blueprint.id)
const canRenameSave = (save: environment.EnvironmentSaveItem): boolean => !isEnvResourceOperating(save.id)
const shouldShowSyncRepairMenu = (state?: string): boolean => state === 'conflict' || state === 'missing'
const isBlueprintDropTarget = (targetParentId: string): boolean => (
  Boolean(draggedBlueprintResourceId.value)
  && blueprintDropTargetId.value !== null
  && blueprintDropTargetId.value === targetParentId
)

const openEnvContextMenu = (event: MouseEvent, target: EnvContextMenuTarget): void => {
  event.preventDefault()
  event.stopPropagation()
  envContextMenu.value.visible = false
  envContextMenu.value.x = event.clientX
  envContextMenu.value.y = event.clientY
  envContextMenu.value.target = target
  void nextTick(() => {
    envContextMenu.value.visible = true
  })
}

const closeEnvContextMenu = (): void => {
  envContextMenu.value.visible = false
}

const openTextDialog = (config: Omit<EnvTextDialogState, 'visible' | 'isSubmitting'>): void => {
  envTextDialog.value = {
    ...config,
    visible: true,
    isSubmitting: false,
  }
}

const openConfirmDialog = (config: Omit<EnvConfirmDialogState, 'visible' | 'isSubmitting'>): void => {
  envConfirmDialog.value = {
    ...config,
    visible: true,
    isSubmitting: false,
  }
}

const openNewBlueprintFolderDialog = (): void => {
  openTextDialog({
    title: '新建文件夹',
    label: '文件夹名称',
    value: '新建文件夹',
    placeholder: '输入文件夹名称',
    positiveText: '创建',
    submit: async (value) => createBlueprintFolder(value),
  })
}

const openBlueprintRenameDialog = (resourceId: string, currentName: string): void => {
  openTextDialog({
    title: '重命名蓝图',
    label: '名称',
    value: currentName,
    placeholder: '输入新的蓝图名称',
    positiveText: '重命名',
    submit: async (value) => {
      if (value !== currentName) {
        await renameBlueprint(resourceId, value)
      }
    },
  })
}

const openBlueprintQuickImportDialog = (): void => {
  envQuickImportDialog.value = {
    visible: true,
    name: '',
    code: '',
    isSubmitting: false,
  }
}

const openSaveRenameDialog = (saveId: string, currentName: string): void => {
  openTextDialog({
    title: '重命名存档',
    label: '存档名称',
    value: currentName,
    placeholder: '输入新的存档名称',
    positiveText: '重命名',
    submit: async (value) => {
      if (value !== currentName) {
        await renameSave(saveId, value)
      }
    },
  })
}

const confirmTextDialog = async (): Promise<boolean> => {
  const value = envTextDialog.value.value.trim()
  if (!value) {
    message.warning('名称不能为空。')
    return false
  }
  const submit = envTextDialog.value.submit
  if (!submit) {
    envTextDialog.value.visible = false
    return true
  }
  envTextDialog.value.isSubmitting = true
  try {
    await submit(value)
    envTextDialog.value.visible = false
    return true
  } catch {
    return false
  } finally {
    envTextDialog.value.isSubmitting = false
  }
}

const confirmBlueprintQuickImport = async (): Promise<boolean> => {
  const name = envQuickImportDialog.value.name.trim()
  const code = envQuickImportDialog.value.code.trim()
  if (!name || !code) {
    message.warning('蓝图名称和编码都不能为空。')
    return false
  }
  envQuickImportDialog.value.isSubmitting = true
  try {
    await quickImportBlueprint(name, code)
    envQuickImportDialog.value.visible = false
    return true
  } catch {
    return false
  } finally {
    envQuickImportDialog.value.isSubmitting = false
  }
}

const confirmEnvDangerAction = async (): Promise<boolean> => {
  const submit = envConfirmDialog.value.submit
  if (!submit) {
    envConfirmDialog.value.visible = false
    return true
  }
  envConfirmDialog.value.isSubmitting = true
  try {
    await submit()
    envConfirmDialog.value.visible = false
    return true
  } catch {
    return false
  } finally {
    envConfirmDialog.value.isSubmitting = false
  }
}

const openModDeleteConfirm = (mod: environment.EnvironmentModItem): void => {
  openConfirmDialog({
    title: '删除模组',
    content: `删除模组资源“${mod.name}”？已启用时会先安全禁用。`,
    positiveText: '删除模组',
    isDanger: true,
    submit: async () => deleteMod(mod),
  })
}

const openBlueprintDeleteConfirm = (resourceId: string, name: string, type: string): void => {
  openConfirmDialog({
    title: type === 'folder' ? '删除文件夹' : '删除蓝图',
    content: `删除蓝图资源“${name}”？已启用蓝图会先安全禁用。`,
    positiveText: type === 'folder' ? '删除文件夹' : '删除蓝图',
    isDanger: true,
    submit: async () => deleteBlueprint(resourceId),
  })
}

const openSaveDeleteConfirm = (saveId: string, name: string): void => {
  openConfirmDialog({
    title: '删除存档',
    content: `删除存档“${name}”？文件会移入应用快照目录。`,
    positiveText: '删除存档',
    isDanger: true,
    submit: async () => deleteSave(saveId),
  })
}

const openUpdateClientMirrorConfirm = (itemId: string, kind: 'mod' | 'blueprint', name: string): void => {
  openConfirmDialog({
    title: '更新客户端镜像',
    content: `使用游戏侧文件更新“${name}”的客户端镜像？`,
    positiveText: '更新镜像',
    isDanger: false,
    submit: async () => updateClientMirrorFromGame(itemId, kind),
  })
}

const openRestoreGameConfirm = (itemId: string, kind: 'mod' | 'blueprint', name: string): void => {
  openConfirmDialog({
    title: '恢复游戏侧文件',
    content: `使用客户端镜像覆盖“${name}”的游戏侧文件？`,
    positiveText: '恢复游戏侧',
    isDanger: false,
    submit: async () => restoreGameFromClientMirror(itemId, kind),
  })
}

const openUnlinkSyncRecordConfirm = (itemId: string, kind: 'mod' | 'blueprint', name: string): void => {
  openConfirmDialog({
    title: '解除同步记录',
    content: `解除“${name}”的同步记录？客户端镜像和游戏侧文件都会保留，之后需要手动处理游戏侧文件。`,
    positiveText: '解除同步',
    isDanger: true,
    submit: async () => unlinkSyncRecord(itemId, kind),
  })
}

const handleEnvContextMenuSelect = async (key: string | number): Promise<void> => {
  const action = String(key) as EnvContextMenuAction
  const target = envContextMenu.value.target
  closeEnvContextMenu()
  if (!target) {
    return
  }

  if (action === 'mod-import') {
    await selectAndImportModDirectory()
    return
  }
  if (action === 'mod-import-zip') {
    await selectAndImportModFiles()
    return
  }
  if (action === 'mod-sync') {
    await syncEnvironmentResources('mod')
    return
  }
  if (action === 'mod-detail' && target.mod) {
    await loadModDetail(target.mod.id)
    return
  }
  if (action === 'mod-open-dir') {
    await openModLibraryDir()
    return
  }
  if (action === 'mod-open-real-dir') {
    await openModRealDir()
    return
  }
  if (action === 'mod-update-client' && target.mod) {
    openUpdateClientMirrorConfirm(target.mod.id, 'mod', target.mod.name)
    return
  }
  if (action === 'mod-restore-game' && target.mod) {
    openRestoreGameConfirm(target.mod.id, 'mod', target.mod.name)
    return
  }
  if (action === 'mod-unlink-sync' && target.mod) {
    openUnlinkSyncRecordConfirm(target.mod.id, 'mod', target.mod.name)
    return
  }
  if (action === 'mod-delete' && target.mod) {
    openModDeleteConfirm(target.mod)
    return
  }
  if (action === 'blueprint-new-folder') {
    openNewBlueprintFolderDialog()
    return
  }
  if (action === 'blueprint-import') {
    await selectAndImportBlueprintFiles()
    return
  }
  if (action === 'blueprint-quick-import') {
    openBlueprintQuickImportDialog()
    return
  }
  if (action === 'blueprint-sync') {
    await syncEnvironmentResources('blueprint')
    return
  }
  if (action === 'blueprint-detail' && target.blueprint) {
    await loadBlueprintDetail(target.blueprint.id)
    return
  }
  if (action === 'blueprint-open-dir') {
    await openBlueprintLibraryDir()
    return
  }
  if (action === 'blueprint-open-real-dir') {
    await openBlueprintRealDir()
    return
  }
  if (action === 'blueprint-select-dir') {
    await selectAndSetEnvironmentRootDirectory('blueprint')
    return
  }
  if (action === 'blueprint-enter-folder' && target.blueprint) {
    await enterBlueprintFolder(target.blueprint.id)
    return
  }
  if (action === 'blueprint-rename' && target.blueprint) {
    openBlueprintRenameDialog(target.blueprint.id, target.blueprint.name)
    return
  }
  if (action === 'blueprint-export' && target.blueprint) {
    await selectAndExportBlueprint(target.blueprint.id)
    return
  }
  if (action === 'blueprint-update-client' && target.blueprint) {
    openUpdateClientMirrorConfirm(target.blueprint.id, 'blueprint', target.blueprint.name)
    return
  }
  if (action === 'blueprint-restore-game' && target.blueprint) {
    openRestoreGameConfirm(target.blueprint.id, 'blueprint', target.blueprint.name)
    return
  }
  if (action === 'blueprint-unlink-sync' && target.blueprint) {
    openUnlinkSyncRecordConfirm(target.blueprint.id, 'blueprint', target.blueprint.name)
    return
  }
  if (action === 'blueprint-delete' && target.blueprint) {
    openBlueprintDeleteConfirm(target.blueprint.id, target.blueprint.name, target.blueprint.type)
    return
  }
  if (action === 'save-import') {
    await selectAndImportSaves()
    return
  }
  if (action === 'save-sync') {
    await syncEnvironmentResources('save')
    return
  }
  if (action === 'save-open-library-dir') {
    await openSaveLibraryDir()
    return
  }
  if (action === 'save-select-dir') {
    await selectAndSetEnvironmentRootDirectory('save')
    return
  }
  if (action === 'save-rename' && target.save) {
    openSaveRenameDialog(target.save.id, target.save.name)
    return
  }
  if (action === 'save-export' && target.save) {
    await selectAndExportSave(target.save.id)
    return
  }
  if (action === 'save-open-dir') {
    await openSaveDir()
    return
  }
  if (action === 'save-delete' && target.save) {
    openSaveDeleteConfirm(target.save.id, target.save.name)
  }
}
</script>

<template>
  <section class="environment-page flex h-full flex-col overflow-hidden">
    <div class="environment-tabs flex shrink-0 items-center border-b px-6">
      <button
        v-for="tab in tabs"
        :key="tab.id"
        type="button"
        class="environment-tab-button"
        :class="{active: activeTab === tab.id}"
        @click="activeTab = tab.id"
      >
        <component :is="tab.icon" :size="15" :stroke-width="1.8" />
        <span>{{ tab.label }}</span>
      </button>
      <span class="environment-tab-summary">{{ activeTabSummary }}</span>
    </div>

    <div class="min-h-0 flex-1 overflow-hidden">
      <!-- ~ 模组资源库 -->
      <!-- * 说明：平铺展示客户端 AppData/plugins 中的目录型模组和不支持项提示 -->
      <!-- * 交互：搜索过滤；左键执行启用/禁用，右键打开统一操作菜单 -->
      <!-- * 依赖：依赖 EnvironmentManagerService.ListMods 与 OpenModLibraryDir -->
      <!-- * 边界：不扫描远端市场，不复制到游戏目录，不支持裸 DLL 管理 -->
      <section v-if="activeTab === 'mods'" class="environment-tab-panel">
        <div class="environment-toolbar">
          <n-input
            v-model:value="modSearch"
            class="environment-search"
            size="small"
            placeholder="搜索模组"
          >
            <template #prefix>
              <Search :size="14" :stroke-width="1.8" class="text-zinc-400" />
            </template>
          </n-input>
        </div>

        <div
          class="environment-grid-scroll mod-drop-zone"
          @dragenter="markModZipDrag"
          @dragover="keepModZipDrag"
          @dragleave="clearModZipDrag"
          @drop="dropModZipFiles"
          @contextmenu="openEnvContextMenu($event, {kind: 'mod-space'})"
        >
          <div v-if="isModZipDragActive" class="mod-zip-drop-mask">
            <div>
              <Package :size="34" :stroke-width="1.7" />
              <strong>导入解析 Mod</strong>
              <span>松开后解析 zip 压缩包</span>
            </div>
          </div>
          <div v-if="!isLoadingMods && mods.length === 0" class="environment-empty">
            <Package :size="30" :stroke-width="1.7" />
            <p>
              资源库里还没有模组，点击
              <button type="button" :disabled="isRunningInitialSync" @click="runInitialSync('mod')">一键同步</button>
              纳管本机模组。
            </p>
          </div>
          <div v-else class="environment-grid">
            <button
              v-for="mod in mods"
              :key="mod.id"
              type="button"
              class="environment-resource-card"
              :class="[mod.state, {unsupported: !mod.supported}]"
              :title="mod.warning || mod.description || mod.name"
              :aria-disabled="!canToggleMod(mod)"
              @click="canToggleMod(mod) && toggleMod(mod)"
              @contextmenu="openEnvContextMenu($event, {kind: 'mod', mod})"
            >
              <n-spin :show="isEnvResourceOperating(mod.id)" size="small">
                <Package :size="30" :stroke-width="1.7" />
              </n-spin>
              <span class="resource-name">{{ mod.name }}</span>
              <span class="resource-meta">
                {{ mod.version || '未知版本' }}
                <em>
                  <i class="state-dot" :class="mod.state"></i>
                  {{ mod.supported ? stateLabel(mod.state) : '不支持' }}
                </em>
              </span>
            </button>
          </div>
        </div>
      </section>

      <!-- ~ 蓝图资源库 -->
      <!-- * 说明：展示客户端蓝图镜像目录树，文件夹优先，蓝图文件随后展示 -->
      <!-- * 交互：面包屑进入/返回目录、搜索过滤、左键启用禁用、右键打开操作菜单 -->
      <!-- * 依赖：依赖 EnvironmentManagerService.ListBlueprints 与当前 parentId -->
      <!-- * 边界：移动会直接提交迁移；目标冲突、复制失败或删除失败由后端阻止 -->
      <section v-else-if="activeTab === 'blueprints'" class="environment-tab-panel">
        <div class="environment-toolbar">
          <n-input
            v-model:value="blueprintSearch"
            class="environment-search"
            size="small"
            placeholder="搜索蓝图"
          >
            <template #prefix>
              <Search :size="14" :stroke-width="1.8" class="text-zinc-400" />
            </template>
          </n-input>

          <div class="environment-breadcrumb">
            <button
              type="button"
              :class="{'drop-target': isBlueprintDropTarget('')}"
              @dragover="markBlueprintDropTarget($event, '')"
              @dragleave="clearBlueprintDropTarget($event)"
              @drop="dropBlueprintToParent($event, '')"
              @click="backToBlueprintRoot"
            >
              蓝图根目录
            </button>
            <template v-for="crumb in blueprintPage?.breadcrumbs || []" :key="crumb.id">
              <ChevronRight :size="13" :stroke-width="1.8" />
              <button
                type="button"
                :class="{'drop-target': isBlueprintDropTarget(crumb.id)}"
                @dragover="markBlueprintDropTarget($event, crumb.id)"
                @dragleave="clearBlueprintDropTarget($event)"
                @drop="dropBlueprintToParent($event, crumb.id)"
                @click="enterBlueprintFolder(crumb.id)"
              >
                {{ crumb.name }}
              </button>
            </template>
          </div>
        </div>

        <div
          class="environment-grid-scroll"
          :class="{'blueprint-drop-active': isBlueprintDropTarget(currentBlueprintParentId)}"
          @dragover="markBlueprintDropTarget($event, currentBlueprintParentId)"
          @dragleave="clearBlueprintDropTarget($event)"
          @drop="dropBlueprintToParent($event, currentBlueprintParentId)"
          @contextmenu="openEnvContextMenu($event, {kind: 'blueprint-space'})"
        >
          <div v-if="!isLoadingBlueprints && blueprintResources.length === 0" class="environment-empty">
            <Layers :size="30" :stroke-width="1.7" />
            <p v-if="blueprintSearch">未找到匹配的蓝图。</p>
            <p v-else>
              这个蓝图文件夹是空的，点击
              <button type="button" :disabled="isRunningInitialSync" @click="runInitialSync('blueprint')">一键同步</button>
              纳管本机蓝图。
            </p>
          </div>
          <div v-else class="environment-grid">
            <button
              v-for="folder in blueprintFolders"
              :key="folder.id"
              type="button"
              class="environment-resource-card folder"
              :class="{
                'dragging': draggedBlueprintResourceId === folder.id,
                'drop-target': isBlueprintDropTarget(folder.id),
              }"
              draggable="true"
              @dragstart="startBlueprintDrag($event, folder)"
              @dragend="clearBlueprintDragState"
              @dragover="markBlueprintDropTarget($event, folder.id)"
              @dragleave="clearBlueprintDropTarget($event)"
              @drop="dropBlueprintToParent($event, folder.id)"
              @click="enterBlueprintFolder(folder.id)"
              @contextmenu="openEnvContextMenu($event, {kind: 'blueprint-folder', blueprint: folder})"
            >
              <FolderOpen :size="30" :stroke-width="1.7" />
              <span class="resource-name">{{ folder.name }}</span>
              <span class="resource-meta">{{ folder.childCount }} 个项目</span>
            </button>
            <button
              v-for="blueprint in blueprintFiles"
              :key="blueprint.id"
              type="button"
              class="environment-resource-card"
              :class="blueprint.state"
              draggable="true"
              :aria-disabled="!canToggleBlueprint(blueprint)"
              @dragstart="startBlueprintDrag($event, blueprint)"
              @dragend="clearBlueprintDragState"
              @click="canToggleBlueprint(blueprint) && toggleBlueprint(blueprint)"
              @contextmenu="openEnvContextMenu($event, {kind: 'blueprint-file', blueprint})"
            >
              <n-spin :show="isEnvResourceOperating(blueprint.id)" size="small">
                <PencilRuler :size="30" :stroke-width="1.7" />
              </n-spin>
              <span class="resource-name">{{ blueprint.name }}</span>
              <span class="resource-meta">
                {{ formatSize(blueprint.sizeBytes) }}
                <em>
                  <i class="state-dot" :class="blueprint.state"></i>
                  {{ stateLabel(blueprint.state) }}
                </em>
              </span>
            </button>
          </div>
        </div>
      </section>

      <!-- ~ 存档目录 -->
      <!-- * 说明：直接展示真实 SaveRoot 下的存档文件扫描结果 -->
      <!-- * 交互：搜索过滤；左键重命名，右键打开操作菜单 -->
      <!-- * 依赖：依赖 EnvironmentManagerService.ListSaves、OpenSaveLibraryDir 与 OpenSaveRealDir -->
      <!-- * 边界：存档不进入客户端虚拟目录，页面以真实目录当前状态为准 -->
      <section v-else class="environment-tab-panel">
        <div class="environment-toolbar">
          <n-input
            v-model:value="saveSearch"
            class="environment-search"
            size="small"
            placeholder="搜索存档"
          >
            <template #prefix>
              <Search :size="14" :stroke-width="1.8" class="text-zinc-400" />
            </template>
          </n-input>
        </div>

        <div class="environment-grid-scroll" @contextmenu="openEnvContextMenu($event, {kind: 'save-space'})">
          <div v-if="!isLoadingSaves && saves.length === 0" class="environment-empty">
            <Archive :size="30" :stroke-width="1.7" />
            <p>没有找到存档文件。</p>
          </div>
          <div v-else class="environment-grid">
            <button
              v-for="save in saves"
              :key="save.id"
              type="button"
              class="environment-resource-card save"
              :aria-disabled="!canRenameSave(save)"
              @click="canRenameSave(save) && openSaveRenameDialog(save.id, save.name)"
              @contextmenu="openEnvContextMenu($event, {kind: 'save', save})"
            >
              <n-spin :show="isEnvResourceOperating(save.id)" size="small">
                <Archive :size="30" :stroke-width="1.7" />
              </n-spin>
              <span class="resource-name">{{ save.name }}</span>
              <span class="resource-meta">
                {{ formatSize(save.sizeBytes) }}
                <em>
                  <Clock :size="11" :stroke-width="1.8" />
                  {{ formatDate(save.lastModifiedAt) }}
                </em>
              </span>
            </button>
          </div>
        </div>
      </section>
    </div>

    <!-- ~ 环境右键菜单 -->
    <!-- * 说明：统一承载模组、蓝图、存档卡片和空白区域的上下文操作 -->
    <!-- * 交互：右键时按鼠标坐标弹出，选择后关闭并分发到对应动作 -->
    <!-- * 依赖：依赖 envContextMenu 与 Naive UI dropdown 的 body 挂载能力 -->
    <!-- * 边界：菜单只收集用户意图，不直接修改文件系统 -->
    <n-dropdown
      trigger="manual"
      placement="bottom-start"
      :show="envContextMenu.visible"
      :x="envContextMenu.x"
      :y="envContextMenu.y"
      :options="envContextMenuOptions"
      :keyboard="false"
      @clickoutside="closeEnvContextMenu"
      @select="handleEnvContextMenuSelect"
    />

    <!-- ~ 单字段输入弹窗 -->
    <!-- * 说明：复用蓝图新建文件夹、蓝图重命名和存档重命名输入 -->
    <!-- * 交互：确认前校验空值，提交期间禁止关闭并显示加载态 -->
    <!-- * 依赖：依赖 envTextDialog.submit 分发具体后端操作 -->
    <!-- * 边界：弹窗不做路径拼接，非法字符与同名冲突由服务层继续校验 -->
    <n-modal
      v-model:show="envTextDialog.visible"
      preset="dialog"
      :title="envTextDialog.title"
      :positive-text="envTextDialog.positiveText"
      negative-text="取消"
      :positive-button-props="{type: 'primary', loading: envTextDialog.isSubmitting, disabled: envTextDialog.isSubmitting}"
      :negative-button-props="{disabled: envTextDialog.isSubmitting}"
      :mask-closable="!envTextDialog.isSubmitting"
      @positive-click="confirmTextDialog"
    >
      <label class="block pt-2 text-sm">
        <span class="mb-2 block font-medium text-zinc-950">{{ envTextDialog.label }}</span>
        <n-input
          v-model:value="envTextDialog.value"
          :placeholder="envTextDialog.placeholder"
          clearable
          @keyup.enter="confirmTextDialog"
        />
      </label>
    </n-modal>

    <!-- ~ 快速导入弹窗 -->
    <!-- * 说明：收集蓝图名称与蓝图编码，写入当前客户端蓝图文件夹 -->
    <!-- * 交互：确认前同时校验名称和编码；提交期间保持弹窗打开展示加载态 -->
    <!-- * 依赖：依赖 QuickImportBlueprint 后端接口生成真实蓝图文件 -->
    <!-- * 边界：不在前端解析编码内容，格式校验由服务层负责 -->
    <n-modal
      v-model:show="envQuickImportDialog.visible"
      preset="dialog"
      title="快速导入"
      positive-text="导入"
      negative-text="取消"
      :positive-button-props="{type: 'primary', loading: envQuickImportDialog.isSubmitting, disabled: envQuickImportDialog.isSubmitting}"
      :negative-button-props="{disabled: envQuickImportDialog.isSubmitting}"
      :mask-closable="!envQuickImportDialog.isSubmitting"
      @positive-click="confirmBlueprintQuickImport"
    >
      <div class="grid gap-4 pt-2 text-sm">
        <label class="block">
          <span class="mb-2 block font-medium text-zinc-950">蓝图名称</span>
          <n-input v-model:value="envQuickImportDialog.name" placeholder="输入蓝图名称" clearable />
        </label>
        <label class="block">
          <span class="mb-2 block font-medium text-zinc-950">蓝图编码</span>
          <n-input
            v-model:value="envQuickImportDialog.code"
            type="textarea"
            placeholder="粘贴蓝图编码"
            :autosize="{minRows: 5, maxRows: 9}"
          />
        </label>
      </div>
    </n-modal>

    <!-- ~ 危险操作确认弹窗 -->
    <!-- * 说明：统一确认模组、蓝图和存档删除等高风险动作 -->
    <!-- * 交互：按钮文案包含具体动作，提交期间禁用取消以避免重复操作 -->
    <!-- * 依赖：依赖 envConfirmDialog.submit 分发删除操作 -->
    <!-- * 边界：弹窗不伪造撤销能力，删除策略由后端事务和快照流程控制 -->
    <n-modal
      v-model:show="envConfirmDialog.visible"
      preset="dialog"
      :title="envConfirmDialog.title"
      :positive-text="envConfirmDialog.positiveText"
      negative-text="取消"
      :positive-button-props="{
        type: envConfirmDialog.isDanger ? 'error' : 'primary',
        loading: envConfirmDialog.isSubmitting,
        disabled: envConfirmDialog.isSubmitting,
      }"
      :negative-button-props="{disabled: envConfirmDialog.isSubmitting}"
      :mask-closable="!envConfirmDialog.isSubmitting"
      @positive-click="confirmEnvDangerAction"
    >
      <p class="pt-2 text-sm leading-6 text-zinc-600">{{ envConfirmDialog.content }}</p>
    </n-modal>

    <!-- ~ 蓝图详情抽屉 -->
    <!-- * 说明：在当前抽屉内嵌入模型预览块，按蓝图编码渲染 3D 模型 -->
    <!-- * 交互：右侧滑出后可在模型区拖拽与缩放；关闭后清空当前蓝图详情，避免展示旧数据 -->
    <!-- * 依赖：依赖 EnvironmentManagerService.GetBlueprintDetail 返回原始 BLUEPRINT 编码 -->
    <!-- * 边界：详情抽屉只做只读预览，不再渲染蓝图标题、版本、校验、描述和建筑组成信息 -->
    <n-drawer
      v-model:show="selectedBlueprintDetailVisible"
      width="50vw"
      placement="right"
      :trap-focus="false"
    >
      <n-drawer-content
        closable
        body-content-style="padding: 0; height: 100%; overflow: hidden;"
      >
        <template #header>
          <div class="blueprint-detail-header">
            <strong>蓝图详情</strong>
            <div v-if="selectedBlueprintDetail" class="blueprint-detail-header-meta">
              <span class="blueprint-detail-header-chip name" :title="selectedBlueprintDetailName">
                <em>名称</em>
                <strong>{{ selectedBlueprintDetailName }}</strong>
              </span>
              <span class="blueprint-detail-header-chip">
                <em>建筑</em>
                <strong>{{ selectedBlueprintBuildingCount }}</strong>
              </span>
            </div>
          </div>
        </template>
        <n-spin
          :show="isLoadingBlueprintDetail"
          size="small"
          class="blueprint-detail-spin"
          content-style="height: 100%;"
        >
          <div v-if="selectedBlueprintDetail" class="blueprint-detail-drawer">
            <BlueprintViewerPanel
              :code="selectedBlueprintDetail.code"
            />
          </div>
        </n-spin>
      </n-drawer-content>
    </n-drawer>

    <!-- ~ 模组详情弹窗 -->
    <!-- * 说明：展示当前扫描到的模组 manifest 详情，和右键“查看详情”保持一致入口 -->
    <!-- * 交互：只读查看，关闭后清空选中模组 -->
    <!-- * 依赖：依赖 GetModDetail 返回的 EnvironmentModDetail 字段 -->
    <!-- * 边界：详情只读展示 manifest，不检查远端依赖安装状态 -->
    <n-modal
      v-model:show="selectedModDetailVisible"
      preset="dialog"
      title="模组详情"
      positive-text="关闭"
    >
      <n-spin :show="isLoadingModDetail" size="small">
      <div v-if="selectedModDetail" class="grid gap-4 pt-2 text-sm">
        <div class="environment-detail-grid">
          <span>名称</span>
          <strong>{{ selectedModDetail.name }}</strong>
          <span>版本</span>
          <strong>{{ selectedModDetail.version || '未知版本' }}</strong>
          <span>作者</span>
          <strong>{{ selectedModDetail.author || '未知作者' }}</strong>
          <span>官网</span>
          <strong>{{ selectedModDetail.websiteUrl || '未提供' }}</strong>
          <span>路径</span>
          <strong>{{ selectedModDetail.libraryPath || '未记录' }}</strong>
          <span>说明</span>
          <strong>{{ selectedModDetail.warning || selectedModDetail.description || '暂无说明' }}</strong>
        </div>
        <div class="environment-detail-section">
          <span>依赖</span>
          <div v-if="selectedModDetail.dependencies.length > 0" class="environment-dependency-list">
            <span v-for="dependency in selectedModDetail.dependencies" :key="`${dependency.name}-${dependency.version}`">
              {{ dependency.name }}<em v-if="dependency.version"> · {{ dependency.version }}</em>
            </span>
          </div>
          <strong v-else>未声明依赖</strong>
        </div>
        <div class="environment-detail-section">
          <span>原始 Manifest</span>
          <n-code
            class="environment-manifest-code"
            :code="selectedModDetail.rawManifest || '{}'"
            language="json"
            show-line-numbers
          />
        </div>
      </div>
      </n-spin>
    </n-modal>
  </section>
</template>

<style scoped lang="scss">
.environment-page {
  --env-bg: var(--ds-bg-shell);
  --env-surface: var(--ds-bg-panel);
  --env-surface-raised: var(--ds-bg-panel-elevated);
  --env-border: var(--ds-border-default);
  --env-border-soft: var(--ds-border-subtle);
  --env-focus: var(--ds-border-strong);
  --env-text: var(--ds-text-primary);
  --env-text-secondary: var(--ds-text-secondary);
  --env-text-muted: var(--ds-text-muted);
  --env-text-disabled: var(--ds-text-disabled);
  --env-brand: var(--ds-brand);
  --env-info: var(--ds-info);
  --env-success: var(--ds-success);
  --env-warning: var(--ds-warning);
  --env-danger: var(--ds-error);
  --env-hover: var(--ds-hover-bg);
  --env-info-soft: color-mix(in srgb, var(--env-info) 12%, transparent);
  --env-success-soft: color-mix(in srgb, var(--env-success) 18%, transparent);
  --env-warning-soft: color-mix(in srgb, var(--env-warning) 18%, transparent);
  --env-danger-soft: color-mix(in srgb, var(--env-danger) 16%, transparent);
  background: var(--env-bg);
  color: var(--env-text);
}

.environment-tabs,
.environment-toolbar,
.environment-breadcrumb {
  border-color: var(--env-border-soft);
}

.environment-tabs {
  display: flex;
  min-height: 42px;
  align-items: center;
}

.environment-tab-button {
  display: inline-flex;
  height: 42px;
  align-items: center;
  gap: 6px;
  border-bottom: 2px solid transparent;
  padding: 0 16px;
  color: var(--env-text-muted);
  font-size: 0.875rem;
  transition:
    color 160ms ease,
    border-color 160ms ease;

  &.active,
  &:hover {
    border-bottom-color: var(--env-brand);
    color: var(--env-text);
  }
}

.environment-tab-summary {
  margin-left: auto;
  color: var(--env-text-muted);
  font-size: 0.8125rem;
  white-space: nowrap;
}

.environment-tab-panel {
  display: flex;
  height: 100%;
  min-height: 0;
  flex-direction: column;
  background: inherit;
}

.environment-toolbar {
  display: flex;
  min-height: 50px;
  flex-shrink: 0;
  align-items: center;
  gap: 8px;
  border-bottom-width: 1px;
  padding: 8px 16px;
}

.environment-search {
  width: min(260px, 34vw);
  --n-border: none !important;
  --n-border-hover: none !important;
  --n-border-focus: none !important;
  --n-box-shadow-focus: none !important;
  --n-color: transparent !important;
  --n-color-focus: transparent !important;
  --n-text-color: var(--env-text-secondary) !important;
  --n-caret-color: var(--env-brand) !important;
  --n-placeholder-color: var(--env-text-disabled) !important;

  :deep(.n-input),
  :deep(.n-input-wrapper) {
    background: transparent;
  }

  :deep(.n-input__border),
  :deep(.n-input__state-border) {
    display: none;
  }
}

.environment-breadcrumb {
  display: flex;
  min-width: 0;
  max-width: calc(100% - 280px);
  min-height: 30px;
  margin-left: auto;
  align-items: center;
  gap: 6px;
  color: var(--env-text-muted);
  font-size: 0.75rem;

  button {
    max-width: 14rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  button:hover {
    color: var(--env-text);
  }

  button.drop-target {
    background: var(--env-info-soft);
    color: var(--env-info);
  }
}

.environment-grid-scroll {
  position: relative;
  min-height: 0;
  flex: 1;
  overflow: auto;
  background: inherit;
  padding: 16px;

  &.blueprint-drop-active {
    background: color-mix(in srgb, var(--env-info) 8%, transparent);
  }
}

.mod-drop-zone {
  --wails-drop-target: drop;
}

.mod-zip-drop-mask {
  position: absolute;
  z-index: 5;
  inset: 16px;
  display: grid;
  place-items: center;
  border: 1px dashed color-mix(in srgb, var(--env-brand) 58%, var(--env-border));
  background: color-mix(in srgb, var(--env-bg) 82%, transparent);
  color: var(--env-text);
  pointer-events: none;

  div {
    display: grid;
    place-items: center;
    gap: 6px;
    padding: 18px 24px;
  }

  svg {
    color: var(--env-brand);
  }

  strong {
    font-size: 0.875rem;
    font-weight: 600;
  }

  span {
    color: var(--env-text-muted);
    font-size: 0.75rem;
  }
}

.environment-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
  gap: 12px;
}

.environment-resource-card {
  display: flex;
  aspect-ratio: 1;
  min-width: 0;
  flex-direction: column;
  justify-content: space-between;
  border: 2px solid var(--env-border);
  border-radius: 4px;
  background: var(--env-surface);
  padding: 14px 10px 10px;
  color: var(--env-text-muted);
  text-align: left;
  transition:
    border-color 160ms ease,
    background 160ms ease,
    color 160ms ease,
    transform 160ms ease;

  > svg {
    align-self: center;
    margin-top: auto;
    margin-bottom: auto;
  }

  &:hover {
    border-color: var(--env-focus);
    background: var(--env-hover);
    color: var(--env-text);
  }

  &:active {
    transform: translateY(1px);
  }

  &.dragging {
    opacity: 0.42;
  }

  &.drop-target {
    border-color: var(--env-info);
    background: var(--env-info-soft);
  }

  &[aria-disabled='true'] {
    cursor: not-allowed;
    opacity: 0.58;
  }

  &[aria-disabled='true']:active {
    transform: none;
  }

  &.enabled {
    border-color: color-mix(in srgb, var(--env-success) 78%, var(--env-border));
    background: color-mix(in srgb, var(--env-success-soft) 36%, var(--env-surface));
    color: var(--env-success);
  }

  &.disabled {
    border-color: color-mix(in srgb, var(--env-text-disabled) 72%, var(--env-border));
    color: var(--env-text-muted);
  }

  &.missing {
    border-color: color-mix(in srgb, var(--env-warning) 64%, var(--env-border));
    background: color-mix(in srgb, var(--env-warning-soft) 30%, var(--env-surface));
    color: var(--env-warning);
  }

  &.unsupported {
    border-color: color-mix(in srgb, var(--env-danger) 62%, var(--env-border));
    background: color-mix(in srgb, var(--env-danger-soft) 28%, var(--env-surface));
    color: var(--env-danger);
  }

  &.folder {
    border-color: color-mix(in srgb, var(--env-warning) 58%, var(--env-border));
    background: color-mix(in srgb, var(--env-warning-soft) 24%, var(--env-surface));
    color: var(--env-warning);
  }

  &.save {
    border-color: color-mix(in srgb, var(--env-info) 54%, var(--env-border));
    background: color-mix(in srgb, var(--env-info-soft) 22%, var(--env-surface));
    color: var(--env-info);
  }

}

.resource-name {
  display: block;
  overflow: hidden;
  color: var(--env-text);
  font-size: 0.75rem;
  line-height: 1.2;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.resource-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  color: var(--env-text-muted);
  font-size: 0.6875rem;
  line-height: 1.2;

  em {
    display: inline-flex;
    min-width: 0;
    align-items: center;
    gap: 4px;
    font-style: normal;
    white-space: nowrap;
  }
}

.environment-detail-grid {
  display: grid;
  grid-template-columns: 5rem minmax(0, 1fr);
  gap: 10px 12px;
  color: var(--env-text-muted);

  strong {
    min-width: 0;
    overflow-wrap: anywhere;
    color: var(--env-text);
    font-weight: 500;
  }
}

.environment-detail-section {
  display: grid;
  gap: 8px;
  color: var(--env-text-muted);

  > span {
    font-weight: 500;
    color: var(--env-text);
  }

  > strong {
    color: var(--env-text-muted);
    font-weight: 500;
  }

  .environment-manifest-code {
    display: block;
    max-height: 220px;
    overflow: auto;
    border: 1px solid var(--env-border-soft);
    background: var(--env-surface-raised);
    padding: 10px;
  }
}

.environment-dependency-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;

  span {
    border: 1px solid var(--env-border-soft);
    padding: 2px 6px;
    color: var(--env-text);
  }

  em {
    color: var(--env-text-muted);
    font-style: normal;
  }
}

.blueprint-detail-header {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: space-between;
  gap: 16px;

  > strong {
    flex: 0 0 auto;
    color: var(--env-text);
    font-size: 0.9375rem;
    font-weight: 650;
  }
}

.blueprint-detail-header-meta {
  display: flex;
  min-width: 0;
  max-width: min(28rem, 70%);
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
}

.blueprint-detail-header-chip {
  display: inline-flex;
  min-width: 0;
  height: 28px;
  align-items: center;
  gap: 6px;
  border: 1px solid var(--env-border-soft);
  background: color-mix(in srgb, var(--env-surface-raised) 86%, transparent);
  padding: 0 9px;
  color: var(--env-text);
  font-size: 0.75rem;
  line-height: 1;

  &.name {
    max-width: min(18rem, 58%);
  }

  em {
    flex: 0 0 auto;
    color: var(--env-text-muted);
    font-style: normal;
  }

  strong {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-weight: 650;
    font-variant-numeric: tabular-nums;
  }
}

.blueprint-detail-drawer {
  height: 100%;
  min-height: 360px;
  color: var(--env-text);
}

.blueprint-detail-spin {
  height: 100%;

  :deep(.n-spin-container) {
    height: 100%;
  }
}

.state-dot {
  display: inline-block;
  width: 7px;
  height: 7px;
  flex: 0 0 auto;
  border: 1px solid var(--env-border);
  border-radius: 999px;
  background: var(--env-text-disabled);

  &.enabled {
    border-color: var(--env-success);
    background: var(--env-success);
  }

  &.disabled {
    border-color: var(--env-text-disabled);
    background: var(--env-text-disabled);
  }

  &.missing {
    border-color: var(--env-warning);
    background: var(--env-warning);
  }

  &.unsupported,
  &.conflict {
    border-color: var(--env-danger);
    background: var(--env-danger);
  }

  &.default {
    background: transparent;
  }
}

.environment-empty {
  display: grid;
  min-height: 18rem;
  place-items: center;
  align-content: center;
  gap: 12px;
  border: 1px dashed var(--env-border);
  color: var(--env-text-muted);
  font-size: 0.875rem;

  p {
    max-width: 28rem;
    text-align: center;
    line-height: 1.7;
  }

  button {
    color: var(--env-brand);
    font-weight: 650;
    text-decoration: underline;
    text-underline-offset: 3px;

    &:hover {
      color: var(--env-info);
    }

    &:disabled {
      cursor: wait;
      opacity: 0.62;
    }
  }
}

@media (max-width: 860px) {
  .environment-toolbar {
    flex-wrap: wrap;
  }

  .environment-search {
    width: 100%;
  }

  .environment-breadcrumb {
    max-width: 100%;
    margin-left: 0;
  }

  .blueprint-detail-header {
    align-items: flex-start;
    flex-direction: column;
    gap: 8px;
  }

  .blueprint-detail-header-meta {
    max-width: 100%;
    justify-content: flex-start;
  }
}
</style>
