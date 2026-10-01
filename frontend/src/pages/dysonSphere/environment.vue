<script setup lang="ts">
import {AlertCircle, Archive, ChevronRight, Clock, FolderOpen, Layers, Package, PencilRuler, RefreshCw, Search, Store} from '@lucide/vue'
import {useMessage} from 'naive-ui'
import {computed, onActivated, onBeforeUnmount, onDeactivated, onMounted, ref, watch} from 'vue'
import {useRouter} from 'vue-router'
import {contract, environment} from '../../../wailsjs/go/models'
import {OnFileDrop, OnFileDropOff} from '../../../wailsjs/runtime/runtime'
import BlueprintViewerPanel from '../../components/dysonSphere/blueprint/viewer-panel.vue'
import ResourceCard from '../../components/dysonSphere/environment/resource-card.vue'
import type {ResourceCardTone} from '../../types/environment'
import {useEnvironmentContextMenu} from '../../composables/dysonSphere/useEnvironmentContextMenu'
import {useEnvironmentDialogs} from '../../composables/dysonSphere/useEnvironmentDialogs'
import {useEnvironmentState} from '../../composables/dysonSphere/useEnvironmentState'
import {useInstalledModState} from '../../composables/dysonSphere/useInstalledModState'
import {getBepInExStatus} from '../../services/dysonSphere/game'
import {formatDate, formatFileSize} from '../../utils/format'

definePage({ meta: { layout: 'main', title: '资源', keepAlive: true } })

const router = useRouter()
const message = useMessage()
const env = useEnvironmentState()
const installed = useInstalledModState()
const dialogs = useEnvironmentDialogs()
const contextMenu = useEnvironmentContextMenu(env, dialogs, installed)

const {
  activeTab,
  overview,
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
  isRefreshingManagedState,
  operatingEnvResourceIds,
  enabledModCount,
  disabledModCount,
  enabledBlueprintCount,
} = env
const {installedPackages, isLoadingInstalledPackages, uninstallingPackageKeys, selectedInstalledPackage} = installed
const {textDialog, confirmDialog, quickImportDialog} = dialogs
const {menu: envContextMenu, options: envContextMenuOptions} = contextMenu

// =============== 标签与摘要 ===============
// 说明：顶部三标签与右侧摘要；模组摘要把市场安装的包计入启用与总计，和游戏内实际生效的模组一致。
const tabs = [
  {id: 'mods', label: '模组', icon: Package},
  {id: 'blueprints', label: '蓝图', icon: Layers},
  {id: 'saves', label: '存档', icon: Archive},
] as const

const blueprintItems = computed(() => blueprintPage.value?.items ?? [])
const blueprintFolders = computed(() => blueprintItems.value.filter((item) => item.type === 'folder'))
const blueprintFiles = computed(() => blueprintItems.value.filter((item) => item.type === 'blueprint'))

const filteredInstalledPackages = computed(() => {
  const keyword = modSearch.value.trim().toLowerCase()
  if (!keyword) {
    return installedPackages.value
  }
  return installedPackages.value.filter((pack) => (
    [pack.displayName, pack.name, pack.namespace, pack.author].some((field) => (field || '').toLowerCase().includes(keyword))
  ))
})

const activeTabSummary = computed(() => {
  if (activeTab.value === 'mods') {
    const enabled = enabledModCount.value + installedPackages.value.length
    const total = mods.value.length + installedPackages.value.length
    return `${enabled} 启用 · ${disabledModCount.value} 禁用 · ${total} 总计`
  }
  if (activeTab.value === 'blueprints') {
    const totalFolders = blueprintPage.value?.totalFolders ?? 0
    const totalBlueprints = blueprintPage.value?.totalBlueprints ?? 0
    const enabled = enabledBlueprintCount.value
    return `${enabled} 启用 · ${totalBlueprints - enabled} 未启用 · ${totalBlueprints} 蓝图 · ${totalFolders} 文件夹`
  }
  return `${saves.value.length} 个存档`
})

// =============== 前置条件提示 ===============
// 说明：资源库模组启用会写入游戏 BepInEx/plugins，游戏目录或 BepInEx 缺失时在模组标签顶部提示。
// 边界：只提示不拦截浏览；后端启用接口仍会独立校验目录。
const isBepInExReady = ref(true)
const hasGameDirectory = computed(() => Boolean(overview.value?.modRoot.path))
const modNotice = computed(() => {
  if (!overview.value) {
    return ''
  }
  if (!hasGameDirectory.value) {
    return '未配置有效的游戏目录，资源库模组暂时无法启用。'
  }
  if (!isBepInExReady.value) {
    return '未检测到 BepInEx，启用的模组不会被游戏加载。'
  }
  return ''
})

const refreshBepInExReady = async (): Promise<void> => {
  if (!hasGameDirectory.value) {
    isBepInExReady.value = false
    return
  }
  try {
    isBepInExReady.value = (await getBepInExStatus()).installed
  } catch {
    isBepInExReady.value = false
  }
}

// =============== 卡片状态 ===============
// 说明：把后端状态映射为卡片色调和状态文案。
const stateLabels: Record<string, string> = {
  enabled: '已启用',
  disabled: '已禁用',
  conflict: '冲突',
  missing: '不可用',
}
const stateLabel = (state: string): string => stateLabels[state] || '未启用'

const stateTone = (state: string): ResourceCardTone => {
  if (state === 'enabled' || state === 'disabled' || state === 'missing' || state === 'conflict') {
    return state
  }
  return 'default'
}

