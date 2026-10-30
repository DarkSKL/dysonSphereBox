import {computed, ref} from 'vue'
import {BrowserOpenURL, WindowIsMinimised} from '../../wailsjs/runtime/runtime'
import {
  captureAnnouncementSession,
  fetchAnnouncementHistory,
  fetchAnnouncementHistoryItem,
  fetchLatestAnnouncement,
  markAnnouncementHistoryRead,
  markAnnouncementRead,
  type NoticeHistoryEntry,
  type NoticeItem,
} from '../services/announcement'
import {renderAnnouncement} from '../utils/announcement-content'
import {serverEffectiveAddress} from '../services/server'
import {accountSessionKey, getAccountSessionRevision} from '../services/account'

// =============== 共享公告状态 ===============
// 说明：启动弹窗与历史入口共用账号作用域；查看和关闭只更新展示，用户按钮才提交已读。
const item = ref<NoticeItem | null>(null)
const html = ref('')
const open = ref(false)
const ready = ref(false)
const pending = ref(false)
const checking = ref(false)
const error = ref('')
const sessionID = ref('')
const visibleReady = ref(false)
const history = ref<NoticeHistoryEntry[]>([])
const historyPage = ref(1)
const historyPageSize = ref(20)
const historyTotal = ref(0)
const historyLoading = ref(false)
const historyError = ref('')
const historyOpen = ref(false)
const historyDetail = ref<NoticeHistoryEntry | null>(null)
const historyDetailOpen = ref(false)
const historyDetailLoading = ref(false)
const historyDetailHtml = ref('')
const historyDetailReady = ref(false)
const historyDetailPending = ref(false)
const historyDetailError = ref('')
// 自动检查按账号作用域去重：只有成功取得最新公告结果后才记为该作用域已完成，失败允许聚焦或手动重试。
let automaticScope = ''
let requestSequence = 0
let historySequence = 0
let historyDetailSequence = 0
let historyServerTime = 0
let historyRequestStartedAt = 0
let historyExpiresAtMono = 0
let expiresAtMono = 0
// 说明：作用域同时包含后端已验证身份和前端当前服务器地址；身份切换、切服或 token 代次变化都会使旧结果失效。
// 边界：尚未完成账号验证时返回空串，自动检查不得据此发起公告请求。
const currentScope = (): string => {
  const sessionKey = accountSessionKey.value
  return sessionKey ? `${getAccountSessionRevision()}/${sessionKey}/${serverEffectiveAddress.value}` : ''
}

