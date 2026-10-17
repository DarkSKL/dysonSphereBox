<script setup lang="ts">
import {computed, nextTick, onBeforeUnmount, onMounted, ref, watch} from 'vue'
import {Bell, ChevronLeft, ChevronRight, RotateCw} from '@lucide/vue'
import {useAnnouncements} from '../../composables/useAnnouncements'

const props = defineProps<{readonly disabled: boolean, readonly hasLatest: boolean}>()
// =============== 列表与详情展示状态 ===============
// 说明：弹层只负责焦点、分页控件和可见性；会话与手动已读行为统一由composable管理。
const show = ref(false)
const trigger = ref<HTMLButtonElement | null>(null)
const panel = ref<HTMLElement | null>(null)
const detailBody = ref<HTMLElement | null>(null)
const announcements = useAnnouncements()
const pageCount = computed(() => Math.max(1, Math.ceil(announcements.historyTotal.value / announcements.historyPageSize.value)))
const canPrevious = computed(() => announcements.historyPage.value > 1)
const canNext = computed(() => announcements.historyPage.value < pageCount.value)
const detailExpired = ref(false)
const pageSizeOptions = [20, 10, 5, 1].map((value) => ({label: `${value} 条/页`, value}))
const kindLabel = (kind: string): string => kind === 'promotion' ? '推广' : kind === 'announcement' ? '公告' : '通知'
const formatTime = (value: string | null | undefined): string => {
  if (!value) return '时间未知'
  const time = new Date(value)
  return Number.isNaN(time.getTime()) ? '时间未知' : time.toLocaleString()
}
// =============== 用户操作与焦点恢复 ===============
// 说明：打开入口按当前页重新查询，错误时可减小每页数量；Escape和详情关闭后将焦点还给入口。
const updateShow = async (value: boolean): Promise<void> => {
  show.value = value && !props.disabled
  if (show.value) await announcements.loadHistory(announcements.historyPage.value)
}
const changePage = async (page: number): Promise<void> => {
  if (page < 1 || page > pageCount.value || announcements.historyLoading.value) return
  await announcements.loadHistory(page)
}
const changePageSize = async (size: number): Promise<void> => {
  await announcements.loadHistory(1, size)
}
const closePanel = (): void => { show.value = false; trigger.value?.focus() }
const openDetail = async (entry: typeof announcements.history.value[number]): Promise<void> => {
  show.value = false
  await announcements.openHistoryDetail(entry)
  await nextTick()
  await verifyDetail()
}
const verifyDetail = async (): Promise<void> => {
  detailExpired.value = announcements.isHistoryExpired()
  await announcements.verifyHistoryVisible(props.disabled ? null : detailBody.value)
}
const acknowledgeDetail = async (): Promise<void> => {
  await verifyDetail()
  if (!props.disabled && announcements.historyDetailReady.value) await announcements.acknowledgeHistory()
}
watch(show, async (opened) => { if (opened) { await nextTick(); panel.value?.focus() } })
watch(() => props.disabled, (disabled) => {
  if (disabled) {
    show.value = false
    announcements.closeHistoryDetail()
  }
})
watch(announcements.historyOpen, (opened) => { if (!opened) show.value = false })
let expiryTimer: ReturnType<typeof setInterval> | undefined
watch(announcements.historyDetailOpen, (opened) => {
  clearInterval(expiryTimer)
  if (opened) expiryTimer = setInterval(() => { void verifyDetail() }, 1000)
})
onMounted(() => { document.addEventListener('visibilitychange', verifyDetail) })
onBeforeUnmount(() => {
  clearInterval(expiryTimer)
  document.removeEventListener('visibilitychange', verifyDetail)
})
</script>

