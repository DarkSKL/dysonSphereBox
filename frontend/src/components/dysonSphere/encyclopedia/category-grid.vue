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
  <!-- * 说明：紧凑切角指令栏，图标与名称横排，序号提供 HUD 层次，尺寸对齐盒子其他工作区控件 -->
  <!-- * 交互：鼠标、触屏或键盘选择分类；选中状态同时提供描边、强调条和 aria-pressed -->
  <!-- * 依赖：使用本地分类图标包，缺失时回退到打包的 Game Icons -->
  <!-- * 边界：建筑、物品、科技、战斗、成就五个入口；窄内容区横排或折为两列，不触发远程图标加载 -->
  <nav class="category-grid" :class="{'animate-entrance': animate}" aria-label="图鉴分类">
    <button v-for="(category, index) in encyclopediaCategories" :key="category.key" type="button"
      class="category-tile" :class="{'is-active': active === category.key}"
      :aria-pressed="active === category.key" :style="{'--tile-delay': `${index * 20}ms`}" @click="emit('select', category.key)">
      <span v-if="encyclopediaCategoryIconsReady.has(category.key)" class="category-icon category-game-icon" :style="{maskImage: `url(/encyclopedia-icons/categories/${category.key}.png)`, WebkitMaskImage: `url(/encyclopedia-icons/categories/${category.key}.png)`}" aria-hidden="true" />
      <Icon v-else :icon="category.icon" class="category-icon" aria-hidden="true" />
      <span class="category-name">{{ category.label }}</span>
      <span class="category-index" aria-hidden="true">{{ String(index + 1).padStart(2, '0') }}</span>
    </button>
  </nav>
</template>

<style scoped lang="scss">
// 切角使用像素 clip-path，任意宽度下斜边保持一致；::before 绘制 1px 描边内衬。
.category-grid { --chamfer: 8px; display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 8px; }
.category-tile { position: relative; display: flex; min-width: 0; height: 76px; flex-direction: column; align-items: center; justify-content: center; gap: 6px; border: 0; padding: 8px 4px 6px; background: var(--ds-border-default); color: var(--ds-text-secondary); cursor: pointer; isolation: isolate;
  clip-path: polygon(var(--chamfer) 0, 100% 0, 100% calc(100% - var(--chamfer)), calc(100% - var(--chamfer)) 100%, 0 100%, 0 var(--chamfer));
  transition: color 160ms, background-color 160ms, transform 160ms; }
.category-tile::before { content: ''; position: absolute; inset: 1px; z-index: -1; background: var(--ds-bg-panel); clip-path: inherit; transition: background-color 160ms; }
.category-tile::after { content: ''; position: absolute; bottom: 0; left: 50%; width: 0; height: 2px; background: var(--ds-brand); transform: translateX(-50%); transition: width 200ms; }
.animate-entrance .category-tile { animation: category-arrive 320ms both; animation-delay: var(--tile-delay); }
.category-icon { width: 30px; height: 30px; flex: 0 0 auto; transition: transform 180ms; }
.category-game-icon { background: currentColor; mask-size: contain; mask-repeat: no-repeat; mask-position: center; -webkit-mask-size: contain; -webkit-mask-repeat: no-repeat; -webkit-mask-position: center; }
.category-name { color: var(--ds-text-primary); font-size: 13px; font-weight: 600; line-height: 1.3; white-space: nowrap; }
.category-index { position: absolute; top: 5px; right: 7px; color: var(--ds-text-muted); font-family: var(--ds-font-display); font-size: 10px; letter-spacing: .04em; }
.category-tile:hover { background: var(--ds-border-strong); color: var(--ds-text-primary); }
.category-tile:hover::before { background: var(--ds-bg-panel-elevated); }
.category-tile:hover .category-icon { transform: translateY(-1px) scale(1.06); }
.category-tile:active { transform: scale(.97); transition-duration: 70ms; }
.category-tile:focus-visible { outline: 2px solid var(--ds-focus-ring); outline-offset: 2px; }
.category-tile.is-active { background: var(--ds-brand); color: var(--ds-brand); }
.category-tile.is-active::before { background: linear-gradient(180deg, var(--ds-bg-panel-elevated), var(--ds-bg-panel)); }
.category-tile.is-active::after { width: 40%; }
.category-tile.is-active .category-index { color: var(--ds-brand); }
@keyframes category-arrive { from { opacity: 0; transform: translateY(6px); } to { opacity: 1; transform: translateY(0); } }
@container encyclopedia (max-width: 1000px) { .category-tile { height: 52px; flex-direction: row; gap: 9px; padding: 6px 10px; } .category-icon { width: 24px; height: 24px; } }
@container encyclopedia (max-width: 520px) { .category-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 6px; } .category-tile { height: 44px; justify-content: flex-start; } }
@media (prefers-reduced-motion: reduce) { .animate-entrance .category-tile { animation: none; } .category-tile, .category-tile::before, .category-tile::after, .category-icon { transition: none; } }
</style>
