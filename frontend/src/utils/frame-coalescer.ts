// =============== 逐帧合并写入 ===============
// 说明：高频事件只保留最新值，下一帧统一写入一次，避免每条事件都触发一轮 Vue 更新。
// 使用：生成阶段事件每秒可达数十条，响应式状态用 push 写入；需要立即清空时先 cancel 再直接赋值。
// 边界：只保留最后一个值，不适用于必须逐条处理的事件；页面隐藏时 rAF 暂停，恢复后补写最新值。
export interface FrameCoalescer<T> {
  /** 记录最新值，下一帧写入 */
  push: (value: T) => void
  /** 丢弃尚未写入的值 */
  cancel: () => void
}

export const createFrameCoalescer = <T>(apply: (value: T) => void): FrameCoalescer<T> => {
  let pending: {value: T} | null = null
  let handle = 0
  const run = (): void => {
    handle = 0
    const next = pending
    pending = null
    if (next) apply(next.value)
  }
  return {
    push: (value) => {
      pending = {value}
      if (!handle) handle = window.requestAnimationFrame(run)
    },
    cancel: () => {
      pending = null
      if (handle) window.cancelAnimationFrame(handle)
      handle = 0
    },
  }
}