<template>
  <!-- ~ 公告历史浮层 -->
  <!-- * 说明：按页展示当前和历史公告、有效期与当前账号首次查阅时间 -->
  <!-- * 交互：点击条目打开详情；查看、翻页和关闭均不提交已读 -->
  <!-- * 依赖：共享公告composable及项目主题token -->
  <!-- * 边界：加载、错误和空列表分别展示；大响应可减少每页数量重试 -->
  <n-popover
    v-model:show="show"
    placement="bottom-end"
    trigger="click"
    :disabled="props.disabled"
    :show-arrow="false"
    raw
    @update:show="updateShow"
  >
    <template #trigger>
      <button
        ref="trigger"
        type="button"
        class="window-control-button announcement-history-trigger"
        :class="{'has-unread': props.hasLatest}"
        :disabled="props.disabled"
        title="公告通知"
        :aria-label="props.hasLatest ? '公告通知，有未查阅的公告' : '公告通知'"
        :aria-expanded="show"
        aria-haspopup="dialog"
      >
        <Bell :size="16" :stroke-width="1.9" aria-hidden="true" />
        <span v-if="props.hasLatest" class="notification-badge" aria-hidden="true" />
      </button>
    </template>

    <section ref="panel" class="announcement-history-panel" role="dialog" aria-label="公告通知历史" :aria-busy="announcements.historyLoading.value" tabindex="-1" @keydown.esc.prevent.stop="closePanel">
      <header class="announcement-history-header">
        <div>
          <h2>公告通知</h2>
        </div>
        <n-button text size="small" title="刷新公告" aria-label="刷新公告" :disabled="announcements.historyLoading.value" @click="announcements.loadHistory(announcements.historyPage.value)"><RotateCw :size="15" /></n-button>
      </header>

      <div v-if="announcements.historyLoading.value" class="announcement-history-state" role="status">正在加载公告...</div>
      <div v-else-if="announcements.historyError.value" class="announcement-history-state is-error" role="alert">
        <span>{{ announcements.historyError.value }}</span>
        <button type="button" class="announcement-history-retry" title="重试" aria-label="重试" @click="announcements.loadHistory(announcements.historyPage.value)">
          <RotateCw :size="15" />
        </button>
      </div>
      <div v-else-if="!announcements.history.value.length" class="announcement-history-state" role="status">暂无公告通知</div>
      <div v-else class="announcement-history-list">
        <button
          v-for="entry in announcements.history.value"
          :key="`${entry.id}:${entry.revision}`"
          type="button"
          class="announcement-history-row"
          :class="{'is-unread': !entry.read}"
          @click="openDetail(entry)"
        >
          <span class="announcement-history-dot" aria-hidden="true" />
          <span class="announcement-history-row-main">
            <span class="announcement-history-row-title" :title="entry.title">{{ entry.title }}</span>
            <span class="announcement-history-row-meta">{{ kindLabel(entry.kind) }} · 发布于 {{ formatTime(entry.publishedAt) }}</span>
            <span v-if="entry.startsAt" class="announcement-history-row-meta">生效于 {{ formatTime(entry.startsAt) }}</span>
            <span v-if="entry.endsAt" class="announcement-history-row-meta">到期于 {{ formatTime(entry.endsAt) }}</span>
            <span v-if="entry.readAt" class="announcement-history-row-meta">已查阅于 {{ formatTime(entry.readAt) }}</span>
          </span>
          <span class="announcement-history-read">{{ entry.read ? '已查阅' : '未查阅' }}</span>
        </button>
      </div>

      <footer class="announcement-history-footer">
        <n-select class="announcement-history-page-size" size="small" aria-label="每页公告数量" :value="announcements.historyPageSize.value" :options="pageSizeOptions" :disabled="announcements.historyLoading.value" @update:value="changePageSize" />
        <span aria-live="polite">{{ announcements.historyTotal.value ? `${announcements.historyPage.value} / ${pageCount}` : '0 / 0' }}</span>
        <span class="announcement-history-pager">
          <button type="button" title="上一页" aria-label="上一页" :disabled="!canPrevious || announcements.historyLoading.value" @click="changePage(announcements.historyPage.value - 1)"><ChevronLeft :size="15" /></button>
          <button type="button" title="下一页" aria-label="下一页" :disabled="!canNext || announcements.historyLoading.value" @click="changePage(announcements.historyPage.value + 1)"><ChevronRight :size="15" /></button>
        </span>
      </footer>
    </section>
  </n-popover>

  <!-- ~ 历史公告详情 -->
  <!-- * 说明：渲染已清洗正文及完整时间信息，明确区分未读、已读和已过期 -->
  <!-- * 交互：正文可见且未过期时允许手动确认，失败保留正文和重试操作 -->
  <!-- * 依赖：Go当前页版本绑定、受控链接处理及可见性检查 -->
  <!-- * 边界：过期正文仍可查看，不能首次标为已读；关闭无副作用 -->
  <n-modal
    :show="announcements.historyDetailOpen.value && !props.disabled"
    preset="card"
    style="width:min(720px, calc(100vw - 32px));"
    :title="announcements.historyDetail.value?.title || '公告详情'"
    @after-enter="verifyDetail"
    @after-leave="trigger?.focus()"
    @update:show="(value) => !value && announcements.closeHistoryDetail()"
  >
    <div v-if="announcements.historyDetailLoading.value" class="announcement-history-state" role="status">正在加载公告详情...</div>
    <div v-if="announcements.historyDetail.value" class="announcement-history-detail-meta">
      <span>{{ kindLabel(announcements.historyDetail.value.kind) }} · 发布于 {{ formatTime(announcements.historyDetail.value.publishedAt) }}</span>
      <span v-if="announcements.historyDetail.value.startsAt">生效于 {{ formatTime(announcements.historyDetail.value.startsAt) }}</span>
      <span v-if="announcements.historyDetail.value.endsAt">到期于 {{ formatTime(announcements.historyDetail.value.endsAt) }}</span>
      <span v-if="announcements.historyDetail.value.read">已查阅于 {{ formatTime(announcements.historyDetail.value.readAt) }}</span>
      <span v-else>未查阅</span>
    </div>
    <div v-if="announcements.historyDetail.value" ref="detailBody" class="announcement-body" v-html="announcements.historyDetailHtml.value" @click="announcements.openLink" @auxclick.prevent="announcements.openLink" @keydown.enter="announcements.openLink" />
    <p v-if="announcements.historyDetail.value && detailExpired" class="announcement-history-expired">此公告已过期，可查看历史内容。</p>
    <p v-if="announcements.historyDetailError.value" class="announcement-error" role="alert">{{ announcements.historyDetailError.value }}</p>
    <template #footer>
      <n-space justify="end">
        <n-button secondary @click="announcements.closeHistoryDetail">关闭</n-button>
        <n-button v-if="announcements.historyDetailError.value && !announcements.historyDetail.value" secondary @click="show = true; announcements.closeHistoryDetail(); announcements.loadHistory(announcements.historyPage.value)">重新加载列表</n-button>
        <n-button v-if="announcements.historyDetail.value && !announcements.historyDetail.value.read && !detailExpired" type="primary" :loading="announcements.historyDetailPending.value" :disabled="!announcements.historyDetailReady.value" @click="acknowledgeDetail">已查阅</n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<style scoped lang="scss">
