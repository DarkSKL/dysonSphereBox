import { onBeforeUnmount, reactive, watch } from 'vue'
import type { AlgoVizProgress } from '../../packages/algorithm-viz/src'
import { observeFactoryGenerationEvents } from '../../pages/services/dysonSphere/factory-progress'
import type { FactoryGenerationProgress, FactoryPreviewOperation } from '../../types/factory-progress'
import type { QuantWorkspace } from './useQuantWorkspace'
import { useAlgorithmActivity } from './useAlgorithmActivity'
import { resetTabIntent, showAlgorithmTab, showPreviewTab, tab as previewTab, userSwitched } from './useBlueprintPreviewTabs'

// =============== 蓝图候选缓存分发层 ===============
// 职责：全量缓存生成期间的所有候选差量；在 preview-complete 时根据 winnerCandidateId 取出对应数据，
//       重写对象身份后全量下发（Replay）给 viewer，发送完成后清理缓存。
// 使用：量化与蓝图页面在 setup 中调用，退订随组件卸载；返回的状态供调试侧栏观测。
// 边界：v3 不参与；不消费 preview-snapshot 分块；最终画面仍由 commitPreview 提交，但推迟到本层重放动画结束后执行。
//       算法页可见时，重放要等平面预览器的播放队列收敛后再开始，切换回蓝图预览也由这一时刻驱动。

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

// =============== 算法舞台收敛判定 ===============
// 说明：渲染包只提供逐栋建筑通知（algorithm:building:before/after），没有"整轮播完"事件；
//       因此以舞台进度收敛作为判据——播放队列、过渡动画、候选下坠全部归零且只剩至多一个候选。
// 边界：进度回调本身按 100ms 节流，判定需要等真实的空闲快照；用户暂停播放或舞台卡住时按超时放行。
const SCENE_IDLE_TIMEOUT = 20000
const SCENE_IDLE_GRACE = 500

const sceneBusy = (progress: AlgoVizProgress): boolean =>
  progress.pending > 0 || progress.animating > 0 || progress.falling > 0 || progress.candidates > 1

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
  // 每轮生成只自动切一次算法可视化；用户手动切过标签后，本轮不再由数据层改写。
  let autoSwitched = false
  // 算法舞台的最新进度由渲染包回调写入共享记录，这里只读取，不参与渲染
  const activity = useAlgorithmActivity()

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
    resetTabIntent() // 新一轮生成重新允许数据层自动切换标签
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

  // =============== 算法播放收敛 ===============
  // 说明：等待算法舞台把队列、过渡动画与候选下坠播完；舞台空闲由进度快照判定，不靠逐栋通知计数。
  // 边界：只等待首个"空闲且发生在开始时点之后"的快照；初始就空闲时给正式结果入队留出宽限期。
  const waitAlgorithmPlayback = (): Promise<void> => new Promise(resolve => {
    let busySeen = Boolean(activity.latest.value && sceneBusy(activity.latest.value))
    let grace = 0
    let timeout = 0
    let unwatch: (() => void) | null = null
    const finish = (): void => {
      window.clearTimeout(grace)
      window.clearTimeout(timeout)
      unwatch?.()
      resolve()
    }
    unwatch = watch(activity.latest, progress => {
      if (!progress) return
      if (sceneBusy(progress)) {
        busySeen = true
        window.clearTimeout(grace)
        return
      }
      // 只采信"先忙后闲"的收敛，避免拿到上一轮遗留的空闲快照就提前放行
      if (busySeen) finish()
    })
    timeout = window.setTimeout(finish, SCENE_IDLE_TIMEOUT)
    if (!busySeen) grace = window.setTimeout(finish, SCENE_IDLE_GRACE)
  })

  /** 收尾：算法页播完后切回蓝图预览，再逐条重放获选候选；算法页不可见时直接重放 */
  const switchBackThenReplay = async (candidateId: string): Promise<void> => {
    const currentGen = replayGeneration
    if (previewTab.value === 'algorithm') {
      await waitAlgorithmPlayback()
      if (currentGen !== replayGeneration) return
      if (previewTab.value === 'algorithm' && !userSwitched.value) showPreviewTab()
    }
    if (currentGen !== replayGeneration) return
    await flushToViewer(candidateId)
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

      // 收到候选数据：命令切到算法可视化，方便跟随生成过程；用户手动切过标签则不再夺回。
      if (!autoSwitched && !userSwitched.value) {
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

    // 3. 完成：算法页播完后切回蓝图预览，再按 winnerCandidateId 触发重放
    if (event.kind === 'preview-complete') {
      const winnerId = (event as any).winnerCandidateId as string | undefined
      if (winnerId) {
        // 异步执行，不阻塞事件循环
        void switchBackThenReplay(winnerId)
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