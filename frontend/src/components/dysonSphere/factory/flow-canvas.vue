<script setup lang="ts">
import {Background} from '@vue-flow/background'
import {useVueFlow, VueFlow, type NodeMouseEvent} from '@vue-flow/core'
import '@vue-flow/core/dist/style.css'
import {Crosshair, LayoutGrid, LayoutList, Maximize2, Minus, Plus, Search, X} from '@lucide/vue'
import {computed, nextTick, ref, shallowRef, watch} from 'vue'
import type {QuantWorkspace} from '../../../composables/dysonSphere/useQuantWorkspace'
import {buildFlowGraph, type FlowLayoutNode} from '../../../services/dysonSphere/factory-graph'
import {ensureFactoryIcons} from '../../../services/dysonSphere/factory-icons'
import FactoryNode, {type FactoryNodeContext} from './factory-node.vue'
import FlowEdge from './flow-edge.vue'
import IconSelect from './icon-select.vue'
import ItemIcon from './item-icon.vue'

const props = defineProps<{w: QuantWorkspace}>()
const selectedNode = defineModel<string | null>('selected', {default: null})
const flowId = 'quant-flow'
const {fitView, zoomIn, zoomOut, setCenter, findNode, getNodes, onNodesInitialized} = useVueFlow(flowId)
ensureFactoryIcons()

// =============== 图数据 ===============
// 说明：节点位置来自确定性分层；用户拖动过的节点在同一方案的后续求解中保留位置，「整理」恢复自动布局。
// 首次渲染按估算高度布局，节点完成测量后用实际高度重排一次，避免同列节点重叠。
const measured = shallowRef(new Map<string, number>())
const graph = computed(() => props.w.plan && !props.w.replay ? buildFlowGraph(props.w.plan, props.w.itemName, id => measured.value.get(id)) : {nodes: [], edges: []})
const pinned = shallowRef(new Map<string, {x: number; y: number}>())
const nodes = computed(() => graph.value.nodes.map((node: FlowLayoutNode) => ({id: node.id, type: 'factory', position: pinned.value.get(node.id) ?? node.position, data: node.data, selected: selectedNode.value === node.id})))
const maxRate = computed(() => Math.max(1, ...graph.value.edges.map(edge => edge.rate)))
const related = computed(() => {
  if (!selectedNode.value) return null
  return new Set(graph.value.edges.filter(edge => edge.source === selectedNode.value || edge.target === selectedNode.value).map(edge => edge.id))
})
const edges = computed(() => graph.value.edges.map(edge => ({
  id: edge.id, source: edge.source, target: edge.target, sourceHandle: edge.sourceHandle, targetHandle: edge.targetHandle, type: 'flow',
  data: {name: edge.name, rate: edge.rate, cycle: edge.cycle, label: `${edge.name} ${props.w.number(edge.rate)}`, width: 1 + 3 * Math.log1p(edge.rate) / Math.log1p(maxRate.value), highlighted: related.value?.has(edge.id) ?? false, dimmed: related.value ? !related.value.has(edge.id) : false},
})))

// 选中节点在重算后消失（如改为外供）时清空，避免面板停留在过期节点。
watch(graph, value => { if (selectedNode.value && !value.nodes.some(node => node.id === selectedNode.value)) selectedNode.value = null })

// 新方案（目标集合变化）首次出现时适配视图；同一方案内参数微调不打断用户当前视角。
const planShape = computed(() => graph.value.nodes.map(node => node.id).sort().join('|'))
let lastShape = ''
let fitPending = true
watch(planShape, shape => {
  const before = new Set(lastShape.split('|').filter(Boolean))
  const after = shape.split('|').filter(Boolean)
  const targetsChanged = after.filter(id => id.startsWith('target-')).join('|') !== [...before].filter(id => id.startsWith('target-')).join('|')
  lastShape = shape
  if (!shape) return
  // 目标变化视为新方案并清除手动位置；配方/喷涂引入或移除节点时只重新适配视图。
  if (targetsChanged) pinned.value = new Map()
  if (targetsChanged || after.some(id => !before.has(id)) || after.length !== before.size) fitPending = true
})
// 节点完成测量：记录实际高度（变化超过 2px 才更新，防止循环），首帧或新方案时适配视图。
onNodesInitialized(async () => {
  const next = new Map(measured.value)
  let changed = false
  for (const node of getNodes.value) {
    const height = node.dimensions.height
    if (height && Math.abs((next.get(node.id) ?? 0) - height) > 2) { next.set(node.id, height); changed = true }
  }
  if (changed) measured.value = next
  if (!fitPending) return
  fitPending = false
  await nextTick()
  void fitView({padding: .08, duration: 200, minZoom: .45, maxZoom: 1})
})

