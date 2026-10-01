import { computed, onBeforeUnmount, ref } from 'vue'

export type Scenario = 'empty' | 'dll' | 'zip' | 'folder' | 'dependency' | 'conflict'
export type PublishState = 'editing' | 'uploading' | 'interrupted' | 'review' | 'rejected' | 'published'

// 原型文件模型只呈现相对路径及识别结果，不读取 DLL 或执行实际上传。
export interface IntakeFile {
  readonly path: string
  readonly bytes: number
  readonly category: 'payload' | 'excluded' | 'reference' | 'pending'
  readonly reason?: string
}

export interface MarketPackage {
  readonly id: string
  readonly name: string
  readonly author: string
  readonly version: string
  readonly description: string
  readonly kind: 'mod' | 'pack'
  readonly icon?: string
  readonly source: 'box' | 'thunderstore'
}

export const PACKAGES: readonly MarketPackage[] = [
  { id: 'starter', name: '轻量物流整合包', author: '星际工坊', version: '1.2.0', description: '物流管理、建造辅助和信息面板。保留原版生产节奏。', kind: 'pack', source: 'box' },
  { id: 'panel', name: '星际生产面板', author: 'DarkSKL', version: '1.0.0', description: '集中查看产线效率与星球资源，快速定位生产瓶颈。', kind: 'mod', source: 'box' },
  { id: 'coop', name: '联机开荒整合包', author: '星际工坊', version: '2.0.1', description: '面向多人合作的固定版本组合与共享配置。', kind: 'pack', source: 'box' },
  { id: 'galactic', name: 'GalacticScale', author: 'Galactic_Scale', version: '2.80.1', description: '自定义恒星、行星与宇宙规模。', kind: 'mod', source: 'thunderstore', icon: 'https://ccdn.thunderstore.io/live/repository/icons/Galactic_Scale-GalacticScale-2.80.1.png' },
  { id: 'collector', name: 'CollectorDeployer', author: 'cat_yummi', version: '1.1.0', description: '自动部署轨道采集器。', kind: 'mod', source: 'thunderstore', icon: 'https://ccdn.thunderstore.io/live/repository/icons/cat_yummi-CollectorDeployer-1.1.0.png' },
]

const payload = (path: string, bytes: number): IntakeFile => ({ path, bytes, category: 'payload' })
const baseFiles: readonly IntakeFile[] = [
  payload('BepInEx/plugins/StellarPanel/StellarPanel.dll', 428032),
  payload('BepInEx/plugins/StellarPanel/assets/panel.bundle', 2476032),
  payload('BepInEx/plugins/StellarPanel/lang/zh-CN.json', 18432),
  payload('BepInEx/plugins/StellarPanel/lang/en-US.json', 16000),
  payload('BepInEx/plugins/StellarPanel/LICENSE', 1080),
  { path: 'BepInEx/config/account.cfg', bytes: 650, category: 'excluded', reason: '含账号凭证' },
  { path: 'BepInEx/config/StellarPanel.cfg', bytes: 420, category: 'excluded', reason: '未识别的配置，默认不分享' },
  { path: 'BepInEx/LogOutput.log', bytes: 154880, category: 'excluded', reason: '运行日志' },
  { path: 'xiaoye97-BepInEx-5.4.17', bytes: 0, category: 'reference', reason: 'Thunderstore 精确依赖引用' },
]

