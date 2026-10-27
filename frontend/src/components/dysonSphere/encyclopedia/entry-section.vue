<script setup lang="ts">
import {Icon} from '@iconify/vue'
import {encyclopediaKindIcons, encyclopediaKindLabels} from '../../../services/encyclopedia/categories'
import type {EncyclopediaEntry, EncyclopediaKey, EncyclopediaKind} from '../../../types/encyclopedia'
import EntryIcon from './entry-icon.vue'

defineProps<{title: string; kind: EncyclopediaKind; entries: readonly EncyclopediaEntry[]; total: number; showNames: boolean; showKinds?: boolean}>()
const emit = defineEmits<{(event: 'open', key: EncyclopediaKey): void}>()
</script>

<template>
  <!-- ~ 实体图标索引 -->
  <!-- * 说明：切角面板内按游戏数据顺序展示建筑或物品，标题栏与网格分层，名称可按需展开 -->
  <!-- * 交互：点击条目打开详情；键盘聚焦时同样显示名称；标题栏右侧 actions 插槽放置分组筛选等控件 -->
  <!-- * 依赖：已筛选实体与分片总量，不直接读取 JSON -->
  <!-- * 边界：切角只裁剪背景伪元素，悬浮名称不被面板裁切；图标缺失交给 EntryIcon 回退 -->
  <section class="entry-section" :aria-label="title" :data-kind="kind">
    <header class="section-heading">
      <div class="section-title"><Icon :icon="encyclopediaKindIcons[kind]" :width="18" :height="18" aria-hidden="true" /><h2>{{ title }}</h2><span class="section-count">{{ total }}</span></div>
      <div v-if="$slots.actions" class="section-actions"><slot name="actions" /></div>
    </header>
    <div class="entry-grid" :class="{'with-names': showNames}">
      <button v-for="entry in entries" :key="`${entry.kind}:${entry.id}`" type="button" class="entry-button ds-bracket-hover" :aria-label="`${entry.name.zh}，${encyclopediaKindLabels[entry.kind]}，${entry.group}`"
        :data-entry="`${entry.kind}:${entry.id}`" @click="emit('open', `${entry.kind}:${entry.id}`)">
        <EntryIcon :entry="entry" :size="40" />
        <span class="entry-name">{{ entry.name.zh }}<span v-if="showKinds" class="entry-kind">{{ encyclopediaKindLabels[entry.kind] }}</span></span>
      </button>
    </div>
  </section>
</template>

<style scoped lang="scss">
// 面板：::before 为描边层、::after 为底色层，二者共用右上 / 左下切角。
.entry-section { --chamfer: 12px; position: relative; min-width: 0; padding: 0 16px 16px; isolation: isolate; }
.entry-section::before, .entry-section::after { content: ''; position: absolute; z-index: -1; clip-path: polygon(0 0, calc(100% - var(--chamfer)) 0, 100% var(--chamfer), 100% 100%, var(--chamfer) 100%, 0 calc(100% - var(--chamfer))); pointer-events: none; }
.entry-section::before { inset: 0; background: var(--ds-border-default); }
.entry-section::after { inset: 1px; background: var(--ds-bg-panel); opacity: .72; }
.section-heading { position: relative; display: flex; height: 44px; align-items: center; justify-content: space-between; gap: 12px; margin: 0 -16px 14px; border-bottom: 1px solid var(--ds-border-subtle); padding: 0 20px 0 16px; }
.section-title { display: flex; min-width: 0; align-items: center; gap: 8px; color: var(--ds-text-primary); }
.section-title > svg { color: var(--ds-text-secondary); }
.section-title h2 { margin: 0; font-size: 15px; font-weight: 700; line-height: 1.4; }
.section-count { color: var(--ds-text-muted); font-family: var(--ds-font-display); font-size: 12px; }
.section-actions { display: flex; min-width: 0; flex: 0 1 auto; align-items: center; gap: 8px; }
.entry-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(50px, 1fr)); gap: 6px; }
.entry-button { position: relative; display: flex; min-width: 0; min-height: 50px; flex-direction: column; align-items: center; justify-content: center; border: 1px solid transparent; border-radius: 2px; padding: 4px 0; background: var(--ds-hover-bg); color: var(--ds-text-primary); cursor: pointer; transition: border-color 150ms, background-color 150ms, transform 150ms; }
.entry-button { --bracket-size: 10px; --bracket-color: var(--ds-brand); }
.entry-button:hover, .entry-button:focus-visible { z-index: 2; outline: none; background: var(--ds-active-bg); transform: translateY(-1px); }
.entry-button:active { transform: scale(.96); transition-duration: 70ms; }
.entry-name { position: absolute; top: calc(100% + 4px); left: 50%; z-index: 3; max-width: 180px; border: 1px solid var(--ds-border-default); border-radius: 2px; padding: 3px 7px; background: var(--ds-bg-panel-elevated); color: var(--ds-text-primary); font-size: 12px; line-height: 1.5; opacity: 0; visibility: hidden; transform: translateX(-50%); pointer-events: none; white-space: nowrap; }
.entry-button:hover .entry-name, .entry-button:focus-visible .entry-name { opacity: 1; visibility: visible; }
.entry-kind { display: block; color: var(--ds-text-secondary); font-size: 11px; }
.with-names { grid-template-columns: repeat(auto-fill, minmax(84px, 1fr)); gap: 6px; }
.with-names .entry-button { justify-content: flex-start; padding: 8px 4px 6px; }
.with-names .entry-name { position: static; width: 100%; max-width: 100%; border: 0; padding: 4px 2px 0; background: transparent; line-height: 1.4; opacity: 1; visibility: visible; transform: none; white-space: normal; overflow-wrap: anywhere; }
@container encyclopedia (max-width: 600px) { .entry-section { padding: 0 12px 12px; } .section-heading { margin-inline: -12px; padding-inline: 12px 16px; } .entry-grid { grid-template-columns: repeat(auto-fill, minmax(46px, 1fr)); } .with-names { grid-template-columns: repeat(auto-fill, minmax(76px, 1fr)); } }
@media (prefers-reduced-motion: reduce) { .entry-button { transition: none; } }
</style>
