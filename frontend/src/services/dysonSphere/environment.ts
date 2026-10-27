import {
  CreateBlueprintFolder as createBlueprintFolderBinding,
  ConfirmBlueprintMigration as confirmBlueprintMigrationBinding,
  DeleteBlueprintItem as deleteBlueprintItemBinding,
  DeleteMod as deleteModBinding,
  DeleteSave as deleteSaveBinding,
  DisableBlueprint as disableBlueprintBinding,
  DisableMod as disableModBinding,
  EnableBlueprint as enableBlueprintBinding,
  EnableMod as enableModBinding,
  ExportBlueprint as exportBlueprintBinding,
  ExportSave as exportSaveBinding,
  GetBlueprintDetail as getBlueprintDetailBinding,
  ImportBlueprintFiles as importBlueprintFilesBinding,
  ImportMod as importModBinding,
  ImportSaves as importSavesBinding,
  GetModDetail as getModDetailBinding,
  GetOperation as getOperationBinding,
  GetOverview as getOverviewBinding,
  ListBlueprints as listBlueprintsBinding,
  ListMods as listModsBinding,
  ListSaves as listSavesBinding,
  MoveBlueprintItem as moveBlueprintItemBinding,
  OpenBlueprintLibraryDir as openBlueprintLibraryDirBinding,
  OpenBlueprintRealDir as openBlueprintRealDirBinding,
  OpenModLibraryDir as openModLibraryDirBinding,
  OpenModRealDir as openModRealDirBinding,
  OpenSaveLibraryDir as openSaveLibraryDirBinding,
  OpenSaveRealDir as openSaveRealDirBinding,
  PreviewBlueprintMigration as previewBlueprintMigrationBinding,
  PreviewInitialSync as previewInitialSyncBinding,
  QuickImportBlueprint as quickImportBlueprintBinding,
  SaveGeneratedBlueprint as saveGeneratedBlueprintBinding,
  RecoverInterruptedOperations as recoverInterruptedOperationsBinding,
  RefreshManagedState as refreshManagedStateBinding,
  RefreshSaves as refreshSavesBinding,
  RenameBlueprintItem as renameBlueprintItemBinding,
  RenameSave as renameSaveBinding,
  RestoreGameFromClientMirror as restoreGameFromClientMirrorBinding,
  RunInitialSync as runInitialSyncBinding,
  SelectBlueprintImportFiles as selectBlueprintImportFilesBinding,
  SelectBlueprintRootDirectory as selectBlueprintRootDirectoryBinding,
  SelectEnvironmentExportDirectory as selectEnvironmentExportDirectoryBinding,
  SelectModImportDirectory as selectModImportDirectoryBinding,
  SelectModImportPaths as selectModImportPathsBinding,
  SelectSaveImportFiles as selectSaveImportFilesBinding,
  SelectSaveRootDirectory as selectSaveRootDirectoryBinding,
  SetEnvironmentRootDirectory as setEnvironmentRootDirectoryBinding,
  UnlinkEnvironmentSyncRecord as unlinkEnvironmentSyncRecordBinding,
  UpdateClientMirrorFromGame as updateClientMirrorFromGameBinding,
} from '../../../wailsjs/go/environment/EnvironmentManagerService'
import {environment} from '../../../wailsjs/go/models'

interface EnvironmentBridgeWindow extends Window {
  go?: {
    environment?: {
      EnvironmentManagerService?: Record<string, unknown>
    }
  }
}

type EnvironmentBridgeMethod =
  | 'GetOverview'
  | 'GetModDetail'
  | 'GetBlueprintDetail'
  | 'GetOperation'
  | 'ListMods'
  | 'ListBlueprints'
  | 'ListSaves'
  | 'RefreshSaves'
  | 'PreviewInitialSync'
  | 'PreviewBlueprintMigration'
  | 'ConfirmBlueprintMigration'
  | 'RunInitialSync'
  | 'EnableMod'
  | 'DisableMod'
  | 'ImportMod'
  | 'DeleteMod'
  | 'CreateBlueprintFolder'
  | 'ImportBlueprintFiles'
  | 'QuickImportBlueprint'
  | 'SaveGeneratedBlueprint'
  | 'RecoverInterruptedOperations'
  | 'RefreshManagedState'
  | 'EnableBlueprint'
  | 'DisableBlueprint'
  | 'MoveBlueprintItem'
  | 'RenameBlueprintItem'
  | 'DeleteBlueprintItem'
  | 'ExportBlueprint'
  | 'RenameSave'
  | 'ImportSaves'
  | 'ExportSave'
  | 'DeleteSave'
  | 'UpdateClientMirrorFromGame'
  | 'RestoreGameFromClientMirror'
  | 'UnlinkEnvironmentSyncRecord'
  | 'OpenModLibraryDir'
  | 'OpenBlueprintLibraryDir'
  | 'OpenSaveLibraryDir'
  | 'OpenModRealDir'
  | 'OpenBlueprintRealDir'
  | 'OpenSaveRealDir'
  | 'SelectModImportPaths'
  | 'SelectModImportDirectory'
  | 'SelectBlueprintImportFiles'
  | 'SelectBlueprintRootDirectory'
  | 'SelectSaveImportFiles'
  | 'SelectSaveRootDirectory'
  | 'SelectEnvironmentExportDirectory'
  | 'SetEnvironmentRootDirectory'

