import { onBeforeUnmount, reactive, watch } from 'vue'
import { observeFactoryGenerationEvents } from '../../pages/services/dysonSphere/factory-progress'
import type { FactoryGenerationProgress, FactoryPreviewOperation } from '../../types/factory-progress'
import type { QuantWorkspace } from './useQuantWorkspace'

// =============== 蓝图候选缓存分发层 ===============
// 职责：在预算内缓存生成期间的候选差量；在 preview-complete 时根据 winnerCandidateId 取出对应数据，
//       重写对象身份后全量下发（Replay）给 viewer，发送完成后清理缓存。
// 使用：量化与蓝图页面在 setup 中调用，退订随组件卸载；返回的状态供调试侧栏观测。
// 边界：v3 不参与；不消费 preview-snapshot 分块；最终画面仍由 commitPreview 提交，但推迟到本层重放动画结束后执行。

export interface BlueprintPreviewTarget {
  applyPreviewDelta(event: FactoryGenerationProgress): boolean
  getPreviewing(): boolean
  /** 量差动画收尾（播放完毕或被中断）；预览层在此刻才加载动画期间缓存的完整蓝图。 */
  finishPreviewReplay?(): void
  getPreviewStats?(): unknown
}

export type FeedPhase = 'idle' | 'wait-result' | 'buffering' | 'replaying'

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

export interface BlueprintPreviewFeedState {
  phase: FeedPhase
  /** 应用计数（重放阶段会批量增加） */
  applied: number
  /** 放弃计数 */
  discarded: number
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

export const useBlueprintPreviewFeed = (w: QuantWorkspace, target: () => BlueprintPreviewTarget | null, visible: () => boolean): BlueprintPreviewFeed => {
  const state = reactive<BlueprintPreviewFeedState>({
    phase: 'idle',
    applied: 0,
    discarded: 0,
    stats: null
  })

  // 缓存只用于获选方案的动画。超过预算时释放本轮缓存，仍由正式蓝图完成渲染。
  const cachePool = new Map<string, FactoryGenerationProgress[]>()
  const operationBudget = 50000
  let bufferedOperations = 0
  let epoch = 0
  let timer: number | undefined

  const refreshStats = (): void => {
    state.stats = (target()?.getPreviewStats?.() ?? null) as BlueprintPreviewStats | null
  }
  const clearBuffer = (): void => {
    cachePool.clear()
    bufferedOperations = 0
  }
  const stopReplay = (): void => {
    epoch++
    window.clearTimeout(timer)
    timer = undefined
    if (state.phase === 'replaying') target()?.finishPreviewReplay?.()
    clearBuffer()
    state.phase = 'idle'
  }

  // 只重放获选候选，其他候选的差量在确认获选时立即释放。
  const replayWinner = (candidateId: string): void => {
    const panel = target()
    const events = cachePool.get(candidateId)
    clearBuffer()
    if (!panel?.getPreviewing() || !events?.length) {
      state.phase = 'wait-result'
      return
    }
    const version = ++epoch
    state.phase = 'replaying'
    let index = 0
    const advance = (): void => {
      if (version !== epoch) return
      if (target() !== panel || !panel.getPreviewing()) { stopReplay(); return }
      try {
        if (!panel.applyPreviewDelta(singleSceneDelta(candidateId, events[index++]))) {
          state.discarded++
          stopReplay()
          return
        }
        state.applied++
        if (index === events.length) {
          refreshStats()
          stopReplay()
        } else {
          timer = window.setTimeout(advance, Math.min(100, 3000 / events.length))
        }
      } catch {
        state.discarded++
        stopReplay()
      }
    }
    advance()
  }

  const onEvent = (event: FactoryGenerationProgress): void => {
    if (event.stage === 'preview-stream-incomplete' || event.kind === 'search-failed') {
      stopReplay()
      return
    }
    if (event.stage === 'preview-stream-end') {
      if (state.phase === 'buffering') stopReplay()
      return
    }
    if (event.protocolVersion !== 4) return
    if (event.kind === 'search-start') {
      stopReplay()
      if (visible() && target()?.getPreviewing()) state.phase = 'buffering'
      return
    }
    if (state.phase !== 'buffering') return
    if (!target()?.getPreviewing()) { stopReplay(); return }
    const id = event.candidateId
    if (event.kind === 'candidate-finish' && id && event.candidateStatus !== 'valid') {
      const events = cachePool.get(id)
      if (events) {
        for (const item of events) bufferedOperations -= item.operations?.length ?? 0
        cachePool.delete(id)
      }
    } else if (event.kind === 'preview-delta' && id && event.operations?.length) {
      bufferedOperations += event.operations.length
      if (bufferedOperations > operationBudget) {
        clearBuffer()
        state.discarded++
        state.phase = 'wait-result'
        return
      }
      let events = cachePool.get(id)
      if (!events) {
        events = []
        cachePool.set(id, events)
      }
      events.push(event)
    } else if (event.kind === 'preview-complete') {
      replayWinner(event.winnerCandidateId ?? '')
    }
  }

  const stop = observeFactoryGenerationEvents(onEvent, () => true, () => w.generationTaskId)
  watch(() => w.generationTaskId, () => {
    stopReplay()
    state.applied = state.discarded = 0
    state.stats = null
  }, {flush: 'sync'})
  watch(visible, shown => { if (!shown) stopReplay() })
  onBeforeUnmount(() => { stop(); stopReplay() })
  return {state, refreshStats}
}