const isOperating = (resourceId: string): boolean => operatingEnvResourceIds.value.includes(resourceId)
const canToggleMod = (mod: environment.EnvironmentModItem): boolean => mod.supported && !mod.installerManaged && !isOperating(mod.id)
const modTone = (mod: environment.EnvironmentModItem): ResourceCardTone => {
  if (!mod.supported) {
    return 'unsupported'
  }
  return mod.installerManaged ? 'market' : stateTone(mod.state)
}
const modStatus = (mod: environment.EnvironmentModItem): string => {
  if (!mod.supported) {
    return '不支持'
  }
  return mod.installerManaged ? '市场管理' : stateLabel(mod.state)
}

const packTitle = (pack: contract.InstalledPackageSummary): string => pack.displayName || pack.name || pack.packageKey

// =============== 卡片点击 ===============
// 说明：资源库模组和蓝图左键切换启用；市场模组打开详情；存档打开重命名。
// 边界：业务错误已由组合函数提示，这里只吞掉重复的 Promise 拒绝。
const ignoreReported = (): void => undefined

const handleModClick = (mod: environment.EnvironmentModItem): void => {
  if (mod.installerManaged) {
    message.info('该模组的游戏侧文件由模组市场管理，请在下方“模组市场”分区卸载。')
    return
  }
  if (!canToggleMod(mod)) {
    return
  }
  void env.toggleMod(mod).catch(ignoreReported)
}

const handleBlueprintClick = (blueprint: environment.EnvironmentBlueprintItem): void => {
  if (isOperating(blueprint.id)) {
    return
  }
  void env.toggleBlueprint(blueprint).catch(ignoreReported)
}

const handleSaveClick = (save: environment.EnvironmentSaveItem): void => {
  if (!isOperating(save.id)) {
    contextMenu.openSaveRename(save)
  }
}

const handleContextMenuSelect = (key: string | number): void => {
  void contextMenu.handleSelect(key).catch(ignoreReported)
}

// =============== 搜索 ===============
// 说明：资源库与蓝图、存档搜索走后端过滤；市场模组按本地列表过滤。
let searchTimer: number | undefined
const scheduleSearch = (loader: () => Promise<void>): void => {
  if (searchTimer !== undefined) {
    window.clearTimeout(searchTimer)
  }
  searchTimer = window.setTimeout(() => {
    void loader()
  }, 180)
}

// =============== 工具栏同步 ===============
// 说明：搜索栏右侧的同步按钮；模组、蓝图先纳管游戏侧新增资源再校验状态，存档重新扫描真实目录。
// 边界：同步进行中禁用按钮，避免重复提交；模组同步后额外刷新市场安装列表。
const syncKindByTab = {mods: 'mod', blueprints: 'blueprint', saves: 'save'} as const
const syncLabelByTab = {mods: '同步本机模组', blueprints: '同步本机蓝图', saves: '重新扫描存档'} as const

const isSyncingActiveTab = computed(() => (
  activeTab.value === 'saves'
    ? isLoadingSaves.value
    : isRunningInitialSync.value || isRefreshingManagedState.value
))
const syncButtonLabel = computed(() => syncLabelByTab[activeTab.value])

const syncActiveTab = async (): Promise<void> => {
  if (isSyncingActiveTab.value) {
    return
  }
  const tab = activeTab.value
  await env.syncEnvironmentResources(syncKindByTab[tab])
  if (tab === 'mods') {
    await installed.loadInstalledPackages()
  }
}

watch(modSearch, () => activeTab.value === 'mods' && scheduleSearch(env.loadMods))
watch(blueprintSearch, () => activeTab.value === 'blueprints' && scheduleSearch(env.loadBlueprints))
watch(saveSearch, () => activeTab.value === 'saves' && scheduleSearch(env.loadSaves))

// =============== 模组压缩包拖入 ===============
// 说明：拖入 zip 时显示蒙版，真实路径只信任 Wails OnFileDrop 回调，并复用后端 ImportMod 规则。
// 边界：页面被 KeepAlive 缓存或不在模组标签时忽略拖入，非 zip 路径不会提交给后端。
const isPageActive = ref(false)
const isModDragActive = ref(false)
const isImportingDroppedMod = ref(false)
let modDragDepth = 0

const hasDraggedFile = (event: DragEvent): boolean => Array.from(event.dataTransfer?.types ?? []).includes('Files')

const importDroppedModPaths = async (paths: string[]): Promise<void> => {
  if (!isPageActive.value || activeTab.value !== 'mods' || isImportingDroppedMod.value) {
    return
  }
  const zipPaths = paths.filter((path) => path.toLowerCase().endsWith('.zip'))
  if (zipPaths.length === 0) {
    message.warning('只支持拖入 zip 压缩包导入模组。')
    return
  }
  isImportingDroppedMod.value = true
  try {
    await env.importMod(zipPaths)
  } catch {
    // 导入失败已由 importMod 提示
  } finally {
    isImportingDroppedMod.value = false
  }
}

const handleModDragEnter = (event: DragEvent): void => {
  if (!hasDraggedFile(event)) {
    return
  }
  event.preventDefault()
  modDragDepth += 1
  isModDragActive.value = true
}

const handleModDragOver = (event: DragEvent): void => {
  if (!hasDraggedFile(event)) {
    return
  }
  event.preventDefault()
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = 'copy'
  }
}

const handleModDragLeave = (event: DragEvent): void => {
  if (!hasDraggedFile(event)) {
    return
  }
  modDragDepth = Math.max(0, modDragDepth - 1)
  isModDragActive.value = modDragDepth > 0
}

const handleModDrop = (event: DragEvent): void => {
  event.preventDefault()
  modDragDepth = 0
  isModDragActive.value = false
}

// =============== 蓝图拖拽移动 ===============
// 说明：卡片可拖入文件夹或面包屑，页面只传递资源 ID 和目标父级 ID。
// 边界：循环移动、启用迁移和文件失败由后端校验并返回错误。
const draggedBlueprintId = ref('')
const blueprintDropTargetId = ref<string | null>(null)
const isMovingBlueprint = ref(false)

