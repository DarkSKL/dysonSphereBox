import {onBeforeUnmount, reactive, watch} from 'vue'
import {observeFactoryGenerationEvents} from '../../pages/services/dysonSphere/factory-progress'
import type {FactoryGenerationProgress, FactoryPreviewOperation} from '../../types/factory-progress'
import type {QuantWorkspace} from './useQuantWorkspace'

// =============== 蓝图单候选转发层 ===============
// 职责：把生成事件流按协议版本分发为单场景差量；只跟踪候选1，重写对象身份后喂给 viewer。
// 使用：量化与蓝图页面在 setup 中调用，退订随组件卸载；返回的状态供调试侧栏观测。
// 边界：v3 是多候选共用同一通知器的合并流（每个候选依次全量替换），一律不转发；
//       不消费 preview-snapshot 分块与 preview-complete 的正式数据（算法可视化的职责）；
//       不切换候选，不参与生成成败；最终画面始终由 useQuantWorkspace 的 commitPreview 决定。

/** 转发目标：viewer-panel 参与增量预览所需的最小动作 */
export interface BlueprintPreviewTarget {
  applyPreviewDelta(event: FactoryGenerationProgress): boolean
  discardPreview(): void
  isReady(): boolean
  getPreviewStats?(): unknown
}

type FeedPhase = 'idle' | 'tracking' | 'wait-result'

/** 渲染侧计数：来自预览库 previewStats()，仅用于调试核对。 */
export interface BlueprintPreviewStats {
  events?: number
  operations?: number
  droppedEvents?: number
  unknownRemoves?: number
  unknownReplaces?: number
  alive?: number
  peakAlive?: number
  filteredByThrash?: number
  lastIngestMs?: number
  maxIngestMs?: number
  coordinateSystem?: string | null
}

/** 调试状态：转发层当前阶段与最近一次渲染计数。 */
export interface BlueprintPreviewFeedState {
  phase: FeedPhase
  /** 固定跟踪的候选序号：蓝图只渲染候选1。 */
  target: number
  tracked: string
  expected: number
  applied: number
  discarded: number
  lastAppliedAt: number
  stats: BlueprintPreviewStats | null
}

/** 转发层对外句柄：状态给调试侧栏，refreshStats 用于手动刷新渲染计数。 */
export interface BlueprintPreviewFeed {
  state: BlueprintPreviewFeedState
  refreshStats(): void
}

/** 跟踪候选序号：蓝图预览固定只渲染候选 1，不随差量到达顺序变化。 */
const TARGET_ORDINAL = 1

/** 从候选身份解析序号；后端身份形如 candidate-3，序号与 candidateOrdinal 同源。 */
const ordinalOf = (candidateId: string | undefined): number => {
  const value = Number((candidateId ?? '').replace(/^candidate-/, ''))
  return Number.isFinite(value) ? value : 0
}

const isTargetCandidate = (event: FactoryGenerationProgress): boolean => (event.candidateOrdinal || ordinalOf(event.candidateId)) === TARGET_ORDINAL

/** 跟踪候选长时间没有新差量（或迟迟没有正式结果）时放弃过程预览，等待 generate 收尾的 commitPreview 原子替换。 */
const TRACK_TIMEOUT = 5 * 60 * 1000

/** 重写操作的 objectId 并同步内部引用，保证单场景内跨候选不撞名。 */
const rewriteOperation = (prefix: string, operation: FactoryPreviewOperation): FactoryPreviewOperation => {
  const objectId = prefix + operation.objectId
  if (operation.type === 'remove') return {type: 'remove', objectId}
  const building = operation.building
  const reference = (value: string | undefined): string | undefined => (value ? prefix + value : value)
  return {type: operation.type, objectId, building: {...building, inputObjectId: reference(building.inputObjectId), outputObjectId: reference(building.outputObjectId)}}
}

/** 剥离候选维度，只保留单场景 viewer 需要的差量字段；保留全局 sequence 供预览库做重复事件去重。 */
const singleSceneDelta = (candidateId: string, event: FactoryGenerationProgress): FactoryGenerationProgress => {
  const prefix = `${candidateId}:`
  return {
    kind: 'preview-delta',
    sequence: event.sequence,
    stage: event.stage,
    message: event.message,
    current: event.current,
    total: event.total,
    coordinateSystem: event.coordinateSystem,
    areas: event.areas,
    primaryAreaIndex: event.primaryAreaIndex,
    operations: (event.operations ?? []).map(operation => rewriteOperation(prefix, operation)),
  }
}

