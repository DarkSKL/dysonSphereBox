<script setup lang="ts">
import {computed, onBeforeUnmount, onMounted, ref, shallowRef, watch, type CSSProperties} from 'vue'

import type {Coordinates, StarPreview} from '../../types/feedback'

const props = defineProps<{quote: StarPreview; anchor: Coordinates}>()
const emit = defineEmits<{hold: []; leave: []}>()

// =============== 星点旁的引文 ===============
// 以正文的实际排版尺寸决定翻边位置，打开侧栏或改变窗口大小时仍留在可阅读的区域内。
const plane = ref<HTMLElement>()
const figure = ref<HTMLElement>()
const measured = shallowRef({areaWidth: 0, areaHeight: 0, width: 0, height: 0})
let observer: ResizeObserver | undefined

function measure(): void {
  if (!plane.value || !figure.value) return
  measured.value = {areaWidth: plane.value.clientWidth, areaHeight: plane.value.clientHeight,
    width: figure.value.offsetWidth, height: figure.value.offsetHeight}
}

const position = computed<CSSProperties>(() => {
  const {areaWidth, areaHeight, width, height} = measured.value
  if (!areaWidth || !areaHeight) return {visibility: 'hidden'}
  const {x, y} = props.anchor
  const margin = 8, gap = 13
  const leftRoom = x - gap - margin, rightRoom = areaWidth - x - gap - margin
  const readable = Math.min(175, (areaWidth - margin * 2) / 2)
  // 优先向星盘外侧排版，空间足够时就地换行，避免把引文盖到银心。
  const right = x >= areaWidth / 2 ? rightRoom >= readable || rightRoom >= leftRoom : leftRoom < readable && rightRoom > leftRoom
  const room = right ? rightRoom : leftRoom
  const maxWidth = Math.max(1, room >= readable ? room : areaWidth - margin * 2)
  const maxHeight = Math.max(1, areaHeight - margin * 2)
  const left = right ? x + gap : x - gap - width
  const top = y + gap + height <= areaHeight - margin ? y + gap : y - gap - height
  return {
    left: Math.max(margin, Math.min(left, areaWidth - margin - width)) + 'px',
    top: Math.max(margin, Math.min(top, areaHeight - margin - height)) + 'px',
    maxWidth: `min(25em, ${Math.floor(maxWidth)}px)`,
    maxHeight: maxHeight + 'px',
    overflowY: height >= maxHeight ? 'auto' : undefined,
    visibility: width ? undefined : 'hidden',
  }
})

onMounted(() => {
  measure()
  observer = new ResizeObserver(measure)
  if (plane.value) observer.observe(plane.value)
  if (figure.value) observer.observe(figure.value)
})
watch(() => props.quote, measure, {flush: 'post'})
onBeforeUnmount(() => observer?.disconnect())
</script>

<template>
  <!-- ~ 引文预览 -->
  <!-- * 说明：正文与署名紧随其后，长文本在可用宽度内自然换行。 -->
  <!-- * 交互：指针可移入引文阅读，Escape 收起。 -->
  <!-- * 依赖：当前可见反馈的首行与服务端已投影的署名。 -->
  <div ref="plane" class="quote-plane">
    <figure id="feedback-star-quote" ref="figure" class="star-preview" role="tooltip" :style="position"
      @pointerenter="emit('hold')" @pointerleave="emit('leave')">
      <blockquote>{{ quote.preview }}</blockquote>
      <figcaption v-if="quote.displayName"><span aria-hidden="true">—</span><bdi>{{ quote.displayName }}</bdi></figcaption>
    </figure>
  </div>
</template>

<style scoped lang="scss">
.quote-plane {
  position: absolute; inset: 0 var(--galaxy-panel-width, 0px) 0 0; z-index: 1; pointer-events: none;
}
.star-preview {
  position: absolute; isolation: isolate; pointer-events: auto; margin: 0;
  box-sizing: border-box; width: max-content; max-width: min(25em, calc(100% - 16px)); padding: 4px 7px;
  color: var(--ds-text-primary); font: 400 14px/1.6 var(--ds-font-ui, sans-serif);
  overflow-wrap: anywhere;
}
// 引文垫一层随主题变化的柔和底：夜间深色、日间近白，正文与云雾叠放时对比度不依赖星云亮度。
.star-preview::before {
  content: ''; position: absolute; z-index: -1; inset: 1px -2px;
  border-radius: 5px; background: color-mix(in srgb, var(--ds-bg-panel-elevated) 86%, transparent); filter: blur(5px);
}
blockquote { display: inline; margin: 0; padding: 0; white-space: pre-wrap; }
figcaption {
  display: inline-flex; align-items: baseline; gap: .38em; max-width: 100%;
  margin-inline-start: .5em; font-size: 12px; color: var(--ds-text-secondary); vertical-align: baseline;
}
figcaption span { flex: none; }
bdi { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
