// =============== 蓝图编辑器桥接 ===============
// 职责：注册并调用 dsp-blueprint-editor ESM 预览库，收敛库模块加载、实例生命周期、主题隔离、增量预览状态机与尺寸同步。
// 使用：components/dysonSphere/blueprint/viewer-panel.vue 经 composables/useBlueprintViewer.ts 使用；
//       与 blueprint-preview-host / blueprint-preview-interaction 同层，共同构成预览库的纯 TS 适配层。
// 边界：纯 TS、不依赖 Vue；只通过端口读取宿主快照（容器、蓝图编码、主题）并回调事件，不持有 DOM 之外的 UI 状态。
//       第三方库会改写 html[data-theme]，所有库调用必须经 isolate() 包裹，主题快照恢复不得外移。

import {claimBlueprintPreview} from './blueprint-preview-host'
import {enableBlueprintPreviewInteraction} from './blueprint-preview-interaction'
import type {
  BlueprintLoadOutcome,
  BlueprintViewer as BlueprintViewerInstance,
  PreviewDeltaEvent,
} from '../../../packages/dsp-blueprint-editor/dsp-blueprint-editor.esm.js'

type BlueprintEditorModule = typeof import('../../../packages/dsp-blueprint-editor/dsp-blueprint-editor.esm.js')
// =============== 模型资产映射 ===============
// 说明：库产物里的 models/ 不随宿主构建复制，直接复用应用的 src/data/models 单份模型，
//       用 ?url 让 Vite 接管 GLB 地址（开发原版、打包按所选档位压缩 + 内容哈希），再交给库的映射钩子。
// 边界：键必须是库内部使用的相对路径（models/xxx.glb）；未命中的路径仍回退库的默认拼接。
//       不得在库目录里另存一份模型副本：那会绕过 model-compression 插件，把原版模型整份打进发布包。
const bundledModelUrls: Record<string, string> = {}
for (const [filePath, url] of Object.entries(
  import.meta.glob<string>('../../../data/models/*.glb', {query: '?url', import: 'default', eager: true}),
)) {
  bundledModelUrls[`models/${filePath.split('/').pop()}`] = url
}

// =============== 宿主端口 ===============

/** 主题名：仅应用自己的明暗两态，不与第三方库的 'editor' / 'auto' 混用 */
export type BlueprintEditorThemeMode = 'light' | 'dark'

/** 主题端口：桥接只经此读写主题，不直接依赖 useThemeMode */
export interface BlueprintEditorThemePort {
  /** 当前应用主题 */
  read(): BlueprintEditorThemeMode
  /** 应用主题是否已从配置加载完成；未完成时快照恢复回退到 html 上的既有取值 */
  isReady(): boolean
}

/** 状态快照：宿主据此同步 loading / error / buildingCount 三个模板状态 */
// 说明：不携带 previewStats——统计查询需要走主题隔离，放进状态会让每次状态变化都做一次快照恢复；
//      需要诊断统计时调用 getPreviewStats()。
export interface BlueprintViewerBridgeState {
  loading: boolean
  error: string
  buildingCount: number
}

/** 事件回调：载荷与组件 emit 一一对应，宿主负责映射回组件的 emit */
export interface BlueprintViewerBridgeEvents {
  onReady(payload: {buildingCount: number; blank?: boolean}): void
  onSelect(payload: {index: number | null}): void
  onError(payload: {code: string; message: string}): void
  /** 状态变化（loading / error / buildingCount 任一改变时触发） */
  onStateChange(state: BlueprintViewerBridgeState): void
}

/** 宿主输入快照：全部用读取函数，桥接每次使用时求值，不缓存 Vue 响应式值 */
export interface BlueprintViewerBridgeInputs {
  /** 挂载容器（组件挂载前后都可能为空） */
  container(): HTMLElement | null
  /** 当前蓝图编码与版本号 */
  blueprint(): {code: string; revision: number}
  /** 是否开启点击拾取选中 */
  interactive(): boolean
  /** 期望聚焦的建筑下标（null 表示无请求） */
  focusIndex(): number | null
  /** 当前选中建筑下标集合，实例重建后用于回填选中态 */
  selectedBuildings(): number[]
  /** 建筑数量上限：超出仍渲染，只上报性能提示 */
  maxBuildings(): number
}

/** 桥接宿主：composable 在 setup 中组装一次 */
export interface BlueprintEditorBridgeHost {
  inputs: BlueprintViewerBridgeInputs
  theme: BlueprintEditorThemePort
  events: BlueprintViewerBridgeEvents
}