export const useBlueprintPreviewFeed = (w: QuantWorkspace, target: () => BlueprintPreviewTarget | null): BlueprintPreviewFeed => {
  const state = reactive<BlueprintPreviewFeedState>({phase: 'idle', target: TARGET_ORDINAL, tracked: '', expected: 0, applied: 0, discarded: 0, lastAppliedAt: 0, stats: null})
  let phase: FeedPhase = 'idle'
  let tracked = ''
  let expected = 0
  let timer: number | undefined
  const viewer = (): BlueprintPreviewTarget | null => {
    const value = target()
    return value && value.isReady() ? value : null
  }
  // 调试：拉取预览库的摄入计数（未知对象删除/替换、被节流过滤等），用于核对 ID 重写与镜像连续性。
  // 只在预览仍在进行时覆盖；commit 后预览层被清空，保留最后一次计数供事后核对。
  const refreshStats = (): void => {
    try {
      const value = (target()?.getPreviewStats?.() ?? null) as BlueprintPreviewStats | null
      if (value) state.stats = value
    } catch { /* 预览层已释放 */ }
  }
  const sync = (): void => {
    state.phase = phase
    state.tracked = tracked
    state.expected = expected
  }
  const stopTimer = (): void => {
    if (timer !== undefined) window.clearTimeout(timer)
    timer = undefined
  }
  const reset = (): void => {
    stopTimer()
    phase = 'idle'
    tracked = ''
    expected = 0
    state.stats = null
    state.applied = 0
    state.discarded = 0
    state.lastAppliedAt = 0
    sync()
  }
  const wait = (): void => {
    stopTimer()
    phase = 'wait-result'
    tracked = ''
    expected = 0
    sync()
  }
  const abandon = (): void => {
    try { viewer()?.discardPreview() } catch { /* 预览层已释放 */ }
    state.discarded++
    wait()
    refreshStats()
  }
  const arm = (): void => {
    stopTimer()
    timer = window.setTimeout(() => { if (phase === 'tracking') wait() }, TRACK_TIMEOUT)
  }
  // 只转发固定候选的 v4 差量；v3 是多候选共享单场景的合并流，一律不渲染。
  const applyDelta = (event: FactoryGenerationProgress): void => {
    const panel = viewer()
    if (!panel || !event.operations?.length) return
    if (event.protocolVersion !== 4 || !isTargetCandidate(event)) return
    if (phase === 'wait-result') return
    const sequence = event.candidateSequence ?? 0
    if (!sequence) return
    if (phase === 'idle') {
      // 自举：候选1 的首条差量决定起点；序号从该候选自身序列接续（通常为 2，1 被 candidate-start 占用）。
      tracked = event.candidateId ?? ''
      expected = sequence + 1
      phase = 'tracking'
    } else if (sequence !== expected) {
      // 断流：临时层已缺对象，继续接收会让 remove 指向不存在的对象；直接放弃过程预览。
      abandon()
      return
    } else {
      expected++
    }
    if (panel.applyPreviewDelta(singleSceneDelta(tracked, event))) {
      state.applied++
      state.lastAppliedAt = Date.now()
      arm()
    }
    sync()
    refreshStats()
  }
  const onEvent = (event: FactoryGenerationProgress): void => {
    // 本地事件由订阅封装派发，不含协议版本；incomplete 终止不完整过程，end 表示不会再有过程事件。
    if (event.stage === 'preview-stream-incomplete') { abandon(); return }
    if (event.stage === 'preview-stream-end') { if (phase === 'tracking') wait(); return }
    if (event.protocolVersion !== 4) return
    if (event.kind === 'search-start') { reset(); return }
    if (event.kind === 'candidate-finish') {
      if (isTargetCandidate(event) && event.candidateStatus !== 'valid') abandon()
      return
    }
    if (event.kind === 'preview-complete') { wait(); return }
    if (event.kind === 'search-failed') { reset(); return }
    // candidate-plan / candidate-start 不参与自举：蓝图固定跟候选1，其余候选的事件一律丢弃。
    if (event.kind === 'preview-delta') applyDelta(event)
  }
  const stop = observeFactoryGenerationEvents(onEvent, () => true, () => w.generationTaskId)
  watch(() => w.generationTaskId, reset, {flush: 'sync'})
  onBeforeUnmount(() => { stop(); stopTimer() })
  return {state, refreshStats}
}
