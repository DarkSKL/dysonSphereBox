<script setup lang="ts">
import { ChevronDown, Copy, Download, Eye, FolderOpen, RefreshCw, RotateCcw, Search, SlidersHorizontal, Trash2, X } from '@lucide/vue'
import { computed, h, onBeforeUnmount, ref, watch } from 'vue'
import { useLogs } from '../../composables/useLogs'
import { logHandlerOptions as handlerOptions, logLevelOptions as levelOptions, logNamespaceOptions as namespaceOptions, type LogEntry } from '../../types/logging'

definePage({ meta: { layout: 'main', title: '系统日志' } })

// =============== 日志展示 ===============
// 说明：筛选、列表和详情共用统一契约；级别只表达严重度，结果直接展示 data.result。
const logs = useLogs()
const { query, items, total, loading, exporting, error, warning, health, healthWarning, selected, timeline, timelineWarning } = logs
const showLogHealthDetails = logs.showLogHealthDetails
const detailOpen = computed({ get: () => selected.value !== null, set: (open: boolean) => { if (!open) selected.value = null } })
const advanced = ref(false)
const searchValue = ref(query.keyword)
const contextEntry = ref<LogEntry | null>(null)
const contextMenu = ref({ show: false, x: 0, y: 0 })
const contextOptions = computed(() => [
  { label: '查看详情', key: 'detail', icon: () => h(Eye, { size: 15 }) },
  { label: '删除日志', key: 'delete', disabled: !!logs.deleting.value, icon: () => h(Trash2, { size: 15, style: { color: 'var(--ds-error)' } }) },
])
let searchTimer: number | undefined
const activeFilterCount = computed(() => [query.levels.length !== levelOptions.length, query.namespaces.length !== namespaceOptions.length, query.handlers.length !== handlerOptions.length, query.source, query.operationId, query.sessionId, query.from || query.to].filter(Boolean).length)
const range = computed(() => total.value ? `${(query.page - 1) * query.pageSize + 1} - ${Math.min(query.page * query.pageSize, total.value)}` : '0')
const json = computed(() => selected.value ? JSON.stringify(selected.value, null, 2) : '')
const handlerLabel = (value: string): string => handlerOptions.find((option) => option.value === value)?.label ?? value
const timeLabel = (value: string): string => { const date = new Date(value); return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN', { hour12: false }) }
const resultLabel = (entry: LogEntry): string => typeof entry.data.result === 'string' ? entry.data.result : ''
const changePage = (value: number): void => { query.page = value; void logs.load() }
const changeSize = (value: number): void => { query.pageSize = value; void logs.load(true) }
const changeRange = (value: [number, number] | null): void => { query.from = value ? new Date(value[0]).toISOString() : ''; query.to = value ? new Date(value[1]).toISOString() : ''; void logs.load(true) }
const runSearch = (): void => { window.clearTimeout(searchTimer); if (query.keyword === searchValue.value) return; query.keyword = searchValue.value; void logs.load(true) }
watch(searchValue, () => { window.clearTimeout(searchTimer); searchTimer = window.setTimeout(runSearch, 250) })
const resetFilters = (): void => { window.clearTimeout(searchTimer); searchValue.value = ''; logs.reset() }
const showContextMenu = (event: MouseEvent, entry: LogEntry): void => { event.preventDefault(); contextEntry.value = entry; contextMenu.value = { show: true, x: event.clientX, y: event.clientY } }
// 菜单关闭后固定本次目标，避免异步删除期间右键其它条目改变操作对象。
const selectContextAction = (key: string): void => {
  contextMenu.value.show = false
  const entry = contextEntry.value
  contextEntry.value = null
  if (!entry) return
  if (key === 'detail') void logs.select(entry)
  if (key === 'delete') void logs.remove(entry)
}
onBeforeUnmount(() => window.clearTimeout(searchTimer))
</script>

<template>
  <section class="logs-page" aria-label="系统日志">
    <div class="logs-sticky-header">
      <!-- ~ 检索与日志操作 -->
      <!-- * 说明：集中展示关键词、高级筛选及刷新、目录、导出图标入口 -->
      <!-- * 交互：关键词实时查询，高级条件浮出；图标悬停提示用途，导出当前筛选结果 -->
      <!-- * 依赖：query、useLogs 与 LogService -->
      <!-- * 边界：默认全选，取消全部选项时无结果，非法时间由后端拒绝；刷新和导出期间显示加载状态 -->
      <div class="logs-filters">
        <div class="logs-search-row">
          <n-input v-model:value="searchValue" :bordered="false" :theme-overrides="{ heightMedium: '66px', borderRadius: '0', color: 'transparent', colorFocus: 'transparent' }" placeholder="搜索行为、原因或数据" clearable aria-label="日志关键词" @keydown.enter.prevent="runSearch"><template #prefix><Search :size="16" /></template></n-input>
          <n-popover v-model:show="advanced" trigger="click" placement="bottom-end" :show-arrow="false" :style="{ width: '720px', maxWidth: 'calc(100vw - 32px)' }">
            <template #trigger><n-button quaternary size="small" :aria-expanded="advanced" aria-controls="logs-advanced-filters"><template #icon><SlidersHorizontal :size="16" /></template>高级筛选<span v-if="activeFilterCount" class="filter-count">{{ activeFilterCount }}</span><ChevronDown :size="14" :class="{ expanded: advanced }" /></n-button></template>
            <form id="logs-advanced-filters" class="logs-advanced" @submit.prevent="logs.load(true); advanced = false" @keydown.esc="advanced = false">
              <p class="logs-filter-hint">默认全选，移除选项可缩小范围；全部移除后不显示日志。</p>
              <div class="logs-filter-field">
                <div class="logs-filter-label"><span>日志级别</span><n-button text size="tiny" :disabled="query.levels.length === levelOptions.length" @click="query.levels = levelOptions.map(option => option.value); logs.load(true)">全选</n-button></div>
                <n-select v-model:value="query.levels" multiple :max-tag-count="'responsive'" :options="levelOptions" placeholder="未选择级别（无结果）" clearable aria-label="日志级别" @update:value="logs.load(true)" />
              </div>
              <div class="logs-filter-field">
                <div class="logs-filter-label"><span>日志模块</span><n-button text size="tiny" :disabled="query.namespaces.length === namespaceOptions.length" @click="query.namespaces = namespaceOptions.map(option => option.value); logs.load(true)">全选</n-button></div>
                <n-select v-model:value="query.namespaces" multiple :max-tag-count="'responsive'" :options="namespaceOptions" placeholder="未选择模块（无结果）" clearable aria-label="日志模块" @update:value="logs.load(true)" />
              </div>
              <div class="logs-filter-field">
                <div class="logs-filter-label"><span>触发者</span><n-button text size="tiny" :disabled="query.handlers.length === handlerOptions.length" @click="query.handlers = handlerOptions.map(option => option.value); logs.load(true)">全选</n-button></div>
                <n-select v-model:value="query.handlers" multiple :max-tag-count="'responsive'" :options="handlerOptions" placeholder="未选择触发者（无结果）" clearable aria-label="触发者" @update:value="logs.load(true)" />
              </div>
              <div class="logs-filter-field"><div class="logs-filter-label"><span>时间范围</span></div><n-date-picker type="datetimerange" clearable aria-label="日志时间范围" @update:value="changeRange" /></div>
              <n-input v-model:value="query.source" placeholder="来源，如 backend.http" clearable aria-label="记录来源" />
              <n-input v-model:value="query.operationId" placeholder="操作关联号" clearable aria-label="操作关联号" />
              <n-input v-model:value="query.sessionId" placeholder="运行会话号" clearable aria-label="运行会话号" />
              <div class="filter-actions"><n-button attr-type="submit" size="small">应用筛选</n-button><n-tooltip><template #trigger><n-button quaternary circle aria-label="重置筛选" @click="resetFilters"><template #icon><RotateCcw :size="16" /></template></n-button></template>重置筛选</n-tooltip></div>
            </form>
          </n-popover>
          <div class="logs-actions" role="group" aria-label="日志操作">
            <n-tooltip><template #trigger><n-button quaternary circle aria-label="刷新日志" :loading="loading" @click="logs.load()"><template #icon><RefreshCw :size="17" /></template></n-button></template>刷新日志</n-tooltip>
            <n-tooltip><template #trigger><n-button quaternary circle aria-label="打开日志目录" @click="logs.openDirectory"><template #icon><FolderOpen :size="17" /></template></n-button></template>打开日志目录</n-tooltip>
            <n-tooltip><template #trigger><n-button quaternary circle aria-label="导出诊断包" :loading="exporting" @click="logs.exportBundle"><template #icon><Download :size="17" /></template></n-button></template>导出诊断包</n-tooltip>
          </div>
        </div>
      </div>
    </div>

    <n-alert v-if="healthWarning || warning" type="warning" :show-icon="true" class="logs-alert">{{ healthWarning || warning }}</n-alert>
    <n-alert v-if="error" type="error" :show-icon="true" class="logs-alert">{{ error }}</n-alert>
    <div class="logs-status"><span>{{ health?.file.fileError ? '文件采集异常' : health ? '文件采集正常' : '采集状态未就绪' }}</span><span v-if="health?.file.fallback">后备目录</span><span class="type-mono">{{ health?.file.directory }}</span><span v-if="health?.pending">{{ health.pending }} 条待索引</span><n-button v-if="health?.file.noiseSuppressed" text size="tiny" @click="showLogHealthDetails = !showLogHealthDetails">本次运行已过滤 {{ health.file.noiseSuppressed }} 条框架噪声</n-button></div>
    <div v-if="showLogHealthDetails && health?.file.noiseSuppressed" class="logs-noise-details">框架调试与信息输出、开发热重载取消请求及重复握手关闭输出已过滤。未知运行时错误仍保留；相同基础设施故障会在诊断事件中附带重复次数和时间范围。</div>

    <!-- ~ 日志列表与详情 -->
    <!-- * 说明：列表以时间线卡片呈现，详情展示完整结构与同一次操作的事件 -->
    <!-- * 交互：点击卡片查看详情，右键可查看或删除日志，详情中可复制结构化记录 -->
    <!-- * 依赖：items、selected、timeline -->
    <!-- * 边界：空态、失败、读取降级分别显示，不用空列表掩盖错误 -->
    <div class="logs-workspace">
      <div class="logs-list" :aria-busy="loading">
        <div class="logs-timeline" role="list" aria-label="日志时间线">
          <article v-for="entry in items" :key="entry.id" role="listitem" class="log-entry" :class="{ selected: selected?.id === entry.id }" tabindex="0" @click="logs.select(entry)" @keydown.enter="logs.select(entry)" @keydown.space.prevent="logs.select(entry)" @contextmenu="showContextMenu($event, entry)">
            <span class="log-node" :data-level="entry.level" aria-hidden="true"></span>
            <div class="log-card">
              <div class="log-card-top"><time class="type-mono">{{ timeLabel(entry.time) }}</time><span class="log-level" :data-level="entry.level">{{ entry.level }}</span><span class="log-namespace type-mono">{{ entry.namespace }}</span></div>
              <div class="log-action">{{ entry.action }}</div>
              <div class="log-reason">{{ entry.reason }}</div>
              <div class="log-card-bottom"><span class="type-mono log-source">{{ entry.source }}</span><span>{{ handlerLabel(entry.handler) }}<span v-if="resultLabel(entry)"> · {{ resultLabel(entry) }}</span></span></div>
              <n-tooltip><template #trigger><n-button class="log-card-action" quaternary circle size="small" :aria-label="`查看日志详情：${entry.action}`" @click.stop="logs.select(entry)"><template #icon><Eye :size="16" /></template></n-button></template>查看详情</n-tooltip>
            </div>
          </article>
          <div v-if="!items.length" class="logs-empty">{{ loading ? '正在读取日志…' : error ? '日志读取失败' : '没有符合条件的记录' }}</div>
        </div>
        <footer class="logs-pagination"><span>{{ range }} / {{ total }}</span><n-pagination :page="query.page" :page-size="query.pageSize" :item-count="total" :page-sizes="[25, 50, 100, 200]" show-size-picker @update:page="changePage" @update:page-size="changeSize" /></footer>
      </div>
    </div>
    <n-modal v-model:show="detailOpen">
      <section v-if="selected" class="log-detail" role="dialog" aria-modal="true" aria-labelledby="log-detail-title">
        <header class="detail-header"><h2 id="log-detail-title">日志详情</h2><div><n-tooltip><template #trigger><n-button quaternary circle aria-label="复制日志" @click="logs.copy"><template #icon><Copy :size="16" /></template></n-button></template>复制日志</n-tooltip><n-button quaternary circle aria-label="关闭详情" @click="detailOpen = false"><template #icon><X :size="17" /></template></n-button></div></header>
        <p class="detail-action">{{ selected.action }}</p><p class="detail-reason">{{ selected.reason }}</p>
        <dl class="detail-facts"><dt>触发者</dt><dd>{{ handlerLabel(selected.handler) }}</dd><dt>来源</dt><dd class="type-mono">{{ selected.source }}</dd><dt>会话</dt><dd class="type-mono">{{ selected.sessionId }}</dd><dt>操作</dt><dd class="type-mono">{{ selected.operationId || '未记录关联号' }}</dd><dt>版本</dt><dd class="type-mono">{{ selected.appVersion }}</dd></dl>
        <template v-if="selected.operationId"><h3>关联事件</h3><p v-if="timelineWarning" class="timeline-warning">{{ timelineWarning }}</p><ol class="log-timeline"><li v-for="item in timeline" :key="item.id"><span class="log-level" :data-level="item.level">{{ item.level }}</span><span>{{ item.action }}</span><small class="type-mono">{{ timeLabel(item.time) }}</small></li></ol></template>
        <h3>完整记录</h3><pre class="type-mono log-json">{{ json }}</pre>
      </section>
    </n-modal>
    <n-dropdown trigger="manual" placement="bottom-start" :show="contextMenu.show" :x="contextMenu.x" :y="contextMenu.y" :options="contextOptions" @clickoutside="contextMenu.show = false" @select="selectContextAction" />
  </section>
</template>

<style scoped lang="scss">
.logs-page { width: 100%; padding: var(--ds-page-padding-top) var(--ds-page-padding-x) var(--ds-page-padding-bottom); min-width: 0; }
.logs-sticky-header {
  position: sticky;
  z-index: 20;
  top: 0;
  background: var(--ds-bg-header);
  -webkit-backdrop-filter: blur(12px);
  backdrop-filter: blur(12px);
}
.logs-pagination, .detail-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.logs-pagination, .logs-status { font-size: 12px; color: var(--ds-text-muted); }
.logs-actions { display: flex; align-items: center; gap: 4px; }
.logs-filters { border-bottom: 1px solid var(--ds-border-default); }
.logs-search-row { display: flex; align-items: center; gap: 10px; padding-right: 20px; }
.logs-search-row > :first-child { flex: 1; min-width: 0; }
.logs-search-row > :not(:first-child) { flex-shrink: 0; }
.logs-advanced { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; padding: 4px; }
.logs-filter-hint { grid-column: 1 / -1; margin: 0; font-size: 12px; color: var(--ds-text-muted); }
.logs-filter-field { min-width: 0; }
.logs-filter-label { display: flex; align-items: center; justify-content: space-between; margin-bottom: 6px; font-size: 12px; color: var(--ds-text-secondary); }
.filter-count { margin-left: 6px; color: var(--ds-brand); }
.logs-search-row .lucide-chevron-down { margin-left: 6px; }
.logs-search-row .expanded { transform: rotate(180deg); }
.filter-actions { display: flex; align-items: center; gap: 8px; }
.logs-alert { margin-top: 12px; }
.logs-status { display: flex; flex-wrap: wrap; gap: 8px 16px; padding: 12px 0; overflow-wrap: anywhere; }
.logs-noise-details { padding: 0 0 12px; color: var(--ds-text-muted); font-size: 12px; }
.logs-workspace { display: grid; min-width: 0; }
.logs-list { min-width: 0; }
.logs-timeline { position: relative; padding: 4px 0 4px 28px; border-block: 1px solid var(--ds-border-default); }
.logs-timeline::before { content: ''; position: absolute; top: 20px; bottom: 20px; left: 10px; width: 1px; background: var(--ds-border-default); }
.log-entry { position: relative; margin: 10px 0; cursor: pointer; min-width: 0; }
.log-node { position: absolute; top: 20px; left: -22px; width: 9px; height: 9px; border: 2px solid var(--ds-info); border-radius: 50%; background: var(--ds-bg-panel); }
.log-node[data-level='Warn'] { border-color: var(--ds-warning); }
.log-node[data-level='Error'], .log-node[data-level='Fatal'] { border-color: var(--ds-error); }
.log-card { position: relative; min-width: 0; padding: 12px 48px 12px 16px; border: 1px solid var(--ds-border-default); border-radius: 4px; background: var(--ds-bg-panel); overflow-wrap: anywhere; }
.log-entry:hover .log-card { background: var(--ds-hover-bg); }
.log-entry.selected .log-card { border-color: var(--ds-brand); background: var(--ds-active-bg); }
.log-entry:focus-visible { outline: 2px solid var(--ds-focus-ring); outline-offset: 2px; }
.log-card-top, .log-card-bottom { display: flex; align-items: center; flex-wrap: wrap; gap: 4px 12px; font-size: 11px; color: var(--ds-text-muted); }
.log-card-top { margin-bottom: 7px; }
.log-card-bottom { margin-top: 9px; }
.log-card-action { position: absolute; top: 10px; right: 10px; opacity: 0; }
.log-entry:hover .log-card-action, .log-entry:focus-within .log-card-action { opacity: 1; }
.log-level { display: inline-block; font-size: 11px; font-family: var(--ds-font-mono); color: var(--ds-text-secondary); }
.log-level[data-level='Error'], .log-level[data-level='Fatal'] { color: var(--ds-error); }
.log-level[data-level='Warn'] { color: var(--ds-warning); }
.log-level[data-level='Info'] { color: var(--ds-info); }
.log-action { font-weight: 700; color: var(--ds-text-primary); }
.log-reason, .log-secondary { color: var(--ds-text-secondary); font-size: 12px; margin-top: 5px; }
.log-namespace { color: var(--ds-text-muted); font-size: 10px; }
.log-source { font-size: 11px; }
.logs-empty { padding: 56px 16px; text-align: center; color: var(--ds-text-muted); }
.logs-pagination { padding: 16px 0; flex-wrap: wrap; }
.log-detail { width: min(720px, calc(100vw - 32px)); max-height: min(80vh, 900px); overflow-y: auto; padding: 20px 24px 24px; border: 1px solid var(--ds-border-default); border-radius: 4px; background: var(--ds-bg-panel-elevated); color: var(--ds-text-primary); }
.detail-header { border-bottom: 1px solid var(--ds-border-default); padding-bottom: 10px; }
.detail-header h2 { font-size: 14px; font-weight: 700; margin: 0; }
.detail-header > div { display: flex; }
.detail-action { font-size: 15px; font-weight: 700; margin: 16px 0 8px; overflow-wrap: anywhere; }
.detail-reason { font-size: 12px; color: var(--ds-text-secondary); overflow-wrap: anywhere; }
.detail-facts { display: grid; grid-template-columns: 44px minmax(0, 1fr); gap: 10px; font-size: 11px; margin: 20px 0; }
.detail-facts dt { color: var(--ds-text-muted); }
.detail-facts dd { margin: 0; overflow-wrap: anywhere; }
.log-detail h3 { font-size: 12px; color: var(--ds-text-secondary); padding-top: 16px; border-top: 1px solid var(--ds-border-default); }
.log-json { font-size: 11px; line-height: 1.7; white-space: pre-wrap; overflow-wrap: anywhere; background: var(--ds-bg-panel); padding: 12px; border-radius: 4px; max-height: 600px; overflow-y: auto; }
.log-timeline { list-style: none; margin: 0; padding: 0; }
.log-timeline li { display: grid; grid-template-columns: 42px minmax(0, 1fr); gap: 4px 8px; padding: 10px 0; border-bottom: 1px solid var(--ds-border-subtle); font-size: 12px; overflow-wrap: anywhere; }
.log-timeline small { grid-column: 2; font-size: 10px; color: var(--ds-text-muted); }
.timeline-warning { color: var(--ds-warning); font-size: 12px; }
@media (max-width: 700px) { .logs-advanced { grid-template-columns: 1fr; } .log-card-action { opacity: 1; } .log-detail { padding: 16px; } }
</style>
