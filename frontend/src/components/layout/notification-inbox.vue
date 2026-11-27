<script setup lang="ts">
import {ChevronDown, ClipboardCheck, Mail, MailX, MessageSquare} from '@lucide/vue'
import {useMessage} from 'naive-ui'
import {computed, ref, watch} from 'vue'
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
const {inbox, requestedPage, listLoading, listError, actionError, reading, refresh, changePage, markRead} = useNotifications()

// =============== 站内信列表与正文 ===============
// 说明：列表显示摘要，同一时间展开一条的完整正文；已撤回消息仅保留状态和时间。
const expandedID = ref('')
const pageCount = computed(() => Math.max(1, Math.ceil((inbox.value?.total ?? 0) / (inbox.value?.pageSize ?? 20))))
watch([accountSessionKey, requestedPage], () => { expandedID.value = '' })

const entryIcon = (entry: InboxEntry) => {
  if (entry.recalledAt) return MailX
  if (entry.bizType === 'feedback') return MessageSquare
  if (entry.bizType === 'forum.review') return ClipboardCheck
  return Mail
}
const isUnread = (entry: InboxEntry): boolean => !entry.readAt && !entry.recalledAt
const toggleEntry = (entry: InboxEntry): void => {
  if (!entry.recalledAt) expandedID.value = expandedID.value === entry.ID ? '' : entry.ID
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
  <!-- ~ 站内信列表 -->
  <!-- * 说明：左侧按业务显示图标，右侧对齐时间和正文展开操作 -->
  <!-- * 交互：点击条目展开正文，随后可标读或进入对应业务 -->
  <!-- * 依赖：账号收件箱、分页和反馈星系深链 -->
  <!-- * 边界：已撤回的内容和业务入口不可见；展开正文不会标为已读 -->
  <NotificationFeed
    :loading="listLoading"
    :error="listError"
    :empty="!inbox?.list.length"
    empty-label="暂无站内信"
    :page="requestedPage"
    :page-count="pageCount"
    :total="inbox?.total ?? 0"
    @retry="refresh()"
    @page="changePage"
  >
    <li v-for="entry in inbox?.list" :key="entry.ID">
      <button
        type="button"
        class="notification-row"
        :class="{'is-unread': isUnread(entry)}"
        :disabled="!!entry.recalledAt"
        :aria-expanded="entry.recalledAt ? undefined : expandedID === entry.ID"
        @click="toggleEntry(entry)"
      >
        <span class="notification-row-icon" aria-hidden="true">
          <component :is="entryIcon(entry)" :size="18" :stroke-width="1.7" />
          <span v-if="isUnread(entry)" class="notification-row-dot" />
        </span>
        <span class="notification-row-content">
          <span class="notification-row-heading">
            <span class="notification-row-title">{{ entry.recalledAt ? '消息已撤回' : entry.title }}</span>
            <time class="notification-row-time" :datetime="entry.createdAt" :title="formatTime(entry.createdAt)">{{ formatDate(entry.createdAt) }}</time>
          </span>
          <span v-if="!entry.recalledAt" class="inbox-row-subline">
            <span v-if="expandedID !== entry.ID" class="notification-row-meta">{{ entry.body }}</span>
            <ChevronDown class="inbox-row-chevron" :size="15" :stroke-width="1.7" aria-hidden="true" />
          </span>
          <span v-if="isUnread(entry)" class="sr-only">未读</span>
        </span>
      </button>

      <div v-if="expandedID === entry.ID && !entry.recalledAt" class="inbox-entry-detail">
        <p v-if="entry.body" class="inbox-entry-body">{{ entry.body }}</p>
        <div v-if="isUnread(entry) || canOpen(entry) || canOpenReview(entry)" class="inbox-entry-actions">
          <n-button v-if="isUnread(entry)" secondary size="small" :loading="reading === entry.ID" :disabled="!!reading" @click="markRead(entry)">标为已读</n-button>
          <n-button v-if="canOpen(entry)" text type="primary" size="small" @click="openFeedback(entry)">查看反馈</n-button>
          <n-button v-if="canOpenReview(entry)" text type="primary" size="small" @click="emit('review', entry.bizID)">查看审核结果</n-button>
        </div>
      </div>
    </li>
    <template v-if="actionError" #feedback>
      <p class="inbox-action-error" role="alert">{{ actionError }}</p>
    </template>
  </NotificationFeed>
</template>

<style scoped lang="scss">
.inbox-row-subline {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 8px;
}

.inbox-row-chevron {
  flex: 0 0 auto;
  margin-left: auto;
  color: var(--ds-text-muted);
}

.notification-row[aria-expanded='true'] .inbox-row-chevron {
  transform: rotate(180deg);
}

.notification-row[aria-expanded='true'] .notification-row-title {
  white-space: normal;
  overflow-wrap: anywhere;
}

.inbox-entry-detail {
  display: grid;
  gap: 10px;
  padding: 10px var(--notification-row-padding) 14px;
  padding-left: calc(var(--notification-row-padding) + var(--notification-icon-size) + var(--notification-row-gap));
}

.inbox-entry-detail:empty {
  display: none;
}

.inbox-entry-body {
  margin: 0;
  color: var(--ds-text-primary);
  font-size: 13px;
  line-height: 1.6;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
}

.inbox-entry-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 12px;
}

.inbox-action-error {
  margin: 0;
  padding: 8px 20px;
  color: var(--ds-error);
  font-size: 12px;
  overflow-wrap: anywhere;
}
</style>
