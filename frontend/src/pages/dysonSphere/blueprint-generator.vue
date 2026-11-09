<script setup lang="ts">
import {computed, reactive, ref, watch} from 'vue'
import {Copy, Download, FileInput, FileJson, FolderOpen, Pause, Play, RotateCcw, Save, Sparkles} from '@lucide/vue'
import BlueprintViewerPanel from '../../components/dysonSphere/blueprint/viewer-panel.vue'
import AlgorithmHeatmap from '../../components/dysonSphere/factory/algorithm-heatmap.vue'
import AlgorithmStage from '../../components/dysonSphere/factory/algorithm-stage.vue'
import FlowCanvas from '../../components/dysonSphere/factory/flow-canvas.vue'
import InspectorPanel from '../../components/dysonSphere/factory/inspector-panel.vue'
import EventTimeline from '../../components/dysonSphere/factory/event-timeline.vue'
import {useQuantWorkspace, type FactoryPreviewApi, type PreviewState} from '../../composables/dysonSphere/useQuantWorkspace'
import {generationDebugOptions} from '../../composables/dysonSphere/useGenerationTimeline'
import {buildFlowGraph} from '../services/dysonSphere/factory-graph'

// 说明：BlueprintViewerPanel defineExpose 暴露的完整方法签名；页面只需要 resetCamera 和预览四动作，其余为类型完整性。
interface ViewerPanelExpose {
  resetCamera(): void
  selectBuildings(indices: number[]): void
  focusBuilding(index: number | null): void
  captureImage(): string | null
  setRenderSettings(showRealModels: boolean, maxInstancesPerModel: number): void
  beginPreview(): void
  applyPreviewDelta(event: unknown): boolean
  commitPreview(blueprint: string): unknown
  discardPreview(): void
  isReady(): boolean
  getPreviewing(): boolean
  getPreviewStats(): unknown | null
}

definePage({meta: {layout: 'main', title: '量化与蓝图', keepAlive: true}})
const w = reactive(useQuantWorkspace())
const fileInput = ref<HTMLInputElement | null>(null)
const viewer = ref<ViewerPanelExpose | null>(null)
// =============== 加载顺序 ===============
// 说明：预览器先挂载并注入空白蓝图渲染星球主体，流程图等预览器出画面（ready 事件）后再挂载。
// 边界：空白蓝图只用于提前渲染与释放门控，不写入工作区的预览统计；渲染失败或存在错误时兜底放行。
const previewerRendered = ref(false)
const onPreviewerReady = (value: {buildingCount: number; blank?: boolean}): void => {
  previewerRendered.value = true
  if (!value.blank) w.onPreviewReady(value)
}
const flowGraphReady = computed(() => previewerRendered.value || !w.showPreview || w.previewState === 'error' || Boolean(w.error))
// =============== 预览协调 ===============
// 说明：把 Viewer 暴露的 beginPreview/applyPreviewDelta/commitPreview/discardPreview 注册给 workspace，
// workspace 在生成时自动编排四步生命周期（begin → 提交生成 → 边收边 apply → commit / discard）。
// 边界：viewer 未挂载时 workspace 自动走旧的静态预览链路，不阻塞生成。
watch(viewer, (panel) => {
  if (panel) {
    const api: FactoryPreviewApi = {
      beginPreview: () => panel.beginPreview(),
      applyPreviewDelta: (event) => panel.applyPreviewDelta(event),
      commitPreview: (blueprint) => panel.commitPreview(blueprint),
      discardPreview: () => panel.discardPreview(),
      isReady: () => panel.isReady(),
    }
    w.registerPreviewApi(api)
  } else {
    w.registerPreviewApi(undefined)
  }
}, {immediate: true})
const previewLabels: Record<PreviewState, string> = {idle: '等待生成', loading: '正在加载预览模型', ready: '实时预览', stale: '等待刷新', error: '预览不可用', empty: '没有建筑'}
const previewStatus = computed(() => {
  if (w.generating) return {state: 'loading', text: w.generationProgress || '正在准备生成'}
  if (w.error?.source === 'generation') return {state: 'error', text: '蓝图生成失败'}
  if (w.previewStale) return {state: 'stale', text: w.generationFailed ? '显示上次成功结果' : '等待刷新'}
  if (w.code && !w.showPreview) return {state: 'idle', text: '预览已折叠'}
  return {state: w.previewState, text: previewLabels[w.previewState]}
})
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

