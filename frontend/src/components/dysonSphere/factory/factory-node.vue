<script setup lang="ts">
import {Handle, Position} from '@vue-flow/core'
import {PackageOpen} from '@lucide/vue'
import {computed} from 'vue'
import type {FlowNodeData} from '../../../services/dysonSphere/factory-graph'
import IconSelect from './icon-select.vue'
import ItemIcon from './item-icon.vue'

// 节点上下文：编辑配置与格式化由画布统一注入，节点只负责渲染端口和触发编辑。
export interface FactoryNodeContext {
  format: (value: number) => string
  recipeOptions: (itemId: number) => {label: string; value: number; icon?: string}[]
  recipeChoice: (itemId: number) => number
  buildingOptions: (itemId: number) => {label: string; value: number; icon?: string}[]
  sprayOptions: (itemId: number) => {label: string; value: number; icon?: string}[]
  pointOptions: {label: string; value: number; icon?: string}[]
  override: (itemId: number) => {building: number; productionMode: number; increasePoints: number}
  itemName: (id: number) => string
  setRecipe: (itemId: number, choice: number) => void
  setOverride: (itemId: number, patch: Partial<{building: number; productionMode: number; increasePoints: number}>) => void
  setTargetRate: (itemId: number, rate: number) => void
  removeTarget: (itemId: number) => void
  setExternal: (itemId: number, external: boolean) => void
  isExternal: (itemId: number) => boolean
  zoneOf: (productionIndex: number | undefined) => {key: string; color: string; label: string} | undefined
  activeZone: string | null
  locked: boolean
}

const props = defineProps<{id: string; data: FlowNodeData; selected: boolean; context: FactoryNodeContext}>()
const production = computed(() => props.data.production)
const override = computed(() => props.context.override(props.data.itemId))
const recipeOptions = computed(() => props.context.recipeOptions(props.data.itemId))
const sprayLabel = computed(() => props.context.sprayOptions(props.data.itemId).find(option => option.value === override.value.productionMode)?.label ?? '不喷涂')
// 分区：多目标时节点左侧色条与顶部标签显示所属目标分区；聚焦某分区时其它节点变淡。
const zone = computed(() => props.context.zoneOf(props.data.productionIndex))
const dimmed = computed(() => Boolean(props.context.activeZone && zone.value && zone.value.key !== props.context.activeZone))
const kindLabel: Record<FlowNodeData['kind'], string> = {source: '外部输入', production: '产线', target: '目标', leftover: '剩余'}
</script>

