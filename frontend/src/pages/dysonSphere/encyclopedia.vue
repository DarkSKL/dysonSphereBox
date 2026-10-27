<script setup lang="ts">
import {Icon} from '@iconify/vue'
import {ChevronRight, Eye, EyeOff, Info, RotateCcw, Search, X} from '@lucide/vue'
import {computed, ref} from 'vue'
import {useRouter} from 'vue-router'
import AchievementList from '../../components/dysonSphere/encyclopedia/achievement-list.vue'
import CategoryGrid from '../../components/dysonSphere/encyclopedia/category-grid.vue'
import EntryDetail from '../../components/dysonSphere/encyclopedia/entry-detail.vue'
import EntrySection from '../../components/dysonSphere/encyclopedia/entry-section.vue'
import {useEncyclopediaState} from '../../composables/dysonSphere/useEncyclopediaState'
import {encyclopediaAchievements} from '../../data/encyclopedia/achievements'
import {encyclopediaManifest} from '../../services/encyclopedia/catalog'
import {encyclopediaCategories} from '../../services/encyclopedia/categories'

definePage({meta: {layout: 'main', title: '游戏图鉴'}})

// =============== 图鉴浏览状态 ===============
// 说明：页面负责布局，搜索、加载、关系与路由同步由 composable 统一协调。
const router = useRouter()
const state = useEncyclopediaState()
const {view, searchKeyword, loading, error, matches, searching, achievements, pageEntries, pageCount, tagOptions, guides, selectedEntry, selectedGuide, drawerOpen, canGoBack, animateEntrance} = state
const sourceOpen = ref(false)
const category = computed(() => encyclopediaCategories.find(category => category.key === view.category))
const resultTitle = computed(() => searching.value ? '搜索结果' : category.value?.label ?? '图鉴')
const isAchievement = computed(() => view.category === 'achievement')
const resultKind = computed(() => pageEntries.value[0]?.kind ?? 'building')
function navigate(path: string): void { void router.push(path) }
</script>