// =============== 预览标签页 ===============
// 说明：下层卡片在「蓝图预览」与「算法可视化」间切换；切走时预览只隐藏不卸载，避免重建 WebGL 视图。
// 算法可视化在首次切换时挂载，之后同样只隐藏，渲染循环随容器不可见自动暂停。
type PreviewTab = 'preview' | 'algorithm'
const previewTabs: {key: PreviewTab, label: string}[] = [{key: 'preview', label: '蓝图预览'}, {key: 'algorithm', label: '算法可视化'}]
const previewTab = ref<PreviewTab>('preview')
const algorithmMounted = ref(false)
watch(previewTab, tab => { if (tab === 'algorithm') algorithmMounted.value = true })

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
  <section ref="host" class="quant-workbench" :class="{'is-debug': generationDebugOptions.debugMode}" :style="{'--split-ratio': split / 100}">
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
    <n-alert v-if="w.error" type="error" closable class="bench-alert" @close="w.error = null">{{ w.error.message }} <span class="type-mono">{{ w.error.code }}</span></n-alert>

    <!-- ~ 上层：网格节点图 -->
    <!-- * 加载顺序：等下方预览器真正渲染出画面后再挂载，确保预览器先于流程图完成加载 -->
    <div class="bench-graph">
      <template v-if="flowGraphReady">
        <FlowCanvas v-model:selected="selectedNode" :w="w" class="graph-canvas" />
        <InspectorPanel :w="w" :node="selectedData" @clear="selectedNode = null" />
      </template>
      <div v-else class="graph-boot">正在等待蓝图预览渲染…</div>
    </div>

    <div class="bench-splitter" role="separator" aria-orientation="horizontal" aria-label="调整网格与预览高度" tabindex="0" :aria-valuenow="Math.round(split)" aria-valuemin="30" aria-valuemax="80" @pointerdown="startResize" @dblclick="split = 58" @keydown="resizeByKey"><span /></div>

    <!-- ~ 下层：实时蓝图预览 -->
    <section class="bench-preview" aria-label="蓝图预览">
      <header class="preview-bar">
        <div class="preview-tabs" role="tablist" aria-label="预览视图">
          <button v-for="tab in previewTabs" :id="`preview-tab-${tab.key}`" :key="tab.key" type="button" role="tab" class="preview-tab" :class="{'is-active': previewTab === tab.key}" :aria-selected="previewTab === tab.key" :aria-controls="`preview-panel-${tab.key}`" @click="previewTab = tab.key">{{ tab.label }}</button>
        </div>
        <span class="bar-divider" aria-hidden="true" />
        <!-- 算法活动热力图：仅在「算法可视化」下显示，占满标签右侧到操作区之间的空间 -->
        <AlgorithmHeatmap v-if="previewTab === 'algorithm'" />
        <!-- 多目标归属只用于查看与聚焦，真实上下游路线由统一流图决定，不要求用户手工跨区分送。 -->
        <div v-if="previewTab === 'preview' && w.zones.length > 1 && !w.replay" class="preview-zones" role="group" aria-label="蓝图分区">
          <n-tooltip v-for="(zone, index) in w.zones" :key="zone.key"><template #trigger><button type="button" class="preview-zone" :class="{'is-active': w.activeZone === zone.key}" :style="{'--zone': zone.color}" :aria-pressed="w.activeZone === zone.key" @click="w.focusZone(zone.key)"><i />{{ zone.label }}</button></template>目标归属 {{ index + 1 }} · {{ zone.productionIndices.length }} 条产线{{ zone.key === 'shared' ? '，多个目标共享同一份上游产线' : '' }}</n-tooltip>
        </div>
        <!-- 蓝图名称：留空即按目标自动命名，输入后改为手动名称 -->
        <n-input v-if="previewTab === 'preview'" v-model:value="w.customName" size="tiny" class="name-input" :placeholder="w.autoName" maxlength="60" clearable aria-label="蓝图名称（留空自动命名）" :title="w.nameIsAuto ? `自动命名：${w.autoName}` : '手动名称，清空恢复自动命名'" />
        <div v-if="previewTab === 'preview'" class="preview-actions">
          <n-button v-if="!w.autoRun" size="tiny" type="primary" :loading="w.generating" :disabled="!w.plan || w.quantStale || w.solving || w.importing" @click="w.generate"><template #icon><Sparkles :size="13" /></template>生成</n-button>
          <n-tooltip><template #trigger><n-button size="tiny" quaternary :disabled="!w.code" aria-label="复制蓝图" @click="w.copy"><template #icon><Copy :size="14" /></template></n-button></template>复制蓝图编码</n-tooltip>
          <n-tooltip><template #trigger><n-button size="tiny" quaternary :disabled="!w.code" aria-label="导出文本" @click="w.download"><template #icon><Download :size="14" /></template></n-button></template>导出文本</n-tooltip>
          <n-tooltip><template #trigger><n-button size="tiny" quaternary :disabled="!w.code" :loading="w.saving" aria-label="存入蓝图库" @click="w.save"><template #icon><Save :size="14" /></template></n-button></template>存入蓝图库</n-tooltip>
          <n-tooltip><template #trigger><n-button size="tiny" quaternary :disabled="!w.code" aria-label="重置视角" @click="viewer?.resetCamera()"><template #icon><RotateCcw :size="14" /></template></n-button></template>重置视角</n-tooltip>
        </div>
        <div v-if="w.generating" class="preview-progress" role="progressbar" aria-label="蓝图生成进度" :aria-valuetext="w.generationProgress || '正在准备生成'" />
      </header>
      <div class="preview-body">
        <div class="preview-stage">
          <!-- * 加载顺序：预览器随页面激活即挂载并预加载 ESM（不再等 code），空态由下方覆盖层呈现 -->
          <BlueprintViewerPanel v-if="w.active && w.showPreview" ref="viewer" :code="w.code" :revision="w.previewRevision" interactive :selected-buildings="w.previewStale ? [] : w.selectedBuildings" :focus-index="w.focusIndex" :focus-revision="w.focusRevision" @ready="onPreviewerReady" @select="w.onBuildingSelect" @error="w.onPreviewError" />
          <div v-if="!w.code || !w.showPreview" class="preview-empty">
            <Sparkles :size="22" />
            <p v-if="w.code && !w.showPreview">{{ w.previewNotice || '预览已折叠' }}</p>
            <p v-else-if="w.generating">{{ w.generationProgress || '正在准备生成' }}…</p>
            <p v-else>产线计算完成后会自动生成蓝图，并在这里实时预览。</p>
            <n-button v-if="w.code && !w.showPreview" size="tiny" @click="w.showPreview = true">仍然打开预览</n-button>
          </div>
          <div v-if="w.previewStale && w.code" class="preview-overlay">{{ w.generationFailed ? '最近一次生成未成功，当前显示上次成功结果' : w.generating ? '正在生成，当前显示上次成功结果' : `参数已改变，${w.autoRun ? '正在刷新预览…' : '点击「生成」刷新预览'}` }}</div>
        </div>
        <!-- 算法可视化：首次切换到该标签时才创建场景，之后保持挂载以保留舞台与播放进度 -->
        <div v-if="algorithmMounted" v-show="previewTab === 'algorithm'" id="preview-panel-algorithm" class="preview-algorithm" role="tabpanel" aria-labelledby="preview-tab-algorithm"><AlgorithmStage :w="w" :active="previewTab === 'algorithm'" /></div>
      </div>
    </section>

    <!-- ~ 最右侧：生成事件时间流（调试） -->
    <!-- * 说明：纵跨网格与预览两层，实时列出蓝图生成事件卡片，点击查看原始消息 -->
    <!-- * 边界：只旁路观察事件，不影响生成、预览与任务过滤；仅在检查面板开启调试模式时挂载，关闭后记录随之清空 -->
    <div v-if="generationDebugOptions.debugMode" class="bench-timeline"><EventTimeline :can-generate="Boolean(w.plan && !w.quantStale && !w.solving && !w.importing)" :generating="w.generating" @generate="w.generate" /></div>

    <n-modal v-model:show="w.importVisible" preset="card" title="导入蓝图并反算" class="import-modal" style="width: min(680px, 92vw)"><p class="muted">只读识别建筑和理论产出，保留原始蓝图供预览与导出。</p><n-input v-model:value="w.importText" type="textarea" :rows="9" placeholder="粘贴 BLUEPRINT: 开头的蓝图串" class="type-mono" /><div class="import-actions"><input ref="fileInput" type="file" accept=".txt,.dspbp" hidden @change="onFile" /><n-button @click="fileInput?.click()">选择文本文件</n-button><n-button type="primary" :loading="w.importing" :disabled="!w.importText.trim()" @click="w.loadCode(w.importText, '导入的蓝图')">严格解析并预览</n-button></div><n-alert v-if="w.error" type="error">{{ w.error.message }}</n-alert></n-modal>
  </section>