// =============== Wails 环境管理桥接 ===============
// 说明：集中封装 EnvironmentManagerService 调用，避免页面直接依赖 window.go 注入细节。
// 边界：若后端绑定尚未生成或开发服务未重启，返回可读错误提醒用户重启运行环境。
const assertEnvironmentBridgeReady = (methodName: EnvironmentBridgeMethod): void => {
  const bridge = (window as EnvironmentBridgeWindow).go?.environment?.EnvironmentManagerService
  if (typeof bridge?.[methodName] !== 'function') {
    throw new Error(`EnvironmentManagerService.${methodName} 尚未绑定，请重新启动 Wails 开发服务或应用后再试。`)
  }
}

export const getEnvironmentOverview = async (): Promise<environment.EnvironmentOverview> => {
  assertEnvironmentBridgeReady('GetOverview')
  return getOverviewBinding()
}

export const getEnvironmentOperation = async (
  operationId: number,
): Promise<environment.EnvironmentOperation> => {
  assertEnvironmentBridgeReady('GetOperation')
  return getOperationBinding(operationId)
}

export const getEnvironmentModDetail = async (
  itemId: string,
): Promise<environment.EnvironmentModDetail> => {
  assertEnvironmentBridgeReady('GetModDetail')
  return getModDetailBinding(itemId)
}

export const getEnvironmentBlueprintDetail = async (
  itemId: string,
): Promise<environment.EnvironmentBlueprintDetail> => {
  assertEnvironmentBridgeReady('GetBlueprintDetail')
  return getBlueprintDetailBinding(itemId)
}

export const listEnvironmentMods = async (query: string): Promise<environment.EnvironmentModItem[]> => {
  assertEnvironmentBridgeReady('ListMods')
  return listModsBinding(query)
}

export const listEnvironmentBlueprints = async (
  parentId: string,
  query: string,
): Promise<environment.EnvironmentBlueprintPage> => {
  assertEnvironmentBridgeReady('ListBlueprints')
  return listBlueprintsBinding(parentId, query)
}

export const listEnvironmentSaves = async (query: string): Promise<environment.EnvironmentSaveItem[]> => {
  assertEnvironmentBridgeReady('ListSaves')
  return listSavesBinding(query)
}

export const refreshEnvironmentSaves = async (): Promise<environment.EnvironmentSaveItem[]> => {
  assertEnvironmentBridgeReady('RefreshSaves')
  return refreshSavesBinding()
}

export const previewEnvironmentInitialSync = async (
  kind: string,
): Promise<environment.EnvironmentInitialSyncPreview> => {
  assertEnvironmentBridgeReady('PreviewInitialSync')
  return previewInitialSyncBinding(kind)
}

export const refreshEnvironmentManagedState = async (
  kind: string,
): Promise<environment.EnvironmentManagedStateSummary> => {
  assertEnvironmentBridgeReady('RefreshManagedState')
  return refreshManagedStateBinding(kind)
}

export const recoverEnvironmentInterruptedOperations = async (): Promise<environment.EnvironmentRecoverySummary> => {
  assertEnvironmentBridgeReady('RecoverInterruptedOperations')
  return recoverInterruptedOperationsBinding()
}

export const runEnvironmentInitialSync = async (
  kind: string,
): Promise<environment.EnvironmentInitialSyncResult> => {
  assertEnvironmentBridgeReady('RunInitialSync')
  return runInitialSyncBinding(new environment.EnvironmentInitialSyncRequest({kind}))
}

export const enableEnvironmentMod = async (
  itemId: string,
): Promise<environment.EnvironmentOperationResult> => {
  assertEnvironmentBridgeReady('EnableMod')
  return enableModBinding(itemId)
}

export const disableEnvironmentMod = async (
  itemId: string,
): Promise<environment.EnvironmentOperationResult> => {
  assertEnvironmentBridgeReady('DisableMod')
  return disableModBinding(itemId)
}

