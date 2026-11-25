<script setup lang="ts">
import {Compass, LoaderCircle, MessageSquarePlus, TriangleAlert} from '@lucide/vue'
import {computed, ref} from 'vue'

import {galaxyStatusContent, type GalaxyStatusKind} from '../../composables/feedback/galaxy-status'

const props = defineProps<{
  kind: GalaxyStatusKind
  message: string
  /** 重试请求进行中，重试按钮显示加载态。 */
  retrying: boolean
}>()
const emit = defineEmits<{
  retry: []
  publish: []
  locate: []
  home: []
}>()

// 各类状态各自一句主文案和至多一个动作，互不重复。error 使用 role=alert，其余为 status。
const content = computed(() => galaxyStatusContent(props.kind, props.message))
const alert = computed(() => props.kind === 'error')
const root = ref<HTMLElement>()
// 聚焦空态里的「提意见」入口；当前不是空态（已有星、错误或其它提示）时返回 false，由调用方回退到星图。
function focusPublish(): boolean {
  if (props.kind !== 'empty') return false
  const button = root.value?.querySelector<HTMLElement>('button:not([disabled])')
  if (!button) return false
  button.focus({preventScroll: true})
  return document.activeElement === button
}
defineExpose({focusPublish})
</script>

<template>
  <!-- ~ 星图状态提示 -->
  <!-- * 说明：加载中、加载失败、确实无反馈、所读星移出视野、所读星当前未单独显示、画面内无星，各用一句文案区分，同一时刻只显示一条。 -->
  <!-- * 交互：失败显示「重试」，无反馈显示「提意见」，移出视野显示「回到这颗星」，画面内无星显示「回到全景」。 -->
  <!-- * 依赖：galaxyStatus 的判定结果；动作由页面接入地图重试、发布面板与镜头回正。 -->
  <!-- * 边界：只占画布下方的一小条，不遮挡星图中心；窄屏下文字换行，按钮保持可点。 -->
  <div v-if="kind" ref="root" class="map-status" :class="['is-' + kind, {'is-center': kind === 'empty' || kind === 'loading'}]"
    :role="alert ? 'alert' : 'status'" :aria-live="alert ? 'assertive' : 'polite'">
    <LoaderCircle v-if="kind === 'loading'" class="status-icon spin" :size="16" aria-hidden="true" />
    <TriangleAlert v-else-if="kind === 'error'" class="status-icon" :size="16" aria-hidden="true" />
    <Compass v-else-if="kind === 'selected-offscreen' || kind === 'offscreen' || kind === 'selected-grouped'" class="status-icon" :size="16" aria-hidden="true" />
    <MessageSquarePlus v-else class="status-icon" :size="16" aria-hidden="true" />
    <div class="status-text">
      <strong>{{ content.title }}</strong>
      <span v-if="content.hint">{{ content.hint }}</span>
    </div>
    <n-button v-if="kind === 'error'" size="small" type="primary" :loading="retrying" @click="emit('retry')">{{ content.action }}</n-button>
    <n-button v-else-if="kind === 'empty'" size="small" type="primary" @click="emit('publish')">{{ content.action }}</n-button>
    <n-button v-else-if="kind === 'selected-offscreen'" size="small" @click="emit('locate')">{{ content.action }}</n-button>
    <n-button v-else-if="kind === 'offscreen'" size="small" @click="emit('home')">{{ content.action }}</n-button>
  </div>
</template>

<style scoped lang="scss">
.map-status {
  // 在面板之外的画布区域水平居中；面板打开时不会被盖住。
  position: absolute; z-index: 2; left: calc((100% - var(--galaxy-panel-width, 0px)) / 2); bottom: 18px; transform: translateX(-50%);
  width: max-content; max-width: min(460px, calc(100% - var(--galaxy-panel-width, 0px) - 24px));
  display: flex; align-items: center; gap: 10px; padding: 8px 12px;
  border: 1px solid var(--ds-border-default); border-radius: 4px;
  color: var(--ds-text-primary); background: color-mix(in srgb, var(--ds-bg-panel-elevated) 92%, transparent);
  font-size: 13px; line-height: 1.5;
}
// 0 条与加载中是画面的主体状态：提示放在银心下方，银心种子光仍是视觉焦点，不被文字盖住。
.map-status.is-center { bottom: auto; top: calc(50% + clamp(56px, 10vh, 96px)); }
.status-icon { flex: none; color: var(--ds-text-secondary); }
.is-error .status-icon { color: var(--ds-error); }
.status-text { min-width: 0; display: flex; flex-direction: column; overflow-wrap: anywhere; }
.status-text strong { font-weight: 600; }
.status-text span { color: var(--ds-text-secondary); }
.spin { animation: map-status-spin 1.1s linear infinite; }
@keyframes map-status-spin { to { transform: rotate(360deg); } }
@media (prefers-reduced-motion: reduce) { .spin { animation: none; } }
@media (max-width: 520px) {
  .map-status { flex-wrap: wrap; justify-content: center; text-align: center; }
  .status-text { align-items: center; }
}
</style>
