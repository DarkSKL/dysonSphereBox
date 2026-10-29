<script setup lang="ts">
import {computed, reactive, ref} from 'vue'
import {Copy, Download, FileInput, FileJson, FolderOpen, Pause, Play, RotateCcw, Save, Sparkles} from '@lucide/vue'
import BlueprintViewerPanel from '../../components/dysonSphere/blueprint/viewer-panel.vue'
import FlowCanvas from '../../components/dysonSphere/factory/flow-canvas.vue'
import InspectorPanel from '../../components/dysonSphere/factory/inspector-panel.vue'
import {useQuantWorkspace, type PreviewState} from '../../composables/dysonSphere/useQuantWorkspace'
import {buildFlowGraph} from '../../services/dysonSphere/factory-graph'

definePage({meta: {layout: 'main', title: '量化与蓝图', keepAlive: true}})
const w = reactive(useQuantWorkspace())
const fileInput = ref<HTMLInputElement | null>(null)
const viewer = ref<{resetCamera: () => void} | null>(null)
const previewLabels: Record<PreviewState, string> = {idle: '等待生成', loading: '正在加载预览模型', ready: '实时预览', stale: '等待刷新', error: '预览不可用', empty: '没有建筑'}
const previewStatus = computed(() => {
  if (w.generating) return {state: 'loading', text: w.generationProgress || '正在准备生成'}
  if (w.error?.source === 'generation') return {state: 'error', text: '蓝图生成失败'}
  if (w.code && !w.showPreview) return {state: 'idle', text: '预览已折叠'}
  return {state: w.previewState, text: previewLabels[w.previewState]}
})
// 缓存页可能保留旧生成结果：仅隐藏旧版通用验收提醒，供料、运力等具体告警继续展示。
const warningList = computed(() => [...new Set(w.warnings)].filter(warning => !(warning.startsWith('已检查静态') && warning.includes('仍需游戏内验证'))))
const hasOverflow = computed(() => w.mode === 6 && warningList.value.some(value => /超过|溢出|缺口|带宽|超出/.test(value)))
const onFile = async (event: Event): Promise<void> => {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (file) await w.importFile(file)
  input.value = ''
}

// =============== 选中节点 ===============
// 说明：画布与格式面板共享选中节点 ID；面板读取同一份图数据，保证数值与画布一致。
const selectedNode = ref<string | null>(null)
const selectedData = computed(() => {
  if (!selectedNode.value || !w.plan || w.replay) return null
  return buildFlowGraph(w.plan, w.itemName).nodes.find(node => node.id === selectedNode.value)?.data ?? null
})

// =============== 上下分栏 ===============
// 说明：拖动分隔条调整网格与预览高度，比例限制在 30%–80%；双击分隔条恢复默认。
const split = ref(58)
const host = ref<HTMLElement | null>(null)
const startResize = (event: PointerEvent): void => {
  const target = event.currentTarget as HTMLElement
  target.setPointerCapture(event.pointerId)
  const move = (next: PointerEvent): void => {
    const rect = host.value?.getBoundingClientRect()
    if (rect) split.value = Math.min(80, Math.max(30, (next.clientY - rect.top) / rect.height * 100))
  }
  const stop = (): void => { target.removeEventListener('pointermove', move); target.removeEventListener('pointerup', stop) }
  target.addEventListener('pointermove', move)
  target.addEventListener('pointerup', stop)
}
const resizeByKey = (event: KeyboardEvent): void => {
  if (event.key === 'ArrowUp') split.value = Math.max(30, split.value - 4)
  if (event.key === 'ArrowDown') split.value = Math.min(80, split.value + 4)
}
</script>