@use '../../styles/announcement-body';
.announcement-history-panel { width: min(390px, calc(100vw - 32px)); overflow: hidden; background: var(--ds-bg-panel); color: var(--ds-text-primary); border: 1px solid var(--ds-border-default); border-radius: 4px; box-shadow: 0 4px 16px rgb(0 0 0 / 14%); }
.announcement-history-header, .announcement-history-footer { display: flex; align-items: center; justify-content: space-between; padding: 13px 15px; border-bottom: 1px solid var(--ds-border-subtle); }
.announcement-history-footer { border-top: 1px solid var(--ds-border-subtle); border-bottom: 0; color: var(--ds-text-secondary); font-size: 12px; }
.announcement-history-header h2 { margin: 0; font-size: 16px; }
.announcement-history-list { max-height: 370px; overflow-y: auto; }
.announcement-history-row { display: flex; align-items: flex-start; gap: 9px; width: 100%; padding: 11px 15px; border: 0; border-bottom: 1px solid color-mix(in srgb, var(--ds-border-subtle) 72%, transparent); background: transparent; color: inherit; text-align: left; cursor: pointer; }
.announcement-history-row:hover, .announcement-history-row:focus-visible { background: var(--ds-hover-bg); }
.announcement-history-row:focus-visible, .announcement-history-retry:focus-visible, .announcement-history-pager button:focus-visible, .announcement-history-trigger:focus-visible { outline: 2px solid var(--ds-brand); outline-offset: -2px; }
.announcement-history-dot { width: 6px; height: 6px; margin-top: 6px; border-radius: 50%; background: transparent; flex: 0 0 auto; }
.is-unread .announcement-history-dot { background: var(--ds-brand); box-shadow: 0 0 0 3px color-mix(in srgb, var(--ds-brand) 14%, transparent); }
.announcement-history-row-main { display: grid; gap: 3px; min-width: 0; flex: 1; }
.announcement-history-row-title { overflow: hidden; font-size: 13px; font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
.announcement-history-row-meta, .announcement-history-read, .announcement-history-detail-meta { color: var(--ds-text-secondary); font-size: 12px; overflow-wrap: anywhere; }
.announcement-history-read { flex: 0 0 auto; }
.announcement-history-state { display: flex; min-height: 130px; align-items: center; justify-content: center; gap: 8px; padding: 18px; color: var(--ds-text-secondary); font-size: 13px; }
.announcement-history-state.is-error { color: var(--ds-error); }
.announcement-history-retry, .announcement-history-pager button { display: inline-flex; align-items: center; justify-content: center; width: 28px; height: 28px; border: 1px solid var(--ds-border-subtle); border-radius: 4px; background: transparent; color: inherit; cursor: pointer; }
.announcement-history-retry:hover, .announcement-history-pager button:hover:not(:disabled) { background: color-mix(in srgb, var(--ds-brand) 10%, transparent); }
.announcement-history-pager { display: flex; gap: 4px; }
.announcement-history-pager button:disabled { opacity: .35; cursor: default; }
.announcement-history-detail-meta { display: grid; gap: 3px; margin-bottom: 12px; }
.announcement-history-page-size { width: 104px; }
.announcement-history-trigger { position: relative; display: grid; place-items: center; width: 30px; height: 30px; border: 1px solid transparent; border-radius: 4px; background: transparent; color: var(--ds-text-muted); cursor: pointer; }
.announcement-history-trigger:hover:not(:disabled), .announcement-history-trigger:focus-visible { border-color: var(--ds-border-default); background: var(--ds-hover-bg); color: var(--ds-brand); }
.announcement-history-trigger:disabled { color: var(--ds-text-disabled); cursor: default; }
.announcement-history-expired { margin-top: 12px; color: var(--ds-text-secondary); font-size: 12px; }
.notification-badge { position: absolute; top: 5px; right: 5px; width: 6px; height: 6px; background: var(--ds-error); border-radius: 50%; }
</style>
