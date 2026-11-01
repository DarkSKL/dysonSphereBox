import assert from 'node:assert/strict'
import {readFile} from 'node:fs/promises'
import test from 'node:test'
import ts from 'typescript'

// 说明：复用已安装的 TypeScript 编译器加载纯展示服务，用 Node 内置测试验证规范流图契约；不引入测试运行库。
// 边界：只覆盖画板映射、双速率语义、自动布局请求归一、稳定坐标与循环边保留，不把前端断言当作后端带路或游戏验收。
const file = new URL('../src/services/dysonSphere/factory-graph.ts', import.meta.url)
const {outputText} = ts.transpileModule(await readFile(file, 'utf8'), {compilerOptions: {module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022}})
const {buildFlowGraph} = await import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`)
const layoutFile = new URL('../src/services/dysonSphere/factory-layout.ts', import.meta.url)
const {outputText: layoutOutput} = ts.transpileModule(await readFile(layoutFile, 'utf8'), {compilerOptions: {module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022}})
const {buildFactoryLayoutRequest} = await import(`data:text/javascript;base64,${Buffer.from(layoutOutput).toString('base64')}`)
const itemName = id => `物品 ${id}`

const fixture = () => ({
  productions: [{mainItemId: 2, outputCount: 40}, {mainItemId: 2, outputCount: 60}, {mainItemId: 3, outputCount: 100}],
  rawInputs: [{itemId: 1, ratePerMinute: 100}],
  targets: [{itemId: 3, ratePerMinute: 100}],
  leftovers: [],
  flowGraph: {
    digest: 'canonical-test',
    productionOrder: [0, 1, 2],
    nodes: [
      {id: 'source:1', kind: 'raw-source', itemId: 1, productionIndex: -1, componentId: 'c0', level: 0},
      {id: 'production:0', kind: 'production', itemId: 2, productionIndex: 0, componentId: 'c1', level: 1},
      {id: 'production:1', kind: 'production', itemId: 2, productionIndex: 1, componentId: 'c2', level: 1},
      {id: 'production:2', kind: 'production', itemId: 3, productionIndex: 2, componentId: 'c3', level: 2},
      {id: 'target:3', kind: 'target-sink', itemId: 3, productionIndex: -1, componentId: 'c4', level: 3},
    ],
    edges: [
      {edgeId: 'a', itemId: 1, sourceNodeId: 'source:1', targetNodeId: 'production:0', ratePerMinute: 40, flowKind: 'external-input'},
      {edgeId: 'b', itemId: 1, sourceNodeId: 'source:1', targetNodeId: 'production:1', ratePerMinute: 60, flowKind: 'external-input'},
      {edgeId: 'c', itemId: 2, sourceNodeId: 'production:0', targetNodeId: 'production:2', ratePerMinute: 40, flowKind: 'internal'},
      {edgeId: 'd', itemId: 2, sourceNodeId: 'production:1', targetNodeId: 'production:2', ratePerMinute: 60, flowKind: 'internal'},
      {edgeId: 'e', itemId: 3, sourceNodeId: 'production:2', targetNodeId: 'target:3', ratePerMinute: 100, flowKind: 'target-output'},
    ],
    components: Array.from({length: 5}, (_, index) => ({id: `c${index}`, nodeIds: [], cyclic: false, level: index})),
  },
})

const layoutFixture = () => ({
  beltLv: 3,
  inserterLv: 4,
  maxLays: 5,
  maxMachineInALine: 80,
  blockWidthLimit: 420,
  mergeType: 1,
  needPower: false,
  useMixed: true,
  mix2: true,
  needCoaterNet: true,
})

test('自动球面请求省略平面专属参数且不改写工作区状态', () => {
  const layout = layoutFixture()
  const request = buildFactoryLayoutRequest(layout, true)

  assert.equal('maxMachineInALine' in request, false)
  assert.equal('blockWidthLimit' in request, false)
  assert.equal('mergeType' in request, false)
  assert.equal(request.beltLv, layout.beltLv)
  assert.equal(request.maxLays, layout.maxLays)
  assert.equal(request.needPower, layout.needPower)
  assert.equal(request.useMixed, false)
  assert.equal(request.mix2, false)
  assert.equal(request.needCoaterNet, false)
  assert.equal(layout.maxMachineInALine, 80)
  assert.equal(layout.blockWidthLimit, 420)
  assert.equal(layout.mergeType, 1)
})

test('旧平面兼容请求保留布局参数并关闭已移除开关', () => {
  const layout = layoutFixture()
  const request = buildFactoryLayoutRequest(layout, false)

  assert.equal(request.maxMachineInALine, 80)
  assert.equal(request.blockWidthLimit, 420)
  assert.equal(request.mergeType, 1)
  assert.equal(request.useMixed, false)
  assert.equal(request.mix2, false)
  assert.equal(request.needCoaterNet, false)
})

test('旧方案缺少规范图时不猜测供需边', () => {
  const plan = fixture()
  delete plan.flowGraph
  const view = buildFlowGraph(plan, itemName)
  assert.equal(view.nodes.length, 0)
  assert.equal(view.edges.length, 0)
  assert.match(view.notice, /重新计算/)
})

test('同主产物的两条产线保持独立，端口和连线严格沿用后端分配', () => {
  const plan = fixture()
  const view = buildFlowGraph(plan, itemName)
  assert.equal(view.notice, '')
  assert.equal(view.nodes.length, 5)
  assert.deepEqual(view.edges.map(edge => [edge.id, edge.source, edge.target, edge.rate]), plan.flowGraph.edges.map(edge => [edge.edgeId, edge.sourceNodeId, edge.targetNodeId, edge.ratePerMinute]))
  assert.equal(view.nodes.find(node => node.id === 'production:0').data.productionIndex, 0)
  assert.equal(view.nodes.find(node => node.id === 'production:1').data.productionIndex, 1)
  assert.equal(view.nodes.find(node => node.id === 'production:2').data.inputs[0].rate, 100)
  assert.equal(view.nodes.find(node => node.id === 'source:1').data.outputs[0].rate, 100)
  assert.ok(view.nodes.find(node => node.id === 'source:1').position.x < view.nodes.find(node => node.id === 'production:0').position.x)
  assert.ok(view.nodes.find(node => node.id === 'production:2').position.x < view.nodes.find(node => node.id === 'target:3').position.x)
})

test('后端保持稳定节点时，打乱 Productions 仅更新索引映射，不改变画板坐标或流量', () => {
  const plan = fixture()
  const before = buildFlowGraph(plan, itemName)
  plan.productions.reverse()
  for (const node of plan.flowGraph.nodes) if (node.kind === 'production') node.productionIndex = 2 - node.productionIndex
  const after = buildFlowGraph(plan, itemName)
  assert.deepEqual(after.nodes.map(node => [node.id, node.position]), before.nodes.map(node => [node.id, node.position]))
  assert.deepEqual(after.edges, before.edges)
  assert.equal(after.nodes.find(node => node.id === 'production:0').data.productionIndex, 2)
})

test('SCC 的所有真实循环边都保留，非循环入口不被标记为循环', () => {
  const plan = fixture()
  plan.flowGraph.nodes[1].componentId = 'loop'
  plan.flowGraph.nodes[2].componentId = 'loop'
  plan.flowGraph.components.push({id: 'loop', nodeIds: ['production:0', 'production:1'], cyclic: true, level: 1})
  plan.flowGraph.edges.push(
    {edgeId: 'loop-a', itemId: 2, sourceNodeId: 'production:0', targetNodeId: 'production:1', ratePerMinute: 10, flowKind: 'internal'},
    {edgeId: 'loop-b', itemId: 2, sourceNodeId: 'production:1', targetNodeId: 'production:0', ratePerMinute: 10, flowKind: 'internal'},
  )
  const view = buildFlowGraph(plan, itemName)
  assert.equal(view.edges.length, 7)
  assert.deepEqual(view.edges.filter(edge => edge.cycle).map(edge => edge.id), ['loop-a', 'loop-b'])
})

test('缺失端点、重复边和无效流量不会成为看似正常的画板', () => {
  const invalidPlans = [fixture(), fixture(), fixture(), fixture()]
  invalidPlans[0].flowGraph.edges[0].targetNodeId = 'missing'
  invalidPlans[1].flowGraph.edges.push({...invalidPlans[1].flowGraph.edges[0]})
  invalidPlans[2].flowGraph.edges[0].ratePerMinute = -1
  invalidPlans[3].flowGraph.nodes[1].productionIndex = 99
  for (const plan of invalidPlans) {
    const view = buildFlowGraph(plan, itemName)
    assert.equal(view.edges.length, 0)
    assert.match(view.notice, /不完整|无效/)
  }
})

test('用户指定外供的来源类型不退化为普通原料，渲染测量只影响画板纵坐标', () => {
  const plan = fixture()
  plan.flowGraph.nodes[0].kind = 'external-source'
  const before = buildFlowGraph(plan, itemName)
  const measured = buildFlowGraph(plan, itemName, () => 400)
  assert.equal(before.nodes[0].data.externalSource, true)
  assert.deepEqual(measured.edges, before.edges)
  assert.deepEqual(measured.nodes.map(node => node.position.x), before.nodes.map(node => node.position.x))
})

const fullLoadCapacity = (expected, fullLoad, cycles = fullLoad) => ({
  basis: 'configured-full-load',
  outputPerMinute: fullLoad,
  outputPerMachinePerMinute: fullLoad,
  cyclesPerMinute: cycles,
  utilization: expected / fullLoad,
  powerMw: .27,
  inputs: [{itemId: 1, name: itemName(1), ratePerMinute: fullLoad}],
  outputs: [{itemId: 2, name: itemName(2), ratePerMinute: fullLoad}],
  grossInputs: [{itemId: 1, name: itemName(1), ratePerMinute: fullLoad}],
  grossOutputs: [{itemId: 2, name: itemName(2), ratePerMinute: fullLoad}],
})

test('期望 40 与设备满负荷 45 独立展示，不改变守恒边与期望端口', () => {
  const plan = fixture()
  const before = buildFlowGraph(plan, itemName)
  plan.productions[0].capacity = fullLoadCapacity(40, 45)
  const view = buildFlowGraph(plan, itemName)
  const node = view.nodes.find(node => node.id === 'production:0').data
  assert.equal(node.expectedRate, 40)
  assert.equal(node.capacity.outputPerMinute, 45)
  assert.equal(node.capacity, plan.productions[0].capacity)
  assert.equal(node.inputs[0].rate, 40)
  assert.equal(node.outputs[0].rate, 40)
  assert.deepEqual(view.edges, before.edges)
  assert.equal(plan.productions[0].outputCount, 40)
})

test('单机循环次数不是物品产量，目标和外供节点不冒用设备满负荷速率', () => {
  const plan = fixture()
  plan.productions[2].outputCount = 120
  plan.productions[2].capacity = fullLoadCapacity(120, 135, 45)
  plan.targets[0].ratePerMinute = 120
  plan.flowGraph.edges.find(edge => edge.edgeId === 'e').ratePerMinute = 120
  const view = buildFlowGraph(plan, itemName)
  const production = view.nodes.find(node => node.id === 'production:2').data
  const target = view.nodes.find(node => node.id === 'target:3').data
  const source = view.nodes.find(node => node.id === 'source:1').data
  assert.equal(production.expectedRate, 120)
  assert.equal(production.capacity.outputPerMachinePerMinute, 135)
  assert.equal(production.capacity.cyclesPerMinute, 45)
  assert.equal(target.expectedRate, 120)
  assert.equal(target.capacity, undefined)
  assert.equal(source.capacity, undefined)
  assert.equal(view.edges.find(edge => edge.id === 'e').rate, 120)
})

test('旧快照或不明容量依据不以期望值补造满负荷数据', () => {
  const legacy = buildFlowGraph(fixture(), itemName)
  const legacyProduction = legacy.nodes.find(node => node.id === 'production:0').data
  assert.equal(legacyProduction.expectedRate, 40)
  assert.equal(legacyProduction.capacity, undefined)
  const plan = fixture()
  plan.productions[0].capacity = {...fullLoadCapacity(40, 45), basis: 'unknown'}
  assert.equal(buildFlowGraph(plan, itemName).nodes.find(node => node.id === 'production:0').data.capacity, undefined)
  plan.productions[0].capacity = {...fullLoadCapacity(40, 45), outputPerMinute: undefined}
  assert.equal(buildFlowGraph(plan, itemName).nodes.find(node => node.id === 'production:0').data.capacity, undefined)
})

test('满负荷毛物料保留循环消耗，不添加或膨胀净物流连线', () => {
  const plan = fixture()
  const capacity = fullLoadCapacity(40, 45)
  capacity.grossInputs.push({itemId: 2, name: itemName(2), ratePerMinute: 90})
  capacity.grossOutputs[0].ratePerMinute = 135
  plan.productions[0].capacity = capacity
  const view = buildFlowGraph(plan, itemName)
  const data = view.nodes.find(node => node.id === 'production:0').data
  assert.deepEqual(data.capacity.grossInputs, capacity.grossInputs)
  assert.equal(data.capacity.outputs[0].ratePerMinute, 45)
  assert.equal(data.capacity.grossOutputs[0].ratePerMinute, 135)
  assert.equal(data.inputs.some(port => port.itemId === 2), false)
  assert.equal(view.edges.length, plan.flowGraph.edges.length)
})