const startBlueprintDrag = (event: DragEvent, item: environment.EnvironmentBlueprintItem): void => {
  draggedBlueprintId.value = item.id
  blueprintDropTargetId.value = null
  event.dataTransfer?.setData('text/plain', item.id)
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
  }
}

const clearBlueprintDrag = (): void => {
  draggedBlueprintId.value = ''
  blueprintDropTargetId.value = null
}

const markBlueprintDropTarget = (event: DragEvent, targetParentId: string): void => {
  if (!draggedBlueprintId.value || draggedBlueprintId.value === targetParentId) {
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

const isBlueprintDropTarget = (targetParentId: string): boolean => (
  Boolean(draggedBlueprintId.value) && blueprintDropTargetId.value === targetParentId
)

const dropBlueprint = async (event: DragEvent, targetParentId: string): Promise<void> => {
  event.preventDefault()
  event.stopPropagation()
  const itemId = draggedBlueprintId.value
  clearBlueprintDrag()
  if (!itemId || isMovingBlueprint.value || itemId === targetParentId || targetParentId === currentBlueprintParentId.value) {
    return
  }
  isMovingBlueprint.value = true
  try {
    await env.moveBlueprint(itemId, targetParentId)
  } catch {
    // 移动失败已由 moveBlueprint 提示
  } finally {
    isMovingBlueprint.value = false
  }
}

// =============== 详情弹窗 ===============
const modDetailVisible = computed({
  get: () => selectedModDetail.value !== null,
  set: (visible: boolean) => {
    if (!visible) {
      env.closeModDetail()
    }
  },
})

const packDetailVisible = computed({
  get: () => selectedInstalledPackage.value !== null,
  set: (visible: boolean) => {
    if (!visible) {
      installed.closeInstalledPackage()
    }
  },
})

const blueprintDetailVisible = computed({
  get: () => selectedBlueprintDetail.value !== null || isLoadingBlueprintDetail.value,
  set: (visible: boolean) => {
    if (!visible) {
      env.closeBlueprintDetail()
    }
  },
})

const uninstallSelectedPackage = (): void => {
  const pack = selectedInstalledPackage.value
  if (pack) {
    contextMenu.openMarketUninstall(pack)
  }
}

// =============== 生命周期 ===============
// 说明：首次挂载恢复中断操作并加载模组；缓存页面重新激活时刷新概览与当前标签，
//       因为模组页安装、卸载或设置页修改游戏目录都可能改变资源状态。
const refreshCurrentView = async (): Promise<void> => {
  await env.refreshActiveTab()
  if (activeTab.value === 'mods') {
    await installed.loadInstalledPackages()
  }
  await refreshBepInExReady()
}

let hasMounted = false
onMounted(async () => {
  isPageActive.value = true
  OnFileDrop((_x, _y, paths) => {
    void importDroppedModPaths(paths)
  }, false)
  await env.recoverInterruptedOperations()
  await env.loadOverview()
  await Promise.all([env.loadMods(), installed.loadInstalledPackages()])
  await refreshBepInExReady()
  hasMounted = true
})

onActivated(() => {
  isPageActive.value = true
  if (hasMounted) {
    void refreshCurrentView()
  }
})

onDeactivated(() => {
  isPageActive.value = false
  isModDragActive.value = false
  modDragDepth = 0
})

onBeforeUnmount(() => {
  OnFileDropOff()
  if (searchTimer !== undefined) {
    window.clearTimeout(searchTimer)
  }
})

watch(activeTab, (tab) => {
  if (tab === 'mods') {
    void installed.loadInstalledPackages()
  }
})
</script>

<template>
  <!-- ~ 资源页面 -->
  <!-- * 说明：三标签管理客户端模组资源库、蓝图镜像目录和真实存档目录 -->
  <!-- * 交互：左键执行卡片主操作，右键打开统一操作菜单；模组标签支持拖入 zip 导入 -->
  <!-- * 依赖：依赖 EnvironmentManagerService 与 InstallerService 的已安装包查询 / 卸载 -->
  <!-- * 边界：市场安装的模组只读展示并提供卸载，安装与冲突处理仍在模组页完成 -->
  <section class="environment-page" aria-label="资源">
    <div class="environment-tabs" role="tablist" aria-label="资源类型">
      <button
        v-for="tab in tabs"
        :key="tab.id"
        type="button"
        role="tab"
        class="environment-tab"
        :class="{'is-active': activeTab === tab.id}"
        :aria-selected="activeTab === tab.id"
        @click="activeTab = tab.id"
      >
        <component :is="tab.icon" :size="15" :stroke-width="1.8" />
        <span>{{ tab.label }}</span>
      </button>
      <span class="environment-summary">{{ activeTabSummary }}</span>
    </div>

    <!-- ~ 模组标签 -->
    <!-- * 说明：上方为客户端资源库模组，下方为模组市场安装的包 -->
    <!-- * 交互：资源库卡片左键启用/禁用；市场卡片左键查看详情；空白处右键导入或同步 -->
    <!-- * 依赖：依赖 ListMods 的 installerManaged 标记识别与市场重名的目录 -->
    <!-- * 边界：市场管理的目录不允许在资源库启用/禁用，避免两套记录争夺同一批文件 -->
    <section v-if="activeTab === 'mods'" class="environment-panel">
      <div class="environment-toolbar">
        <n-input
          v-model:value="modSearch"
          class="environment-search"
          size="small"
          clearable
          placeholder="搜索模组"
          :input-props="{spellcheck: false, autocomplete: 'off', 'aria-label': '搜索模组'}"
        >
          <template #prefix>
            <Search :size="14" :stroke-width="1.8" />
          </template>
        </n-input>

        <div class="environment-toolbar-actions">
          <button
            type="button"
            class="environment-sync-button"
            :disabled="isSyncingActiveTab"
            :title="syncButtonLabel"
            :aria-label="syncButtonLabel"
            :aria-busy="isSyncingActiveTab"
            @click="syncActiveTab"
          >
            <RefreshCw :size="15" :stroke-width="1.8" :class="{'is-spinning': isSyncingActiveTab}" />
          </button>
        </div>
      </div>

      <div
        class="environment-scroll is-drop-zone"
        @dragenter="handleModDragEnter"
        @dragover="handleModDragOver"
        @dragleave="handleModDragLeave"
        @drop="handleModDrop"
        @contextmenu="contextMenu.open($event, {kind: 'mod-space'})"
      >
        <div v-if="isModDragActive" class="environment-drop-mask" aria-hidden="true">
          <Package :size="32" :stroke-width="1.7" />
          <strong>导入模组</strong>
          <span>松开后解析 zip 压缩包</span>
        </div>

        <div v-if="modNotice" class="environment-notice" role="status">
          <AlertCircle :size="15" :stroke-width="1.8" />
          <span>{{ modNotice }}</span>
          <n-button size="tiny" secondary @click="router.push('/dysonSphere/settings')">前往游戏设置</n-button>
        </div>

        <h2 class="environment-section-title">
          <span>资源库</span>
          <em>{{ mods.length }}</em>
        </h2>
        <div v-if="!isLoadingMods && mods.length === 0" class="environment-empty">
          <Package :size="26" :stroke-width="1.7" />
          <p v-if="modSearch.trim()">没有匹配的资源库模组。</p>
          <p v-else>
            资源库里还没有模组，可以拖入 zip 导入，或点击
            <button type="button" :disabled="isRunningInitialSync" @click="env.runInitialSync('mod')">一键同步</button>
            纳管游戏目录里已有的模组。
          </p>
        </div>
        <div v-else class="environment-grid">
          <ResourceCard
            v-for="mod in mods"
            :key="mod.id"
            :name="mod.name"
            :meta="mod.version || '未知版本'"
            :status="modStatus(mod)"
            :tone="modTone(mod)"
            :operating="isOperating(mod.id)"
            :title="mod.warning || mod.description || mod.name"
            :aria-disabled="!canToggleMod(mod)"
            @click="handleModClick(mod)"
            @contextmenu="contextMenu.open($event, {kind: 'mod', mod})"
          >
            <Package :size="24" :stroke-width="1.7" />
          </ResourceCard>
        </div>

        <h2 class="environment-section-title">
          <span>模组市场</span>
          <em>{{ installedPackages.length }}</em>
          <button type="button" class="environment-section-link" @click="router.push('/dysonSphere/mod')">浏览市场</button>
        </h2>
        <div v-if="!isLoadingInstalledPackages && filteredInstalledPackages.length === 0" class="environment-empty is-compact">
          <Store :size="22" :stroke-width="1.7" />
          <p>{{ modSearch.trim() ? '没有匹配的市场模组。' : '还没有从模组市场安装的模组。' }}</p>
        </div>
        <div v-else class="environment-grid">
          <ResourceCard
            v-for="pack in filteredInstalledPackages"
            :key="pack.packageKey"
            :name="packTitle(pack)"
            :meta="pack.version || '未知版本'"
            status="已启用"
            tone="enabled"
            :operating="uninstallingPackageKeys[pack.packageKey] || false"
            :title="pack.description || packTitle(pack)"
            @click="installed.openInstalledPackage(pack.packageKey)"
            @contextmenu="contextMenu.open($event, {kind: 'market', pack})"
          >
            <Store :size="24" :stroke-width="1.7" />
          </ResourceCard>
        </div>
      </div>
    </section>

    <!-- ~ 蓝图标签 -->
    <!-- * 说明：展示客户端蓝图镜像目录树，文件夹优先，蓝图文件随后 -->
    <!-- * 交互：面包屑进入/返回目录、拖拽移动、左键启用禁用、右键操作菜单 -->
    <!-- * 依赖：依赖 ListBlueprints 与当前 parentId -->
    <!-- * 边界：移动冲突、复制失败或删除失败由后端阻止 -->
    <section v-else-if="activeTab === 'blueprints'" class="environment-panel">
      <div class="environment-toolbar">
        <n-input
          v-model:value="blueprintSearch"
          class="environment-search"
          size="small"
          clearable
          placeholder="搜索蓝图"
          :input-props="{spellcheck: false, autocomplete: 'off', 'aria-label': '搜索蓝图'}"
        >
          <template #prefix>
            <Search :size="14" :stroke-width="1.8" />
          </template>
        </n-input>

        <!-- * 说明：蓝图路径放在同步按钮左侧，二者共同靠右对齐；路径过长时从左侧截断 -->
        <div class="environment-toolbar-actions">
          <nav class="environment-breadcrumb" aria-label="蓝图目录">
            <button
              type="button"
              :class="{'is-drop-target': isBlueprintDropTarget('')}"
              @dragover="markBlueprintDropTarget($event, '')"
              @dragleave="clearBlueprintDropTarget"
              @drop="dropBlueprint($event, '')"
              @click="env.backToBlueprintRoot()"
            >
              蓝图根目录
            </button>
            <template v-for="crumb in blueprintPage?.breadcrumbs || []" :key="crumb.id">
              <ChevronRight :size="13" :stroke-width="1.8" />
              <button
                type="button"
                :class="{'is-drop-target': isBlueprintDropTarget(crumb.id)}"
                @dragover="markBlueprintDropTarget($event, crumb.id)"
                @dragleave="clearBlueprintDropTarget"
                @drop="dropBlueprint($event, crumb.id)"
                @click="env.enterBlueprintFolder(crumb.id)"
              >
                {{ crumb.name }}
              </button>
            </template>
          </nav>

          <span class="environment-toolbar-divider" aria-hidden="true"></span>
          <button
            type="button"
            class="environment-sync-button"
            :disabled="isSyncingActiveTab"
            :title="syncButtonLabel"
            :aria-label="syncButtonLabel"
            :aria-busy="isSyncingActiveTab"
            @click="syncActiveTab"
          >
            <RefreshCw :size="15" :stroke-width="1.8" :class="{'is-spinning': isSyncingActiveTab}" />
          </button>
        </div>
      </div>

      <div class="environment-scroll" @contextmenu="contextMenu.open($event, {kind: 'blueprint-space'})">
        <div v-if="!isLoadingBlueprints && blueprintItems.length === 0" class="environment-empty">
          <Layers :size="26" :stroke-width="1.7" />
          <p v-if="blueprintSearch.trim()">没有匹配的蓝图。</p>
          <p v-else>
            这个文件夹是空的，点击
            <button type="button" :disabled="isRunningInitialSync" @click="env.runInitialSync('blueprint')">一键同步</button>
            纳管本机蓝图，或右键导入。
          </p>
        </div>
        <div v-else class="environment-grid">
          <ResourceCard
            v-for="folder in blueprintFolders"
            :key="folder.id"
            :name="folder.name"
            :meta="`${folder.childCount} 个项目`"
            tone="folder"
            :class="{'is-dragging': draggedBlueprintId === folder.id, 'is-drop-target': isBlueprintDropTarget(folder.id)}"
            draggable="true"
            @dragstart="startBlueprintDrag($event, folder)"
            @dragend="clearBlueprintDrag"
            @dragover="markBlueprintDropTarget($event, folder.id)"
            @dragleave="clearBlueprintDropTarget"
            @drop="dropBlueprint($event, folder.id)"
            @click="env.enterBlueprintFolder(folder.id)"
            @contextmenu="contextMenu.open($event, {kind: 'blueprint-folder', blueprint: folder})"
          >
            <FolderOpen :size="24" :stroke-width="1.7" />
          </ResourceCard>
          <ResourceCard
            v-for="blueprint in blueprintFiles"
            :key="blueprint.id"
            :name="blueprint.name"
            :meta="formatFileSize(blueprint.sizeBytes)"
            :status="stateLabel(blueprint.state)"
            :tone="stateTone(blueprint.state)"
            :operating="isOperating(blueprint.id)"
            :class="{'is-dragging': draggedBlueprintId === blueprint.id}"
            :aria-disabled="isOperating(blueprint.id)"
            draggable="true"
            @dragstart="startBlueprintDrag($event, blueprint)"
            @dragend="clearBlueprintDrag"
            @click="handleBlueprintClick(blueprint)"
            @contextmenu="contextMenu.open($event, {kind: 'blueprint-file', blueprint})"
          >
            <PencilRuler :size="24" :stroke-width="1.7" />
          </ResourceCard>
        </div>
      </div>
    </section>

    <!-- ~ 存档标签 -->
    <!-- * 说明：直接展示真实存档目录扫描结果，按修改时间倒序 -->
    <!-- * 交互：左键重命名，右键导入、导出、删除 -->
    <!-- * 依赖：依赖 ListSaves 与存档目录解析结果 -->
    <!-- * 边界：存档不进入客户端资源库，页面以真实目录当前状态为准 -->
    <section v-else class="environment-panel">
      <div class="environment-toolbar">
        <n-input
          v-model:value="saveSearch"
          class="environment-search"
          size="small"
          clearable
          placeholder="搜索存档"
          :input-props="{spellcheck: false, autocomplete: 'off', 'aria-label': '搜索存档'}"
        >
          <template #prefix>
            <Search :size="14" :stroke-width="1.8" />
          </template>
        </n-input>

        <div class="environment-toolbar-actions">
          <button
            type="button"
            class="environment-sync-button"
            :disabled="isSyncingActiveTab"
            :title="syncButtonLabel"
            :aria-label="syncButtonLabel"
            :aria-busy="isSyncingActiveTab"
            @click="syncActiveTab"
          >
            <RefreshCw :size="15" :stroke-width="1.8" :class="{'is-spinning': isSyncingActiveTab}" />
          </button>
        </div>
      </div>

      <div class="environment-scroll" @contextmenu="contextMenu.open($event, {kind: 'save-space'})">
        <div v-if="!isLoadingSaves && saves.length === 0" class="environment-empty">
          <Archive :size="26" :stroke-width="1.7" />
          <p v-if="saveSearch.trim()">没有匹配的存档。</p>
          <p v-else-if="overview && !overview.saveRoot.exists">
            {{ overview.saveRoot.message || '未发现存档目录' }}，可以
            <button type="button" @click="env.selectAndSetEnvironmentRootDirectory('save')">手动选择存档目录</button>。
          </p>
          <p v-else>没有找到存档文件。</p>
        </div>
        <div v-else class="environment-grid">
          <ResourceCard
            v-for="save in saves"
            :key="save.id"
            :name="save.name"
            :meta="formatFileSize(save.sizeBytes)"
            :status="formatDate(save.lastModifiedAt)"
            tone="save"
            :operating="isOperating(save.id)"
            :title="save.fileName"
            :aria-disabled="isOperating(save.id)"
            @click="handleSaveClick(save)"
            @contextmenu="contextMenu.open($event, {kind: 'save', save})"
          >
            <Archive :size="24" :stroke-width="1.7" />
          </ResourceCard>
        </div>
      </div>
    </section>

    <!-- ~ 右键菜单 -->
    <n-dropdown
      trigger="manual"
      placement="bottom-start"
      :show="envContextMenu.visible"
      :x="envContextMenu.x"
      :y="envContextMenu.y"
      :options="envContextMenuOptions"
      @clickoutside="contextMenu.close"
      @select="handleContextMenuSelect"
    />

    <!-- ~ 单字段输入弹窗 -->
    <!-- * 说明：复用蓝图新建文件夹、重命名和存档重命名 -->
    <!-- * 边界：非法字符与同名冲突由服务层校验 -->
    <n-modal
      v-model:show="textDialog.visible"
      preset="dialog"
      :title="textDialog.title"
      :positive-text="textDialog.positiveText"
      negative-text="取消"
      :positive-button-props="{type: 'primary', loading: textDialog.isSubmitting, disabled: textDialog.isSubmitting}"
      :negative-button-props="{disabled: textDialog.isSubmitting}"
      :mask-closable="!textDialog.isSubmitting"
      @positive-click="dialogs.submitTextDialog"
    >
      <label class="environment-field">
        <span>{{ textDialog.label }}</span>
        <n-input
          v-model:value="textDialog.value"
          :placeholder="textDialog.placeholder"
          clearable
          @keyup.enter="dialogs.submitTextDialog"
        />
      </label>
    </n-modal>

    <!-- ~ 蓝图快速导入弹窗 -->
    <n-modal
      v-model:show="quickImportDialog.visible"
      preset="dialog"
      title="快速导入"
      positive-text="导入"
      negative-text="取消"
      :positive-button-props="{type: 'primary', loading: quickImportDialog.isSubmitting, disabled: quickImportDialog.isSubmitting}"
      :negative-button-props="{disabled: quickImportDialog.isSubmitting}"
      :mask-closable="!quickImportDialog.isSubmitting"
      @positive-click="dialogs.submitQuickImportDialog"
    >
      <div class="environment-field-group">
        <label class="environment-field">
          <span>蓝图名称</span>
          <n-input v-model:value="quickImportDialog.name" placeholder="输入蓝图名称" clearable />
        </label>
        <label class="environment-field">
          <span>蓝图编码</span>
          <n-input
            v-model:value="quickImportDialog.code"
            type="textarea"
            placeholder="粘贴蓝图编码"
            :autosize="{minRows: 5, maxRows: 9}"
          />
        </label>
      </div>
    </n-modal>

    <!-- ~ 确认弹窗 -->
    <!-- * 说明：删除、卸载、冲突修复等高风险动作统一二次确认 -->
    <n-modal
      v-model:show="confirmDialog.visible"
      preset="dialog"
      :title="confirmDialog.title"
      :positive-text="confirmDialog.positiveText"
      negative-text="取消"
      :positive-button-props="{
        type: confirmDialog.isDanger ? 'error' : 'primary',
        loading: confirmDialog.isSubmitting,
        disabled: confirmDialog.isSubmitting,
      }"
      :negative-button-props="{disabled: confirmDialog.isSubmitting}"
      :mask-closable="!confirmDialog.isSubmitting"
      @positive-click="dialogs.submitConfirmDialog"
    >
      <p class="environment-confirm-text">{{ confirmDialog.content }}</p>
    </n-modal>

    <!-- ~ 资源库模组详情 -->
    <!-- * 说明：展示资源库 manifest 解析信息、依赖和原始 JSON -->
    <n-modal v-model:show="modDetailVisible" preset="card" title="模组详情" class="environment-detail-modal">
      <n-spin :show="isLoadingModDetail" size="small">
        <div v-if="selectedModDetail" class="environment-detail">
          <dl class="environment-detail-grid">
            <dt>名称</dt>
            <dd>{{ selectedModDetail.name }}</dd>
            <dt>版本</dt>
            <dd>{{ selectedModDetail.version || '未知版本' }}</dd>
            <dt>作者</dt>
            <dd>{{ selectedModDetail.author || '未知作者' }}</dd>
            <dt>官网</dt>
            <dd>{{ selectedModDetail.websiteUrl || '未提供' }}</dd>
            <dt>路径</dt>
            <dd>{{ selectedModDetail.libraryPath || '未记录' }}</dd>
            <dt>说明</dt>
            <dd>{{ selectedModDetail.warning || selectedModDetail.description || '暂无说明' }}</dd>
          </dl>
          <div class="environment-detail-section">
            <span>依赖</span>
            <div v-if="selectedModDetail.dependencies.length > 0" class="environment-chip-list">
              <span v-for="dependency in selectedModDetail.dependencies" :key="`${dependency.name}-${dependency.version}`">
                {{ dependency.name }}<em v-if="dependency.version"> · {{ dependency.version }}</em>
              </span>
            </div>
            <p v-else>未声明依赖</p>
          </div>
          <div class="environment-detail-section">
            <span>原始 Manifest</span>
            <n-code class="environment-code" :code="selectedModDetail.rawManifest || '{}'" language="json" />
          </div>
        </div>
      </n-spin>
    </n-modal>

    <!-- ~ 市场模组详情 -->
    <!-- * 说明：展示安装记录中的版本、插件目录和组件明细，提供卸载入口 -->
    <!-- * 边界：卸载按安装记录清理未共享文件，与模组页卸载一致 -->
    <n-modal v-model:show="packDetailVisible" preset="card" title="市场模组详情" class="environment-detail-modal">
      <div v-if="selectedInstalledPackage" class="environment-detail">
        <dl class="environment-detail-grid">
          <dt>名称</dt>
          <dd>{{ packTitle(selectedInstalledPackage) }}</dd>
          <dt>版本</dt>
          <dd>{{ selectedInstalledPackage.version || '未知版本' }}</dd>
          <dt>作者</dt>
          <dd>{{ selectedInstalledPackage.author || selectedInstalledPackage.namespace || '未知作者' }}</dd>
          <dt>来源</dt>
          <dd>{{ selectedInstalledPackage.source === 'manual' ? '手动导入' : 'Thunderstore' }}</dd>
          <dt>安装时间</dt>
          <dd>{{ formatDate(selectedInstalledPackage.installedAt) }}</dd>
          <dt>文件</dt>
          <dd>{{ selectedInstalledPackage.fileCount }} 个 · {{ formatFileSize(selectedInstalledPackage.sizeBytes) }}</dd>
          <dt>说明</dt>
          <dd>{{ selectedInstalledPackage.description || '暂无说明' }}</dd>
        </dl>
        <div class="environment-detail-section">
          <span>插件目录</span>
          <div v-if="selectedInstalledPackage.pluginDirs.length > 0" class="environment-chip-list">
            <span v-for="dir in selectedInstalledPackage.pluginDirs" :key="dir">{{ dir }}</span>
          </div>
          <p v-else>没有写入 plugins 目录的文件</p>
        </div>
        <div class="environment-detail-section">
          <span>组件</span>
          <div v-if="selectedInstalledPackage.components.length > 0" class="environment-chip-list">
            <span v-for="component in selectedInstalledPackage.components" :key="component.componentKey">
              {{ component.displayName || component.name || component.componentKey }}<em v-if="component.version"> · {{ component.version }}</em>
            </span>
          </div>
          <p v-else>没有组件记录</p>
        </div>
      </div>
      <template #footer>
        <div class="environment-detail-actions">
          <n-button
            type="error"
            secondary
            :loading="selectedInstalledPackage ? uninstallingPackageKeys[selectedInstalledPackage.packageKey] : false"
            @click="uninstallSelectedPackage"
          >
            卸载模组
          </n-button>
        </div>
      </template>
    </n-modal>

    <!-- ~ 蓝图详情抽屉 -->
    <!-- * 说明：嵌入 3D 预览，按蓝图编码渲染模型 -->
    <!-- * 边界：只读预览；viewer-panel 负责主题快照恢复与样式清理 -->
    <n-drawer v-model:show="blueprintDetailVisible" width="50vw" placement="right" :trap-focus="false">
      <n-drawer-content closable body-content-style="padding: 0; height: 100%; overflow: hidden;">
        <template #header>
          <div class="blueprint-detail-header">
            <strong>蓝图详情</strong>
            <span v-if="selectedBlueprintDetail" class="blueprint-detail-meta">
              {{ selectedBlueprintDetail.item.name }} · {{ selectedBlueprintDetail.parsed.summary.buildingCount }} 个建筑
            </span>
          </div>
        </template>
        <n-spin :show="isLoadingBlueprintDetail" size="small" class="blueprint-detail-spin" content-style="height: 100%;">
          <div v-if="selectedBlueprintDetail" class="blueprint-detail-body">
            <BlueprintViewerPanel :code="selectedBlueprintDetail.code" />
          </div>
        </n-spin>
      </n-drawer-content>
    </n-drawer>
  </section>
</template>

<style scoped lang="scss">
.environment-page {
  display: flex;
  height: 100%;
  min-height: 0;
  flex-direction: column;
  overflow: hidden;
  color: var(--ds-text-primary);
}

// =============== 标签栏 ===============
.environment-tabs {
  display: flex;
  min-height: 42px;
  flex-shrink: 0;
  align-items: center;
  padding: 0 var(--ds-page-padding-x);
  border-bottom: 1px solid var(--ds-border-subtle);
}

.environment-tab {
  display: inline-flex;
  height: 42px;
  align-items: center;
  gap: 6px;
  padding: 0 14px;
  border-bottom: 2px solid transparent;
  color: var(--ds-text-muted);
  font-size: 13px;
  transition:
    color 160ms ease,
    border-color 160ms ease;

  &:hover {
    color: var(--ds-text-primary);
  }

  &.is-active {
    border-bottom-color: var(--ds-brand);
    color: var(--ds-text-primary);
  }

  &:focus-visible {
    outline: 2px solid var(--ds-focus-ring);
    outline-offset: -2px;
  }
}

.environment-summary {
  margin-left: auto;
  color: var(--ds-text-muted);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

// =============== 工具栏 ===============
.environment-panel {
  display: flex;
  min-height: 0;
  flex: 1;
  flex-direction: column;
}

.environment-toolbar {
  display: flex;
  min-height: 46px;
  flex-shrink: 0;
  align-items: center;
  gap: 8px;
  padding: 6px var(--ds-page-padding-x);
}

.environment-search {
  width: min(260px, 40vw);
  --n-border: none !important;
  --n-border-hover: none !important;
  --n-border-focus: none !important;
  --n-box-shadow-focus: none !important;
  --n-color: transparent !important;
  --n-color-focus: transparent !important;
  --n-text-color: var(--ds-text-secondary) !important;
  --n-placeholder-color: var(--ds-text-disabled) !important;
}

// 说明：搜索栏右侧操作区，蓝图路径与同步按钮共用；路径可收缩，按钮固定尺寸
.environment-toolbar-actions {
  display: flex;
  min-width: 0;
  flex: 1;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
}

.environment-toolbar-divider {
  width: 1px;
  height: 14px;
  flex-shrink: 0;
  background: var(--ds-border-default);
}

.environment-sync-button {
  display: inline-flex;
  width: 28px;
  height: 28px;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  color: var(--ds-text-muted);
  transition:
    background-color 160ms ease,
    color 160ms ease;

  &:hover:not(:disabled) {
    background: var(--ds-hover-bg);
    color: var(--ds-text-primary);
  }

  &:focus-visible {
    outline: 2px solid var(--ds-focus-ring);
    outline-offset: 1px;
  }

  &:disabled {
    cursor: progress;
  }

  .is-spinning {
    animation: environment-spin 1s linear infinite;
  }
}

@keyframes environment-spin {
  to {
    transform: rotate(360deg);
  }
}

// 说明：路径靠右排列，层级过深时左侧被裁掉，始终保留当前所在文件夹
.environment-breadcrumb {
  display: flex;
  min-width: 0;
  overflow: hidden;
  align-items: center;
  justify-content: flex-end;
  gap: 4px;
  color: var(--ds-text-muted);
  font-size: 12px;

  > svg {
    flex-shrink: 0;
  }

  button {
    flex-shrink: 0;
    max-width: 14rem;
    overflow: hidden;
    padding: 2px 6px;
    border-radius: 4px;
    text-overflow: ellipsis;
    white-space: nowrap;

    &:hover {
      color: var(--ds-text-primary);
    }

    &.is-drop-target {
      background: color-mix(in srgb, var(--ds-info) 14%, transparent);
      color: var(--ds-info);
    }
  }
}

// =============== 内容区 ===============
.environment-scroll {
  position: relative;
  min-height: 0;
  flex: 1;
  overflow: auto;
  padding: 4px var(--ds-page-padding-x) var(--ds-page-padding-bottom);

  &.is-drop-zone {
    --wails-drop-target: drop;
  }
}

.environment-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(118px, 1fr));
  gap: 12px;
}

