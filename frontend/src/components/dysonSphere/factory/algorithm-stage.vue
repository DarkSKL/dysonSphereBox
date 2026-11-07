<script setup lang="ts">
import {computed, onActivated, onBeforeUnmount, onDeactivated, onMounted, ref, watch, watchEffect} from 'vue'
import {ChevronsRight, Crosshair, Pause, Play, RefreshCw, RotateCcw, RotateCw, Sparkles, X} from '@lucide/vue'
import basisTranscoderJs from 'three/examples/jsm/libs/basis/basis_transcoder.js?url'
import basisTranscoderWasm from 'three/examples/jsm/libs/basis/basis_transcoder.wasm?url'
import {ALGORITHM_VIZ_MODEL_KEYS, createAlgorithmViz, type AlgorithmVizController, type AlgoVizBuilding, type AlgoVizProgress, type AlgoVizSelection} from '../../../packages/algorithm-viz/src'
import {useThemeMode} from '../../../composables/useThemeMode'
import {ALGORITHM_VIZ_LIMIT, algorithmFollow, algorithmFollowKey, useAlgorithmFeed} from '../../../composables/dysonSphere/useAlgorithmFeed'
import type {QuantWorkspace} from '../../../composables/dysonSphere/useQuantWorkspace'

const props = defineProps<{w: QuantWorkspace; active: boolean}>()

// =============== 模型资源 ===============
// 说明：只打包建筑模型；无人机、运输船、火箭等非建筑模型与空的传送带占位文件不进入构建产物。
// 模型贴图为 KTX2 格式，Basis 转码器随 three 提供，以 ?url 导入后交给渲染包按实际地址加载。
// 边界：模型按需加载，缺失或加载失败的建筑由场景以几何体占位，不影响播放。
const modelFiles = import.meta.glob([
  '../../../data/models/*.glb',
  '!**/belt-*.glb',
  '!**/*-drone.glb',
  '!**/logistic-vessel.glb',
  '!**/rocket.glb',
  '!**/space-capsule.glb',
  '!**/stone-wall.glb',
  '!**/energy-exchanger-mcl-*.glb',
], {as: 'url', eager: true})
// 涂装遮罩：游戏只把材质颜色染在贴图 alpha 通道标出的涂装区，GLB 的贴图没有保留这个通道，单独以图片提供
const maskFiles = import.meta.glob('../../../data/models/masks/mask-*.png', {as: 'url', eager: true})
const maskUrls: Record<string, string> = {}
for (const [path, url] of Object.entries(maskFiles)) {
  maskUrls[path.slice(path.lastIndexOf('mask-') + 'mask-'.length, -'.png'.length)] = url
}
const knownKeys = new Set(ALGORITHM_VIZ_MODEL_KEYS)
const modelUrls: Record<string, string> = {}
for (const [path, url] of Object.entries(modelFiles)) {
  const key = path.slice(path.lastIndexOf('/') + 1, -'.glb'.length)
  if (knownKeys.has(key)) modelUrls[key] = url
}

// =============== 场景与数据源 ===============
// 说明：挂载时创建 three.js 场景，数据源把生成事件与结果翻译为场景操作；卸载时释放 WebGL 资源。
const host = ref<HTMLDivElement | null>(null)
const {themeMode, loadThemeMode} = useThemeMode()
let viz: AlgorithmVizController | null = null
const feed = useAlgorithmFeed(props.w, () => viz)
const {source: feedSource, notice: feedNotice, decoding, phase} = feed
const progress = ref<AlgoVizProgress | null>(null)
const speeds = [0.5, 1, 2, 4]
const speed = ref(1)
const paused = ref(false)
const selection = ref<AlgoVizSelection | null>(null)

