import {computed, onActivated, onDeactivated, onScopeDispose, ref, shallowRef, type ComputedRef, type Ref} from 'vue'
import {useMessage} from 'naive-ui'
import type {r2modman} from '../../../wailsjs/go/models'
import {
  getR2ModmanProfile,
  listR2ModmanProfiles,
  manageR2ModmanProfile,
  removeR2ModmanMod,
  selectR2ModmanProfileDirectory,
  setR2ModmanModEnabled,
  syncR2ModmanProfiles,
} from '../../services/r2modman'
import {formatErrorMessage} from '../../utils/format'

// =============== 页面间快照复用 ===============
// 说明：路由或标签重新挂载时先复用上次画面，再静默读取盒子数据库，避免卡片和数量闪空。
// 边界：这里只保留会话内副本，持久数据仍由后端同步快照拥有，不触发实盘扫描。
let lastSnapshot: r2modman.ProfileState | null = null
let lastProfiles: r2modman.Profile[] = []

// =============== 面板状态契约 ===============
// 说明：领域 DTO 直接来自 Wails；loading 仅标识缓存读取，busy 只标识显式同步和写入。
interface R2ModmanState {
  profile: Ref<r2modman.Profile | null>
  mods: Ref<r2modman.Mod[]>
  profiles: Ref<r2modman.Profile[]>
  loading: Ref<boolean>
  hasLoaded: Ref<boolean>
  loadError: Ref<string>
  busy: Ref<boolean>
  operatingModName: Ref<string>
  profileReady: Ref<boolean>
  summary: ComputedRef<{total: number, enabled: number, disabled: number}>
  filteredMods: ComputedRef<r2modman.Mod[]>
  refresh: () => Promise<void>
  syncProfiles: () => Promise<boolean>
  selectProfile: (path: string) => Promise<boolean>
  selectDirectory: () => Promise<boolean>
  setModEnabled: (path: string, name: string, enabled: boolean, confirmedNames?: string[]) => Promise<boolean>
  removeMod: (path: string, name: string) => Promise<boolean>
}