// =============== 节点交互上下文 ===============
// 说明：节点内控件通过同一上下文修改工作区请求；配方与喷涂的选项按节点主产物取当前选择的配方。
const context = computed<FactoryNodeContext>(() => ({
  format: props.w.number,
  itemName: props.w.itemName,
  recipeOptions: props.w.recipeOptions,
  recipeChoice: id => props.w.itemRecipeChoices[props.w.itemName(id)] ?? 1,
  buildingOptions: id => props.w.buildingOptions(props.w.selectedRecipe(id)),
  sprayOptions: id => props.w.sprayModes(props.w.selectedRecipe(id)),
  pointOptions: props.w.pointOptions,
  override: id => props.w.overrideFor(props.w.selectedRecipe(id)),
  setRecipe: props.w.updateRecipe,
  setOverride: (id, patch) => props.w.updateOverride(props.w.selectedRecipe(id), patch),
  setTargetRate: props.w.setTargetRate,
  removeTarget: props.w.removeTarget,
  setExternal: props.w.toggleExternal,
  isExternal: props.w.isExternal,
  zoneOf: index => index === undefined || props.w.zones.length < 2 ? undefined : props.w.zoneOfProduction.get(index),
  activeZone: props.w.activeZone,
  locked: props.w.loadingDataset || props.w.importing,
}))

const onNodeClick = ({node}: NodeMouseEvent): void => {
  selectedNode.value = node.id
  const index = (node.data as FlowLayoutNode['data']).productionIndex
  if (index !== undefined) props.w.selectProduction(index)
}
const onPaneClick = (): void => { selectedNode.value = null }
const onNodeDragStop = ({node}: NodeMouseEvent): void => {
  const next = new Map(pinned.value)
  next.set(node.id, {...node.position})
  pinned.value = next
}
const relayout = async (): Promise<void> => {
  pinned.value = new Map()
  measured.value = new Map(getNodes.value.map(node => [node.id, node.dimensions.height]))
  await nextTick()
  void fitView({padding: .15, duration: 200, maxZoom: 1})
}

// 预览选中建筑时，在图中定位并高亮对应产线节点。
watch(() => props.w.selectedProductions, async value => {
  const index = value[0]
  if (index === undefined) return
  const target = graph.value.nodes.find(node => node.data.productionIndex === index)
  if (!target) return
  selectedNode.value = target.id
  const live = findNode(target.id)
  if (live) void setCenter(live.position.x + (live.dimensions.width || 232) / 2, live.position.y + (live.dimensions.height || 160) / 2, {zoom: 1, duration: 250})
})

// =============== 物品面板 ===============
// 说明：类似 draw.io 的元件库，搜索物品后点击加入目标；已有目标再次点击会累加产量。
const paletteOpen = ref(true)
const paletteQuery = ref('')
// 物品库视图：列表显示名称，网格只显示图标（名称在提示中）；选择记在本机，下次打开保持。
const paletteViewKey = 'dsbox.quant.paletteView'
const readPaletteView = (): 'list' | 'grid' => { try { return localStorage.getItem(paletteViewKey) === 'grid' ? 'grid' : 'list' } catch { return 'list' } }
const paletteView = ref<'list' | 'grid'>(readPaletteView())
watch(paletteView, value => { try { localStorage.setItem(paletteViewKey, value) } catch { /* 存储不可用时只保留本次会话 */ } })