export const importEnvironmentMod = async (
  paths: string[],
): Promise<environment.EnvironmentOperationResult> => {
  assertEnvironmentBridgeReady('ImportMod')
  return importModBinding(paths)
}

export const deleteEnvironmentMod = async (
  itemId: string,
): Promise<environment.EnvironmentOperationResult> => {
  assertEnvironmentBridgeReady('DeleteMod')
  return deleteModBinding(itemId)
}

export const createEnvironmentBlueprintFolder = async (
  parentId: string,
  name: string,
): Promise<environment.EnvironmentOperationResult> => {
  assertEnvironmentBridgeReady('CreateBlueprintFolder')
  return createBlueprintFolderBinding(parentId, name)
}

export const importEnvironmentBlueprintFiles = async (
  parentId: string,
  paths: string[],
): Promise<environment.EnvironmentOperationResult> => {
  assertEnvironmentBridgeReady('ImportBlueprintFiles')
  return importBlueprintFilesBinding(parentId, paths)
}

export const quickImportEnvironmentBlueprint = async (
  parentId: string,
  name: string,
  code: string,
): Promise<environment.EnvironmentOperationResult> => {
  assertEnvironmentBridgeReady('QuickImportBlueprint')
  return quickImportBlueprintBinding(parentId, name, code)
}

// 量化工作台保存生成结果：同名文件直接覆盖，便于同一方案反复生成后更新。
export const saveGeneratedEnvironmentBlueprint = async (
  parentId: string,
  name: string,
  code: string,
): Promise<environment.EnvironmentOperationResult> => {
  assertEnvironmentBridgeReady('SaveGeneratedBlueprint')
  return saveGeneratedBlueprintBinding(parentId, name, code)
}

export const enableEnvironmentBlueprint = async (
  itemId: string,
): Promise<environment.EnvironmentOperationResult> => {
  assertEnvironmentBridgeReady('EnableBlueprint')
  return enableBlueprintBinding(itemId)
}

export const disableEnvironmentBlueprint = async (
  itemId: string,
): Promise<environment.EnvironmentOperationResult> => {
  assertEnvironmentBridgeReady('DisableBlueprint')
  return disableBlueprintBinding(itemId)
}

export const moveEnvironmentBlueprintItem = async (
  itemId: string,
  targetParentId: string,
): Promise<environment.EnvironmentOperationResult> => {
  assertEnvironmentBridgeReady('MoveBlueprintItem')
  return moveBlueprintItemBinding(itemId, targetParentId)
}

export const previewEnvironmentBlueprintMigration = async (
  itemId: string,
  operation: string,
  targetParentId: string,
  targetName: string,
): Promise<environment.EnvironmentBlueprintMigrationPreview> => {
  assertEnvironmentBridgeReady('PreviewBlueprintMigration')
  return previewBlueprintMigrationBinding(itemId, operation, targetParentId, targetName)
}

export const confirmEnvironmentBlueprintMigration = async (
  preview: environment.EnvironmentBlueprintMigrationPreview,
): Promise<environment.EnvironmentOperationResult> => {
  assertEnvironmentBridgeReady('ConfirmBlueprintMigration')
  return confirmBlueprintMigrationBinding(new environment.EnvironmentBlueprintMigrationRequest({
    itemId: preview.itemId,
    operation: preview.operation,
    targetParentId: preview.targetParentId,
    targetName: preview.targetName,
  }))
}

export const renameEnvironmentBlueprintItem = async (
  itemId: string,
  name: string,
): Promise<environment.EnvironmentOperationResult> => {
  assertEnvironmentBridgeReady('RenameBlueprintItem')
  return renameBlueprintItemBinding(itemId, name)
}

export const deleteEnvironmentBlueprintItem = async (
  itemId: string,
): Promise<environment.EnvironmentOperationResult> => {
  assertEnvironmentBridgeReady('DeleteBlueprintItem')
  return deleteBlueprintItemBinding(itemId)
}

export const exportEnvironmentBlueprint = async (
  itemId: string,
  targetDir: string,
): Promise<environment.EnvironmentOperationResult> => {
  assertEnvironmentBridgeReady('ExportBlueprint')
  return exportBlueprintBinding(itemId, targetDir)
}

export const renameEnvironmentSave = async (
  saveId: string,
  name: string,
): Promise<environment.EnvironmentOperationResult> => {
  assertEnvironmentBridgeReady('RenameSave')
  return renameSaveBinding(saveId, name)
}

export const importEnvironmentSaves = async (
  paths: string[],
): Promise<environment.EnvironmentOperationResult> => {
  assertEnvironmentBridgeReady('ImportSaves')
  return importSavesBinding(paths)
}

