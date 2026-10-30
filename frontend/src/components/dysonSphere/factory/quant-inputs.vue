<script setup lang="ts">
import {computed, ref} from 'vue'
import type {QuantWorkspace} from '../../../composables/dysonSphere/useQuantWorkspace'
import IconSelect from './icon-select.vue'

const props = defineProps<{w: QuantWorkspace}>()
const file = ref<HTMLInputElement | null>(null)
const busy = computed(() => props.w.loadingDataset || props.w.quantImporting || props.w.importing || props.w.saving)
const items = computed(() => props.w.detail?.items.filter(item => item.id > 0) ?? [])
const targetOptions = computed(() => items.value.filter(item => (props.w.detail?.recipes ?? []).some(recipe => recipe.results.includes(item.id))).map(item => ({label: item.name, value: item.name, icon: item.name})))
const optionsFor = (itemName: string) => {
  const item = items.value.find(value => value.name === itemName)
  return props.w.detail?.recipes.map((recipe, index) => ({recipe, index})).filter(({recipe}) => recipe.results.includes(item?.id ?? -1)) ?? []
}
const recipeFor = (line: typeof props.w.naturalProductionLine[number]) => optionsFor(line.targetItem)[line.recipeChoice - 1]?.recipe
const changeItem = (index: number, name: string) => props.w.setNaturalTarget(index, name)
const changeRecipe = (index: number, choice: number) => props.w.setNaturalRecipe(index, choice)
const changeBuilding = (index: number, building: number) => { props.w.naturalProductionLine[index].building = building }
</script>

<template>
  <!-- ~ 已有产线与量化文件 -->
  <!-- * 说明：维护仅用于物料平衡的固定产线，并保存/恢复纯量化输入 -->
  <!-- * 交互：无效产线阻止求解；文件恢复由工作区完成完整校验后原子应用 -->
  <!-- * 依赖：量化工作区与本地 JSON 文件，不调用蓝图导出接口 -->
  <!-- * 边界：这些既有建筑不会生成，功耗不计入新增制造功耗 -->
  <section class="quant-inputs" aria-label="已有产线与量化配置">
    <h3>已有产线</h3>
    <p class="hint">仅计入量化物料平衡，不生成这些已有建筑；已有产线功耗不是新增制造功耗。</p>
    <p v-if="!w.naturalProductionLine.length" class="hint">添加正在运行的设施，计算还需补建的产能。</p>
    <div v-for="(line, index) in w.naturalProductionLine" :key="index" class="line">
      <label>目标物品<IconSelect :value="line.targetItem" size="small" filterable :disabled="busy" :aria-label="`已有产线 ${index + 1} 目标物品`" :options="targetOptions" @update:value="value => changeItem(index, String(value))" /></label>
      <label>配方<IconSelect :value="line.recipeChoice" size="small" :disabled="busy" :aria-label="`已有产线 ${index + 1} 配方`" :options="optionsFor(line.targetItem).map(({recipe}, choiceIndex) => ({label: recipe.name, value: choiceIndex + 1, icon: w.itemName(recipe.results[0])}))" @update:value="value => changeRecipe(index, Number(value))" /></label>
      <label>设施<IconSelect :value="line.building" size="small" :disabled="busy" :aria-label="`已有产线 ${index + 1} 设施`" :options="(recipeFor(line)?.factories ?? []).map((id, buildingIndex) => ({label: w.itemName(id), value: buildingIndex, icon: w.itemName(id)}))" @update:value="value => changeBuilding(index, Number(value))" /></label>
      <label>机器数<n-input-number v-model:value="line.machineCount" size="small" :disabled="busy" :aria-label="`已有产线 ${index + 1} 机器数`" :min="0" :show-button="false" /></label>
      <label>模式<IconSelect v-model:value="line.productionMode" size="small" :disabled="busy" :aria-label="`已有产线 ${index + 1} 模式`" :options="w.sprayModes(recipeFor(line))" /></label>
      <label>点数<IconSelect v-model:value="line.increasePoints" size="small" :options="w.pointOptions" :aria-label="`已有产线 ${index + 1} 点数`" :disabled="busy || !line.productionMode" /></label>
      <n-button size="tiny" :disabled="busy" :aria-label="`移除已有产线 ${index + 1}`" @click="w.removeNaturalLine(index)">移除</n-button>
    </div>
    <n-button size="small" secondary :disabled="busy || !targetOptions.length" @click="w.addNaturalLine">添加已有产线</n-button>
    <p v-if="w.naturalLineError" class="error" role="alert">{{ w.naturalLineError }}</p>
    <h3>量化配置</h3>
    <input ref="file" class="file-input" type="file" accept="application/json,.json" aria-label="选择量化配置 JSON" @change="w.importQuantConfig(($event.target as HTMLInputElement).files?.[0] ?? null); if (file) file.value = ''" />
    <div class="actions"><n-button size="small" :loading="w.quantImporting" :disabled="busy || !w.detail" @click="file?.click()">{{ w.quantImporting ? '正在恢复' : '恢复量化配置' }}</n-button><n-button size="small" :disabled="busy || !w.detail || Boolean(w.naturalLineError)" @click="w.exportQuantConfig">保存量化配置</n-button></div>
    <p v-if="w.error?.code.startsWith('QCFG_')" class="error" role="alert">{{ w.error.message }}</p>
  </section>
</template>

<style scoped lang="scss">
.quant-inputs { display: grid; min-width: 0; gap: 7px; }
.quant-inputs h3 { margin: 8px 0 0; color: var(--ds-text-muted); font-size: 11px; }
.hint { margin: 0; color: var(--ds-text-muted); font-size: 10px; line-height: 1.45; }
.line { display: grid; min-width: 0; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 6px; border-bottom: 1px solid var(--ds-border-subtle); padding: 6px 0; }
label { display: grid; min-width: 0; gap: 3px; color: var(--ds-text-secondary); font-size: 10px; }
input { box-sizing: border-box; width: 100%; min-width: 0; border: 1px solid var(--ds-border-default); border-radius: 2px; padding: 4px; background: var(--ds-bg-panel-elevated); color: var(--ds-text-primary); font-size: 11px; }
.actions { display: flex; flex-wrap: wrap; gap: 6px; }
.file-input { display: none; }
.error { margin: 0; color: var(--ds-error); font-size: 11px; }
</style>