// =============== 顶部参数栏 ===============
// 说明：最常改的数据集、生成模式、传送带与分拣器放在画布顶部，其余参数留在格式面板。
// icon 为图鉴中文名：生成模式取该模式的核心物流设施，传送带与分拣器取对应等级；数据集不显示物品图标。
const modeOptions = [{label: '机器直连带', value: 1, icon: '传送带'}, {label: '物流配送器', value: 2, icon: '物流配送器'}, {label: '行星内物流塔', value: 3, icon: '行星内物流运输站'}, {label: '星际物流塔', value: 4, icon: '星际物流运输站'}, {label: '四向分流', value: 5, icon: '四向分流器'}, {label: '混带总线', value: 6, icon: '急速传送带'}]
const beltOptions = [{label: '传送带', value: 1, icon: '传送带'}, {label: '高速带', value: 2, icon: '高速传送带'}, {label: '极速带', value: 3, icon: '急速传送带'}]
const inserterOptions = [{label: '分拣器', value: 1, icon: '分拣器'}, {label: '高速分拣', value: 2, icon: '高速分拣器'}, {label: '极速分拣', value: 3, icon: '极速分拣器'}, {label: '集装分拣', value: 4, icon: '集装分拣器'}]
const datasetOptions = computed(() => props.w.datasets.map(item => ({label: item.name, value: item.key, icon: ''})))
const paletteItems = computed(() => {
  const query = paletteQuery.value.trim().toLowerCase()
  const list = props.w.itemOptions.filter(item => props.w.recipeOptions(item.value).length > 0)
  return (query ? list.filter(item => item.label.toLowerCase().includes(query) || String(item.value).includes(query)) : list).slice(0, 120)
})
const targetIds = computed(() => new Set(props.w.targets.map(target => target.itemId)))
</script>

