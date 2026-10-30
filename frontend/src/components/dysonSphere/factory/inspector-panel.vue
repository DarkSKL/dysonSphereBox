<script setup lang="ts">
import {ArrowRightLeft, Factory, PackageOpen, Settings2, Trash2, X} from '@lucide/vue'
import {computed} from 'vue'
import type {QuantWorkspace} from '../../../composables/dysonSphere/useQuantWorkspace'
import type {FlowNodeData} from '../../../services/dysonSphere/factory-graph'
import IconSelect from './icon-select.vue'
import ItemIcon from './item-icon.vue'

const props = defineProps<{w: QuantWorkspace; node: FlowNodeData | null}>()
const emit = defineEmits<{(event: 'clear'): void}>()

const merges = [{label: '纵向单列', value: 1, icon: ''}, {label: '按宽度换列', value: 2, icon: ''}]
const surfaceModes = [{label: '自动选择纬圈', value: 'autoTropic', icon: ''}, {label: '指定纬圈', value: 'explicitTropic', icon: ''}, {label: '旧平面兼容', value: 'planar', icon: ''}]

// 选中节点对应的配方与配置；节点主产物决定配方，喷涂与设备按配方保存。
const recipe = computed(() => props.node ? props.w.selectedRecipe(props.node.itemId) : null)
const override = computed(() => props.w.overrideFor(recipe.value))
const production = computed(() => props.node?.production)
const kindLabel: Record<FlowNodeData['kind'], string> = {source: '外部输入', production: '产线', target: '目标产物', leftover: '剩余副产物'}
const importedRows = computed(() => {
  const rows = new Map<number, {recipeId: number; count: number; first: number; outputs: string}>()
  for (const machine of props.w.inspection?.machines ?? []) {
    const existing = rows.get(machine.recipeId)
    if (existing) existing.count++
    else rows.set(machine.recipeId, {recipeId: machine.recipeId, count: 1, first: machine.buildingIndex, outputs: machine.outputs.map(output => props.w.itemName(output.itemId)).join('、')})
  }
  return [...rows.values()]
})
</script>