<template>
  <!-- ~ 量化与蓝图工作台 -->
  <!-- * 说明：上层为 draw.io 式网格节点图（物品库 · 画布 · 格式面板），下层为实时蓝图预览 -->
  <!-- * 交互：节点或面板中改参数 => 自动量化 => 自动生成并刷新预览；可暂停实时模式改为手动生成 -->
  <!-- * 依赖：useQuantWorkspace 统一编排 Go 量化、生成、导入与导出；预览复用唯一的 BlueprintViewerPanel -->
  <!-- * 边界：生成串行执行，过程中的修改只保留最新一次；预览超过 10000 建筑时折叠，不影响导出 -->
  <section ref="host" class="quant-workbench" :style="{'--split-ratio': split / 100}">
    <header class="bench-bar">
      <div class="bench-title"><h1>量化与蓝图</h1></div>
      <div class="bench-actions">
        <n-tooltip><template #trigger><n-button size="small" :type="w.autoRun ? 'primary' : 'default'" secondary @click="w.autoRun = !w.autoRun"><template #icon><Pause v-if="w.autoRun" :size="14" /><Play v-else :size="14" /></template>{{ w.autoRun ? '实时' : '已暂停' }}</n-button></template>{{ w.autoRun ? '修改参数后自动计算并生成预览，点击暂停' : '已暂停自动计算，点击恢复实时模式' }}</n-tooltip>
        <n-button v-if="!w.autoRun" size="small" :loading="w.solving" :disabled="w.loadingDataset || !w.detail || w.generating || w.importing" @click="w.calculate">计算</n-button>
        <span class="bar-divider" />
        <n-button size="small" quaternary aria-label="导入反算" :title="'导入反算'" @click="w.importVisible = true"><template #icon><FileInput :size="14" /></template>导入反算</n-button>
        <n-button size="small" quaternary aria-label="蓝图库" :title="'蓝图库'" @click="w.openLibrary"><template #icon><FolderOpen :size="14" /></template>蓝图库</n-button>
        <n-button size="small" quaternary aria-label="导出方案" :title="'导出方案'" :disabled="!w.plan || w.quantStale || w.solving || w.generating" :loading="w.saving" @click="w.exportWorkspace"><template #icon><FileJson :size="14" /></template>导出方案</n-button>
      </div>
    </header>
    <n-alert v-if="w.error && w.error.source !== 'generation'" type="error" closable class="bench-alert" @close="w.error = null">{{ w.error.message }} <span class="type-mono">{{ w.error.code }}</span></n-alert>

    <!-- ~ 上层：网格节点图 -->
    <div class="bench-graph">
      <FlowCanvas v-model:selected="selectedNode" :w="w" class="graph-canvas" />
      <InspectorPanel :w="w" :node="selectedData" @clear="selectedNode = null" />
    </div>

    <div class="bench-splitter" role="separator" aria-orientation="horizontal" aria-label="调整网格与预览高度" tabindex="0" :aria-valuenow="Math.round(split)" aria-valuemin="30" aria-valuemax="80" @pointerdown="startResize" @dblclick="split = 58" @keydown="resizeByKey"><span /></div>

    <!-- ~ 下层：实时蓝图预览 -->
    <section class="bench-preview" aria-label="蓝图预览">
      <header class="preview-bar">
        <span class="preview-state" :class="`is-${previewStatus.state}`" :title="previewStatus.text" role="status" aria-live="polite" aria-atomic="true"><i aria-hidden="true" /><span class="preview-state-text">{{ previewStatus.text }}</span></span>
        <span v-if="w.code" class="preview-meta type-display">{{ w.number(w.expectedBuildings) }} 建筑</span>
        <span v-if="w.generated?.surfacePlan" class="preview-meta surface-plan" :title="`首选纬圈格 ${w.generated.surfacePlan.selectedLatitudeGrid} · 平面占地 ${w.generated.surfacePlan.layoutWidth}×${w.generated.surfacePlan.layoutHeight} · 环向容量 ${w.generated.surfacePlan.ringGridCells} 格`">整块平面蓝图 · 粘贴锚点 {{ w.number(w.generated.surfacePlan.anchorLatitudeGrid * 360 / (w.generated.surfacePlan.gridSegment * 5)) }}°</span>
        <!-- 多目标分区：蓝图中各目标的节点流横向分开排布，色块与网格图一致，点击定位该分区首台机器 -->
        <div v-if="w.zones.length > 1 && !w.replay" class="preview-zones" role="group" aria-label="蓝图分区">
          <n-tooltip v-for="(zone, index) in w.zones" :key="zone.key"><template #trigger><button type="button" class="preview-zone" :class="{'is-active': w.activeZone === zone.key}" :style="{'--zone': zone.color}" :aria-pressed="w.activeZone === zone.key" @click="w.focusZone(zone.key)"><i />{{ zone.label }}</button></template>分区 {{ index + 1 }} · {{ zone.productionIndices.length }} 条产线{{ zone.key === 'shared' ? '，产物需分送到各目标分区' : '' }}</n-tooltip>
        </div>
        <!-- 蓝图名称：留空即按目标自动命名，输入后改为手动名称 -->
        <n-input v-model:value="w.customName" size="tiny" class="name-input" :placeholder="w.autoName" maxlength="60" clearable aria-label="蓝图名称（留空自动命名）" :title="w.nameIsAuto ? `自动命名：${w.autoName}` : '手动名称，清空恢复自动命名'" />
        <div class="preview-actions">
          <n-button v-if="!w.autoRun" size="tiny" type="primary" :loading="w.generating" :disabled="!w.plan || w.quantStale || w.solving || w.importing" @click="w.generate"><template #icon><Sparkles :size="13" /></template>生成</n-button>
          <n-tooltip><template #trigger><n-button size="tiny" quaternary :disabled="!w.code" aria-label="复制蓝图" @click="w.copy"><template #icon><Copy :size="14" /></template></n-button></template>复制蓝图编码</n-tooltip>
          <n-tooltip><template #trigger><n-button size="tiny" quaternary :disabled="!w.code" aria-label="导出文本" @click="w.download"><template #icon><Download :size="14" /></template></n-button></template>导出文本</n-tooltip>
          <n-tooltip><template #trigger><n-button size="tiny" quaternary :disabled="!w.code" :loading="w.saving" aria-label="存入蓝图库" @click="w.save"><template #icon><Save :size="14" /></template></n-button></template>存入蓝图库</n-tooltip>
          <n-tooltip><template #trigger><n-button size="tiny" quaternary :disabled="!w.code" aria-label="重置视角" @click="viewer?.resetCamera()"><template #icon><RotateCcw :size="14" /></template></n-button></template>重置视角</n-tooltip>
        </div>
      </header>
      <div class="preview-body">
        <div class="preview-stage">
          <BlueprintViewerPanel v-if="w.active && w.code && w.showPreview" ref="viewer" :code="w.code" :revision="w.previewRevision" interactive :selected-index="w.previewStale ? null : w.selectedBuilding" @ready="w.onPreviewReady" @select="w.onBuildingSelect" @error="w.onPreviewError" />
          <div v-else class="preview-empty">
            <Sparkles :size="22" />
            <p v-if="w.code && !w.showPreview">{{ w.previewNotice || '预览已折叠' }}</p>
            <p v-else-if="w.generating">{{ w.generationProgress || '正在准备生成' }}…</p>
            <p v-else>产线计算完成后会自动生成蓝图，并在这里实时预览。</p>
            <n-button v-if="w.code && !w.showPreview" size="tiny" @click="w.showPreview = true">仍然打开预览</n-button>
          </div>
          <div v-if="w.previewStale && w.code" class="preview-overlay">参数已改变，{{ w.autoRun ? '正在刷新预览…' : '点击「生成」刷新预览' }}</div>
        </div>
        <aside v-if="warningList.length || w.previewNotice || w.error?.source === 'generation'" class="preview-notes">
          <n-alert v-if="w.error?.source === 'generation'" title="蓝图生成失败" type="error" closable @close="w.error = null">{{ w.error.message }} <span class="type-mono">{{ w.error.code }}</span></n-alert>
          <p v-if="w.previewNotice && w.showPreview" class="note">{{ w.previewNotice }}</p>
          <div v-if="warningList.length" class="warning-list ds-bracket" :class="{'is-error': hasOverflow}"><div v-for="warning in warningList" :key="warning">{{ warning }}</div></div>
        </aside>
      </div>
    </section>

    <n-modal v-model:show="w.importVisible" preset="card" title="导入蓝图并反算" class="import-modal" style="width: min(680px, 92vw)"><p class="muted">只读识别建筑和理论产出，保留原始蓝图供预览与导出。</p><n-input v-model:value="w.importText" type="textarea" :rows="9" placeholder="粘贴 BLUEPRINT: 开头的蓝图串" class="type-mono" /><div class="import-actions"><input ref="fileInput" type="file" accept=".txt,.dspbp" hidden @change="onFile" /><n-button @click="fileInput?.click()">选择文本文件</n-button><n-button type="primary" :loading="w.importing" :disabled="!w.importText.trim()" @click="w.loadCode(w.importText, '导入的蓝图')">严格解析并预览</n-button></div><n-alert v-if="w.error" type="error">{{ w.error.message }}</n-alert></n-modal>
  </section>
