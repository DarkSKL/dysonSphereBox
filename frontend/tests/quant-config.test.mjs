import test from 'node:test'
import assert from 'node:assert/strict'
import {decodeQuantConfig, encodeQuantConfig, parseQuantConfig, QuantConfigError, validateNaturalLine, validateQuantRequest} from '../src/services/dysonSphere/quant-config.ts'

const dataset = {
  info: {key: 'Vanilla', dataVersion: 'abc123'},
  items: [{id: 1, name: '铁矿'}, {id: 2, name: '铁块'}, {id: 3, name: '熔炉'}],
  recipes: [{id: 10, results: [2], factories: [3], proliferator: 3}],
}
const valid = () => ({dataset: 'Vanilla', targets: [{itemId: 2, ratePerMinute: 60}], scheme: {itemRecipeChoices: {'铁块': 1}, schemeForRecipe: [{building: 0, productionMode: 1, increasePoints: 4}], costWeight: {space: 1, power: 0, buildingCost: {'熔炉': 2}, itemExtraCost: {'铁矿': {cost: -2, enabled: true, cumulative: false, overflowCost: -3}}}}, settings: {fixedNum: 0, proliferateItself: false, blueBuff: false, naturalProductionLine: [{targetItem: '铁块', recipeChoice: 1, building: 0, machineCount: 1.5, productionMode: 1, increasePoints: 4}]}})
const ctx = () => structuredClone(dataset)
const code = (fn, errorCode) => assert.throws(fn, error => error instanceof QuantConfigError && error.code === errorCode)

test('新配置完整往返并保留零、false、成本与每配方输入', () => {
  const request = valid()
  const text = encodeQuantConfig(request, ctx(), '熔炼')
  const result = decodeQuantConfig(text, ctx())
  assert.equal(result.request.settings.fixedNum, 0)
  assert.equal(result.request.settings.proliferateItself, false)
  assert.equal(result.request.settings.blueBuff, false)
  assert.equal(result.request.settings.naturalProductionLine[0].machineCount, 1.5)
  assert.equal(result.request.scheme.schemeForRecipe[0].productionMode, 1)
  assert.equal(result.request.scheme.costWeight.itemExtraCost['铁矿'].overflowCost, -3)
  assert.equal(result.name, '熔炼')
})

test('Settings 缺省回 Go 默认，倍率 0 归一默认', () => {
  const request = valid(); request.settings = {miningSpeedMultiple: 0}
  const normalized = validateQuantRequest(request, ctx())
  assert.equal(normalized.settings.miningSpeedMultiple, 1)
  assert.equal(normalized.settings.blueBuff, false)
  assert.equal(normalized.settings.proliferateItself, true)
})

test('旧工作区只迁移量化输入并忽略 plan、布局和蓝图攻击字段', () => {
  const old = {formatVersion: 1, name: '旧名', request: valid(), plan: {dataVersion: 'abc123', productions: [{hack: true}]}, setting: {hack: true}, surfaceLayout: {path: 'x'}, mode: 6, blueprint: 'ignored'}
  const result = decodeQuantConfig(JSON.stringify(old), ctx())
  assert.equal(result.migrated, true)
  assert.deepEqual(Object.keys(result), ['request', 'name', 'migrated', 'notice'])
  assert.equal(result.request.settings.naturalProductionLine.length, 1)
})

test('旧无指纹记录提示重新计算；错误的 plan 指纹拒绝', () => {
  const old = {formatVersion: 1, request: valid(), plan: {productions: []}, setting: {}, mode: 1}
  assert.match(decodeQuantConfig(JSON.stringify(old), ctx()).notice, /按当前内置数据重新计算/)
  old.plan.dataVersion = 'old'
  code(() => decodeQuantConfig(JSON.stringify(old), ctx()), 'QCFG_E_DATASET')
})

test('多字节超限、JSON、格式、数据版本错误有稳定本地码', () => {
  const large = JSON.parse(encodeQuantConfig(valid(), ctx())); large.future = '界'.repeat(400000)
  code(() => decodeQuantConfig(JSON.stringify(large), ctx()), 'QCFG_E_TOO_LARGE')
  code(() => decodeQuantConfig(' '.repeat(1024 * 1024 + 1), ctx()), 'QCFG_E_TOO_LARGE')
  code(() => parseQuantConfig('{'), 'QCFG_E_FORMAT')
  code(() => parseQuantConfig(JSON.stringify({format: 'other', formatVersion: 1})), 'QCFG_E_FORMAT')
  const text = encodeQuantConfig(valid(), ctx())
  code(() => decodeQuantConfig(text.replace('abc123', 'wrong'), ctx()), 'QCFG_E_DATASET')
})

