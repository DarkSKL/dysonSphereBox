import {onScopeDispose, ref, shallowRef, watch, type Ref} from 'vue'

import {accountSessionKey} from '../../pages/services/account'
import {feedbackCall, FeedbackError} from '../../services/feedback'
import type {StarDetail} from '../../types/feedback'

const DISCUSSION_PAGE_SIZE = 20

// =============== 独立详情与可见性 ===============
// 说明：星图移动不切换阅读目标，重新取数只更新当前目标、页码和会话。
export function useFeedbackDetail(foreground: Ref<boolean>, onUnavailable: (starId: string) => void, onSeen: () => void) {
  const target = ref('')
  const highlightedEntry = ref('')
  const detail = shallowRef<StarDetail | null>(null)
  const page = ref(1)
  const busy = ref(false)
  const error = ref('')
  const unavailable = ref(false)
  let selection = 0
  let inFlight: Promise<void> | undefined
  let lastSeenVersion = ''

  function close(): void {
    selection++
    target.value = ''
    highlightedEntry.value = ''
    detail.value = null
    page.value = 1
    busy.value = false
    error.value = ''
    unavailable.value = false
    inFlight = undefined
    lastSeenVersion = ''
  }

  function open(starId: string, contentId = ''): void {
    close()
    target.value = starId
    highlightedEntry.value = contentId
    void refresh()
  }

  // 仅推进到当前页实际收到的已公开玩家留言，审核游标使用精确的十进制字符串。
  async function markSeen(snapshot: StarDetail, scope: number): Promise<void> {
    if (!foreground.value || !snapshot.owned || snapshot.withdrawn) return
    const entries = snapshot.replies.list ?? []
    const newest = entries.reduce((latest, entry) => {
      const version = entry.reviewVersion
      if (entry.kind !== 'comment' || entry.reviewState !== 'approved' || !/^\d+$/.test(version)) return latest
      return version.length > latest.length || (version.length === latest.length && version > latest) ? version : latest
    }, '')
    if (!newest || newest.length < lastSeenVersion.length ||
      (newest.length === lastSeenVersion.length && newest <= lastSeenVersion)) return
    try {
      await feedbackCall('Seen', {starId: snapshot.starId, reviewVersion: newest})
      if (scope === selection) {
        if (newest.length > lastSeenVersion.length || (newest.length === lastSeenVersion.length && newest > lastSeenVersion)) {
          lastSeenVersion = newest
          onSeen()
        }
      }
    } catch {
      // 游标失败不影响阅读，下次详情同步继续尝试。
    }
  }

  function clearUnavailable(starId: string): void {
    if (target.value !== starId) return
    selection++
    inFlight = undefined
    busy.value = false
    detail.value = null
    unavailable.value = true
    error.value = '这颗星已不可用。'
    onUnavailable(starId)
  }

  function refresh(quiet = false, force = false): Promise<void> {
    if (!target.value || (unavailable.value && quiet)) return Promise.resolve()
    if (inFlight && !force) return inFlight
    const scope = ++selection
    const starId = target.value
    busy.value = !quiet
    const request = feedbackCall('Detail', {starId, page: page.value, pageSize: DISCUSSION_PAGE_SIZE})
      .then((snapshot) => {
        if (scope !== selection) return
        if (snapshot.withdrawn) {
          clearUnavailable(starId)
          return
        }
        detail.value = snapshot
        unavailable.value = false
        error.value = ''
        void markSeen(snapshot, scope)
      })
      .catch((failure: unknown) => {
        if (scope !== selection) return
        if (failure instanceof FeedbackError && failure.reason === 'unavailable') clearUnavailable(starId)
        else error.value = failure instanceof Error ? failure.message : '详情加载失败。'
      })
      .finally(() => {
        if (scope !== selection) return
        busy.value = false
        inFlight = undefined
      })
    inFlight = request
    return request
  }

  function changePage(next: number): void {
    page.value = next
    void refresh(false, true)
  }

  watch(accountSessionKey, close, {flush: 'sync'})
  onScopeDispose(close)

  return {target, detail, highlightedEntry, busy, error, unavailable, refresh, open, close, changePage, clearUnavailable}
}

