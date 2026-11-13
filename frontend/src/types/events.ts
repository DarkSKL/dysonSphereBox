import type {AlgoVizBuildingEvent} from '../packages/algorithm-viz/src'

// =============== 前端业务事件契约 ===============
// 说明：事件统一按「模块:功能:触发点」命名，before 表示操作开始，after 表示操作完成。
// 边界：事件只通知当前前端中的订阅者，不保存状态，也不代表后端任务成功；新增事件在此集中声明载荷。
export type AppEventPhase = 'before' | 'after'
export type AppEventPattern = `${string}:${string}:${AppEventPhase}`

export type AppEvents = {
  /** 单栋建筑开始新增或替换；排队但尚未应用的操作不触发。 */
  'algorithm:building:before': AlgoVizBuildingEvent
  /** 单栋建筑姿态落定；撤销、重置、被替换打断或候选淘汰的操作不触发。 */
  'algorithm:building:after': AlgoVizBuildingEvent
}

export type AppEventName = keyof AppEvents & AppEventPattern
export type AppEventHandler<Event extends AppEventName> = (payload: AppEvents[Event]) => void | Promise<void>
