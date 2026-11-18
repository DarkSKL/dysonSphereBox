import type {quant} from '../../../../wailsjs/go/models'
import type {FactoryBuildingStatistics, FactoryLineStatistics, FactoryMaterialStatistics, FactoryStatistics} from '../../../types/factory-statistics'

// =============== 量化分析派生数据 ===============
// 职责：把后端整体统计整理为量化分析各图表直接使用的行、块与结论文案；使用：量化分析标签页的图表组件。
// 边界：纯函数，各口径按全厂符合条件的设施统计，不做局部区域统计；不重算后端功率与产量，未知负载保持 null，不把未知解释为零。

/** 判定为零的速率与功率阈值，避免浮点残差被当作真实流量 */
const EPS = 1e-9
const sum = (values: readonly number[]): number => values.reduce((total, value) => total + value, 0)

// =============== 分类与格式 ===============
// 说明：分类顺序与后端 Category 一致；中文名用于图例、行标签和提示。
export const ANALYSIS_CATEGORY_ORDER = ['manufacturing', 'lab', 'belt', 'inserter', 'splitter', 'piler', 'monitor', 'spray-coater', 'storage', 'logistics', 'power', 'other'] as const
const CATEGORY_LABELS: Record<string, string> = {
  manufacturing: '制造设备', lab: '研究站', belt: '传送带', inserter: '分拣器', splitter: '分流器', piler: '集装机',
  monitor: '流速监测器', 'spray-coater': '喷涂机', storage: '储物与储液', logistics: '物流设施', power: '电力设施', other: '其他',
}
export const categoryLabel = (key: string): string => CATEGORY_LABELS[key] ?? '其他'
const categoryRank = (key: string): number => {
  const index = (ANALYSIS_CATEGORY_ORDER as readonly string[]).indexOf(key)
  return index < 0 ? ANALYSIS_CATEGORY_ORDER.length : index
}

