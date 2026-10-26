<script setup lang="ts">
import {Icon} from '@iconify/vue'
import {encyclopediaCategories} from '../../../services/encyclopedia/categories'
import {encyclopediaCategoryIconsReady} from '../../../services/encyclopedia/catalog'
import type {EncyclopediaCategory} from '../../../types/encyclopedia'

defineProps<{active: EncyclopediaCategory; animate: boolean}>()
const emit = defineEmits<{(event: 'select', category: EncyclopediaCategory): void}>()
</script>

<template>
  <!-- ~ 图鉴分类入口 -->
  <!-- * 说明：按参考页顺序展示十个图文入口，切角轮廓与实体索引保持同一语言 -->
  <!-- * 交互：鼠标、触屏或键盘选择分类；选中状态同时提供描边和 aria-pressed -->
  <!-- * 依赖：使用本地分类图标包，缺失时回退到打包的 Game Icons -->
  <!-- * 边界：窄内容区保留五列阅读顺序，不触发远程图标加载 -->
  <nav class="category-grid" :class="{'animate-entrance': animate}" aria-label="图鉴分类">
    <button v-for="(category, index) in encyclopediaCategories" :key="category.key" type="button"
      class="category-tile" :class="{'is-active': active !== 'overview' && active === category.key}"
      :aria-pressed="active === category.key" :style="{'--tile-delay': `${index * 24}ms`}" @click="emit('select', category.key)">
      <svg class="category-frame" viewBox="0 0 180 164" preserveAspectRatio="none" aria-hidden="true">
        <path class="frame-base" d="M1 12 12 1H179V151L166 163H1Z" />
        <path class="frame-corners" d="M1 27V12L12 1H30M151 1H179V26M179 136V151L166 163H147M28 163H1V140" />
        <path class="frame-detail" d="M145 159H163L175 147" />
      </svg>
      <span v-if="encyclopediaCategoryIconsReady.has(category.key)" class="category-icon category-game-icon" :style="{maskImage: `url(/encyclopedia-icons/categories/${category.key}.png)`, WebkitMaskImage: `url(/encyclopedia-icons/categories/${category.key}.png)`}" aria-hidden="true" />
      <Icon v-else :icon="category.icon" class="category-icon" aria-hidden="true" />
      <span class="category-name">{{ category.label }}</span>
    </button>
  </nav>
</template>

<style scoped lang="scss">
.category-grid { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 16px; }
.category-tile { position: relative; display: flex; min-width: 0; min-height: 112px; aspect-ratio: 1.14; flex-direction: column; align-items: center; justify-content: center; gap: 12px; border: 0; padding: 18px 8px; background: transparent; color: var(--ds-text-primary); cursor: pointer; transition: color 160ms, background-color 160ms, transform 160ms; }
.animate-entrance .category-tile { animation: category-arrive 360ms both; animation-delay: var(--tile-delay); }
.category-frame { position: absolute; inset: 0; width: 100%; height: 100%; fill: none; pointer-events: none; }
.category-frame path { vector-effect: non-scaling-stroke; }
.frame-base { stroke: var(--ds-border-default); stroke-width: 1; }
.frame-corners { stroke: var(--ds-text-secondary); stroke-width: 1.6; }
.frame-detail { stroke: var(--ds-text-muted); stroke-width: 1.2; }
.category-icon { width: 72px; height: 72px; flex: 0 0 auto; transition: transform 180ms; }
.category-game-icon { background: currentColor; mask-size: contain; mask-repeat: no-repeat; mask-position: center; -webkit-mask-size: contain; -webkit-mask-repeat: no-repeat; -webkit-mask-position: center; }
.category-name { font-size: 16px; font-weight: 700; }
.category-tile:hover { color: var(--ds-brand); background: var(--ds-hover-bg); transform: translateY(-2px); }
.category-tile:hover .category-icon { transform: scale(1.06); }
.category-tile:active { transform: translateY(1px) scale(.98); transition-duration: 70ms; }
.category-tile:focus-visible { outline: 2px solid var(--ds-focus-ring); outline-offset: 4px; }
.category-tile.is-active { background: var(--ds-active-bg); color: var(--ds-brand); }
.is-active .frame-corners { stroke: var(--ds-brand); stroke-width: 2; }
@keyframes category-arrive { from { opacity: 0; transform: translateY(8px); } to { opacity: 1; transform: translateY(0); } }
@container encyclopedia (max-width: 850px) { .category-tile { gap: 10px; padding: 12px 6px; } .category-icon { width: 52px; height: 52px; } }
@container encyclopedia (max-width: 600px) {
  .category-grid { gap: 10px; }
  .category-tile { min-height: 94px; gap: 8px; padding: 12px 4px; }
  .category-icon { width: 40px; height: 40px; }
  .category-name { font-size: 14px; }
}
@media (prefers-reduced-motion: reduce) { .animate-entrance .category-tile { animation: none; } .category-tile, .category-icon { transition: none; } }
</style>
