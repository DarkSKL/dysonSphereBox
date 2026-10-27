<script setup lang="ts">
import {computed, reactive, ref} from 'vue'
import {Calculator, Copy, Download, FileInput, FolderOpen, PanelRight, Save, Sparkles} from '@lucide/vue'
import BlueprintViewerPanel from '../../components/dysonSphere/blueprint/viewer-panel.vue'
import {useQuantWorkspace, type PreviewState} from '../../composables/dysonSphere/useQuantWorkspace'

definePage({meta: {layout: 'main', title: '量化与蓝图', keepAlive: true}})
const w = reactive(useQuantWorkspace())
const fileInput = ref<HTMLInputElement | null>(null)
const viewer = ref<{resetCamera: () => void} | null>(null)
const modes = [{label: '1 · 机器直连带', value: 1}, {label: '2 · 物流配送器', value: 2}, {label: '3 · 行星内物流塔', value: 3}, {label: '4 · 星际物流塔', value: 4}, {label: '5 · 四向分流', value: 5}, {label: '6 · 混带总线', value: 6}]
const belts = [{label: '传送带', value: 1}, {label: '高速传送带', value: 2}, {label: '极速传送带', value: 3}]
const inserters = [{label: '分拣器', value: 1}, {label: '高速分拣器', value: 2}, {label: '极速分拣器', value: 3}, {label: '集装分拣器', value: 4}]
const previewLabels: Record<PreviewState, string> = {idle: '等待生成', loading: '正在加载', ready: '预览就绪', stale: '上次结果', error: '预览不可用', empty: '没有建筑'}
const warningList = computed(() => [...new Set(w.warnings)])
const hasOverflow = computed(() => w.mode === 6 && warningList.value.some(value => /超过|溢出|缺口|带宽|超出/.test(value)))
const importedRows = computed(() => {
  const rows = new Map<number, {recipeId: number; count: number; first: number; outputs: string}>()
  for (const machine of w.inspection?.machines ?? []) {
    const existing = rows.get(machine.recipeId)
    if (existing) existing.count++
    else rows.set(machine.recipeId, {recipeId: machine.recipeId, count: 1, first: machine.buildingIndex, outputs: machine.outputs.map(output => w.itemName(output.itemId)).join('、')})
  }
  return [...rows.values()]
})
const selectedImportRecipe = computed(() => w.inspection?.machines.find(machine => machine.buildingIndex === w.selectedBuilding)?.recipeId)
const number = (value: number | undefined): string => (value ?? 0).toLocaleString('zh-CN', {maximumFractionDigits: w.fixedNum ?? 2})
const onFile = async (event: Event): Promise<void> => {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (file) await w.importFile(file)
  input.value = ''
}
</script>