/** 按量级保留有效位的千分位数字：≥100 取整，≥10 一位，≥1 两位，小于 1 三位 */
export const formatValue = (value: number): string => {
  const abs = Math.abs(value)
  const digits = abs >= 100 ? 0 : abs >= 10 ? 1 : abs >= 1 ? 2 : abs > 0 ? 3 : 0
  return value.toLocaleString('zh-CN', {maximumFractionDigits: digits})
}
export const formatInteger = (value: number): string => Math.round(value).toLocaleString('zh-CN')
export const formatPercent = (ratio: number, digits = 0): string => `${(ratio * 100).toFixed(digits)}%`
/** 占比文字：零为 0，小于 0.1% 写作 <0.1%，小于 10% 保留一位小数 */
export const formatShare = (share: number): string => (share <= EPS ? '0' : share < 0.001 ? '<0.1%' : formatPercent(share, share < 0.1 ? 1 : 0))
/** 功率拆成数值与单位：小于 1 MW 的非零值换算为 kW，便于和游戏面板对照 */
export const splitPower = (mw: number): {value: string; unit: string} => (Math.abs(mw) > EPS && Math.abs(mw) < 1 ? {value: formatValue(mw * 1000), unit: 'kW'} : {value: formatValue(mw), unit: 'MW'})
export const formatPower = (mw: number): string => { const {value, unit} = splitPower(mw); return `${value} ${unit}` }
export const formatLength = (meters: number): string => (meters >= 1000 ? `${formatValue(meters / 1000)} km` : `${formatInteger(meters)} m`)
/** 提示框 HTML 转义：物品名来自数据集，仍按不可信文本处理 */
export const escapeHtml = (text: string): string => text.replace(/[&<>"']/g, char => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'})[char] ?? char)
/** 量化喷涂模式名称：与工作区喷涂选项一致 */
export const sprayModeLabel = (mode: number): string => ['不喷涂', '加速', '额外产出', '透镜', '分馏'][mode] ?? `模式 ${mode}`

// =============== 范围与总览 ===============
export type AnalysisScope = 'planned' | 'generated'
export const analysisScopeOf = (stats: FactoryStatistics): AnalysisScope => (stats.scope === 'generated-blueprint' ? 'generated' : 'planned')

export interface AnalysisKpi {
  readonly key: string
  readonly label: string
  /** 指标旁的建筑图标（图鉴中文名），帮助快速扫读 */
  readonly icon: string
  readonly value: string
  readonly unit: string
  readonly hint: string
  /** warning：需要处理的持续剩余；quiet：当前范围尚未提供 */
  readonly tone?: 'warning' | 'quiet'
}

// 总览指标：实体范围显示全部设施和占地；量化范围只有制造设备，占地与带长明确标为待生成，不以零代替。
export function buildAnalysisKpis(stats: FactoryStatistics): AnalysisKpi[] {
  const generated = analysisScopeOf(stats) === 'generated'
  const power = stats.power
  const rated = splitPower(power.configuredWorkMw)
  const unknown = power.unknownCount + power.missingCount
  const imports = stats.materials.filter(material => material.importedPerMinute > EPS)
  const space = stats.space ?? null
  return [
    {key: 'buildings', label: generated ? '建筑' : '制造设备', icon: '制造台 Mk.I', value: formatInteger(generated ? stats.buildingCount : stats.productionMachineCount), unit: '座',
      hint: generated ? `制造 ${stats.productionMachineCount} · 产线 ${stats.productionLineCount}` : `产线 ${stats.productionLineCount} · 物流设施待生成`},
    {key: 'power', label: '额定功率', icon: '电力感应塔', value: rated.value, unit: rated.unit,
      hint: power.expectedMw != null ? `按负载预测 ${formatPower(power.expectedMw)}` : `可估算 ${formatPower(power.knownExpectedMw)} · ${unknown} 台负载未知`},
    space
      ? {key: 'area', label: '占地', icon: '地基', value: formatInteger(space.occupiedAreaGrid2), unit: '格²', hint: `包络利用 ${formatPercent(space.utilization)}`}
      : {key: 'area', label: '占地', icon: '地基', value: '—', unit: '', hint: '生成蓝图后统计', tone: 'quiet'},
    space
      ? {key: 'belts', label: '传送带', icon: '传送带', value: formatInteger(space.beltCount), unit: '节', hint: `总长 ${formatLength(space.totalBeltLengthMeters)}`}
      : {key: 'belts', label: '传送带', icon: '传送带', value: '—', unit: '', hint: '生成蓝图后统计', tone: 'quiet'},
    {key: 'imports', label: '外部输入', icon: '行星内物流运输站', value: String(imports.length), unit: '种', hint: `合计 ${formatValue(sum(imports.map(material => material.importedPerMinute)))} 件/min`},
    {key: 'surplus', label: '持续剩余', icon: '小型储物仓', value: String(stats.surplusItemCount), unit: '种', hint: stats.hasSurplus ? '需接出口持续外运' : '全部内部消化或交付', tone: stats.hasSurplus ? 'warning' : undefined},
  ]
}

// =============== 整体占比 ===============
// 说明：建筑与电力按设施原型统计，生产按产出物品统计，空间按最终蓝图包络拆成建筑占地、运输线路与空地；
//       建筑数量排除传送带和分拣器，电力仍包含所有耗电设施。
export type ShareMetric = 'building' | 'power' | 'production' | 'space'
export interface ShareMetricInfo {
  readonly key: ShareMetric
  /** 卡片编号，用 Oxanium 等宽数字显示 */
  readonly code: string
  readonly label: string
  readonly short: string
}
export const SHARE_METRICS: readonly ShareMetricInfo[] = [
  {key: 'building', code: '01', label: '建筑占比', short: '建筑'},
  {key: 'power', code: '02', label: '电力占比', short: '电力'},
  {key: 'production', code: '03', label: '生产占比', short: '生产'},
  {key: 'space', code: '04', label: '空间利用率', short: '空间'},
]
interface ShareEntryBase {
  /** 物品或建筑 ID；空间构成没有对应物品，用负数占位，只作列表键。 */
  readonly itemId: number
  readonly name: string
  /** 图鉴图标名：设施与物品为自身名称，空间构成取代表性的建筑或材料。 */
  readonly icon: string
  /** 非适用口径为零，浮层只展示条目所属口径。 */
  readonly shares: Readonly<Record<ShareMetric, number>>
}
export interface BuildingShareEntry extends ShareEntryBase {
  readonly kind: 'building'
  readonly category: string
  /** 原始实体数量；保留物流设施台数供详情使用，不直接作为建筑占比的计数。 */
  readonly count: number
  /** 额定工作功率（含增产附加），MW */
  readonly powerMw: number
  readonly group: FactoryBuildingStatistics
}
export interface ProductionShareEntry extends ShareEntryBase {
  readonly kind: 'product'
  /** 按物品汇总的期望产出，包含会在后续工序中消耗的半成品；单位件/min。 */
  readonly productionRate: number
}
/** 空间构成的三部分：固定设施投影、只被传送带与分拣器覆盖的格子、包络内的空地。 */
export type SpaceRole = 'building' | 'logistics' | 'empty'
export interface SpaceShareEntry extends ShareEntryBase {
  readonly kind: 'space'
  readonly role: SpaceRole
  /** 格网面积（格²），与后端空间统计同一保守近似口径，不是真实地表面积 */
  readonly areaGrid2: number
}
export type ShareEntry = BuildingShareEntry | ProductionShareEntry | SpaceShareEntry
export interface ShareSpaceSummary {
  readonly envelopeGrid2: number
  readonly occupiedGrid2: number
  /** 后端给出的占地 ÷ 包络；保守轮廓可能略超 1，原样展示 */
  readonly utilization: number
}
export interface ShareComposition {
  readonly entries: readonly BuildingShareEntry[]
  readonly productionEntries: readonly ProductionShareEntry[]
  /** 固定顺序：建筑占地、运输线路、空地；量化阶段尚无布局时为空 */
  readonly spaceEntries: readonly SpaceShareEntry[]
  readonly space: ShareSpaceSummary | null
  readonly totals: Readonly<Record<ShareMetric, number>>
}
/** 建筑占比的有效数量；按分类排除所有等级的传送带和分拣器，不改写原始台数与功率。 */
const buildingShareCountOf = (entry: Pick<BuildingShareEntry, 'category' | 'count'>): number => (entry.category === 'belt' || entry.category === 'inserter' ? 0 : entry.count)
/** 当前口径的统计值；平台分块和环形图共用同一筛选规则，电力与生产不套用建筑数量的排除规则。 */
export const shareValueOf = (entry: ShareEntry, metric: ShareMetric): number => {
  if (entry.kind === 'space') return metric === 'space' ? entry.areaGrid2 : 0
  if (entry.kind === 'product') return metric === 'production' ? entry.productionRate : 0
  if (metric === 'building') return buildingShareCountOf(entry)
  return metric === 'power' ? entry.powerMw : 0
}
/** 条目所属口径：设施对应建筑与电力，物品对应生产，空间构成只对应空间利用率。 */
export const shareMetricsOf = (entry: ShareEntry): readonly ShareMetric[] => (entry.kind === 'product' ? ['production'] : entry.kind === 'space' ? ['space'] : ['building', 'power'])
/** 口径使用的条目列表：建筑与电力共用设施列表，同一下标指向同一设施，悬停与固定可以在两者间联动。 */
export const shareListOf = (metric: ShareMetric): 'facility' | 'product' | 'space' => (metric === 'production' ? 'product' : metric === 'space' ? 'space' : 'facility')
/** 条目在指定口径下的读数文字（座、功率、件/min 或格²） */
export const shareReadingOf = (entry: ShareEntry, metric: ShareMetric): string => {
  if (entry.kind === 'space') return `${formatInteger(entry.areaGrid2)} 格²`
  if (entry.kind === 'product') return `${formatValue(entry.productionRate)} 件/min`
  return metric === 'power' ? formatPower(entry.powerMw) : `${formatInteger(entry.count)} 座`
}

// 空间构成的显示名与图标：建筑占地取制造台、运输线路取传送带、空地取地基。
const SPACE_PARTS: readonly {role: SpaceRole; itemId: number; name: string; icon: string}[] = [
  {role: 'building', itemId: -1, name: '建筑占地', icon: '制造台 Mk.I'},
  {role: 'logistics', itemId: -2, name: '运输线路', icon: '传送带'},
  {role: 'empty', itemId: -3, name: '空地', icon: '地基'},
]

// 生产直接复用后端按物品 ID 汇总的 Produced：保留内部消耗的半成品、最终产物和联产物，不以工厂净剩余或设备原型筛选。
// 空间把包络面积拆成三份：固定设施占地是全部占地的子集，运输线路为其余占地，空地为包络减去占地；占地超出包络时空地记零。
// 边界：后端已合并跨线产出并扣除同线回流；这里只计算占比，不重新抵消跨线消耗或推导满负荷产量。
export function buildShareComposition(stats: FactoryStatistics): ShareComposition {
  const products = stats.materials.filter(material => Number.isFinite(material.producedPerMinute) && material.producedPerMinute > EPS)
    .sort((a, b) => a.itemId - b.itemId)
  const groups = [...stats.buildings].sort((a, b) => categoryRank(a.category) - categoryRank(b.category) || b.count - a.count || a.itemId - b.itemId)
  const layout = stats.space ?? null
  const occupied = Math.max(0, layout?.occupiedAreaGrid2 ?? 0)
  const fixedArea = Math.min(occupied, Math.max(0, layout?.buildingFootprintAreaGrid2 ?? 0))
  const areas: Record<SpaceRole, number> = {building: fixedArea, logistics: occupied - fixedArea, empty: Math.max(0, (layout?.envelopeAreaGrid2 ?? 0) - occupied)}
  const space = layout && layout.envelopeAreaGrid2 > EPS ? {envelopeGrid2: layout.envelopeAreaGrid2, occupiedGrid2: occupied, utilization: layout.utilization} : null
  const totals: Record<ShareMetric, number> = {
    building: sum(groups.map(buildingShareCountOf)),
    power: sum(groups.map(group => group.power.configuredWorkMw)),
    production: sum(products.map(material => material.producedPerMinute)),
    space: space ? sum(Object.values(areas)) : 0,
  }
  const ratio = (value: number, total: number): number => (total > EPS ? value / total : 0)
  const entries = groups.map<BuildingShareEntry>(group => ({
    kind: 'building', itemId: group.itemId, name: group.name, icon: group.name, category: group.category, count: group.count, powerMw: group.power.configuredWorkMw, group,
    shares: {building: ratio(buildingShareCountOf(group), totals.building), power: ratio(group.power.configuredWorkMw, totals.power), production: 0, space: 0},
  }))
  const productionEntries = products.map<ProductionShareEntry>(material => ({
    kind: 'product', itemId: material.itemId, name: material.name, icon: material.name, productionRate: material.producedPerMinute,
    shares: {building: 0, power: 0, production: ratio(material.producedPerMinute, totals.production), space: 0},
  }))
  const spaceEntries = space ? SPACE_PARTS.map<SpaceShareEntry>(part => ({
    kind: 'space', ...part, areaGrid2: areas[part.role],
    shares: {building: 0, power: 0, production: 0, space: ratio(areas[part.role], totals.space)},
  })) : []
  return {entries, productionEntries, spaceEntries, space, totals}
}

/** 最大余数法：按数值把 total 块分给各项，合计恰好为 total；零值项永远不分块，全部为零时返回全零 */
export function allocateTiles(values: readonly number[], total = 100): number[] {
  const whole = sum(values)
  if (whole <= EPS) return values.map(() => 0)
  const raw = values.map(value => (value > EPS ? value / whole * total : 0))
  const tiles = raw.map(value => Math.floor(value))
  let remaining = total - sum(tiles)
  const order = raw.map((value, index) => ({index, fraction: value - Math.floor(value)})).sort((a, b) => b.fraction - a.fraction || values[b.index] - values[a.index] || a.index - b.index)
  for (const {index, fraction} of order) {
    if (remaining <= 0 || fraction <= 0) break
    tiles[index]++
    remaining--
  }
  return tiles
}

export interface PlatformTile {
  readonly column: number
  readonly row: number
  /** 当前口径传入的 entries 下标，可对应设施或产出物品。 */
  readonly entry: number
}
/** 平台方块排布：从后角（东北）起按行蛇形铺开，order 中靠前（份额大）的项先占后排，同一项的方块连成一片 */
export function layoutPlatform(tiles: readonly number[], order: readonly number[], columns = 10, rows = 10): PlatformTile[] {
  const cells: {column: number; row: number}[] = []
  for (let row = rows - 1; row >= 0; row--) {
    const reversed = (rows - 1 - row) % 2 === 0
    for (let step = 0; step < columns; step++) cells.push({column: reversed ? columns - 1 - step : step, row})
  }
  const placed: PlatformTile[] = []
  let cursor = 0
  for (const entry of order) {
    for (let n = 0; n < (tiles[entry] ?? 0) && cursor < cells.length; n++) placed.push({...cells[cursor++], entry})
  }
  return placed
}
/** 区域锚点：离该项方块重心最近的方块，供建筑插画站立；没有方块时返回 null */
export function regionAnchor(placed: readonly PlatformTile[], entry: number): {column: number; row: number} | null {
  const own = placed.filter(tile => tile.entry === entry)
  if (!own.length) return null
  const cx = sum(own.map(tile => tile.column)) / own.length
  const cy = sum(own.map(tile => tile.row)) / own.length
  return own.reduce((best, tile) => (Math.hypot(tile.column - cx, tile.row - cy) < Math.hypot(best.column - cx, best.row - cy) ? tile : best))
}

// =============== 电力构成 ===============
// 各阶段分别给前段加工、中间工序和目标产线分配工作负载。数值是演示用的启停比例，
// 常规阶段从待机功率叠加参与工序的预测增量。满载阶段直接取各型号的配置额定工作功率。
export const POWER_STAGES = [
  {start: 0, name: '全厂待机', load: [0, 0, 0]},
  {start: 18, name: '前段启动', load: [.65, 0, 0]},
  {start: 38, name: '中段接续', load: [.65, .8, 0]},
  {start: 54, name: '末段启动', load: [.65, .8, .65]},
  {start: 70, name: '全厂满载', load: 'full'},
  {start: 82, name: '分批停机', load: [0, .35, .55]},
  {start: 94, name: '恢复待机', load: [0, 0, 0]},
] as const

// 每个阶段先保持当前负载，再用两端斜率为零的曲线过渡。图形与发电满载区间共享同一个时间尺度。
export const powerStageHoldEnd = (start: number, end: number): number => start + (end - start) * .35

/** 计算并网机组随模拟负载出力、受蓝图额定容量限制时的峰值区间。未统计机组时仅给出供电需求。 */
function peakGenerationWindow(totals: readonly number[], demandPeak: number, ratedMw: number) {
  if (demandPeak <= 0) return null
  const hasCapacity = Number.isFinite(ratedMw) && ratedMw > 0
  const mw = hasCapacity ? Math.min(demandPeak, ratedMw) : demandPeak
  const first = totals.findIndex(value => value >= mw)
  let last = totals.length - 1
  while (totals[last] < mw) last--

  // 在上升沿和下降沿分别反求负载达到峰值的时刻。两控制点位于横轴三等分处，
  // 所以横坐标匀速，纵坐标按 t²(3−2t) 插值，与画布的三次贝塞尔路径一致。
  const crossing = (i: number): number => {
    const end = POWER_STAGES[i + 1].start
    const hold = powerStageHoldEnd(POWER_STAGES[i].start, end)
    const ratio = (mw - totals[i]) / (totals[i + 1] - totals[i])
    if (ratio <= 0) return hold
    if (ratio >= 1) return end
    let lo = 0
    let hi = 1
    for (let step = 0; step < 24; step++) {
      const t = (lo + hi) / 2
      if (t * t * (3 - 2 * t) < ratio) lo = t
      else hi = t
    }
    return hold + (end - hold) * (lo + hi) / 2
  }
  return {mw, hasCapacity, start: first === 0 ? 0 : crossing(first - 1), end: last === totals.length - 1 ? 100 : crossing(last)}
}

/** 按净产出与净输入区分目标、使用自产中间件的产线和前段产线；同型号跨工序时分别归集工作增量。 */
export function buildPowerProfile(stats: FactoryStatistics) {
  const targets = new Set(stats.materials.filter(item => item.targetPerMinute > EPS).map(item => item.itemId))
  const produced = new Set(stats.lines.flatMap(line => line.netOutputs.map(item => item.itemId)))
  const increments = new Map<number, number[]>()
  for (const line of stats.lines) {
    const work = increments.get(line.machineId) ?? [0, 0, 0]
    const targetLine = line.netOutputs.some(item => targets.has(item.itemId))
    const usesIntermediate = line.netInputs.some(item => produced.has(item.itemId))
    const phase = targetLine ? 2 : usesIntermediate ? 1 : 0
    if (line.power.expectedMw != null) work[phase] += Math.max(0, line.power.expectedMw - line.power.idleMw)
    increments.set(line.machineId, work)
  }
  const groups = stats.buildings.filter(({power}) => power.configuredWorkMw > EPS || power.idleMw > EPS)
    .sort((a, b) => b.power.configuredWorkMw - a.power.configuredWorkMw || a.itemId - b.itemId)
  let totals = POWER_STAGES.map(() => 0)
  const bands = groups.map(group => {
    const {idleMw, expectedMw, configuredWorkMw} = group.power
    // 额定值已按设备数量和增产配置汇总；只有待机功率的设施在满载阶段仍维持基线。
    const fullLoadMw = Math.max(idleMw, configuredWorkMw)
    // 常规阶段，无产线的设施随前段启动，未知运行负载的部分仅计待机。
    const work = increments.get(group.itemId) ?? [Math.max(0, (expectedMw ?? idleMw) - idleMw), 0, 0]
    const readings = POWER_STAGES.map(({load}) => load === 'full'
      ? fullLoadMw
      : Math.min(fullLoadMw, idleMw + sum(work.map((mw, i) => mw * load[i]))))
    const lower = totals
    totals = readings.map((mw, i) => lower[i] + mw)
    return {group, lower, upper: totals, readings}
  })
  const peakMw = Math.max(...totals)
  const peakProgress = POWER_STAGES[totals.indexOf(peakMw)].start
  const ratedGenerationMw = stats.thermal?.ratedGenerationMw ?? 0
  const generation = peakGenerationWindow(totals, peakMw, ratedGenerationMw)
  return {bands, totals, idleMw: totals[0], peakMw, peakProgress, constant: peakMw === totals[0], ratedGenerationMw, generation}
}

// =============== 产线负载 ===============
export interface LineRow {
  readonly index: number
  /** 产线主产物名称，同时作为图标名 */
  readonly name: string
  readonly machineName: string
  readonly count: number
  readonly exact: number
  readonly utilization: number
  /** 取整多出的设备台数 count − exact */
  readonly idle: number
  readonly mode: number
  readonly points: number
  readonly configuredMw: number
  readonly expectedMw: number | null
  readonly line: FactoryLineStatistics
}

// 产线负载：主产物优先取同一方案的量化产线，方案不一致时退回统计净输出；按负载升序，把取整浪费最大的产线排在前面。
export function buildLineRows(stats: FactoryStatistics, plan: quant.Result | null, itemName: (id: number) => string): LineRow[] {
  const productions = plan && plan.planId === stats.planId ? plan.productions ?? [] : []
  return stats.lines.map<LineRow>(line => {
    const mainItemId = productions[line.productionIndex]?.mainItemId
    const name = mainItemId ? itemName(mainItemId) : line.netOutputs[0]?.name ?? line.outputs[0]?.name ?? `产线 ${line.productionIndex + 1}`
    return {index: line.productionIndex, name, machineName: line.machineName, count: line.count, exact: line.exactCount, utilization: line.utilization, idle: Math.max(0, line.count - line.exactCount),
      mode: line.proliferatorMode, points: line.points, configuredMw: line.power.configuredWorkMw, expectedMw: line.power.expectedMw ?? null, line}
  }).sort((a, b) => a.utilization - b.utilization || a.index - b.index)
}

// =============== 物料边界 ===============
export interface BoundaryRow {
  readonly itemId: number
  readonly name: string
  readonly imported: number
  readonly target: number
  readonly surplus: number
  readonly material: FactoryMaterialStatistics
}

// 工厂边界：只保留有外部输入、目标交付或持续剩余的物料；输入在前按量降序，其后为输出按量降序。纯内部中间品不进入边界图。
export function buildBoundaryRows(stats: FactoryStatistics): BoundaryRow[] {
  const rows = stats.materials
    .filter(material => material.importedPerMinute > EPS || material.targetPerMinute > EPS || material.surplusPerMinute > EPS)
    .map<BoundaryRow>(material => ({itemId: material.itemId, name: material.name, imported: material.importedPerMinute > EPS ? material.importedPerMinute : 0,
      target: material.targetPerMinute > EPS ? material.targetPerMinute : 0, surplus: material.surplusPerMinute > EPS ? material.surplusPerMinute : 0, material}))
  return rows.sort((a, b) => (b.imported > 0 ? 1 : 0) - (a.imported > 0 ? 1 : 0) || b.imported - a.imported || (b.target + b.surplus) - (a.target + a.surplus) || a.itemId - b.itemId)
}

// =============== 等距投影 ===============
// 说明：与平面预览一致的纪念碑谷式 2:1 等距视角；平台东向为列、北向为行，西南角在画面最下方。
export interface IsoFrame {
  /** 半格宽度（像素）；半格高度为其一半 */
  readonly unit: number
  readonly originX: number
  readonly originY: number
}
/** 网格点（列、行，可为小数）投影到画面像素；lift 为向上抬起的像素高度 */
export const isoPoint = (frame: IsoFrame, gx: number, gy: number, lift = 0): [number, number] => [frame.originX + (gx - gy) * frame.unit, frame.originY - (gx + gy) * frame.unit / 2 - lift]

/** 让整块平台居中放入画布；liftUnits 为平台上最高物体的高度、thicknessUnits 为平台厚度，均以 unit 计 */
export function fitIsoFrame(columns: number, rows: number, width: number, height: number, padding: number, liftUnits: number, thicknessUnits: number, maxUnit = 28): IsoFrame {
  const span = Math.max(1, columns + rows)
  const unit = Math.max(1.5, Math.min(maxUnit, (width - padding * 2) / span, (height - padding * 2) / (span / 2 + liftUnits + thicknessUnits)))
  return {unit, originX: width / 2 - (columns - rows) * unit / 2, originY: height / 2 + span * unit / 4 + liftUnits * unit / 2 - thicknessUnits * unit / 2}
}