const productionLabel = (building: AlgoVizBuilding): string => {
  const group = building.group ?? -1
  if (group < 0) return '共享设施'
  const production = props.w.replay ? undefined : props.w.plan?.productions[group]
  return production ? `${props.w.itemName(production.mainItemId)} 产线` : props.w.replay ? `配方组 ${group + 1}` : `产线 ${group + 1}`
}
const describe = (building: AlgoVizBuilding): string => `${props.w.itemName(building.itemId)} · ${productionLabel(building)}`

// 选中建筑时同步工作区：节点图高亮对应产线，蓝图预览定位同一建筑
const onSelect = (value: AlgoVizSelection | null): void => {
  selection.value = value
  const index = value?.building.index
  if (index !== undefined && !value?.building.temporary) props.w.onBuildingSelect({index})
}

onMounted(async () => {
  await loadThemeMode()
  if (!host.value) return
  viz = createAlgorithmViz(host.value, {
    theme: themeMode.value,
    modelUrls,
    maskUrls,
    transcoderUrls: {js: basisTranscoderJs, wasm: basisTranscoderWasm},
    speed: speed.value,
    maxObjects: ALGORITHM_VIZ_LIMIT,
    viewPaddingTop: 64,
    describe,
    onProgress: value => { progress.value = value },
    onSelect,
  })
  void feed.sync()
})
onBeforeUnmount(() => {
  algorithmFollow.active = false
  viz?.dispose()
  viz = null
})

// =============== 实时跟随 ===============
// 说明：标签页可见且所在页面处于激活状态时，生成请求开启过程差量，画面边计算边播放；
//       切走标签或离开页面（KeepAlive 停用）即关闭，避免后台生成承担额外事件开销。
const pageActive = ref(true)
onActivated(() => { pageActive.value = true })
onDeactivated(() => { pageActive.value = false })
watchEffect(() => { algorithmFollow.active = props.active && pageActive.value })
const followUnsupported = computed(() => algorithmFollow.unsupported.has(algorithmFollowKey(props.w.mode, props.w.density)))
const canRegenerate = computed(() => Boolean(props.w.plan && props.w.plan.productions.length && !props.w.quantStale && !props.w.solving && !props.w.generating && !props.w.importing && !props.w.replay))
// 重新运行算法：对当前方案重新生成一次，可视化从头跟随整个计算过程
const regenerate = (): void => {
  if (!canRegenerate.value) return
  viz?.select(null)
  void props.w.generate()
}

// =============== 外部状态同步 ===============
// 说明：只读应用主题；应用自身有圆形揭示动画，场景立即切换避免新旧配色混杂。切走标签页时关闭选中卡片。
watch(themeMode, mode => viz?.setTheme(mode, true))
watch(() => props.active, active => { if (!active) viz?.select(null) })

// =============== 播放控制 ===============
const togglePause = (): void => {
  paused.value = !paused.value
  viz?.setPaused(paused.value)
}
const cycleSpeed = (): void => {
  speed.value = speeds[(speeds.indexOf(speed.value) + 1) % speeds.length]
  viz?.setSpeed(speed.value)
}
const skip = (): void => viz?.flush()
const rebuild = (): void => {
  viz?.select(null)
  feed.rebuild()
}

// =============== 状态展示 ===============
const sourceLabel = computed(() => decoding.value ? '解析结果' : feedSource.value === 'live'
  ? props.w.generating ? '计算中 · 候选演示' : progress.value && (progress.value.pending || progress.value.animating || progress.value.falling || progress.value.candidates > 1) ? '计算完成 · 演示中' : feedNotice.value ? '本次演示已结束' : '已保留获选方案'
  : feedSource.value === 'replay' ? '结果回放' : '等待结果')
const sourceHint = computed(() => feedSource.value === 'live'
  ? '所有候选完整铺开，按独立视觉速度展示真实装配过程；失败候选播至失败节点即下坠，最终只保留获选方案。计算完成后演示可继续，可调速、暂停或跳过'
  : followUnsupported.value
    ? '当前模式与密度暂不提供完整的过程快照，已改为结果回放：生成完成后按装配顺序重建'
    : '显示的是打开本页前生成或导入的结果，按装配顺序回放；点击「重新运行算法」可从头跟随计算过程')