<template>
  <!-- ~ 本地量化工作区：输入和结果由 Go 引擎处理，页面只编辑请求、展示快照并复用唯一预览宿主。 -->
  <section class="quant-workspace">
    <header class="workspace-header">
      <div><div class="workspace-eyebrow">戴森球计划 / 生产规划</div><h1>量化计算器与蓝图</h1><p>从目标产量，计算原料与机器，再生成可导入游戏的蓝图。</p></div>
      <div class="header-actions">
        <n-button size="small" @click="w.importVisible = true"><template #icon><FileInput :size="15" /></template>导入反算</n-button>
        <n-button size="small" @click="w.openLibrary"><template #icon><FolderOpen :size="15" /></template>蓝图库</n-button>
        <n-button size="small" :type="w.showPreview ? 'primary' : 'default'" @click="w.showPreview = !w.showPreview"><template #icon><PanelRight :size="15" /></template>预览</n-button>
      </div>
    </header>
    <n-alert v-if="w.error && w.error.source !== 'generation'" type="error" closable class="notice" @close="w.error = null">{{ w.error.message }} <span class="type-mono">{{ w.error.code }}</span></n-alert>
    <div class="workspace-grid" :class="{'has-preview': w.showPreview && !w.narrow}">
      <main class="workspace-main">
        <section class="input-section">
          <div class="section-heading"><h2>目标产量</h2><n-select v-model:value="w.datasetKey" size="small" class="dataset-select" :options="w.datasets.map(item => ({label: item.name, value: item.key}))" :loading="w.loadingDataset" :disabled="w.solving || w.generating" /></div>
          <p v-if="w.detail" class="muted version-note">数据来自游戏 <span class="type-mono">v{{ w.detail.info.gameVersion }}</span>，请与实际游戏及 Mod 版本核对。</p>
          <div v-for="(target, index) in w.targets" :key="target.key" class="target-row">
            <n-select v-model:value="target.itemId" filterable :options="w.itemOptions" placeholder="选择目标产物" class="target-item" />
            <n-input-number v-model:value="target.ratePerMinute" :min="0.000001" :max="1000000000" :show-button="false" class="target-rate"><template #suffix>/min</template></n-input-number>
            <n-tooltip><template #trigger><n-button size="small" :disabled="!target.itemId" @click="w.setExternal(target.itemId)">外供</n-button></template>将该物品设为外部供应，不展开生产配方</n-tooltip>
            <n-button size="small" quaternary :disabled="w.targets.length === 1" :aria-label="`移除目标 ${index + 1}`" @click="w.targets.splice(index, 1)">移除</n-button>
          </div>
          <div class="target-footer"><n-button size="small" text :disabled="w.targets.length >= 100" @click="w.addTarget">＋ 添加目标</n-button><n-button type="primary" :loading="w.solving" :disabled="w.loadingDataset || !w.detail || w.generating || w.importing" @click="w.calculate"><template #icon><Calculator :size="16" /></template>计算产线</n-button></div>
          <n-collapse class="advanced-section">
            <n-collapse-item title="计算与布局参数" name="settings">
              <div class="settings-grid">
                <label>采矿倍率<n-input-number v-model:value="w.miningSpeed" :min="0.01" :step="0.1" /></label>
                <label>显示小数位<n-input-number v-model:value="w.fixedNum" :min="0" :max="8" :precision="0" /></label>
                <label>研究站堆叠<n-input-number v-model:value="w.researchStack" :min="1" :max="15" /></label>
                <label>占地成本权重<n-input-number v-model:value="w.spaceCost" :min="0" /></label>
                <label>电力成本权重<n-input-number v-model:value="w.powerCost" :min="0" /></label>
                <label>单列机器上限<n-input-number v-model:value="w.layout.maxMachineInALine" :min="1" :max="200" /></label>
                <label>传送带<n-select v-model:value="w.layout.beltLv" :options="belts" /></label>
                <label>分拣器<n-select v-model:value="w.layout.inserterLv" :options="inserters" /></label>
                <label>蓝图研究站层数<n-input-number v-model:value="w.layout.maxLays" :min="1" :max="15" /></label>
                <label>街区宽度<n-input-number v-model:value="w.layout.blockWidthLimit" :min="10" :max="500" /></label>
                <label>排列方式<n-select v-model:value="w.layout.mergeType" :options="[{label: '纵向单列', value: 1}, {label: '按宽度换列', value: 2}]" /></label>
                <div class="settings-flags"><n-checkbox v-model:checked="w.layout.needCoaterNet">连接喷涂供剂网络</n-checkbox><n-checkbox v-model:checked="w.sprayItself">增产剂自喷涂</n-checkbox><n-checkbox v-model:checked="w.layout.needPower">生成供电设施</n-checkbox><n-checkbox v-model:checked="w.layout.useMixed">混带输入（模式 1/3/4/6）</n-checkbox><n-checkbox v-model:checked="w.layout.mix2" :disabled="!w.layout.useMixed">混带 2.0 补料</n-checkbox></div>
              </div>
              <label class="external-label">外部供应物品<n-select v-model:value="w.mineralized" multiple filterable clearable :options="w.itemOptions.map(item => ({label: item.label, value: item.label}))" placeholder="选择不展开生产的原料或物品" /></label>
            </n-collapse-item>
          </n-collapse>
        </section>

        <!-- ~ 结果快照：配方和喷涂改变只标记过期，重新计算前不冒充当前产线。 -->
        <section v-if="w.plan && !w.replay" class="result-section">
          <div class="section-heading"><h2>量化结果</h2><n-button size="tiny" :disabled="w.quantStale || w.solving || w.generating" :loading="w.saving" @click="w.exportWorkspace">导出方案 JSON</n-button><span v-if="w.quantStale" class="state-warning">参数已改变，请重新计算</span><span v-else class="muted">点击产线可定位首台机器</span></div>
          <div class="result-metrics"><span><strong class="type-display">{{ number(w.plan.machineCount) }}</strong> 台制造设备</span><span><strong class="type-display">{{ number(w.plan.powerMw) }}</strong> MW 制造工作功耗</span><span class="type-mono">{{ w.plan.dataset }}</span></div>
          <div class="result-table-wrap"><table class="result-table"><thead><tr><th>产物 / 配方</th><th>设备</th><th>喷涂策略</th><th class="numeric">产出 /min</th><th class="numeric">精确台数</th><th class="numeric">取整台数</th><th class="numeric">余量</th></tr></thead><tbody>
            <tr v-for="(production, index) in w.plan.productions" :key="`${production.mainItemId}-${production.recipeId}`" :class="{selected: w.selectedProductions.includes(index)}" tabindex="0" :aria-selected="w.selectedProductions.includes(index)" @click="w.selectProduction(index)" @keydown.enter="w.selectProduction(index)">
              <td><strong>{{ w.itemName(production.mainItemId) }}</strong><n-select size="tiny" :value="w.itemRecipeChoices[w.itemName(production.mainItemId)] ?? 1" :options="w.recipeOptions(production.mainItemId)" @update:value="value => w.updateRecipe(production.mainItemId, value)" @click.stop /></td>
              <td><n-select size="tiny" :value="w.overrideFor(w.selectedRecipe(production.mainItemId)).building" :options="w.buildingOptions(w.selectedRecipe(production.mainItemId))" @update:value="value => w.updateOverride(w.selectedRecipe(production.mainItemId), {building: value})" @click.stop /></td>
              <td><n-select size="tiny" :value="w.overrideFor(w.selectedRecipe(production.mainItemId)).productionMode" :options="w.sprayModes(w.selectedRecipe(production.mainItemId))" @update:value="value => w.updateOverride(w.selectedRecipe(production.mainItemId), {productionMode: value})" @click.stop /><n-select size="tiny" :value="w.overrideFor(w.selectedRecipe(production.mainItemId)).increasePoints" :options="w.pointOptions" @update:value="value => w.updateOverride(w.selectedRecipe(production.mainItemId), {increasePoints: value})" @click.stop /></td>
              <td class="numeric type-display">{{ number(production.outputCount) }}</td><td class="numeric type-display">{{ number(production.exactMachineCount) }}</td><td class="numeric type-display">{{ production.machineCount }}</td><td class="numeric type-display">{{ number(production.roundingMargin) }}</td>
            </tr>
          </tbody></table></div>
          <div v-if="!w.plan.productions.length" class="empty-section">目标全部作为外部供应，没有需要生成的制造设备。</div>
          <div class="material-grid">
            <div><h3>原矿与外部供给</h3><div v-for="item in w.plan.rawInputs" :key="item.itemId" class="material-row"><span>{{ item.name }}</span><span class="type-display">{{ number(item.ratePerMinute) }} /min</span></div></div>
            <div><h3>剩余副产物</h3><div v-for="item in w.plan.leftovers" :key="item.itemId" class="material-row"><span>{{ item.name }}</span><span class="type-display">{{ number(item.ratePerMinute) }} /min</span></div><p v-if="!w.plan.leftovers.length" class="muted">当前理论平衡没有剩余副产物</p></div>
          </div>
        </section>

        <!-- ~ 导入回放：独立展示 Inspect 结果，不构造伪量化方案。 -->
        <section v-if="w.replay && w.inspection" class="result-section">
          <div class="section-heading"><h2>{{ w.sourceName }} · 只读回放</h2><n-button size="small" @click="w.recalculateImport">转为量化目标</n-button></div>
          <p class="muted">共 {{ w.inspection.summary.buildingCount }} 个建筑，{{ w.inspection.machines.length }} 台可识别制造设备。点击行可选中建筑。</p>
          <div class="result-table-wrap"><table class="result-table inspection-table"><thead><tr><th>配方</th><th>产物</th><th>机器数</th></tr></thead><tbody><tr v-for="row in importedRows" :key="row.recipeId" tabindex="0" :class="{selected: selectedImportRecipe === row.recipeId}" @click="w.selectedBuilding = row.first" @keydown.enter="w.selectedBuilding = row.first"><td class="type-mono">{{ row.recipeId }}</td><td>{{ row.outputs }}</td><td class="type-display">{{ row.count }}</td></tr></tbody></table></div>
          <h3>未喷涂理论产出</h3><div class="material-grid"><div v-for="output in w.inspection.outputs" :key="output.itemId" class="material-row"><span>{{ output.name }}</span><span class="type-display">{{ number(output.ratePerMinute) }} /min</span></div></div>
          <n-alert v-if="w.inspection.unknownIds.length" type="warning">当前数据集无法识别的原型：{{ w.inspection.unknownIds.join('、') }}</n-alert>
        </section>

        <section v-if="w.plan || w.code" class="generation-section">
          <div class="section-heading"><h2>蓝图生成与导出</h2><span v-if="w.previewStale" class="state-warning">预览为上次结果</span></div>
          <div class="generation-inputs"><n-input v-model:value="w.name" placeholder="蓝图名称" maxlength="100" /><n-select v-model:value="w.mode" :options="modes" /></div>
          <div class="generation-actions"><n-button type="primary" :loading="w.generating" :disabled="!w.plan || w.quantStale || w.solving || w.importing || (w.showPreview && Boolean(w.code) && w.previewState === 'loading')" @click="w.generate"><template #icon><Sparkles :size="15" /></template>生成蓝图</n-button><n-button :disabled="!w.code" @click="w.copy"><template #icon><Copy :size="15" /></template>复制</n-button><n-button :disabled="!w.code" @click="w.download"><template #icon><Download :size="15" /></template>导出文本</n-button><n-button :disabled="!w.code" :loading="w.saving" @click="w.save"><template #icon><Save :size="15" /></template>存入蓝图库</n-button></div>
          <!-- ~ 生成失败详情：紧随操作按钮持久展示，关闭通知弹窗后仍可在原处查看和重试。 -->
          <n-alert v-if="w.error?.source === 'generation'" title="蓝图生成失败" type="error" closable class="generation-error" @close="w.error = null">{{ w.error.message }} <span class="type-mono">{{ w.error.code }}</span></n-alert>
          <p v-if="w.code" class="muted">{{ number(w.expectedBuildings) }} 个建筑 · 已通过 Go 编码与结构自检。游戏内实际加载及运行需另行确认。</p>
          <n-alert v-if="warningList.length" :type="hasOverflow ? 'error' : 'warning'" class="warning-list"><div v-for="warning in warningList" :key="warning">{{ warning }}</div></n-alert>
        </section>
        <div v-if="!w.plan && !w.replay && !w.code" class="empty-section"><Calculator :size="26" /><p>选择目标产物并计算，查看配方、机器、原料与电力。</p><span class="muted">量化与蓝图处理均在本机完成。</span></div>
      </main>

      <!-- ~ 宽窗口预览；窄窗口用抽屉，二者互斥且失活时释放 WebGL。 -->
      <aside v-if="w.showPreview && !w.narrow" class="preview-section"><div class="section-heading"><h2>模型预览</h2><span class="muted">{{ previewLabels[w.previewState] }}</span><n-button size="tiny" :disabled="!w.code" @click="viewer?.resetCamera()">重置视角</n-button></div><n-alert v-if="w.previewStale" type="warning">预览属于上次参数，重新生成后可定位。导出上次结果需要确认。</n-alert><p v-if="w.previewNotice" class="preview-notice">{{ w.previewNotice }}</p><div class="preview-stage"><BlueprintViewerPanel v-if="w.active && w.code" ref="viewer" :code="w.code" :revision="w.previewRevision" interactive :selected-index="w.previewStale ? null : w.selectedBuilding" @ready="w.onPreviewReady" @select="w.onBuildingSelect" @error="w.onPreviewError" /><div v-else class="empty-section">生成或导入蓝图后，可在这里查看模型。</div></div></aside>
    </div>
    <n-drawer :show="w.narrow && w.showPreview && Boolean(w.code)" width="92vw" placement="right" :trap-focus="false" @update:show="value => w.showPreview = value"><n-drawer-content title="蓝图预览" closable body-content-style="padding: 0; height: 100%;"><div class="drawer-preview"><n-alert v-if="w.previewStale" type="warning">预览为上次结果，重新生成后可定位。</n-alert><p v-if="w.previewNotice" class="preview-notice">{{ w.previewNotice }}</p><BlueprintViewerPanel v-if="w.active && w.narrow && w.showPreview && w.code" ref="viewer" :code="w.code" :revision="w.previewRevision" interactive :selected-index="w.previewStale ? null : w.selectedBuilding" @ready="w.onPreviewReady" @select="w.onBuildingSelect" @error="w.onPreviewError" /></div></n-drawer-content></n-drawer>
    <n-modal v-model:show="w.importVisible" preset="card" title="导入蓝图并反算" class="import-modal" style="width: min(680px, 92vw)"><p class="muted">只读识别建筑和理论产出，保留原始蓝图供预览与导出。</p><n-input v-model:value="w.importText" type="textarea" :rows="9" placeholder="粘贴 BLUEPRINT: 开头的蓝图串" class="type-mono" /><div class="import-actions"><input ref="fileInput" type="file" accept=".txt,.dspbp" hidden @change="onFile" /><n-button @click="fileInput?.click()">选择文本文件</n-button><n-button type="primary" :loading="w.importing" :disabled="!w.importText.trim()" @click="w.loadCode(w.importText, '导入的蓝图')">严格解析并预览</n-button></div><n-alert v-if="w.error" type="error">{{ w.error.message }}</n-alert></n-modal>
  </section>
