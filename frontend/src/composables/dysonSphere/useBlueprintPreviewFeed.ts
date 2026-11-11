import { onBeforeUnmount, reactive, watch } from 'vue'
import { observeFactoryGenerationEvents } from '../../pages/services/dysonSphere/factory-progress'
import type { FactoryGenerationProgress, FactoryPreviewOperation } from '../../types/factory-progress'
import type { QuantWorkspace } from './useQuantWorkspace'
import { showAlgorithmTab } from './useBlueprintPreviewTabs'

// =============== 蓝图候选缓存分发层 ===============
// 职责：全量缓存生成期间的所有候选差量；在 preview-complete 时根据 winnerCandidateId 取出对应数据，
//       重写对象身份后全量下发（Replay）给 viewer，发送完成后清理缓存。
// 使用：量化与蓝图页面在 setup 中调用，退订随组件卸载；返回的状态供调试侧栏观测。
// 边界：v3 不参与；不消费 preview-snapshot 分块；最终画面仍由 commitPreview 提交，但推迟到本层重放动画结束后执行。

export interface BlueprintPreviewTarget {
  applyPreviewDelta(event: FactoryGenerationProgress): boolean
  discardPreview(): void
  isReady(): boolean
  /** 量差动画收尾（播放完毕或被中断）；预览层在此刻才加载动画期间缓存的完整蓝图。 */
  finishPreviewReplay?(): void
  getPreviewStats?(): unknown
}

// 注意这里扩充了 'buffering' 和 'replaying'，兼容旧代码的 'tracking' 等
export type FeedPhase = 'idle' | 'tracking' | 'wait-result' | 'buffering' | 'replaying'

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

// 完美保留旧 UI 所需字段，同时增加缓存池所需的字段
export interface BlueprintPreviewFeedState {
  phase: FeedPhase
  /** 旧字段：应用计数（重放阶段会批量增加） */
  applied: number
  /** 旧字段：放弃计数 */
  discarded: number
  /** 旧字段：最后应用时间 */
  lastAppliedAt: number
  
  /** 新增字段：当前选中的候选 ID（重放阶段使用） */
  activeCandidateId: string
  /** 新增字段：缓存池中的候选列表 */
  cachedCandidates: string[]
  /** 新增字段：缓存的事件总数 */
  cachedEventsCount: number
  
  stats: BlueprintPreviewStats | null
}

export interface BlueprintPreviewFeed {
  state: BlueprintPreviewFeedState
  refreshStats(): void
}

/** 重写操作的 objectId 并同步内部引用 */
const rewriteOperation = (prefix: string, operation: FactoryPreviewOperation): FactoryPreviewOperation => {
  const objectId = prefix + operation.objectId
  if (operation.type === 'remove') return { type: 'remove', objectId }
  const building = operation.building
  const reference = (value: string | undefined): string | undefined => (value ? prefix + value : value)
  return { 
    type: operation.type, 
    objectId, 
    building: { ...building, inputObjectId: reference(building.inputObjectId), outputObjectId: reference(building.outputObjectId) } 
  }
}

/** 剥离候选维度，保留单场景 viewer 需要的差量字段 */
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
  // 初始化 state，补齐旧 UI 需要的基础字段
  const state = reactive<BlueprintPreviewFeedState>({
    phase: 'idle', 
    applied: 0, 
    discarded: 0, 
    lastAppliedAt: 0, 
    
    activeCandidateId: '',
    cachedCandidates: [],
    cachedEventsCount: 0,
    
    stats: null
  })

  // 缓存池：候选 ID -> 该候选的差量事件数组
  const cachePool = new Map<string, FactoryGenerationProgress[]>()
  let phase: FeedPhase = 'idle'

  // 新增：用于打断异步重放的令牌。每次 reset 或重新开始重放时自增。
  let replayGeneration = 0
  // 每轮生成只自动切一次算法可视化；用户切回蓝图预览后不再被后续数据夺回。
  let autoSwitched = false

  const viewer = (): BlueprintPreviewTarget | null => {
    const value = target()
    return value && value.isReady() ? value : null
  }

  const refreshStats = (): void => {
    try {
      const value = (target()?.getPreviewStats?.() ?? null) as BlueprintPreviewStats | null
      if (value) state.stats = value
    } catch { /* 预览层已释放 */ }
  }

  const syncState = (): void => {
    state.phase = phase
    state.cachedCandidates = Array.from(cachePool.keys())
    state.cachedEventsCount = Array.from(cachePool.values()).reduce((acc, list) => acc + list.length, 0)
  }

  const reset = (): void => {
    replayGeneration++ // 打断当前正在执行的异步循环
    autoSwitched = false
    // 动画被打断时也让预览层收尾（提交已缓存的完整蓝图），避免悬挂的待提交状态吞掉后续 code 变化
    viewer()?.finishPreviewReplay?.()
    phase = 'idle'
    state.activeCandidateId = ''
    cachePool.clear()
    state.applied = 0
    state.lastAppliedAt = 0
    state.stats = null
    syncState()
  }

  /** 核心：将选中的候选数据全量下发给 Viewer */