/** 供组件 defineExpose 的公开面；与拆分前组件的暴露方法完全一致 */
export interface BlueprintViewerBridgeApi {
  selectBuildings(indices: number[]): void
  focusBuilding(index: number | null): void
  resetCamera(): void
  captureImage(): string | null
  setRenderSettings(showRealModels: boolean, maxInstancesPerModel: number): void
  beginPreview(): void
  applyPreviewDelta(event: unknown): boolean
  commitPreview(blueprint: string): unknown
  beginPreviewReplay(): void
  finishPreviewReplay(): void
  finishPreviewClear(): void
  discardPreview(): void
  isReady(): boolean
  getPreviewing(): boolean
  getPreviewStats(): unknown
}

// =============== 空白蓝图 ===============
// 说明：仅包含星球主体、没有任何建筑的合法蓝图；预览器首次加载（还没有任何 code 时）用它先渲染星球主体。
// 边界：增量预览不再用它清场（那会把星球一起重建）；只在分批清场失败时作为兜底重建空场景。
export const BLANK_BLUEPRINT = 'BLUEPRINT:1,10,0,0,0,0,0,0,639273500692970000,0.10.35.29104,%E6%96%B0%E8%93%9D%E5%9B%BE,,,,"H4sIAAAAAAAAA2NiYGDgAGJmBggQAGI2KJuR4T8DwwmoMBtUCAgA9wEkuTQAAAA="7FFF389CC152F6E763D8B8B72DF4D44A'

// =============== 分批清场参数 ===============
// 说明：deleteBuilding 每次调用都会触发一次整场景重建（库文档 4.10 明确"要删很多目标请合并成一次调用"），
//       所以批次数按规模自适应：大场景合并成一次，只有中小场景才拆批，做出"逐批摘掉"的渐进感。
// 边界：单批目标数 = 总栋数 / 批次数；批与批之间隔 CLEAR_TICK_MS，让出一帧给渲染与算法页动画。
const CLEAR_TICK_MS = 200

/**
 * 分批清场的批次数。
 * 说明：栋数越多单次重建越贵（库文档 4.10：3 万级每次重建是秒级开销），分批只会成倍放大总耗时，
 *       因此 6000 栋以上直接一次删完；1200 栋以下才按 300 栋一批拆开，最多 4 批。
 */
const clearBatches = (total: number): number => total > 6000 ? 1 : total > 1200 ? 2 : Math.min(4, Math.max(1, Math.ceil(total / 300)))

// =============== 预览器模块加载 ===============
// 说明：按需导入本地 ESM 产物；库以 import.meta.url 定位同目录的 models/、img/，开发期无需额外配置。
//       导入完成后立即注册模型地址映射，保证首次创建实例前钩子已就位。
// 边界：缓存导入 Promise 避免重复求值；失败时清空缓存，让下一次加载可以重试。
let blueprintEditorModule: Promise<BlueprintEditorModule> | null = null

const ensureBlueprintEditor = (): Promise<BlueprintEditorModule> => {
  if (!blueprintEditorModule) {
    blueprintEditorModule = import('../../../packages/dsp-blueprint-editor/dsp-blueprint-editor.esm.js')
      .then((module) => {
        // 未命中即返回 null → 回退库的默认拼接
        module.configureAssetUrlMapper(path => bundledModelUrls[path] ?? null)
        return module
      })
      .catch((error: unknown) => {
        blueprintEditorModule = null
        throw error
      })
  }
  return blueprintEditorModule
}

// =============== 应用主题隔离 ===============
// 说明：第三方蓝图库加载时会写入 html[data-theme] 与自己的主题缓存，这里在每次外部调用后恢复盒子当前主题。
// 边界：只恢复应用根主题和蓝图库自己的主题缓存，不接管用户在设置页选择主题的业务状态。
const blueprintEditorThemeStorageKey = 'dsp-editor-theme'

interface AppThemeSnapshot {
  readonly rootThemeMode: BlueprintEditorThemeMode | null
  readonly blueprintEditorStoredTheme: string | null
}

const readRootThemeMode = (): BlueprintEditorThemeMode | null => {
  const rootTheme = document.documentElement.dataset.theme
  if (rootTheme === 'dark' || rootTheme === 'light') {
    return rootTheme
  }

  return null
}

// =============== 桥接实现 ===============

