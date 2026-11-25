import {computed, onScopeDispose, ref, shallowRef, watch, type Ref} from 'vue'

import {accountSessionKey} from '../../pages/services/account'
import {feedbackCall, FeedbackError} from '../../services/feedback'
import type {Bounds, StarPoint} from '../../types/feedback'
import {HOME_CAMERA, queryBounds, queryRegionKey, type Camera, type ViewSize} from './galaxy-camera'
import {nextSnapshotOrder} from './galaxy-status'
import {useGalaxyZoom} from './useGalaxyZoom'

interface InFlightRun {
  epoch: number
  scope: string
  promise: Promise<boolean>
}

interface ActiveRetry {
  epoch: number
  scope: string
  promise: Promise<void>
  abort: () => void
}

// =============== 条件快照 ===============
// 说明：视区变化合并后再查询，前台定时续查。只接受当前视区与会话的结果。
export function useGalaxyMap(foreground: Ref<boolean>) {
  const camera = shallowRef<Camera>(HOME_CAMERA)
  const lens = useGalaxyZoom(camera)
  const size = shallowRef<ViewSize>({width: 0, height: 0})
  const points = shallowRef<StarPoint[]>([])
  const snapshotVersion = ref('')
  // 最近一次成功结算的 Map 请求发起时的因果序号（unchanged: true/false 均更新），用于核验快照是否晚于详情过审观测。
  const snapshotSyncedOrder = ref(0)
  const arrivals = shallowRef<ReadonlySet<string>>(new Set())
  const ready = ref(false)
  const error = ref('')
  // 请求在途标志：在当前会话的 Map 请求在途或手动重试尚未结算（含连续切视区补查衔接间隙）期间持续为真。
  const loading = ref(false)
  // 最近一次成功快照对应的实际查询范围键（'all' 或归一化四元边界）与对应边界对象。
  const snapshotRegion = ref('')
  const snapshotBounds = shallowRef<Bounds | null>(null)
  // 未聚合的全景已包含全部可读记录，旋转或变焦只需本地裁切；全景的条件轮询仍可发现变化。
  const completeCatalog = ref(false)
  const bounds = computed(() => {
    if (completeCatalog.value) return null
    const outward = lens.outwardTarget.value
    return queryBounds(outward === null ? camera.value : {...camera.value, zoom: outward}, size.value)
  })
  const region = computed(() => queryRegionKey(bounds.value))
  let revision = 0
  // 会话代次：每次 accountSessionKey 变更递增，按会话代次隔离在途请求与活跃重试，避免旧会话收尾吞掉新会话首次查询。
  let sessionEpoch = 0
  let currentRun: InFlightRun | null = null
  let currentRetry: ActiveRetry | null = null
  let scheduled: number | undefined
  let version = ''
  let needsRefresh = false
  let reconnecting = true
  let disposed = false
  let viewChangedAt = -Infinity

  function syncLoading(): void {
    const scope = accountSessionKey.value
    const runningForSession = !!scope && currentRun?.epoch === sessionEpoch && currentRun.scope === scope
    const retryingForSession = !!scope && currentRetry?.epoch === sessionEpoch && currentRetry.scope === scope
    loading.value = runningForSession || retryingForSession
  }

  function clearScheduled(): void {
    window.clearTimeout(scheduled)
    scheduled = undefined
  }

  function detachInFlightAndRetry(): void {
    currentRun = null
    const staleRetry = currentRetry
    currentRetry = null
    staleRetry?.abort()
    syncLoading()
  }

  function schedule(delay: number): void {
    clearScheduled()
    if (foreground.value && accountSessionKey.value) {
      scheduled = window.setTimeout(() => {
        scheduled = undefined
        void refresh()
      }, delay)
    }
  }

  function updateCamera(next: Camera, dimensions: ViewSize): void {
    size.value = dimensions
    camera.value = next
  }

  function removeStar(starId: string): void {
    points.value = points.value.filter((point) => point.starId !== starId)
    // 正在返回的旧快照可能包含已撤回的星，立即废弃该结果。
    revision++
    version = ''
    schedule(0)
  }

  function refreshNow(): void {
    schedule(Math.max(0, 160 - (performance.now() - viewChangedAt)))
  }

  // 手动重试：立即发请求，返回的 Promise 与 loading 状态持续到当前会话最新视区的有效请求真正结算（成功或失败）。
  // 使用：重试按钮用它驱动转圈，指示与有效请求同生命周期，不依赖任何固定延时。
  // 边界：同会话重复/并发调用复用活跃重试 Promise；在途期间视区连续切换多次时持续跟进补查；
  // 切后台、换账号或卸载时立即解除等待，绝不阻塞或吞掉新会话的首次地图查询。
  function retry(): Promise<void> {
    const scope = accountSessionKey.value
    const epoch = sessionEpoch
    const active = () => !disposed && foreground.value && !!size.value.width && !!size.value.height &&
      !!scope && epoch === sessionEpoch && accountSessionKey.value === scope
    if (!active()) return Promise.resolve()
    if (currentRetry && currentRetry.epoch === epoch && currentRetry.scope === scope) {
      return currentRetry.promise
    }

    let aborted = false
    let notifyAbort: (() => void) | undefined
    const abortSignal = new Promise<'aborted'>((resolve) => {
      notifyAbort = () => {
        aborted = true
        resolve('aborted')
      }
    })
    const stillActive = () => !aborted && active()

    const retryEntry: ActiveRetry = {
      epoch,
      scope,
      promise: Promise.resolve(),
      abort: () => notifyAbort?.(),
    }
    currentRetry = retryEntry
    syncLoading()

    const runRetry = (async () => {
      try {
        clearScheduled()
        if (currentRun && currentRun.epoch === epoch && currentRun.scope === scope) {
          const outcome = await Promise.race([currentRun.promise, abortSignal])
          if (outcome === 'aborted' || !stillActive()) return
          if (outcome && !error.value && snapshotRegion.value === region.value) return
        }
        while (stillActive()) {
          if (currentRun && currentRun.epoch === epoch && currentRun.scope === scope) {
            const outcome = await Promise.race([currentRun.promise, abortSignal])
            if (outcome === 'aborted' || outcome || !stillActive()) return
            continue
          }
          clearScheduled()
          needsRefresh = false
          const outcome = await Promise.race([refresh(), abortSignal])
          if (outcome === 'aborted' || outcome) return
        }
      } finally {
        if (currentRetry === retryEntry) currentRetry = null
        syncLoading()
        // 防御性交接：若退出时仍有当前活跃会话的待补查请求且无在途请求/重试，立即补发，绝不遗漏。
        const currentScope = accountSessionKey.value
        if (needsRefresh && currentScope && foreground.value &&
          currentRun?.epoch !== sessionEpoch && currentRetry?.epoch !== sessionEpoch) {
          needsRefresh = false
          refreshNow()
        }
      }
    })()

    retryEntry.promise = runRetry
    return runRetry
  }

  // 返回 true 表示本次请求正常结算（成功或失败）；false 表示未发出或因 revision / 会话变化被废弃。
  async function refresh(): Promise<boolean> {
    const scope = accountSessionKey.value
    const epoch = sessionEpoch
    if (!foreground.value || !size.value.width || !size.value.height || !scope) return false
    if (currentRun && currentRun.epoch === epoch && currentRun.scope === scope) {
      needsRefresh = true
      return false
    }
    const requestedBounds = bounds.value
    const requestedRegion = queryRegionKey(requestedBounds)
    const requestRevision = revision
    const requestOrder = nextSnapshotOrder()
    const sameRegion = snapshotRegion.value === requestedRegion
    let settled = false
    const entry: InFlightRun = {epoch, scope, promise: Promise.resolve(false)}
    currentRun = entry
    syncLoading()
    const run = (async () => {
      try {
        const snapshot = await feedbackCall('Map', {
          ...(requestedBounds ?? {}),
          ...(sameRegion && version ? {version} : {}),
        })
        if (requestRevision !== revision || !foreground.value || epoch !== sessionEpoch || accountSessionKey.value !== scope) {
          return false
        }
        if (!snapshot.unchanged) {
          const next = snapshot.points ?? []
          const known = new Set(points.value.map((point) => point.starId))
          if (sameRegion && !reconnecting && !error.value) {
            arrivals.value = new Set(next.filter((point) => !known.has(point.starId)).map((point) => point.starId))
          }
          points.value = next
          if (requestedRegion === 'all') completeCatalog.value = next.every(point => (point.count ?? 1) <= 1)
        }
        version = snapshot.version
        snapshotVersion.value = snapshot.version
        snapshotBounds.value = requestedBounds
        snapshotRegion.value = requestedRegion
        snapshotSyncedOrder.value = requestOrder
        error.value = ''
        reconnecting = false
        ready.value = true
        settled = true
        return true
      } catch (failure) {
        if (requestRevision !== revision || !foreground.value || epoch !== sessionEpoch || accountSessionKey.value !== scope) {
          return false
        }
        error.value = failure instanceof FeedbackError ? failure.message : '星图加载失败，请重试。'
        reconnecting = true
        version = ''
        settled = true
        return true
      } finally {
        if (currentRun === entry) {
          currentRun = null
          syncLoading()
          if (needsRefresh) {
            needsRefresh = false
            const currentScope = accountSessionKey.value
            const retryingCurrent = !!currentScope && currentRetry?.epoch === sessionEpoch && currentRetry.scope === currentScope
            if (!retryingCurrent && currentScope) refreshNow()
          }
        }
      }
    })()
    entry.promise = run
    return (await run) && settled
  }

  watch(region, () => {
    revision++
    viewChangedAt = performance.now()
    schedule(160)
  }, {flush: 'sync'})
  watch(() => size.value.width > 0 && size.value.height > 0, refreshNow)
  watch(foreground, (visible) => {
    revision++
    reconnecting = true
    if (visible) {
      refreshNow()
    } else {
      clearScheduled()
      needsRefresh = false
      detachInFlightAndRetry()
      lens.stopZoom()
    }
  }, {flush: 'sync'})
  watch(accountSessionKey, (nextScope) => {
    sessionEpoch++
    revision++
    clearScheduled()
    needsRefresh = false
    detachInFlightAndRetry()
    snapshotRegion.value = ''
    snapshotBounds.value = null
    snapshotSyncedOrder.value = 0
    version = ''
    snapshotVersion.value = ''
    points.value = []
    completeCatalog.value = false
    arrivals.value = new Set()
    ready.value = false
    error.value = ''
    reconnecting = true
    camera.value = HOME_CAMERA
    lens.stopZoom()
    // 清除 camera 重置连带触发的视区防抖，使新会话首次查询立即可发、空会话绝不发请求。
    viewChangedAt = -Infinity
    clearScheduled()
    syncLoading()
    if (nextScope) schedule(0)
  }, {flush: 'sync'})
  onScopeDispose(() => {
    disposed = true
    revision++
    clearScheduled()
    needsRefresh = false
    detachInFlightAndRetry()
  })

  return {camera, size, points, snapshotVersion, snapshotSyncedOrder, arrivals, ready, error, loading, snapshotRegion, viewRegion: region, snapshotBounds, updateCamera, removeStar, refreshNow, retry,
    zoomBy: lens.zoomBy, stopZoom: lens.stopZoom}
}
