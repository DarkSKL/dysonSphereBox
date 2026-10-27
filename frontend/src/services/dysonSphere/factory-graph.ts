import type {quant} from '../../../wailsjs/go/models'

// =============== 量化节点图布局 ===============
// 职责：把求解快照转换为「来源 → 配方 → 目标/剩余」的分层有向图；使用：量化工作区网格画布。
// 边界：只读取 Go 返回的产线物料流，不重算配方比例；布局是确定性分层，同一方案多次转换坐标一致。

export type FlowNodeKind = 'source' | 'production' | 'target' | 'leftover'
export interface FlowPort {id: string; itemId: number; name: string; rate: number}
export interface FlowNodeData {
  kind: FlowNodeKind
  itemId: number
  title: string
  rate: number
  inputs: FlowPort[]
  outputs: FlowPort[]
  productionIndex?: number
  production?: quant.Production
}
export interface FlowLayoutNode {id: string; position: {x: number; y: number}; data: FlowNodeData}
export interface FlowLayoutEdge {id: string; source: string; target: string; sourceHandle: string; targetHandle: string; itemId: number; name: string; rate: number; cycle: boolean}

const COLUMN_GAP = 300
const ROW_GAP = 28
const PORT_HEIGHT = 22
const NODE_BASE = {source: 60, production: 160, target: 100, leftover: 60}

export const portId = (side: 'in' | 'out', itemId: number): string => `${side}-${itemId}`
const estimateHeight = (node: FlowLayoutNode): number => NODE_BASE[node.data.kind] + Math.max(node.data.inputs.length, node.data.outputs.length, 1) * PORT_HEIGHT

/**
 * buildFlowGraph 把一份量化结果展开为节点与边。
 * 逻辑流：1. 建来源/产线/目标/剩余节点 => 2. 每种物品的供给按比例分配给消费者 => 3. 最长路分层并按重心排序。
 * measure 可提供渲染后的实际高度，未测量的节点按端口数估算。
 */
export function buildFlowGraph(plan: quant.Result, itemName: (id: number) => string, measure?: (id: string) => number | undefined): {nodes: FlowLayoutNode[]; edges: FlowLayoutEdge[]} {
  const nodes: FlowLayoutNode[] = []
  const suppliers = new Map<number, {node: string; rate: number}[]>()
  const consumers = new Map<number, {node: string; rate: number}[]>()
  const push = (map: Map<number, {node: string; rate: number}[]>, itemId: number, node: string, rate: number): void => {
    if (rate <= 1e-9) return
    const list = map.get(itemId) ?? []
    list.push({node, rate})
    map.set(itemId, list)
  }
  const port = (itemId: number, rate: number, side: 'in' | 'out'): FlowPort => ({id: portId(side, itemId), itemId, name: itemName(itemId), rate})

  for (const raw of plan.rawInputs ?? []) {
    const id = `source-${raw.itemId}`
    nodes.push({id, position: {x: 0, y: 0}, data: {kind: 'source', itemId: raw.itemId, title: raw.name, rate: raw.ratePerMinute, inputs: [], outputs: [port(raw.itemId, raw.ratePerMinute, 'out')]}})
    push(suppliers, raw.itemId, id, raw.ratePerMinute)
  }
  plan.productions.forEach((production, index) => {
    const id = `production-${production.mainItemId}`
    const inputs = (production.inputs ?? []).map(value => port(value.itemId, value.ratePerMinute, 'in'))
    const outputs = (production.outputs ?? []).map(value => port(value.itemId, value.ratePerMinute, 'out'))
    nodes.push({id, position: {x: 0, y: 0}, data: {kind: 'production', itemId: production.mainItemId, title: itemName(production.mainItemId), rate: production.outputCount, inputs, outputs, productionIndex: index, production}})
    inputs.forEach(value => push(consumers, value.itemId, id, value.rate))
    outputs.forEach(value => push(suppliers, value.itemId, id, value.rate))
  })
  for (const target of plan.targets ?? []) {
    const id = `target-${target.itemId}`
    nodes.push({id, position: {x: 0, y: 0}, data: {kind: 'target', itemId: target.itemId, title: itemName(target.itemId), rate: target.ratePerMinute, inputs: [port(target.itemId, target.ratePerMinute, 'in')], outputs: []}})
    push(consumers, target.itemId, id, target.ratePerMinute)
  }
  for (const left of plan.leftovers ?? []) {
    const id = `leftover-${left.itemId}`
    nodes.push({id, position: {x: 0, y: 0}, data: {kind: 'leftover', itemId: left.itemId, title: left.name, rate: left.ratePerMinute, inputs: [port(left.itemId, left.ratePerMinute, 'in')], outputs: []}})
    push(consumers, left.itemId, id, left.ratePerMinute)
  }

  // 供给按占比分配：一种物品有多个来源时，每个消费者从各来源按供给比例取料，边速率之和与端口速率一致。
  const edges: FlowLayoutEdge[] = []
  for (const [itemId, needs] of consumers) {
    const offers = suppliers.get(itemId) ?? []
    const supply = offers.reduce((sum, offer) => sum + offer.rate, 0)
    if (supply <= 0) continue
    for (const need of needs) for (const offer of offers) {
      if (offer.node === need.node) continue
      const rate = need.rate * offer.rate / supply
      if (rate <= 1e-9) continue
      edges.push({id: `${offer.node}:${need.node}:${itemId}`, source: offer.node, target: need.node, sourceHandle: portId('out', itemId), targetHandle: portId('in', itemId), itemId, name: itemName(itemId), rate, cycle: false})
    }
  }

  layout(nodes, edges, node => measure?.(node.id) ?? estimateHeight(node))
  return {nodes, edges}
}

