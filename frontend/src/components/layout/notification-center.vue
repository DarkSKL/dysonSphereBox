<script setup lang="ts">
import {computed, nextTick, onBeforeUnmount, onMounted, ref, watch} from 'vue'
import {Bell, Gift, Megaphone, RotateCw, X} from '@lucide/vue'
import {useAnnouncements} from '../../composables/useAnnouncements'
import {useNotifications} from '../../composables/useNotifications'
import {accountSessionKey} from '../../pages/services/account'
import type {NoticeHistoryEntry} from '../../pages/services/announcement'
import {formatDate} from '../../utils/format'
import ForumReviewNotice from './forum-review-notice.vue'
import NotificationFeed from './notification-feed.vue'
import NotificationInbox from './notification-inbox.vue'

const props = defineProps<{
  readonly disabled: boolean
  readonly mountTarget: HTMLElement | null
}>()
type NotificationTab = 'inbox' | 'announcement'

// =============== 消息侧栏状态 ===============
// 说明：侧栏管理入口、分类和可见性；列表查询和确认查阅使用应用级状态。
const show = ref(false)
const tab = ref<NotificationTab>('inbox')
const pageSizeMenuOpen = ref(false)
const reviewID = ref('')
const trigger = ref<HTMLButtonElement | null>(null)
const panel = ref<HTMLElement | null>(null)
const announcements = useAnnouncements()
const notifications = useNotifications()
const triggerLabel = computed(() => {
  const unread = notifications.unreadCount.value
  const latest = announcements.hasUnread.value
  return ['消息中心', unread ? `${unread} 条未读站内信` : '', latest ? '有待查阅公告' : ''].filter(Boolean).join('，')
})
const counterLabel = computed(() => notifications.unreadCount.value > 99 ? '99+' : String(notifications.unreadCount.value))
const refreshing = computed(() => tab.value === 'announcement' ? announcements.historyLoading.value : notifications.listLoading.value)

// =============== 侧栏操作与焦点 ===============
// 说明：点按或聚焦到外部时收起；键盘关闭时将焦点还给铃铛。
const restoreTriggerFocus = (): void => {
  if (!show.value && !props.disabled) trigger.value?.focus({preventScroll: true})
}
const closePanel = (restoreFocus = true): void => {
  show.value = false
  if (restoreFocus) restoreTriggerFocus()
}
const togglePanel = (): void => {
  if (show.value) closePanel()
  else if (!props.disabled) show.value = true
}
const selectTab = (value: NotificationTab): void => {
  if (tab.value === value) return
  tab.value = value
  notifications.setAnnouncementTab(value === 'announcement')
  if (show.value && value === 'announcement') refreshCurrentTab()
}
const handleTabKeys = (event: KeyboardEvent): void => {
  let next: NotificationTab
  switch (event.key) {
    case 'Home': next = 'inbox'; break
    case 'End': next = 'announcement'; break
    case 'ArrowLeft':
    case 'ArrowRight': next = tab.value === 'inbox' ? 'announcement' : 'inbox'; break
    default: return
  }
  event.preventDefault()
  selectTab(next)
  panel.value?.querySelector<HTMLElement>(`#notification-tab-${next}`)?.focus()
}
const dismissFromOutside = (event: Event): void => {
  if (!show.value || !(event.target instanceof Node)) return
  if (!trigger.value?.contains(event.target) && !panel.value?.contains(event.target)) closePanel(false)
}
const dismissFromKeyboard = (event: KeyboardEvent): void => {
  event.preventDefault()
  event.stopPropagation()
  if (pageSizeMenuOpen.value) pageSizeMenuOpen.value = false
  else closePanel()
}
const refreshCurrentTab = (): void => {
  if (tab.value === 'announcement') void announcements.loadHistory(announcements.historyPage.value)
  else notifications.refresh()
}
const openReview = (id: string): void => {
  closePanel(false)
  reviewID.value = id
}
const closeReview = (): void => {
  reviewID.value = ''
  void nextTick(restoreTriggerFocus)
}

// =============== 公告列表与查阅 ===============
// 说明：详情沿用正文可见性和版本校验；刷新与分页重新查询历史公告。
const pageCount = computed(() => Math.max(1, Math.ceil(announcements.historyTotal.value / announcements.historyPageSize.value)))
const pageSizeOptions = [20, 10, 5, 1].map((value) => ({label: `${value} 条/页`, value}))
const detailBody = ref<HTMLElement | null>(null)
const detailExpired = ref(false)
const kindLabel = (kind: string): string => kind === 'promotion' ? '推广' : kind === 'announcement' ? '公告' : '通知'
const formatTime = (value: string | null | undefined): string => {
  if (!value) return '时间未知'
  const time = new Date(value)
  return Number.isNaN(time.getTime()) ? '时间未知' : time.toLocaleString()
}
const openDetail = async (entry: NoticeHistoryEntry): Promise<void> => {
  closePanel(false)
  await announcements.openHistoryDetail(entry)
  await nextTick()
  await verifyDetail()
}
const reopenAnnouncementList = (): void => {
  announcements.closeHistoryDetail()
  if (!props.disabled) show.value = true
}
const verifyDetail = async (): Promise<void> => {
  detailExpired.value = announcements.isHistoryExpired()
  await announcements.verifyHistoryVisible(props.disabled ? null : detailBody.value)
}
const acknowledgeDetail = async (): Promise<void> => {
  await verifyDetail()
  if (!props.disabled && announcements.historyDetailReady.value) await announcements.acknowledgeHistory()
}

