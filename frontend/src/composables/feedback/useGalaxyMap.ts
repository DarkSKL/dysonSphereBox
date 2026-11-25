import {computed, onScopeDispose, ref, shallowRef, watch, type Ref} from 'vue'

import {accountSessionKey} from '../../pages/services/account'
import {feedbackCall, FeedbackError} from '../../services/feedback'
import type {Bounds, StarPoint} from '../../types/feedback'
import {HOME_CAMERA, queryBounds, queryRegionKey, type Camera, type ViewSize} from './galaxy-camera'
import {useGalaxyZoom} from './useGalaxyZoom'

// =============== 条件快照 ===============
// 说明：视区变化合并后再查询，前台定时续查。只接受当前视区与会话的结果。
export function useGalaxyMap(foreground: Ref<boolean>) {
  const camera = shallowRef<Camera>(HOME_CAMERA)
  const lens = useGalaxyZoom(camera)
  const size = shallowRef<ViewSize>({width: 0, height: 0})
  const points = shallowRef<StarPoint[]>([])
  const snapshotVersion = ref('')
  const arrivals = shallowRef<ReadonlySet<string>>(new Set())
  const ready = ref(false)
  const error = ref('')
  // 请求在途标志：在 Map 请求在途或手动重试尚未结算（含连续切视区补查衔接间隙）期间持续为真。
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
  let requestRunning = false
  let retryCount = 0
  let inFlight: Promise<boolean> | null = null
  let scheduled: number | undefined
  let version = ''
  let needsRefresh = false
  let reconnecting = true
  let disposed = false
  let viewChangedAt = -Infinity

  function schedule(delay: number): void {
    window.clearTimeout(scheduled)
    if (foreground.value) scheduled = window.setTimeout(() => void refresh(), delay)
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

  // 手动重试：立即发请求，返回的 Promise 与 loading 状态持续到最新视区的有效请求真正结算（成功或失败）。
  // 使用：重试按钮用它驱动转圈，指示与有效请求同生命周期，不依赖任何固定延时。
  // 边界：若点击时已有旧请求在途，先等其结算；若在途期间视区连续切换多次导致前序请求作废，
  // 持续跟进对最新视区的补查且不丢弃最后一次查询；切后台、换账号或卸载时立即退出避免悬停。
  async function retry(): Promise<void> {
    const scope = accountSessionKey.value
    const active = () => !disposed && foreground.value && !!size.value.width && !!size.value.height &&
      !!scope && accountSessionKey.value === scope
    if (!active()) return
    retryCount++
    loading.value = true
    try {
      window.clearTimeout(scheduled)
      scheduled = undefined
      if (inFlight) {
        const settled = await inFlight
        if (!active()) return
        if (settled && !error.value && snapshotRegion.value === region.value) return
      }
      while (active()) {
        if (inFlight) {
          const settled = await inFlight
          if (settled || !active()) return
          continue
        }
        window.clearTimeout(scheduled)
        scheduled = undefined
        needsRefresh = false
        const settled = await refresh()
        if (settled) return
      }
    } finally {
      retryCount--
      loading.value = requestRunning || retryCount > 0
    }
  }

  // 返回 true 表示本次请求正常结算（成功或失败）；false 表示未发出或因 revision 变化被废弃。
  async function refresh(): Promise<boolean> {
    if (!foreground.value || !size.value.width || !size.value.height || !accountSessionKey.value) return false
    if (requestRunning) {
      needsRefresh = true
      return false
    }
    requestRunning = true
    loading.value = true
    const requestedBounds = bounds.value
    const requestedRegion = queryRegionKey(requestedBounds)
    const requestRevision = revision
    const sameRegion = snapshotRegion.value === requestedRegion
    let settled = false
    const run = (async () => {
      try {
        const snapshot = await feedbackCall('Map', {
          ...(requestedBounds ?? {}),
          ...(sameRegion && version ? {version} : {}),
        })
        if (requestRevision !== revision || !foreground.value) return false
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
        error.value = ''
        reconnecting = false
        ready.value = true
        settled = true
        return true
      } catch (failure) {
        if (requestRevision !== revision || !foreground.value) return false
        error.value = failure instanceof FeedbackError ? failure.message : '星图加载失败，请重试。'
        reconnecting = true
        version = ''
        settled = true
        return true
      } finally {
        requestRunning = false
        inFlight = null
        loading.value = retryCount > 0
        if (needsRefresh) {
          needsRefresh = false
          if (retryCount === 0) refreshNow()
        }
      }
    })()
    inFlight = run
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
    if (visible) refreshNow()
    else { window.clearTimeout(scheduled); lens.stopZoom() }
  }, {flush: 'sync'})
  watch(accountSessionKey, () => {
    revision++
    snapshotRegion.value = ''
    snapshotBounds.value = null
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
    refreshNow()
  }, {flush: 'sync'})
  onScopeDispose(() => {
    disposed = true
    revision++
    window.clearTimeout(scheduled)
  })

  return {camera, size, points, snapshotVersion, arrivals, ready, error, loading, snapshotRegion, viewRegion: region, snapshotBounds, updateCamera, removeStar, refreshNow, retry,
    zoomBy: lens.zoomBy, stopZoom: lens.stopZoom}
}