<template>
  <!-- ~ 游戏图鉴 -->
  <!-- * 说明：保留应用外壳，以切角指令栏和分层图标面板组织五个分类，尺寸与盒子其他工作区一致 -->
  <!-- * 交互：关键词搜索、分类与分组筛选、名称开关、详情及相关实体跳转 -->
  <!-- * 依赖：离线 catalog 和 useEncyclopediaState，不依赖账号或游戏目录 -->
  <!-- * 边界：加载、失败与空结果分开处理；数据来源窗口说明当前数据版本 -->
  <div class="encyclopedia-page">
    <header class="encyclopedia-header">
      <div class="encyclopedia-heading"><h1>游戏图鉴</h1></div>
      <n-input v-model:value="searchKeyword" class="encyclopedia-search encyclopedia-field" :placeholder="isAchievement ? '搜索成就名称、条件或编号…' : '搜索物品、建筑、科技…'" clearable :aria-label="isAchievement ? '搜索成就' : '搜索图鉴'" @keyup.enter="state.commitSearch">
        <template #prefix><Search :size="15" /></template>
      </n-input>
    </header>

    <CategoryGrid :active="view.category" :animate="animateEntrance" @select="state.selectCategory" />

    <div v-if="error" class="catalog-state" role="alert"><Info :size="32" /><h2>图鉴暂时无法加载</h2><p>{{ error }}</p><n-button @click="state.retry"><template #icon><RotateCcw :size="16" /></template>重新加载</n-button></div>
    <div v-else-if="loading && !isAchievement && !pageEntries.length" class="catalog-skeleton" aria-label="正在加载图鉴" aria-busy="true"><n-skeleton height="220px" :sharp="false" /><n-skeleton height="180px" :sharp="false" /></div>
    <div v-else class="catalog-content" :class="{'is-first-visit': animateEntrance}">
        <div v-if="guides.length" class="guide-list"><button v-for="guide in guides" :key="guide.id" type="button" class="guide-row" :data-guide="guide.id" @click="state.openGuide(guide.id)"><Icon v-if="category" :icon="category.icon" :width="24" :height="24" aria-hidden="true" /><div><h2>{{ guide.title }}</h2><p>{{ guide.summary }}</p></div><ChevronRight :size="15" /></button></div>
        <AchievementList v-if="isAchievement" :achievements="achievements" :total="achievements.length" @open="state.openEntry">
          <template #actions>
            <n-input v-model:value="view.achievementSearch" class="section-search encyclopedia-field" size="small" placeholder="搜索名称、条件或编号" clearable aria-label="搜索成就" @keyup.enter="state.commitSearch">
              <template #prefix><Search :size="14" /></template>
            </n-input>
            <n-select :value="view.tag" class="group-select encyclopedia-field" :options="[{label: '全部分组', value: ''}, ...tagOptions]" size="small" aria-label="筛选成就分组" @update:value="state.selectTag" />
          </template>
        </AchievementList>
        <!-- ~ 分区控件 -->
        <!-- * 说明：分组选择与名称开关位于分区右上角，与成就分区共用同一组控件尺寸，数量由分区标题显示 -->
        <!-- * 交互：名称开关保留在当前页面会话；关键词跨分类搜索时隐藏分组筛选 -->
        <!-- * 依赖：标签选项与名称开关状态 -->
        <!-- * 边界：搜索结果始终显示名称与类型，名称开关不改变该行为 -->
        <EntrySection v-if="!isAchievement && pageEntries.length" :title="resultTitle" :kind="resultKind" :entries="pageEntries" :total="matches.length" :show-names="view.showNames || searching" :show-kinds="searching" @open="state.openEntry">
          <template #actions>
            <n-select v-if="!searching && tagOptions.length" :value="view.tag" class="group-select encyclopedia-field" :options="[{label: '全部分组', value: ''}, ...tagOptions]" size="small" aria-label="筛选图鉴分组" @update:value="state.selectTag" />
            <n-tooltip v-if="!searching"><template #trigger><button type="button" class="names-button" :class="{'is-active': view.showNames}" :aria-pressed="view.showNames" :aria-label="view.showNames ? '隐藏名称' : '显示名称'" @click="view.showNames = !view.showNames"><EyeOff v-if="view.showNames" :size="15" /><Eye v-else :size="15" /></button></template>{{ view.showNames ? '隐藏名称' : '显示名称' }}</n-tooltip>
          </template>
        </EntrySection>
        <div v-if="!loading && !isAchievement && !pageEntries.length && !guides.length" class="catalog-state"><Search :size="34" /><h2>没有找到相关条目</h2><p>试试其他名称、英文名或 ID，也可以清空当前条件。</p><n-button @click="state.resetFilters"><template #icon><X :size="15" /></template>清空筛选</n-button></div>
        <div v-if="!isAchievement && pageCount > 1" class="catalog-pagination"><span>每页 60 个条目</span><n-pagination :page="view.page" :page-count="pageCount" :page-slot="5" size="small" @update:page="state.selectPage" /></div>
    </div>

    <footer class="catalog-footer"><span>图鉴数据可离线查阅</span><button type="button" @click="sourceOpen = true">数据来源与版本<ChevronRight :size="13" /></button></footer>

    <EntryDetail :show="drawerOpen" :entry="selectedEntry" :guide="selectedGuide" :can-go-back="canGoBack" @close="state.closeDetail" @back="state.backDetail" @open="state.openEntry" @navigate="navigate" />

    <!-- ~ 数据来源 -->
    <!-- * 说明：提供数据版本、范围、维护来源及图标包边界 -->
    <!-- * 交互：通过标题或页脚打开，关闭后保留目录位置 -->
    <!-- * 依赖：已校验的本地 manifest -->
    <!-- * 边界：不宣称是最新游戏数据，不自动打开第三方页面 -->
    <n-modal v-model:show="sourceOpen" preset="card" title="图鉴数据来源" class="encyclopedia-source-modal" style="width: min(520px, calc(100vw - 32px))">
      <dl class="source-properties"><dt>游戏版本</dt><dd>{{ encyclopediaManifest.gameVersion }}</dd><dt>数据集</dt><dd>{{ encyclopediaManifest.datasetVersion }}</dd><dt>数据来源</dt><dd>FactorioLab · MIT</dd><dt>收录范围</dt><dd>{{ encyclopediaManifest.counts.building }} 建筑 · {{ encyclopediaManifest.counts.item }} 物品 · {{ encyclopediaManifest.counts.recipe }} 配方 · {{ encyclopediaManifest.counts.technology }} 科技 · {{ encyclopediaManifest.counts.resource }} 资源 · {{ encyclopediaAchievements.length }} 成就</dd><dt>上游提交</dt><dd>{{ encyclopediaManifest.sources[0]?.commit }}</dd></dl>
      <p class="source-note">数值来自固定版本的公开数据集，不包含存档加成。分类布局参考戴森球计划 BWIKI；成就编号与达成条件参考 BWIKI「成就」页；离线阅读内容由本模块编写。</p><p class="source-note">游戏图标属于可替换资源包，缺失时使用 Game Icons 类别图标。游戏素材、名称与商标归原权利人所有。</p>
    </n-modal>
  </div>
</template>