test('非法索引、数值、模式、名称、原型键和成本类型拒绝', () => {
  const bad = valid(); bad.settings.naturalProductionLine[0].machineCount = Infinity
  code(() => validateQuantRequest(bad, ctx()), 'QCFG_E_INVALID')
  const badChoice = valid(); badChoice.scheme.itemRecipeChoices['铁块'] = 2
  code(() => validateQuantRequest(badChoice, ctx()), 'QCFG_E_INVALID')
  const badMode = valid(); badMode.settings.naturalProductionLine[0].productionMode = 4
  code(() => validateQuantRequest(badMode, ctx()), 'QCFG_E_INVALID')
  const badProto = valid(); badProto.scheme.costWeight.buildingCost = JSON.parse('{"__proto__":2}')
  code(() => validateQuantRequest(badProto, ctx()), 'QCFG_E_INVALID')
  const badBool = valid(); badBool.scheme.costWeight.itemExtraCost['铁矿'].enabled = 'false'
  code(() => validateQuantRequest(badBool, ctx()), 'QCFG_E_INVALID')
})

test('自然产线合法引用与数据版本闭合', () => {
  assert.equal(validateNaturalLine(valid().settings.naturalProductionLine[0], ctx()).building, 0)
  const wrong = valid(); wrong.dataset = 'Unknown'
  code(() => validateQuantRequest(wrong, ctx()), 'QCFG_E_DATASET')
})

test('已知容器和逐配方项的显式 null 不等于缺省', () => {
  for (const mutate of [
    request => { request.scheme = null },
    request => { request.settings = null },
    request => { request.scheme.costWeight = null },
    request => { request.scheme.schemeForRecipe = [null] },
    request => { request.settings.mineralizeList = null },
    request => { request.settings.naturalProductionLine = null },
  ]) {
    const request = valid(); mutate(request)
    assert.throws(() => validateQuantRequest(request, ctx()), QuantConfigError)
  }
})

test('量化名称两端同限；未知格式不得作为旧工作区别名', () => {
  const root = JSON.parse(encodeQuantConfig(valid(), ctx()))
  root.name = '字'.repeat(121)
  code(() => decodeQuantConfig(JSON.stringify(root), ctx()), 'QCFG_E_INVALID')
  code(() => encodeQuantConfig(valid(), ctx(), root.name), 'QCFG_E_INVALID')
  root.name = '名'; root.format = 'dyson-sphere-box/workspace'
  code(() => decodeQuantConfig(JSON.stringify(root), ctx()), 'QCFG_E_FORMAT')
  const unicodeName = '😀'.repeat(120)
  assert.equal(decodeQuantConfig(encodeQuantConfig(valid(), ctx(), unicodeName), ctx()).name, unicodeName)
})

test('分馏的 mode0 点数不在 TS 丢弃，交给 Go 统一归一', () => {
  const fraction = ctx(); fraction.info.key = 'FractionateEverything'; fraction.recipes[0].proliferator = 8
  const request = valid(); request.dataset = fraction.info.key
  request.scheme.schemeForRecipe[0] = {building: 0, productionMode: 0, increasePoints: 2}
  request.settings.naturalProductionLine[0] = {...request.settings.naturalProductionLine[0], productionMode: 0, increasePoints: 2}
  const result = decodeQuantConfig(encodeQuantConfig(request, fraction), fraction)
  assert.equal(result.request.scheme.schemeForRecipe[0].increasePoints, 2)
  assert.equal(result.request.settings.naturalProductionLine[0].increasePoints, 2)
})

test('完整校验不改原输入，未知字段不进入保存结果', () => {
  const request = valid(); request.settings.future = {path: 'ignored'}; request.future = true
  const before = structuredClone(request)
  const result = validateQuantRequest(request, ctx())
  assert.deepEqual(request, before)
  assert.equal(Object.hasOwn(result, 'future'), false)
  assert.equal(Object.hasOwn(result.settings, 'future'), false)
  result.scheme.schemeForRecipe[0].building = 999
  assert.deepEqual(request, before)
})
