import type {BoundaryRow} from '../factory-analysis'

// =============== 物料边界布局 ===============
// 职责：把跨越工厂边界的物料流（外部输入、目标交付、持续剩余）排成单位方格网格，或排成面积与速率成正比的矩形树图；
// 使用：物料统计页的「网格」与「方块」两种视图；
// 边界：纯布局计算，不读取主题或 DOM；网格每段至少一格，方块按速率分配并为小流量保留图标空间，精确数值由浮层给出。

export type BoundaryFlow = 'input' | 'target' | 'surplus'
export const BOUNDARY_FLOWS: readonly {readonly key: BoundaryFlow; readonly label: string}[] = [
  {key: 'input', label: '外部输入'},
  {key: 'target', label: '目标交付'},
  {key: 'surplus', label: '持续剩余'},
]
export const boundaryRateOf = (row: BoundaryRow, flow: BoundaryFlow): number => (flow === 'input' ? row.imported : flow === 'target' ? row.target : row.surplus)

export interface BoundarySegment {
  readonly flow: BoundaryFlow
  readonly row: BoundaryRow
  /** 该物料在此流向上的速率，件/min */
  readonly rate: number
}
/** 按流向分段：外部输入 → 目标交付 → 持续剩余，同一流向内按速率从大到小；同一物料可同时出现在交付与剩余两段。 */
export function boundarySegments(rows: readonly BoundaryRow[]): BoundarySegment[] {
  return BOUNDARY_FLOWS.flatMap(({key}) => rows
    .map(row => ({flow: key, row, rate: boundaryRateOf(row, key)}))
    .filter(segment => segment.rate > 1e-9)
    .sort((a, b) => b.rate - a.rate || a.row.itemId - b.row.itemId))
}

// =============== 单位方格网格 ===============
export interface WaffleLayout {
  /** 每格代表的速率，件/min；取 1、2、5 × 10ⁿ 的整齐数值 */
  readonly unit: number
  readonly size: number
  readonly gap: number
  readonly columns: number
  /** 按阅读顺序排列的格子：分段下标，null 为空槽 */
  readonly cells: readonly (number | null)[]
}
const UNITS = Array.from({length: 10}, (_, step) => step - 2).flatMap(exponent => [1, 2, 5].map(base => base * 10 ** exponent))
/** 同屏格子上限：保证图标仍可辨认，也控制节点数量 */
const MAX_CELLS = 600
// 格子边长 22–40px：与参考设计的密度相当（约 34px 一格），图标约为格子的 2/3。
const MIN_SIZE = 22
const MAX_SIZE = 40
const cellCountOf = (segments: readonly BoundarySegment[], unit: number): number => segments.reduce((sum, segment) => sum + Math.max(1, Math.round(segment.rate / unit)), 0)
const gapOf = (size: number): number => Math.max(3, Math.round(size * 0.1))
const capacityOf = (size: number, width: number, height: number): {columns: number; rows: number} => {
  const gap = gapOf(size)
  return {columns: Math.max(1, Math.floor((width + gap) / (size + gap))), rows: Math.max(1, Math.floor((height + gap) / (size + gap)))}
}
/** 能放下 count 个格子的最大边长（偶数像素）；最小格也放不下时返回 null */
const largestSize = (count: number, width: number, height: number): number | null => {
  for (let size = MAX_SIZE; size >= MIN_SIZE; size -= 2) {
    const {columns, rows} = capacityOf(size, width, height)
    if (columns * rows >= count) return size
  }
  return null
}

/**
 * 网格排布：逐个尝试整齐单位，每个单位取能放下全部格子的最大边长，按「填满程度 × 格子大小」打分（小于 30px 的格子扣分），
 * 取最高分，同分时取更细的单位。格子因此尽量多、尽量铺满画幅，空仓位只剩最后一行的零头。物料段多到最小格也放不下时，每段一格并纵向滚动。
 */
export function waffleLayout(segments: readonly BoundarySegment[], width: number, height: number): WaffleLayout | null {
  if (!segments.length || width < 80 || height < 40) return null
  let best: {unit: number; size: number; score: number} | null = null
  for (const unit of UNITS) {
    const count = cellCountOf(segments, unit)
    if (count > MAX_CELLS) continue
    const size = largestSize(count, width, height)
    if (size === null) continue
    const {columns, rows} = capacityOf(size, width, height)
    const score = count / (columns * rows) * Math.min(1, size / 30)
    if (!best || score > best.score + 1e-9) best = {unit, size, score}
  }
  const unit = best?.unit ?? UNITS[UNITS.length - 1]
  const size = best?.size ?? MIN_SIZE
  const {columns, rows} = capacityOf(size, width, height)
  const cells: (number | null)[] = []
  segments.forEach((segment, index) => { for (let n = Math.max(1, Math.round(segment.rate / unit)); n > 0; n--) cells.push(index) })
  const total = Math.ceil(Math.max(cells.length, columns * rows) / columns) * columns
  while (cells.length < total) cells.push(null)
  return {unit, size, gap: gapOf(size), columns, cells}
}

