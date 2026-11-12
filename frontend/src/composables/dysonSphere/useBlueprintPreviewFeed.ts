import { onBeforeUnmount, reactive, watch } from 'vue'
import { observeFactoryGenerationEvents } from '../../pages/services/dysonSphere/factory-progress'
import type { FactoryGenerationProgress, FactoryPreviewOperation } from '../../types/factory-progress'
import type { QuantWorkspace } from './useQuantWorkspace'
// 预览标签页命令
import { showAlgorithmTab } from './useBlueprintPreviewTabs'

// =============== 蓝图候选缓存分发层 ===============
// 职责：全量缓存生成期间的所有候选差量；在 preview-complete 时根据 winnerCandidateId 取出对应数据，
//       重写对象身份后全量下发（Replay）给 viewer，发送完成后清理缓存。
// 使用：量化与蓝图页面在 setup 中调用，退订随组件卸载；返回的状态供调试侧栏观测。
// 边界：v3 不参与；不消费 preview-snapshot 分块；最终画面仍由 commitPreview 提交，但推迟到本层重放动画结束后执行。

// =============== 常量与类型定义 ===============
/** 重放事件之间的间隔时间，单位为毫秒，用于控制增量动画的播放节奏 */
const eventIntervalTime = 100

/** Viewer 预览目标接口，定义了动画重放和状态查询的最小动作集合 */
export interface BlueprintPreviewTarget {
  applyPreviewDelta(event: FactoryGenerationProgress): boolean
  discardPreview(): void
  isReady(): boolean
  /** 量差动画收尾（播放完毕或被中断）；预览层在此刻才加载动画期间缓存的完整蓝图。 */
  finishPreviewReplay?(): void
  getPreviewStats?(): unknown
}

/** 状态机阶段：包含旧逻辑的 tracking/wait-result 以及新增的 buffering/replaying 状态 */
export type FeedPhase = 'idle' | 'tracking' | 'wait-result' | 'buffering' | 'replaying'

/** 渲染侧计数，由预览库 previewStats() 提供，用于调试侧栏的数据核对 */
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

/** 向外暴露的响应式状态接口，兼容旧 UI 字段同时扩充缓存池监控字段 */
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

/** 转发层对外句柄 */
export interface BlueprintPreviewFeed {
  state: BlueprintPreviewFeedState
  refreshStats(): void
}

// =============== 辅助纯函数 ===============
/** 重写操作的 objectId 并同步内部引用，保证单场景内跨候选不撞名 */
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

// =============== 蓝图候选缓存分发逻辑 ===============
/**
 * 说明：创建并管理蓝图预览的缓存流。
 * 使用：在组件 setup 阶段调用，接收工作区实例与预览目标获取函数。
 * 边界：维护局部缓存池 cachePool，组件卸载时必须触发 onBeforeUnmount 清理监听。
 */
