<script setup lang="ts">
import {Icon} from '@iconify/vue'
import {ArrowLeft, ArrowRight, ChevronRight, Eye, EyeOff, Info, RotateCcw, Search, X} from '@lucide/vue'
import {computed, ref} from 'vue'
import {useRouter} from 'vue-router'
import CategoryGrid from '../../components/dysonSphere/encyclopedia/category-grid.vue'
import EntryDetail from '../../components/dysonSphere/encyclopedia/entry-detail.vue'
import EntrySection from '../../components/dysonSphere/encyclopedia/entry-section.vue'
import {useEncyclopediaState} from '../../composables/dysonSphere/useEncyclopediaState'
import {encyclopediaManifest} from '../../services/encyclopedia/catalog'
import {encyclopediaCategories, encyclopediaKindLabels} from '../../services/encyclopedia/categories'

definePage({meta: {layout: 'main', title: '游戏图鉴'}})

// =============== 图鉴浏览状态 ===============
// 说明：页面负责布局，搜索、加载、关系与路由同步由 composable 统一协调。
const router = useRouter()
const state = useEncyclopediaState()
const {view, loading, error, matches, pageEntries, overview, overviewSections, pageCount, tagOptions, guides, selectedEntry, selectedGuide, drawerOpen, canGoBack, animateEntrance} = state
const sourceOpen = ref(false)
const total = Object.values(encyclopediaManifest.counts).reduce((sum, count) => sum + count, 0)
const category = computed(() => encyclopediaCategories.find(category => category.key === view.category))
const resultTitle = computed(() => view.search.trim() ? '搜索结果' : (view.category === 'overview' ? '全部条目' : category.value?.label ?? '图鉴'))
const resultKind = computed(() => pageEntries.value[0]?.kind ?? 'building')
function navigate(path: string): void { void router.push(path) }
</script>

