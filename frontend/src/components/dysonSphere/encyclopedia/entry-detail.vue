<script setup lang="ts">
import {ArrowLeft, ArrowUpRight, ChevronRight} from '@lucide/vue'
import {computed, ref} from 'vue'
import {BrowserOpenURL} from '../../../../wailsjs/runtime/runtime'
import {encyclopediaManifest, resolveCatalogEntry} from '../../../services/encyclopedia/catalog'
import {encyclopediaKindLabels} from '../../../services/encyclopedia/categories'
import {formatProperty} from '../../../services/encyclopedia/catalog-core'
import type {EncyclopediaEntry, EncyclopediaGuide, EncyclopediaKey, EncyclopediaRelationType} from '../../../types/encyclopedia'
import EntryIcon from './entry-icon.vue'
import RecipePanel from './recipe-panel.vue'

const props = defineProps<{show: boolean; entry?: EncyclopediaEntry; guide?: EncyclopediaGuide; canGoBack: boolean}>()
const emit = defineEmits<{(event: 'close'): void; (event: 'back'): void; (event: 'open', key: EncyclopediaKey): void; (event: 'navigate', path: string): void}>()
// =============== 属性与关联分区 ===============
// 说明：关系分组只渲染已解析实体；详情内跳转继续使用父级 URL 入口，不维护另一份数据。
const expanded = ref(new Set<string>())
const relationLabels: Record<EncyclopediaRelationType, string> = {ingredient: '研究材料', product: '产出', producer: '制作设备', prerequisite: '前置科技', unlocks: '解锁配方', recipe: '生产配方', usedBy: '用于制作', researchedBy: '研究配方', obtainedFrom: '采集来源'}
const recipes = computed(() => props.entry?.kind === 'recipe' ? [props.entry] : (props.entry?.relations ?? [])
  .filter(relation => relation.type === 'recipe').map(relation => resolveCatalogEntry(relation.target)).filter((entry): entry is EncyclopediaEntry => Boolean(entry)))
const groups = computed(() => {
  const skip = props.entry?.kind === 'recipe' ? ['ingredient', 'product', 'recipe'] : ['recipe', 'researchedBy']
  return (Object.keys(relationLabels) as EncyclopediaRelationType[]).filter(type => !skip.includes(type)).map(type => ({
    type, title: relationLabels[type], items: (props.entry?.relations ?? []).filter(relation => relation.type === type)
      .map(relation => ({...relation, entry: resolveCatalogEntry(relation.target)})).filter(relation => Boolean(relation.entry)),
  })).filter(group => group.items.length)
})
const guideEntries = computed(() => (props.guide?.related ?? []).map(key => ({key, entry: resolveCatalogEntry(key)})).filter(item => item.entry))
function toggleGroup(type: string): void {
  const next = new Set(expanded.value)
  next.has(type) ? next.delete(type) : next.add(type)
  expanded.value = next
}
function openWiki(): void {
  BrowserOpenURL('https://wiki.biligame.com/dsp/%E9%A6%96%E9%A1%B5')
}
</script>

<template>
  <!-- ~ 图鉴详情抽屉 -->
  <!-- * 说明：同一位置查看基础属性、配方流程、科技前置和离线阅读内容 -->
  <!-- * 交互：关联按钮在抽屉内跳转；关闭与返回交给父级维护 URL 和筛选上下文 -->
  <!-- * 依赖：统一 catalog 与已校验的实体，不解析外部 HTML -->
  <!-- * 边界：缺失属性不伪造数值；外部 Wiki 仅在用户明确点击后打开 -->
  <n-drawer :show="show" :width="'min(560px, calc(100vw - 24px))'" placement="right" @update:show="value => !value && emit('close')">
    <n-drawer-content closable :native-scrollbar="false" body-content-style="padding: 24px;">
      <template #header><div class="detail-bar"><button v-if="canGoBack" type="button" aria-label="返回上一条目" class="back-button" @click="emit('back')"><ArrowLeft :size="18" /></button><span>{{ entry ? `${encyclopediaKindLabels[entry.kind]}详情` : '图鉴阅读' }}</span></div></template>
      <template v-if="entry">
        <div class="detail-identity"><EntryIcon :entry="entry" :size="76" /><div><p class="detail-kind">{{ entry.group }}</p><h2>{{ entry.name.zh }}</h2><p v-if="entry.name.en" class="detail-en">{{ entry.name.en }}</p></div></div>
        <p v-if="entry.description" class="detail-description">{{ entry.description }}</p>
        <dl v-if="entry.properties.length" class="detail-properties"><div v-for="property in entry.properties" :key="property.label"><dt>{{ property.label }}</dt><dd>{{ formatProperty(property) }}</dd></div></dl>
        <section v-if="recipes.length" class="detail-section"><h3 class="detail-section-heading">{{ entry.kind === 'recipe' ? '配方流程' : '生产配方' }}<span>{{ recipes.length }}</span></h3><div class="recipe-list"><RecipePanel v-for="recipe in recipes.slice(0, expanded.has('recipes') ? undefined : 4)" :key="recipe.id" :recipe="recipe" @open="key => emit('open', key)" /></div><button v-if="recipes.length > 4" type="button" class="expand-button" @click="toggleGroup('recipes')">{{ expanded.has('recipes') ? '收起配方' : `查看全部 ${recipes.length} 个配方` }}</button></section>
        <section v-for="group in groups" :key="group.type" class="detail-section"><h3 class="detail-section-heading">{{ group.title }}<span>{{ group.items.length }}</span></h3><div class="relation-list"><template v-for="relation in group.items.slice(0, expanded.has(group.type) ? undefined : 12)" :key="relation.target"><button v-if="relation.entry" type="button" class="relation-button" @click="emit('open', relation.target)"><EntryIcon :entry="relation.entry" :size="29" /><span>{{ relation.entry.name.zh }}</span><strong v-if="relation.quantity">× {{ relation.quantity }}</strong><ChevronRight :size="14" /></button></template></div><button v-if="group.items.length > 12" type="button" class="expand-button" @click="toggleGroup(group.type)">{{ expanded.has(group.type) ? '收起' : `查看全部 ${group.items.length} 项` }}</button></section>
        <details class="data-source"><summary>数据来源与版本</summary><dl><dt>来源</dt><dd>{{ entry.source }} · MIT</dd><dt>游戏版本</dt><dd>{{ entry.gameVersion }}</dd><dt>实体 ID</dt><dd>{{ entry.kind }}:{{ entry.id }}</dd><dt>数据集</dt><dd>{{ encyclopediaManifest.datasetVersion }}</dd></dl></details>
      </template>
      <template v-else-if="guide">
        <div class="guide-detail"><h2>{{ guide.title }}</h2><p class="guide-summary">{{ guide.summary }}</p><p v-for="paragraph in guide.paragraphs" :key="paragraph" class="guide-paragraph">{{ paragraph }}</p></div>
        <section v-if="guideEntries.length" class="detail-section"><h3 class="detail-section-heading">继续查阅</h3><div class="relation-list"><template v-for="item in guideEntries" :key="item.key"><button v-if="item.entry" type="button" class="relation-button" @click="emit('open', item.key)"><EntryIcon :entry="item.entry" :size="30" /><span>{{ item.entry.name.zh }}</span><ChevronRight :size="14" /></button></template></div></section>
        <n-button v-if="guide.to" type="primary" class="guide-action" @click="emit('navigate', guide.to)">{{ guide.action }}</n-button>
        <button type="button" class="wiki-link" @click="openWiki">前往 BWIKI 查阅更多<ArrowUpRight :size="15" /></button>
      </template>
    </n-drawer-content>
  </n-drawer>
