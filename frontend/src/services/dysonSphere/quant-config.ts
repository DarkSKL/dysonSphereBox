import type {quant} from '../../../wailsjs/go/models'

// =============== 量化输入契约 ===============
// 说明：仅编解码量化输入；类型引用在运行时擦除，不依赖 Wails、Vue 或文件系统。
export const QUANT_CONFIG_FORMAT = 'dyson-sphere-box/quant'
export const QUANT_CONFIG_MAX_BYTES = 1024 * 1024
export type QuantConfigErrorCode = 'QCFG_E_FORMAT' | 'QCFG_E_TOO_LARGE' | 'QCFG_E_DATASET' | 'QCFG_E_INVALID'
export class QuantConfigError extends Error {
  readonly code: QuantConfigErrorCode
  constructor(code: QuantConfigErrorCode, message: string) { super(message); this.name = 'QuantConfigError'; this.code = code }
}
export interface QuantDataset {
  info: {key: string; dataVersion: string}
  items: readonly {id: number; name: string}[]
  recipes: readonly {id: number; results: number[]; factories: number[]; proliferator: number}[]
}
export type QuantSettingKey = keyof typeof QUANT_SETTING_DEFAULTS
export type QuantSettings = Required<Omit<quant.Settings, 'convertValues'>>
export interface QuantInputRequest {
  dataset: string
  targets: quant.Target[]
  scheme: {itemRecipeChoices: Record<string, number>; schemeForRecipe: quant.RecipeScheme[]; costWeight: Omit<quant.CostWeight, 'convertValues'>}
  settings: QuantSettings
}
export interface ParsedQuantConfig {request: Record<string, unknown>; dataset: string; dataVersion?: string; name?: string; migrated: boolean}
export interface QuantConfigResult {request: QuantInputRequest; name?: string; migrated: boolean; notice?: string}

// =============== 标量与名称校验 ===============
// 说明：白名单读取后新建对象；拒绝危险字典键，不复制任意导入字段或对象原型。
const fail = (message: string): never => { throw new QuantConfigError('QCFG_E_INVALID', message) }
const record = (value: unknown, label: string): Record<string, unknown> => {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) throw new QuantConfigError('QCFG_E_FORMAT', `${label} 必须是对象`)
  return value as Record<string, unknown>
}
const defaulted = (value: Record<string, unknown>, key: string, fallback: unknown): unknown => Object.prototype.hasOwnProperty.call(value, key) ? value[key] : fallback
const number = (value: unknown, label: string): number => typeof value === 'number' && Number.isFinite(value) ? value : fail(`${label} 必须是有限数值`)
const integer = (value: unknown, min: number, max: number, label: string): number => {
  const result = number(value, label)
  return Number.isInteger(result) && result >= min && result <= max ? result : fail(`${label} 超出合法整数范围`)
}
const boolean = (value: unknown, label: string): boolean => typeof value === 'boolean' ? value : fail(`${label} 必须是布尔值`)
const safeKey = (key: string): boolean => !['__proto__', 'constructor', 'prototype'].includes(key)
const nameValue = (value: unknown): string | undefined => {
  if (value === undefined) return undefined
  if (typeof value !== 'string' || [...value].length > 120 || /[\u0000-\u001f\u007f]/.test(value)) return fail('量化名称需不超过 120 字且不含控制字符')
  return value
}
export const QUANT_SETTING_DEFAULTS = Object.freeze({
  miningSpeedOil: 3, miningSpeedHydrogen: 1, miningSpeedDeuterium: .05, miningSpeedGasHydrate: .8,
  miningSpeedHelium: .02, miningSpeedAmmonia: .3, miningSpeedNitrogen: 1.2, miningSpeedOxygen: .6,
  miningSpeedCarbonDioxide: .4, miningSpeedSulfurDioxide: .6, coveredVeinsSmall: 8, coveredVeinsLarge: 16,
  miningEfficiencyLarge: 3, miningSpeedMultiple: 1, enemyDropMultiple: 1, icarusManufacturingSpeed: 1,
  fractionatingSpeed: 30, stackResearchLab: 15, accRate: 1, incRate: 1,
})

