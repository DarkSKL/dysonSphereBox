import mitt, {type Emitter} from 'mitt'
import type {AppEventHandler, AppEventName, AppEvents} from '../types/events'

// =============== 前端事件总线 ===============
// 说明：全应用共用一个 mitt 实例；业务模块用 emit 发布、on 订阅，on 返回仅取消本次订阅的函数。
// 边界：同步分发、不重放历史；订阅者的同步异常和异步失败单独记录，不阻断舞台或其他订阅者。
// 开发期热更新也复用同一实例，避免保留中的舞台和新订阅者使用不同总线。
const holder = globalThis as typeof globalThis & {__dsboxEventBus?: Emitter<AppEvents>}
const emitter = holder.__dsboxEventBus ??= mitt<AppEvents>()

export const eventBus = {
  emit<Event extends AppEventName>(name: Event, payload: AppEvents[Event]): void {
    emitter.emit(name, payload)
  },

  /** 普通 TypeScript 模块使用；Vue 组件优先使用 useAppEvent 自动管理订阅作用域。 */
  on<Event extends AppEventName>(name: Event, handler: AppEventHandler<Event>): () => void {
    let active = true
    const reportError = (error: unknown): void => { console.error('[event-bus] 事件监听失败', name, error) }
    const listener = (payload: AppEvents[Event]): void => {
      if (!active) return
      try {
        const result = handler(payload)
        if (result) void result.catch(reportError)
      } catch (error) { reportError(error) }
    }
    emitter.on(name, listener)
    return () => { active = false; emitter.off(name, listener) }
  },
}