// =============== 矩形树图 ===============
export interface BlockRect { readonly x: number; readonly y: number; readonly width: number; readonly height: number }
export interface TreemapLayout {
  readonly height: number
  readonly groups: readonly {readonly flow: BoundaryFlow; readonly rect: BlockRect}[]
  readonly blocks: readonly {readonly segment: number; readonly rect: BlockRect}[]
}
/** 最差长宽比：行内面积越均匀、越接近正方形，值越小 */
const worstRatio = (areas: readonly number[], side: number): number => {
  const total = areas.reduce((sum, area) => sum + area, 0)
  const max = Math.max(...areas)
  const min = Math.min(...areas)
  return Math.max(side * side * max / (total * total), total * total / (side * side * min))
}
/** Squarified 排布（Bruls 等）：values 需从大到小，返回与输入同序的矩形，面积与数值严格成正比 */
function squarify(values: readonly number[], rect: BlockRect): BlockRect[] {
  const total = values.reduce((sum, value) => sum + value, 0)
  if (!(total > 0) || rect.width <= 0 || rect.height <= 0) return values.map(() => ({...rect, width: 0, height: 0}))
  const areas = values.map(value => value / total * rect.width * rect.height)
  const result: BlockRect[] = []
  let {x, y, width, height} = rect
  let start = 0
  while (start < areas.length) {
    const side = Math.min(width, height)
    let end = start + 1
    while (end < areas.length && worstRatio(areas.slice(start, end + 1), side) <= worstRatio(areas.slice(start, end), side)) end++
    const row = areas.slice(start, end)
    const thickness = row.reduce((sum, area) => sum + area, 0) / side
    let offset = 0
    for (const area of row) {
      const length = area / thickness
      result.push(width >= height ? {x, y: y + offset, width: thickness, height: length} : {x: x + offset, y, width: length, height: thickness})
      offset += length
    }
    if (width >= height) { x += thickness; width -= thickness } else { y += thickness; height -= thickness }
    start = end
  }
  return result
}
const inset = (rect: BlockRect, amount: number): BlockRect => ({x: rect.x + amount, y: rect.y + amount, width: Math.max(0, rect.width - amount * 2), height: Math.max(0, rect.height - amount * 2)})

// 先锁定按比例会过小的项，其余空间仍按真实速率分配。调用方已为全部下限预留容量。
function proportionalSizes(weights: readonly number[], minimums: readonly number[], space: number): number[] {
  const sizes = new Array<number>(weights.length)
  let pending = weights.map((_, index) => index)
  while (pending.length) {
    const total = pending.reduce((sum, i) => sum + weights[i], 0)
    const scale = space / total
    const small = pending.filter(i => weights[i] * scale < minimums[i])
    if (!small.length) {
      for (const i of pending) sizes[i] = weights[i] * scale
      break
    }
    for (const i of small) {
      sizes[i] = minimums[i]
      space -= sizes[i]
    }
    pending = pending.filter(i => sizes[i] === undefined)
  }
  return sizes
}

// 常规树图能容纳所有图标时沿用原布局；否则按行分配，每行的高度和行内每块的宽度都有下限。
function iconSizedBlocks(weights: number[], rect: BlockRect, minimum: number): BlockRect[] {
  const cells = squarify(weights, rect)
  if (cells.every(cell => Math.min(cell.width, cell.height) >= minimum)) return cells
  const maxRows = Math.max(1, Math.floor(rect.height / minimum))
  const maxColumns = Math.max(1, Math.floor(rect.width / minimum))
  const columns = Math.min(maxColumns, Math.max(Math.ceil(weights.length / maxRows), Math.round(Math.sqrt(weights.length * rect.width / rect.height))))
  const rows: number[][] = []
  for (let i = 0; i < weights.length; i += columns) rows.push(weights.slice(i, i + columns))
  const heights = proportionalSizes(rows.map(row => row.reduce((a, b) => a + b, 0)), rows.map(() => minimum), rect.height)
  let y = rect.y
  return rows.flatMap((row, index) => {
    const height = heights[index]
    const top = y
    y += height
    let x = rect.x
    return proportionalSizes(row, row.map(() => minimum), rect.width).map(width => {
      const cell = {x, y: top, width, height}
      x += width
      return cell
    })
  })
}

/** 方块排布：三种流向按总量切成并列的条带（宽画幅自左向右：输入 → 交付 → 剩余），条带内物料按速率 squarify；块间留 gap，条带间留 groupGap。 */
export function treemapLayout(segments: readonly BoundarySegment[], width: number, height: number, gap = 3, groupGap = 10): TreemapLayout | null {
  if (!segments.length || width < 80 || height < 40) return null
  const groups = BOUNDARY_FLOWS
    .map(({key}) => ({flow: key, members: segments.map((segment, index) => ({segment, index})).filter(item => item.segment.flow === key)}))
    .filter(group => group.members.length)
  const totals = groups.map(group => group.members.reduce((sum, item) => sum + item.segment.rate, 0))
  // 每块至少容纳 20px 图标和两侧各 6px 内边距；块间距单独计算。
  const minimum = 32 + gap
  const gutters = groupGap * (groups.length - 1)
  const minimumSpans = (cross: number) => groups.map(group => Math.ceil(group.members.length / Math.max(1, Math.floor(cross / minimum))) * minimum)
  const sum = (values: number[]) => values.reduce((a, b) => a + b, 0)
  const widths = minimumSpans(height)
  const horizontal = width >= height && width >= gutters + sum(widths)
  const minima = horizontal ? widths : minimumSpans(width)
  const plotHeight = horizontal ? height : Math.max(height, gutters + sum(minima))
  const lengths = proportionalSizes(totals, minima, (horizontal ? width : plotHeight) - gutters)
  let cursor = 0
  const placedGroups: {flow: BoundaryFlow; rect: BlockRect}[] = []
  const blocks: {segment: number; rect: BlockRect}[] = []
  groups.forEach((group, order) => {
    const length = lengths[order]
    const rect: BlockRect = horizontal ? {x: cursor, y: 0, width: length, height} : {x: 0, y: cursor, width, height: length}
    cursor += length + groupGap
    placedGroups.push({flow: group.flow, rect})
    iconSizedBlocks(group.members.map(item => item.segment.rate), rect, minimum).forEach((cell, rank) => blocks.push({segment: group.members[rank].index, rect: inset(cell, gap / 2)}))
  })
  return {height: plotHeight, groups: placedGroups, blocks}
}
