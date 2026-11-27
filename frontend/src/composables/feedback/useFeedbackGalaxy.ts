import {computed, nextTick, onActivated, onDeactivated, onMounted, onScopeDispose, ref, shallowRef, watch} from 'vue'
import {useRoute} from 'vue-router'

import {accountSessionKey} from '../../pages/services/account'
import {feedbackCall, FeedbackError, feedbackDeepLinkRevision} from '../../services/feedback'
import {reviewLabels, type Coordinates, type DisplayStar, type GalaxyPosition, type MyStar} from '../../types/feedback'
import {HOME_CAMERA, includeStar, projectVisibleStars, usefulZoomLimit} from './galaxy-camera'
import {useFeedbackDetail} from './useFeedbackDetail'
import {useFeedbackEditor} from './useFeedbackEditor'
import {useFeedbackPersonal} from './useFeedbackPersonal'
import {useFeedbackPreview} from './useFeedbackPreview'
import {useGalaxyMap} from './useGalaxyMap'

type Panel = 'detail' | 'publish' | 'mine' | 'notices' | ''

// =============== 页面编排 ===============
// 说明：统一处理前台生命周期、选择和写入后校准。地图和详情分别维护相机与阅读状态。
export function useFeedbackGalaxy(restoreFocus?: () => void) {
  const route = useRoute()
  const active = ref(false)
  const visible = ref(!document.hidden)
  const foreground = computed(() => active.value && visible.value && !!accountSessionKey.value)
  const map = useGalaxyMap(foreground)
  const panel = ref<Panel>('')
  const panelHeading = ref<HTMLElement>()
  const browse = ref(false)
  const actionMenu = shallowRef<Coordinates | null>(null)
  const confirmWithdraw = ref(false)
  const writing = ref(false)
  const actionError = ref('')
  const status = ref('')
  const recentlySaved = shallowRef<MyStar | null>(null)
  const previewActive = computed(() => foreground.value && !map.error.value)
  const preview = useFeedbackPreview(previewActive, id => {
    if (recentlySaved.value?.starId === id) recentlySaved.value = null
    if (privateStars.value.has(id)) personal.forgetMapStar(id)
    map.removeStar(id)
  })
  const detail = useFeedbackDetail(foreground, onUnavailable, () => { void personal.loadMine(undefined, true) })
  const personal = useFeedbackPersonal(openDetail)
  const editor = useFeedbackEditor((mode, result, body) => {
    status.value = mode === 'publish' ? '已提交审核，可在「我的星星」查看进展。'
      : mode === 'edit' ? '改稿已提交审核。' : mode === 'supplement' ? '补充已提交审核。' : '留言已提交审核。'
    if (result) {
      recentlySaved.value = {
        ...result, preview: [...body.split('\n', 1)[0]].slice(0, 120).join(''), progress: detail.detail.value?.progress ?? 'feedback',
        newComment: false, createdAt: new Date().toISOString(),
      }
      if (result.reviewState !== 'approved') map.removeStar(result.starId)
    } else map.refreshNow()
    if (mode === 'publish' && result) {
      locate(result)
      showPanel('mine')
      void personal.loadMine(1)
    } else {
      void detail.refresh(false, true)
      void personal.loadMine(undefined, true)
    }
  }, (reason, starId) => {
    if (reason === 'unavailable') detail.clearUnavailable(starId)
    if (reason === 'editLocked' || reason === 'versionChanged') void detail.refresh(false, true)
  })

  const privateStars = computed(() => {
    const owned = new Map((personal.mine.value.page.list ?? []).map((star) => [star.starId, star]))
    const saved = recentlySaved.value
    if (saved && !owned.has(saved.starId)) owned.set(saved.starId, saved)
    const publicIDs = new Set(map.points.value.map((star) => star.starId))
    return new Map([...owned].filter(([id, star]) => !publicIDs.has(id) && !star.withdrawn &&
      (star.reviewState === 'pending' || star.reviewState === 'manual')))
  })
  const points = computed<DisplayStar[]>(() => [
    ...map.points.value,
    ...[...privateStars.value.values()].map((star) => ({
      starId: star.starId, x: star.x, y: star.y, z: star.z ?? 0, progress: star.progress, brightness: 0,
      privateLabel: reviewLabels[star.reviewState],
    })),
  ])
  const visibleStars = computed(() => projectVisibleStars(points.value, map.camera.value, map.size.value))
  const zoomLimit = computed(() => usefulZoomLimit(points.value, map.size.value))
  const currentCollection = computed(() => panel.value === 'mine' ? personal.mine.value : personal.notices.value)
  let previousFocus: HTMLElement | null = null
  let deepLink = ''
  let ticks = 0
  let timer: number | undefined

  function showPanel(mode: Panel): void {
    dismissActions()
    if (!panel.value && mode) previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
    if (mode !== 'detail') detail.close()
    if (mode !== 'detail' && mode !== 'publish') editor.close()
    panel.value = mode
    actionError.value = ''
    confirmWithdraw.value = false
    if (mode) {
      // 发布表单在挂载时聚焦输入框，其余面板聚焦标题。
      if (mode !== 'publish') void nextTick(() => panelHeading.value?.focus({preventScroll: true}))
    } else {
      const previous = previousFocus
      previousFocus = null
      void nextTick(() => {
        if (panel.value) return
        // 原入口仍挂载就回到原处；入口已被卸载（例如空态卡片在面板打开时消失）则交给页面选择当前存在的落点。
        if (previous?.isConnected) previous.focus({preventScroll: true})
        else restoreFocus?.()
      })
    }
  }

  function closePanel(): void {
    if (writing.value || editor.saving.value) return
    showPanel('')
  }

  function openActions(point: Coordinates): void {
    if (!foreground.value || writing.value || editor.saving.value) return
    preview.clear()
    browse.value = false
    actionMenu.value = point
  }

  function dismissActions(): void {
    actionMenu.value = null
  }

  function openPanel(mode: 'mine' | 'notices' | 'publish'): void {
    dismissActions()
    if (!foreground.value || writing.value || editor.saving.value) return
    if (panel.value === mode) {
      panelHeading.value?.focus({preventScroll: true})
      return
    }
    showPanel(mode)
    if (mode === 'mine') void personal.loadMine()
    else if (mode === 'notices') void personal.loadNotices()
    else editor.open('publish')
  }

  function changePersonalPage(page?: number): void {
    if (panel.value === 'mine') void personal.loadMine(page)
    if (panel.value === 'notices') void personal.loadNotices(page)
  }

  function openDetail(id: string, contentId?: string): void {
    if (!foreground.value || writing.value || editor.saving.value) return
    editor.close()
    preview.clear()
    showPanel('detail')
    detail.open(id, contentId)
  }

  function locate(point: GalaxyPosition): void {
    map.stopZoom()
    const next = includeStar(map.camera.value, point, map.size.value)
    if (next !== map.camera.value) map.updateCamera(next, map.size.value)
  }

  function openMine(star: MyStar): void {
    if (!star.withdrawn && star.reviewState === 'approved') locate(star)
    openDetail(star.starId)
  }

  function onUnavailable(starId: string): void {
    if (recentlySaved.value?.starId === starId) recentlySaved.value = null
    if (preview.hovered.value === starId) preview.clear()
    editor.close()
    confirmWithdraw.value = false
    map.removeStar(starId)
    void personal.loadMine(undefined, true)
  }

  function hover(point: DisplayStar | null, anchor?: Coordinates): void {
    preview.select(point, anchor)
  }

  // 点赞等写入成功后以服务响应为准，重读避免较早的详情回执覆盖修改。
  async function changeStar(action: 'like' | 'withdraw'): Promise<void> {
    const star = detail.detail.value
    const scope = accountSessionKey.value
    if (!star || !scope || writing.value || editor.saving.value) return
    writing.value = true
    actionError.value = ''
    try {
      if (action === 'withdraw') {
        await feedbackCall('Withdraw', {starId: star.starId})
        if (scope !== accountSessionKey.value) return
        detail.clearUnavailable(star.starId)
        status.value = '已撤回这颗星。'
      } else {
        const result = await feedbackCall('Like', {
          starId: star.starId, bodyVersion: star.original.bodyVersion, liked: !star.liked,
        })
        if (scope !== accountSessionKey.value) return
        if (detail.detail.value?.starId === star.starId) detail.detail.value = {...detail.detail.value, ...result}
        void detail.refresh(true, true)
        map.refreshNow()
      }
    } catch (failure) {
      if (scope !== accountSessionKey.value) return
      actionError.value = failure instanceof Error ? failure.message : '操作失败，请重试。'
      if (failure instanceof FeedbackError) {
        if (failure.reason === 'unavailable') detail.clearUnavailable(star.starId)
        if (failure.reason === 'versionChanged') void detail.refresh(false, true)
      }
    } finally {
      if (scope === accountSessionKey.value) writing.value = false
    }
  }

  function resetCamera(): void {
    map.stopZoom()
    preview.clear()
    map.updateCamera(HOME_CAMERA, map.size.value)
  }

  function zoom(factor: number): void {
    if (map.zoomBy(factor, zoomLimit.value)) preview.clear()
  }

  function syncPersonal(): void {
    void personal.loadMine(undefined, true)
    if (panel.value === 'notices') void personal.loadNotices(undefined, true)
  }

  function sync(): void {
    if (!foreground.value) return
    map.refreshNow()
    if (panel.value === 'detail' && !writing.value && !editor.saving.value) void detail.refresh(true)
    if (++ticks % 3 === 0 || panel.value === 'mine' || panel.value === 'notices') syncPersonal()
  }

  function handleVisibility(): void {
    visible.value = !document.hidden
  }

  function enter(): void {
    active.value = true
  }

  function leave(): void {
    active.value = false
  }

  watch([points, map.snapshotVersion, previewActive], ([current, version, enabled]) => {
    if (!enabled) return
    preview.reconcile(current, privateStars.value, version)
    preview.prewarm(visibleStars.value)
  }, {immediate: true})
  watch([map.camera, map.size], (_, __, cleanup) => {
    const timer = window.setTimeout(() => preview.prewarm(visibleStars.value), 100)
    cleanup(() => window.clearTimeout(timer))
  })
  watch(detail.detail, value => { if (value) preview.primeDetail(value) })
  watch(personal.mine, ({page}) => {
    if (recentlySaved.value && page.list?.some((star) => star.starId === recentlySaved.value?.starId)) recentlySaved.value = null
  })
  watch(foreground, (isForeground) => {
    window.clearInterval(timer)
    if (!isForeground) { dismissActions(); return }
    syncPersonal()
    sync()
    timer = window.setInterval(sync, 2000)
  }, {flush: 'post'})
  watch([foreground, () => route.query.star, feedbackDeepLinkRevision], ([isForeground, id, revision], [, , previousRevision]) => {
    if (isForeground && route.name === '/community/feedback' && typeof id === 'string' && /^\d+$/.test(id) && (id !== deepLink || revision !== previousRevision)) {
      deepLink = id
      openDetail(id)
    }
  }, {flush: 'post'})
  watch(accountSessionKey, () => {
    dismissActions()
    deepLink = ''
    recentlySaved.value = null
    previousFocus = null
    panel.value = ''
    browse.value = false
    confirmWithdraw.value = false
    writing.value = false
    actionError.value = ''
    status.value = ''
  }, {flush: 'sync'})
  onMounted(() => {
    enter()
    document.addEventListener('visibilitychange', handleVisibility)
    window.addEventListener('focus', sync)
    window.addEventListener('blur', dismissActions)
    window.addEventListener('resize', dismissActions)
    window.addEventListener('online', sync)
  })
  onActivated(enter)
  onDeactivated(leave)
  onScopeDispose(() => {
    leave()
    document.removeEventListener('visibilitychange', handleVisibility)
    window.removeEventListener('focus', sync)
    window.removeEventListener('blur', dismissActions)
    window.removeEventListener('resize', dismissActions)
    window.removeEventListener('online', sync)
    window.clearInterval(timer)
  })

  return {
    foreground, map, preview, personal, editor, detail, panel, panelHeading, browse, points, visibleStars,
    confirmWithdraw, writing, actionError, status, currentCollection, showPanel, closePanel, openPanel,
    actionMenu, openActions, dismissActions,
    changePersonalPage, openDetail, openMine, hover, changeStar, resetCamera, zoom, zoomLimit, locate,
  }
}