const phaseText = computed(() => {
  const value = phase.value
  if (!value) return ''
  return value.total > 0 ? `${value.message} · ${value.current}/${value.total}` : value.message
})
const busy = computed(() => Boolean(progress.value && (progress.value.pending > 0 || progress.value.animating > 0)))
const cardStyle = computed(() => {
  const value = selection.value
  if (!value || !host.value) return {}
  const width = host.value.clientWidth
  const height = host.value.clientHeight
  return {left: `${Math.min(Math.max(value.screen.x + 14, 8), width - 228)}px`, top: `${Math.min(Math.max(value.screen.y - 24, 48), height - 120)}px`}
})
const format = (value: number | undefined): string => (value ?? 0).toFixed(value !== undefined && Number.isInteger(value) ? 0 : 2)
</script>

<template>
  <!-- ~ 算法可视化 -->
  <!-- * 说明：在等距视角的浮空平台上播放蓝图生成算法：建筑落位、平移替换、候选回退与正式提交 -->
  <!-- * 交互：左键拖动旋转（松手吸附 90°），右键拖动平移，滚轮缩放，双击空白复位；点击建筑查看详情并联动产线 -->
  <!-- * 依赖：@dyson/algorithm-viz 渲染包、useAlgorithmFeed 数据源、useQuantWorkspace 结果与产线 -->
  <!-- * 边界：只读旁路；可见时为生成请求开启过程差量，模式无法提供完整过程时由工作区改为普通生成；WebGL 不可用时由渲染包显示降级提示 -->
  <div class="algo-stage">
    <div ref="host" class="algo-canvas" />

    <!-- 左上：数据来源、操作统计与算法实时阶段 -->
    <div class="algo-hud">
      <div class="algo-hud-row">
        <n-tooltip placement="bottom-start"><template #trigger><span class="algo-source" :class="`is-${decoding ? 'busy' : feedSource}`"><i />{{ sourceLabel }}</span></template>{{ sourceHint }}</n-tooltip>
        <span v-if="progress" class="algo-stats">
          <span v-if="progress.candidates">平台 <strong class="type-display">{{ progress.candidates }}</strong></span>
          <span>建筑 <strong class="type-display">{{ progress.objects }}</strong></span>
          <span class="is-add">新增 <strong class="type-display">{{ progress.added }}</strong></span>
          <span class="is-replace">替换 <strong class="type-display">{{ progress.replaced }}</strong></span>
          <span class="is-remove">撤销 <strong class="type-display">{{ progress.removed }}</strong></span>
          <span v-if="progress.pending">队列 <strong class="type-display">{{ progress.pending }}</strong></span>
        </span>
      </div>
      <!-- 算法实时阶段：直接取自生成过程的阶段事件，与画面播放进度无关；生成结束后隐藏 -->
      <p v-if="phaseText" class="algo-phase" role="status" aria-live="polite"><i aria-hidden="true" />{{ phaseText }}</p>
    </div>

    <!-- 右上：算法、播放与视角控制 -->
    <div class="algo-tools" role="toolbar" aria-label="算法可视化控制">
      <n-tooltip><template #trigger><button type="button" class="tool-button run" :disabled="!canRegenerate" aria-label="重新运行算法" @click="regenerate"><Sparkles :size="13" /><span>重新运行算法</span></button></template>对当前方案重新生成一次，从头跟随算法的计算过程</n-tooltip>
      <span class="tool-divider" />
      <n-tooltip><template #trigger><button type="button" class="tool-button icon-only" :class="{'is-on': paused}" :aria-label="paused ? '继续播放' : '暂停播放'" @click="togglePause"><Play v-if="paused" :size="14" /><Pause v-else :size="14" /></button></template>{{ paused ? '继续播放' : '暂停播放' }}</n-tooltip>
      <n-tooltip><template #trigger><button type="button" class="tool-button speed" aria-label="切换播放速度" @click="cycleSpeed"><span class="type-display">{{ speed }}×</span></button></template>播放速度，点击切换</n-tooltip>
      <n-tooltip><template #trigger><button type="button" class="tool-button icon-only" :disabled="!busy" aria-label="跳过动画" @click="skip"><ChevronsRight :size="14" /></button></template>跳过动画，直接显示最终状态</n-tooltip>
      <n-tooltip><template #trigger><button type="button" class="tool-button icon-only" :disabled="!w.code" aria-label="重新回放" @click="rebuild"><RefreshCw :size="14" /></button></template>清空舞台，按装配顺序重新回放当前结果</n-tooltip>
      <span class="tool-divider" />
      <n-tooltip><template #trigger><button type="button" class="tool-button icon-only" aria-label="向左旋转" @click="viz?.rotate(-1)"><RotateCcw :size="14" /></button></template>向左旋转 90°</n-tooltip>
      <n-tooltip><template #trigger><button type="button" class="tool-button icon-only" aria-label="向右旋转" @click="viz?.rotate(1)"><RotateCw :size="14" /></button></template>向右旋转 90°</n-tooltip>
      <n-tooltip><template #trigger><button type="button" class="tool-button icon-only" aria-label="复位视角" @click="viz?.resetView()"><Crosshair :size="14" /></button></template>复位视角并恢复自动取景</n-tooltip>
    </div>

    <!-- 选中建筑卡片：跟随点击位置，显示建筑、产线与坐标 -->
    <div v-if="selection && active" class="algo-card" :style="cardStyle" role="dialog" aria-label="建筑详情">
      <header><strong>{{ w.itemName(selection.building.itemId) }}</strong><button type="button" class="icon-button" aria-label="关闭详情" @click="viz?.select(null)"><X :size="12" /></button></header>
      <dl>
        <dt>归属</dt><dd>{{ productionLabel(selection.building) }}</dd>
        <dt>坐标</dt><dd class="type-mono">{{ format(selection.building.x) }}, {{ format(selection.building.y) }}, {{ format(selection.building.z) }}</dd>
        <dt>状态</dt><dd>{{ selection.building.temporary ? '生成中的临时建筑' : selection.building.index !== undefined ? `正式记录 #${selection.building.index}` : '正式建筑' }}</dd>
      </dl>
    </div>

    <p v-if="feedNotice" class="algo-notice" role="status">{{ feedNotice }}</p>
  </div>