<template>
  <!-- ~ 量化节点 -->
  <!-- * 说明：类似数字电路元件，左侧为输入端口、右侧为输出端口，端口标注物品与每分钟速率 -->
  <!-- * 交互：产线节点内可直接切换配方、设备、喷涂与点数；目标节点可改产量；来源节点可取消外供 -->
  <!-- * 依赖：画布注入的 FactoryNodeContext；数值均来自 Go 求解快照 -->
  <!-- * 边界：控件阻止拖拽冒泡（nodrag），编辑只修改请求参数，由工作区重新求解 -->
  <div class="factory-node" :class="[`is-${data.kind}`, {'is-selected': selected, 'is-dimmed': dimmed}]" :style="zone ? {'--accent': zone.color} : undefined" :data-production="data.productionIndex" :data-zone="zone?.key">
    <header class="node-head">
      <ItemIcon :name="data.title" :size="data.kind === 'production' ? 30 : 24" />
      <div class="node-title"><span class="node-kind">{{ data.kind === 'source' ? data.externalSource ? '指定外供' : '外部原料' : kindLabel[data.kind] }}<template v-if="zone"> · {{ zone.label }}</template></span><strong>{{ data.title }}</strong></div>
      <span class="node-rate" :title="data.kind === 'production' ? '期望 / 配置满负荷。满负荷要求供料、供电和出货畅通，不是游戏实测；蓝图物流仍按期望净流规划。' : '规划期望流量'"><span class="rate-label">{{ data.kind === 'production' ? '期望 / 配置满负荷' : '期望' }}</span><span class="type-display">{{ context.format(data.expectedRate) }}<small>/min</small><template v-if="data.kind === 'production'"> / {{ data.capacity ? context.format(data.capacity.outputPerMinute) : '—' }}<small>/min</small></template></span></span>
      <n-tooltip v-if="data.kind === 'production'"><template #trigger><button type="button" class="node-action nodrag" :disabled="context.locked" :aria-label="`${data.title} 改为外部供应`" @click.stop="context.setExternal(data.itemId, true)"><PackageOpen :size="13" /></button></template>改为外部供应</n-tooltip>
    </header>

    <!-- 产线参数：机器数 × 设备，配方，喷涂与功耗 -->
    <div v-if="data.kind === 'production' && production" class="node-body nodrag nowheel">
      <div class="node-metrics">
        <span class="machine-count type-display">{{ production.machineCount }}<small>×</small></span>
        <span class="machine-name">{{ context.itemName(production.machineId) }}</span>
        <span class="node-power type-display" title="按期望速率折算的工作功率">{{ context.format(production.powerMw) }} MW</span>
      </div>
      <div class="node-fields">
        <IconSelect v-if="recipeOptions.length > 1" class="node-field" :icon-size="14" :value="context.recipeChoice(data.itemId)" :options="recipeOptions" :disabled="context.locked" aria-label="配方" @update:value="value => context.setRecipe(data.itemId, Number(value))" />
        <IconSelect v-if="context.buildingOptions(data.itemId).length > 1" class="node-field" :icon-size="14" :value="override.building" :options="context.buildingOptions(data.itemId)" :disabled="context.locked" aria-label="设备" @update:value="value => context.setOverride(data.itemId, {building: Number(value)})" />
        <div class="spray-row">
          <IconSelect class="node-field" :icon-size="14" :value="override.productionMode" :options="context.sprayOptions(data.itemId)" :disabled="context.locked" aria-label="喷涂模式" @update:value="value => context.setOverride(data.itemId, {productionMode: Number(value)})" />
          <IconSelect v-if="override.productionMode > 0" class="node-field points" :icon-size="14" :value="override.increasePoints" :options="context.pointOptions" :disabled="context.locked" aria-label="喷涂点数" @update:value="value => context.setOverride(data.itemId, {increasePoints: Number(value)})" />
        </div>
      </div>
      <div class="node-foot"><span :class="{'is-spray': override.productionMode > 0}">{{ sprayLabel }}</span><span>精确 {{ context.format(production.exactMachineCount) }} · 余量 {{ context.format(production.roundingMargin) }}</span></div>
    </div>

    <div v-else-if="data.kind === 'target'" class="node-body nodrag nowheel">
      <n-input-number size="tiny" class="node-field" :value="data.expectedRate" :min="0.000001" :max="1000000000" :show-button="false" :disabled="context.locked" aria-label="期望目标产量" @update:value="value => value && context.setTargetRate(data.itemId, value)"><template #suffix>/min</template></n-input-number>
      <button type="button" class="node-link" :disabled="context.locked" @click="context.removeTarget(data.itemId)">移除目标</button>
    </div>
    <div v-else-if="data.kind === 'source' && context.isExternal(data.itemId)" class="node-body nodrag">
      <button type="button" class="node-link" :disabled="context.locked" @click="context.setExternal(data.itemId, false)">改为本地生产</button>
    </div>

    <!-- 端口：每个物品一个连接点，标签只显示期望净流量，不以设备满负荷替换守恒边 -->
    <div class="node-ports" title="端口为期望净流量 /min">
      <div class="port-column is-in">
        <div v-for="port in data.inputs" :key="port.id" class="port-row">
          <Handle :id="port.id" type="target" :position="Position.Left" class="port-handle" :connectable="false" />
          <ItemIcon :name="port.name" :size="16" /><span class="port-name">{{ port.name }}</span><span class="port-rate type-display">{{ context.format(port.rate) }}</span>
        </div>
      </div>
      <div class="port-column is-out">
        <div v-for="port in data.outputs" :key="port.id" class="port-row">
          <span class="port-rate type-display">{{ context.format(port.rate) }}</span><span class="port-name">{{ port.name }}</span><ItemIcon :name="port.name" :size="16" />
          <Handle :id="port.id" type="source" :position="Position.Right" class="port-handle" :connectable="false" />
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
// 元件外壳：右上 / 左下切角与图鉴面板一致，左侧色条区分节点类型。
.factory-node { --chamfer: 10px; --accent: var(--ds-brand); position: relative; width: 232px; color: var(--ds-text-primary); font-size: 12px; isolation: isolate; }
.factory-node::before, .factory-node::after { content: ''; position: absolute; z-index: -1; clip-path: polygon(0 0, calc(100% - var(--chamfer)) 0, 100% var(--chamfer), 100% 100%, var(--chamfer) 100%, 0 calc(100% - var(--chamfer))); }
.factory-node::before { inset: 0; background: var(--ds-border-default); transition: background-color 150ms; }
.factory-node::after { inset: 1px; background: var(--ds-bg-panel-elevated); }
.factory-node:hover::before { background: var(--ds-border-strong); }
.factory-node.is-selected::before { background: var(--ds-brand); }
.factory-node.is-dimmed { opacity: .32; }
.factory-node { transition: opacity 150ms; }
.is-source { --accent: var(--ds-success); width: 200px; }
.is-target { --accent: var(--ds-warning); width: 200px; }
.is-leftover { --accent: var(--ds-text-muted); width: 200px; }
.node-head { position: relative; display: flex; align-items: center; gap: 8px; border-bottom: 1px solid var(--ds-border-subtle); padding: 8px 10px 8px 12px; }
.node-head::before { content: ''; position: absolute; top: 9px; bottom: 9px; left: 0; width: 2px; background: var(--accent); }
.node-title { display: flex; min-width: 0; flex: 1; flex-direction: column; }
.node-title strong { overflow: hidden; font-size: 13px; font-weight: 600; line-height: 1.35; text-overflow: ellipsis; white-space: nowrap; }
.node-kind { overflow: hidden; color: var(--ds-text-muted); font-size: 10px; letter-spacing: .04em; text-overflow: ellipsis; white-space: nowrap; }
.node-rate { display: flex; flex: 0 0 auto; flex-direction: column; align-items: flex-end; color: var(--accent); font-size: 12px; font-weight: 600; line-height: 1.2; white-space: nowrap; }
.node-rate .rate-label { color: var(--ds-text-muted); font-size: 9px; font-weight: 400; }
.node-rate small { margin-left: 1px; color: var(--ds-text-muted); font-size: 10px; font-weight: 400; }
.node-body { display: grid; gap: 6px; border-bottom: 1px solid var(--ds-border-subtle); padding: 8px 10px; }
.node-metrics { display: flex; align-items: baseline; gap: 6px; }
.machine-count { font-size: 18px; font-weight: 700; line-height: 1; }
.machine-count small { margin-left: 1px; color: var(--ds-text-muted); font-size: 11px; }
.machine-name { min-width: 0; flex: 1; overflow: hidden; color: var(--ds-text-secondary); text-overflow: ellipsis; white-space: nowrap; }
.node-power { flex: 0 0 auto; color: var(--ds-text-muted); font-size: 11px; }
.node-fields { display: grid; gap: 4px; }
.spray-row { display: flex; gap: 4px; }
.spray-row > .node-field { min-width: 0; flex: 1; }
.spray-row > .points { flex: 0 0 78px; }
.node-field :deep(.n-base-selection) { --n-font-size: 12px !important; --n-height: 22px !important; }
.node-foot { display: flex; justify-content: space-between; gap: 6px; color: var(--ds-text-muted); font-size: 10px; }
.node-foot .is-spray { color: var(--ds-brand); }
.node-action { display: inline-flex; width: 22px; height: 22px; flex: 0 0 auto; align-items: center; justify-content: center; margin-right: -4px; border: 0; border-radius: 2px; padding: 0; background: transparent; color: var(--ds-text-muted); cursor: pointer; opacity: 0; transition: opacity 150ms, background-color 150ms; }
.factory-node:hover .node-action, .node-action:focus-visible { opacity: 1; }
.node-action:hover:not(:disabled) { background: var(--ds-hover-bg); color: var(--ds-text-primary); }
.node-action:disabled { cursor: not-allowed; }
.node-link { justify-self: start; border: 0; padding: 0; background: transparent; color: var(--ds-text-secondary); font-size: 11px; cursor: pointer; }
.node-link:hover:not(:disabled) { color: var(--ds-brand); }
.node-link:disabled { cursor: not-allowed; opacity: .5; }
.node-ports { display: flex; justify-content: space-between; gap: 8px; padding: 5px 0; }
.port-column { display: flex; min-width: 0; flex: 1; flex-direction: column; }
.port-column.is-out { align-items: flex-end; }
.port-row { position: relative; display: flex; height: 22px; max-width: 100%; align-items: center; gap: 4px; padding: 0 10px; }
.port-name { min-width: 0; overflow: hidden; color: var(--ds-text-secondary); font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
.port-rate { flex: 0 0 auto; color: var(--ds-text-primary); font-size: 11px; }
// 端口点：与节点描边同色的小方块，悬停端口行时提升为强调色。
.port-handle { width: 7px !important; height: 7px !important; min-width: 0 !important; min-height: 0 !important; border: 1px solid var(--ds-bg-panel-elevated) !important; border-radius: 1px !important; background: var(--ds-text-muted) !important; }
.is-in .port-handle { left: -4px !important; }
.is-out .port-handle { right: -4px !important; }
.port-row:hover .port-handle { background: var(--ds-brand) !important; }
</style>