// =============== 面板与账号生命周期 ===============
// 说明：打开时同步当前类别；账号切换、门禁关闭和卸载时撤销面板交互。
watch(show, async (opened) => {
  notifications.setPanelOpen(opened)
  if (!opened) {
    pageSizeMenuOpen.value = false
    return
  }
  if (tab.value === 'announcement') refreshCurrentTab()
  await nextTick()
  if (show.value) panel.value?.focus({preventScroll: true})
}, {flush: 'sync'})
watch(() => props.disabled, (disabled) => {
  if (disabled) {
    closePanel(false)
    reviewID.value = ''
    announcements.closeHistoryDetail()
  }
})
watch(accountSessionKey, () => {
  closePanel(false)
  reviewID.value = ''
  selectTab('inbox')
})
let expiryTimer: ReturnType<typeof setInterval> | undefined
watch(announcements.historyDetailOpen, (opened) => {
  clearInterval(expiryTimer)
  if (opened) expiryTimer = setInterval(() => { void verifyDetail() }, 1000)
})
onMounted(() => {
  document.addEventListener('pointerdown', dismissFromOutside, true)
  document.addEventListener('focusin', dismissFromOutside)
  document.addEventListener('visibilitychange', verifyDetail)
})
onBeforeUnmount(() => {
  notifications.setPanelOpen(false)
  notifications.setAnnouncementTab(false)
  clearInterval(expiryTimer)
  document.removeEventListener('pointerdown', dismissFromOutside, true)
  document.removeEventListener('focusin', dismissFromOutside)
  document.removeEventListener('visibilitychange', verifyDetail)
})
</script>

