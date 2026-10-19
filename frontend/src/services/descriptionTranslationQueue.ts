import type {DescriptionTranslation, DescriptionTranslationQueue, TranslationBatch} from '../types/translation'

// =============== 可见描述翻译队列 ===============
// 说明：按原文字节去重，短时间滚动合并为最多二十项的串行请求；失败只由用户重试。
// 边界：调用方提供按来源排序的可见文本；关闭、停用和作用域切换使在途结果失效，不修改源 DTO。
export function createDescriptionTranslationQueue(
  send: (texts: string[], context: string) => Promise<TranslationBatch>,
  changed: () => void,
  delay = 120,
): DescriptionTranslationQueue {
  const values = new Map<string, DescriptionTranslation>()
  let visible: string[] = []
  let enabled = false
  let active = true
  let context = ''
  let epoch = 0
  let busy = false
  let disposed = false
  let timer: ReturnType<typeof setTimeout> | undefined

  function cancelPending(): void {
    epoch++
    clearTimeout(timer)
    timer = undefined
    for (const [text, value] of values) {
      if (value.status === 'loading') values.delete(text)
    }
    changed()
  }

  function candidates(): string[] {
    return [...new Set(visible)].filter((text) => text.trim() && !values.has(text))
  }

  function schedule(): void {
    clearTimeout(timer)
    timer = undefined
    if (disposed || !enabled || !active || !context || busy || candidates().length === 0) return
    timer = setTimeout(() => { void flush() }, delay)
  }

  async function flush(): Promise<void> {
    timer = undefined
    if (disposed || !enabled || !active || busy) return
    const texts = candidates().slice(0, 20)
    if (!texts.length) return
    const requestEpoch = epoch
    const requestContext = context
    busy = true
    texts.forEach((text) => values.set(text, {status: 'loading', reason: '', data: text}))
    changed()
    try {
      const result = await send(texts, requestContext)
      if (requestEpoch !== epoch || disposed) return
      if (!Array.isArray(result.items) || result.items.length !== texts.length) throw new Error('batchMismatch')
      texts.forEach((text, index) => {
        const item = result.items[index]
        values.set(text, item?.status === 'ready' && !item.reason && typeof item.data === 'string' && item.data.trim()
          ? {status: 'ready', reason: '', data: item.data}
          : {status: 'error', reason: item?.reason || 'providerFailed', data: text})
      })
    } catch {
      if (requestEpoch !== epoch || disposed) return
      texts.forEach((text) => values.set(text, {status: 'error', reason: 'requestFailed', data: text}))
    } finally {
      busy = false
      changed()
      schedule()
    }
  }

  return {
    setEnabled(value: boolean): void {
      if (enabled === value) return
      enabled = value
      cancelPending()
      schedule()
    },
    setActive(value: boolean): void {
      if (active === value) return
      active = value
      cancelPending()
      schedule()
    },
    setContext(value: string): void {
      if (context === value) return
      context = value
      cancelPending()
      values.clear()
      changed()
      schedule()
    },
    setVisibleTexts(texts: string[]): void {
      visible = texts
      schedule()
    },
    get(text: string): DescriptionTranslation | undefined {
      return values.get(text)
    },
    retry(text: string): void {
      if (!enabled || !active || !visible.includes(text) || values.get(text)?.status !== 'error') return
      values.delete(text)
      changed()
      schedule()
    },
    dispose(): void {
      disposed = true
      cancelPending()
      values.clear()
    },
  }
}
