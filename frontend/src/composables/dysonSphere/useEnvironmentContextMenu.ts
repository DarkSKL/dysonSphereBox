import type {DropdownOption} from 'naive-ui'
import {computed, nextTick, ref} from 'vue'
import {contract, environment} from '../../../wailsjs/go/models'
import type {useEnvironmentDialogs} from './useEnvironmentDialogs'
import type {useEnvironmentState} from './useEnvironmentState'
import type {useInstalledModState} from './useInstalledModState'

export type EnvContextMenuKind =
  | 'mod-space'
  | 'mod'
  | 'market'
  | 'blueprint-space'
  | 'blueprint-folder'
  | 'blueprint-file'
  | 'save-space'
  | 'save'

export interface EnvContextMenuTarget {
  kind: EnvContextMenuKind
  mod?: environment.EnvironmentModItem
  pack?: contract.InstalledPackageSummary
  blueprint?: environment.EnvironmentBlueprintItem
  save?: environment.EnvironmentSaveItem
}

type EnvironmentState = ReturnType<typeof useEnvironmentState>
type EnvironmentDialogs = ReturnType<typeof useEnvironmentDialogs>
type InstalledModState = ReturnType<typeof useInstalledModState>

// =============== 资源页右键菜单 ===============
// 说明：按右键目标生成菜单项，并把选择分发到资源库、蓝图、存档和市场模组动作。
// 边界：菜单只收集意图；所有真实文件写入、冲突判断和回滚仍由后端服务负责。
export const useEnvironmentContextMenu = (
  env: EnvironmentState,
  dialogs: EnvironmentDialogs,
  installed: InstalledModState,
) => {
  const menu = ref<{visible: boolean, x: number, y: number, target: EnvContextMenuTarget | null}>({
    visible: false,
    x: 0,
    y: 0,
    target: null,
  })

  const needsSyncRepair = (state?: string): boolean => state === 'conflict' || state === 'missing'

  const syncRepairOptions = (prefix: 'mod' | 'blueprint'): DropdownOption[] => [
    {type: 'divider', key: `${prefix}-repair-divider`},
    {label: '用游戏侧更新客户端镜像', key: `${prefix}-update-client`},
    {label: '用客户端镜像恢复游戏侧', key: `${prefix}-restore-game`},
    {label: '解除同步记录', key: `${prefix}-unlink-sync`},
  ]

  const modMaintenanceOptions = (): DropdownOption[] => [
    {type: 'divider', key: 'mod-maintenance-divider'},
    {label: '同步本机模组', key: 'mod-sync'},
    {label: '打开资源库目录', key: 'mod-open-dir'},
    {label: '打开游戏插件目录', key: 'mod-open-real-dir'},
  ]

  const blueprintMaintenanceOptions = (): DropdownOption[] => [
    {type: 'divider', key: 'blueprint-maintenance-divider'},
    {label: '同步本机蓝图', key: 'blueprint-sync'},
    {label: '打开资源库目录', key: 'blueprint-open-dir'},
    {label: '打开游戏蓝图目录', key: 'blueprint-open-real-dir'},
    {label: '选择蓝图目录', key: 'blueprint-select-dir'},
  ]

  const saveMaintenanceOptions = (): DropdownOption[] => [
    {type: 'divider', key: 'save-maintenance-divider'},
    {label: '重新扫描存档', key: 'save-sync'},
    {label: '打开存档目录', key: 'save-open-dir'},
    {label: '选择存档目录', key: 'save-select-dir'},
  ]

  const options = computed<DropdownOption[]>(() => {
    const target = menu.value.target
    if (!target) {
      return []
    }
    switch (target.kind) {
      case 'mod-space':
        return [
          {label: '导入 zip 压缩包', key: 'mod-import-zip'},
          {label: '导入模组目录', key: 'mod-import-dir'},
          ...modMaintenanceOptions(),
        ]
      case 'mod': {
        const managed = Boolean(target.mod?.installerManaged)
        return [
          {label: '查看详情', key: 'mod-detail'},
          ...(!managed && needsSyncRepair(target.mod?.state) ? syncRepairOptions('mod') : []),
          ...modMaintenanceOptions(),
          {type: 'divider', key: 'mod-danger-divider'},
          {label: managed ? '删除资源库副本' : '删除模组', key: 'mod-delete'},
        ]
      }
      case 'market':
        return [
          {label: '查看详情', key: 'market-detail'},
          {label: '打开游戏插件目录', key: 'mod-open-real-dir'},
          {type: 'divider', key: 'market-danger-divider'},
          {label: '卸载模组', key: 'market-uninstall'},
        ]
      case 'blueprint-space':
        return [
          {label: '新建文件夹', key: 'blueprint-new-folder'},
          {label: '导入文件', key: 'blueprint-import'},
          {label: '快速导入', key: 'blueprint-quick-import'},
          ...blueprintMaintenanceOptions(),
        ]
      case 'blueprint-folder':
        return [
          {label: '进入文件夹', key: 'blueprint-enter-folder'},
          {label: '重命名', key: 'blueprint-rename'},
          ...blueprintMaintenanceOptions(),
          {type: 'divider', key: 'blueprint-folder-danger-divider'},
          {label: '删除文件夹', key: 'blueprint-delete'},
        ]
      case 'blueprint-file':
        return [
          {label: '查看详情', key: 'blueprint-detail'},
          {label: '重命名', key: 'blueprint-rename'},
          {label: '导出蓝图', key: 'blueprint-export'},
          ...(needsSyncRepair(target.blueprint?.state) ? syncRepairOptions('blueprint') : []),
          ...blueprintMaintenanceOptions(),
          {type: 'divider', key: 'blueprint-file-danger-divider'},
          {label: '删除蓝图', key: 'blueprint-delete'},
        ]
      case 'save-space':
        return [
          {label: '导入存档', key: 'save-import'},
          ...saveMaintenanceOptions(),
        ]
      default:
        return [
          {label: '重命名', key: 'save-rename'},
          {label: '导出存档', key: 'save-export'},
          ...saveMaintenanceOptions(),
          {type: 'divider', key: 'save-danger-divider'},
          {label: '删除存档', key: 'save-delete'},
        ]
    }
  })

  const open = (event: MouseEvent, target: EnvContextMenuTarget): void => {
    event.preventDefault()
    event.stopPropagation()
    menu.value.visible = false
    menu.value.x = event.clientX
    menu.value.y = event.clientY
    menu.value.target = target
    void nextTick(() => {
      menu.value.visible = true
    })
  }

  const close = (): void => {
    menu.value.visible = false
  }

  // =============== 可复用弹窗入口 ===============
  // 说明：卡片点击和右键菜单共用的输入、确认弹窗，保持两种入口的文案与行为一致。
  const openSaveRename = (save: environment.EnvironmentSaveItem): void => {
    dialogs.openTextDialog({
      title: '重命名存档',
      label: '存档名称',
      value: save.name,
      placeholder: '输入新的存档名称',
      positiveText: '重命名',
      submit: async (value) => {
        if (value !== save.name) {
          await env.renameSave(save.id, value)
        }
      },
    })
  }

  const openMarketUninstall = (pack: contract.InstalledPackageSummary): void => {
    dialogs.openConfirmDialog({
      title: '卸载模组',
      content: `卸载“${pack.displayName || pack.name}”？将移除不再被其它模组引用的文件，并删除本地安装记录。`,
      positiveText: '卸载',
      isDanger: true,
      submit: async () => {
        await installed.uninstallInstalledPackage(pack.packageKey)
        await env.loadMods()
      },
    })
  }

  const openNewBlueprintFolder = (): void => {
    dialogs.openTextDialog({
      title: '新建文件夹',
      label: '文件夹名称',
      value: '新建文件夹',
      placeholder: '输入文件夹名称',
      positiveText: '创建',
      submit: async (value) => env.createBlueprintFolder(value),
    })
  }

  const openBlueprintQuickImport = (): void => {
    dialogs.openQuickImportDialog(async (name, code) => env.quickImportBlueprint(name, code))
  }

  const openModDelete = (mod: environment.EnvironmentModItem): void => {
    dialogs.openConfirmDialog({
      title: mod.installerManaged ? '删除资源库副本' : '删除模组',
      content: mod.installerManaged
        ? `删除“${mod.name}”的资源库副本？游戏侧文件由模组市场管理，不会被删除。`
        : `删除模组资源“${mod.name}”？已启用时会先安全禁用。`,
      positiveText: '删除',
      isDanger: true,
      submit: async () => env.deleteMod(mod),
    })
  }

  const openSyncRepairConfirm = (
    action: 'update-client' | 'restore-game' | 'unlink-sync',
    itemId: string,
    kind: 'mod' | 'blueprint',
    name: string,
  ): void => {
    if (action === 'update-client') {
      dialogs.openConfirmDialog({
        title: '更新客户端镜像',
        content: `使用游戏侧文件更新“${name}”的客户端镜像？`,
        positiveText: '更新镜像',
        isDanger: false,
        submit: async () => env.updateClientMirrorFromGame(itemId, kind),
      })
      return
    }
    if (action === 'restore-game') {
      dialogs.openConfirmDialog({
        title: '恢复游戏侧文件',
        content: `使用客户端镜像覆盖“${name}”的游戏侧文件？`,
        positiveText: '恢复游戏侧',
        isDanger: false,
        submit: async () => env.restoreGameFromClientMirror(itemId, kind),
      })
      return
    }
    dialogs.openConfirmDialog({
      title: '解除同步记录',
      content: `解除“${name}”的同步记录？客户端镜像和游戏侧文件都会保留，之后需要手动处理游戏侧文件。`,
      positiveText: '解除同步',
      isDanger: true,
      submit: async () => env.unlinkSyncRecord(itemId, kind),
    })
  }

  // =============== 菜单分发 ===============
  // 说明：危险或覆盖类操作统一先进入确认弹窗；目录、导入和详情类操作直接执行。
  const handleSelect = async (key: string | number): Promise<void> => {
    const action = String(key)
    const target = menu.value.target
    close()
    if (!target) {
      return
    }
    const {mod, pack, blueprint, save} = target

    switch (action) {
      case 'mod-import-zip':
        return env.selectAndImportModFiles()
      case 'mod-import-dir':
        return env.selectAndImportModDirectory()
      case 'mod-sync':
        return env.syncEnvironmentResources('mod')
      case 'mod-open-dir':
        return env.openModLibraryDir()
      case 'mod-open-real-dir':
        return env.openModRealDir()
      case 'blueprint-new-folder':
        return openNewBlueprintFolder()
      case 'blueprint-import':
        return env.selectAndImportBlueprintFiles()
      case 'blueprint-quick-import':
        return openBlueprintQuickImport()
      case 'blueprint-sync':
        return env.syncEnvironmentResources('blueprint')
      case 'blueprint-open-dir':
        return env.openBlueprintLibraryDir()
      case 'blueprint-open-real-dir':
        return env.openBlueprintRealDir()
      case 'blueprint-select-dir':
        return env.selectAndSetEnvironmentRootDirectory('blueprint')
      case 'save-import':
        return env.selectAndImportSaves()
      case 'save-sync':
        return env.syncEnvironmentResources('save')
      case 'save-open-dir':
        return env.openSaveDir()
      case 'save-select-dir':
        return env.selectAndSetEnvironmentRootDirectory('save')
    }

    if (mod) {
      if (action === 'mod-detail') {
        return env.loadModDetail(mod.id)
      }
      if (action === 'mod-delete') {
        return openModDelete(mod)
      }
      if (action.startsWith('mod-')) {
        return openSyncRepairConfirm(action.slice(4) as 'update-client' | 'restore-game' | 'unlink-sync', mod.id, 'mod', mod.name)
      }
    }
    if (pack) {
      if (action === 'market-detail') {
        return installed.openInstalledPackage(pack.packageKey)
      }
      if (action === 'market-uninstall') {
        return openMarketUninstall(pack)
      }
    }
    if (blueprint) {
      switch (action) {
        case 'blueprint-detail':
          return env.loadBlueprintDetail(blueprint.id)
        case 'blueprint-enter-folder':
          return env.enterBlueprintFolder(blueprint.id)
        case 'blueprint-export':
          return env.selectAndExportBlueprint(blueprint.id)
        case 'blueprint-rename':
          return dialogs.openTextDialog({
            title: blueprint.type === 'folder' ? '重命名文件夹' : '重命名蓝图',
            label: '名称',
            value: blueprint.name,
            placeholder: '输入新的名称',
            positiveText: '重命名',
            submit: async (value) => {
              if (value !== blueprint.name) {
                await env.renameBlueprint(blueprint.id, value)
              }
            },
          })
        case 'blueprint-delete':
          return dialogs.openConfirmDialog({
            title: blueprint.type === 'folder' ? '删除文件夹' : '删除蓝图',
            content: `删除蓝图资源“${blueprint.name}”？已启用蓝图会先安全禁用。`,
            positiveText: '删除',
            isDanger: true,
            submit: async () => env.deleteBlueprint(blueprint.id),
          })
        case 'blueprint-update-client':
        case 'blueprint-restore-game':
        case 'blueprint-unlink-sync':
          return openSyncRepairConfirm(action.slice(10) as 'update-client' | 'restore-game' | 'unlink-sync', blueprint.id, 'blueprint', blueprint.name)
      }
    }
    if (save) {
      switch (action) {
        case 'save-rename':
          return openSaveRename(save)
        case 'save-export':
          return env.selectAndExportSave(save.id)
        case 'save-delete':
          return dialogs.openConfirmDialog({
            title: '删除存档',
            content: `删除存档“${save.name}”？存档及同名联机附加数据会一起移入应用快照目录。`,
            positiveText: '删除存档',
            isDanger: true,
            submit: async () => env.deleteSave(save.id),
          })
      }
    }
  }

  return {
    menu,
    options,
    open,
    close,
    handleSelect,
    openSaveRename,
    openMarketUninstall,
    openNewBlueprintFolder,
    openBlueprintQuickImport,
  }
}