<template>
  <!-- ~ 游戏图鉴 -->
  <!-- * 说明：保留应用外壳，以十分类和切角图标索引承接用户提供的 Wiki 视觉 -->
  <!-- * 交互：关键词搜索、分类与分组筛选、名称开关、详情及相关实体跳转 -->
  <!-- * 依赖：离线 catalog 和 useEncyclopediaState，不依赖账号或游戏目录 -->
  <!-- * 边界：加载、失败与空结果分开处理；数据来源窗口说明当前数据版本 -->
  <div class="encyclopedia-page">
    <header class="encyclopedia-header">
      <div class="encyclopedia-heading"><h1>游戏图鉴</h1><button type="button" class="dataset-button" aria-label="查看图鉴数据来源" @click="sourceOpen = true"><span class="dataset-status" aria-hidden="true" />{{ encyclopediaManifest.gameVersion }}<Info :size="14" /></button></div>
      <n-input v-model:value="view.search" class="encyclopedia-search" placeholder="搜索物品、建筑、科技…" clearable aria-label="搜索图鉴" @keyup.enter="state.commitSearch">
        <template #prefix><Search :size="17" /></template>
      </n-input>
    </header>

    <CategoryGrid :active="view.category" :animate="animateEntrance" @select="state.selectCategory" />

    <!-- ~ 目录工具栏 -->
    <!-- * 说明：总览保持图标矩阵，筛选后提供完整结果与分页 -->
    <!-- * 交互：清空回到总览；名称开关保留在当前页面会话；分组条件与关键词共同生效 -->
    <!-- * 依赖：分类、搜索结果、标签选项和数据加载状态 -->
    <!-- * 边界：加载期间不把未就绪数据标记为空结果 -->
    <div class="catalog-toolbar">
      <div class="catalog-context"><button v-if="!overview" type="button" class="reset-button" aria-label="返回全部图鉴" @click="state.resetFilters"><ArrowLeft :size="17" /></button><span v-if="!overview">{{ resultTitle }}</span><span class="catalog-total">{{ overview ? `${total} 个条目` : `${matches.length} 个结果` }}</span></div>
      <div class="catalog-actions"><n-select v-if="!overview && tagOptions.length" :value="view.tag" class="group-select" :options="[{label: '全部分组', value: ''}, ...tagOptions]" size="small" aria-label="筛选图鉴分组" @update:value="state.selectTag" /><button type="button" class="names-button" :class="{'is-active': view.showNames}" :aria-pressed="view.showNames" @click="view.showNames = !view.showNames"><Eye v-if="!view.showNames" :size="16" /><EyeOff v-else :size="16" /><span>{{ view.showNames ? '隐藏名称' : '显示名称' }}</span></button></div>
    </div>

    <div v-if="error" class="catalog-state" role="alert"><Info :size="32" /><h2>图鉴暂时无法加载</h2><p>{{ error }}</p><n-button @click="state.retry"><template #icon><RotateCcw :size="16" /></template>重新加载</n-button></div>
    <div v-else-if="loading && !overviewSections[0]?.entries.length" class="catalog-skeleton" aria-label="正在加载图鉴" aria-busy="true"><n-skeleton height="280px" :sharp="false" /><n-skeleton height="220px" :sharp="false" /></div>
    <div v-else class="catalog-content" :class="{'is-first-visit': animateEntrance}">
      <template v-if="overview"><EntrySection v-for="section in overviewSections" :key="section.kind" :kind="section.kind" :title="encyclopediaKindLabels[section.kind]" :entries="section.entries" :total="section.total" :show-names="view.showNames" expandable @open="state.openEntry" @expand="state.showKind(section.kind)" /><button class="all-recipes-button" type="button" @click="state.selectCategory('gameplay')"><span>生产配方</span><span class="recipe-total">{{ encyclopediaManifest.counts.recipe }} 个配方</span><ArrowRight :size="18" /></button></template>
      <template v-else>
        <div v-if="guides.length" class="guide-list"><button v-for="guide in guides" :key="guide.id" type="button" class="guide-row" :data-guide="guide.id" @click="state.openGuide(guide.id)"><Icon v-if="category" :icon="category.icon" :width="32" :height="32" aria-hidden="true" /><div><h2>{{ guide.title }}</h2><p>{{ guide.summary }}</p></div><ChevronRight :size="18" /></button></div>
        <EntrySection v-if="pageEntries.length" :title="resultTitle" :kind="resultKind" :entries="pageEntries" :total="matches.length" :show-names="view.showNames || Boolean(view.search.trim())" :show-kinds="Boolean(view.search.trim())" @open="state.openEntry" />
        <div v-if="!loading && !pageEntries.length && !guides.length" class="catalog-state"><Search :size="34" /><h2>没有找到相关条目</h2><p>试试其他名称、英文名或 ID，也可以清空当前条件。</p><n-button @click="state.resetFilters"><template #icon><X :size="15" /></template>清空筛选</n-button></div>
        <div v-if="pageCount > 1" class="catalog-pagination"><span>每页 60 个条目</span><n-pagination :page="view.page" :page-count="pageCount" :page-slot="5" size="small" @update:page="state.selectPage" /></div>
      </template>
    </div>

    <footer class="catalog-footer"><span>图鉴数据可离线查阅</span><button type="button" @click="sourceOpen = true">数据来源与版本<ChevronRight :size="13" /></button></footer>

    <EntryDetail :show="drawerOpen" :entry="selectedEntry" :guide="selectedGuide" :can-go-back="canGoBack" @close="state.closeDetail" @back="state.backDetail" @open="state.openEntry" @navigate="navigate" />

    <!-- ~ 数据来源 -->
    <!-- * 说明：提供数据版本、范围、维护来源及图标包边界 -->
    <!-- * 交互：通过标题或页脚打开，关闭后保留目录位置 -->
    <!-- * 依赖：已校验的本地 manifest -->
    <!-- * 边界：不宣称是最新游戏数据，不自动打开第三方页面 -->
    <n-modal v-model:show="sourceOpen" preset="card" title="图鉴数据来源" class="encyclopedia-source-modal" style="width: min(520px, calc(100vw - 32px))">
      <dl class="source-properties"><dt>游戏版本</dt><dd>{{ encyclopediaManifest.gameVersion }}</dd><dt>数据集</dt><dd>{{ encyclopediaManifest.datasetVersion }}</dd><dt>数据来源</dt><dd>FactorioLab · MIT</dd><dt>收录范围</dt><dd>{{ encyclopediaManifest.counts.building }} 建筑 · {{ encyclopediaManifest.counts.item }} 物品 · {{ encyclopediaManifest.counts.recipe }} 配方 · {{ encyclopediaManifest.counts.technology }} 科技 · {{ encyclopediaManifest.counts.resource }} 资源</dd><dt>上游提交</dt><dd>{{ encyclopediaManifest.sources[0]?.commit }}</dd></dl>
      <p class="source-note">数值来自固定版本的公开数据集，不包含存档加成。分类布局参考戴森球计划 BWIKI；离线阅读内容由本模块编写。</p><p class="source-note">游戏图标属于可替换资源包，缺失时使用 Game Icons 类别图标。游戏素材、名称与商标归原权利人所有。</p>
    </n-modal>
  </div>
</template>

