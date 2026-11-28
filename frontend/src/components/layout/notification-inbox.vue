<script setup lang="ts">
import {ArrowLeft, Check, ChevronRight, ClipboardCheck, Mail, MailX, MessageSquare} from '@lucide/vue'
import {useMessage} from 'naive-ui'
import {computed, nextTick, ref, watch} from 'vue'
import {isNavigationFailure, NavigationFailureType, useRouter} from 'vue-router'
import {useNotifications} from '../../composables/useNotifications'
import {accountSessionKey} from '../../pages/services/account'
import {reopenFeedbackDeepLink} from '../../services/feedback'
import type {InboxEntry} from '../../services/notifications'
import {formatDate} from '../../utils/format'
import NotificationFeed from './notification-feed.vue'

const emit = defineEmits<{
  (event: 'navigate'): void
  (event: 'review', id: string): void
}>()
const router = useRouter()
const message = useMessage()
const {inbox, hasMore, moreError, listLoading, listError, actionError, reading, refresh, loadMore, markRead} = useNotifications()

// =============== 站内信列表与正文 ===============
// 说明：列表与正文共用抽屉空间，阅读时保留列表滚动位置。
const selectedID = ref('')
const selected = computed(() => inbox.value?.list.find(entry => entry.ID === selectedID.value && !entry.recalledAt))
const backButton = ref<HTMLButtonElement | null>(null)
let opener: HTMLButtonElement | null = null
watch(accountSessionKey, () => { selectedID.value = '' })

const entryIcon = (entry: InboxEntry) => {
  if (entry.recalledAt) return MailX
  if (entry.bizType === 'feedback') return MessageSquare
  if (entry.bizType === 'forum.review') return ClipboardCheck
  return Mail
}
const isUnread = (entry: InboxEntry): boolean => !entry.readAt && !entry.recalledAt
// 阅读时将焦点移入详情，返回后回到原消息按钮。
async function openEntry(entry: InboxEntry, event: MouseEvent): Promise<void> {
  if (entry.recalledAt) return
  opener = event.currentTarget as HTMLButtonElement
  selectedID.value = entry.ID
  await nextTick()
  backButton.value?.focus({preventScroll: true})
}

async function returnToList(): Promise<void> {
  selectedID.value = ''
  await nextTick()
  opener?.focus({preventScroll: true})
  opener = null
}

const formatTime = (value: string): string => {
  const time = new Date(value)
  return Number.isNaN(time.getTime()) ? '' : time.toLocaleString()
}

// =============== 业务跳转 ===============
// 说明：合法的业务 ID 可打开反馈或审核结果；展开正文和跳转都由用户单独操作。
const canOpen = (entry: InboxEntry): boolean => !entry.recalledAt && entry.bizType === 'feedback' && /^[1-9]\d*$/.test(entry.bizID)
const canOpenReview = (entry: InboxEntry): boolean => !entry.recalledAt && entry.bizType === 'forum.review' && /^[1-9]\d*$/.test(entry.bizID)

// 打开星图深链，同页重复打开时重新定位到对应反馈。
async function openFeedback(entry: InboxEntry): Promise<void> {
  if (!canOpen(entry) || !accountSessionKey.value) return
  const key = accountSessionKey.value
  try {
    const failure = await router.push({name: '/community/feedback', query: {star: entry.bizID}})
    if (accountSessionKey.value !== key) return
    if (isNavigationFailure(failure, NavigationFailureType.duplicated)) reopenFeedbackDeepLink()
    else if (failure) throw failure
    emit('navigate')
  } catch {
    if (accountSessionKey.value === key) message.error('无法打开反馈详情，请重试。')
  }
}
</script>