/**
 * 蓝图预览库桥接。
 *
 * 生命周期由宿主调用：
 *   activate()  → 建立 ResizeObserver、预加载 ESM、加载当前蓝图
 *   deactivate() / dispose() → 释放实例与全局资源（组件 onDeactivated / onBeforeUnmount）
 * 输入同步（宿主 watch 中调用）：
 *   load()            → code / revision 变化
 *   selectBuildings() → selectedBuildings 变化
 *   focusRequested()  → focusRevision 变化
 *   setTheme()        → 应用主题变化
 */
export class BlueprintEditorBridge implements BlueprintViewerBridgeApi {
  private readonly host: BlueprintEditorBridgeHost

  private viewer: BlueprintViewerInstance | null = null
  private loading = false
  private error = ''
  private buildingCount = 0

  private resizeObserver: ResizeObserver | null = null
  private pendingLoadFrame = 0
  private pendingResizeFrame = 0
  private resizeSettleTimer = 0
  private lastSize = {width: 0, height: 0}
  private active = true
  private loadVersion = 0
  private releaseHost: (() => void) | null = null
  private releaseInteraction: (() => void) | null = null

  // =============== 增量预览快照 ===============
  // 说明：库的 beginPreview 不会清空正式场景（保留上一次成功结果）。为避免旧蓝图与预览层叠加，
  //       这里在 beginPreview 前保存旧蓝图编码与建筑数，用空白蓝图清场；commitPreview 后丢弃快照，
  //       discardPreview 时用快照恢复旧蓝图。
  // 边界：只在 viewer 实例就绪时操作；快照随 commit/discard 清除，释放实例时统一清空。
  private previewSnapshot: {code: string; buildingCount: number} | null = null
  // commitPreview 成功后正式场景已替换为新蓝图，code 变化触发的全量重载应跳过，避免闪白与多余加载遮罩
  private skipNextCodeReload = false

  // =============== 量差动画提交闸门 ===============
  // 说明：从本轮开始重放差量起，到转发层在动画收尾时调用 finishPreviewReplay 为止，到达的 commitPreview（完整蓝图）先缓存；
  //       这段时间内的 code 变化同理跳过，避免整份蓝图提前覆盖动画画面。
  // 边界：闸门必须在动画真正开始之前就合上（beginPreviewReplay），否则 generate() 的完整蓝图会在转发层等待算法舞台的空档里
  //       直接落到场景上，重放时的 add 全部变成"重复新增"被库拒绝，动画就放不出来了；没有重放过差量时 commitPreview 立即提交，
  //       beginPreview / discardPreview / 释放实例都会清空缓存与闸门。
  private replayingDeltas = false
  private replayPending = false
  private pendingCommitBlueprint: string | null = null

  // =============== 分批清场计划 ===============
  // 说明：ids 是开始清场时快照的整批建筑 ID；cursor 是已删数量；batch 是单批上限；timer 是下一批的定时器。
  // 边界：ids 是快照，但 deleteBuilding(mode=1) 按建筑 ID 定位、不依赖数组下标，所以后续删除造成的下标重排不影响；
  //       任何会整份替换场景的路径（提交、丢弃、卸载、全量重载）都必须先 stopClear，否则会按旧 ID 删到新场景上。
  private clearPlan: {ids: number[]; cursor: number; batch: number; timer: number} | null = null

  // =============== 全站实例协调 ===============
  // 说明：claimBlueprintPreview 用函数身份判断"是否同一个预览实例"，每次都传新闭包会被误判为另一个实例、
  //       进而触发上一次的释放动作（拆掉刚建好的 ResizeObserver）。这里固定一个引用，语义与拆分前的
  //       组件内 disposeViewer 一致。
  private readonly disposeRef = (): void => this.teardown()

  constructor(host: BlueprintEditorBridgeHost) {
    this.host = host
  }

  // =============== 生命周期 ===============

  /** 激活：标记活跃、建立尺寸观察、预加载模块并加载当前蓝图；nextTick 由宿主负责 */
  async activate(): Promise<void> {
    this.active = true
    if (!this.resizeObserver) {
      this.resizeObserver = new ResizeObserver(() => {
        if (!this.viewer && this.hasRenderableSize()) void this.loadCurrentBlueprint()
        else this.resizeViewer()
      })
      const container = this.host.inputs.container()
      if (container) this.resizeObserver.observe(container)
    }
    // 先启动预览器模块预加载，再加载蓝图（无编码时加载空白蓝图渲染星球主体）
    void this.preloadBlueprintEditor()
    await this.loadCurrentBlueprint()
  }