// 分层：来源固定为第 0 列，目标/剩余固定为最后一列；产线按「到目标的最长距离」倒排，回边标记为循环。
function layout(nodes: FlowLayoutNode[], edges: FlowLayoutEdge[], nodeHeight: (node: FlowLayoutNode) => number): void {
  const outgoing = new Map<string, string[]>()
  for (const edge of edges) outgoing.set(edge.source, [...(outgoing.get(edge.source) ?? []), edge.target])
  const depth = new Map<string, number>()
  const visiting = new Set<string>()
  const backEdges = new Set<string>()
  const measure = (id: string): number => {
    const known = depth.get(id)
    if (known !== undefined) return known
    visiting.add(id)
    let value = 0
    for (const next of outgoing.get(id) ?? []) {
      if (visiting.has(next)) { backEdges.add(`${id}>${next}`); continue }
      value = Math.max(value, measure(next) + 1)
    }
    visiting.delete(id)
    depth.set(id, value)
    return value
  }
  nodes.forEach(node => measure(node.id))
  for (const edge of edges) edge.cycle = backEdges.has(`${edge.source}>${edge.target}`)

  const productionDepth = Math.max(0, ...nodes.filter(node => node.data.kind === 'production').map(node => depth.get(node.id) ?? 0))
  const columnOf = (node: FlowLayoutNode): number => {
    if (node.data.kind === 'source') return 0
    if (node.data.kind === 'target' || node.data.kind === 'leftover') return productionDepth + 2
    return productionDepth - (depth.get(node.id) ?? 1) + 2
  }
  const columns = new Map<number, FlowLayoutNode[]>()
  for (const node of nodes) columns.set(columnOf(node), [...(columns.get(columnOf(node)) ?? []), node])

  // 自左向右放置，每列按上游重心排序，减少交叉；同重心保持求解器顺序，保证结果稳定。
  const centerY = new Map<string, number>()
  const incoming = new Map<string, string[]>()
  for (const edge of edges) if (!edge.cycle) incoming.set(edge.target, [...(incoming.get(edge.target) ?? []), edge.source])
  for (const column of [...columns.keys()].sort((a, b) => a - b)) {
    const list = columns.get(column) ?? []
    const weight = (node: FlowLayoutNode, order: number): number => {
      const parents = (incoming.get(node.id) ?? []).map(id => centerY.get(id)).filter((value): value is number => value !== undefined)
      return parents.length ? parents.reduce((sum, value) => sum + value, 0) / parents.length : order * 1000
    }
    const sorted = list.map((node, order) => ({node, key: weight(node, order), order})).sort((a, b) => a.key - b.key || a.order - b.order).map(entry => entry.node)
    let y = 0
    for (const node of sorted) {
      const height = nodeHeight(node)
      node.position = {x: column * COLUMN_GAP, y}
      centerY.set(node.id, y + height / 2)
      y += height + ROW_GAP
    }
    const offset = y / 2
    for (const node of sorted) {
      node.position.y -= offset
      centerY.set(node.id, (centerY.get(node.id) ?? 0) - offset)
    }
  }
}
