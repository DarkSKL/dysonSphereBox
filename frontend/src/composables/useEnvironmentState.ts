import {useMessage} from 'naive-ui'
import {computed, ref, watch} from 'vue'
import {environment} from '../../wailsjs/go/models'
import {
  confirmEnvironmentBlueprintMigration,
  createEnvironmentBlueprintFolder,
  deleteEnvironmentBlueprintItem,
  deleteEnvironmentMod,
  deleteEnvironmentSave,
  disableEnvironmentBlueprint,
  disableEnvironmentMod,
  enableEnvironmentBlueprint,
  enableEnvironmentMod,
  exportEnvironmentBlueprint,
  exportEnvironmentSave,
  getEnvironmentModDetail,
  getEnvironmentOverview,
  importEnvironmentBlueprintFiles,
  importEnvironmentMod,
  importEnvironmentSaves,
  listEnvironmentBlueprints,
  listEnvironmentMods,
  listEnvironmentSaves,
  moveEnvironmentBlueprintItem,
  openEnvironmentBlueprintLibraryDir,
  openEnvironmentModLibraryDir,
  openEnvironmentSaveDir,
  previewEnvironmentInitialSync,
  quickImportEnvironmentBlueprint,
  recoverEnvironmentInterruptedOperations,
  refreshEnvironmentManagedState,
  refreshEnvironmentSaves,
  renameEnvironmentBlueprintItem,
  renameEnvironmentSave,
  restoreEnvironmentGameFromClientMirror,
  runEnvironmentInitialSync,
  selectEnvironmentBlueprintImportFiles,
  selectEnvironmentBlueprintRootDirectory,
  selectEnvironmentExportDirectory,
  selectEnvironmentModImportDirectory,
  selectEnvironmentModImportPaths,
  selectEnvironmentSaveImportFiles,
  selectEnvironmentSaveRootDirectory,
  setEnvironmentRootDirectory,
  unlinkEnvironmentSyncRecord,
  updateEnvironmentClientMirrorFromGame,
} from '../services/environment'

export type EnvironmentTabId = 'mods' | 'blueprints' | 'saves'