<template>
  <!-- ~ 全局消息入口 -->
  <!-- * 说明：站内信与公告共用标题栏铃铛 -->
  <!-- * 交互：按下铃铛从右侧打开面板，再次按下收起 -->
  <!-- * 依赖：应用级通知和公告的未读状态 -->
  <!-- * 边界：会话未验证或全局弹窗、教程占用时禁用 -->
  <button
    ref="trigger"
    type="button"
    class="window-control-button notification-trigger"
    :disabled="props.disabled"
    :title="triggerLabel"
    :aria-label="triggerLabel"
    :aria-expanded="show"
    aria-controls="notification-drawer"
    aria-haspopup="dialog"
    @click="togglePanel"
  >
    <Bell :size="16" :stroke-width="1.9" aria-hidden="true" />
    <span v-if="notifications.hasUnread.value" class="notification-trigger-dot" aria-hidden="true" />
  </button>

  <!-- ~ 右侧消息中心 -->
  <!-- * 说明：标题栏下方叠放消息列表，主页面保持可见 -->
  <!-- * 交互：切换类别、刷新当前列表或收起面板 -->
  <!-- * 依赖：MainLayout 提供工作区挂载层 -->
  <!-- * 边界：非模态侧栏；关闭不会写入查阅回执 -->
  <Teleport v-if="props.mountTarget" :to="props.mountTarget">
    <Transition name="notification-drawer">
      <section
        v-show="show"
        id="notification-drawer"
        ref="panel"
        class="notification-drawer"
        role="dialog"
        aria-labelledby="notification-drawer-title"
        :inert="!show"
        :aria-hidden="!show"
        tabindex="-1"
        @keydown.esc.capture="dismissFromKeyboard"
      >
        <header class="notification-drawer-header">
          <h2 id="notification-drawer-title">消息中心</h2>
          <div class="notification-drawer-actions">
            <button type="button" title="刷新" aria-label="刷新当前消息列表" :disabled="refreshing" @click="refreshCurrentTab">
              <RotateCw :size="16" :stroke-width="1.8" aria-hidden="true" />
            </button>
            <button type="button" title="关闭" aria-label="关闭消息中心" @click="closePanel()">
              <X :size="18" :stroke-width="1.8" aria-hidden="true" />
            </button>
          </div>
        </header>

        <!-- ~ 消息分类 -->
        <!-- * 说明：站内信显示未读数量，公告提示待查阅 -->
        <!-- * 交互：支持方向键或 Home、End 切换类别 -->
        <!-- * 依赖：当前类别及应用级未读状态 -->
        <!-- * 边界：计数徽标显示到 99+，读屏可获取准确数量 -->
        <div class="notification-tabs" role="tablist" aria-label="消息类别" @keydown="handleTabKeys">
          <button
            id="notification-tab-inbox"
            type="button"
            role="tab"
            class="notification-tab"
            aria-controls="notification-inbox"
            :aria-selected="tab === 'inbox'"
            :tabindex="tab === 'inbox' ? 0 : -1"
            @click="selectTab('inbox')"
          >
            站内信
            <span v-if="notifications.unreadCount.value" class="notification-tab-count" aria-hidden="true">{{ counterLabel }}</span>
            <span v-if="notifications.unreadCount.value" class="sr-only">，{{ notifications.unreadCount.value }} 条未读</span>
          </button>
          <button
            id="notification-tab-announcement"
            type="button"
            role="tab"
            class="notification-tab"
            aria-controls="notification-announcements"
            :aria-selected="tab === 'announcement'"
            :tabindex="tab === 'announcement' ? 0 : -1"
            @click="selectTab('announcement')"
          >
            公告
            <span v-if="announcements.hasUnread.value" class="notification-tab-dot" aria-hidden="true" />
            <span v-if="announcements.hasUnread.value" class="sr-only">，有待查阅公告</span>
          </button>
        </div>

        <NotificationInbox
          v-show="tab === 'inbox'"
          id="notification-inbox"
          role="tabpanel"
          aria-labelledby="notification-tab-inbox"
          @navigate="closePanel(false)"
          @review="openReview"
        />

        <!-- ~ 历史公告列表 -->
        <!-- * 说明：按条展示发布日期、公告类型和查阅状态 -->
        <!-- * 交互：点击条目查看详情；可刷新、翻页和调整每页数量 -->
        <!-- * 依赖：已登录会话与 Go 提供的当前页版本 -->
        <!-- * 边界：打开或翻阅列表不会写回执 -->
        <NotificationFeed
          v-show="tab === 'announcement'"
          id="notification-announcements"
          role="tabpanel"
          aria-labelledby="notification-tab-announcement"
          :loading="announcements.historyLoading.value"
          :error="announcements.historyError.value"
          :empty="!announcements.history.value.length"
          empty-label="暂无历史公告"
          :page="announcements.historyPage.value"
          :page-count="pageCount"
          :total="announcements.historyTotal.value"
          @retry="refreshCurrentTab"
          @page="announcements.loadHistory"
        >
          <li v-for="entry in announcements.history.value" :key="`${entry.id}:${entry.revision}`">
            <button type="button" class="notification-row" :class="{'is-unread': !entry.read}" @click="openDetail(entry)">
              <span class="notification-row-icon" aria-hidden="true">
                <component :is="entry.kind === 'promotion' ? Gift : Megaphone" :size="18" :stroke-width="1.7" />
                <span v-if="!entry.read" class="notification-row-dot" />
              </span>
              <span class="notification-row-content">
                <span class="notification-row-heading">
                  <span class="notification-row-title" :title="entry.title">{{ entry.title }}</span>
                  <time class="notification-row-time" :datetime="entry.publishedAt" :title="formatTime(entry.publishedAt)">{{ formatDate(entry.publishedAt) }}</time>
                </span>
                <span class="notification-row-meta">{{ kindLabel(entry.kind) }} · {{ entry.read ? '已查阅' : '未查阅' }}</span>
              </span>
            </button>
          </li>
          <template #controls>
            <n-select
              v-model:show="pageSizeMenuOpen"
              class="announcement-history-page-size"
              size="small"
              aria-label="每页公告数量"
              :value="announcements.historyPageSize.value"
              :options="pageSizeOptions"
              :disabled="announcements.historyLoading.value"
              :to="false"
              @update:value="(size: number) => announcements.loadHistory(1, size)"
            />
          </template>
        </NotificationFeed>
      </section>
    </Transition>
  </Teleport>

  <ForumReviewNotice v-if="reviewID && !props.disabled" :review-id="reviewID" @close="closeReview" />

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
    @after-leave="restoreTriggerFocus"
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
        <n-button v-if="announcements.historyDetailError.value && !announcements.historyDetail.value" secondary @click="reopenAnnouncementList">重新加载列表</n-button>
        <n-button v-if="announcements.historyDetail.value && !announcements.historyDetail.value.read && !detailExpired" type="primary" :loading="announcements.historyDetailPending.value" :disabled="!announcements.historyDetailReady.value" @click="acknowledgeDetail">已查阅</n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<style scoped lang="scss">
@use '../../styles/announcement-body';