.environment-section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 18px 0 10px;
  color: var(--ds-text-secondary);
  font-size: 12px;
  font-weight: 600;

  &:first-of-type {
    margin-top: 6px;
  }

  em {
    color: var(--ds-text-muted);
    font-style: normal;
    font-weight: 400;
    font-variant-numeric: tabular-nums;
  }
}

.environment-section-link {
  margin-left: auto;
  color: var(--ds-text-muted);
  font-size: 12px;
  font-weight: 400;

  &:hover {
    color: var(--ds-brand);
  }
}

.environment-notice {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 6px 0 4px;
  padding: 7px 8px 7px 12px;
  border: 1px solid var(--ds-warning-border);
  border-radius: 4px;
  background: var(--ds-warning-bg);
  color: var(--ds-warning);
  font-size: 12px;

  span {
    min-width: 0;
    flex: 1;
  }
}

.environment-empty {
  display: grid;
  min-height: 11rem;
  place-items: center;
  align-content: center;
  gap: 10px;
  border: 1px dashed var(--ds-border-default);
  border-radius: 4px;
  color: var(--ds-text-muted);
  font-size: 13px;

  &.is-compact {
    min-height: 6rem;
  }

  p {
    max-width: 30rem;
    margin: 0;
    padding: 0 16px;
    line-height: 1.7;
    text-align: center;
  }

  button {
    color: var(--ds-brand);
    font-weight: 600;
    text-decoration: underline;
    text-underline-offset: 3px;

    &:disabled {
      cursor: wait;
      opacity: 0.6;
    }
  }
}