// =============== 环境页状态 ===============
// 说明：集中维护环境页三标签、搜索词、目录导航、加载态和 Wails 查询结果。
// 边界：写入操作只在后端真实成功后刷新状态；失败会保留当前列表并向调用弹窗抛出错误。
export function useEnvironmentState() {
  const message = useMessage()
  const activeTab = ref<EnvironmentTabId>('mods')
  const overview = ref<environment.EnvironmentOverview | null>(null)
  const mods = ref<environment.EnvironmentModItem[]>([])
  const selectedModDetail = ref<environment.EnvironmentModDetail | null>(null)
  const blueprintPage = ref<environment.EnvironmentBlueprintPage | null>(null)
  const saves = ref<environment.EnvironmentSaveItem[]>([])
  const modSearch = ref('')
  const blueprintSearch = ref('')
  const saveSearch = ref('')
  const currentBlueprintParentId = ref('')
  const isLoadingOverview = ref(false)
  const isLoadingMods = ref(false)
  const isLoadingModDetail = ref(false)
  const isLoadingBlueprints = ref(false)
  const isLoadingSaves = ref(false)
  const initialSyncPreview = ref<environment.EnvironmentInitialSyncPreview | null>(null)
  const operatingEnvResourceIds = ref<string[]>([])
  const isRunningInitialSync = ref(false)
  const isRefreshingManagedState = ref(false)
  const managedStateSummary = ref<environment.EnvironmentManagedStateSummary | null>(null)
  const recoverySummary = ref<environment.EnvironmentRecoverySummary | null>(null)
  const isSelectingEnvironmentRoot = ref(false)

  const enabledModCount = computed(() => mods.value.filter((item) => item.state === 'enabled').length)
  const disabledModCount = computed(() => mods.value.filter((item) => item.state === 'disabled').length)
  const enabledBlueprintCount = computed(
    () => blueprintPage.value?.items.filter((item) => item.type === 'blueprint' && item.state === 'enabled').length ?? 0,
  )
  const saveCount = computed(() => saves.value.length)
  const hasInitialSyncCandidates = computed(() => {
    const preview = initialSyncPreview.value ?? overview.value?.initialSync
    return Boolean(preview && (preview.mods > 0 || preview.blueprints > 0 || preview.saves > 0))
  })

  // =============== 数据加载 ===============
  // 说明：按页面标签拆分加载流程，刷新概览不会强制覆盖当前蓝图目录位置。
  // 边界：单个标签加载失败只提示错误，不清空其它标签已经可用的数据。
  const loadOverview = async (): Promise<void> => {
    isLoadingOverview.value = true
    try {
      overview.value = await getEnvironmentOverview()
      initialSyncPreview.value = overview.value.initialSync
    } catch (error) {
      message.error(`读取环境概览失败：${formatEnvironmentError(error)}`)
    } finally {
      isLoadingOverview.value = false
    }
  }

  const recoverInterruptedOperations = async (): Promise<void> => {
    try {
      const summary = await recoverEnvironmentInterruptedOperations()
      recoverySummary.value = summary
      if (summary.recoveredOperations > 0) {
        message.success(`已恢复 ${summary.recoveredOperations} 个未完成的环境操作。`)
      }
      if (summary.failedOperations > 0) {
        message.warning(`有 ${summary.failedOperations} 个环境操作恢复失败，请复核状态。`)
      }
      if (summary.managedState) {
        managedStateSummary.value = summary.managedState
      }
    } catch (error) {
      message.error(`恢复环境操作失败：${formatEnvironmentError(error)}`)
    }
  }

  const loadMods = async (): Promise<void> => {
    isLoadingMods.value = true
    try {
      mods.value = await listEnvironmentMods(modSearch.value.trim())
    } catch (error) {
      message.error(`读取模组资源库失败：${formatEnvironmentError(error)}`)
    } finally {
      isLoadingMods.value = false
    }
  }

  const loadModDetail = async (itemId: string): Promise<void> => {
    isLoadingModDetail.value = true
    try {
      selectedModDetail.value = await getEnvironmentModDetail(itemId)
    } catch (error) {
      message.error(`读取模组详情失败：${formatEnvironmentError(error)}`)
    } finally {
      isLoadingModDetail.value = false
    }
  }

  const closeModDetail = (): void => {
    selectedModDetail.value = null
  }

  const loadBlueprints = async (): Promise<void> => {
    isLoadingBlueprints.value = true
    try {
      blueprintPage.value = await listEnvironmentBlueprints(
        currentBlueprintParentId.value,
        blueprintSearch.value.trim(),
      )
    } catch (error) {
      message.error(`读取蓝图资源库失败：${formatEnvironmentError(error)}`)
    } finally {
      isLoadingBlueprints.value = false
    }
  }

  const loadSaves = async (): Promise<void> => {
    isLoadingSaves.value = true
    try {
      saves.value = await listEnvironmentSaves(saveSearch.value.trim())
    } catch (error) {
      message.error(`读取存档目录失败：${formatEnvironmentError(error)}`)
    } finally {
      isLoadingSaves.value = false
    }
  }

  const refreshActiveTab = async (): Promise<void> => {
    await loadOverview()
    if (activeTab.value === 'mods') {
      await loadMods()
      return
    }
    if (activeTab.value === 'blueprints') {
      await loadBlueprints()
      return
    }
    await loadSaves()
  }

  // =============== 蓝图目录导航 ===============
  // 说明：按后端节点 ID 进入文件夹或面包屑目录，保持搜索条件不变。
  // 边界：文件夹不存在时后端会返回空列表，前端不自行猜路径。
  const enterBlueprintFolder = async (folderId: string): Promise<void> => {
    currentBlueprintParentId.value = folderId
    await loadBlueprints()
  }

  const backToBlueprintRoot = async (): Promise<void> => {
    currentBlueprintParentId.value = ''
    await loadBlueprints()
  }

  // =============== 目录与预览操作 ===============
  // 说明：暴露打开目录、同步预览和根目录手动兜底等环境级操作。
  // 边界：打开目录失败时保留当前页面状态，并提示用户配置路径。
  const refreshInitialSyncPreview = async (kind: string = 'all'): Promise<void> => {
    try {
      initialSyncPreview.value = await previewEnvironmentInitialSync(kind)
    } catch (error) {
      message.error(`同步预览失败：${formatEnvironmentError(error)}`)
    }
  }

  const openModLibraryDir = async (): Promise<void> => {
    try {
      await openEnvironmentModLibraryDir()
    } catch (error) {
      message.error(`打开模组目录失败：${formatEnvironmentError(error)}`)
    }
  }

  const openBlueprintLibraryDir = async (): Promise<void> => {
    try {
      await openEnvironmentBlueprintLibraryDir()
    } catch (error) {
      message.error(`打开蓝图目录失败：${formatEnvironmentError(error)}`)
    }
  }

  const openSaveDir = async (): Promise<void> => {
    try {
      await openEnvironmentSaveDir()
    } catch (error) {
      message.error(`打开存档目录失败：${formatEnvironmentError(error)}`)
    }
  }

  const refreshSaveList = async (): Promise<void> => {
    isLoadingSaves.value = true
    try {
      saves.value = saveSearch.value.trim()
        ? await listEnvironmentSaves(saveSearch.value.trim())
        : await refreshEnvironmentSaves()
    } catch (error) {
      message.error(`同步存档失败：${formatEnvironmentError(error)}`)
    } finally {
      isLoadingSaves.value = false
    }
  }

  const refreshManagedState = async (kind: string = 'all'): Promise<void> => {
    if (isRefreshingManagedState.value) {
      return
    }
    isRefreshingManagedState.value = true
    try {
      const summary = await refreshEnvironmentManagedState(kind)
      managedStateSummary.value = summary
      if (summary.changedItems > 0) {
        message.warning(`同步完成：${summary.changedItems} 个资源状态变化，${summary.conflictItems} 个冲突。`)
      } else {
        message.success(`同步完成：${summary.checkedItems} 个资源状态一致。`)
      }
      await refreshActiveTab()
    } catch (error) {
      message.error(`同步状态失败：${formatEnvironmentError(error)}`)
    } finally {
      isRefreshingManagedState.value = false
    }
  }

  const syncEnvironmentResources = async (kind: 'mod' | 'blueprint' | 'save'): Promise<void> => {
    if (kind === 'save') {
      await refreshSaveList()
      return
    }
    await refreshManagedState(kind)
  }

  const selectAndSetEnvironmentRootDirectory = async (kind: 'blueprint' | 'save'): Promise<void> => {
    if (isSelectingEnvironmentRoot.value) {
      return
    }
    isSelectingEnvironmentRoot.value = true
    try {
      const selection = kind === 'blueprint'
        ? await selectEnvironmentBlueprintRootDirectory()
        : await selectEnvironmentSaveRootDirectory()
      if (selection.canceled || selection.paths.length === 0) {
        return
      }
      const result = await setEnvironmentRootDirectory(kind, selection.paths[0])
      overview.value = result.overview
      initialSyncPreview.value = result.overview.initialSync
      message.success(kind === 'blueprint' ? '蓝图目录已保存。' : '存档目录已保存。')
      if (kind === 'blueprint') {
        await loadBlueprints()
      } else {
        await loadSaves()
      }
    } catch (error) {
      message.error(`保存环境目录失败：${formatEnvironmentError(error)}`)
    } finally {
      isSelectingEnvironmentRoot.value = false
    }
  }

  // =============== 写入类资源操作 ===============
  // 说明：调用后端真实同步、禁用、重命名和首次纳管接口，并在成功后刷新对应视图。
  // 边界：不做前端伪成功；后端报错时保留原列表状态并显示错误。
  const reportAndThrowEnvironmentError = (messagePrefix: string, error: unknown): never => {
    message.error(`${messagePrefix}：${formatEnvironmentError(error)}`)
    throw markEnvironmentErrorReported(error)
  }

  const withEnvResourceOperation = async (resourceId: string, action: () => Promise<void>): Promise<void> => {
    if (operatingEnvResourceIds.value.includes(resourceId)) {
      return
    }
    operatingEnvResourceIds.value = [...operatingEnvResourceIds.value, resourceId]
    try {
      await action()
    } finally {
      operatingEnvResourceIds.value = operatingEnvResourceIds.value.filter((id) => id !== resourceId)
    }
  }

  const runInitialSync = async (kind: string = 'all'): Promise<void> => {
    if (isRunningInitialSync.value) {
      return
    }
    isRunningInitialSync.value = true
    try {
      const result = await runEnvironmentInitialSync(kind)
      overview.value = result.overview
      initialSyncPreview.value = result.overview.initialSync
      message.success(`已纳管 ${result.syncedMods} 个模组、${result.syncedBlueprints} 个蓝图，索引 ${result.indexedSaves} 个存档。`)
      if (result.failures.length > 0) {
        message.warning(`部分资源同步失败：${result.failures.slice(0, 2).join('；')}`)
      }
      await refreshActiveTab()
    } catch (error) {
      message.error(`同步本机内容失败：${formatEnvironmentError(error)}`)
    } finally {
      isRunningInitialSync.value = false
    }
  }

  const toggleMod = async (mod: environment.EnvironmentModItem): Promise<void> => {
    await withEnvResourceOperation(mod.id, async () => {
      try {
        const result = mod.state === 'enabled'
          ? await disableEnvironmentMod(mod.id)
          : await enableEnvironmentMod(mod.id)
        message.success(result.message)
        await loadMods()
        await loadOverview()
      } catch (error) {
        message.error(`模组操作失败：${formatEnvironmentError(error)}`)
        throw error
      }
    })
  }

  const importMod = async (paths: string[]): Promise<void> => {
    try {
      const result = await importEnvironmentMod(paths)
      if (result.mods) {
        mods.value = result.mods
      } else {
        await loadMods()
      }
      message.success(result.message)
      await loadOverview()
    } catch (error) {
      reportAndThrowEnvironmentError('导入模组失败', error)
    }
  }

  const selectAndImportModFiles = async (): Promise<void> => {
    try {
      const selection = await selectEnvironmentModImportPaths()
      if (selection.canceled || selection.paths.length === 0) {
        return
      }
      await importMod(selection.paths)
    } catch (error) {
      if (!isEnvironmentErrorReported(error)) {
        message.error(`选择模组文件失败：${formatEnvironmentError(error)}`)
      }
    }
  }

  const selectAndImportModDirectory = async (): Promise<void> => {
    try {
      const selection = await selectEnvironmentModImportDirectory()
      if (selection.canceled || selection.paths.length === 0) {
        return
      }
      await importMod(selection.paths)
    } catch (error) {
      if (!isEnvironmentErrorReported(error)) {
        message.error(`选择模组目录失败：${formatEnvironmentError(error)}`)
      }
    }
  }

  const deleteMod = async (mod: environment.EnvironmentModItem): Promise<void> => {
    await withEnvResourceOperation(mod.id, async () => {
      try {
        const result = await deleteEnvironmentMod(mod.id)
        if (result.mods) {
          mods.value = result.mods
        } else {
          await loadMods()
        }
        message.success(result.message)
        await loadOverview()
      } catch (error) {
        reportAndThrowEnvironmentError('删除模组失败', error)
      }
    })
  }

  const toggleBlueprint = async (blueprint: environment.EnvironmentBlueprintItem): Promise<void> => {
    if (blueprint.type !== 'blueprint') {
      return
    }
    await withEnvResourceOperation(blueprint.id, async () => {
      try {
        const result = blueprint.state === 'enabled'
          ? await disableEnvironmentBlueprint(blueprint.id)
          : await enableEnvironmentBlueprint(blueprint.id)
        if (result.blueprintPage) {
          blueprintPage.value = result.blueprintPage
        } else {
          await loadBlueprints()
        }
        message.success(result.message)
        await loadOverview()
      } catch (error) {
        message.error(`蓝图操作失败：${formatEnvironmentError(error)}`)
        throw error
      }
    })
  }

  const createBlueprintFolder = async (name = '新建文件夹'): Promise<void> => {
    try {
      const result = await createEnvironmentBlueprintFolder(currentBlueprintParentId.value, name)
      if (result.blueprintPage) {
        blueprintPage.value = result.blueprintPage
      } else {
        await loadBlueprints()
      }
      message.success(result.message)
      await loadOverview()
    } catch (error) {
      reportAndThrowEnvironmentError('创建蓝图文件夹失败', error)
    }
  }

  const importBlueprintFiles = async (paths: string[]): Promise<void> => {
    try {
      const result = await importEnvironmentBlueprintFiles(currentBlueprintParentId.value, paths)
      if (result.blueprintPage) {
        blueprintPage.value = result.blueprintPage
      } else {
        await loadBlueprints()
      }
      message.success(result.message)
      await loadOverview()
    } catch (error) {
      reportAndThrowEnvironmentError('导入蓝图失败', error)
    }
  }

  const selectAndImportBlueprintFiles = async (): Promise<void> => {
    try {
      const selection = await selectEnvironmentBlueprintImportFiles()
      if (selection.canceled || selection.paths.length === 0) {
        return
      }
      await importBlueprintFiles(selection.paths)
    } catch (error) {
      if (!isEnvironmentErrorReported(error)) {
        message.error(`选择蓝图文件失败：${formatEnvironmentError(error)}`)
      }
    }
  }

  const quickImportBlueprint = async (name: string, code: string): Promise<void> => {
    try {
      const result = await quickImportEnvironmentBlueprint(currentBlueprintParentId.value, name, code)
      if (result.blueprintPage) {
        blueprintPage.value = result.blueprintPage
      } else {
        await loadBlueprints()
      }
      message.success(result.message)
      await loadOverview()
    } catch (error) {
      reportAndThrowEnvironmentError('快速导入蓝图失败', error)
    }
  }

  const renameBlueprint = async (itemId: string, name: string): Promise<void> => {
    await withEnvResourceOperation(itemId, async () => {
      try {
        const result = await renameEnvironmentBlueprintItem(itemId, name)
        if (result.blueprintPage) {
          blueprintPage.value = result.blueprintPage
        } else {
          await loadBlueprints()
        }
        message.success(result.message)
        await loadOverview()
      } catch (error) {
        reportAndThrowEnvironmentError('重命名蓝图失败', error)
      }
    })
  }

  const moveBlueprint = async (itemId: string, targetParentId: string): Promise<void> => {
    await withEnvResourceOperation(itemId, async () => {
      try {
        const result = await moveEnvironmentBlueprintItem(itemId, targetParentId)
        if (result.blueprintPage) {
          blueprintPage.value = result.blueprintPage
          currentBlueprintParentId.value = result.blueprintPage.parentId
        } else {
          await loadBlueprints()
        }
        message.success(result.message)
        await loadOverview()
      } catch (error) {
        reportAndThrowEnvironmentError('移动蓝图失败', error)
      }
    })
  }

  const confirmBlueprintMigration = async (
    preview: environment.EnvironmentBlueprintMigrationPreview,
  ): Promise<void> => {
    await withEnvResourceOperation(preview.itemId, async () => {
      try {
        const result = await confirmEnvironmentBlueprintMigration(preview)
        if (result.blueprintPage) {
          blueprintPage.value = result.blueprintPage
          currentBlueprintParentId.value = result.blueprintPage.parentId
        } else {
          await loadBlueprints()
        }
        message.success(result.message)
        await loadOverview()
      } catch (error) {
        reportAndThrowEnvironmentError('迁移蓝图失败', error)
      }
    })
  }

  const deleteBlueprint = async (itemId: string): Promise<void> => {
    await withEnvResourceOperation(itemId, async () => {
      try {
        const result = await deleteEnvironmentBlueprintItem(itemId)
        if (result.blueprintPage) {
          blueprintPage.value = result.blueprintPage
        } else {
          await loadBlueprints()
        }
        message.success(result.message)
        await loadOverview()
      } catch (error) {
        reportAndThrowEnvironmentError('删除蓝图失败', error)
      }
    })
  }

  const exportBlueprint = async (itemId: string, targetDir: string): Promise<void> => {
    await withEnvResourceOperation(itemId, async () => {
      try {
        const result = await exportEnvironmentBlueprint(itemId, targetDir)
        message.success(result.message)
      } catch (error) {
        reportAndThrowEnvironmentError('导出蓝图失败', error)
      }
    })
  }

  const selectAndExportBlueprint = async (itemId: string): Promise<void> => {
    await withEnvResourceOperation(itemId, async () => {
      try {
        const selection = await selectEnvironmentExportDirectory()
        if (selection.canceled || selection.paths.length === 0) {
          return
        }
        const result = await exportEnvironmentBlueprint(itemId, selection.paths[0])
        message.success(result.message)
      } catch (error) {
        reportAndThrowEnvironmentError('导出蓝图失败', error)
      }
    })
  }

  const renameSave = async (saveId: string, name: string): Promise<void> => {
    await withEnvResourceOperation(saveId, async () => {
      try {
        const result = await renameEnvironmentSave(saveId, name)
        saves.value = result.saves ?? []
        message.success(result.message)
        await loadOverview()
      } catch (error) {
        reportAndThrowEnvironmentError('重命名存档失败', error)
      }
    })
  }

  const importSaves = async (paths: string[]): Promise<void> => {
    try {
      const result = await importEnvironmentSaves(paths)
      saves.value = result.saves ?? []
      message.success(result.message)
      await loadOverview()
    } catch (error) {
      reportAndThrowEnvironmentError('导入存档失败', error)
    }
  }

  const selectAndImportSaves = async (): Promise<void> => {
    try {
      const selection = await selectEnvironmentSaveImportFiles()
      if (selection.canceled || selection.paths.length === 0) {
        return
      }
      await importSaves(selection.paths)
    } catch (error) {
      if (!isEnvironmentErrorReported(error)) {
        message.error(`选择存档文件失败：${formatEnvironmentError(error)}`)
      }
    }
  }

  const exportSave = async (saveId: string, targetDir: string): Promise<void> => {
    await withEnvResourceOperation(saveId, async () => {
      try {
        const result = await exportEnvironmentSave(saveId, targetDir)
        message.success(result.message)
      } catch (error) {
        reportAndThrowEnvironmentError('导出存档失败', error)
      }
    })
  }

  const selectAndExportSave = async (saveId: string): Promise<void> => {
    await withEnvResourceOperation(saveId, async () => {
      try {
        const selection = await selectEnvironmentExportDirectory()
        if (selection.canceled || selection.paths.length === 0) {
          return
        }
        const result = await exportEnvironmentSave(saveId, selection.paths[0])
        message.success(result.message)
      } catch (error) {
        reportAndThrowEnvironmentError('导出存档失败', error)
      }
    })
  }

  const deleteSave = async (saveId: string): Promise<void> => {
    await withEnvResourceOperation(saveId, async () => {
      try {
        const result = await deleteEnvironmentSave(saveId)
        saves.value = result.saves ?? []
        message.success(result.message)
        await loadOverview()
      } catch (error) {
        reportAndThrowEnvironmentError('删除存档失败', error)
      }
    })
  }

  const updateClientMirrorFromGame = async (itemId: string, kind: 'mod' | 'blueprint'): Promise<void> => {
    await withEnvResourceOperation(itemId, async () => {
      try {
        const result = await updateEnvironmentClientMirrorFromGame(itemId, kind)
        applyEnvironmentOperationResult(result, kind)
        message.success(result.message)
        await loadOverview()
      } catch (error) {
        reportAndThrowEnvironmentError('更新客户端镜像失败', error)
      }
    })
  }

  const restoreGameFromClientMirror = async (itemId: string, kind: 'mod' | 'blueprint'): Promise<void> => {
    await withEnvResourceOperation(itemId, async () => {
      try {
        const result = await restoreEnvironmentGameFromClientMirror(itemId, kind)
        applyEnvironmentOperationResult(result, kind)
        message.success(result.message)
        await loadOverview()
      } catch (error) {
        reportAndThrowEnvironmentError('恢复游戏侧文件失败', error)
      }
    })
  }

  const unlinkSyncRecord = async (itemId: string, kind: 'mod' | 'blueprint'): Promise<void> => {
    await withEnvResourceOperation(itemId, async () => {
      try {
        const result = await unlinkEnvironmentSyncRecord(itemId, kind)
        applyEnvironmentOperationResult(result, kind)
        message.success(result.message)
        await loadOverview()
      } catch (error) {
        reportAndThrowEnvironmentError('解除同步记录失败', error)
      }
    })
  }

  const applyEnvironmentOperationResult = (
    result: environment.EnvironmentOperationResult,
    kind: 'mod' | 'blueprint',
  ): void => {
    if (kind === 'mod') {
      if (result.mods) {
        mods.value = result.mods
      } else if (result.mod) {
        const updatedMod = result.mod
        mods.value = mods.value.map((modResource) => (modResource.id === updatedMod.id ? updatedMod : modResource))
      }
      return
    }
    if (result.blueprintPage) {
      blueprintPage.value = result.blueprintPage
    }
  }

  const notifyReadonlyStage = (action: string): void => {
    message.info(`${action} 将在环境管理后续阶段开放，当前版本先提供真实目录扫描。`)
  }

  watch(activeTab, async () => {
    if (activeTab.value === 'mods' && mods.value.length === 0) {
      await loadMods()
    } else if (activeTab.value === 'blueprints' && blueprintPage.value === null) {
      await loadBlueprints()
    } else if (activeTab.value === 'saves' && saves.value.length === 0) {
      await loadSaves()
    }
  })

  return {
    activeTab,
    overview,
    mods,
    selectedModDetail,
    blueprintPage,
    saves,
    modSearch,
    blueprintSearch,
    saveSearch,
    currentBlueprintParentId,
    isLoadingOverview,
    isLoadingMods,
    isLoadingModDetail,
    isLoadingBlueprints,
    isLoadingSaves,
    initialSyncPreview,
    operatingEnvResourceIds,
    isRunningInitialSync,
    isRefreshingManagedState,
    isSelectingEnvironmentRoot,
    managedStateSummary,
    recoverySummary,
    enabledModCount,
    disabledModCount,
    enabledBlueprintCount,
    saveCount,
    hasInitialSyncCandidates,
    loadOverview,
    recoverInterruptedOperations,
    loadMods,
    loadModDetail,
    closeModDetail,
    loadBlueprints,
    loadSaves,
    refreshActiveTab,
    enterBlueprintFolder,
    backToBlueprintRoot,
    refreshInitialSyncPreview,
    runInitialSync,
    toggleMod,
    importMod,
    selectAndImportModFiles,
    selectAndImportModDirectory,
    deleteMod,
    toggleBlueprint,
    createBlueprintFolder,
    importBlueprintFiles,
    selectAndImportBlueprintFiles,
    quickImportBlueprint,
    renameBlueprint,
    moveBlueprint,
    confirmBlueprintMigration,
    deleteBlueprint,
    exportBlueprint,
    selectAndExportBlueprint,
    renameSave,
    importSaves,
    selectAndImportSaves,
    exportSave,
    selectAndExportSave,
    deleteSave,
    updateClientMirrorFromGame,
    restoreGameFromClientMirror,
    unlinkSyncRecord,
    openModLibraryDir,
    openBlueprintLibraryDir,
    openSaveDir,
    refreshSaveList,
    refreshManagedState,
    syncEnvironmentResources,
    selectAndSetEnvironmentRootDirectory,
    notifyReadonlyStage,
  }
}

