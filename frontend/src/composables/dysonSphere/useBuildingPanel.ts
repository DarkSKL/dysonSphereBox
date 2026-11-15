import {computed, ref, watch, type Ref} from 'vue'
import type {blueprint, quant} from '../../../wailsjs/go/models'
import buildingData from '../../data/encyclopedia/building.json'
import type {AlgoVizBuilding, AlgoVizSelection} from '../../packages/algorithm-viz/src'
import {factoryEncyclopediaName} from '../../pages/services/dysonSphere/factory-icons'
import type {BuildingPanel, BuildingPanelHead, BuildingPanelItem, LabProliferatorEffect, StationParameters} from '../../types/building-panel'
import {parseStationParameters, STATION_FACTS} from '../../utils/station-parameters'
import type {QuantWorkspace} from './useQuantWorkspace'

// =============== 选中建筑的游戏风格面板 ===============
// 说明：把算法可视化中选中的建筑整理为面板数据：矩阵研究站与物流运输站对应游戏内的建筑窗口，其余建筑对应光标悬停浮窗。
// 数据：工作功率、配方与设备速度取量化数据集，数据集没有的待机功率（及缺失的工作功率）取图鉴建筑属性；产线负荷与增产效果取当前量化方案；
//       物流塔槽位与设置按记录下标读取蓝图参数。
// 边界：只读展示；面板中的件数为每次制作的配方件数，不是游戏运行时库存。生成进行中或临时建筑不读取蓝图参数，避免读到上一份结果的记录。

const LAB_ITEMS = new Set([2901, 2902])

// 图鉴建筑功率（瓦），按图鉴中文名与稳定 ID 索引
type EncyclopediaBuilding = {id: string; name?: {zh?: string}; properties?: {label: string; value: number}[]}
const encyclopediaPower = new Map<string, {work: number; idle: number}>()
for (const entry of buildingData as unknown as EncyclopediaBuilding[]) {
  const value = (label: string): number => entry.properties?.find(property => property.label === label)?.value ?? 0
  const power = {work: value('工作功率'), idle: value('待机功率')}
  encyclopediaPower.set(entry.id, power)
  if (entry.name?.zh) encyclopediaPower.set(entry.name.zh, power)
}

/** 增产剂效果表，与后端 helper/quant/graph.go effects 一致：返回产出倍率、速度倍率与耗电倍率 */
const sprayEffects = (points: number): {increase: number; speed: number; power: number} => {
  if (points <= 0) return {increase: 1, speed: 1, power: 1}
  if (points === 1) return {increase: 1.125, speed: 1.25, power: 1.3}
  return {increase: 1.15 + .025 * points, speed: 1 + .25 * points, power: .9 + .4 * points}
}

const formatCoordinate = (value: number | undefined): string => (value ?? 0).toFixed(value !== undefined && Number.isInteger(value) ? 0 : 2)

interface StationState {
  readonly id: string
  readonly parameters: StationParameters | null
  readonly loading: boolean
  readonly notice: string
}

export interface BuildingPanelOptions {
  /** 产线归属文案 */
  readonly owner: (building: AlgoVizBuilding) => string
  /** 按记录下标读取当前蓝图的建筑记录 */
  readonly recordAt: (index: number) => Promise<blueprint.Building | null>
}

