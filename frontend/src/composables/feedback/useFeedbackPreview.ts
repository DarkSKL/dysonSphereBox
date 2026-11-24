import {computed, onScopeDispose, shallowRef, watch, type Ref} from 'vue'

import {accountSessionKey} from '../../pages/services/account'
import {feedbackCall, FeedbackError} from '../../services/feedback'
import type {Coordinates, DisplayStar, MyStar, StarDetail, StarPreview} from '../../types/feedback'

interface Hover {id: string; anchor?: Coordinates}
interface Slot {
  id: string
  revision: string
  personal: boolean
  reviewVersion?: string
  quote?: StarPreview
  expires: number
  checked: number
  retryRead: number
  retryName: number
}

const BATCH_SIZE = 128
const FRESH_MS = 60_000
const RETRY_MS = 5_000

// =============== 会话内的引文缓存 ===============
// 每个槽只属于一个已授权的星和版本。移开指针保留引文，移出快照、切号、离页和断线均废弃相应回执。
export function useFeedbackPreview(active: Ref<boolean>, onUnavailable: (starId: string) => void) {
  const hover = shallowRef<Hover | null>(null)
  const quote = shallowRef<StarPreview | null>(null)
  const hovered = computed(() => hover.value?.id ?? '')
  let slots = new Map<string, Slot>()
  let warm: Slot[] = []
  let previousWarm: Slot[] = []
  let nearby = '', dismissed = ''
  let epoch = 0
  let batchSupported = true, batchBusy = false, batchRetry = 0
  let singleReads = 0, nameBusy = false, legacyNames = false, queued = false
  let hideTimer: number | undefined
  const pending = new Set<Slot>()

  function fresh(slot: Slot): boolean {
    return !!slot.quote && performance.now() < slot.expires
  }

  function paint(): void {
    const slot = slots.get(hovered.value)
    quote.value = slot && fresh(slot) ? slot.quote ?? null : null
  }

  function remember(slot: Slot, value: StarPreview, checked: boolean): void {
    const name = value.displayName?.trim() || (fresh(slot) ? slot.quote?.displayName : undefined)
    slot.quote = {starId: slot.id, preview: value.preview, displayName: name}
    slot.expires = performance.now() + FRESH_MS
    if (checked) slot.checked = slot.expires
    if (hovered.value === slot.id) paint()
  }

  function hold(): void {
    window.clearTimeout(hideTimer)
    hideTimer = undefined
  }

  function clear(): void {
    hold()
    hover.value = null
    quote.value = null
    nearby = dismissed = ''
  }

  function leave(): void {
    if (!hover.value?.anchor || !quote.value?.preview) {
      hover.value = null
      quote.value = null
    } else if (hideTimer === undefined) {
      // 仅给移向引文的指针留出跨越间隙的时间，进入另一颗星会直接切换。
      hideTimer = window.setTimeout(() => {
        hideTimer = undefined
        hover.value = null
        quote.value = null
        wake()
      }, 100)
    }
  }

  function dismiss(): void {
    const id = hovered.value
    clear()
    dismissed = id
  }

  function valid(slot: Slot, scope: number): boolean {
    return scope === epoch && active.value && slots.get(slot.id) === slot
  }

  function forget(slot: Slot): void {
    slots.delete(slot.id)
    slot.quote = undefined
    slot.expires = 0
    slot.retryRead = performance.now() + RETRY_MS
    if (hovered.value === slot.id) clear()
    onUnavailable(slot.id)
  }

  function needsRead(slot: Slot): boolean {
    const now = performance.now()
    return !pending.has(slot) && now >= slot.retryRead &&
      (!fresh(slot) || (!slot.quote?.displayName && slot.checked <= now))
  }

  // =============== 有界预取与按需回退 ===============
  // 首批覆盖可见代表星。鼠标命中优先于邻近意图；同一槽共享在途请求。旧服务端只按意图读取单条。
  function wake(): void {
    if (queued || !active.value) return
    queued = true
    queueMicrotask(() => { queued = false; if (active.value) pump() })
  }

  async function readBatch(group: Slot[]): Promise<void> {
    const scope = epoch
    batchBusy = true
    group.forEach(slot => pending.add(slot))
    try {
      const result = await feedbackCall('Previews', {starIds: group.map(slot => slot.id)})
      if (scope !== epoch) return
      if (!Array.isArray(result.items)) { batchRetry = performance.now() + RETRY_MS; return }
      const byId = new Map(result.items.map(item => [item.starId, item]))
      for (const slot of group) {
        if (!valid(slot, scope)) continue
        const item = byId.get(slot.id)
        if (!item) forget(slot)
        else {
          if (!item.displayName?.trim()) legacyNames = true
          remember(slot, item, true)
        }
      }
    } catch (failure) {
      if (scope !== epoch) return
      if (failure instanceof FeedbackError && failure.reason === 'unsupported') batchSupported = false
      else batchRetry = performance.now() + RETRY_MS
    } finally {
      if (scope === epoch) {
        group.forEach(slot => pending.delete(slot))
        batchBusy = false
        wake()
      }
    }
  }

  async function readOne(slot: Slot): Promise<void> {
    const scope = epoch
    pending.add(slot)
    singleReads++
    try {
      const item = await feedbackCall('Preview', {starId: slot.id})
      if (!valid(slot, scope)) return
      if (item.starId !== slot.id) { slot.retryRead = performance.now() + RETRY_MS; return }
      if (!item.displayName?.trim()) legacyNames = true
      remember(slot, item, true)
    } catch (failure) {
      if (!valid(slot, scope)) return
      if (failure instanceof FeedbackError && failure.reason === 'unavailable') forget(slot)
      else slot.retryRead = performance.now() + RETRY_MS
    } finally {
      if (scope === epoch) { pending.delete(slot); singleReads--; wake() }
    }
  }

  function acceptDetail(slot: Slot, detail: StarDetail): void {
    if (detail.starId !== slot.id) return
    if (detail.withdrawn || (slot.personal ? !detail.owned : detail.original.reviewState !== 'approved')) {
      forget(slot)
      return
    }
    if (slot.reviewVersion && slot.reviewVersion !== detail.original.reviewVersion) return
    if (slot.personal && slot.revision !== 'mine/' + detail.original.bodyVersion + '/' + detail.original.reviewState) return
    const line = [...detail.original.body.split('\n', 1)[0]].slice(0, 120).join('')
    remember(slot, {starId: slot.id, preview: line, displayName: detail.original.displayName}, true)
  }

  async function readLegacyName(slot: Slot): Promise<void> {
    const scope = epoch
    nameBusy = true
    slot.retryName = performance.now() + RETRY_MS
    try {
      // 老回执缺少署名时只为当前命中或邻近目标补查。只保留原文首行和服务端可展示署名。
      const detail = await feedbackCall('Detail', {starId: slot.id, page: 1, pageSize: 1})
      if (valid(slot, scope)) acceptDetail(slot, detail)
    } catch (failure) {
      if (valid(slot, scope) && failure instanceof FeedbackError && failure.reason === 'unavailable') forget(slot)
    } finally {
      if (scope === epoch) { nameBusy = false; wake() }
    }
  }

  function pump(): void {
    const focus = slots.get(hovered.value), hint = slots.get(nearby)
    const intended = focus ?? hint
    const canBatch = batchSupported && performance.now() >= batchRetry
    if (canBatch && !batchBusy) {
      const candidates = new Set([focus, hint, ...warm])
      warm = []
      const group: Slot[] = []
      for (const slot of candidates) {
        if (slot && slots.get(slot.id) === slot && !slot.personal && needsRead(slot)) group.push(slot)
        if (group.length === BATCH_SIZE) break
      }
      if (group.length) void readBatch(group)
    }
    if (!intended) return
    if (needsRead(intended) && singleReads < (focus ? 2 : 1)) void readOne(intended)
    if (!nameBusy && legacyNames && !intended.quote?.displayName && intended.retryName <= performance.now() &&
      intended.retryRead <= performance.now() && (fresh(intended) || !!focus)) void readLegacyName(intended)
  }

  // =============== 视区、指针与键盘 ===============
  function reconcile(points: readonly DisplayStar[], privateStars: ReadonlyMap<string, MyStar>, version: string): void {
    if (!active.value) return
    const next = new Map<string, Slot>()
    for (const point of points) {
      const id = point.starId, own = privateStars.get(id)
      const revision = own ? 'mine/' + own.bodyVersion + '/' + own.reviewState : 'public/' + (point.previewVersion || version)
      const old = slots.get(id)
      const slot: Slot = old?.revision === revision ? old : {
        id, revision, personal: !!own, reviewVersion: own ? undefined : point.previewVersion,
        expires: 0, checked: 0, retryRead: 0, retryName: 0,
      }
      next.set(id, slot)
      if (own && (!fresh(slot) || slot.quote?.preview !== own.preview || (own.displayName && own.displayName !== slot.quote?.displayName))) {
        remember(slot, {starId: id, preview: own.preview, displayName: own.displayName}, false)
      }
    }
    slots = next
    if (hover.value && !slots.has(hover.value.id)) clear()
    paint()
    wake()
  }

  function prewarm(visible: readonly {point: DisplayStar}[]): void {
    if (!active.value || !batchSupported) return
    const candidates: Slot[] = []
    for (const {point} of visible) {
      const slot = slots.get(point.starId)
      if (slot && !slot.personal) candidates.push(slot)
      if (candidates.length === BATCH_SIZE) break
    }
    if (candidates.length !== previousWarm.length || candidates.some((slot, i) => slot !== previousWarm[i])) {
      previousWarm = candidates
      warm = candidates
    }
    wake()
  }

  function select(point: DisplayStar | null, anchor?: Coordinates): void {
    if (!point || !active.value) {
      dismissed = ''
      leave()
      return
    }
    hold()
    if (!slots.has(point.starId)) { clear(); return }
    if (dismissed === point.starId) return
    dismissed = ''
    if (hovered.value !== point.starId || !!anchor !== !!hover.value?.anchor) hover.value = {id: point.starId, anchor}
    paint()
    wake()
  }

  function approach(point: DisplayStar | null): void {
    const id = point?.starId ?? ''
    if (!active.value || nearby === id) return
    nearby = id
    wake()
  }

  function primeDetail(detail: StarDetail): void {
    const slot = slots.get(detail.starId)
    if (slot && active.value && (slot.personal || slot.reviewVersion)) acceptDetail(slot, detail)
  }

  function reset(): void {
    epoch++
    slots.clear()
    pending.clear()
    warm = previousWarm = []
    batchSupported = true
    batchBusy = nameBusy = legacyNames = false
    batchRetry = singleReads = 0
    clear()
  }
  watch(accountSessionKey, reset, {flush: 'sync'})
  watch(active, enabled => { if (!enabled) reset() }, {flush: 'sync'})
  const onKey = (event: KeyboardEvent) => {
    if (event.key === 'Escape' && hover.value?.anchor) dismiss()
  }
  window.addEventListener('keydown', onKey)
  onScopeDispose(() => {
    window.removeEventListener('keydown', onKey)
    reset()
  })
  return {hover, hovered, quote, select, approach, clear, hold, leave, reconcile, prewarm, primeDetail}
}