  /** 挂起（onDeactivated）：停止加载并完整释放实例 */
  deactivate(): void {
    this.active = false
    this.teardown()
  }

  /** 释放（onBeforeUnmount）：语义与 deactivate 一致 */
  dispose(): void {
    this.active = false
    this.teardown()
  }

  // =============== 输入同步 ===============

  /** code / revision 变化入口：空编码清场；跳过标记与量差动画期间不重载 */
  load(): void {
    const {code} = this.host.inputs.blueprint()
    if (!code) {
      this.teardown()
      this.setBuildingCount(0)
      return
    }
    // commitPreview 成功后正式场景已替换为新蓝图，跳过此次全量重载，避免闪白与加载遮罩；
    // 量差待重放 / 正在重放期间同理：完整蓝图已缓存，等 finishPreviewReplay 在动画收尾时替换场景。
    if (this.skipNextCodeReload || this.replayingDeltas || this.replayPending) {
      this.skipNextCodeReload = false
      return
    }
    void this.loadCurrentBlueprint()
  }

  /** 主题变化：转发给库的 setTheme */
  setTheme(mode: BlueprintEditorThemeMode): void {
    if (!this.viewer) return
    this.isolate(() => this.viewer?.setTheme?.(mode))
  }

  /** 聚焦请求（focusRevision 变化时由宿主调用）：选中态已由 selectBuildings 处理，这里只移动镜头 */
  focusRequested(): void {
    const index = this.host.inputs.focusIndex()
    if (index === null || !Number.isInteger(index) || index < 0 || index >= this.buildingCount) return
    this.isolate(() => this.viewer?.focusBuilding(index, {select: false}))
  }

  /** 当前状态快照（宿主初始化用） */
  getState(): BlueprintViewerBridgeState {
    return {
      loading: this.loading,
      error: this.error,
      buildingCount: this.buildingCount,
    }
  }

  // =============== 公开 API（与组件 defineExpose 一致） ===============

  // =============== 多建筑选中 ===============
  // 说明：预览库提供 selectedBuildings 批量高亮，这里只过滤越界下标并转调；空数组等价取消选中。
  // 边界：库在任一建筑找不到时整次调用失败并保持原选中，因此必须先按建筑总数过滤。
  selectBuildings(indices: number[]): void {
    const next = [...new Set(indices)].filter(index => Number.isInteger(index) && index >= 0 && index < this.buildingCount)
    this.isolate(() => this.viewer?.selectedBuildings(next.length ? next : null))
  }

  focusBuilding(index: number | null): void {
    this.isolate(() => this.viewer?.focusBuilding(index))
  }

  resetCamera(): void {
    this.isolate(() => this.viewer?.resetCamera())
  }

  captureImage(): string | null {
    return this.isolate(() => this.viewer?.toDataURL?.() ?? null)
  }

  setRenderSettings(showRealModels: boolean, maxInstancesPerModel: number): void {
    this.isolate(() => this.viewer?.setRenderSettings({showRealModels, maxInstancesPerModel}))
  }

  // =============== 增量预览 API ===============
  // 说明：对接后端 preview-delta 事件流；所有调用走主题隔离，避免第三方库写入 html[data-theme]。
  // 边界：调用顺序由上层协调层负责（beginPreview → 提交生成 → applyPreviewDelta → commitPreview/discardPreview）；
  //       转发层另有 beginPreviewReplay 提前合闸、finishPreviewReplay 收尾放行，把完整蓝图压到动画播完之后。
  //       beginPreview 前保存并清空旧正式场景；commitPreview 后丢弃快照；discardPreview 恢复旧场景。
  beginPreview(): void {
    if (!this.viewer) return
    // 新一轮量差动画：清空上一轮缓存的待提交蓝图、动画标记与闸门
    this.replayingDeltas = false
    this.replayPending = false
    this.pendingCommitBlueprint = null
    // 保存旧蓝图快照（编码 + 建筑数），供 discardPreview 恢复
    this.previewSnapshot = {code: this.host.inputs.blueprint().code, buildingCount: this.buildingCount}
    // 清空正式场景：库的 beginPreview 保留旧场景，这里把建筑按 ID 分批摘掉。
    // 不再整份加载空白蓝图——那会把星球和相机一起重建，且把内部数据整份替换成空蓝图；
    // 剩余目标在"响应到达、准备切页"时由 finishPreviewClear 合并成一次调用删完。
    this.startClear()
    this.isolate(() => this.viewer?.beginPreview())
  }

