import type {Bounds, DisplayStar, ReviewState, StarDetail} from '../../types/feedback'
import {GALAXY_EDGE, normalizeQueryScope, type QueryScope} from './galaxy-camera'

// =============== 星图提示状态判定 ===============
// 说明：把「正在加载、加载失败、确实无反馈、所选星移出视野、所选星由聚合星代表、视野内无星」从同一组输入推出唯一状态，
// 让页面始终只显示一条区分明确的提示，且不会在首次加载完成前或视区切换等待新范围回执期间误报旧空态。
// 使用：页面把地图与详情状态传入 galaxyStatus；返回值交给 map-status 组件渲染。
// 边界：只做判定，不读取画布、不发请求；文案在组件内，便于样式与文案同处维护。
export type GalaxyStatusKind = 'loading' | 'error' | 'empty' | 'selected-offscreen' | 'selected-grouped' | 'offscreen' | ''

/** 聚合快照中单格覆盖的最大占比（按至少 16×16 空间网格保守估算）。 */
export const AGGREGATE_CELL_DIVISOR = 16

/**
 * 只有审核通过且未撤回的反馈才进入公开星图体系；
 * 待审（pending）、待人工复核（manual）、未通过（rejected）均为私有状态，绝不归因于聚合星隐藏。
 */
export function isPublicFeedback(reviewState: ReviewState | undefined, withdrawn = false): boolean {
  return !withdrawn && reviewState === 'approved'
}

/**
 * 判断一颗公开反馈星在当前快照中是否存在明确的聚合覆盖依据：
 * 快照中必须存在至少一颗公开聚合星（count > 1，非私有星且非目标本身），
 * 且目标星的盘面坐标落在其对应快照跨度的聚合邻域内，而非遥远无关处恰好存在聚合点。
 */
export function isCoveredByAggregate(
  target: Pick<StarDetail, 'starId' | 'x' | 'y' | 'withdrawn'> & {original: Pick<StarDetail['original'], 'reviewState'>},
  points: readonly DisplayStar[],
  snapshotBounds: Bounds | null,
): boolean {
  if (!isPublicFeedback(target.original.reviewState, target.withdrawn)) return false
  const spanX = snapshotBounds ? Math.max(1, snapshotBounds.maxX - snapshotBounds.minX) : 2 * GALAXY_EDGE
  const spanY = snapshotBounds ? Math.max(1, snapshotBounds.maxY - snapshotBounds.minY) : 2 * GALAXY_EDGE
  const cellX = spanX / AGGREGATE_CELL_DIVISOR
  const cellY = spanY / AGGREGATE_CELL_DIVISOR
  for (const point of points) {
    if (point.privateLabel || (point.count ?? 1) <= 1 || point.starId === target.starId) continue
    if (Math.abs(target.x - point.x) <= cellX && Math.abs(target.y - point.y) <= cellY) return true
  }
  return false
}

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
   * 最近一次成功快照对应的实际查询范围（'all' 表示全景，四元边界表示局部，空串表示尚无成功快照）。
   * 只有全景快照为空才表示整片星图没有反馈；局部快照为空只说明该局部范围没有星。
   */
  snapshotRegion: QueryScope | string
  /** 当前镜头对应的实际查询范围。与 snapshotRegion 归一化后不一致，说明正在等新范围回执（含请求合并窗口期）。 */
  viewRegion: QueryScope | string
  /** 当前有 Map 请求在途。 */
  loading: boolean
  /** 详情面板正在阅读且应参与星图位置提示的星，没有则为空串。 */
  selectedId: string
  /** 所读的星是否已审核通过并进入公开星图体系；待审/转人工/驳回等私有反馈恒为 false。 */
  selectedPublic: boolean
  /** 所读的星按它自己的坐标是否投影在当前画面内（不看快照里有没有它）。 */
  selectedVisible: boolean
  /** 所读的星当前是否有自己的可点击星点。 */
  selectedClickable: boolean
  /** 当前快照中是否存在明确覆盖该公开星位置的聚合星依据。 */
  selectedGrouped: boolean
}

export function galaxyStatus(input: GalaxyStatusInput): GalaxyStatusKind {
  if (!input.foreground) return ''
  if (input.error) return 'error'
  if (!input.ready) return 'loading'
  // 所读星是否在画面内按其自有坐标与当前相机实时判定，不依赖新范围回执。
  if (input.selectedId && !input.selectedVisible) return 'selected-offscreen'
  const snapshotScope = normalizeQueryScope(input.snapshotRegion)
  const viewScope = normalizeQueryScope(input.viewRegion)
  // 镜头已切换到与最近成功快照不同的实际查询范围（含 160ms 请求合并窗口期与请求在途期）：
  // 在新范围快照到达前，不沿用旧快照宣布「确实无反馈」、「画面内无星」或旧聚合状态。
  if (!snapshotScope || snapshotScope !== viewScope) return ''
  if (input.total === 0 && snapshotScope === 'all') return 'empty'
  if (input.selectedId && input.selectedPublic && !input.selectedClickable && input.selectedGrouped) return 'selected-grouped'
  if (input.visible === 0 && !input.loading) return 'offscreen'
  return ''
}