export const useBuildingPanel = (w: QuantWorkspace, selection: Ref<AlgoVizSelection | null>, options: BuildingPanelOptions) => {
  const items = computed(() => new Map((w.detail?.items ?? []).map(item => [item.id, item] as const)))
  const recipes = computed(() => new Map((w.detail?.recipes ?? []).map(recipe => [recipe.id, recipe] as const)))

  // 量化方案中的产线：优先按建筑所属产线，配方不一致时按配方与机器查找；导入回放没有方案
  const productionOf = (building: AlgoVizBuilding): quant.Production | undefined => {
    if (w.replay || !w.plan) return undefined
    const productions = w.plan.productions ?? []
    const grouped = (building.group ?? -1) >= 0 ? productions[building.group as number] : undefined
    if (grouped && (!building.recipeId || grouped.recipeId === building.recipeId)) return grouped
    return building.recipeId ? productions.find(production => production.recipeId === building.recipeId && production.machineId === building.itemId) : undefined
  }
  const sprayOf = (production: quant.Production | undefined): {mode: number; effects: ReturnType<typeof sprayEffects>} => {
    const mode = production?.proliferatorMode ?? 0
    return {mode, effects: mode === 1 || mode === 2 ? sprayEffects(production?.increasePoints ?? 0) : sprayEffects(0)}
  }
  const recipeItems = (recipe: quant.Recipe | undefined): {products: BuildingPanelItem[]; inputs: BuildingPanelItem[]} => ({
    products: (recipe?.results ?? []).map((itemId, index) => ({itemId, name: w.itemName(itemId), role: 'product' as const, count: recipe?.resultCounts[index]})),
    inputs: (recipe?.items ?? []).map((itemId, index) => ({itemId, name: w.itemName(itemId), role: 'input' as const, count: recipe?.itemCounts[index]})),
  })

  // =============== 物流塔参数 ===============
  // 说明：选中物流塔后按记录下标读取蓝图参数；选中项、蓝图编码或生成状态变化时重新读取，过期的读取结果直接丢弃。
  const station = ref<StationState | null>(null)
  let stationVersion = 0
  watch(() => {
    const value = selection.value
    return [value?.id, value?.building.itemId, value?.building.index, Boolean(value?.building.temporary), w.code, w.generating] as const
  }, async ([id, itemId, index, temporary, code, generating]) => {
    const version = ++stationVersion
    const facts = itemId === undefined ? undefined : STATION_FACTS[itemId]
    if (!id || !facts) { station.value = null; return }
    const unavailable = temporary ? '临时建筑尚未写入蓝图参数' : generating ? '生成完成后显示站点参数' : index === undefined || !code ? '没有对应的蓝图记录' : ''
    if (unavailable) { station.value = {id, parameters: null, loading: false, notice: unavailable}; return }
    station.value = {id, parameters: null, loading: true, notice: '正在读取站点参数'}
    try {
      const record = await options.recordAt(index as number)
      if (version !== stationVersion) return
      const parameters = record && record.itemId === itemId ? parseStationParameters(facts, record.parameters) : null
      station.value = {id, parameters, loading: false, notice: parameters ? '' : '蓝图记录中没有可识别的站点参数'}
    } catch (caught) {
      if (version === stationVersion) station.value = {id, parameters: null, loading: false, notice: `读取站点参数失败：${caught instanceof Error ? caught.message : String(caught)}`}
    }
  }, {immediate: true})

  // =============== 面板数据 ===============
  const panel = computed<BuildingPanel | null>(() => {
    const value = selection.value
    if (!value) return null
    const building = value.building
    const head: BuildingPanelHead = {
      id: value.id,
      itemId: building.itemId,
      name: w.itemName(building.itemId),
      owner: options.owner(building),
      record: building.temporary ? '生成中的临时建筑' : building.index !== undefined ? `正式记录 #${building.index}` : '正式建筑',
      temporary: Boolean(building.temporary),
      coords: `${formatCoordinate(building.x)}, ${formatCoordinate(building.y)}, ${formatCoordinate(building.z)}`,
    }
    const machine = items.value.get(building.itemId)
    const recipe = building.recipeId ? recipes.value.get(building.recipeId) : undefined
    const production = productionOf(building)
    const spray = sprayOf(production)
    const reference = encyclopediaPower.get(factoryEncyclopediaName(head.name))
    const baseWork = machine && machine.workEnergyPerTick > 0 ? machine.workEnergyPerTick * 60 : reference?.work ?? 0
    const workPower = baseWork * spray.effects.power
    const idlePower = machine && machine.idleEnergyPerTick > 0 ? machine.idleEnergyPerTick * 60 : reference?.idle ?? 0

    const facts = STATION_FACTS[building.itemId]
    if (facts) {
      const state = station.value?.id === value.id ? station.value : null
      const storage = state?.parameters?.storage ?? []
      const slots = Array.from({length: facts.slots}, (_, slot) => {
        const entry = storage[slot]
        return entry ? {...entry, name: w.itemName(entry.itemId)} : null
      })
      return {...head, kind: 'station', facts, slots, settings: state?.parameters?.settings ?? null, loading: state?.loading ?? true, notice: state?.notice ?? ''}
    }

    if (LAB_ITEMS.has(building.itemId)) {
      const {products, inputs} = recipeItems(recipe)
      const speed = (machine?.speed ?? 0) > 0 ? machine?.speed as number : 1
      const cycle = recipe && recipe.timeSpend > 0 ? recipe.timeSpend / 60 / speed / (spray.mode === 1 ? spray.effects.speed : 1) : null
      const fallbackRate = cycle && products[0]?.count ? products[0].count * 60 / cycle * (spray.mode === 2 ? spray.effects.increase : 1) : null
      const proliferator: LabProliferatorEffect = spray.mode === 1
        ? {mode: 'speed', output: spray.effects.speed - 1, power: spray.effects.power - 1}
        : {mode: 'extra', output: spray.effects.increase - 1, power: spray.effects.power - 1}
      return {
        ...head,
        kind: 'lab',
        inputs,
        output: products[0] ?? null,
        cycle,
        rate: production?.capacity?.outputPerMachinePerMinute ?? fallbackRate,
        power: workPower || null,
        utilization: production?.capacity?.utilization ?? null,
        proliferator,
      }
    }

    // 普通建筑：机器显示配方产物与原料，带节点显示标记物品；有耗电的建筑显示功率
    const {products, inputs} = recipeItems(recipe)
    const marker: BuildingPanelItem[] = !recipe && building.iconItemId ? [{itemId: building.iconItemId, name: w.itemName(building.iconItemId), role: 'marker'}] : []
    return {
      ...head,
      kind: 'brief',
      items: [...products, ...inputs, ...marker],
      power: workPower || idlePower ? {work: workPower, idle: idlePower} : null,
    }
  })

  return {panel}
}