  applyPreviewDelta(event: unknown): boolean {
    // 防御：仍带候选维度（candidateId）或显式 v4 标记的事件说明页面转发层漏过滤。
    // 此时返回 true 表示"已由上层处理"，但不交给底层库，避免多候选对象污染单场景镜像。
    const value = event as {candidateId?: unknown; protocolVersion?: unknown} | null
    if (value && (value.candidateId !== undefined || value.protocolVersion === 4)) return true
    // 后端 FactoryGenerationProgress 与库 PreviewDeltaEvent 结构兼容，协议层已校验 kind/protocolVersion/operations，这里直接透传。
    const applied = this.isolate(() => this.viewer?.applyPreviewDelta(event as PreviewDeltaEvent) ?? false)
    // 差量已被接受：标记量差动画进行中，后续到达的完整蓝图要缓存到动画收尾
    if (applied) this.replayingDeltas = true
    return applied
  }

  /**
   * 合上提交闸门：本轮已有差量进入缓存、确定会重放时由转发层调用。
   * 说明：转发层要等算法舞台播完才切回蓝图预览并开始重放，这段空档里 generate() 的完整蓝图随时会到达；
   *       闸门必须在那一刻之前合上，否则整份蓝图先落进场景，重放就变成了对全量场景的重复新增。
   * 边界：只在 viewer 就绪时生效；finishPreviewReplay / discardPreview / beginPreview / 释放实例都会打开。
   */
  beginPreviewReplay(): void {
    if (!this.viewer) return
    this.replayPending = true
  }

  commitPreview(blueprint: string): unknown {
    // 差量已就绪待重放、或动画正在播放：只缓存完整蓝图，等 finishPreviewReplay 在动画播完后提交，避免整份蓝图提前覆盖动画
    if (this.replayingDeltas || this.replayPending) {
      this.pendingCommitBlueprint = blueprint
      this.skipNextCodeReload = true
      return {ok: true}
    }
    return this.commitBlueprint(blueprint)
  }

  /** 量差动画收尾：预览转发层在重放结束（含被打断）时调用，此刻才加载动画期间缓存的完整蓝图 */
  finishPreviewReplay(): void {
    this.replayingDeltas = false
    this.replayPending = false
    const blueprint = this.pendingCommitBlueprint
    this.pendingCommitBlueprint = null
    if (blueprint !== null) this.commitBlueprint(blueprint)
    // 动画期间的 code 变化已在 load 中跳过，这里清掉跳过标记，保证后续导入等加载照常执行
    this.skipNextCodeReload = false
  }

  /**
   * 立刻删完剩余的清场目标：转发层在"响应已到、准备切到蓝图预览并开始重放"时调用。
   * 说明：渐进清场把重建开销摊在生成期间，这里把剩下的建筑合并成**一次** deleteBuilding 调用删完，
   *       保证切页那一帧正式场景已经空了，重放不会与上一轮的旧建筑叠在一起。
   * 边界：没有进行中的清场时是空操作；删除失败会退回空白蓝图清场，返回后正式场景一定是空的。
   */
  finishPreviewClear(): void {
    const plan = this.clearPlan
    if (!plan) return
    const rest = plan.ids.slice(plan.cursor)
    if (rest.length > 0 && !this.removeBuildings(rest)) return
    this.completeClear()
  }

  discardPreview(): void {
    this.stopClear() // 先停清场：下面要按快照整份重载，继续按旧 ID 删会把恢复回来的建筑删掉
    this.isolate(() => this.viewer?.discardPreview())
    // 丢弃待提交的完整蓝图、动画标记与闸门，避免悬挂状态吞掉下一次 code 变化
    this.replayingDeltas = false
    this.replayPending = false
    this.pendingCommitBlueprint = null
    // 恢复旧蓝图快照：重新加载旧编码（无编码时回退空白蓝图）、还原建筑数与选中态
    const snapshot = this.previewSnapshot
    this.previewSnapshot = null
    if (snapshot && this.viewer) {
      const restoreCode = snapshot.code || BLANK_BLUEPRINT
      this.isolate(() => this.viewer!.loadBlueprint(restoreCode))
      this.setBuildingCount(this.viewer?.getBlueprintData()?.buildings.length ?? snapshot.buildingCount)
      this.selectBuildings(this.host.inputs.selectedBuildings())
      this.host.events.onReady({buildingCount: this.buildingCount})
    }
  }