/** 核心：将选中的候选数据全量下发给 Viewer（每个差量延迟 100ms） */
  const flushToViewer = async (candidateId: string): Promise<void> => {
    const panel = viewer()
    const events = cachePool.get(candidateId)
    
    if (!panel || !events || events.length === 0) {
      reset()
      return
    }

    const currentGen = ++replayGeneration
    phase = 'replaying'
    state.activeCandidateId = candidateId
    syncState()

    // 1. 按顺序重写并应用全量事件，每个间隔 100ms
    for (let i = 0; i < events.length; i++) {
      // 在每次循环前检查令牌，如果被 reset 或被新任务覆盖则直接退出
      if (currentGen !== replayGeneration) return
      
      // 检查 viewer 是否依然就绪（防止中途切走页面或销毁）
      const currentPanel = viewer()
      if (!currentPanel) break

      const event = events[i]
      if (currentPanel.applyPreviewDelta(singleSceneDelta(candidateId, event))) {
        state.applied++
        state.lastAppliedAt = Date.now()
      }

      // 如果这不是最后一个事件，等待 100ms 再执行下一个
      if (i < events.length - 1) {
        await new Promise(resolve => setTimeout(resolve, 100))
      }
    }

    // 2. 重放完成后，确认没有被外部打断，再通知预览层收尾并清理
    if (currentGen === replayGeneration) {
      // 量差动画已播完：此刻才让预览层加载动画期间缓存的完整蓝图，避免整份蓝图提前覆盖动画画面
      viewer()?.finishPreviewReplay?.()
      refreshStats()
      cachePool.clear()
      
      phase = 'idle'
      state.activeCandidateId = ''
      syncState()
    }
  }

  const onEvent = (event: FactoryGenerationProgress): void => {
    if (event.protocolVersion !== 4) return

    // 1. 任务开始，重置状态并进入缓冲
    if (event.kind === 'search-start') {
      reset()
      phase = 'buffering'
      syncState()
      return
    }

    if (event.kind === 'search-failed') {
      reset()
      return
    }

    // 2. 持续写入缓存
    if (event.kind === 'preview-delta' && event.operations?.length && phase === 'buffering') {
      const cid = event.candidateId ?? 'unknown'

      // 收到候选数据：命令切到算法可视化，方便跟随生成过程；切回蓝图预览保留标签上的手动入口。
      if (!autoSwitched) {
        autoSwitched = true
        showAlgorithmTab()
      }

      if (!cachePool.has(cid)) {
        cachePool.set(cid, [])
      }
      cachePool.get(cid)!.push(event)
      syncState()
      return
    }

    // 3. 完成：根据 winnerCandidateId 触发重放
    if (event.kind === 'preview-complete') {
      const winnerId = (event as any).winnerCandidateId as string | undefined
      if (winnerId) {
        // 异步执行，不阻塞事件循环
        void flushToViewer(winnerId)
      } else {
        reset()
      }
      return
    }

    // 兜底：处理中断
    if (event.stage === 'preview-stream-incomplete') {
      state.discarded++ 
      reset()
      return
    }
    if (event.stage === 'preview-stream-end') {
      if (phase === 'buffering') {
        phase = 'idle'
        syncState()
      }
      return
    }
  }

  const stop = observeFactoryGenerationEvents(onEvent, () => true, () => w.generationTaskId)
  
  watch(() => w.generationTaskId, reset, { flush: 'sync' })
  
  onBeforeUnmount(() => { 
    stop() 
  })

  return {
    state,
    refreshStats
  }
}