</template>

<style scoped lang="scss">
.algo-stage { position: absolute; inset: 0; overflow: hidden; container-type: inline-size; }
.algo-canvas { position: absolute; inset: 0; }
.algo-hud { position: absolute; top: 10px; left: 12px; z-index: 2; display: flex; max-width: calc(100% - 430px); flex-direction: column; align-items: flex-start; gap: 6px; pointer-events: none; }
.algo-hud-row { display: flex; max-width: 100%; align-items: center; gap: 8px; }
.algo-hud-row > * { pointer-events: auto; }
.algo-phase { display: inline-flex; max-width: 100%; align-items: center; gap: 6px; margin: 0; overflow: hidden; padding: 0 2px; color: var(--ds-text-secondary); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; text-shadow: 0 0 8px var(--ds-bg-app); }
.algo-phase i { width: 6px; height: 6px; flex: 0 0 6px; border-radius: 50%; background: var(--ds-brand); animation: algo-pulse 1s ease-in-out infinite; }
.algo-source { display: inline-flex; height: 24px; flex: 0 0 auto; align-items: center; gap: 6px; border: 1px solid var(--ds-border-default); border-radius: 2px; padding: 0 8px; background: color-mix(in srgb, var(--ds-bg-panel-elevated) 88%, transparent); color: var(--ds-text-primary); font-size: 12px; white-space: nowrap; }
.algo-source i { width: 6px; height: 6px; border-radius: 50%; background: var(--ds-text-muted); }
.algo-source.is-live i { background: var(--ds-brand); box-shadow: 0 0 6px var(--ds-brand); animation: algo-pulse 1.2s ease-in-out infinite; }
.algo-source.is-replay i { background: var(--ds-success); }
.algo-source.is-busy i { background: var(--ds-warning); animation: algo-pulse 1s ease-in-out infinite; }
.algo-stats { display: inline-flex; height: 24px; min-width: 0; align-items: center; gap: 10px; overflow: hidden; border: 1px solid var(--ds-border-default); border-radius: 2px; padding: 0 10px; background: color-mix(in srgb, var(--ds-bg-panel-elevated) 88%, transparent); color: var(--ds-text-muted); font-size: 11px; white-space: nowrap; }
.algo-stats strong { margin-left: 2px; color: var(--ds-text-primary); font-size: 12px; font-variant-numeric: tabular-nums; }
.algo-stats .is-add strong { color: var(--ds-success); }
.algo-stats .is-replace strong { color: var(--ds-info); }
.algo-stats .is-remove strong { color: var(--ds-warning); }
.algo-tools { position: absolute; top: 10px; right: 12px; z-index: 2; display: flex; align-items: center; gap: 4px; }
.tool-button { display: inline-flex; height: 28px; align-items: center; justify-content: center; border: 1px solid var(--ds-border-default); border-radius: 2px; padding: 0 8px; background: var(--ds-bg-panel-elevated); color: var(--ds-text-secondary); font-size: 12px; cursor: pointer; transition: border-color 150ms, color 150ms; }
.tool-button.icon-only { width: 28px; padding: 0; }
.tool-button.speed { min-width: 40px; }
.tool-button.run { gap: 5px; }
.tool-button:hover:not(:disabled) { border-color: var(--ds-border-strong); color: var(--ds-text-primary); }
.tool-button:disabled { cursor: not-allowed; opacity: .45; }
.tool-button.is-on { border-color: var(--ds-brand); color: var(--ds-brand); }
.tool-button:focus-visible { outline: 1px solid var(--ds-brand); outline-offset: 1px; }
.tool-divider { width: 1px; height: 18px; margin: 0 2px; background: var(--ds-border-default); }
.algo-card { position: absolute; z-index: 3; width: 220px; border: 1px solid var(--ds-border-default); border-radius: 2px; background: color-mix(in srgb, var(--ds-bg-panel-elevated) 94%, transparent); color: var(--ds-text-primary); font-size: 12px; backdrop-filter: blur(6px); }
.algo-card header { display: flex; align-items: center; justify-content: space-between; gap: 6px; border-bottom: 1px solid var(--ds-border-subtle); padding: 6px 6px 6px 10px; }
.algo-card header strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.algo-card dl { display: grid; grid-template-columns: auto 1fr; gap: 4px 10px; margin: 0; padding: 8px 10px; }
.algo-card dt { color: var(--ds-text-muted); }
.algo-card dd { min-width: 0; margin: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.icon-button { display: inline-flex; width: 20px; height: 20px; align-items: center; justify-content: center; border: 0; border-radius: 2px; background: transparent; color: var(--ds-text-muted); cursor: pointer; }
.icon-button:hover { background: var(--ds-hover-bg); color: var(--ds-text-primary); }
.algo-notice { position: absolute; bottom: 10px; left: 12px; z-index: 2; max-width: calc(100% - 24px); margin: 0; border: 1px solid var(--ds-border-default); border-radius: 2px; padding: 4px 10px; background: var(--ds-bg-panel-elevated); color: var(--ds-warning); font-size: 12px; }
@keyframes algo-pulse { 50% { opacity: .35; } }
@media (prefers-reduced-motion: reduce) { .algo-source i, .algo-phase i { animation: none !important; } .tool-button { transition: none; } }
@container (max-width: 760px) { .algo-stats { display: none; } .tool-button.run span { display: none; } .tool-button.run { width: 28px; padding: 0; } }
</style>