.notification-trigger {
  --wails-draggable: no-drag;
  position: relative;
  display: grid;
  width: 30px;
  height: 30px;
  place-items: center;
  border: 1px solid transparent;
  border-radius: 4px;
  background: transparent;
  color: var(--ds-text-muted);
  cursor: pointer;
}

.notification-trigger:hover:not(:disabled),
.notification-trigger[aria-expanded='true'] {
  border-color: var(--ds-border-default);
  background: var(--ds-active-bg);
  color: var(--ds-brand);
}

.notification-trigger:disabled {
  color: var(--ds-text-disabled);
  cursor: default;
}

.notification-trigger-dot {
  position: absolute;
  top: 4px;
  right: 4px;
  width: 7px;
  height: 7px;
  border: 1px solid var(--ds-bg-panel-elevated);
  border-radius: 50%;
  background: var(--ds-error);
}

.notification-drawer {
  position: absolute;
  inset: 8px 0 10px auto;
  display: flex;
  width: clamp(370px, 34vw, 416px);
  max-width: calc(100% - 12px);
  flex-direction: column;
  overflow: hidden;
  border: 1px solid var(--ds-border-default);
  border-right: 0;
  border-radius: 8px 0 0 8px;
  background: var(--ds-bg-panel-elevated);
  box-shadow: -8px 3px 24px rgb(0 0 0 / 16%);
  color: var(--ds-text-primary);
  pointer-events: auto;
}

.notification-drawer:focus {
  outline: none;
}

.notification-drawer[inert] {
  pointer-events: none;
}

.notification-drawer-enter-active,
.notification-drawer-leave-active {
  transition:
    transform 240ms cubic-bezier(0.32, 0.72, 0, 1),
    opacity 180ms ease;
}

.notification-drawer-enter-from,
.notification-drawer-leave-to {
  transform: translateX(100%);
  opacity: 0;
}

@media (prefers-reduced-motion: reduce) {
  .notification-drawer-enter-active,
  .notification-drawer-leave-active {
    transition: opacity 120ms ease;
  }

  .notification-drawer-enter-from,
  .notification-drawer-leave-to {
    transform: none;
  }
}

.notification-drawer-header {
  display: flex;
  min-height: 56px;
  flex: 0 0 auto;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 0 14px 0 20px;
}

.notification-drawer-header h2 {
  margin: 0;
  font-size: 18px;
  font-weight: 700;
  line-height: 1.35;
}

.notification-drawer-actions {
  display: flex;
  gap: 4px;
}

.notification-drawer-actions button {
  display: grid;
  width: 30px;
  height: 30px;
  place-items: center;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--ds-text-secondary);
  cursor: pointer;
}

.notification-drawer-actions button:hover:not(:disabled) {
  background: var(--ds-hover-bg);
  color: var(--ds-text-primary);
}

.notification-drawer-actions button:active:not(:disabled) {
  background: var(--ds-active-bg);
}

.notification-drawer-actions button:disabled {
  color: var(--ds-text-disabled);
  cursor: default;
}

.notification-tabs {
  display: flex;
  min-height: 42px;
  flex: 0 0 auto;
  gap: 23px;
  padding: 0 20px;
  border-bottom: 1px solid var(--ds-border-subtle);
}

.notification-tab {
  position: relative;
  display: flex;
  min-width: 42px;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 0 2px;
  border: 0;
  background: transparent;
  color: var(--ds-text-secondary);
  font-size: 14px;
  cursor: pointer;
}

.notification-tab:hover,
.notification-tab[aria-selected='true'] {
  color: var(--ds-text-primary);
}

.notification-tab[aria-selected='true'] {
  font-weight: 700;
}

.notification-tab[aria-selected='true']::after {
  position: absolute;
  right: 2px;
  bottom: -1px;
  left: 2px;
  height: 2px;
  border-radius: 1px 1px 0 0;
  background: var(--ds-brand);
  content: '';
}

.notification-tab-count {
  min-width: 20px;
  padding: 0 5px;
  border-radius: 9px;
  background: var(--ds-active-bg);
  color: var(--ds-brand);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  line-height: 17px;
  text-align: center;
}

.notification-tab-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--ds-brand);
}

.notification-trigger:focus-visible,
.notification-drawer-actions button:focus-visible,
.notification-tab:focus-visible {
  outline: 2px solid var(--ds-brand);
  outline-offset: -2px;
}

.announcement-history-page-size {
  width: 96px;
}

.announcement-history-detail-meta {
  display: grid;
  gap: 3px;
  margin-bottom: 12px;
  color: var(--ds-text-secondary);
  font-size: 12px;
  overflow-wrap: anywhere;
}

.announcement-history-state {
  display: grid;
  min-height: 130px;
  place-items: center;
  padding: 18px;
  color: var(--ds-text-secondary);
  font-size: 13px;
}

.announcement-history-expired {
  margin-top: 12px;
  color: var(--ds-text-secondary);
  font-size: 12px;
}
</style>