<template>
  <div class="inbox-panel">
    <!-- ~ 站内信列表 -->
    <!-- * 说明：左侧按业务显示图标，右侧对齐时间和正文展开操作 -->
    <!-- * 交互：点击条目展开正文，随后可标读或进入对应业务 -->
    <!-- * 依赖：账号收件箱、滚动续载和反馈星系深链 -->
    <!-- * 边界：已撤回的内容和业务入口不可见；展开正文不会标为已读 -->
    <NotificationFeed
      v-show="!selected || !!listError"
      :loading="listLoading"
      :error="listError"
      :empty="!inbox?.list.length"
      empty-label="暂无站内信"
      :has-more="hasMore"
      :more-error="moreError"
      @more="loadMore"
      @retry="refresh()"
    >
      <li v-for="entry in inbox?.list" :key="entry.ID">
        <button
          type="button"
          class="notification-row"
          :class="{'is-unread': isUnread(entry)}"
          :disabled="!!entry.recalledAt"
          @click="openEntry(entry, $event)"
        >
          <span class="notification-row-icon" aria-hidden="true">
            <component :is="entryIcon(entry)" :size="18" :stroke-width="1.7" />
            <span v-if="isUnread(entry)" class="notification-row-dot" />
          </span>
          <span class="notification-row-content">
            <span class="notification-row-heading">
              <span class="notification-row-title">{{ entry.recalledAt ? '消息已撤回' : entry.title }}</span>
              <time class="notification-row-time" :datetime="entry.createdAt" :title="formatTime(entry.createdAt)">{{ formatDate(entry.createdAt) }}</time>
              <ChevronRight v-if="!entry.recalledAt" class="inbox-row-chevron" :size="14" aria-hidden="true" />
            </span>
            <span v-if="!entry.recalledAt" class="notification-row-meta">{{ entry.body }}</span>
            <span v-if="isUnread(entry)" class="sr-only">未读</span>
          </span>
        </button>
      </li>
    </NotificationFeed>
    <!-- ~ 站内信阅读视图 -->
    <!-- * 说明：业务来源、时间、标题和完整正文依次呈现；长消息在抽屉内滚动。 -->
    <!-- * 交互：返回保留列表位置，只有显式标读才更新回执。 -->
    <!-- * 边界：刷新后依据当前条目重新取值，已撤回消息无法继续阅读。 -->
    <article v-if="selected && !listError" class="inbox-entry-detail" aria-labelledby="inbox-detail-title">
      <button ref="backButton" type="button" class="inbox-back" @click="returnToList">
        <ArrowLeft :size="15" aria-hidden="true" />返回站内信
      </button>
      <header class="inbox-detail-heading">
        <div class="inbox-detail-meta">
          <component :is="entryIcon(selected)" :size="15" aria-hidden="true" />
          <span>{{ selected.bizType === 'forum.review' ? '论坛审核' : selected.bizType === 'feedback' ? '反馈星系' : '站内信' }}</span>
          <time :datetime="selected.createdAt">{{ formatTime(selected.createdAt) }}</time>
        </div>
        <h3 id="inbox-detail-title">{{ selected.title }}</h3>
      </header>
      <p v-if="selected.body" class="inbox-entry-body">{{ selected.body }}</p>
      <div class="inbox-entry-actions">
        <n-button v-if="canOpenReview(selected)" type="primary" size="small" @click="emit('review', selected.bizID)">查看审核结果</n-button>
        <n-button v-else-if="canOpen(selected)" type="primary" size="small" @click="openFeedback(selected)">查看反馈</n-button>
        <n-button v-if="isUnread(selected)" quaternary size="small" :loading="reading === selected.ID" :disabled="!!reading" @click="markRead(selected)">
          <template #icon><Check :size="14" aria-hidden="true" /></template>
          标为已读
        </n-button>
        <span v-else class="inbox-read" role="status"><Check :size="14" aria-hidden="true" />已读</span>
      </div>
    </article>
    <p v-if="actionError" class="inbox-action-error" role="alert">{{ actionError }}</p>
  </div>
</template>

<style scoped lang="scss">
.inbox-panel {
  display: flex;
  min-height: 0;
  flex: 1 1 0;
  flex-direction: column;
}

.notification-row-heading .notification-row-time {
  margin-inline-start: auto;
}

.inbox-row-chevron {
  flex: none;
  align-self: center;
  color: var(--ds-text-muted);
}

.inbox-entry-detail {
  flex: 1 1 0;
  min-height: 0;
  overflow: auto;
  overscroll-behavior: contain;
  scrollbar-gutter: stable;
  padding: 12px 20px 24px;
}

.inbox-back,
.inbox-read,
.inbox-detail-meta {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--ds-text-secondary);
  font-size: 12px;
}

.inbox-back {
  min-height: 32px;
  padding: 0 6px;
  margin-inline-start: -6px;
  border-radius: 4px;
}

.inbox-back:hover {
  background: var(--ds-hover-bg);
  color: var(--ds-brand);
}

.inbox-back:focus-visible {
  outline: 2px solid var(--ds-brand);
  outline-offset: 1px;
}

.inbox-detail-heading {
  margin-top: 16px;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--ds-border-default);
}

.inbox-detail-meta {
  flex-wrap: wrap;
}

.inbox-detail-meta time {
  margin-inline-start: 6px;
  font-variant-numeric: tabular-nums;
}

.inbox-detail-heading h3 {
  margin: 6px 0 0;
  font-size: 19px;
  font-weight: 700;
  line-height: 1.45;
  overflow-wrap: anywhere;
}

.inbox-entry-body {
  margin: 16px 0 0;
  color: var(--ds-text-primary);
  font-size: 14px;
  line-height: 1.75;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
}

.inbox-entry-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 12px;
  margin-top: 20px;
}

.inbox-action-error {
  margin: 0;
  padding: 8px 20px;
  color: var(--ds-error);
  font-size: 12px;
  overflow-wrap: anywhere;
}
</style>
