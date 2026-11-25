import type {Bounds, DisplayStar, ReviewState, StarDetail} from '../../types/feedback'
import {GALAXY_EDGE, normalizeQueryScope, type QueryScope} from './galaxy-camera'

// =============== 星图提示状态判定 ===============
// 说明：把「正在加载、加载失败、确实无反馈、所选星移出视野、所选星由聚合星代表、视野内无星」从同一组输入推出唯一状态，
// 让页面始终只显示一条区分明确的提示，且不会在首次加载完成前或视区切换等待新范围回执期间误报旧空态。
// 使用：页面把地图与详情状态传入 galaxyStatus；返回值交给 map-status 组件渲染。
// 边界：只做判定，不读取画布、不发请求；文案在组件内，便于样式与文案同处维护。
export type GalaxyStatusKind = 'loading' | 'error' | 'empty' | 'selected-offscreen' | 'selected-grouped' | 'offscreen' | ''

/** 服务端聚合快照使用的网格宽度（32×32 = 1024 桶，与 server/service/feedback/map.go:18-19 一致）。 */
export const AGGREGATE_GRID_WIDTH = 32

let snapshotOrderCounter = 0

/**
 * 分配严格递增的客户端因果序号，用于标记「地图请求发起」与「详情观测到已过审」的先后顺序，
 * 避免依赖可能同毫秒碰撞的墙钟时间。
 */
export function nextSnapshotOrder(): number {
  return ++snapshotOrderCounter
}

export interface AggregateCell {
  col: number
  row: number
  index: number
}

/**
 * 计算当前快照用于 32×32 分桶的有效边界，与服务端 intersectGalaxy(view)（map.go:86-91）保持一致：
 * 全景（null）对应 [-GALAXY_EDGE, GALAXY_EDGE]；局部边界取整后与星系边界求交，交集为空返回 null。
 */
export function aggregateGridBounds(snapshotBounds: Bounds | null): Bounds | null {
  if (!snapshotBounds) {
    return {minX: -GALAXY_EDGE, minY: -GALAXY_EDGE, maxX: GALAXY_EDGE, maxY: GALAXY_EDGE}
  }
  if (!Number.isFinite(snapshotBounds.minX) || !Number.isFinite(snapshotBounds.minY) ||
    !Number.isFinite(snapshotBounds.maxX) || !Number.isFinite(snapshotBounds.maxY)) {
    return null
  }
  const minX = Math.max(Math.trunc(snapshotBounds.minX), -GALAXY_EDGE)
  const minY = Math.max(Math.trunc(snapshotBounds.minY), -GALAXY_EDGE)
  const maxX = Math.min(Math.trunc(snapshotBounds.maxX), GALAXY_EDGE)
  const maxY = Math.min(Math.trunc(snapshotBounds.maxY), GALAXY_EDGE)
  if (minX > maxX || minY > maxY) return null
  return {minX, minY, maxX, maxY}
}

/**
 * 计算坐标 (x, y) 在当前快照 32×32 网格中的分桶坐标，与服务端 cellIndex(view, x, y)
 * 及 approvedInView 闭区间 BETWEEN（map_geometry.go:48-52, 105）完全一致：
 * - 坐标落在有效闭区间 [minX, maxX] × [minY, maxY] 之外返回 null；
 * - col = floor((x - minX) * 32 / (maxX - minX + 1))；
 * - row = floor((y - minY) * 32 / (maxY - minY + 1))；
 * - index = row * 32 + col。
 */
export function aggregateCellOf(
  point: Pick<StarDetail, 'x' | 'y'>,
  snapshotBounds: Bounds | null,
): AggregateCell | null {
  const inside = aggregateGridBounds(snapshotBounds)
  if (!inside) return null
  if (!Number.isFinite(point.x) || !Number.isFinite(point.y)) return null
  const x = Math.trunc(point.x)
  const y = Math.trunc(point.y)
  if (x < inside.minX || x > inside.maxX || y < inside.minY || y > inside.maxY) return null
  const spanX = inside.maxX - inside.minX + 1
  const spanY = inside.maxY - inside.minY + 1
  const col = Math.floor(((x - inside.minX) * AGGREGATE_GRID_WIDTH) / spanX)
  const row = Math.floor(((y - inside.minY) * AGGREGATE_GRID_WIDTH) / spanY)
  return {col, row, index: row * AGGREGATE_GRID_WIDTH + col}
}

/**
 * 只有审核通过且未撤回的反馈才进入公开星图体系；
 * 待审（pending）、待人工复核（manual）、未通过（rejected）均为私有状态，绝不归因于聚合星隐藏。
 */
export function isPublicFeedback(reviewState: ReviewState | undefined, withdrawn = false): boolean {
  return !withdrawn && reviewState === 'approved'
}

export interface AggregateCoverageOptions {
  /**
   * 当前快照是否已由「在观测到该星审核通过之后才发起」的 Map 请求结算确认。
   * 为 false 时表示旧快照可能尚未收录刚过审的新星，证据不足，一律不判为聚合覆盖。
   */
  snapshotConfirmed?: boolean
}

/**
 * 判断一颗公开反馈星在当前快照中是否存在明确的聚合覆盖依据：
 * 1. 目标必须是已审核通过且未撤回的公开反馈；
 * 2. 若调用方提供快照因果确认标志，必须为真（证据不足绝不误报）；
 * 3. 目标本身未直接出现在当前快照的公开星点中；
 * 4. 快照中存在至少一颗公开聚合代表星（count > 1、非私有星），与目标落在与服务端完全一致的 32×32 同一分桶内；
 *    跨过网格线的相邻桶即使几何距离极近也绝不匹配，同桶 count === 1 独立星同样不匹配。
 */
export function isCoveredByAggregate(
  target: Pick<StarDetail, 'starId' | 'x' | 'y' | 'withdrawn'> & {original: Pick<StarDetail['original'], 'reviewState'>},
  points: readonly DisplayStar[],
  snapshotBounds: Bounds | null,
  options?: AggregateCoverageOptions,
): boolean {
  if (!isPublicFeedback(target.original.reviewState, target.withdrawn)) return false
  if (options?.snapshotConfirmed === false) return false
  if (!points.length) return false
  const targetCell = aggregateCellOf(target, snapshotBounds)
  if (!targetCell) return false
  let sameCellAggregate = false
  for (const point of points) {
    if (point.privateLabel) continue
    if (point.starId === target.starId) return false
    if ((point.count ?? 1) <= 1) continue
    const cell = aggregateCellOf(point, snapshotBounds)
    if (cell && cell.index === targetCell.index) sameCellAggregate = true
  }
  return sameCellAggregate
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
