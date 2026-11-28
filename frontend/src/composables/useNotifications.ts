import {computed, ref, shallowRef, watch} from 'vue'
import {EventsOn} from '../../wailsjs/runtime/runtime'
import {accountSessionKey, syncNotificationSession} from '../pages/services/account'
import {
  fetchNotificationInbox,
  fetchNotificationUnread,
  markNotificationRead,
  type InboxEntry,
  type InboxPage,
  type UnreadCount,
} from '../services/notifications'
import {useAnnouncements} from './useAnnouncements'

// =============== 应用级消息状态 ===============
// 说明：切换账号时整体作废同步队列；连续 SSE 事件合并为当前一轮和至多一轮补查。
interface RefreshScope {
  key: string
  unread: boolean
  list: boolean
  more: boolean
  running: boolean
}

const announcements = useAnnouncements()
const panelOpen = ref(false)
const announcementTab = ref(false)
const unread = shallowRef<UnreadCount | null>(null)
const inbox = shallowRef<Pick<InboxPage, 'page' | 'list'> | null>(null)
const hasMore = ref(false)
const moreError = ref('')
const listLoading = ref(false)
const listError = ref('')
const actionError = ref('')
const reading = ref('')
const unreadCount = computed(() => unread.value?.total ?? 0)
const hasUnread = computed(() => unreadCount.value > 0 || announcements.hasUnread.value)
let scope: RefreshScope | undefined
let syncAnnouncement = false

function isCurrent(current: RefreshScope): boolean {
  return scope === current && current.key === accountSessionKey.value
}

// 后端查询没有前端副作用，只有请求归属与当前页面匹配时才更新数据。
async function loadCount(current: RefreshScope): Promise<void> {
  try {
    const count = await fetchNotificationUnread(current.key)
    if (isCurrent(current)) unread.value = count
  } catch {
    if (isCurrent(current)) void syncNotificationSession(current.key)
  }
}

// 续载只读取下一批；刷新重读已经展示的范围，使新增、撤回及已读状态同步生效。
async function loadInbox(current: RefreshScope, append: boolean): Promise<void> {
  const previous = inbox.value
  const last = previous?.page ?? 1
  const first = append ? last + 1 : 1
  const end = append ? first : last
  const entries = new Map<string, InboxEntry>()
  if (append) {
    for (const entry of previous?.list ?? []) entries.set(entry.ID, entry)
  }
  listLoading.value = true
  listError.value = ''
  moreError.value = ''
  try {
    for (let page = first; page <= end; page++) {
      const result = await fetchNotificationInbox(current.key, page)
      if (!isCurrent(current)) return
      for (const entry of result.list) entries.set(entry.ID, entry)
      const remaining = result.list.length > 0 && page * result.pageSize < result.total
      if (!remaining || page === end) {
        inbox.value = {page, list: [...entries.values()]}
        hasMore.value = remaining
        break
      }
    }
  } catch (error) {
    if (isCurrent(current)) {
      const target = append ? moreError : listError
      target.value = error instanceof Error ? error.message : String(error)
      void syncNotificationSession(current.key)
    }
  } finally {
    if (isCurrent(current)) listLoading.value = false
  }
}

// 先消费本轮刷新标志，执行中到达的信号会在本轮完成后补查。
async function drain(current: RefreshScope): Promise<void> {
  current.running = true
  try {
    while (isCurrent(current) && (current.unread || current.list || current.more)) {
      const count = current.unread
      const list = current.list && panelOpen.value
      const append = current.more && panelOpen.value && !announcementTab.value
      current.more = false
      current.unread = false
      current.list = false
      await Promise.all([
        count ? loadCount(current) : undefined,
        list || append ? loadInbox(current, !list) : undefined,
      ])
    }
  } finally {
    current.running = false
  }
}

function refresh(count = true, list = panelOpen.value): void {
  if (!scope) return
  scope.unread ||= count
  scope.list ||= list
  if (!scope.running) void drain(scope)
}

// =============== 旧公告同步 ===============
// 说明：连接成功后重查公告；等待正在展示的历史详情和用户确认完成，以保留其版本绑定。
function flushAnnouncements(): void {
  if (!scope || !syncAnnouncement || announcements.checking.value || announcements.pending.value || announcements.historyDetailOpen.value || announcements.historyDetailPending.value) return
  syncAnnouncement = false
  void announcements.check(false, true)
}

// =============== 应用监听 ===============
// 说明：在账号引导前接收 Wails 事件；账号首次可用时补查，即使连接或事件先于界面初始化。
export function installNotificationSync(): () => void {
  const disposeScope = watch(accountSessionKey, (key) => {
    scope = key ? {key, unread: false, list: false, more: false, running: false} : undefined
    unread.value = null
    inbox.value = null
    panelOpen.value = false
    announcementTab.value = false
    hasMore.value = false
    moreError.value = ''
    listLoading.value = false
    listError.value = ''
    actionError.value = ''
    reading.value = ''
    syncAnnouncement = false
    announcements.resetSession()
    if (key) {
      refresh()
      void announcements.check()
    }
  }, {immediate: true, flush: 'sync'})
  const disposeAnnouncement = watch([
    announcements.checking, announcements.pending, announcements.historyDetailOpen, announcements.historyDetailPending,
  ], flushAnnouncements)
  const runtime = window as Window & {runtime?: {EventsOnMultiple?: unknown}}
  const disposeEvents = typeof runtime.runtime?.EventsOnMultiple === 'function'
    ? EventsOn('system:notifications', (key: string, event: string) => {
        if (scope?.key !== key) return
        if (event === 'invalidated') {
          void syncNotificationSession(key)
          return
        }
        if (event === 'connected') {
          syncAnnouncement = true
          flushAnnouncements()
          if (panelOpen.value && announcementTab.value) void announcements.loadHistory()
        }
        if (event === 'connected' || event === 'notify:message' || event === 'notify:recall') refresh()
      })
    : undefined
  const retry = (): void => { if (scope) void announcements.check() }
  window.addEventListener('focus', retry)
  window.addEventListener('online', retry)
  return () => {
    window.removeEventListener('focus', retry)
    window.removeEventListener('online', retry)
    disposeEvents?.()
    disposeScope()
    disposeAnnouncement()
    scope = undefined
  }
}

// =============== 面板操作 ===============
// 说明：面板打开和翻页只查询；标读有独立的提交反馈，切号后丢弃任何旧结果。
export function useNotifications() {
  function setPanelOpen(open: boolean): void {
    panelOpen.value = open && !!scope
    if (panelOpen.value) refresh()
  }

  function setAnnouncementTab(active: boolean): void {
    announcementTab.value = active
  }

  function loadMore(): void {
    if (!scope || !panelOpen.value || announcementTab.value || listLoading.value || !hasMore.value) return
    scope.more = true
    if (!scope.running) void drain(scope)
  }

  async function markRead(entry: InboxEntry): Promise<void> {
    const current = scope
    if (!current || reading.value || entry.readAt || entry.recalledAt) return
    reading.value = entry.ID
    actionError.value = ''
    try {
      await markNotificationRead(current.key, entry.ID)
      if (isCurrent(current)) refresh()
    } catch (error) {
      if (isCurrent(current)) {
        actionError.value = error instanceof Error ? error.message : String(error)
        void syncNotificationSession(current.key)
      }
    } finally {
      if (isCurrent(current)) reading.value = ''
    }
  }

  return {unreadCount, hasUnread, inbox, hasMore, moreError, listLoading, listError, actionError, reading, setPanelOpen, setAnnouncementTab, loadMore, markRead, refresh}
}