<style scoped lang="scss">
.encyclopedia-page { container: encyclopedia / inline-size; min-height: 100%; box-sizing: border-box; padding: var(--ds-page-padding-top) var(--ds-page-padding-x) var(--ds-page-padding-bottom); color: var(--ds-text-primary); }
.encyclopedia-header { display: flex; align-items: flex-end; justify-content: space-between; gap: 20px; margin-bottom: 20px; }
.encyclopedia-heading { min-width: 0; }
h1 { margin: 0; color: var(--ds-text-primary); font-size: 23px; font-weight: 700; line-height: 1.4; }
.encyclopedia-search { width: 280px; flex: 0 1 280px; }
.encyclopedia-search :deep(.n-input__placeholder) { color: var(--ds-text-secondary); }
// 名称开关与筛选控件同组：无描边、无填充，只用悬停与开启状态的浅色底区分。
.names-button { display: inline-flex; width: 28px; height: 28px; flex-shrink: 0; align-items: center; justify-content: center; border: 0; border-radius: 2px; padding: 0; background: transparent; color: var(--ds-text-muted); cursor: pointer; transition: background-color 150ms, color 150ms; }
.names-button:hover { background: var(--ds-hover-bg); color: var(--ds-text-primary); }
.names-button:focus-visible { outline: 2px solid var(--ds-focus-ring); outline-offset: 1px; }
.names-button.is-active { background: var(--ds-active-bg); color: var(--ds-brand); }
.group-select { width: 150px; }
.section-search { width: 220px; }
// 图鉴输入控件：去掉填充底色与描边，让筛选控件融进分区块；仅悬停、聚焦时铺一层极浅中性底，保持可操作提示。
.encyclopedia-field { --n-color: transparent !important; --n-color-focus: var(--ds-hover-bg) !important; --n-border: none !important; --n-border-hover: none !important; --n-border-focus: none !important; --n-border-radius: 2px !important; --n-box-shadow-focus: none !important; --n-box-shadow-active: none !important; --n-placeholder-color: var(--ds-text-muted) !important; --n-text-color: var(--ds-text-primary) !important; }
.encyclopedia-field.n-input:hover { background-color: var(--ds-hover-bg); }
.encyclopedia-field :deep(.n-base-selection) { --n-color: transparent !important; --n-color-active: var(--ds-hover-bg) !important; --n-border: none !important; --n-border-hover: none !important; --n-border-focus: none !important; --n-border-active: none !important; --n-border-radius: 2px !important; --n-box-shadow-focus: none !important; --n-box-shadow-active: none !important; --n-text-color: var(--ds-text-secondary) !important; --n-arrow-color: var(--ds-text-muted) !important; }
.encyclopedia-field :deep(.n-base-selection:hover .n-base-selection-label), .encyclopedia-field :deep(.n-base-selection--focus .n-base-selection-label) { background-color: var(--ds-hover-bg); }
.catalog-content, .catalog-skeleton { display: grid; gap: 14px; }
.category-grid + .catalog-state, .catalog-skeleton, .catalog-content { margin-top: 16px; }
.catalog-content.is-first-visit { animation: catalog-arrive 250ms both; }
.guide-list { border-top: 1px solid var(--ds-border-default); }
.guide-row { display: flex; width: 100%; align-items: center; gap: 14px; border: 0; border-bottom: 1px solid var(--ds-border-subtle); padding: 14px 10px; background: transparent; color: var(--ds-text-primary); text-align: left; cursor: pointer; transition: background-color 150ms; }
.guide-row > svg { flex: 0 0 auto; color: var(--ds-text-secondary); }
.guide-row > div { min-width: 0; flex: 1; }
.guide-row h2 { margin: 0 0 3px; font-size: 14px; font-weight: 600; line-height: 1.5; }
.guide-row p { margin: 0; color: var(--ds-text-secondary); font-size: 12px; line-height: 1.6; }
.guide-row:hover { background: var(--ds-hover-bg); }
.catalog-state { display: flex; min-height: 220px; flex-direction: column; align-items: center; justify-content: center; gap: 10px; padding: 24px 20px; color: var(--ds-text-muted); text-align: center; }
.catalog-state h2 { margin: 0; color: var(--ds-text-primary); font-size: 15px; }
.catalog-state p { max-width: 360px; margin: 0 0 6px; font-size: 13px; line-height: 1.7; }
.catalog-pagination { display: flex; align-items: center; justify-content: space-between; gap: 12px; color: var(--ds-text-muted); font-size: 12px; }
.catalog-footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-top: 24px; border-top: 1px solid var(--ds-border-subtle); padding-top: 14px; color: var(--ds-text-muted); font-size: 12px; }
.catalog-footer button { display: flex; align-items: center; gap: 4px; border: 0; padding: 0; background: transparent; color: inherit; font-size: inherit; cursor: pointer; }
.catalog-footer button:hover { color: var(--ds-text-primary); }
.source-properties { display: grid; grid-template-columns: 70px minmax(0, 1fr); gap: 10px 14px; margin: 0 0 18px; font-size: 13px; }
.source-properties dt { color: var(--ds-text-secondary); }
.source-properties dd { margin: 0; overflow-wrap: anywhere; }
.source-note { margin: 10px 0 0; color: var(--ds-text-secondary); font-size: 12px; line-height: 1.8; }
@keyframes catalog-arrive { from { opacity: 0; transform: translateY(6px); } to { opacity: 1; transform: translateY(0); } }
@container encyclopedia (max-width: 650px) { .encyclopedia-header { align-items: stretch; flex-direction: column; gap: 12px; margin-bottom: 16px; } .encyclopedia-search { width: 100%; flex-basis: auto; } }
@container encyclopedia (max-width: 600px) { .group-select { width: 120px; } .section-search { width: 140px; } }
@media (prefers-reduced-motion: reduce) { .catalog-content.is-first-visit { animation: none; } .guide-row { transition: none; } }
</style>