export function useMarketPrototype() {
  const view = ref<'market' | 'publish' | 'mine'>('market')
  const source = ref<'box' | 'thunderstore'>('box')
  const search = ref('')
  const kind = ref('all')
  const scenario = ref<Scenario>('folder')
  const files = ref<IntakeFile[]>([...baseFiles])
  const name = ref('星际生产面板')
  const version = ref('1.0.0')
  const description = ref('集中查看产线效率与星球资源，快速定位生产瓶颈。')
  const contentKind = ref('mod')
  const dependencyStatus = ref<'known' | 'unknown' | 'none'>('known')
  const dependency = ref('')
  const resolvedConflict = ref(false)
  const fileSearch = ref('')
  const showExcluded = ref(false)
  const state = ref<PublishState>('editing')
  const progress = ref(0)
  const loggedIn = ref(false)
  const showLogin = ref(false)
  const loginName = ref('')
  const inlineError = ref('')
  const detail = ref<MarketPackage | null>(null)
  const installState = ref<'idle' | 'installing' | 'installed' | 'blocked' | 'failed'>('idle')
  const isolatedProfiles = ref(false)
  const serverOffline = ref(false)
  const interruptUpload = ref(false)
  const installFailure = ref(false)
  const notice = ref('')
  let uploadTimer: ReturnType<typeof setInterval> | undefined
  let installTimer: ReturnType<typeof setTimeout> | undefined

  const catalog = computed(() => PACKAGES.filter(item => item.source === source.value &&
    (kind.value === 'all' || item.kind === kind.value) &&
    `${item.name} ${item.author} ${item.description}`.toLowerCase().includes(search.value.trim().toLowerCase())))
  const included = computed(() => files.value.filter(file => file.category === 'payload'))
  const excluded = computed(() => files.value.filter(file => file.category === 'excluded'))
  const references = computed(() => files.value.filter(file => file.category === 'reference'))
  const pending = computed(() => files.value.filter(file => file.category === 'pending'))
  const visibleFiles = computed(() => files.value.filter(file =>
    (showExcluded.value || file.category !== 'excluded') && file.path.toLowerCase().includes(fileSearch.value.toLowerCase())))
  const totalBytes = computed(() => included.value.reduce((total, file) => total + file.bytes, 0))
  const issues = computed(() => {
    const result: string[] = []
    if (!files.value.length) result.push('请添加模组文件或文件夹。')
    if (pending.value.length) result.push('文件识别尚未完成，不能提交。')
    if (dependencyStatus.value === 'unknown' && !dependency.value.trim()) result.push('无法确定 DLL 的依赖，请补充依赖或确认不需要额外依赖。')
    if (scenario.value === 'conflict' && !resolvedConflict.value) result.push('2 个文件将安装到同一路径，请选择保留的文件。')
    if (!name.value.trim()) result.push('请填写名称。')
    if (!/^\d+\.\d+\.\d+$/.test(version.value)) result.push('版本格式应为 1.0.0。')
    if (!description.value.trim()) result.push('请填写说明。')
    return result
  })

  function setScenario(next: Scenario): void {
    clearInterval(uploadTimer)
    scenario.value = next
    state.value = 'editing'
    progress.value = 0
    inlineError.value = ''
    resolvedConflict.value = false
    dependency.value = ''
    dependencyStatus.value = next === 'dll' || next === 'dependency' ? 'unknown' : 'known'
    files.value = next === 'empty' ? [] : next === 'dll' || next === 'dependency'
      ? [payload('StellarPanel.dll', 428032)] : [...baseFiles]
    if (next === 'zip') {
      files.value = [payload('plugins/StellarPanel/StellarPanel.dll', 428032), payload('plugins/StellarPanel/assets/panel.bundle', 2476032),
        { path: 'xiaoye97-BepInEx-5.4.17', bytes: 0, category: 'reference', reason: '从 manifest.json 读取' }]
    }
    if (next === 'conflict') files.value.push(payload('another-package/StellarPanel.dll → BepInEx/plugins/StellarPanel/StellarPanel.dll', 580000))
    view.value = 'publish'
  }

  // 浏览器仅收集真实文件名与相对目录；归类为“待识别”，避免伪装成后端解析成功。
  function addLocalFiles(selected: FileList | null): void {
    if (!selected?.length) return
    const next = Array.from(selected).map((file): IntakeFile => ({
      path: file.webkitRelativePath || file.name, bytes: file.size, category: 'pending', reason: '等待客户端识别服务',
    }))
    files.value.push(...next)
    state.value = 'editing'
    inlineError.value = ''
  }

  function startUpload(): void {
    state.value = 'uploading'
    clearInterval(uploadTimer)
    uploadTimer = setInterval(() => {
      progress.value = Math.min(100, progress.value + 10)
      if (interruptUpload.value && progress.value >= 40 && progress.value < 100) {
        clearInterval(uploadTimer)
        state.value = 'interrupted'
      } else if (progress.value >= 100) {
        clearInterval(uploadTimer)
        state.value = 'review'
      }
    }, 180)
  }

  function publish(): void {
    inlineError.value = issues.value[0] || ''
    if (issues.value.length) return
    if (!loggedIn.value) { showLogin.value = true; return }
    startUpload()
  }

  function continueAfterLogin(): void {
    if (!loginName.value.trim()) return
    loggedIn.value = true
    showLogin.value = false
    publish()
  }

  function install(): void {
    if (!detail.value || installState.value === 'installing') return
    if (detail.value.kind === 'pack' && !isolatedProfiles.value) { installState.value = 'blocked'; return }
    installState.value = 'installing'
    clearTimeout(installTimer)
    installTimer = setTimeout(() => { installState.value = installFailure.value ? 'failed' : 'installed' }, 1000)
  }

  function cancelInstall(): void {
    clearTimeout(installTimer)
    installState.value = 'idle'
    notice.value = '安装已取消，原有环境保留。'
  }

  onBeforeUnmount(() => { clearInterval(uploadTimer); clearTimeout(installTimer) })
  return { view, source, search, kind, scenario, files, name, version, description, contentKind,
    dependencyStatus, dependency, resolvedConflict, fileSearch, showExcluded, state, progress,
    loggedIn, showLogin, loginName, inlineError, detail, installState, isolatedProfiles, serverOffline,
    interruptUpload, installFailure, notice, catalog, included, excluded, references, visibleFiles, totalBytes, issues,
    setScenario, addLocalFiles, publish, continueAfterLogin, startUpload, install, cancelInstall }
}

export function formatBytes(bytes: number): string {
  return bytes >= 1048576 ? `${(bytes / 1048576).toFixed(2)} MB` : `${Math.max(1, Math.round(bytes / 1024))} KB`
}