<style scoped lang="scss">
.encyclopedia-page { container: encyclopedia / inline-size; min-height: 100%; box-sizing: border-box; padding: 26px var(--ds-page-padding-x) 28px; }
.encyclopedia-header { display: flex; align-items: center; justify-content: space-between; gap: 20px; margin-bottom: 24px; }
.encyclopedia-heading { display: flex; min-width: 0; flex-wrap: wrap; align-items: center; gap: 14px; }
h1 { margin: 0; color: var(--ds-text-primary); font-size: 24px; font-weight: 700; line-height: 1.4; }
.dataset-button { display: inline-flex; align-items: center; gap: 6px; border: 0; padding: 5px 0; background: transparent; color: var(--ds-text-secondary); font-family: var(--ds-font-mono); font-size: 12px; cursor: pointer; }
.dataset-status { width: 5px; height: 5px; border-radius: 50%; background: var(--ds-success); }
.encyclopedia-search { width: 310px; flex: 0 1 310px; }
.encyclopedia-search :deep(.n-input__placeholder) { color: var(--ds-text-secondary); }
.catalog-toolbar { display: flex; min-height: 30px; align-items: center; justify-content: space-between; gap: 12px; margin: 14px 0 11px; }
.catalog-context, .catalog-actions { display: flex; min-width: 0; align-items: center; gap: 9px; }
.catalog-context { color: var(--ds-text-primary); font-size: 14px; }
.catalog-context > svg { color: var(--ds-text-secondary); }
.catalog-total { color: var(--ds-text-secondary); font-family: var(--ds-font-display); font-size: 13px; white-space: nowrap; }
.names-button, .reset-button { display: inline-flex; flex-shrink: 0; align-items: center; justify-content: center; gap: 6px; border: 0; border-radius: 3px; padding: 5px 7px; background: transparent; color: var(--ds-text-secondary); font-size: 13px; cursor: pointer; }
.names-button:hover, .names-button.is-active, .reset-button:hover { background: var(--ds-hover-bg); color: var(--ds-text-primary); }
.group-select { width: 138px; }
.catalog-content, .catalog-skeleton { display: grid; gap: 22px; }
.catalog-content.is-first-visit { animation: catalog-arrive 250ms both; }
.all-recipes-button { display: flex; align-items: center; gap: 12px; border: 1px solid var(--ds-border-default); border-radius: 3px; padding: 17px 22px; background: transparent; color: var(--ds-text-primary); font-size: 16px; cursor: pointer; }
.all-recipes-button:hover { border-color: var(--ds-border-strong); background: var(--ds-hover-bg); }
.all-recipes-button > svg { margin-left: auto; }
.recipe-total { color: var(--ds-text-secondary); font-family: var(--ds-font-display); font-size: 13px; }
.guide-list { border-top: 1px solid var(--ds-border-default); }
.guide-row { display: flex; width: 100%; align-items: center; gap: 16px; border: 0; border-bottom: 1px solid var(--ds-border-default); padding: 22px 12px; background: transparent; color: var(--ds-text-primary); text-align: left; cursor: pointer; transition: background-color 160ms; }
.guide-row > svg { flex: 0 0 auto; }
.guide-row > div { min-width: 0; flex: 1; }
.guide-row h2 { margin: 0 0 5px; font-size: 16px; line-height: 1.5; }
.guide-row p { margin: 0; color: var(--ds-text-secondary); font-size: 14px; line-height: 1.7; }
.guide-row:hover { background: var(--ds-hover-bg); }
.catalog-state { display: flex; min-height: 260px; flex-direction: column; align-items: center; justify-content: center; gap: 13px; padding: 30px 20px; color: var(--ds-text-secondary); text-align: center; }
.catalog-state h2 { margin: 0; color: var(--ds-text-primary); font-size: 18px; }
.catalog-state p { max-width: 360px; margin: 0 0 6px; font-size: 14px; line-height: 1.7; }
.catalog-pagination { display: flex; align-items: center; justify-content: space-between; gap: 12px; color: var(--ds-text-secondary); font-size: 13px; }
.catalog-footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-top: 30px; border-top: 1px solid var(--ds-border-subtle); padding-top: 18px; color: var(--ds-text-secondary); font-size: 13px; }
.catalog-footer button { display: flex; align-items: center; gap: 4px; border: 0; padding: 0; background: transparent; color: inherit; font-size: inherit; cursor: pointer; }
.source-properties { display: grid; grid-template-columns: 70px minmax(0, 1fr); gap: 13px 15px; margin: 0 0 22px; font-size: 14px; }
.source-properties dt { color: var(--ds-text-secondary); }
.source-properties dd { margin: 0; overflow-wrap: anywhere; }
.source-note { margin: 12px 0 0; color: var(--ds-text-secondary); font-size: 14px; line-height: 1.8; }
@keyframes catalog-arrive { from { opacity: 0; transform: translateY(6px); } to { opacity: 1; transform: translateY(0); } }
@container encyclopedia (max-width: 650px) { .encyclopedia-header { align-items: flex-start; flex-direction: column; gap: 14px; margin-bottom: 20px; } .encyclopedia-search { width: 100%; flex-basis: auto; } .encyclopedia-heading { justify-content: space-between; width: 100%; } .catalog-toolbar { flex-wrap: wrap; } .catalog-actions { margin-left: auto; } }
@media (max-width: 700px) { .encyclopedia-page { padding: 20px 16px 26px; } .catalog-context { gap: 6px; } .catalog-total { font-size: 12px; } .group-select { width: 115px; } .names-button { padding-inline: 3px; } .names-button span { display: none; } }
@media (prefers-reduced-motion: reduce) { .catalog-content.is-first-visit { animation: none; } .guide-row { transition: none; } }
</style>