.environment-drop-mask {
  position: absolute;
  z-index: 5;
  inset: 8px var(--ds-page-padding-x);
  display: grid;
  place-content: center;
  justify-items: center;
  gap: 6px;
  border: 1px dashed var(--ds-border-strong);
  border-radius: 4px;
  background: var(--ds-overlay-bg);
  color: var(--ds-text-primary);
  pointer-events: none;

  svg {
    color: var(--ds-brand);
  }

  strong {
    font-size: 14px;
  }

  span {
    color: var(--ds-text-muted);
    font-size: 12px;
  }
}

// =============== 弹窗内容 ===============
.environment-field-group {
  display: grid;
  gap: 14px;
  padding-top: 8px;
}

.environment-field {
  display: block;
  padding-top: 8px;
  font-size: 13px;

  > span {
    display: block;
    margin-bottom: 8px;
    color: var(--ds-text-primary);
    font-weight: 500;
  }
}

.environment-confirm-text {
  margin: 0;
  padding-top: 8px;
  color: var(--ds-text-secondary);
  font-size: 13px;
  line-height: 1.7;
}

.environment-detail {
  display: grid;
  gap: 16px;
  font-size: 13px;
}

.environment-detail-grid {
  display: grid;
  grid-template-columns: 5rem minmax(0, 1fr);
  gap: 8px 12px;
  margin: 0;

  dt {
    color: var(--ds-text-muted);
  }

  dd {
    min-width: 0;
    margin: 0;
    overflow-wrap: anywhere;
    color: var(--ds-text-primary);
  }
}

