import {computed, onScopeDispose, ref, shallowRef, watch, type Ref} from 'vue'

import {accountSessionKey} from '../../pages/services/account'
import {feedbackCall, FeedbackError} from '../../services/feedback'
import type {StarPoint} from '../../types/feedback'
import {HOME_CAMERA, queryBounds, type Camera, type ViewSize} from './galaxy-camera'
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
  // 请求在途标志：只在 Map 请求真正发出到结算（成功、失败、被废弃）之间为真，供重试指示与「等待全景回执」判定使用。
  const loading = ref(false)
  // 最近一次成功快照的查询范围是否为全景（无边界）。空结果只有在全景范围下才表示整片星图没有反馈。
  const snapshotIsPanorama = ref(false)
  // 当前镜头要查询的范围是否为全景。与 snapshotIsPanorama 不一致，说明画面正在等新范围的回执。
  const viewIsPanorama = computed(() => region.value === 'all')
  // 未聚合的全景已包含全部可读记录，旋转或变焦只需本地裁切；全景的条件轮询仍可发现变化。
  const completeCatalog = ref(false)
  const bounds = computed(() => {
    if (completeCatalog.value) return null
    const outward = lens.outwardTarget.value
    return queryBounds(outward === null ? camera.value : {...camera.value, zoom: outward}, size.value)
  })
  const region = computed(() => bounds.value
    ? [bounds.value.minX, bounds.value.minY, bounds.value.maxX, bounds.value.maxY].join(',')
    : 'all')
  let revision = 0
  let requestRunning = false
  let inFlight: Promise<boolean> | null = null
  let scheduled: number | undefined
  let snapshotRegion = ''
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

  // 手动重试：立即发请求，返回的 Promise 在本次请求结算（成功或失败）时才完成。
  // 使用：重试按钮用它驱动转圈，指示与请求同生命周期，不依赖任何固定延时。
  // 边界：若点击时已有旧请求在途，先等其结算；若旧请求未清除错误，再发起一次新的重试并等待其结算。
  // 若本次重试因视区变化被废弃，最多跟进一次补发；切后台、换账号或卸载时立即退出避免悬停。
  async function retry(): Promise<void> {
    const scope = accountSessionKey.value
    const active = () => !disposed && foreground.value && !!size.value.width && !!size.value.height && accountSessionKey.value === scope
    if (!active()) return
    window.clearTimeout(scheduled)
    if (inFlight) {
      await inFlight
      if (!active()) return
      if (!error.value && snapshotRegion === region.value) return
      window.clearTimeout(scheduled)
    }
    for (let attempt = 0; attempt < 2 && active(); attempt++) {
      const settled = await refresh()
      if (settled || !active()) return
      window.clearTimeout(scheduled)
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
    const requestedRegion = region.value
    const requestRevision = revision
    const sameRegion = snapshotRegion === requestedRegion
    let settled = false
    const run = (async () => {
      try {
        const snapshot = await feedbackCall('Map', {
          ...(bounds.value ?? {}),
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
        snapshotRegion = requestedRegion
        snapshotIsPanorama.value = requestedRegion === 'all'
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
        loading.value = false
        if (needsRefresh) {
          needsRefresh = false
          refreshNow()
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
    snapshotRegion = ''
    snapshotIsPanorama.value = false
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

  return {camera, size, points, snapshotVersion, arrivals, ready, error, loading, snapshotIsPanorama, viewIsPanorama, updateCamera, removeStar, refreshNow, retry,
    zoomBy: lens.zoomBy, stopZoom: lens.stopZoom}
}