</template>

<style scoped lang="scss">
.quant-workspace { padding: var(--ds-page-padding-top) var(--ds-page-padding-x) var(--ds-page-padding-bottom); color: var(--ds-text-primary); min-width: 0; }
.workspace-header { display: flex; justify-content: space-between; align-items: flex-start; gap: 20px; margin-bottom: 24px; h1 { margin: 5px 0 7px; font-size: 23px; font-weight: 700; } p { margin: 0; color: var(--ds-text-secondary); font-size: 13px; } }
.workspace-eyebrow { color: var(--ds-text-muted); font-size: 12px; }
.header-actions, .generation-actions, .import-actions { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
.workspace-grid { display: grid; grid-template-columns: minmax(0, 1fr); gap: 24px; &.has-preview { grid-template-columns: minmax(0, 1.15fr) minmax(340px, .85fr); } }
.workspace-main { min-width: 0; }
.section-heading { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; margin-bottom: 12px; h2 { margin: 0; margin-right: auto; font-size: 15px; font-weight: 700; } }
h3 { margin: 14px 0 8px; font-size: 13px; font-weight: 700; }
.dataset-select { width: min(240px, 100%); }
.muted { color: var(--ds-text-muted); font-size: 12px; line-height: 1.6; }
.version-note { margin-bottom: 14px; }
.target-row { display: flex; align-items: center; gap: 8px; margin: 10px 0; .target-item { flex: 1; min-width: 160px; } .target-rate { width: 140px; } }
.target-footer { display: flex; justify-content: space-between; align-items: center; margin: 14px 0; }
.advanced-section { border-top: 1px solid var(--ds-border-subtle); padding-top: 12px; }
.settings-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; label { display: flex; flex-direction: column; gap: 5px; color: var(--ds-text-secondary); font-size: 12px; } }
.settings-flags { grid-column: 1 / -1; display: flex; flex-wrap: wrap; gap: 12px; }
.external-label { display: grid; gap: 6px; margin-top: 16px; color: var(--ds-text-secondary); font-size: 12px; }
.result-section, .generation-section { border-top: 1px solid var(--ds-border-default); margin-top: 24px; padding-top: 20px; }
.result-metrics { display: flex; align-items: baseline; flex-wrap: wrap; gap: 20px; margin: 14px 0; color: var(--ds-text-secondary); font-size: 12px; strong { font-size: 23px; color: var(--ds-text-primary); margin-right: 4px; } }
.result-table-wrap { max-width: 100%; overflow: auto; border-top: 1px solid var(--ds-border-subtle); border-bottom: 1px solid var(--ds-border-subtle); }
.result-table { width: 100%; min-width: 820px; border-collapse: collapse; text-align: left; font-size: 12px; th { padding: 10px 8px; color: var(--ds-text-muted); font-weight: 400; white-space: nowrap; } td { padding: 10px 8px; border-top: 1px solid var(--ds-border-subtle); vertical-align: middle; } td:first-child { min-width: 180px; strong { display: block; margin-bottom: 6px; } } td:nth-child(2) { min-width: 140px; } td:nth-child(3) { min-width: 115px; :deep(.n-select + .n-select) { margin-top: 5px; } } .numeric { text-align: right; white-space: nowrap; } tbody tr { cursor: pointer; &:hover { background: var(--ds-hover-bg); } &.selected { background: var(--ds-active-bg); outline: 1px solid var(--ds-brand); outline-offset: -1px; } } }
.inspection-table { min-width: 350px; }
.material-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px 24px; }
.material-row { display: flex; align-items: baseline; justify-content: space-between; gap: 12px; padding: 4px 0; font-size: 12px; color: var(--ds-text-secondary); }
.state-warning { color: var(--ds-warning); font-size: 12px; }
.generation-inputs { display: grid; grid-template-columns: 1fr 1.1fr; gap: 10px; margin-bottom: 14px; }
.generation-error { margin-top: 14px; overflow-wrap: anywhere; }
.notice { margin-bottom: 16px; }
.warning-list { margin-top: 14px; max-height: 180px; overflow-y: auto; line-height: 1.7; font-size: 12px; }
.preview-section { position: sticky; top: 16px; align-self: start; min-width: 0; border-left: 1px solid var(--ds-border-default); padding-left: 20px; }
.preview-stage { height: clamp(430px, 66vh, 760px); min-height: 430px; border: 1px solid var(--ds-border-subtle); background: var(--ds-bg-panel); }
.preview-notice { color: var(--ds-warning); padding: 8px; font-size: 12px; line-height: 1.5; }
.empty-section { display: flex; min-height: 160px; padding: 24px; flex-direction: column; justify-content: center; align-items: center; gap: 8px; text-align: center; color: var(--ds-text-muted); font-size: 13px; }
.drawer-preview { display: flex; flex-direction: column; height: 100%; :deep(.blueprint-viewer-panel) { flex: 1; } }
.import-actions { justify-content: space-between; margin: 16px 0; }
@media (max-width: 1099px) { .workspace-header { flex-direction: column; } .settings-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
@media (max-width: 700px) { .target-row { flex-wrap: wrap; } .settings-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } .material-grid, .generation-inputs { grid-template-columns: 1fr; } }
</style>
