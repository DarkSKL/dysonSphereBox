<script setup lang="ts">
import {Icon} from '@iconify/vue'
import {ArrowRight} from '@lucide/vue'
import {computed, onBeforeUnmount, onMounted, ref} from 'vue'
import {encyclopediaKindIcons, encyclopediaKindLabels} from '../../../services/encyclopedia/categories'
import type {EncyclopediaEntry, EncyclopediaKey, EncyclopediaKind} from '../../../types/encyclopedia'
import EntryIcon from './entry-icon.vue'

defineProps<{title: string; kind: EncyclopediaKind; entries: readonly EncyclopediaEntry[]; total: number; showNames: boolean; expandable?: boolean; showKinds?: boolean}>()
const emit = defineEmits<{(event: 'open', key: EncyclopediaKey): void; (event: 'expand'): void}>()

// =============== 切角轮廓尺寸 ===============
// 说明：用实际容器尺寸绘制固定像素切角，避免长列表把标题斜边拉长；观察器在组件卸载时释放。
const root = ref<HTMLElement | null>(null)
const dimensions = ref({width: 700, height: 300})
const frameOutline = computed(() => {
  const {width: w, height: h} = dimensions.value
  return `M1 25 25 1H145L177 18H${w - 18}L${w - 1} 32V${h - 15}L${w - 16} ${h - 1}H1Z`
})
const frameCorners = computed(() => {
  const {width: w, height: h} = dimensions.value
  return `M1 45V25L25 1H44M${w - 30} 18H${w - 18}L${w - 1} 32V45M${w - 1} ${h - 30}V${h - 15}L${w - 16} ${h - 1}H${w - 38}M22 ${h - 1}H1V${h - 24}`
})
let observer: ResizeObserver | undefined
onMounted(() => {
  if (!root.value) return
  observer = new ResizeObserver(([entry]) => { if (entry) dimensions.value = {width: entry.borderBoxSize[0]?.inlineSize ?? entry.contentRect.width, height: entry.borderBoxSize[0]?.blockSize ?? entry.contentRect.height} })
  observer.observe(root.value)
})
onBeforeUnmount(() => observer?.disconnect())
</script>

<template>
  <!-- ~ 实体图标索引 -->
  <!-- * 说明：切角分区内按游戏数据顺序展示建筑或物品，名称可按需展开 -->
  <!-- * 交互：点击条目打开详情；键盘聚焦时同样显示名称；更多入口进入完整分类 -->
  <!-- * 依赖：已筛选实体与分片总量，不直接读取 JSON -->
  <!-- * 边界：数量由真实数据计算，图标缺失交给 EntryIcon 回退 -->
  <section ref="root" class="entry-section" :aria-label="title" :data-kind="kind">
    <svg class="section-frame" :viewBox="`0 0 ${dimensions.width} ${dimensions.height}`" preserveAspectRatio="none" aria-hidden="true">
      <path class="section-outline" :d="frameOutline" />
      <path class="section-corners" :d="frameCorners" />
    </svg>
    <header class="section-heading">
      <div class="section-title"><Icon :icon="encyclopediaKindIcons[kind]" :width="25" :height="25" aria-hidden="true" /><h2>{{ title }}</h2><span class="section-count">{{ total }}</span></div>
      <button v-if="expandable && total > entries.length" type="button" class="section-more" @click="emit('expand')">查看全部<ArrowRight :size="15" /></button>
    </header>
    <div class="entry-grid" :class="{'with-names': showNames}">
      <button v-for="entry in entries" :key="`${entry.kind}:${entry.id}`" type="button" class="entry-button" :aria-label="`${entry.name.zh}，${encyclopediaKindLabels[entry.kind]}，${entry.group}`"
        :data-entry="`${entry.kind}:${entry.id}`" @click="emit('open', `${entry.kind}:${entry.id}`)">
        <EntryIcon :entry="entry" />
        <span class="entry-name">{{ entry.name.zh }}<span v-if="showKinds" class="entry-kind">{{ encyclopediaKindLabels[entry.kind] }}</span></span>
      </button>
    </div>
  </section>
</template>

<style scoped lang="scss">
.entry-section { position: relative; min-width: 0; padding: 29px 24px 24px; }
.section-frame { position: absolute; inset: 0; width: 100%; height: 100%; fill: none; pointer-events: none; }
.section-frame path { vector-effect: non-scaling-stroke; }
.section-outline { stroke: var(--ds-border-default); stroke-width: 1; }
.section-corners { stroke: var(--ds-text-secondary); stroke-width: 1.6; }
.section-detail { stroke: var(--ds-text-muted); stroke-width: 1; }
.section-heading { position: relative; display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 20px; }
.section-title { display: flex; min-width: 0; align-items: center; gap: 10px; color: var(--ds-text-primary); }
.section-title h2 { margin: 0; font-size: 20px; font-weight: 700; line-height: 1.4; }
.section-count { margin-left: 3px; color: var(--ds-text-secondary); font-family: var(--ds-font-display); font-size: 14px; }
.section-more { display: inline-flex; align-items: center; flex: 0 0 auto; gap: 7px; border: 0; padding: 5px 0 5px 8px; background: transparent; color: var(--ds-text-secondary); font-size: 13px; cursor: pointer; }
.section-more:hover { color: var(--ds-brand); }
.entry-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(58px, 1fr)); gap: 12px 9px; }
.entry-button { position: relative; display: flex; min-width: 0; min-height: 58px; flex-direction: column; align-items: center; justify-content: center; border: 1px solid transparent; border-radius: 3px; padding: 3px 0; background: transparent; color: var(--ds-text-primary); cursor: pointer; transition: border-color 160ms, background-color 160ms, transform 160ms; }
.entry-button:hover, .entry-button:focus-visible { z-index: 2; outline: none; border-color: var(--ds-border-strong); background: var(--ds-active-bg); transform: translateY(-2px); }
.entry-button:active { transform: scale(.96); transition-duration: 70ms; }
.entry-name { position: absolute; top: calc(100% + 3px); left: 50%; z-index: 3; max-width: 180px; border: 1px solid var(--ds-border-default); border-radius: 3px; padding: 4px 7px; background: var(--ds-bg-panel-elevated); color: var(--ds-text-primary); font-size: 13px; line-height: 1.5; opacity: 0; visibility: hidden; transform: translateX(-50%); pointer-events: none; white-space: nowrap; }
.entry-button:hover .entry-name, .entry-button:focus-visible .entry-name { opacity: 1; visibility: visible; }
.entry-kind { display: block; color: var(--ds-text-secondary); font-size: 12px; }
.with-names { grid-template-columns: repeat(auto-fill, minmax(94px, 1fr)); gap: 14px 8px; }
.with-names .entry-name { position: static; width: 100%; max-width: 100%; border: 0; padding: 5px 2px 0; background: transparent; line-height: 1.5; opacity: 1; visibility: visible; transform: none; white-space: normal; overflow-wrap: anywhere; }
.with-names .entry-button { justify-content: flex-start; }
@container encyclopedia (max-width: 600px) { .entry-section { padding: 25px 17px 21px; } .section-title h2 { font-size: 18px; } .entry-grid { gap: 10px 5px; grid-template-columns: repeat(auto-fill, minmax(54px, 1fr)); } .with-names { grid-template-columns: repeat(auto-fill, minmax(84px, 1fr)); } }
@media (prefers-reduced-motion: reduce) { .entry-button { transition: none; } }
</style>