  /** 内部 viewer 实例是否就绪（ESM 已加载且实例已创建）；未就绪时增量预览调用全部静默失败 */
  isReady(): boolean {
    return this.viewer !== null
  }

  getPreviewing(): boolean {
    return this.viewer?.previewing ?? false
  }

  getPreviewStats(): unknown {
    return this.isolate(() => this.viewer?.previewStats() ?? null)
  }

  // =============== 内部实现 ===============

  /** 应用主题隔离：执行库调用前后恢复 html[data-theme] 与库自己的主题缓存 */
  private isolate<T>(operation: () => T): T {
    const snapshot: AppThemeSnapshot = {
      rootThemeMode: readRootThemeMode(),
      blueprintEditorStoredTheme: localStorage.getItem(blueprintEditorThemeStorageKey),
    }
    try {
      return operation()
    } finally {
      if (snapshot.blueprintEditorStoredTheme === null) {
        localStorage.removeItem(blueprintEditorThemeStorageKey)
      } else {
        localStorage.setItem(blueprintEditorThemeStorageKey, snapshot.blueprintEditorStoredTheme)
      }

      const restoreThemeMode = this.host.theme.isReady()
        ? this.host.theme.read()
        : snapshot.rootThemeMode ?? this.host.theme.read()
      document.documentElement.dataset.theme = restoreThemeMode
      document.documentElement.style.colorScheme = restoreThemeMode
    }
  }

  private setLoading(value: boolean): void {
    if (this.loading === value) return
    this.loading = value
    this.emitState()
  }

  private setError(message: string): void {
    if (this.error === message) return
    this.error = message
    this.emitState()
  }

  private setBuildingCount(count: number): void {
    if (this.buildingCount === count) return
    this.buildingCount = count
    this.emitState()
  }

  private emitState(): void {
    this.host.events.onStateChange(this.getState())
  }

  /** 说明：组件挂载即预加载预览器 ESM，不等蓝图编码就绪，缩短首次渲染的等待时间。 */
  private async preloadBlueprintEditor(): Promise<void> {
    try {
      await ensureBlueprintEditor()
    } catch {
      // 加载失败不在这里上报：code 就绪后 loadCurrentBlueprint 会走 error 事件。
    }
  }

  // =============== 分批清场 ===============

  /** 按建筑 ID（`BlueprintBuilding.index`）列出当前正式场景的全部建筑；数据未就绪时返回空数组 */
  private buildingIds(): number[] {
    const buildings = this.viewer?.getBlueprintData()?.buildings ?? []
    const ids: number[] = []
    for (const building of buildings) ids.push(building.index)
    return ids
  }

  /** 开始分批清场：首批立即执行，其余按 CLEAR_TICK_MS 摊到算法页播放期间；无建筑则直接归零 */
  private startClear(): void {
    this.stopClear()
    if (!this.viewer) return
    const ids = this.buildingIds()
    if (ids.length === 0) {
      this.setBuildingCount(0)
      return
    }
    const batch = Math.ceil(ids.length / clearBatches(ids.length))
    this.clearPlan = {ids, cursor: 0, batch, timer: 0}
    this.pumpClear()
  }

  /** 推进一批；删完后收尾并把建筑数归 0 */
  private pumpClear(): void {
    const plan = this.clearPlan
    if (!plan) return
    plan.timer = 0
    if (!this.viewer) {
      this.stopClear()
      return
    }
    const chunk = plan.ids.slice(plan.cursor, plan.cursor + plan.batch)
    if (chunk.length > 0 && !this.removeBuildings(chunk)) return
    if (!this.clearPlan) return // 自愈重试已经把剩余建筑一次删完并停掉了计划
    plan.cursor += chunk.length
    if (plan.cursor >= plan.ids.length) {
      this.completeClear()
      return
    }
    plan.timer = window.setTimeout(() => this.pumpClear(), CLEAR_TICK_MS)
  }

  /** 清场完成：丢弃计划并把建筑数归 0（正式场景此时只剩星球） */
  private completeClear(): void {
    this.clearPlan = null
    this.setBuildingCount(0)
  }

  /** 停掉未完成的清场计划；整份替换场景的路径必须先调用，避免按旧 ID 删到新场景上 */
  private stopClear(): void {
    if (this.clearPlan && this.clearPlan.timer) window.clearTimeout(this.clearPlan.timer)
    this.clearPlan = null
  }