<template>
  <!-- ~ 格式面板 -->
  <!-- * 说明：类似 draw.io 右侧格式面板；未选节点时编辑全局计算与布局参数，选中节点时编辑该元件 -->
  <!-- * 交互：所有修改立即写入工作区请求，由自动量化与生成刷新画布和预览 -->
  <!-- * 依赖：工作区请求状态；节点数据来自量化快照 -->
  <!-- * 边界：导入回放为只读，只提供转为量化目标的入口 -->
  <aside class="inspector" aria-label="参数面板">
    <!-- 导入回放 -->
    <template v-if="w.replay && w.inspection">
      <header class="inspector-head"><span>{{ w.sourceName }} · 只读回放</span></header>
      <div class="inspector-body">
        <div class="metric-grid ds-bracket"><div><span>建筑</span><strong class="type-display">{{ w.inspection.summary.buildingCount }}</strong></div><div><span>制造设备</span><strong class="type-display">{{ w.inspection.machines.length }}</strong></div></div>
        <div class="list">
          <button v-for="row in importedRows" :key="row.recipeId" type="button" class="list-row" :class="{'is-active': w.selectedBuilding === row.first}" @click="w.selectedBuilding = row.first"><span class="row-main">{{ row.outputs }}</span><span class="row-value type-display">× {{ row.count }}</span></button>
        </div>
        <h3>未喷涂理论产出</h3>
        <div class="list"><div v-for="output in w.inspection.outputs" :key="output.itemId" class="list-row"><ItemIcon :name="output.name" :size="18" /><span class="row-main">{{ output.name }}</span><span class="row-value type-display">{{ w.number(output.ratePerMinute) }}</span></div></div>
        <n-alert v-if="w.inspection.unknownIds.length" type="warning" class="inline-alert">无法识别的原型：{{ w.inspection.unknownIds.join('、') }}</n-alert>
        <n-button size="small" type="primary" block @click="w.recalculateImport"><template #icon><ArrowRightLeft :size="14" /></template>转为量化目标</n-button>
      </div>
    </template>

    <!-- 选中节点 -->
    <template v-else-if="node">
      <header class="inspector-head"><ItemIcon :name="node.title" :size="22" /><span class="head-title"><small>{{ kindLabel[node.kind] }}</small>{{ node.title }}</span><button type="button" class="icon-button" aria-label="返回全局参数" @click="emit('clear')"><X :size="14" /></button></header>
      <div class="inspector-body">
        <template v-if="node.kind === 'production' && production">
          <div class="metric-grid ds-bracket">
            <div><span>产出</span><strong class="type-display">{{ w.number(production.outputCount) }}<small>/min</small></strong></div>
            <div><span>设备</span><strong class="type-display">{{ production.machineCount }}<small>台</small></strong></div>
            <div><span>功耗</span><strong class="type-display">{{ w.number(production.powerMw) }}<small>MW</small></strong></div>
            <div><span>取整余量</span><strong class="type-display">{{ w.number(production.roundingMargin) }}</strong></div>
          </div>
          <label class="field">配方<IconSelect size="small" :value="w.itemRecipeChoices[w.itemName(node.itemId)] ?? 1" :options="w.recipeOptions(node.itemId)" @update:value="value => w.updateRecipe(node!.itemId, Number(value))" /></label>
          <label class="field">设备<IconSelect size="small" :value="override.building" :options="w.buildingOptions(recipe)" @update:value="value => w.updateOverride(recipe, {building: Number(value)})" /></label>
          <div class="field-row">
            <label class="field">喷涂<IconSelect size="small" :value="override.productionMode" :options="w.sprayModes(recipe)" @update:value="value => w.updateOverride(recipe, {productionMode: Number(value)})" /></label>
            <label class="field">点数<IconSelect size="small" :value="override.increasePoints" :options="w.pointOptions" :disabled="!override.productionMode" @update:value="value => w.updateOverride(recipe, {increasePoints: Number(value)})" /></label>
          </div>
          <h3>输入</h3>
          <div class="list"><div v-for="port in node.inputs" :key="port.id" class="list-row"><ItemIcon :name="port.name" :size="18" /><span class="row-main">{{ port.name }}</span><span class="row-value type-display">{{ w.number(port.rate) }}</span></div></div>
          <h3>输出</h3>
          <div class="list"><div v-for="port in node.outputs" :key="port.id" class="list-row"><ItemIcon :name="port.name" :size="18" /><span class="row-main">{{ port.name }}</span><span class="row-value type-display">{{ w.number(port.rate) }}</span></div></div>
          <n-button size="small" block secondary @click="w.toggleExternal(node.itemId, true)">改为外部供应</n-button>
        </template>
        <template v-else-if="node.kind === 'target'">
          <label class="field">目标产量<n-input-number size="small" :value="node.rate" :min="0.000001" :max="1000000000" :show-button="false" @update:value="value => value && w.setTargetRate(node!.itemId, value)"><template #suffix>/min</template></n-input-number></label>
          <n-button size="small" block secondary :disabled="w.targets.length <= 1" @click="w.removeTarget(node.itemId); emit('clear')"><template #icon><Trash2 :size="14" /></template>移除目标</n-button>
        </template>
        <template v-else-if="node.kind === 'source'">
          <div class="metric-grid single ds-bracket"><div><span>外部供应</span><strong class="type-display">{{ w.number(node.rate) }}<small>/min</small></strong></div></div>
          <n-button v-if="w.isExternal(node.itemId)" size="small" block secondary @click="w.toggleExternal(node.itemId, false)">改为本地生产</n-button>
        </template>
        <template v-else>
          <div class="metric-grid single ds-bracket"><div><span>剩余</span><strong class="type-display">{{ w.number(node.rate) }}<small>/min</small></strong></div></div>
          <n-button size="small" block secondary @click="w.addTargetItem(node.itemId, node.rate)">作为目标产出</n-button>
        </template>
      </div>
    </template>

    <!-- 全局参数 -->
    <template v-else>
      <header class="inspector-head"><Settings2 :size="15" /><span class="head-title">计算与布局</span></header>
      <div class="inspector-body">
        <!-- 目标：图标 + 名称 + 速率一行，速率直接输入；物品库或画布中添加，名称可下拉更换 -->
        <h3>目标<span class="count">{{ w.targets.length }}</span></h3>
        <div class="target-list">
          <div v-for="target in w.targets" :key="target.key" class="target-card ds-bracket">
            <ItemIcon :name="target.itemId ? w.itemName(target.itemId) : ''" :size="26" />
            <div class="target-main">
              <IconSelect v-model:value="target.itemId" filterable :options="w.itemOptions" :icon-size="0" placeholder="选择物品" class="target-item" aria-label="目标物品" />
              <n-input-number v-model:value="target.ratePerMinute" size="tiny" :min="0.000001" :max="1000000000" :show-button="false" class="target-rate" aria-label="目标产量"><template #suffix>/min</template></n-input-number>
            </div>
            <button type="button" class="icon-button" :disabled="w.targets.length <= 1" aria-label="移除目标" @click="w.targets.splice(w.targets.indexOf(target), 1)"><X :size="13" /></button>
          </div>
          <button type="button" class="target-add" :disabled="w.targets.length >= 100" @click="w.addTarget">＋ 添加目标</button>
        </div>

        <h3>计算</h3>
        <div class="field-row">
          <label class="field">采矿倍率<n-input-number v-model:value="w.miningSpeed" size="small" :min="0.01" :max="1000000" :step="0.1" /></label>
          <label class="field">小数位<n-input-number v-model:value="w.fixedNum" size="small" :min="0" :max="8" :precision="0" /></label>
        </div>
        <div class="field-row">
          <label class="field">占地权重<n-input-number v-model:value="w.spaceCost" size="small" :min="0" /></label>
          <label class="field">电力权重<n-input-number v-model:value="w.powerCost" size="small" :min="0" /></label>
        </div>
        <label class="field">研究站堆叠<n-input-number v-model:value="w.researchStack" size="small" :min="1" :max="15" /></label>
        <n-checkbox v-model:checked="w.sprayItself" size="small">增产剂自喷涂</n-checkbox>
        <!-- ~ 高级量化设置 -->
        <!-- * 说明：编辑现有量化内核的采集基础值和倍率，不包含蓝图布局参数 -->
        <!-- * 交互：清空数值恢复默认，修改后沿用工作区计算流程 -->
        <!-- * 依赖：useQuantWorkspace 的 Settings 请求映射与 Go 边界校验 -->
        <!-- * 边界：气体和蓝 Buff 只对相应数据集生效，不代表采集器最终净产量 -->
        <details class="advanced-settings">
          <summary>高级采集与增产设置</summary>
          <div class="advanced-fields">
            <p class="surface-hint">速率为基础面板值（物品/秒），不是采集器净产量。清空恢复默认；气体与蓝 Buff 仅对相应数据集生效。</p>
            <div class="field-row"><label class="field">油井面板 /s<n-input-number v-model:value="w.miningSpeedOil" size="small" :min="0.000001" :max="1000000" /></label><label class="field">巨星氢基础 /s<n-input-number v-model:value="w.miningSpeedHydrogen" size="small" :min="0.000001" :max="1000000" /></label></div>
            <div class="field-row"><label class="field">重氢基础 /s<n-input-number v-model:value="w.miningSpeedDeuterium" size="small" :min="0.000001" :max="1000000" /></label><label class="field">可燃冰基础 /s<n-input-number v-model:value="w.miningSpeedGasHydrate" size="small" :min="0.000001" :max="1000000" /></label></div>
            <div class="field-row"><label class="field">氦基础 /s<n-input-number v-model:value="w.miningSpeedHelium" size="small" :min="0.000001" :max="1000000" /></label><label class="field">氨基础 /s<n-input-number v-model:value="w.miningSpeedAmmonia" size="small" :min="0.000001" :max="1000000" /></label></div>
            <div class="field-row"><label class="field">氮基础 /s<n-input-number v-model:value="w.miningSpeedNitrogen" size="small" :min="0.000001" :max="1000000" /></label><label class="field">氧基础 /s<n-input-number v-model:value="w.miningSpeedOxygen" size="small" :min="0.000001" :max="1000000" /></label></div>
            <div class="field-row"><label class="field">二氧化碳基础 /s<n-input-number v-model:value="w.miningSpeedCarbonDioxide" size="small" :min="0.000001" :max="1000000" /></label><label class="field">二氧化硫基础 /s<n-input-number v-model:value="w.miningSpeedSulfurDioxide" size="small" :min="0.000001" :max="1000000" /></label></div>
            <div class="field-row"><label class="field">小矿覆盖（条）<n-input-number v-model:value="w.coveredVeinsSmall" size="small" :min="0.000001" :max="1000000" /></label><label class="field">大矿覆盖（条）<n-input-number v-model:value="w.coveredVeinsLarge" size="small" :min="0.000001" :max="1000000" /></label></div>
            <div class="field-row"><label class="field">大矿机效率倍率<n-input-number v-model:value="w.miningEfficiencyLarge" size="small" :min="0.000001" :max="1000000" /></label><label class="field">敌人掉落倍率<n-input-number v-model:value="w.enemyDropMultiple" size="small" :min="0.000001" :max="1000000" /></label></div>
            <div class="field-row"><label class="field">机甲制造倍率<n-input-number v-model:value="w.icarusManufacturingSpeed" size="small" :min="0.000001" :max="1000000" /></label><label class="field">分馏过带 /s<n-input-number v-model:value="w.fractionatingSpeed" size="small" :min="0.000001" :max="1000000" /></label></div>
            <div class="field-row"><label class="field">加速修正倍率<n-input-number v-model:value="w.accRate" size="small" :min="0.000001" :max="1000000" /></label><label class="field">增产修正倍率<n-input-number v-model:value="w.incRate" size="small" :min="0.000001" :max="1000000" /></label></div>
            <n-checkbox v-model:checked="w.blueBuff" size="small">深空来敌蓝 Buff</n-checkbox>
          </div>
        </details>
        <template v-if="w.mineralized.length">
          <h3><PackageOpen :size="13" />外部供应<span class="count">{{ w.mineralized.length }}</span></h3>
          <div class="chip-list">
            <span v-for="name in w.mineralized" :key="name" class="supply-chip"><ItemIcon :name="name" :size="16" />{{ name }}<button type="button" class="chip-remove" :aria-label="`${name} 改为本地生产`" @click="w.mineralized = w.mineralized.filter(value => value !== name)"><X :size="11" /></button></span>
          </div>
        </template>

        <h3><Factory :size="13" />蓝图布局</h3>
        <div class="field-row">
          <label class="field">球面策略<IconSelect v-model:value="w.surfaceLayout.mode" size="small" :options="surfaceModes" /></label>
          <label class="field" v-if="w.surfaceLayout.mode === 'explicitTropic'">纬向格索引<n-input-number v-model:value="w.surfaceLayout.latitudeGridIndex" size="small" :min="-w.surfaceLayout.gridSegment * 5 / 4" :max="w.surfaceLayout.gridSegment * 5 / 4" :precision="0" /></label>
        </div>
        <n-checkbox v-if="w.surfaceLayout.mode !== 'planar'" v-model:checked="w.surfaceLayout.expandAdjacent" size="small">容量不足时将后续产线扩展到邻圈</n-checkbox>
        <p v-if="w.surfaceLayout.mode !== 'planar'" class="surface-hint">适用于原版星球，按纬圈容量安排完整产线。</p>
        <p v-else class="surface-hint">按平面网格排列产线，不进行纬圈适配。</p>
        <div class="field-row">
          <label class="field">单列机器<n-input-number v-model:value="w.layout.maxMachineInALine" size="small" :min="1" :max="200" /></label>
          <label class="field">研究站层数<n-input-number v-model:value="w.layout.maxLays" size="small" :min="1" :max="15" /></label>
        </div>
        <div class="field-row">
          <label class="field">街区宽度<n-input-number v-model:value="w.layout.blockWidthLimit" size="small" :min="10" :max="500" /></label>
          <label class="field">排列<IconSelect v-model:value="w.layout.mergeType" size="small" :options="merges" /></label>
        </div>
        <div class="flags">
          <n-checkbox v-model:checked="w.layout.needPower" size="small">生成供电设施</n-checkbox>
          <n-checkbox v-model:checked="w.layout.needCoaterNet" size="small">连接喷涂供剂网络</n-checkbox>
          <n-checkbox v-model:checked="w.layout.useMixed" size="small">混带输入（模式 1/3/4/6）</n-checkbox>
          <n-checkbox v-model:checked="w.layout.mix2" size="small" :disabled="!w.layout.useMixed">混带 2.0 补料</n-checkbox>
        </div>
      </div>
    </template>
  </aside>
