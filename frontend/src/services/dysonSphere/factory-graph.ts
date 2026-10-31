import type {quant} from '../../../wailsjs/go/models'

// =============== 规范物料图展示 ===============
// 说明：只把后端守恒流图映射为画板节点、端口和坐标，不再根据产线供需另行猜测连接。
// 边界：节点拖动仅影响展示；缺少规范图的旧快照必须重新量化，由后端重建流边。

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
  externalSource?: boolean
}
export interface FlowLayoutNode {id: string; position: {x: number; y: number}; data: FlowNodeData}
export interface FlowLayoutEdge {id: string; source: string; target: string; sourceHandle: string; targetHandle: string; itemId: number; name: string; rate: number; cycle: boolean}
export interface FlowLayout {nodes: FlowLayoutNode[]; edges: FlowLayoutEdge[]; notice: string}

const COLUMN_GAP = 300
const ROW_GAP = 28
const PORT_HEIGHT = 22
const NODE_BASE = {source: 60, production: 160, target: 100, leftover: 60}

export const portId = (side: 'in' | 'out', itemId: number): string => `${side}-${itemId}`
const estimateHeight = (node: FlowLayoutNode): number => NODE_BASE[node.data.kind] + Math.max(node.data.inputs.length, node.data.outputs.length, 1) * PORT_HEIGHT

/**
 * buildFlowGraph 展示后端的唯一规范物料图。
 * 逻辑流：1. 读取稳定节点与精确流边 => 2. 汇总边速率作为端口标签 => 3. 沿后端 SCC 层级排布。
 * measure 只修正节点视觉高度，不参与后端路线、流量分配或蓝图坐标计算。
 */
export function buildFlowGraph(plan: quant.Result, itemName: (id: number) => string, measure?: (id: string) => number | undefined): FlowLayout {
  const graph = plan.flowGraph
  if (!graph) return {nodes: [], edges: [], notice: '此方案没有规范物流图，请重新计算；前端不会推测供需连接。'}

  const nodeIds = new Set(graph.nodes.map(node => node.id))
  const edgeIds = new Set(graph.edges.map(edge => edge.edgeId))
  if (nodeIds.size !== graph.nodes.length || edgeIds.size !== graph.edges.length || graph.nodes.some(node => node.kind === 'production' && !plan.productions[node.productionIndex]) || graph.edges.some(edge => !nodeIds.has(edge.sourceNodeId) || !nodeIds.has(edge.targetNodeId) || !Number.isFinite(edge.ratePerMinute) || edge.ratePerMinute <= 0)) {
    return {nodes: [], edges: [], notice: '规范物流图不完整或含无效流量，请重新计算；未显示推测连线。'}
  }
  const cyclicComponents = new Set((graph.components ?? []).filter(component => component.cyclic).map(component => component.id))
  const componentOf = new Map(graph.nodes.map(node => [node.id, node.componentId]))
  const edges: FlowLayoutEdge[] = graph.edges.map(edge => ({
    id: edge.edgeId,
    source: edge.sourceNodeId,
    target: edge.targetNodeId,
    sourceHandle: portId('out', edge.itemId),
    targetHandle: portId('in', edge.itemId),
    itemId: edge.itemId,
    name: itemName(edge.itemId),
    rate: edge.ratePerMinute,
    cycle: componentOf.get(edge.sourceNodeId) === componentOf.get(edge.targetNodeId) && cyclicComponents.has(componentOf.get(edge.sourceNodeId) ?? ''),
  }))

  const ports = new Map<string, {inputs: Map<number, number>; outputs: Map<number, number>}>()
  for (const node of graph.nodes) ports.set(node.id, {inputs: new Map(), outputs: new Map()})
  // 上面的完整性检查保证所有边端点都有端口容器，断言不用于接受缺失节点。
  for (const edge of edges) {
    const inputs = ports.get(edge.target)!.inputs
    const outputs = ports.get(edge.source)!.outputs
    inputs.set(edge.itemId, (inputs.get(edge.itemId) ?? 0) + edge.rate)
    outputs.set(edge.itemId, (outputs.get(edge.itemId) ?? 0) + edge.rate)
  }
  const toPorts = (rates: Map<number, number>, side: 'in' | 'out'): FlowPort[] => [...rates.entries()].sort(([a], [b]) => a - b).map(([itemId, rate]) => ({id: portId(side, itemId), itemId, name: itemName(itemId), rate}))
  const nodes: FlowLayoutNode[] = graph.nodes.map(node => {
    const production = node.kind === 'production' ? plan.productions[node.productionIndex] : undefined
    const nodePorts = ports.get(node.id)!
    const inputs = toPorts(nodePorts.inputs, 'in')
    const outputs = toPorts(nodePorts.outputs, 'out')
    const kind: FlowNodeKind = node.kind === 'production' ? 'production' : node.kind === 'target-sink' ? 'target' : node.kind === 'leftover-sink' ? 'leftover' : 'source'
    return {id: node.id, position: {x: 0, y: 0}, data: {
      kind,
      itemId: node.itemId,
      title: itemName(node.itemId),
      rate: production?.outputCount ?? (kind === 'source' ? outputs : inputs).reduce((sum, port) => sum + port.rate, 0),
      inputs,
      outputs,
      productionIndex: production ? node.productionIndex : undefined,
      production,
      externalSource: node.kind === 'external-source',
    }}
  })

  layout(nodes, edges, new Map(graph.nodes.map(node => [node.id, node.level])), node => measure?.(node.id) ?? estimateHeight(node))
  return {nodes, edges, notice: ''}
}

// =============== 画板分层 ===============
// 说明：后端完成 SCC 缩并与上游到下游层级；前端只在同列按已知上游的流量重心排序，减少视觉交叉。
// 边界：循环边完整保留并用虚线标识，不以 DFS 回边删除真实依赖，不把坐标回传为生成输入。
function layout(nodes: FlowLayoutNode[], edges: FlowLayoutEdge[], levels: Map<string, number>, nodeHeight: (node: FlowLayoutNode) => number): void {
  const columns = new Map<number, FlowLayoutNode[]>()
  for (const node of nodes) {
    const column = levels.get(node.id) ?? 0
    columns.set(column, [...(columns.get(column) ?? []), node])
  }
  const incoming = new Map<string, FlowLayoutEdge[]>()
  for (const edge of edges) if (!edge.cycle) incoming.set(edge.target, [...(incoming.get(edge.target) ?? []), edge])
  const centerY = new Map<string, number>()
  for (const column of [...columns.keys()].sort((a, b) => a - b)) {
    const list = columns.get(column) ?? []
    const weight = (node: FlowLayoutNode, order: number): number => {
      const parents = (incoming.get(node.id) ?? []).filter(edge => centerY.has(edge.source))
      const total = parents.reduce((sum, edge) => sum + edge.rate, 0)
      return total > 0 ? parents.reduce((sum, edge) => sum + centerY.get(edge.source)! * edge.rate, 0) / total : order * 1000
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