export const useBlueprintPreviewFeed = (w: QuantWorkspace, target: () => BlueprintPreviewTarget | null): BlueprintPreviewFeed => {
  
  // =============== 状态与内部变量 ===============
  // 响应式状态，用于驱动 UI 更新和调试展示
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

  // 候选 ID 到差量事件数组的映射缓存池
  const cachePool = new Map<string, FactoryGenerationProgress[]>()
  let phase: FeedPhase = 'idle'

  // 防竞态令牌：用于打断异步重放循环（每次 reset 或重新开始重放时自增）
  let replayGeneration = 0
  // 自动切换标签页控制，每轮生成只自动切一次算法可视化
  let autoSwitched = false

  // =============== 内部工具方法 ===============
  /** 获取当前就绪的 Viewer 实例，若未就绪则返回 null */
  const viewer = (): BlueprintPreviewTarget | null => {
    const value = target()
    return value && value.isReady() ? value : null
  }

  /** 拉取预览库的摄入计数用于调试核对，异常捕获防止预览层已销毁时报错 */
  const refreshStats = (): void => {
    try {
      const value = (target()?.getPreviewStats?.() ?? null) as BlueprintPreviewStats | null
      if (value) state.stats = value
    } catch { /* 预览层已释放 */ }
  }

  /** 同步内部状态至 Vue 响应式对象，触发 UI 视图更新 */
  const syncState = (): void => {
    state.phase = phase
    state.cachedCandidates = Array.from(cachePool.keys())
    state.cachedEventsCount = Array.from(cachePool.values()).reduce((acc, list) => acc + list.length, 0)
  }

  /** 重置所有状态与清空缓存池，并通过递增令牌打断可能正在进行的重放循环 */
  const reset = (): void => {
    replayGeneration++ // 自增令牌，使正在 await 的异步循环失效
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

  // =============== 核心业务：重放动画 ===============
  /**
   * 说明：将选中的候选数据按照固定间隔（100ms）全量下发给 Viewer 播放动画。
   * 边界：如果在 await 期间被打断（令牌改变），立即终止重放并防止数据污染。
   */
  const flushToViewer = async (candidateId: string): Promise<void> => {
    const panel = viewer()
    const events = cachePool.get(candidateId)
    
    // 兜底：无 Viewer 或无缓存数据直接复位
    if (!panel || !events || events.length === 0) {
      reset()
      return
    }

    // 记录当前重放令牌，用于后续中断检测
    const currentGen = ++replayGeneration
    phase = 'replaying'
    state.activeCandidateId = candidateId
    syncState()

    // 1. 按序播放全量事件（动画阶段）
    for (let i = 0; i < events.length; i++) {
      // 中断检测：若在 await 期间被外部 reset，立即静默退出
      if (currentGen !== replayGeneration) return
      
      // 页面切走检测：若 Viewer 就绪状态丢失则跳出循环
      const currentPanel = viewer()
      if (!currentPanel) break

      const event = events[i]
      if (currentPanel.applyPreviewDelta(singleSceneDelta(candidateId, event))) {
        state.applied++
        state.lastAppliedAt = Date.now()
      }

      // 除最后一个事件外，均等待固定延迟，形成动画效果
      if (i < events.length - 1) {
        await new Promise(resolve => setTimeout(resolve, eventIntervalTime))
      }
    }

    // 2. 动画收尾（加载全量模型与清理）
    // 确认未被外部打断才执行收尾
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

  // =============== 核心业务：事件分发器 ===============
  /**
   * 说明：处理来自底层订阅的流式事件，驱动状态机流转。
   * 边界：仅处理 protocolVersion 为 4 的事件；非 buffer 状态下的增量数据将被忽略。
   */
  const onEvent = (event: FactoryGenerationProgress): void => {
    if (event.protocolVersion !== 4) return

    // 1. 任务开始
    if (event.kind === 'search-start') {
      reset()
      phase = 'buffering'
      syncState()
      return
    }

    // 2. 候选声明或开始，初始化缓存槽位
    if (event.kind === 'candidate-plan' || event.kind === 'candidate-start') {
      const cid = event.candidateId ?? 'unknown'
      cachePool.set(cid, [])
      syncState()
      return
    }

    // 3. 任务失败，清理复位
    if (event.kind === 'search-failed') {
      console.log(event.message)
      reset()
      return
    }

    // 4. 接收增量数据（仅限缓冲阶段）
    if (event.kind === 'preview-delta' && event.operations?.length && phase === 'buffering') {
      const cid = event.candidateId ?? 'unknown'

      // 首个增量到达时自动切换到算法可视化页面（单次）
      if (!autoSwitched) {
        autoSwitched = true
        showAlgorithmTab()
      }

      // 防御性初始化并存入缓存池
      if (!cachePool.has(cid)) {
        cachePool.set(cid, [])
      }
      cachePool.get(cid)!.push(event)
      syncState()
      return
    }

    // 5. 普通事件状态流转，仅记录日志
    if (event.kind === 'phase') {
      console.log(event.message)
      return
    }

    // 6. 候选结束，输出可行性状态
    if (event.kind === 'candidate-finish') {
      console.log("可行候选：" + event.candidateId + " 状态" + event.candidateStatus)
      return
    }

    // 7. 正式快照（本层不负责消费，忽略）
    if (event.kind === 'preview-snapshot') {
      return
    }

    const showPreviewTab = false
    // 算法可视化状态流转（等接口）
    // 业务：算法可视化页面动画完成，响应状态生成事件，切换页面。
    // if (event.kind === '?') {
    //   showPreviewTab()
    //   showPreviewTab = true
    // }

    // 8. 任务完成：提取 winnerId 触发重放动画
    if (event.kind === 'preview-complete'&& showPreviewTab) {
      const winnerId = (event as any).winnerCandidateId as string | undefined
      if (winnerId) {
        void flushToViewer(winnerId) // 异步执行，不阻塞事件循环
      } else {
        reset() // 兜底防挂起
      }
      return
    }

    // 9. 中断与结束兜底
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

  // =============== 生命周期挂载 ===============
  // 订阅工厂生成事件
  const stop = observeFactoryGenerationEvents(onEvent, () => true, () => w.generationTaskId)
  
  // 监听外部任务 ID 变化，立即重置缓存与动画
  watch(() => w.generationTaskId, reset, { flush: 'sync' })
  
  // 组件卸载时取消订阅
  onBeforeUnmount(() => { 
    stop() 
  })

  return {
    state,
    refreshStats
  }
}