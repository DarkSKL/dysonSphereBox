import {EventsOn} from '../../../wailsjs/runtime/runtime'
import type {FactoryGenerationProgress, FactoryGenerationProgressEvent} from '../../types/factory-progress'

const eventName = 'factory:generationProgress'

const isProgressEvent = (value: unknown): value is FactoryGenerationProgressEvent => {
  if (!value || typeof value !== 'object') return false
  const event = value as Partial<FactoryGenerationProgressEvent>
  if (!(typeof event.requestId === 'string' && typeof event.stage === 'string'
    && typeof event.message === 'string' && Number.isInteger(event.current)
    && Number.isInteger(event.total) && (event.current ?? -1) >= 0
    && (event.total ?? -1) >= (event.current ?? 0))) return false

  if (event.protocolVersion === undefined && event.kind === undefined) return true
  if (event.protocolVersion !== 1 || !Number.isInteger(event.sequence) || (event.sequence ?? 0) < 1) return false

  if (event.kind === 'phase') return true
  if (event.coordinateSystem !== 'blueprint-area-local-v1') return false
  if (event.kind === 'building-batch') {
    const batch = event.previewBatch
    return !!batch && typeof batch.module?.id === 'string' && typeof batch.module?.name === 'string'
      && Number.isInteger(batch.module?.productionIndex) && Number.isInteger(batch.batchIndex)
      && Number.isInteger(batch.batchCount) && (batch.batchIndex ?? -1) >= 0
      && (batch.batchCount ?? 0) > (batch.batchIndex ?? -1)
      && Array.isArray(batch.buildings) && batch.buildings.length > 0 && Array.isArray(batch.areas)
  }
  if (event.kind === 'preview-complete') {
    return Number.isInteger(event.buildingCount) && (event.buildingCount ?? -1) >= 0
      && Number.isInteger(event.batchCount) && (event.batchCount ?? -1) >= 0
  }
  return false
}

/** 职责：订阅单次生成阶段；使用：调用 Generate 前安装，结束或卸载时释放；边界：无运行时则回退普通生成，AbortSignal 仅停止观察、不取消 Go 任务。 */
export const observeFactoryGeneration = (
  requestId: string,
  onProgress?: (progress: FactoryGenerationProgress) => void,
  signal?: AbortSignal,
): (() => void) => {
  let listening = !signal?.aborted
  let unsubscribe: (() => void) | undefined
  const stop = (): void => {
    listening = false
    unsubscribe?.()
    unsubscribe = undefined
    signal?.removeEventListener('abort', stop)
  }
  const runtime = (window as Window & {runtime?: {EventsOnMultiple?: unknown}}).runtime
  if (listening && onProgress && typeof runtime?.EventsOnMultiple === 'function') {
    unsubscribe = EventsOn(eventName, (event: unknown) => {
      if (listening && isProgressEvent(event) && event.requestId === requestId) onProgress(event)
    })
    signal?.addEventListener('abort', stop, {once: true})
  }
  return stop
}