// =============== 持久快照与展示状态 ===============
// 说明：页面进入仅恢复盒子缓存；搜索和汇总不会触发实盘检索。
export const useR2ModmanState = (query: Readonly<Ref<string>>): R2ModmanState => {
  const message = useMessage()
  const profile = shallowRef<r2modman.Profile | null>(lastSnapshot?.profile ?? null)
  const mods = shallowRef<r2modman.Mod[]>(lastSnapshot?.mods ?? [])
  const profiles = shallowRef<r2modman.Profile[]>(lastProfiles)
  const loading = ref(!lastSnapshot)
  const hasLoaded = ref(Boolean(lastSnapshot))
  const loadError = ref('')
  const busy = ref(false)
  const operatingModName = ref('')
  const profileReady = ref(Boolean(lastSnapshot))
  let disposed = false
  let active = true
  let selectionVersion = 0
  let cacheVersion = 0
  let inFlight: Promise<boolean> | null = null
  let refreshing: Promise<void> | null = null

  const summary = computed(() => {
    const total = mods.value.length
    const enabled = mods.value.filter((mod) => mod.enabled).length
    return {total, enabled, disabled: total - enabled}
  })
  const filteredMods = computed(() => {
    const keyword = query.value.trim().toLocaleLowerCase()
    if (!keyword) return mods.value
    return mods.value.filter((mod) => [
      mod.name, mod.displayName, mod.author, mod.version, mod.description,
      mod.websiteUrl, mod.warning, mod.disableReason, mod.removeReason, ...(mod.dependencies ?? []),
    ].join(' ').toLocaleLowerCase().includes(keyword))
  })

  // 说明：缓存列表保留当前快照条目，保证父工具栏的选中路径始终有对应选项。
  const applyProfiles = (items: r2modman.Profile[]): void => {
    if (disposed) return
    const current = profile.value
    if (!current) { profiles.value = items; lastProfiles = items; return }
    const exists = items.some((item) => item.path === current.path)
    profiles.value = exists
      ? items.map((item) => item.path === current.path ? current : item)
      : [...items, current]
    lastProfiles = profiles.value
  }

  // 说明：空指针按运行时值判断；操作返回后保留后端最新数量，不依赖 Wails nullable 声明。
  const applyState = (state: r2modman.ProfileState): void => {
    if (disposed) return
    lastSnapshot = state
    const current = state?.profile
    profile.value = current?.path ? current : null
    mods.value = profile.value ? state.mods ?? [] : []
    profileReady.value = true
    hasLoaded.value = true
    loadError.value = ''
    applyProfiles(profiles.value)
  }

  const showFailure = (label: string, cause: unknown): void => {
    if (!disposed && active) message.error(`${label}失败：${formatErrorMessage(cause)}`)
  }

  // =============== 非阻塞缓存读取 ===============
  // 说明：合并挂载、激活及父页面的刷新，不使用写入锁或清空旧列表。
  // 边界：读取若跨越写操作则丢弃旧响应，等待操作结束后补读；不会调用实盘同步。
  const refresh = (): Promise<void> => {
    if (disposed) return Promise.resolve()
    if (refreshing) return refreshing
    loading.value = true
    refreshing = (async (): Promise<void> => {
      while (!disposed) {
        if (inFlight) await inFlight
        if (disposed) return
        const version = cacheVersion
        const [snapshot, entries] = await Promise.allSettled([
          getR2ModmanProfile(), listR2ModmanProfiles(),
        ])
        if (disposed) return
        if (version !== cacheVersion || busy.value) continue
        if (snapshot.status === 'fulfilled') applyState(snapshot.value)
        if (entries.status === 'fulfilled') applyProfiles(entries.value ?? [])
        if (snapshot.status === 'rejected') {
          if (!hasLoaded.value) loadError.value = formatErrorMessage(snapshot.reason)
          showFailure('读取 Profile 缓存', snapshot.reason)
        }
        if (entries.status === 'rejected') showFailure('读取 Profile 列表缓存', entries.reason)
        return
      }
    })().finally(() => {
      loading.value = false
      refreshing = null
    })
    return refreshing
  }

  // =============== 显式同步与写入串行化 ===============
  // 说明：只有用户同步、选择和模组修改占用 busy；失败 toast 后补读缓存，不重放写入。
  const run = (label: string, action: () => Promise<void>): Promise<boolean> => {
    if (busy.value || disposed || !active) return Promise.resolve(false)
    cacheVersion += 1
    busy.value = true
    const pending = (async (): Promise<boolean> => {
      try {
        await action()
        return !disposed
      } catch (cause) {
        showFailure(label, cause)
        return false
      } finally {
        cacheVersion += 1
        busy.value = false
        operatingModName.value = ''
      }
    })()
    inFlight = pending
    void pending.then((success) => {
      if (inFlight === pending) inFlight = null
      if (!success && !disposed) void refresh()
    })
    return pending
  }

  // 说明：显式同步先更新实盘快照，再读取数据库列表；完整成功后提示。
  const syncProfiles = async (): Promise<boolean> => run('同步 Profile', async () => {
    applyState(await syncR2ModmanProfiles())
    const entries = await listR2ModmanProfiles()
    applyProfiles(entries ?? [])
    if (!disposed && active) message.success('Profile 同步成功')
  })

  // 说明：父工具栏选择即切换，已缓存路径由后端直接读取缓存，不额外扫描。
  const selectProfile = async (path: string): Promise<boolean> => run('切换 Profile', async () => {
    if (!path.trim()) throw new Error('请选择一个 Profile。')
    applyState(await manageR2ModmanProfile(path))
  })

  // 说明：手动定位后直接托管；取消、失活或卸载后的目录返回不得继续提交。
  const selectDirectory = async (): Promise<boolean> => {
    const version = selectionVersion
    return run('定位 Profile 目录', async () => {
      const path = await selectR2ModmanProfileDirectory()
      if (!path || disposed || !active || version !== selectionVersion) return
      applyState(await manageR2ModmanProfile(path))
    })
  }

  // =============== 显式目标与后端管理边界 ===============
  // 说明：提交前重新读取当前条目权限，确认框和菜单必须携带原始 Profile 路径。
  const assertManageableMod = (path: string, name: string): r2modman.Mod => {
    if (!profileReady.value || !path || profile.value?.path !== path) {
      throw new Error('当前 Profile 已变化，请重新操作。')
    }
    const mod = mods.value.find((item) => item.name === name)
    if (!mod) throw new Error('此模组已不在当前 Profile 中。')
    if (!mod.canManage) throw new Error(mod.warning || '此模组暂不支持直接修改。')
    return mod
  }

  // 说明：启用依赖联动由后端完成；联动禁用提交确认时捕获的完整范围，由后端重新核对后整体回填。
  const setModEnabled = async (path: string, name: string, enabled: boolean, confirmedNames: string[] = []): Promise<boolean> => (
    run(enabled ? '启用模组' : '禁用模组', async () => {
      const mod = assertManageableMod(path, name)
      if (!enabled && !mod.canDisable) throw new Error(mod.disableReason || '此模组当前无法禁用。')
      operatingModName.value = name
      applyState(await setR2ModmanModEnabled(path, name, enabled, confirmedNames))
    })
  )

  // 说明：只从确认入口移除；依赖者检查、Profile 内备份及持久快照更新由后端完成。
  const removeMod = async (path: string, name: string): Promise<boolean> => run('移除模组', async () => {
    const mod = assertManageableMod(path, name)
    if (!mod.canRemove) throw new Error(mod.removeReason || '此模组不能单独移除。')
    operatingModName.value = name
    applyState(await removeR2ModmanMod(path, name))
  })

  onActivated(() => { active = true })
  onDeactivated(() => { active = false; selectionVersion += 1 })
  onScopeDispose(() => { disposed = true; active = false; selectionVersion += 1; cacheVersion += 1 })

  return {
    profile, mods, profiles, loading, hasLoaded, loadError, busy, operatingModName,
    profileReady, summary, filteredMods, refresh, syncProfiles, selectProfile,
    selectDirectory, setModEnabled, removeMod,
  }
}
