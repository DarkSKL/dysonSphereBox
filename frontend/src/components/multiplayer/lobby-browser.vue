<script setup lang="ts">
import {ArrowRight, KeyRound, LockKeyhole, Plus, RefreshCw, Search} from '@lucide/vue'
import {computed, ref, watch} from 'vue'
import type {LobbyQuery, MultiplayerState, RoomSummary} from '../../types/multiplayer'
import {portText} from '../../utils/multiplayer'

// =============== 大厅浏览与筛选 ===============
// 说明：列表来自后端投影；输入草稿提交后才搜索，排序、分页和每页条数交给页面更新请求。
const props = defineProps<{
  state: MultiplayerState | null; query: LobbyQuery; online: boolean; admissionEnabled: boolean; loading: boolean
  ready: boolean; error: string; selected: string; busy: boolean
  connectionTitle: string; connectionHint: string
}>()
const emit = defineEmits<{
  (event: 'search', value: string): void
  (event: 'sort', value: LobbyQuery['sort']): void
  (event: 'page' | 'pageSize', value: number): void
  (event: 'join', roomId: string): void
  (event: 'refresh' | 'create' | 'invite' | 'connect'): void
}>()
const searchDraft = ref(props.query.keyword)
watch(() => props.query.keyword, (value) => { searchDraft.value = value })
const sortOptions = [{label: '最新创建', value: 'createdAt'}, {label: '人数最多', value: 'memberCount'}]
const pageSizeOptions = [10, 20, 50].map((value) => ({label: `${value} 条/页`, value}))
const list = computed<RoomSummary[]>(() => {
  if (!props.online || !props.ready) return []
  // 增量事件会把新房间放在前面，按真实创建时间重新整理，保持默认排序稳定。
  return [...props.state?.lobby.list ?? []].sort((a, b) => props.query.sort === 'createdAt'
    ? b.createdAtMs - a.createdAtMs : b.memberCount - a.memberCount || b.createdAtMs - a.createdAtMs).slice(0, props.query.pageSize)
})
const total = computed(() => props.state?.lobby.total ?? 0)
const pageCount = computed(() => Math.max(1, Math.ceil(total.value / props.query.pageSize)))
const emptyTitle = computed(() => !props.online ? props.connectionTitle : props.error ? '暂时无法读取大厅' : props.query.keyword ? '没有找到匹配的房间' : '下一段旅程，从你的房间开始')
const emptyHint = computed(() => !props.online ? props.connectionHint : props.error || (props.query.keyword ? '试试其他房间名，或使用好友发来的邀请码。' : '目前没有可加入的公开房间。创建一个房间，邀请好友一起建设星系。'))

// 说明：更新时间只在读取完成或列表内容真正变化时刷新，其他投影推送（链路、诊断）不影响。
const updatedAt = ref(0)
const listSignature = computed(() => props.ready ? `${total.value}|${list.value.map((item) => `${item.roomId}:${item.memberCount}`).join(',')}` : '')
watch(() => props.ready, (ready) => { if (ready) updatedAt.value = Date.now() })
watch(listSignature, (signature) => { if (signature) updatedAt.value = Date.now() })
const updatedText = computed(() => updatedAt.value ? new Date(updatedAt.value).toLocaleTimeString('zh-CN', {hour12: false}) : '')