  /**
   * 删除一批建筑 ID；返回是否成功。
   * 说明：deleteBuilding 是整批失败语义（任一 ID 不存在则零变更），数据被其它路径改过时先按当前数据重算再试一次；
   *       仍然失败就退回空白蓝图清场，保证进入重放前正式场景一定是空的。
   */
  private removeBuildings(ids: number[]): boolean {
    const outcome = this.isolate(() => this.viewer?.deleteBuilding(1, ids))
    if (outcome?.ok) return true
    this.stopClear()
    const fresh = this.buildingIds()
    if (fresh.length > 0 && this.isolate(() => this.viewer?.deleteBuilding(1, fresh))?.ok === true) {
      this.setBuildingCount(0)
      return true
    }
    console.warn('[blueprint-preview] 分批清场失败，退回空白蓝图清场', outcome)
    this.loadBlankBlueprint()
    this.setBuildingCount(0)
    return false
  }

  /** 说明：把正式场景切回空白蓝图；返回是否成功，失败已上报 PREVIEW_E_CLEAR。 */
  private loadBlankBlueprint(): boolean {
    if (!this.viewer) return false
    const outcome = this.isolate(() => this.viewer!.loadBlueprint(BLANK_BLUEPRINT))
    if (!outcome.ok) {
      this.host.events.onError({code: 'PREVIEW_E_CLEAR', message: outcome.error.message})
      return false
    }
    return true
  }

  /** 完整蓝图原子替换：成功后正式场景即新蓝图，跳过 code 变化触发的全量重载 */
  private commitBlueprint(blueprint: string): BlueprintLoadOutcome | undefined {
    this.stopClear() // 新场景整份替换，未完成的分批清场没有意义，且会按旧 ID 删到新场景上
    const outcome = this.isolate(() => this.viewer?.commitPreview(blueprint))
    const ok = outcome && typeof outcome === 'object' && 'ok' in outcome && outcome.ok === true
    if (!ok) {
      // commit 失败：不设跳过标记，code 变化会触发全量重载作为兜底
      return outcome
    }
    // commitPreview 内部先建后换，正式场景已替换为新蓝图；跳过 code 变化触发的全量重载
    this.skipNextCodeReload = true
    this.setBuildingCount(this.viewer?.getBlueprintData()?.buildings.length ?? 0)
    this.previewSnapshot = null
    const maxBuildings = this.host.inputs.maxBuildings()
    if (this.buildingCount > maxBuildings) {
      this.host.events.onError({code: 'PREVIEW_E_LIMIT', message: `共 ${this.buildingCount} 个建筑，模型交互可能较慢`})
    }
    this.host.events.onReady({buildingCount: this.buildingCount})
    return outcome
  }

  /** 释放预览器实例 */
  private releaseViewer(): void {
    const canvas = this.viewer?.getCanvas?.() ?? this.host.inputs.container()?.querySelector('canvas')
    this.releaseInteraction?.()
    this.releaseInteraction = null
    this.isolate(() => this.viewer?.dispose())
    this.viewer = null
    const context = canvas?.getContext('webgl2') ?? canvas?.getContext('webgl')
    context?.getExtension('WEBGL_lose_context')?.loseContext()
  }

  /** 释放预览器实例、尺寸观察与全部悬挂状态 */
  private teardown(): void {
    this.loadVersion++
    this.stopClear()
    if (this.pendingLoadFrame) window.cancelAnimationFrame(this.pendingLoadFrame)
    this.pendingLoadFrame = 0
    if (this.pendingResizeFrame) window.cancelAnimationFrame(this.pendingResizeFrame)
    this.pendingResizeFrame = 0
    window.clearTimeout(this.resizeSettleTimer)
    this.lastSize = {width: 0, height: 0}
    this.resizeObserver?.disconnect()
    this.resizeObserver = null
    this.releaseViewer()
    this.releaseHost?.()
    this.releaseHost = null
    this.previewSnapshot = null
    this.skipNextCodeReload = false
    this.replayingDeltas = false
    this.replayPending = false
    this.pendingCommitBlueprint = null
    this.setLoading(false)
  }

