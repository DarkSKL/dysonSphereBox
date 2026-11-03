import {EventsOn} from '../../../../wailsjs/runtime/runtime'
import type {FactoryGenerationProgress} from '../../../types/factory-progress'
import {isFactoryGenerationProgressEvent} from '../../../utils/factory-preview-protocol'

const eventName = 'factory:generationProgress'

/** 职责：订阅单次生成阶段；使用：调用 Generate 前安装，结束或卸载时释放；边界：无运行时则回退普通生成，AbortSignal 仅停止观察、不取消 Go 任务。 */
export const observeFactoryGeneration = (
  taskId: string,
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
      if (listening && isFactoryGenerationProgressEvent(event) && event.taskId === taskId) onProgress(event)
    })
    signal?.addEventListener('abort', stop, {once: true})
  }
  return stop
}
