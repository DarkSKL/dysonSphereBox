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
  let scheduled: number | undefined
  let snapshotRegion = ''
  let version = ''
  let needsRefresh = false
  let reconnecting = true
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

  async function refresh(): Promise<void> {
    if (!foreground.value || !size.value.width || !size.value.height || !accountSessionKey.value) return
    if (requestRunning) {
      needsRefresh = true
      return
    }
    requestRunning = true
    const requestedRegion = region.value
    const requestRevision = revision
    const sameRegion = snapshotRegion === requestedRegion
    try {
      const snapshot = await feedbackCall('Map', {
        ...(bounds.value ?? {}),
        ...(sameRegion && version ? {version} : {}),
      })
      if (requestRevision !== revision || !foreground.value) return
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
      error.value = ''
      reconnecting = false
      ready.value = true
    } catch (failure) {
      if (requestRevision !== revision || !foreground.value) return
      error.value = failure instanceof FeedbackError ? failure.message : '星图加载失败，请重试。'
      reconnecting = true
      version = ''
    } finally {
      requestRunning = false
      if (needsRefresh) {
        needsRefresh = false
        refreshNow()
      }
    }
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
    revision++
    window.clearTimeout(scheduled)
  })

  return {camera, size, points, snapshotVersion, arrivals, ready, error, updateCamera, removeStar, refreshNow,
    zoomBy: lens.zoomBy, stopZoom: lens.stopZoom}
}