  /** 说明：WebGL setSize 会清空画布，连续拖动时逐帧重设会闪白。拖动期间只用 CSS 拉伸上一帧，
   *       停止约 120ms 后再按最终尺寸重设并立即重绘；同一帧内的多次通知合并为一次。 */
  private applyViewerSize(): void {
    this.pendingResizeFrame = 0
    if (!this.viewer) return
    const container = this.host.inputs.container()
    if (!container) return
    const {width, height} = container.getBoundingClientRect()
    const next = {width: Math.floor(width), height: Math.floor(height)}
    if (next.width <= 0 || next.height <= 0 || (next.width === this.lastSize.width && next.height === this.lastSize.height)) return
    this.lastSize = next
    this.isolate(() => {
      this.viewer?.setSize(next.width, next.height)
      this.viewer?.toDataURL?.()
    })
  }

  private resizeViewer(): void {
    if (!this.viewer) return
    window.clearTimeout(this.resizeSettleTimer)
    this.resizeSettleTimer = window.setTimeout(() => {
      if (this.pendingResizeFrame) return
      this.pendingResizeFrame = window.requestAnimationFrame(() => this.applyViewerSize())
    }, 120)
  }

  private hasRenderableSize(): boolean {
    const container = this.host.inputs.container()
    if (!container) return false
    const {width, height} = container.getBoundingClientRect()
    return width > 0 && height > 0
  }

  /** 蓝图加载器入口：无编码时先加载空白蓝图渲染星球主体，真实编码到达后由 load() 重新加载 */
  private async loadCurrentBlueprint(): Promise<void> {
    this.stopClear() // 整份重载会替换场景与数据，未完成的分批清场必须作废
    const container = this.host.inputs.container()
    if (!this.active || !container) return
    if (!this.hasRenderableSize()) return
    const {code} = this.host.inputs.blueprint()
    const blank = !code

    const version = ++this.loadVersion
    this.setLoading(true)
    this.setError('')
    let phase: 'load' | 'render' = 'load'

    try {
      // 加载蓝图编辑器
      const {BlueprintViewer} = await ensureBlueprintEditor()
      // 检查状态
      if (!this.active || version !== this.loadVersion) return
      const currentContainer = this.host.inputs.container()
      if (!currentContainer) return
      phase = 'render'
      // 清理旧实例
      this.releaseViewer()
      // 创建新实例
      if (!this.viewer) {
        // 登记释放函数，保持全站唯一预览实例
        this.releaseHost = claimBlueprintPreview(this.disposeRef)
        this.viewer = this.isolate(() => new BlueprintViewer(
          {container: currentContainer, theme: this.host.theme.read()},
          {
            // 注册建筑选择事件
            onBuildingSelect: pick => this.isolate(() => {
              if (this.active && this.host.inputs.interactive()) {
                this.host.events.onSelect({index: pick?.index ?? null})
              }
            }),
          },
        ))
        // 获取画布并启用预览交互（左键平移 / 右键旋转）
        const canvas = this.viewer.getCanvas?.()
        if (canvas) this.releaseInteraction = enableBlueprintPreviewInteraction(this.viewer, canvas)
      }
      // this.setRenderSettings(false, 1024)
      // 加载蓝图模型：无编码时用空白蓝图先渲染星球主体，保证预览器先于流程图出画面
      if (blank) {
        if (!this.loadBlankBlueprint()) return
      } else {
        this.isolate(() => this.viewer?.loadBlueprint(code))
      }
      this.setBuildingCount(this.viewer.getBlueprintData()?.buildings.length ?? 0)
      // 强制重置尺寸并同步画布尺寸
      this.lastSize = {width: 0, height: 0}
      this.applyViewerSize()
      this.resetCamera()
      // 回填选中态与渲染参数
      this.selectBuildings(this.host.inputs.selectedBuildings())
      // this.setRenderSettings(true, 1024)
      // 提示建筑数量
      const maxBuildings = this.host.inputs.maxBuildings()
      if (this.buildingCount > maxBuildings) {
        this.host.events.onError({code: 'PREVIEW_E_LIMIT', message: `共 ${this.buildingCount} 个建筑，模型交互可能较慢`})
      }
      this.host.events.onReady({buildingCount: this.buildingCount, blank})
    } catch (error) {
      if (!this.active || version !== this.loadVersion) return
      const message = error instanceof Error ? error.message : String(error || '蓝图模型渲染失败。')
      this.setError(message)
      this.host.events.onError({code: phase === 'load' ? 'PREVIEW_E_LOAD' : 'PREVIEW_E_RENDER', message})
    } finally {
      if (version === this.loadVersion) this.setLoading(false)
    }
  }
}