export const exportEnvironmentSave = async (
  saveId: string,
  targetDir: string,
): Promise<environment.EnvironmentOperationResult> => {
  assertEnvironmentBridgeReady('ExportSave')
  return exportSaveBinding(saveId, targetDir)
}

export const deleteEnvironmentSave = async (
  saveId: string,
): Promise<environment.EnvironmentOperationResult> => {
  assertEnvironmentBridgeReady('DeleteSave')
  return deleteSaveBinding(saveId)
}

export const updateEnvironmentClientMirrorFromGame = async (
  itemId: string,
  kind: string,
): Promise<environment.EnvironmentOperationResult> => {
  assertEnvironmentBridgeReady('UpdateClientMirrorFromGame')
  return updateClientMirrorFromGameBinding(itemId, kind)
}

export const restoreEnvironmentGameFromClientMirror = async (
  itemId: string,
  kind: string,
): Promise<environment.EnvironmentOperationResult> => {
  assertEnvironmentBridgeReady('RestoreGameFromClientMirror')
  return restoreGameFromClientMirrorBinding(itemId, kind)
}

export const unlinkEnvironmentSyncRecord = async (
  itemId: string,
  kind: string,
): Promise<environment.EnvironmentOperationResult> => {
  assertEnvironmentBridgeReady('UnlinkEnvironmentSyncRecord')
  return unlinkEnvironmentSyncRecordBinding(itemId, kind)
}

export const openEnvironmentModLibraryDir = async (): Promise<void> => {
  assertEnvironmentBridgeReady('OpenModLibraryDir')
  return openModLibraryDirBinding()
}

export const openEnvironmentBlueprintLibraryDir = async (): Promise<void> => {
  assertEnvironmentBridgeReady('OpenBlueprintLibraryDir')
  return openBlueprintLibraryDirBinding()
}

export const openEnvironmentSaveLibraryDir = async (): Promise<void> => {
  assertEnvironmentBridgeReady('OpenSaveLibraryDir')
  return openSaveLibraryDirBinding()
}

export const openEnvironmentModRealDir = async (): Promise<void> => {
  assertEnvironmentBridgeReady('OpenModRealDir')
  return openModRealDirBinding()
}

export const openEnvironmentBlueprintRealDir = async (): Promise<void> => {
  assertEnvironmentBridgeReady('OpenBlueprintRealDir')
  return openBlueprintRealDirBinding()
}

export const openEnvironmentSaveRealDir = async (): Promise<void> => {
  assertEnvironmentBridgeReady('OpenSaveRealDir')
  return openSaveRealDirBinding()
}

export const selectEnvironmentModImportPaths = async (): Promise<environment.EnvironmentPathSelection> => {
  assertEnvironmentBridgeReady('SelectModImportPaths')
  return selectModImportPathsBinding()
}

export const selectEnvironmentModImportDirectory = async (): Promise<environment.EnvironmentPathSelection> => {
  assertEnvironmentBridgeReady('SelectModImportDirectory')
  return selectModImportDirectoryBinding()
}

export const selectEnvironmentBlueprintImportFiles = async (): Promise<environment.EnvironmentPathSelection> => {
  assertEnvironmentBridgeReady('SelectBlueprintImportFiles')
  return selectBlueprintImportFilesBinding()
}

export const selectEnvironmentBlueprintRootDirectory = async (): Promise<environment.EnvironmentPathSelection> => {
  assertEnvironmentBridgeReady('SelectBlueprintRootDirectory')
  return selectBlueprintRootDirectoryBinding()
}

export const selectEnvironmentSaveImportFiles = async (): Promise<environment.EnvironmentPathSelection> => {
  assertEnvironmentBridgeReady('SelectSaveImportFiles')
  return selectSaveImportFilesBinding()
}

export const selectEnvironmentSaveRootDirectory = async (): Promise<environment.EnvironmentPathSelection> => {
  assertEnvironmentBridgeReady('SelectSaveRootDirectory')
  return selectSaveRootDirectoryBinding()
}

export const selectEnvironmentExportDirectory = async (): Promise<environment.EnvironmentPathSelection> => {
  assertEnvironmentBridgeReady('SelectEnvironmentExportDirectory')
  return selectEnvironmentExportDirectoryBinding()
}

export const setEnvironmentRootDirectory = async (
  kind: string,
  path: string,
): Promise<environment.EnvironmentRootDirectoryResult> => {
  assertEnvironmentBridgeReady('SetEnvironmentRootDirectory')
  return setEnvironmentRootDirectoryBinding(kind, path)
}