// =============== 公告查询与用户操作 ===============
// 说明：所有异步结果同时核对请求次序和账号代次；旧响应不得修改当前展示或查阅状态。
export function useAnnouncements() {
  const title = computed(() => item.value?.title ?? '')
  const hasUnread = computed(() => Boolean(item.value) || history.value.some((entry) => !entry.read))
  // 同一账号作用域只自动检查一次，普通历史浏览不会重新触发启动弹窗。
  const check = async (manual = false): Promise<void> => {
    const scope = currentScope()
    if (!scope) {
      if (manual) {
        error.value = '请先登录账号并完成账号信息同步。'
        open.value = true
      }
      return
    }
    if (!manual && automaticScope === scope) return
    if (checking.value || pending.value || historyLoading.value || historyDetailPending.value) return
    checking.value = true
    item.value = null
    html.value = ''
    open.value = false
    ready.value = false
    visibleReady.value = false
    error.value = ''
    const sequence = ++requestSequence
    try {
      const session = sessionID.value || await captureAnnouncementSession()
      if (sequence !== requestSequence || scope !== currentScope()) return
      sessionID.value = session
      const requestStartedAt = performance.now()
      const latest = await fetchLatestAnnouncement(session)
      if (sequence !== requestSequence || scope !== currentScope()) return
      // 服务器已给出确定性结果（含“暂无公告”），同一作用域不再自动重复查询；失败分支不写入，允许重试。
      automaticScope = scope
      if (!latest.item) { item.value = null; open.value = false; return }
      const serverNow = Date.parse(latest.serverTime)
      if (!Number.isFinite(serverNow)) throw new Error('公告服务器时间无效')
      expiresAtMono = latest.item.endsAt ? requestStartedAt + (Date.parse(latest.item.endsAt) - serverNow) : Number.POSITIVE_INFINITY
      if (expiresAtMono <= performance.now()) { open.value = false; return }
      item.value = latest.item
      if (latest.item.contentFormat !== 'html' && latest.item.contentFormat !== 'markdown') throw new Error('公告格式不受支持')
      const configuredServer = serverEffectiveAddress.value || 'http://127.0.0.1:8888'
      html.value = renderAnnouncement(latest.item.contentFormat, latest.item.content, new URL(configuredServer).origin)
      ready.value = false
      visibleReady.value = Boolean(html.value)
      error.value = visibleReady.value ? '' : '公告正文无法显示。'
      open.value = Boolean(html.value)
    } catch (cause) {
      if (sequence !== requestSequence || scope !== currentScope()) return
      error.value = cause instanceof Error ? cause.message : String(cause)
      if (manual) open.value = true
    } finally {
      if (sequence === requestSequence) checking.value = false
    }
  }
  const resetAutoAttempt = (): void => { automaticScope = '' }
  const closeHistoryDetail = (): void => {
    historyDetailSequence++
    historyDetailOpen.value = false
    historyDetailLoading.value = false
    historyDetail.value = null
    historyDetailHtml.value = ''
    historyDetailReady.value = false
    historyDetailError.value = ''
    historyDetailPending.value = false
    historyExpiresAtMono = 0
  }
  const resetSession = (): void => {
    requestSequence++
    historySequence++
    automaticScope = ''
    item.value = null
    html.value = ''
    open.value = false
    ready.value = false
    visibleReady.value = false
    sessionID.value = ''
    error.value = ''
    checking.value = false
    pending.value = false
    history.value = []
    historyPage.value = 1
    historyPageSize.value = 20
    historyTotal.value = 0
    historyLoading.value = false
    historyError.value = ''
    historyOpen.value = false
    closeHistoryDetail()
  }
  // 每次查询先撤销旧页选择权，错误时保留分页位置供重试，并允许减少每页数量。
  const loadHistory = async (page = 1, pageSize = historyPageSize.value): Promise<void> => {
    if (historyLoading.value || historyDetailPending.value || checking.value || pending.value) return
    if (page < 1 || ![1, 5, 10, 20].includes(pageSize)) return
    const sequence = ++historySequence
    const scope = currentScope()
    historyLoading.value = true
    historyOpen.value = true
    historyPage.value = page
    historyPageSize.value = pageSize
    historyError.value = ''
    history.value = []
    closeHistoryDetail()
    try {
      if (!sessionID.value) {
        const session = await captureAnnouncementSession()
        if (sequence !== historySequence || scope !== currentScope()) return
        sessionID.value = session
      }
      const requestStartedAt = performance.now()
      const result = await fetchAnnouncementHistory(sessionID.value, page, pageSize)
      if (sequence !== historySequence || scope !== currentScope()) return
      historyServerTime = Date.parse(result.serverTime)
      if (!Number.isFinite(historyServerTime)) throw new Error('公告服务器时间无效')
      historyRequestStartedAt = requestStartedAt
      history.value = result.list ?? []
      historyPage.value = result.page
      historyPageSize.value = result.pageSize
      historyTotal.value = result.total
      historyOpen.value = true
    } catch (cause) {
      if (sequence === historySequence && scope === currentScope()) historyError.value = cause instanceof Error ? cause.message : String(cause)
    } finally {
      if (sequence === historySequence && scope === currentScope()) historyLoading.value = false
    }
  }
  // 详情必须取自Go保存的当前页，避免任意ID或已失效分页建立确认权限。
  const openHistoryDetail = async (entry: NoticeHistoryEntry): Promise<void> => {
    if (!sessionID.value || historyLoading.value || historyDetailPending.value) return
    const sequence = historySequence
    const detailSequence = ++historyDetailSequence
    const scope = currentScope()
    historyDetailOpen.value = true
    historyDetailLoading.value = true
    historyDetail.value = null
    historyDetailHtml.value = ''
    historyDetailReady.value = false
    historyDetailError.value = ''
    historyDetailPending.value = false
    try {
      const detail = await fetchAnnouncementHistoryItem(sessionID.value, entry.id, entry.revision)
      if (sequence !== historySequence || detailSequence !== historyDetailSequence || scope !== currentScope()) return
      historyDetail.value = detail
      historyExpiresAtMono = detail.endsAt ? historyRequestStartedAt + Date.parse(detail.endsAt) - historyServerTime : Number.POSITIVE_INFINITY
      const configuredServer = serverEffectiveAddress.value || 'http://127.0.0.1:8888'
      if (detail.contentFormat !== 'html' && detail.contentFormat !== 'markdown') throw new Error('公告格式不受支持')
      const rendered = renderAnnouncement(detail.contentFormat, detail.content, new URL(configuredServer).origin)
      historyDetailHtml.value = rendered
      historyDetailError.value = rendered ? '' : '公告正文无法显示。'
    } catch (cause) {
      if (sequence === historySequence && detailSequence === historyDetailSequence && scope === currentScope()) historyDetailError.value = cause instanceof Error ? cause.message : String(cause)
    } finally {
      if (sequence === historySequence && detailSequence === historyDetailSequence && scope === currentScope()) historyDetailLoading.value = false
    }
  }
  const isHistoryExpired = (): boolean => historyExpiresAtMono <= performance.now()
  // 正文真实可见只解锁按钮和延迟图片，过期条目依然允许查看。
  const verifyHistoryVisible = async (element: HTMLElement | null): Promise<void> => {
    const sequence = historyDetailSequence
    historyDetailReady.value = false
    if (!element || !historyDetailOpen.value || !historyDetailHtml.value || document.visibilityState !== 'visible') return
    try { if (await WindowIsMinimised()) return } catch { return }
    if (sequence !== historyDetailSequence || !element.isConnected || document.visibilityState !== 'visible' || !historyDetailOpen.value) return
    const rect = element.getBoundingClientRect()
    const visible = rect.width > 0 && rect.height > 0 && rect.bottom > 0 && rect.right > 0 && rect.top < innerHeight && rect.left < innerWidth
    const style = getComputedStyle(element)
    if (!visible || style.visibility !== 'visible' || style.display === 'none' || document.querySelector('.driver-overlay, .driver-active-element')) return
    element.querySelectorAll<HTMLImageElement>('img[data-announcement-src]').forEach((image) => image.src = image.dataset.announcementSrc ?? '')
    historyDetailReady.value = !isHistoryExpired()
  }
  const acknowledgeHistory = async (): Promise<void> => {
    const selected = historyDetail.value
    if (!selected || selected.read || !historyDetailOpen.value || !historyDetailReady.value || isHistoryExpired() || document.visibilityState !== 'visible' || historyDetailPending.value || !sessionID.value) return
    const sequence = historySequence
    const detailSequence = historyDetailSequence
    const scope = currentScope()
    historyDetailPending.value = true
    historyDetailError.value = ''
    try {
      const receipt = await markAnnouncementHistoryRead(sessionID.value, selected.id, selected.revision)
      if (sequence === historySequence && detailSequence === historyDetailSequence && scope === currentScope() && receipt.id === selected.id && receipt.revision === selected.revision) {
        applyReadReceipt(selected.id, selected.revision, receipt.firstReadAt)
      }
    } catch (cause) {
      if (sequence === historySequence && detailSequence === historyDetailSequence && scope === currentScope()) historyDetailError.value = cause instanceof Error ? cause.message : String(cause)
    } finally {
      if (sequence === historySequence && detailSequence === historyDetailSequence && scope === currentScope()) historyDetailPending.value = false
    }
  }
  const markVisible = (element: HTMLElement | null): void => {
    if (!open.value || !visibleReady.value || !element || document.visibilityState !== 'visible' || expiresAtMono <= performance.now()) { ready.value = false; return }
    const rect = element.getBoundingClientRect()
    const visible = rect.width > 0 && rect.height > 0 && rect.bottom > 0 && rect.right > 0 && rect.top < innerHeight && rect.left < innerWidth
    const style = getComputedStyle(element)
    ready.value = element.isConnected && visible && style.visibility === 'visible' && style.display !== 'none' && !document.querySelector('.driver-overlay, .driver-active-element')
    if (ready.value) element.querySelectorAll<HTMLImageElement>('img[data-announcement-src]').forEach((image) => image.src = image.dataset.announcementSrc ?? '')
  }
  const verifyVisible = async (element: HTMLElement | null): Promise<void> => {
    const sequence = requestSequence
    if (!element || document.visibilityState !== 'visible') { ready.value = false; return }
    try { if (await WindowIsMinimised()) { ready.value = false; return } } catch { ready.value = false; return }
    if (sequence === requestSequence && document.visibilityState === 'visible' && open.value) markVisible(element)
  }
  const acknowledge = async (): Promise<void> => {
    if (!ready.value || expiresAtMono <= performance.now() || document.visibilityState !== 'visible' || pending.value || !item.value || !sessionID.value) return
    const sequence = requestSequence
    const scope = currentScope()
    const submittedItem = item.value
    pending.value = true
    try {
      const receipt = await markAnnouncementRead(sessionID.value, submittedItem.id, submittedItem.revision)
      if (sequence === requestSequence && currentScope() === scope && receipt.id === submittedItem.id && receipt.revision === submittedItem.revision) {
        applyReadReceipt(submittedItem.id, submittedItem.revision, receipt.firstReadAt)
        open.value = false
        item.value = null
      }
    } catch (cause) {
      if (sequence === requestSequence && currentScope() === scope) error.value = cause instanceof Error ? cause.message : String(cause)
    }
    finally {
      if (sequence === requestSequence && currentScope() === scope) pending.value = false
    }
  }
  const close = (): void => { open.value = false; ready.value = false }
  // 最新弹窗与历史详情共享同一首次回执，避免徽标或已加载行残留未读状态。
  const applyReadReceipt = (id: string, revision: number, readAt: string): void => {
    const row = history.value.find((entry) => entry.id === id && entry.revision === revision)
    if (row) { row.read = true; row.readAt = readAt }
    if (historyDetail.value?.id === id && historyDetail.value.revision === revision) {
      historyDetail.value.read = true
      historyDetail.value.readAt = readAt
    }
    if (item.value?.id === id && item.value.revision === revision) {
      close()
      item.value = null
    }
  }
  // 清洗器已移除href；仅委托鼠标主键/中键和Enter，防止右键打开或整块正文吞掉Enter。
  const openLink = (event: Event): void => {
    if (!(event.target instanceof Element)) return
    const target = event.target.closest<HTMLAnchorElement>('a[data-external-href]')
    const href = target?.dataset.externalHref
    if (!href) return
    if (event instanceof MouseEvent && event.button > 1) return
    event.preventDefault(); event.stopPropagation(); void BrowserOpenURL(href)
  }
  return {item, title, html, open, ready, pending, checking, error, hasUnread, check, acknowledge, close, openLink, markVisible, verifyVisible, resetAutoAttempt, resetSession, history, historyPage, historyPageSize, historyTotal, historyLoading, historyError, historyOpen, historyDetail, historyDetailOpen, historyDetailLoading, historyDetailHtml, historyDetailReady, historyDetailPending, historyDetailError, loadHistory, openHistoryDetail, closeHistoryDetail, acknowledgeHistory, verifyHistoryVisible, isHistoryExpired}
}
