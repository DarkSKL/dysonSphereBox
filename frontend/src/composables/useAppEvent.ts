import {getCurrentScope, onScopeDispose} from 'vue'
import {eventBus} from '../utils/event-bus'
import type {AppEventHandler, AppEventName} from '../types/events'

// =============== 事件订阅生命周期 ===============
// 说明：在 setup 或 effectScope 中订阅业务事件，作用域销毁时自动取消，也可调用返回函数提前停止。
// 边界：KeepAlive 停用不销毁作用域，因此仍可接收事件；需要可见性限制的订阅者自行判断当前激活状态。
// 使用：useAppEvent('algorithm:building:after', ({id, building}) => { ... })。
export const useAppEvent = <Event extends AppEventName>(name: Event, handler: AppEventHandler<Event>): (() => void) => {
  if (!getCurrentScope()) throw new Error('useAppEvent 必须在 setup 或 effectScope 中使用')
  const stop = eventBus.on(name, handler)
  onScopeDispose(stop)
  return stop
}
