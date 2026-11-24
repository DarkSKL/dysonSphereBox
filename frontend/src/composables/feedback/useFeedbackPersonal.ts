import {onScopeDispose, shallowRef, watch} from 'vue'

import {accountSessionKey} from '../../pages/services/account'
import {feedbackCall} from '../../services/feedback'
import type {FeedbackNotice, MyStar, Page, PageRequest} from '../../types/feedback'

export const PERSONAL_PAGE_SIZE = 12

interface Collection<T> {
  page: Page<T>
  busy: boolean
  error: string
}

function initialCollection<T>(): Collection<T> {
  return {page: {page: 1, pageSize: PERSONAL_PAGE_SIZE, total: 0, list: []}, busy: false, error: ''}
}

// =============== 作者列表和收件箱 ===============
// 说明：个人读取随账号失效，提醒的未读数与列表使用同一分页响应。
export function useFeedbackPersonal(
  onOpen: (starId: string, contentId?: string) => void,
) {
  const mine = shallowRef(initialCollection<MyStar>())
  const notices = shallowRef(initialCollection<FeedbackNotice>())
  const unreadCount = shallowRef(0)
  const readingNotices = new Set<string>()
  let generation = 0
  let mineRequest = 0
  let noticeRequest = 0
  let minePage = 1
  let noticePage = 1
  let minePending = false
  let noticePending = false

  async function loadMine(page = minePage, quiet = false): Promise<void> {
    if (quiet && minePending) return
    minePage = page
    minePending = true
    const request = ++mineRequest
    mine.value = {...mine.value, busy: !quiet, error: ''}
    try {
      const result = await feedbackCall('Mine', pageRequest(page))
      if (request !== mineRequest) return
      mine.value = {page: result, busy: false, error: ''}
    } catch (error) {
      if (request !== mineRequest) return
      mine.value = {...mine.value, busy: false, error: error instanceof Error ? error.message : '无法读取我的星星。'}
    } finally {
      if (request === mineRequest) minePending = false
    }
  }

  async function loadNotices(page = noticePage, quiet = false): Promise<void> {
    if (quiet && noticePending) return
    noticePage = page
    noticePending = true
    const request = ++noticeRequest
    notices.value = {...notices.value, busy: !quiet, error: ''}
    try {
      const result = await feedbackCall('Notices', pageRequest(page))
      if (request !== noticeRequest) return
      unreadCount.value = result.unreadCount
      notices.value = {page: result, busy: false, error: ''}
    } catch (error) {
      if (request !== noticeRequest) return
      notices.value = {...notices.value, busy: false, error: error instanceof Error ? error.message : '无法读取提醒。'}
    } finally {
      if (request === noticeRequest) noticePending = false
    }
  }

  // 已读失败仍允许读取当前详情，收件箱会保留未读状态并提供重试。
  async function openNotice(notice: FeedbackNotice): Promise<void> {
    onOpen(notice.starId, notice.contentId)
    if (notice.readAt || readingNotices.has(notice.id)) return
    const scope = generation
    readingNotices.add(notice.id)
    try {
      await feedbackCall('ReadNotice', {id: notice.id})
      if (scope === generation) await loadNotices()
    } catch (error) {
      if (scope === generation) {
        notices.value = {...notices.value, error: error instanceof Error ? error.message : '标记已读失败。'}
      }
    } finally {
      if (scope === generation) readingNotices.delete(notice.id)
    }
  }

  function forgetMapStar(starId: string): void {
    const {page} = mine.value
    if (!page.list?.some(star => star.starId === starId)) return
    minePending = false
    mine.value = {...mine.value, page: {...page, list: page.list.filter(star => star.starId !== starId)}}
    void loadMine(undefined, true)
  }

  function reset(): void {
    generation++
    mineRequest++
    noticeRequest++
    minePage = noticePage = 1
    minePending = noticePending = false
    mine.value = initialCollection()
    notices.value = initialCollection()
    readingNotices.clear()
    unreadCount.value = 0
  }

  watch(accountSessionKey, reset, {flush: 'sync'})
  onScopeDispose(reset)

  return {mine, notices, unreadCount, loadMine, loadNotices, openNotice, forgetMapStar}
}

function pageRequest(page: number): PageRequest {
  return {page, pageSize: PERSONAL_PAGE_SIZE}
}