// =============== 数据引用校验 ===============
// 说明：配方为目标物品的一基选项，建筑为该配方的零基选项；只校验不计算任何配方流量。
const itemNames = (dataset: QuantDataset): Set<string> => new Set(dataset.items.map(item => item.name))
const checkName = (value: unknown, names: ReadonlySet<string>, label: string): string => {
  if (typeof value !== 'string' || !safeKey(value) || !names.has(value)) return fail(`${label} 引用了未知名称`)
  return value
}
const recipesFor = (dataset: QuantDataset, name: string) => {
  const id = dataset.items.find(item => item.name === name)?.id
  return dataset.recipes.filter(recipe => recipe.results.includes(id ?? -1))
}
const recipeScheme = (input: unknown, recipe: QuantDataset['recipes'][number], dataset: QuantDataset): quant.RecipeScheme => {
  const raw = record(input, '配方配置')
  const building = integer(defaulted(raw, 'building', 0), 0, recipe.factories.length - 1, '建筑序号')
  const productionMode = integer(defaulted(raw, 'productionMode', 0), 0, 4, '生产模式')
  if (productionMode > 0 && !(recipe.proliferator & (1 << (productionMode - 1)))) return fail('生产模式不适用于所选配方')
  const increasePoints = integer(defaulted(raw, 'increasePoints', 0), 0, 10, '增产点数')
  if (![0, 1, 2, 4, ...(dataset.info.key.includes('FractionateEverything') ? [10] : [])].includes(increasePoints)) return fail('增产点数不适用于当前数据集')
  return {building, productionMode, increasePoints}
}
/** 已有产线整项校验；编辑器和导入共用，不容许静默删除无效行。 */
export const validateNaturalLine = (input: unknown, dataset: QuantDataset): quant.NaturalProduction => {
  const raw = record(input, '已有产线')
  const targetItem = checkName(raw.targetItem, itemNames(dataset), '已有产线物品')
  const choices = recipesFor(dataset, targetItem)
  const recipeChoice = integer(defaulted(raw, 'recipeChoice', 1), 1, choices.length, '已有产线配方')
  const machineCount = number(defaulted(raw, 'machineCount', 0), '已有机器数')
  if (machineCount < 0) return fail('已有机器数不能为负数')
  return {targetItem, recipeChoice, machineCount, ...recipeScheme(raw, choices[recipeChoice - 1], dataset)}
}

// =============== 请求白名单与默认值 ===============
// 说明：完整复制所有量化字段；缺省项使用 Go 默认，不继承工作区原值；有限成本可为负数。
export const validateQuantRequest = (input: unknown, dataset: QuantDataset): QuantInputRequest => {
  const raw = record(input, 'request')
  if (raw.dataset !== dataset.info.key) throw new QuantConfigError('QCFG_E_DATASET', '未知或不匹配的数据集')
  const names = itemNames(dataset)
  const ids = new Set(dataset.items.filter(item => item.id > 0).map(item => item.id))
  if (!Array.isArray(raw.targets) || raw.targets.length < 1 || raw.targets.length > 100) return fail('目标数量需为 1 到 100 项')
  const targets = raw.targets.map(value => {
    const target = record(value, '目标')
    const itemId = integer(target.itemId, 1, Number.MAX_SAFE_INTEGER, '目标物品')
    const ratePerMinute = number(target.ratePerMinute, '目标产量')
    if (!ids.has(itemId) || ratePerMinute <= 0 || ratePerMinute > 1e9) return fail('目标物品或产量无效')
    return {itemId, ratePerMinute}
  })
  const sourceScheme = record(defaulted(raw, 'scheme', {}), 'scheme')
  const itemRecipeChoices: Record<string, number> = {}
  for (const [key, value] of Object.entries(record(defaulted(sourceScheme, 'itemRecipeChoices', {}), '配方选择'))) {
    checkName(key, names, '配方选择')
    itemRecipeChoices[key] = integer(value, 1, recipesFor(dataset, key).length, '配方选项')
  }
  const sourceRecipes = defaulted(sourceScheme, 'schemeForRecipe', [])
  if (!Array.isArray(sourceRecipes) || sourceRecipes.length > dataset.recipes.length) return fail('逐配方配置数量无效')
  const schemeForRecipe = dataset.recipes.map((recipe, index) => recipeScheme(index < sourceRecipes.length ? sourceRecipes[index] : {}, recipe, dataset))
  const cost = record(defaulted(sourceScheme, 'costWeight', {}), '成本配置')
  const buildingCost: Record<string, number> = {}
  const buildingNames = new Set(dataset.recipes.flatMap(recipe => recipe.factories).map(id => dataset.items.find(item => item.id === id)?.name ?? ''))
  for (const [key, value] of Object.entries(record(defaulted(cost, 'buildingCost', {}), '建筑成本'))) {
    checkName(key, buildingNames, '建筑成本'); buildingCost[key] = number(value, '建筑成本')
  }
  const itemExtraCost: Record<string, quant.ItemCost> = {}
  for (const [key, value] of Object.entries(record(defaulted(cost, 'itemExtraCost', {}), '物品成本'))) {
    checkName(key, names, '物品成本')
    const item = record(value, '物品成本')
    itemExtraCost[key] = {cost: number(defaulted(item, 'cost', 0), '物品成本'), enabled: boolean(defaulted(item, 'enabled', false), '成本启用'), cumulative: boolean(defaulted(item, 'cumulative', false), '累计成本'), overflowCost: number(defaulted(item, 'overflowCost', 0), '溢出成本')}
  }
  const sourceSettings = record(defaulted(raw, 'settings', {}), 'settings')
  const settings: QuantSettings = {...QUANT_SETTING_DEFAULTS, fixedNum: 2, proliferateItself: true, blueBuff: false, mineralizeList: {}, naturalProductionLine: []}
  for (const key of Object.keys(QUANT_SETTING_DEFAULTS) as QuantSettingKey[]) {
    const value = number(defaulted(sourceSettings, key, QUANT_SETTING_DEFAULTS[key]), key)
    if (value < 0 || value > 1e6) return fail(`${key} 必须为 0（默认）或不超过 1000000 的正数`)
    settings[key] = value === 0 ? QUANT_SETTING_DEFAULTS[key] : value
  }
  settings.fixedNum = integer(defaulted(sourceSettings, 'fixedNum', 2), 0, 8, '小数位')
  settings.proliferateItself = boolean(defaulted(sourceSettings, 'proliferateItself', true), '增产剂自喷涂')
  settings.blueBuff = boolean(defaulted(sourceSettings, 'blueBuff', false), '蓝 Buff')
  const mineralizeList: Record<string, boolean> = {}
  for (const [key, value] of Object.entries(record(defaulted(sourceSettings, 'mineralizeList', {}), '外部供应'))) {
    checkName(key, names, '外部供应'); mineralizeList[key] = boolean(value, '外部供应')
  }
  settings.mineralizeList = mineralizeList
  const sourceLines = defaulted(sourceSettings, 'naturalProductionLine', [])
  if (!Array.isArray(sourceLines)) return fail('已有产线必须为列表')
  settings.naturalProductionLine = sourceLines.map(line => validateNaturalLine(line, dataset))
  return {dataset: dataset.info.key, targets, scheme: {itemRecipeChoices, schemeForRecipe, costWeight: {space: number(defaulted(cost, 'space', 1), '占地成本'), power: number(defaulted(cost, 'power', 0), '电力成本'), buildingCost, itemExtraCost}}, settings}
}