function search(): void { emit('search', searchDraft.value.trim()) }
function full(item: RoomSummary): boolean { return item.memberCount >= item.maxMembers }
// 说明：当天创建只显示时分，跨天补充日期；不做相对时间，避免需要定时刷新。
function createdText(ms: number): string {
  if (!ms) return '—'
  const date = new Date(ms)
  const sameDay = date.toDateString() === new Date().toDateString()
  return sameDay
    ? date.toLocaleTimeString('zh-CN', {hour12: false, hour: '2-digit', minute: '2-digit'})
    : date.toLocaleString('zh-CN', {hour12: false, month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit'})
}
</script>

<template>
  <!-- ~ 公开大厅：顶部工具栏、带分页的房间表格与明确的连接/加载/空状态 -->
  <section class="lobby-browser" aria-label="公开房间" :aria-busy="loading">
    <!-- * 工具栏：左侧搜索与排序，右侧刷新、邀请码加入与创建房间 -->
    <form class="lobby-toolbar" @submit.prevent="search">
      <n-input v-model:value="searchDraft" class="search-input" placeholder="搜索房间名，回车确认" aria-label="搜索房间名" clearable :disabled="!online" @clear="emit('search', '')">
        <template #prefix><Search :size="16" /></template>
      </n-input>
      <n-select :value="query.sort" :options="sortOptions" :disabled="!online || loading" aria-label="房间排序" class="sort-select" @update:value="emit('sort', $event)" />
      <span class="toolbar-spacer" />
      <n-tooltip><template #trigger><n-button :disabled="!online || loading" :loading="loading" aria-label="刷新大厅" @click="emit('refresh')"><template #icon><RefreshCw :size="16" /></template></n-button></template>刷新大厅</n-tooltip>
      <n-button :disabled="!admissionEnabled || busy" @click="emit('invite')"><template #icon><KeyRound :size="16" /></template>邀请码加入</n-button>
      <n-button type="primary" :disabled="!admissionEnabled || busy" @click="emit('create')"><template #icon><Plus :size="17" /></template>创建房间</n-button>
    </form>

    <div class="room-table" role="table" aria-label="公开房间">
      <div class="room-row room-columns" role="row">
        <span role="columnheader">房间 · 游戏端口</span><span role="columnheader" class="col-host">房主</span><span role="columnheader">人数</span><span role="columnheader" class="col-created">创建时间</span><span role="columnheader" class="col-lock">密码</span><span role="columnheader" class="col-action">操作</span>
      </div>
      <div v-if="loading" class="room-loading" role="status" aria-label="正在读取房间">
        <div v-for="index in 5" :key="index" class="room-row loading-row"><n-skeleton text :repeat="2" /><n-skeleton class="col-host" text /><n-skeleton width="56px" text /><n-skeleton class="col-created" width="64px" text /><span class="col-lock" /><n-skeleton class="col-action" width="64px" height="28px" /></div>
      </div>
      <template v-else-if="list.length && !error">
        <div v-for="item in list" :key="item.roomId" class="room-row room-item" role="row" :class="{'is-selected': selected === item.roomId}">
          <div class="room-info" role="cell">
            <h3 :title="item.name">{{ item.name }}</h3>
            <p class="port-text" :title="(item.ports ?? []).map(portText).join(' · ')">{{ (item.ports ?? []).map(portText).join(' · ') || '未声明端口' }}</p>
          </div>
          <span class="col-host host-name" role="cell" :title="item.hostNickname">{{ item.hostNickname }}</span>
          <div class="occupancy" role="cell" :aria-label="`${item.memberCount} / ${item.maxMembers} 人`">
            <span class="type-display">{{ item.memberCount }}<small> / {{ item.maxMembers }}</small></span>
            <span class="occupancy-track" aria-hidden="true"><i :style="{width: `${Math.min(100, item.memberCount / Math.max(1, item.maxMembers) * 100)}%`}" /></span>
          </div>
          <span class="col-created created-text" role="cell">{{ createdText(item.createdAtMs) }}</span>
          <span class="col-lock" role="cell"><LockKeyhole v-if="item.hasPassword" :size="15" aria-label="需要密码" /><span v-else class="muted" aria-label="无需密码">—</span></span>
          <span class="col-action" role="cell">
            <n-button size="small" :type="selected === item.roomId ? 'primary' : 'default'" :disabled="!admissionEnabled || busy || full(item)" :aria-label="`加入房间 ${item.name}`" @click="emit('join', item.roomId)"><template #icon><ArrowRight :size="14" /></template>{{ full(item) ? '已满' : '加入' }}</n-button>
          </span>
        </div>
      </template>
      <div v-else class="lobby-empty" role="status">
        <h3>{{ emptyTitle }}</h3><p>{{ emptyHint }}</p>
        <n-button v-if="online && !error && !query.keyword" secondary :disabled="!admissionEnabled || busy" @click="emit('create')">创建第一个房间<template #icon><ArrowRight :size="16" /></template></n-button>
        <n-button v-else-if="online && error" secondary @click="emit('refresh')">重新读取大厅</n-button>
        <n-button v-else-if="online && query.keyword" secondary @click="emit('search', '')">查看全部房间</n-button>
        <n-button v-else-if="state?.connection.state === 'offline' && state.account.loggedIn" secondary @click="emit('connect')">连接大厅</n-button>
      </div>
    </div>
    <footer class="lobby-footer"><span>大厅仅显示公开且未满员的房间</span><span>私密房间请使用好友发来的邀请码加入</span></footer>

    <!-- * 分页：仅在有房间时显示，置于列表说明之后，更新时间与分页、每页条数靠右 -->
    <div v-if="online && ready && total > 0" class="lobby-pager">
      <span v-if="updatedText" class="updated-text">更新于 {{ updatedText }}</span>
      <n-pagination :page="query.page" :page-count="pageCount" :page-slot="5" :disabled="loading" @update:page="emit('page', $event)" />
      <n-select :value="query.pageSize" :options="pageSizeOptions" :disabled="loading" aria-label="每页条数" class="size-select" @update:value="emit('pageSize', $event)" />
    </div>
  </section>
</template>

<style scoped lang="scss">
.lobby-browser { min-width: 0; }
.lobby-toolbar, .lobby-pager, .lobby-footer { display: flex; align-items: center; }
.lobby-toolbar { gap: 8px; margin-bottom: 20px; }
.search-input { width: 320px; flex-shrink: 1; min-width: 180px; }
.sort-select { width: 128px; flex-shrink: 0; }
.toolbar-spacer { flex: 1; }
.lobby-pager { justify-content: flex-end; flex-wrap: wrap; gap: 8px 12px; padding-top: 12px; border-top: 1px solid var(--ds-border-subtle); }
.updated-text { color: var(--ds-text-muted); font-size: 11px; font-variant-numeric: tabular-nums; }
.size-select { width: 108px; }

// 说明：桌面表格与表头共用同一列模板，窄窗口依次隐藏创建时间、房主与密码列。
.room-table { border-top: 1px solid var(--ds-border-default); }
.room-row { display: grid; grid-template-columns: minmax(0, 1fr) 150px 112px 112px 52px 92px; gap: 20px; align-items: center; padding-inline: 12px; }
.room-columns { padding-block: 10px; border-bottom: 1px solid var(--ds-border-default); color: var(--ds-text-secondary); font-size: 12px; }
.col-lock { display: flex; justify-content: center; color: var(--ds-text-secondary); }
.col-action { display: flex; justify-content: flex-end; }
.room-item { min-height: 64px; padding-block: 12px; border-bottom: 1px solid var(--ds-border-subtle); transition: background 160ms ease-out; }
.room-item:hover { background: var(--ds-hover-bg); }
.room-item.is-selected { background: var(--ds-active-bg); }
.room-info { min-width: 0; }
.room-info h3 { margin: 0 0 4px; font-size: 14px; font-weight: 700; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.port-text { margin: 0; color: var(--ds-text-secondary); font-family: var(--ds-font-mono); font-size: 11px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.host-name { font-size: 13px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.occupancy { display: flex; flex-direction: column; gap: 6px; align-items: flex-start; font-size: 15px; font-variant-numeric: tabular-nums; }
.occupancy small { color: var(--ds-text-secondary); font-size: 12px; }
.occupancy-track { width: 56px; height: 3px; background: var(--ds-border-default); }
.occupancy-track i { display: block; height: 100%; background: var(--ds-text-secondary); }
.created-text { color: var(--ds-text-secondary); font-size: 12px; font-variant-numeric: tabular-nums; }
.muted { color: var(--ds-text-muted); }
.room-loading .loading-row { min-height: 64px; padding-block: 12px; border-bottom: 1px solid var(--ds-border-subtle); }
.lobby-empty { min-height: 360px; padding: 56px 24px 64px; display: flex; flex-direction: column; align-items: center; justify-content: center; text-align: center; border-bottom: 1px solid var(--ds-border-subtle); }
.lobby-empty h3 { margin: 0; font-size: 18px; font-weight: 700; }
.lobby-empty p { max-width: 340px; margin: 12px 0 24px; color: var(--ds-text-secondary); font-size: 13px; line-height: 1.8; }
.lobby-footer { justify-content: space-between; flex-wrap: wrap; gap: 12px; padding: 14px 0; color: var(--ds-text-muted); font-size: 11px; }
@media (max-width: 1180px) { .room-row { grid-template-columns: minmax(0, 1fr) 130px 100px 52px 92px; gap: 14px; } .col-created { display: none; } }
@media (max-width: 860px) {
  .lobby-toolbar { flex-wrap: wrap; } .search-input { width: auto; flex: 1 1 100%; } .toolbar-spacer { display: none; }
  .lobby-pager { justify-content: flex-start; }
  .room-row { grid-template-columns: minmax(0, 1fr) 88px 84px; gap: 12px; padding-inline: 8px; } .col-host, .col-lock { display: none; }
}
@media (max-width: 560px) { .lobby-empty { min-height: 300px; padding: 36px 16px; } }
@media (prefers-reduced-motion: reduce) { .room-item { transition: none; } }
</style>