</template>

<style scoped lang="scss">
// 命名区域：提示条按需出现时不会挤占其它行；网格与预览按分隔比例分配剩余高度。
.quant-workbench { display: grid; height: 100%; min-height: 600px; box-sizing: border-box; overflow: hidden; grid-template-areas: 'bar' 'alert' 'graph' 'split' 'preview'; grid-template-rows: auto auto minmax(240px, calc((100% - 120px) * var(--split-ratio))) 10px minmax(200px, 1fr); padding: 14px var(--ds-page-padding-x) 14px; color: var(--ds-text-primary); }
.bench-bar { position: relative; z-index: 2; grid-area: bar; }
.bench-alert { grid-area: alert; }
.bench-graph { grid-area: graph; }
.bench-splitter { grid-area: split; }
.bench-preview { grid-area: preview; }
.bench-bar { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 10px; }
.bench-title h1 { margin: 0; font-size: 19px; font-weight: 700; line-height: 1.3; }
.bench-actions { display: flex; min-width: 0; flex-wrap: wrap; justify-content: flex-end; align-items: center; gap: 4px; }
.bench-title { flex: 0 0 auto; }
.bar-divider { width: 1px; height: 18px; margin: 0 4px; background: var(--ds-border-default); }
.bench-alert { margin-bottom: 8px; }
// 两层面板共用切角外框，与图鉴等页面同一视觉语言。
.bench-graph, .bench-preview { --chamfer: 12px; position: relative; display: flex; min-height: 0; overflow: hidden; isolation: isolate; }
.bench-graph::before, .bench-preview::before { content: ''; position: absolute; inset: 0; z-index: -2; background: var(--ds-border-default); clip-path: polygon(0 0, calc(100% - var(--chamfer)) 0, 100% var(--chamfer), 100% 100%, var(--chamfer) 100%, 0 calc(100% - var(--chamfer))); }
.bench-graph::after, .bench-preview::after { content: ''; position: absolute; inset: 1px; z-index: -1; background: var(--ds-bg-panel); clip-path: polygon(0 0, calc(100% - var(--chamfer)) 0, 100% var(--chamfer), 100% 100%, var(--chamfer) 100%, 0 calc(100% - var(--chamfer))); }
.graph-canvas { min-width: 0; flex: 1; }
.bench-splitter { display: flex; align-items: center; justify-content: center; cursor: row-resize; touch-action: none; }
.bench-splitter span { width: 48px; height: 3px; border-radius: 2px; background: var(--ds-border-default); transition: background-color 150ms, width 150ms; }
.bench-splitter:hover span, .bench-splitter:focus-visible span { width: 72px; background: var(--ds-brand); }
.bench-splitter:focus-visible { outline: none; }
.bench-preview { flex-direction: column; }
.preview-bar { position: relative; display: flex; min-height: 38px; min-width: 0; align-items: center; gap: 10px; white-space: nowrap; border-bottom: 1px solid var(--ds-border-subtle); padding: 0 16px 0 14px; }
.preview-state { display: inline-flex; width: 240px; min-width: 100px; flex: 0 1 240px; align-items: center; gap: 6px; font-size: 13px; font-weight: 600; }
.preview-state-text { overflow: hidden; text-overflow: ellipsis; }
.preview-state i { width: 6px; height: 6px; flex: 0 0 6px; border-radius: 50%; background: var(--ds-text-muted); }
.preview-state.is-ready i { background: var(--ds-success); box-shadow: 0 0 6px var(--ds-success); }
.preview-state.is-loading i, .preview-state.is-stale i { background: var(--ds-brand); animation: pulse 1s ease-in-out infinite; }
.preview-state.is-error i { background: var(--ds-error); }
.preview-meta { color: var(--ds-text-muted); font-size: 12px; }
.surface-plan { color: var(--ds-warning); }
.surface-plan small { color: var(--ds-text-muted); font-size: 10px; }
.preview-zones { display: flex; min-width: 0; gap: 2px; overflow-x: auto; scrollbar-width: none; }
.preview-zone { --zone: var(--ds-brand); position: relative; display: inline-flex; height: 24px; flex: 0 0 auto; align-items: center; gap: 5px; border: 0; border-radius: 2px; padding: 0 8px; background: transparent; color: var(--ds-text-secondary); font-size: 12px; white-space: nowrap; cursor: pointer; }
.preview-zone i { width: 8px; height: 8px; border-radius: 1px; background: var(--zone); }
.preview-zone::after { content: ''; position: absolute; right: 8px; bottom: 0; left: 8px; height: 2px; background: var(--zone); transform: scaleX(0); transition: transform 150ms; }
.preview-zone:hover { background: var(--ds-hover-bg); color: var(--ds-text-primary); }
.preview-zone.is-active { color: var(--ds-text-primary); font-weight: 600; }
.preview-zone.is-active::after { transform: scaleX(1); }
.name-input { width: 180px; flex: 0 1 180px; min-width: 96px; margin-left: auto; --n-border-radius: 2px !important; --n-color: var(--ds-hover-bg) !important; }
.preview-actions { display: flex; flex: 0 0 auto; align-items: center; gap: 2px; }
.preview-body { display: flex; min-height: 0; flex: 1; }
.preview-stage { position: relative; min-width: 0; flex: 1; }
.preview-empty { display: flex; height: 100%; flex-direction: column; align-items: center; justify-content: center; gap: 8px; color: var(--ds-text-muted); font-size: 13px; }
.preview-empty p { margin: 0; }
.preview-overlay { position: absolute; top: 10px; left: 50%; border: 1px solid var(--ds-border-default); border-radius: 2px; padding: 3px 10px; background: var(--ds-bg-panel-elevated); color: var(--ds-text-secondary); font-size: 12px; transform: translateX(-50%); pointer-events: none; }
.preview-notes { display: flex; width: 280px; flex: 0 0 auto; flex-direction: column; gap: 8px; overflow-y: auto; border-left: 1px solid var(--ds-border-subtle); padding: 10px 12px; }
.note { margin: 0; color: var(--ds-text-secondary); font-size: 11px; line-height: 1.6; }
.warning-list { --bracket-color: var(--ds-warning-border); display: grid; gap: 4px; padding: 8px 10px 8px 12px; background: var(--ds-warning-bg); color: var(--ds-warning); font-size: 11px; line-height: 1.6; }
.warning-list.is-error { --bracket-color: var(--ds-error-border); background: var(--ds-error-bg); color: var(--ds-error); }
.muted { color: var(--ds-text-muted); font-size: 12px; line-height: 1.6; }
.import-actions { display: flex; justify-content: space-between; margin: 16px 0; }
@keyframes pulse { 50% { opacity: .35; } }
// 窄窗口：说明栏收起、按钮只留图标，分区标签保留色块并横向滚动。
@media (max-width: 1099px) { .preview-notes { display: none; } .bench-actions :deep(.n-button__content) { display: none; } .bench-actions :deep(.n-button__icon) { margin: 0; } .preview-meta { display: none; } }
@media (prefers-reduced-motion: reduce) { .preview-state i { animation: none !important; } .bench-splitter span { transition: none; } }
</style>