<template>
  <!-- ~ 量化网格画布 -->
  <!-- * 说明：上层为 draw.io 式网格节点图：左侧物品面板加入目标，画布展示来源 → 产线 → 目标的物料流 -->
  <!-- * 交互：节点内直接改配方、设备、喷涂、产量与外供；拖动节点、平移缩放；点击节点高亮相关连线并联动预览 -->
  <!-- * 依赖：工作区量化快照与求解请求编辑方法；布局与连线速率来自 factory-graph -->
  <!-- * 边界：连线由求解器决定，不能手动连接；导入回放只读时不显示图 -->
  <section class="flow-shell">
    <aside v-show="paletteOpen" class="flow-palette" aria-label="物品面板">
      <header class="palette-head">
        <span>物品库</span>
        <div class="view-switch" role="radiogroup" aria-label="物品库视图">
          <n-tooltip><template #trigger><button type="button" role="radio" class="icon-button" :class="{'is-on': paletteView === 'list'}" :aria-checked="paletteView === 'list'" aria-label="列表视图" @click="paletteView = 'list'"><LayoutList :size="14" /></button></template>列表</n-tooltip>
          <n-tooltip><template #trigger><button type="button" role="radio" class="icon-button" :class="{'is-on': paletteView === 'grid'}" :aria-checked="paletteView === 'grid'" aria-label="图标网格视图" @click="paletteView = 'grid'"><LayoutGrid :size="14" /></button></template>图标网格</n-tooltip>
        </div>
        <button type="button" class="icon-button" aria-label="收起物品库" @click="paletteOpen = false"><X :size="14" /></button>
      </header>
      <n-input v-model:value="paletteQuery" size="small" class="palette-search flow-field" placeholder="搜索物品或 ID" clearable aria-label="搜索物品">
        <template #prefix><Search :size="13" /></template>
      </n-input>
      <div class="palette-list" :class="`is-${paletteView}`">
        <button v-for="item in paletteItems" :key="item.value" type="button" class="palette-item" :class="{'is-target': targetIds.has(item.value)}" :title="`${item.label}：${targetIds.has(item.value) ? '已是目标，点击追加 60/min' : '加入目标 60/min'}`" :aria-label="`${item.label}，${targetIds.has(item.value) ? '追加 60/min' : '加入目标'}`" @click="w.addTargetItem(item.value)">
          <ItemIcon :name="item.label" :size="paletteView === 'grid' ? 28 : 22" /><template v-if="paletteView === 'list'"><span class="palette-label">{{ item.label }}</span><Plus :size="12" /></template>
        </button>
        <p v-if="!paletteItems.length" class="palette-empty">没有匹配的可生产物品</p>
      </div>
    </aside>

    <div class="flow-stage">
      <!-- 浮动工具栏：视图控制与统计 -->
      <!-- 顶部工具栏：左侧常用参数，中间统计与同步状态，最右侧视图控制 -->
      <div class="flow-toolbar">
        <button v-if="!paletteOpen" type="button" class="tool-button" aria-label="展开物品库" @click="paletteOpen = true"><LayoutGrid :size="14" /><span>物品库</span></button>
        <div class="tool-group">
          <IconSelect v-model:value="w.datasetKey" class="tool-select dataset" :options="datasetOptions" :loading="w.loadingDataset" :disabled="w.solving || w.generating" aria-label="数据集" />
          <IconSelect v-model:value="w.mode" class="tool-select" :options="modeOptions" aria-label="生成模式" />
          <IconSelect v-model:value="w.layout.beltLv" class="tool-select short" :options="beltOptions" aria-label="传送带" />
          <IconSelect v-model:value="w.layout.inserterLv" class="tool-select short" :options="inserterOptions" aria-label="分拣器" />
        </div>
        <span v-if="w.plan && !w.replay" class="flow-stats"><span><strong class="type-display">{{ w.plan.machineCount }}</strong> 台</span><span><strong class="type-display">{{ w.number(w.plan.powerMw) }}</strong> MW</span><span><strong class="type-display">{{ w.plan.productions.length }}</strong> 线</span></span>
        <span class="flow-state" :class="{'is-busy': w.solving}">{{ w.solving ? '计算中…' : w.quantStale ? '等待计算' : w.plan ? '已同步' : '' }}</span>
        <div class="tool-group is-view">
          <n-tooltip><template #trigger><button type="button" class="tool-button icon-only" aria-label="放大" @click="zoomIn({duration: 150})"><Plus :size="14" /></button></template>放大</n-tooltip>
          <n-tooltip><template #trigger><button type="button" class="tool-button icon-only" aria-label="缩小" @click="zoomOut({duration: 150})"><Minus :size="14" /></button></template>缩小</n-tooltip>
          <n-tooltip><template #trigger><button type="button" class="tool-button icon-only" aria-label="适配视图" @click="fitView({padding: .15, duration: 200, maxZoom: 1})"><Maximize2 :size="14" /></button></template>适配视图</n-tooltip>
          <n-tooltip><template #trigger><button type="button" class="tool-button icon-only" aria-label="整理布局" @click="relayout"><Crosshair :size="14" /></button></template>整理布局</n-tooltip>
        </div>
      </div>
      <!-- 多目标分区图例：与预览分区同色，点击聚焦对应节点流 -->
      <div v-if="w.zones.length > 1 && !w.replay" class="zone-legend" role="group" aria-label="目标分区">
        <button v-for="zone in w.zones" :key="zone.key" type="button" class="zone-chip" :class="{'is-active': w.activeZone === zone.key, 'is-muted': w.activeZone && w.activeZone !== zone.key}" :style="{'--zone': zone.color}" :aria-pressed="w.activeZone === zone.key" @click="w.focusZone(zone.key)"><i />{{ zone.label }}<span>{{ zone.productionIndices.length }}</span></button>
      </div>

      <VueFlow v-if="nodes.length" :id="flowId" :nodes="nodes" :edges="edges" :min-zoom=".2" :max-zoom="1.6" :nodes-connectable="false" :edges-updatable="false" :elements-selectable="true" :snap-to-grid="true" :snap-grid="[20, 20]" :default-viewport="{zoom: .8, x: 40, y: 200}" class="flow-canvas" @node-click="onNodeClick" @pane-click="onPaneClick" @node-drag-stop="onNodeDragStop">
        <Background variant="lines" :gap="20" :line-width="1" color="var(--flow-grid)" />
        <Background id="major" variant="lines" :gap="100" :line-width="1" color="var(--flow-grid-major)" />
        <template #node-factory="nodeProps"><FactoryNode :id="nodeProps.id" :data="nodeProps.data" :selected="nodeProps.selected" :context="context" /></template>
        <template #edge-flow="edgeProps"><FlowEdge v-bind="edgeProps" /></template>
      </VueFlow>
      <div v-else class="flow-empty">
        <Background variant="lines" :gap="20" :line-width="1" color="var(--flow-grid)" />
        <LayoutGrid :size="26" />
        <p v-if="w.replay">导入的蓝图为只读回放，转为量化目标后可在网格中编辑。</p>
        <p v-else-if="w.solving">正在计算产线…</p>
        <p v-else>从左侧物品库选择目标产物，产线会自动计算并展开为网格图。</p>
      </div>
    </div>
  </section>
