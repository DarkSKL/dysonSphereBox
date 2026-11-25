// =============== 星图提示状态判定 ===============
// 说明：把「正在加载、加载失败、确实无反馈、所选星移出视野、视野内无星」从同一组输入推出唯一状态，
// 让页面始终只显示一条区分明确的提示，且不会在首次加载完成前把空结果误判为「无反馈」。
// 使用：页面把地图与详情状态传入 galaxyStatus；返回值交给 map-status 组件渲染。
// 边界：只做判定，不读取画布、不发请求；文案在组件内，便于样式与文案同处维护。
export type GalaxyStatusKind = 'loading' | 'error' | 'empty' | 'selected-offscreen' | 'offscreen' | ''

export interface GalaxyStatusInput {
  /** 页面在前台且账号会话有效。 */
  foreground: boolean
  /** 至少成功读取过一次星图快照。 */
  ready: boolean
  /** 最近一次读取的错误文案，空串表示无错误。 */
  error: string
  /** 当前快照中的全部星数，含作者待审的私有星。 */
  total: number
  /** 当前画面内可点击的星数。 */
  visible: number
  /** 当前镜头倍率；1 表示全景，此时查询不带范围，空结果才代表整片星图没有反馈。 */
  zoom: number
  /** 详情面板正在阅读的星，没有则为空串。 */
  selectedId: string
  /** 所读的星是否在当前画面内。 */
  selectedVisible: boolean
}

export function galaxyStatus(input: GalaxyStatusInput): GalaxyStatusKind {
  if (!input.foreground) return ''
  if (input.error) return 'error'
  if (!input.ready) return 'loading'
  if (input.total === 0 && input.zoom <= 1.001) return 'empty'
  if (input.selectedId && !input.selectedVisible) return 'selected-offscreen'
  if (input.visible === 0) return 'offscreen'
  return ''
}