// =============== 错误格式化 ===============
// 说明：统一把 Wails/JS 错误转换为用户可读文本，并标记已经展示过的业务错误。
// 边界：不吞掉错误；选择器取消不进入错误流程，业务失败会继续向弹窗确认逻辑抛出。
const environmentErrorReportedKey = Symbol('environmentErrorReported')

type ReportedEnvironmentError = Error & {
  [environmentErrorReportedKey]?: boolean
}

// 说明：把已提示过的错误打上内部标记，避免选择器外层 catch 重复弹 toast。
const markEnvironmentErrorReported = (error: unknown): unknown => {
  if (error instanceof Error) {
    ;(error as ReportedEnvironmentError)[environmentErrorReportedKey] = true
    return error
  }

  const wrappedError = new Error(formatEnvironmentError(error)) as ReportedEnvironmentError
  wrappedError[environmentErrorReportedKey] = true
  return wrappedError
}

// 说明：判断错误是否已经由业务动作展示过，供选择器 catch 决定是否补充选择失败提示。
const isEnvironmentErrorReported = (error: unknown): boolean => {
  return error instanceof Error && Boolean((error as ReportedEnvironmentError)[environmentErrorReportedKey])
}

const formatEnvironmentError = (error: unknown): string => {
  if (error instanceof Error) {
    return error.message
  }

  return String(error || '未知错误')
}