</template>

<style scoped lang="scss">
// 命名区域：提示条按需出现时不会挤占其它行；网格与预览按分隔比例分配剩余高度。
.quant-workbench { display: grid; height: 100%; min-height: 600px; box-sizing: border-box; overflow: hidden; grid-template-areas: 'bar bar' 'alert alert' 'graph timeline' 'split timeline' 'preview timeline'; grid-template-columns: minmax(0, 1fr) auto; column-gap: 10px; grid-template-rows: auto auto minmax(240px, calc((100% - 120px) * var(--split-ratio))) 10px minmax(200px, 1fr); padding: 14px var(--ds-page-padding-x) 14px; color: var(--ds-text-primary); }
.bench-bar { position: relative; z-index: 2; grid-area: bar; }
.bench-alert { grid-area: alert; }
.bench-graph { grid-area: graph; }
.bench-splitter { grid-area: split; }
.bench-preview { grid-area: preview; }
.bench-timeline { grid-area: timeline; }
// 非调试模式不挂载时间流：去掉右侧列与列间距，网格与预览占满宽度。
.quant-workbench:not(.is-debug) { grid-template-areas: 'bar' 'alert' 'graph' 'split' 'preview'; grid-template-columns: minmax(0, 1fr); column-gap: 0; }
.bench-bar { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 10px; }
.bench-title h1 { margin: 0; font-size: 19px; font-weight: 700; line-height: 1.3; }
.bench-actions { display: flex; min-width: 0; flex-wrap: wrap; justify-content: flex-end; align-items: center; gap: 4px; }
.bench-title { flex: 0 0 auto; }
.bar-divider { width: 1px; height: 18px; margin: 0 4px; background: var(--ds-border-default); }
.bench-alert { margin-bottom: 8px; }
// 两层面板共用切角外框，与图鉴等页面同一视觉语言。
.bench-graph, .bench-preview, .bench-timeline { --chamfer: 12px; position: relative; display: flex; min-height: 0; overflow: hidden; isolation: isolate; }
.bench-graph::before, .bench-preview::before, .bench-timeline::before { content: ''; position: absolute; inset: 0; z-index: -2; background: var(--ds-border-default); clip-path: polygon(0 0, calc(100% - var(--chamfer)) 0, 100% var(--chamfer), 100% 100%, var(--chamfer) 100%, 0 calc(100% - var(--chamfer))); }
.bench-graph::after, .bench-preview::after, .bench-timeline::after { content: ''; position: absolute; inset: 1px; z-index: -1; background: var(--ds-bg-panel); clip-path: polygon(0 0, calc(100% - var(--chamfer)) 0, 100% var(--chamfer), 100% 100%, var(--chamfer) 100%, 0 calc(100% - var(--chamfer))); }
.graph-canvas { min-width: 0; flex: 1; }
.bench-splitter { display: flex; align-items: center; justify-content: center; cursor: row-resize; touch-action: none; }
.bench-splitter span { width: 48px; height: 3px; border-radius: 2px; background: var(--ds-border-default); transition: background-color 150ms, width 150ms; }
.bench-splitter:hover span, .bench-splitter:focus-visible span { width: 72px; background: var(--ds-brand); }
.bench-splitter:focus-visible { outline: none; }
.bench-preview { flex-direction: column; }
.preview-bar { position: relative; display: flex; min-height: 38px; min-width: 0; flex-shrink: 0; align-items: center; gap: 10px; white-space: nowrap; border-bottom: 1px solid var(--ds-border-subtle); padding: 0 16px 0 14px; }
.preview-progress { position: absolute; right: 0; bottom: -1px; left: 0; height: 2px; overflow: hidden; background: var(--ds-border-subtle); pointer-events: none; }
.preview-progress::after { content: ''; position: absolute; inset: 0; background: var(--ds-brand); transform-origin: left; animation: preview-progress-travel 1.4s linear infinite; }
.preview-zones { display: flex; min-width: 0; gap: 2px; overflow-x: auto; scrollbar-width: none; }
.preview-zone { --zone: var(--ds-brand); position: relative; display: inline-flex; height: 24px; flex: 0 0 auto; align-items: center; gap: 5px; border: 0; border-radius: 2px; padding: 0 8px; background: transparent; color: var(--ds-text-secondary); font-size: 12px; white-space: nowrap; cursor: pointer; }
.preview-zone i { width: 8px; height: 8px; border-radius: 1px; background: var(--zone); }
.preview-zone::after { content: ''; position: absolute; right: 8px; bottom: 0; left: 8px; height: 2px; background: var(--zone); transform: scaleX(0); transition: transform 150ms; }
.preview-zone:hover { background: var(--ds-hover-bg); color: var(--ds-text-primary); }
.preview-zone.is-active { color: var(--ds-text-primary); font-weight: 600; }
.preview-zone.is-active::after { transform: scaleX(1); }
.name-input { width: 180px; flex: 0 1 180px; min-width: 96px; margin-left: auto; --n-border-radius: 2px !important; --n-color: var(--ds-hover-bg) !important; }
.preview-actions { display: flex; flex: 0 0 auto; align-items: center; gap: 2px; }
.preview-tabs { display: flex; flex: 0 0 auto; align-self: stretch; gap: 2px; }
.preview-tab { position: relative; display: inline-flex; flex: 0 0 auto; align-items: center; border: 0; padding: 0 8px; background: transparent; color: var(--ds-text-secondary); font-size: 13px; white-space: nowrap; cursor: pointer; transition: color 150ms; }
.preview-tab::after { content: ''; position: absolute; right: 8px; bottom: -1px; left: 8px; height: 2px; background: var(--ds-brand); transform: scaleX(0); transition: transform 150ms; }
.preview-tab:hover { color: var(--ds-text-primary); }
.preview-tab:focus-visible { outline: 1px solid var(--ds-brand); outline-offset: -4px; }
.preview-tab.is-active { color: var(--ds-text-primary); font-weight: 600; }
.preview-tab.is-active::after { transform: scaleX(1); }
// 隐藏而非卸载：保留预览尺寸与 WebGL 上下文，切回时无需重建。
.preview-stage.is-hidden { visibility: hidden; pointer-events: none; }
.preview-algorithm { position: absolute; inset: 0; }
.preview-body { position: relative; display: flex; min-height: 0; flex: 1; }
.preview-stage { position: relative; min-width: 0; flex: 1; }
// 空态覆盖在已挂载的预览器之上：不遮挡空白蓝图渲染出的星球主体，仅给提示文字加底。
.preview-empty { position: absolute; inset: 0; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 8px; color: var(--ds-text-muted); font-size: 13px; pointer-events: none; }
.preview-empty p { margin: 0; padding: 4px 10px; border-radius: 2px; background: color-mix(in srgb, var(--ds-bg-panel) 82%, transparent); }
.preview-empty :deep(.n-button) { pointer-events: auto; }
// 预览器预加载期间占用网格区域，避免流程图抢先挂载导致初始化顺序倒置。
.graph-boot { display: flex; flex: 1; align-items: center; justify-content: center; color: var(--ds-text-muted); font-size: 13px; }
.preview-overlay { position: absolute; top: 10px; left: 50%; border: 1px solid var(--ds-border-default); border-radius: 2px; padding: 3px 10px; background: var(--ds-bg-panel-elevated); color: var(--ds-text-secondary); font-size: 12px; transform: translateX(-50%); pointer-events: none; }
.muted { color: var(--ds-text-muted); font-size: 12px; line-height: 1.6; }
.import-actions { display: flex; justify-content: space-between; margin: 16px 0; }
@keyframes preview-progress-travel { from { transform: translateX(-35%) scaleX(.35); } to { transform: translateX(100%) scaleX(.35); } }
@media (max-width: 1099px) { .bench-actions :deep(.n-button__content) { display: none; } .bench-actions :deep(.n-button__icon) { margin: 0; } }
@media (prefers-reduced-motion: reduce) { .preview-progress::after { animation: none; transform: scaleX(1); opacity: .65; } .bench-splitter span { transition: none; } .preview-tab, .preview-tab::after { transition: none; } }
</style>