</template>

<style scoped lang="scss">
.flow-shell { --flow-grid: color-mix(in srgb, var(--ds-text-primary) 5%, transparent); --flow-grid-major: color-mix(in srgb, var(--ds-text-primary) 9%, transparent); position: relative; display: flex; min-height: 0; height: 100%; }
.flow-palette { display: flex; width: clamp(168px, 16vw, 208px); flex: 0 0 auto; flex-direction: column; border-right: 1px solid var(--ds-border-subtle); background: color-mix(in srgb, var(--ds-bg-panel) 70%, transparent); }
.palette-head { display: flex; height: 36px; align-items: center; gap: 4px; padding: 0 8px 0 12px; color: var(--ds-text-primary); font-size: 13px; font-weight: 600; }
.palette-head > span { flex: 1; }
.view-switch { display: flex; gap: 2px; }
.view-switch .icon-button.is-on { background: var(--ds-active-bg); color: var(--ds-text-primary); }
.palette-search { margin: 0 8px 6px; width: auto; }
.palette-list { min-height: 0; flex: 1; overflow-y: auto; padding: 0 4px 8px; }
.palette-item { display: flex; width: 100%; align-items: center; gap: 8px; border: 1px solid transparent; border-radius: 2px; padding: 3px 6px; background: transparent; color: var(--ds-text-secondary); font-size: 12px; text-align: left; cursor: pointer; }
.palette-label { min-width: 0; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.palette-item > svg { color: var(--ds-text-muted); opacity: 0; }
.palette-item:hover { background: var(--ds-hover-bg); color: var(--ds-text-primary); }
.palette-item:hover > svg { opacity: 1; }
.palette-item.is-target { color: var(--ds-text-primary); }
.palette-item.is-target::after { content: ''; width: 5px; height: 5px; border-radius: 50%; background: var(--ds-warning); }
.palette-list.is-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(38px, 1fr)); align-content: start; gap: 3px; padding: 0 8px 8px; }
.is-grid .palette-item { position: relative; aspect-ratio: 1; justify-content: center; padding: 0; background: var(--ds-hover-bg); }
.is-grid .palette-item:hover { border-color: var(--ds-border-strong); background: var(--ds-active-bg); }
.is-grid .palette-item.is-target::after { position: absolute; top: 3px; right: 3px; }
.palette-empty { padding: 16px 8px; color: var(--ds-text-muted); font-size: 12px; text-align: center; }
// 画布按自身宽度响应：窄时参数选择器收缩并可横向滚动，统计与同步状态隐藏，视图按钮始终固定在最右侧。
.flow-stage { position: relative; min-width: 0; flex: 1; container: flow-stage / inline-size; }
.flow-canvas { width: 100%; height: 100%; }
.flow-toolbar { position: absolute; top: 10px; left: 10px; right: 10px; z-index: 5; display: flex; align-items: center; gap: 6px; pointer-events: none; }
.tool-group { display: flex; min-width: 0; flex: 0 1 auto; align-items: center; gap: 4px; overflow-x: auto; scrollbar-width: none; }
.tool-group::-webkit-scrollbar { display: none; }
.tool-group.is-view { flex: 0 0 auto; margin-left: auto; overflow: visible; }
.tool-group > * { flex: 0 0 auto; }
.tool-select { width: 128px; }
.tool-select.dataset { width: 132px; }
.tool-select.short { width: 108px; }
.tool-select :deep(.n-base-selection) { --n-color: var(--ds-bg-panel-elevated) !important; --n-height: 28px !important; --n-font-size: 12px !important; }
.zone-legend { position: absolute; top: 44px; left: 10px; z-index: 5; display: flex; flex-wrap: wrap; gap: 4px; max-width: calc(100% - 20px); }
.zone-chip { --zone: var(--ds-brand); display: inline-flex; height: 22px; align-items: center; gap: 5px; border: 1px solid color-mix(in srgb, var(--zone) 45%, transparent); border-radius: 2px; padding: 0 7px; background: color-mix(in srgb, var(--ds-bg-panel-elevated) 88%, var(--zone)); color: var(--ds-text-primary); font-size: 11px; cursor: pointer; transition: opacity 150ms, border-color 150ms; }
.zone-chip i { width: 8px; height: 8px; border-radius: 1px; background: var(--zone); }
.zone-chip span { color: var(--ds-text-muted); font-family: var(--ds-font-display); }
.zone-chip.is-active { border-color: var(--zone); box-shadow: inset 0 0 0 1px var(--zone); }
.zone-chip.is-muted { opacity: .45; }
.flow-toolbar > * { pointer-events: auto; }
.tool-button { display: inline-flex; height: 28px; align-items: center; gap: 5px; border: 1px solid var(--ds-border-default); border-radius: 2px; padding: 0 8px; background: var(--ds-bg-panel-elevated); color: var(--ds-text-secondary); font-size: 12px; cursor: pointer; transition: border-color 150ms, color 150ms; }
.tool-button.icon-only { width: 28px; justify-content: center; padding: 0; }
.tool-button:hover { border-color: var(--ds-border-strong); color: var(--ds-text-primary); }
.icon-button { display: inline-flex; width: 22px; height: 22px; align-items: center; justify-content: center; border: 0; border-radius: 2px; background: transparent; color: var(--ds-text-muted); cursor: pointer; }
.icon-button:hover { background: var(--ds-hover-bg); color: var(--ds-text-primary); }
.flow-stats { display: inline-flex; height: 28px; flex: 0 0 auto; align-items: center; gap: 10px; margin-left: 4px; white-space: nowrap; border: 1px solid var(--ds-border-default); border-radius: 2px; padding: 0 10px; background: var(--ds-bg-panel-elevated); color: var(--ds-text-muted); font-size: 11px; }
.flow-stats strong { margin-right: 2px; color: var(--ds-text-primary); font-size: 13px; }
.flow-state { flex: 0 0 auto; color: var(--ds-text-muted); font-size: 11px; white-space: nowrap; }
@container flow-stage (max-width: 980px) { .flow-state { display: none; } }
@container flow-stage (max-width: 860px) { .flow-stats { display: none; } .tool-select { width: 112px; } .tool-select.dataset { width: 112px; } .tool-select.short { width: 96px; } }
.flow-state.is-busy { color: var(--ds-brand); }
.flow-empty { position: relative; display: flex; height: 100%; flex-direction: column; align-items: center; justify-content: center; gap: 10px; color: var(--ds-text-muted); font-size: 13px; text-align: center; }
.flow-empty p { max-width: 320px; margin: 0; line-height: 1.7; }
.flow-field { --n-color: var(--ds-hover-bg) !important; --n-border-radius: 2px !important; --n-box-shadow-focus: none !important; }
:deep(.vue-flow__node-factory) { padding: 0; border: 0; background: transparent; }
:deep(.vue-flow__node.selected), :deep(.vue-flow__node:focus) { outline: none; box-shadow: none; }
:deep(.vue-flow__edge-path) { stroke-linecap: round; }
</style>