</template>

<style scoped lang="scss">
.detail-bar { display: flex; align-items: center; gap: 10px; font-size: 16px; }
.back-button { display: inline-flex; border: 0; padding: 4px; background: transparent; color: var(--ds-text-primary); cursor: pointer; }
.detail-identity { display: flex; align-items: center; gap: 17px; margin-bottom: 22px; }
.detail-identity > div { min-width: 0; }
.detail-kind { margin: 0 0 5px; color: var(--ds-text-secondary); font-size: 13px; }
h2 { margin: 0; color: var(--ds-text-primary); font-size: 24px; font-weight: 700; line-height: 1.4; overflow-wrap: anywhere; }
.detail-en { margin: 5px 0 0; color: var(--ds-text-secondary); font-size: 13px; overflow-wrap: anywhere; }
.detail-description, .guide-paragraph { color: var(--ds-text-secondary); font-size: 14px; line-height: 1.85; }
.detail-properties { margin: 0; border-top: 1px solid var(--ds-border-default); }
.detail-properties > div { display: flex; justify-content: space-between; gap: 16px; border-bottom: 1px solid var(--ds-border-subtle); padding: 11px 0; font-size: 14px; }
.detail-properties dt { color: var(--ds-text-secondary); }
.detail-properties dd { margin: 0; color: var(--ds-text-primary); font-family: var(--ds-font-display); font-weight: 600; text-align: right; }
.detail-section { margin-top: 28px; }
.detail-section-heading { display: flex; align-items: center; gap: 9px; margin: 0 0 13px; color: var(--ds-text-primary); font-size: 15px; font-weight: 700; }
.detail-section-heading > span { color: var(--ds-text-secondary); font-family: var(--ds-font-display); font-size: 13px; font-weight: 400; }
.recipe-list { display: grid; gap: 12px; }
.relation-list { display: flex; flex-wrap: wrap; gap: 8px; }
.relation-button { display: inline-flex; min-width: 0; align-items: center; gap: 7px; border: 1px solid var(--ds-border-default); border-radius: 3px; padding: 6px 8px; background: transparent; color: var(--ds-text-primary); font-size: 13px; cursor: pointer; transition: background-color 150ms, border-color 150ms; }
.relation-button > span { overflow-wrap: anywhere; }
.relation-button > strong { flex-shrink: 0; font-family: var(--ds-font-display); font-weight: 500; }
.relation-button > svg { flex-shrink: 0; color: var(--ds-text-secondary); }
.relation-button:hover, .relation-button:focus-visible { border-color: var(--ds-border-strong); background: var(--ds-active-bg); outline: none; }
.expand-button { border: 0; padding: 12px 0 0; background: transparent; color: var(--ds-brand); font-size: 13px; cursor: pointer; }
.data-source { margin-top: 30px; border-top: 1px solid var(--ds-border-default); padding-top: 16px; color: var(--ds-text-secondary); font-size: 13px; }
.data-source summary { cursor: pointer; }
.data-source dl { display: grid; grid-template-columns: 65px minmax(0, 1fr); gap: 9px; margin: 15px 0 0; }
.data-source dd { margin: 0; font-family: var(--ds-font-mono); font-size: 12px; overflow-wrap: anywhere; }
.guide-summary { margin: 16px 0 24px; color: var(--ds-text-primary); font-size: 16px; line-height: 1.7; }
.guide-paragraph { margin: 0 0 18px; }
.guide-action { margin-top: 28px; }
.wiki-link { display: flex; align-items: center; gap: 6px; margin-top: 30px; border: 0; padding: 0; background: transparent; color: var(--ds-text-secondary); font-size: 13px; cursor: pointer; }
</style>
