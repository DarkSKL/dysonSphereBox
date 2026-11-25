// =============== 星图提示状态判定 ===============
// 说明：把「正在加载、加载失败、确实无反馈、所选星移出视野、视野内无星」从同一组输入推出唯一状态，
// 让页面始终只显示一条区分明确的提示，且不会在首次加载完成前把空结果误判为「无反馈」。
// 使用：页面把地图与详情状态传入 galaxyStatus；返回值交给 map-status 组件渲染。
// 边界：只做判定，不读取画布、不发请求；文案在组件内，便于样式与文案同处维护。
export type GalaxyStatusKind = 'loading' | 'error' | 'empty' | 'selected-offscreen' | 'selected-grouped' | 'offscreen' | ''

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
  /**
   * 最近一次成功快照是否为全景范围（查询不带边界）。只有全景快照为空才表示整片星图没有反馈；
   * 局部范围为空只说明当前画面里没有星。不能用当前镜头倍率判断：镜头回到全景时，全景回执可能还没到。
   */
  snapshotIsPanorama: boolean
  /** 当前镜头对应的查询范围是否仍是全景。与 snapshotIsPanorama 不一致说明正在等新范围的回执。 */
  viewIsPanorama: boolean
  /** 当前有 Map 请求在途。 */
  loading: boolean
  /** 详情面板正在阅读的星，没有则为空串。 */
  selectedId: string
  /** 所读的星按它自己的坐标是否投影在当前画面内（不看快照里有没有它）。 */
  selectedVisible: boolean
  /**
   * 所读的星当前是否有自己的可点击星点：快照里有它自己，或仍能在画面里点到。
   * 为 false 且在画面内，说明它被并入聚合星（当前倍率下只有代表星可点），这不是「移出画面」。
   */
  selectedClickable: boolean
  /** 当前快照里是否存在聚合星（count 大于 1）。没有聚合证据时，不把「画面内但不可点」归因于聚合。 */
  aggregated: boolean
}

export function galaxyStatus(input: GalaxyStatusInput): GalaxyStatusKind {
  if (!input.foreground) return ''
  if (input.error) return 'error'
  if (!input.ready) return 'loading'
  // 画面正在等新范围的回执：现有数据仍属上一次的快照，既不能宣布「确实无反馈」，也不该提示「画面内无星」。
  const awaitingScope = input.viewIsPanorama !== input.snapshotIsPanorama
  if (awaitingScope) return ''
  if (input.total === 0 && input.snapshotIsPanorama) return 'empty'
  if (input.selectedId && !input.selectedVisible) return 'selected-offscreen'
  if (input.selectedId && !input.selectedClickable && input.aggregated) return 'selected-grouped'
  if (input.visible === 0 && !input.loading) return 'offscreen'
  return ''
}
