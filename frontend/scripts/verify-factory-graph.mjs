import assert from 'node:assert/strict'
import {readFile} from 'node:fs/promises'
import test from 'node:test'
import ts from 'typescript'

// 说明：复用已安装的 TypeScript 编译器加载纯展示服务，用 Node 内置测试验证规范流图契约；不引入测试运行库。
// 边界：只覆盖画板映射、稳定坐标与循环边保留，不把前端断言当作后端带路或游戏验收。
const file = new URL('../src/services/dysonSphere/factory-graph.ts', import.meta.url)
const {outputText} = ts.transpileModule(await readFile(file, 'utf8'), {compilerOptions: {module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022}})
const {buildFlowGraph} = await import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`)
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
