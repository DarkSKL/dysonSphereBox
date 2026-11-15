import {EventsOn} from '../../../../wailsjs/runtime/runtime'
import {ReadGenerationProgress} from '../../../../wailsjs/go/factory/Service'
import type {FactoryGenerationProgress, FactoryGenerationProgressEvent} from '../../../types/factory-progress'
import {isFactoryGenerationProgressEvent} from '../../../utils/factory-preview-protocol'

const eventName = 'factory:generationProgress'
const observedListeners = new Set<(event: FactoryGenerationProgress) => void>()
const readiness = new Map<(event: FactoryGenerationProgress) => void, {ready: () => boolean; task: () => string | undefined}>()

export const observeFactoryGenerationEvents = (listener: (event: FactoryGenerationProgress) => void, ready: () => boolean = () => true, task: () => string | undefined = () => undefined): (() => void) => {
  observedListeners.add(listener)
  readiness.set(listener, {ready, task})
  return () => { observedListeners.delete(listener); readiness.delete(listener) }
}

/** 职责：订阅单次生成阶段；使用：调用 Generate 前安装，结束或卸载时释放；边界：无运行时则回退普通生成，AbortSignal 仅停止观察、不取消 Go 任务。 */
export const observeFactoryGeneration = (
  taskId: string,
  onProgress?: (progress: FactoryGenerationProgress) => void,
  signal?: AbortSignal,
  recoverable = false,
): (() => void) & {drain: () => Promise<void>; finish: () => void} => {
  let listening = !signal?.aborted
  let unsubscribe: (() => void) | undefined
  let timer: number | undefined
  let lastSequence = 0
  let recovering: Promise<void> | null = null
  let incomplete = false
  let returned = false
  let started = false
  const canDeliver = (): boolean => Array.from(readiness.values()).every(({ready, task}) => {
    const owner = task()
    return owner !== undefined && owner !== taskId ? true : ready()
  })
  const pending = new Map<number, FactoryGenerationProgressEvent>()
  const dispatch = (event: FactoryGenerationProgress): void => {
    for (const listener of observedListeners) {
      const owner = readiness.get(listener)?.task()
      if (owner !== undefined && owner !== event.taskId) continue
      try { listener(event) } catch (error) { console.warn('[factory-preview] 演示事件处理失败', error) }
    }
    onProgress?.(event)
  }
  const failRecovery = (message: string): void => {
    if (!listening || incomplete) return
    incomplete = true
    pending.clear()
    dispatch({taskId, stage: 'preview-stream-incomplete', message, current: 0, total: 0})
    if (returned) stop()
  }
  const ingest = (event: FactoryGenerationProgressEvent): void => {
    if (!listening || incomplete) return
    if (!recoverable) { dispatch(event); return }
    if (!canDeliver()) { pending.clear(); return }
    const sequence = event.sequence
    if (sequence === undefined || sequence <= lastSequence) return
    pending.set(sequence, event)
    let next = pending.get(lastSequence + 1)
    while (next) {
      if (!canDeliver()) { pending.clear(); return }
      pending.delete(++lastSequence)
      dispatch(next)
      next = pending.get(lastSequence + 1)
    }
  }
  const recover = (force = false): Promise<void> => {
    if (!listening || !recoverable || incomplete) return Promise.resolve()
    if (!started && !force) return Promise.resolve()
    if (recovering) return recovering
    recovering = (async () => {
      try {
        while (listening && !incomplete) {
          if (!canDeliver()) return
          const before = lastSequence
          const page = await ReadGenerationProgress(taskId, before)
          if (!listening) return
          if (page.incomplete) { failRecovery(page.message || '过程记录不完整，将使用正式结果'); return }
          for (const event of page.events ?? []) {
            if (!isFactoryGenerationProgressEvent(event) || event.taskId !== taskId) {
              failRecovery('过程记录校验失败，将使用正式结果')
              return
            }
            ingest(event)
          }
          if (page.complete && lastSequence >= page.next) {
            if (returned) {
              dispatch({taskId, stage: 'preview-stream-end', message: '生成过程记录读取完成', current: 0, total: 0})
              stop()
            }
            return
          }
          if (lastSequence === before || (page.events?.length ?? 0) < 16) return
          // 连续补页时让出主线程，保留所有序号与控制事件，不把动画队列长度当成停止读取条件。
          await new Promise<void>(resolve => { window.setTimeout(resolve, 0) })
        }
      } catch {
        failRecovery('过程记录恢复失败，将使用正式结果')
      }
    })().finally(() => { recovering = null })
    return recovering
  }
  const stop = (): void => {
    listening = false
    unsubscribe?.()
    unsubscribe = undefined
    signal?.removeEventListener('abort', stop)
    if (timer !== undefined) window.clearInterval(timer)
    pending.clear()
  }
  const runtime = (window as Window & {runtime?: {EventsOnMultiple?: unknown}}).runtime
  if (listening && onProgress && typeof runtime?.EventsOnMultiple === 'function') {
    unsubscribe = EventsOn(eventName, (event: unknown) => {
      if (listening && isFactoryGenerationProgressEvent(event) && event.taskId === taskId) {
        started = true
        ingest(event)
        if (pending.size) void recover()
      }
    })
    signal?.addEventListener('abort', stop, {once: true})
    if (recoverable) timer = window.setInterval(() => { void recover() }, 500)
  }
  return Object.assign(stop, {
    drain: async (): Promise<void> => { await recover(true); await recover(true) },
    finish: (): void => {
      returned = true
      unsubscribe?.()
      unsubscribe = undefined
      if (!recoverable || incomplete) stop()
      else void recover(true)
    },
  })
}

/** 职责：旁路订阅全部生成事件原始载荷；使用：调试时间流在页面挂载时安装，卸载时调用返回值释放；边界：不按任务过滤、不做协议校验，不影响生成任务本身的观察者。 */
export const observeRawFactoryGenerationEvents = (onEvent: (payload: unknown) => void): (() => void) => {
  const runtime = (window as Window & {runtime?: {EventsOnMultiple?: unknown}}).runtime
  if (typeof runtime?.EventsOnMultiple !== 'function') return () => undefined
  return EventsOn(eventName, onEvent)
}