.environment-detail-section {
  display: grid;
  gap: 8px;

  > span {
    color: var(--ds-text-primary);
    font-weight: 500;
  }

  > p {
    margin: 0;
    color: var(--ds-text-muted);
  }
}

.environment-chip-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;

  span {
    padding: 2px 8px;
    border: 1px solid var(--ds-border-subtle);
    border-radius: 4px;
    color: var(--ds-text-primary);
  }

  em {
    color: var(--ds-text-muted);
    font-style: normal;
  }
}

.environment-code {
  display: block;
  max-height: 220px;
  overflow: auto;
  padding: 10px;
  border: 1px solid var(--ds-border-subtle);
  border-radius: 4px;
  background: var(--ds-bg-panel-elevated);
}

.environment-detail-actions {
  display: flex;
  justify-content: flex-end;
}

.blueprint-detail-header {
  display: flex;
  min-width: 0;
  align-items: baseline;
  gap: 12px;

  strong {
    flex-shrink: 0;
    font-size: 15px;
  }
}

.blueprint-detail-meta {
  min-width: 0;
  overflow: hidden;
  color: var(--ds-text-muted);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.blueprint-detail-spin,
.blueprint-detail-spin :deep(.n-spin-container) {
  height: 100%;
}

.blueprint-detail-body {
  height: 100%;
  min-height: 360px;
}

@media (max-width: 760px) {
  .environment-toolbar {
    flex-wrap: wrap;
  }

  .environment-search {
    width: 100%;
  }
}
</style>

<style lang="scss">
// 详情弹窗由 n-modal 渲染到页面外部，scoped 样式无法命中
.environment-detail-modal {
  width: min(560px, calc(100vw - 48px));
}
</style>