</template>

<style scoped lang="scss">
.inspector { display: flex; width: clamp(232px, 20vw, 280px); min-height: 0; flex: 0 0 auto; flex-direction: column; border-left: 1px solid var(--ds-border-subtle); background: color-mix(in srgb, var(--ds-bg-panel) 70%, transparent); color: var(--ds-text-primary); }
.inspector-head { position: relative; display: flex; min-height: 40px; align-items: center; gap: 8px; border-bottom: 1px solid var(--ds-border-subtle); padding: 6px 10px 6px 14px; font-size: 13px; font-weight: 600; }
.inspector-head > svg { color: var(--ds-text-secondary); }
.head-title { display: flex; min-width: 0; flex: 1; flex-direction: column; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.head-title small { color: var(--ds-text-muted); font-size: 10px; font-weight: 400; }
.inspector-body { display: flex; min-height: 0; flex: 1; flex-direction: column; gap: 8px; overflow-y: auto; padding: 10px 12px 16px; }
h3 { display: flex; align-items: center; gap: 5px; margin: 8px 0 0; color: var(--ds-text-muted); font-size: 11px; font-weight: 600; letter-spacing: .06em; }
.field { display: flex; min-width: 0; flex: 1; flex-direction: column; gap: 4px; color: var(--ds-text-secondary); font-size: 11px; }
.field-row { display: flex; gap: 8px; }
.advanced-settings summary { color: var(--ds-text-secondary); font-size: 11px; cursor: pointer; }
.advanced-fields { display: grid; gap: 8px; padding-top: 8px; }
.surface-hint { margin: 0; color: var(--ds-text-muted); font-size: 10px; line-height: 1.45; }
.flags { display: grid; gap: 4px; }
.count { margin-left: auto; color: var(--ds-text-muted); font-family: var(--ds-font-display); font-weight: 400; letter-spacing: 0; }
.metric-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 1px; background: var(--ds-border-subtle); }
.metric-grid > div { display: flex; flex-direction: column; gap: 2px; padding: 6px 8px; background: var(--ds-bg-panel-elevated); }
.metric-grid.single { grid-template-columns: 1fr; }
.metric-grid span { color: var(--ds-text-muted); font-size: 10px; }
.metric-grid strong { font-size: 15px; }
.metric-grid small { margin-left: 2px; color: var(--ds-text-muted); font-size: 10px; font-weight: 400; }
.list { display: grid; gap: 1px; }
.list-row { display: flex; width: 100%; align-items: center; gap: 6px; border: 0; border-radius: 2px; padding: 3px 4px; background: transparent; color: var(--ds-text-secondary); font-size: 12px; text-align: left; }
button.list-row { cursor: pointer; }
button.list-row:hover, .list-row.is-active { background: var(--ds-hover-bg); color: var(--ds-text-primary); }
.row-main { min-width: 0; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.row-value { flex: 0 0 auto; color: var(--ds-text-primary); }
.target-list { display: grid; gap: 4px; }
.target-card { --bracket-color: var(--ds-border-strong); display: flex; align-items: center; gap: 8px; padding: 6px 4px 6px 10px; background: var(--ds-bg-panel-elevated); }
.target-card > * { position: relative; z-index: 1; }
.target-card:hover, .target-card:focus-within { --bracket-color: var(--ds-brand); }
.target-main { display: flex; min-width: 0; flex: 1; flex-direction: column; gap: 2px; }
.target-item { min-width: 0; }
.target-item :deep(.n-base-selection) { --n-border: 1px solid transparent !important; --n-color: transparent !important; --n-padding-single-left: 2px !important; --n-font-size: 13px !important; font-weight: 600; }
.target-rate { width: 100%; }
.target-rate :deep(.n-input) { --n-border: 1px solid transparent !important; --n-color: var(--ds-hover-bg) !important; --n-padding-left: 4px !important; --n-font-size: 12px !important; font-family: var(--ds-font-display); }
.target-add { border: 1px dashed var(--ds-border-default); border-radius: 2px; padding: 5px 0; background: transparent; color: var(--ds-text-secondary); font-size: 12px; cursor: pointer; transition: border-color 150ms, color 150ms; }
.target-add:hover:not(:disabled) { border-color: var(--ds-brand); color: var(--ds-brand); }
.target-add:disabled { cursor: not-allowed; opacity: .4; }
.chip-list { display: flex; flex-wrap: wrap; gap: 4px; }
.supply-chip { display: inline-flex; height: 22px; align-items: center; gap: 4px; border: 1px solid var(--ds-border-subtle); border-radius: 2px; padding: 0 2px 0 5px; background: var(--ds-bg-panel-elevated); color: var(--ds-text-secondary); font-size: 11px; }
.chip-remove { display: inline-flex; width: 16px; height: 16px; align-items: center; justify-content: center; border: 0; border-radius: 2px; padding: 0; background: transparent; color: var(--ds-text-muted); cursor: pointer; }
.chip-remove:hover { background: var(--ds-hover-bg); color: var(--ds-text-primary); }
.icon-button { display: inline-flex; width: 22px; height: 22px; flex: 0 0 auto; align-items: center; justify-content: center; border: 0; border-radius: 2px; background: transparent; color: var(--ds-text-muted); cursor: pointer; }
.icon-button:hover:not(:disabled) { background: var(--ds-hover-bg); color: var(--ds-text-primary); }
.icon-button:disabled { cursor: not-allowed; opacity: .4; }
.inline-alert { font-size: 12px; }
.inspector :deep(.n-input), .inspector :deep(.n-base-selection) { --n-border-radius: 2px !important; }
</style>
