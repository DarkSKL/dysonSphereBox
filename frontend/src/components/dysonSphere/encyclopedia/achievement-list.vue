<script setup lang="ts">
import {Icon} from '@iconify/vue'
import trophy from '@iconify-icons/game-icons/trophy-cup'
import type {EncyclopediaAchievement, EncyclopediaEntry, EncyclopediaKey} from '../../../types/encyclopedia'
import EntryIcon from './entry-icon.vue'

type ResolvedAchievement = EncyclopediaAchievement & {entries: {key: EncyclopediaKey; entry: EncyclopediaEntry}[]}
defineProps<{achievements: readonly ResolvedAchievement[]; total: number}>()
const emit = defineEmits<{(event: 'open', key: EncyclopediaKey): void}>()
</script>

<template>
  <!-- ~ 成就列表 -->
  <!-- * 说明：单一切角面板按编号连续展示成就，标题栏与实体分区同构，不分页、不拆分区块 -->
  <!-- * 交互：标题栏右侧 actions 插槽放置分组选择与搜索；关联图标打开图鉴详情 -->
  <!-- * 依赖：已筛选的成就与已加载的 catalog 实体 -->
  <!-- * 边界：不读取 Steam 或存档解锁状态，标记只说明前置条件 -->
  <section class="achievement-section" aria-label="成就">
    <header class="section-heading">
      <div class="section-title"><Icon :icon="trophy" :width="18" :height="18" aria-hidden="true" /><h2>成就</h2><span class="section-count">{{ total }}</span></div>
      <div v-if="$slots.actions" class="section-actions"><slot name="actions" /></div>
    </header>
    <ol v-if="achievements.length" class="achievement-list">
      <li v-for="achievement in achievements" :key="achievement.id" class="achievement-row">
        <span class="achievement-index">{{ String(achievement.id).padStart(3, '0') }}</span>
        <div class="achievement-body">
          <div class="achievement-title"><h3>{{ achievement.name }}</h3><span v-if="achievement.combat" class="achievement-flag is-combat">黑雾模式</span><span v-if="achievement.noMetadata" class="achievement-flag">不可使用元数据</span></div>
          <p class="achievement-condition">{{ achievement.condition }}</p>
          <p v-if="achievement.tip" class="achievement-tip">{{ achievement.tip }}</p>
        </div>
        <div v-if="achievement.entries.length" class="achievement-related">
          <button v-for="item in achievement.entries" :key="item.key" type="button" class="related-button ds-bracket-hover" :title="item.entry.name.zh" :aria-label="`查看${item.entry.name.zh}`" @click="emit('open', item.key)"><EntryIcon :entry="item.entry" :size="28" /></button>
        </div>
      </li>
    </ol>
    <p v-else class="achievement-empty">没有匹配的成就，换个关键词或分组试试</p>
  </section>
</template>

<style scoped lang="scss">
.achievement-section { --chamfer: 12px; position: relative; min-width: 0; isolation: isolate; }
.achievement-section::before, .achievement-section::after { content: ''; position: absolute; z-index: -1; clip-path: polygon(0 0, calc(100% - var(--chamfer)) 0, 100% var(--chamfer), 100% 100%, var(--chamfer) 100%, 0 calc(100% - var(--chamfer))); pointer-events: none; }
.achievement-section::before { inset: 0; background: var(--ds-border-default); }
.achievement-section::after { inset: 1px; background: var(--ds-bg-panel); opacity: .72; }
.section-heading { position: relative; display: flex; height: 44px; align-items: center; justify-content: space-between; gap: 12px; border-bottom: 1px solid var(--ds-border-subtle); padding: 0 20px 0 16px; }
.section-title { display: flex; min-width: 0; align-items: center; gap: 8px; color: var(--ds-text-primary); }
.section-title > svg { color: var(--ds-text-secondary); }
.section-title h2 { margin: 0; font-size: 15px; font-weight: 700; line-height: 1.4; }
.section-count { color: var(--ds-text-muted); font-family: var(--ds-font-display); font-size: 12px; }
.section-actions { display: flex; min-width: 0; flex: 0 1 auto; align-items: center; gap: 8px; }
.achievement-empty { margin: 0; padding: 40px 16px; color: var(--ds-text-muted); font-size: 13px; text-align: center; }
.achievement-list { margin: 0; padding: 4px 0; list-style: none; }
.achievement-row { display: grid; grid-template-columns: 40px minmax(0, 1fr) auto; align-items: center; gap: 12px; padding: 10px 16px; transition: background-color 150ms; }
.achievement-row + .achievement-row { border-top: 1px solid var(--ds-border-subtle); }
.achievement-row:hover { background: var(--ds-hover-bg); }
.achievement-index { align-self: start; padding-top: 1px; color: var(--ds-text-muted); font-family: var(--ds-font-display); font-size: 12px; letter-spacing: .04em; }
.achievement-body { min-width: 0; }
.achievement-title { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 8px; }
.achievement-title h3 { margin: 0; color: var(--ds-text-primary); font-size: 14px; font-weight: 600; line-height: 1.5; }
.achievement-flag { border: 1px solid var(--ds-border-default); border-radius: 2px; padding: 0 5px; color: var(--ds-text-secondary); font-size: 11px; line-height: 18px; white-space: nowrap; }
.achievement-flag.is-combat { border-color: var(--ds-error-border); background: var(--ds-error-bg); color: var(--ds-error); }
.achievement-condition { margin: 2px 0 0; color: var(--ds-text-secondary); font-size: 13px; line-height: 1.6; }
.achievement-tip { margin: 2px 0 0; color: var(--ds-text-muted); font-size: 12px; line-height: 1.6; }
.achievement-related { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 4px; max-width: 180px; }
.related-button { display: inline-flex; align-items: center; justify-content: center; width: 34px; height: 34px; border: 1px solid transparent; border-radius: 2px; padding: 0; background: var(--ds-hover-bg); cursor: pointer; transition: border-color 150ms, background-color 150ms; }
.related-button { --bracket-size: 8px; --bracket-color: var(--ds-brand); }
.related-button:hover, .related-button:focus-visible { background: var(--ds-active-bg); outline: none; }
@container encyclopedia (max-width: 600px) { .section-heading { padding-inline: 12px 16px; } .achievement-row { grid-template-columns: 32px minmax(0, 1fr); padding: 10px 12px; } .achievement-related { grid-column: 2; justify-content: flex-start; max-width: none; } }
@media (prefers-reduced-motion: reduce) { .achievement-row, .related-button { transition: none; } }
</style>