// =============== 格式解析与迁移 ===============
// 说明：第一步只读取信封以加载目标数据集；第二步必须验证指纹和全部引用后才可提交到工作区。
const checkSize = (text: string): void => {
  if (new TextEncoder().encode(text).byteLength > QUANT_CONFIG_MAX_BYTES) throw new QuantConfigError('QCFG_E_TOO_LARGE', '量化配置超过 1 MiB（UTF-8）')
}
export const parseQuantConfig = (text: string): ParsedQuantConfig => {
  checkSize(text)
  let parsed: unknown
  try { parsed = JSON.parse(text.replace(/^\uFEFF/, '')) } catch { throw new QuantConfigError('QCFG_E_FORMAT', 'JSON 格式无效') }
  const root = record(parsed, '配置')
  const migrated = root.format === undefined
  if (root.formatVersion !== 1 || (!migrated && root.format !== QUANT_CONFIG_FORMAT)) throw new QuantConfigError('QCFG_E_FORMAT', '未知量化格式或版本')
  if (migrated && (!('plan' in root) || !('setting' in root) || !('mode' in root))) throw new QuantConfigError('QCFG_E_FORMAT', '不是受支持的 v1 工作区')
  let dataVersion = root.dataVersion
  if (migrated) dataVersion = root.plan === undefined ? undefined : record(root.plan, '旧工作区 plan').dataVersion
  if ((!migrated && (typeof dataVersion !== 'string' || !dataVersion)) || (dataVersion !== undefined && (typeof dataVersion !== 'string' || !dataVersion))) throw new QuantConfigError('QCFG_E_DATASET', '数据集指纹无效')
  const request = record(root.request, 'request')
  if (typeof request.dataset !== 'string' || !request.dataset || !safeKey(request.dataset)) throw new QuantConfigError('QCFG_E_DATASET', '数据集键无效')
  return {dataset: request.dataset, dataVersion: dataVersion as string | undefined, request, name: nameValue(root.name), migrated}
}
export const decodeQuantConfig = (text: string, dataset: QuantDataset): QuantConfigResult => {
  const parsed = parseQuantConfig(text)
  if (parsed.dataset !== dataset.info.key || (parsed.dataVersion !== undefined && parsed.dataVersion !== dataset.info.dataVersion)) throw new QuantConfigError('QCFG_E_DATASET', '数据集或指纹不匹配')
  return {request: validateQuantRequest(parsed.request, dataset), name: parsed.name, migrated: parsed.migrated, notice: parsed.migrated && !parsed.dataVersion ? '按当前内置数据重新计算' : undefined}
}
export const encodeQuantConfig = (request: unknown, dataset: QuantDataset, name?: string): string => {
  if (!dataset.info.dataVersion) throw new QuantConfigError('QCFG_E_DATASET', '缺少数据集指纹')
  const text = JSON.stringify({format: QUANT_CONFIG_FORMAT, formatVersion: 1, dataVersion: dataset.info.dataVersion, request: validateQuantRequest(request, dataset), name: nameValue(name)}, null, 2)
  checkSize(text)
  return text
}
