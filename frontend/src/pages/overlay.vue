<script setup lang="ts">
import {ChevronLeft, ChevronRight, MonitorUp, RadioTower} from '@lucide/vue'
import {computed, nextTick, onBeforeUnmount, onMounted, ref} from 'vue'
import {WindowGetPosition, WindowGetSize, WindowSetPosition, WindowSetSize} from '../../wailsjs/runtime/runtime'
import {GetOverlayTargetWindow, SetOverlayDragging} from '../../wailsjs/go/overlay/OverlayRuntimeService'
import {overlay} from '../../wailsjs/go/models'

type OverlayAnchorSide = 'left' | 'right'
type OverlayWindowRect = {left: number; top: number; width: number; height: number}

definePage({
  meta: {
    layout: 'overlay',
    title: '悬浮窗',
  },
})

// =============== 目标窗口状态 ===============
// 说明：维护 Overlay 附进程最近贴附到的浏览器窗口快照。
// 使用：页面定时读取后端控制器状态，只展示当前监听结果，不承担窗口定位。
const targetWindow = ref<overlay.WindowSnapshotDTO | null>(null)
const lastUpdatedAt = ref('')
const isDrawerVisible = ref(true)
const anchorSide = ref<OverlayAnchorSide>('left')
const isDraggingToolbar = ref(false)
let refreshTimer: number | undefined
let isRefreshingTarget = false
let lastAppliedRect: {left: number; top: number; width: number; height: number} | null = null
let lastSizeAppliedAt = 0
let anchorRatioY = 0.3
let dragToolbarOffsetX = 0
let dragWatchTimer: number | undefined
let dragWatchLastPosition: {x: number; y: number} | null = null
let dragStableTicks = 0

const overlayFollowInterval = 80
const overlayMoveThreshold = 2
const overlaySizeThreshold = 16
const overlaySizeApplyInterval = 360
const overlayToolbarWidth = 108
const overlayVerticalMargin = 24
const dragSettleDelay = 80
const dragWatchInterval = 120
const dragStableThreshold = 2

const statusLabel = computed(() => {
  if (!targetWindow.value?.found) {
    return '等待浏览器窗口'
  }
  if (targetWindow.value.minimized) {
    return '目标已最小化'
  }
  return '已贴附窗口'
})

const rectLabel = computed(() => {
  const rect = targetWindow.value?.attachedRect?.width ? targetWindow.value.attachedRect : targetWindow.value?.rect
  if (!targetWindow.value?.found || !rect) {
    return '未获取窗口坐标'
  }
  return `${rect.left}, ${rect.top} · ${rect.width} x ${rect.height}`
})

const drawerButtonLabel = computed(() => (isDrawerVisible.value ? '隐藏展开栏' : '显示展开栏'))

const activeOverlayWidth = (rectWidth: number): number => {
  return isDrawerVisible.value ? rectWidth : overlayToolbarWidth
}

const clamp = (value: number, min: number, max: number): number => {
  if (max < min) {
    return min
  }
  return Math.min(Math.max(value, min), max)
}

const getAnchorTopRange = (snapshot: overlay.WindowSnapshotDTO, overlayHeight: number): {min: number; max: number} => {
  const targetRect = snapshot.rect
  const min = targetRect.top + overlayVerticalMargin
  const max = Math.max(min, targetRect.bottom - overlayHeight - overlayVerticalMargin)
  return {min, max}
}

const resolveOverlayRect = (snapshot: overlay.WindowSnapshotDTO): OverlayWindowRect | null => {
  const attachedRect = snapshot.attachedRect
  const targetRect = snapshot.rect
  if (!snapshot.found || !attachedRect || !targetRect || attachedRect.width <= 0 || attachedRect.height <= 0) {
    return null
  }

  const width = activeOverlayWidth(attachedRect.width)
  const height = attachedRect.height
  const topRange = getAnchorTopRange(snapshot, height)
  const top = topRange.min + (topRange.max - topRange.min) * anchorRatioY

  return {
    left: anchorSide.value === 'left' ? targetRect.left : targetRect.right - width,
    top: clamp(top, topRange.min, topRange.max),
    width,
    height,
  }
}

const resolveToolbarBounds = (overlayRect: OverlayWindowRect, toolbarOffsetX?: number): {left: number; right: number} => {
  if (typeof toolbarOffsetX === 'number') {
    return {
      left: overlayRect.left + toolbarOffsetX,
      right: overlayRect.left + toolbarOffsetX + overlayToolbarWidth,
    }
  }
  if (anchorSide.value === 'right' && isDrawerVisible.value) {
    return {
      left: overlayRect.left + overlayRect.width - overlayToolbarWidth,
      right: overlayRect.left + overlayRect.width,
    }
  }
  return {
    left: overlayRect.left,
    right: overlayRect.left + overlayToolbarWidth,
  }
}

const syncAnchorRatioFromTop = (snapshot: overlay.WindowSnapshotDTO, top: number, overlayHeight: number): void => {
  const range = getAnchorTopRange(snapshot, overlayHeight)
  if (range.max <= range.min) {
    anchorRatioY = 0
    return
  }
  anchorRatioY = (clamp(top, range.min, range.max) - range.min) / (range.max - range.min)
}

// =============== 状态刷新 ===============
// 说明：轮询附进程后端快照，便于原型阶段直接确认 Overlay 正在跟随哪个窗口。
// 边界：失败时只清空状态，不弹出主程序消息框。
const refreshTargetWindow = async (): Promise<void> => {
  if (isRefreshingTarget) {
    return
  }
  isRefreshingTarget = true
  try {
    const snapshot = await GetOverlayTargetWindow()
    targetWindow.value = snapshot
    if (!isDraggingToolbar.value) {
      await renderOverlayWindow(snapshot)
    }
    lastUpdatedAt.value = new Date().toLocaleTimeString()
  } catch {
    targetWindow.value = null
  } finally {
    isRefreshingTarget = false
  }
}

// =============== Overlay 窗口渲染约束 ===============
// 说明：集中处理目标窗口矩形、吸附边、展开状态、纵向比例和尺寸节流，所有窗口定位都必须走这里。
// 边界：仅在后端已经计算出有效目标快照时执行，不在这里修改目标窗口选择。
const resolveOverlayLeft = (snapshot: overlay.WindowSnapshotDTO, width: number): number => {
  return anchorSide.value === 'left' ? snapshot.rect.left : snapshot.rect.right - width
}

const afterFrame = (): Promise<void> => {
  return new Promise((resolve) => {
    window.requestAnimationFrame(() => resolve())
  })
}

const renderOverlayWindow = async (
  snapshot: overlay.WindowSnapshotDTO,
  options: {forceSize?: boolean; forceMove?: boolean} = {},
): Promise<void> => {
  const rect = resolveOverlayRect(snapshot)
  if (!rect) {
    return
  }
  if (!lastAppliedRect) {
    lastAppliedRect = {
      left: rect.left,
      top: rect.top,
      width: rect.width,
      height: rect.height,
    }
    WindowSetSize(rect.width, rect.height)
    if (anchorSide.value === 'right') {
      await afterFrame()
      const size = await WindowGetSize()
      rect.width = size.w
      rect.left = resolveOverlayLeft(snapshot, size.w)
      lastAppliedRect.left = rect.left
      lastAppliedRect.width = size.w
    }
    WindowSetPosition(rect.left, rect.top)
    return
  }

  const shouldMove =
    options.forceMove ||
    Math.abs(rect.left - lastAppliedRect.left) >= overlayMoveThreshold ||
    Math.abs(rect.top - lastAppliedRect.top) >= overlayMoveThreshold
  const sizeChanged =
    Math.abs(rect.width - lastAppliedRect.width) >= overlaySizeThreshold ||
    Math.abs(rect.height - lastAppliedRect.height) >= overlaySizeThreshold
  const now = Date.now()

  if (sizeChanged && (options.forceSize || now - lastSizeAppliedAt >= overlaySizeApplyInterval)) {
    lastSizeAppliedAt = now
    lastAppliedRect.width = rect.width
    lastAppliedRect.height = rect.height
    WindowSetSize(rect.width, rect.height)
    if (anchorSide.value === 'right') {
      await afterFrame()
      const size = await WindowGetSize()
      rect.width = size.w
      rect.left = resolveOverlayLeft(snapshot, size.w)
      lastAppliedRect.width = size.w
    }
  }

  if (shouldMove) {
    lastAppliedRect.left = rect.left
    lastAppliedRect.top = rect.top
    WindowSetPosition(rect.left, rect.top)
  }
}

// =============== 工具栏拖动吸边 ===============
// 说明：拖动左侧工具栏时先自由预览移动，松手后按鼠标所在目标窗口半区吸附到左边或右边，并记录相对顶部偏移。
// 边界：只改变当前 Overlay 的锚点状态，不改变后端目标窗口选择。
const settleToolbarDrag = async (): Promise<void> => {
  const snapshot = targetWindow.value
  if (!isDraggingToolbar.value) {
    return
  }
  isDraggingToolbar.value = false
  void SetOverlayDragging(false)
  if (dragWatchTimer) {
    window.clearInterval(dragWatchTimer)
    dragWatchTimer = undefined
  }
  window.removeEventListener('pointerup', stopToolbarDrag)
  window.removeEventListener('pointercancel', stopToolbarDrag)
  if (snapshot?.found && snapshot.rect && snapshot.attachedRect?.height) {
    const position = await WindowGetPosition()
    const overlayWidth = activeOverlayWidth(snapshot.attachedRect.width)
    lastAppliedRect = {
      left: position.x,
      top: position.y,
      width: overlayWidth,
      height: snapshot.attachedRect.height,
    }
    const targetRect = snapshot.rect
    const toolbarBounds = resolveToolbarBounds(lastAppliedRect, dragToolbarOffsetX)
    const leftDistance = Math.abs(toolbarBounds.left - targetRect.left)
    const rightDistance = Math.abs(toolbarBounds.right - targetRect.right)
    anchorSide.value = leftDistance <= rightDistance ? 'left' : 'right'
    syncAnchorRatioFromTop(snapshot, lastAppliedRect.top, snapshot.attachedRect.height)
    await nextTick()
    await renderOverlayWindow(snapshot, {forceMove: true, forceSize: true})
  }
}

const stopToolbarDrag = (): void => {
  window.setTimeout(() => {
    void settleToolbarDrag()
  }, dragSettleDelay)
}

const cleanupToolbarDrag = (): void => {
  if (dragWatchTimer) {
    window.clearInterval(dragWatchTimer)
    dragWatchTimer = undefined
  }
  window.removeEventListener('pointerup', stopToolbarDrag)
  window.removeEventListener('pointercancel', stopToolbarDrag)
  isDraggingToolbar.value = false
  void SetOverlayDragging(false)
}

const watchNativeDragSettled = async (): Promise<void> => {
  if (!isDraggingToolbar.value) {
    return
  }
  const position = await WindowGetPosition()
  if (
    dragWatchLastPosition &&
    Math.abs(position.x - dragWatchLastPosition.x) < 1 &&
    Math.abs(position.y - dragWatchLastPosition.y) < 1
  ) {
    dragStableTicks += 1
  } else {
    dragStableTicks = 0
    dragWatchLastPosition = position
  }
  if (dragStableTicks >= dragStableThreshold) {
    void settleToolbarDrag()
  }
}

const startToolbarDrag = (event: PointerEvent): void => {
  const target = event.target as HTMLElement | null
  if (target?.closest('.overlay-tool-button')) {
    return
  }
  if (!lastAppliedRect || !targetWindow.value?.found) {
    return
  }
  isDraggingToolbar.value = true
  void SetOverlayDragging(true)
  dragToolbarOffsetX = anchorSide.value === 'right' && isDrawerVisible.value ? lastAppliedRect.width - overlayToolbarWidth : 0
  dragWatchLastPosition = null
  dragStableTicks = 0
  if (dragWatchTimer) {
    window.clearInterval(dragWatchTimer)
  }
  dragWatchTimer = window.setInterval(() => {
    void watchNativeDragSettled()
  }, dragWatchInterval)
  window.addEventListener('pointerup', stopToolbarDrag)
  window.addEventListener('pointercancel', stopToolbarDrag)
}

// =============== 展开栏控制 ===============
// 说明：左侧工具栏按钮控制右侧展开栏显隐，并同步调整原生 Overlay 窗口宽度。
// 边界：只改变当前附进程窗口尺寸，不修改后端目标窗口选择和吸附锚点。
const toggleDrawer = async (): Promise<void> => {
  isDrawerVisible.value = !isDrawerVisible.value
  const snapshot = targetWindow.value
  if (snapshot?.found) {
    await nextTick()
    await renderOverlayWindow(snapshot, {forceMove: true, forceSize: true})
  }
}

onMounted(() => {
  document.documentElement.dataset.overlayRuntime = 'true'
  document.body.dataset.overlayRuntime = 'true'
  void refreshTargetWindow()
  refreshTimer = window.setInterval(() => {
    void refreshTargetWindow()
  }, overlayFollowInterval)
})

onBeforeUnmount(() => {
  delete document.documentElement.dataset.overlayRuntime
  delete document.body.dataset.overlayRuntime
  if (refreshTimer) {
    window.clearInterval(refreshTimer)
  }
  cleanupToolbarDrag()
})
</script>

<template>
  <main class="overlay-root">
    <!-- ~ 悬浮工具栏与展开栏 -->
    <!-- * 说明：工具栏吸附目标窗口单侧边缘，展开栏朝窗口内部展示当前监听状态 -->
    <!-- * 交互：工具栏可点击、可拖动，松手后按最近单侧边缘吸附 -->
    <!-- * 依赖：依赖 OverlayRuntimeService.GetOverlayTargetWindow 返回的窗口快照 -->
    <!-- * 边界：未找到浏览器时展示等待态，不阻塞后端继续扫描 -->
    <section
      class="overlay-shell"
      :class="{
        'is-collapsed': !isDrawerVisible,
        'is-side-right': anchorSide === 'right',
        'is-dragging': isDraggingToolbar,
      }"
      aria-live="polite"
    >
      <aside class="overlay-toolbar" @pointerdown="startToolbarDrag">
        <button class="overlay-tool-button" type="button" :aria-label="drawerButtonLabel" @click="toggleDrawer">
          <MonitorUp :size="22" :stroke-width="2" />
          <ChevronLeft v-if="isDrawerVisible" class="overlay-toggle-icon" :size="16" :stroke-width="2.2" />
          <ChevronRight v-else class="overlay-toggle-icon" :size="16" :stroke-width="2.2" />
        </button>
        <span class="overlay-toolbar-label">悬浮工具栏</span>
        <span class="overlay-toolbar-hint">{{ isDrawerVisible ? '收起' : '展开' }}</span>
      </aside>

      <section v-show="isDrawerVisible" class="overlay-drawer">
        <div class="overlay-panel-header">
          <div class="overlay-icon-box">
            <RadioTower :size="18" :stroke-width="2" />
          </div>
          <div class="min-w-0">
            <p class="overlay-title">悬浮展开栏</p>
            <p class="overlay-subtitle">{{ statusLabel }}</p>
          </div>
        </div>

        <div class="overlay-grid">
          <div class="overlay-metric">
            <span>目标进程</span>
            <strong>{{ targetWindow?.processName || '浏览器待启动' }}</strong>
          </div>
          <div class="overlay-metric">
            <span>吸附区域</span>
            <strong>{{ rectLabel }}</strong>
          </div>
        </div>

        <div class="overlay-target-title">
          <RadioTower :size="15" :stroke-width="2" />
          <span>{{ targetWindow?.title || '打开 Chrome / Edge / Firefox 后，悬浮窗会贴附到浏览器窗口上。' }}</span>
        </div>

        <p class="overlay-footer">Last sync {{ lastUpdatedAt || '--:--:--' }}</p>
      </section>
    </section>
  </main>
</template>

<style scoped lang="scss">
.overlay-root {
  position: fixed;
  inset: 0;
  overflow: hidden;
  background: transparent;
  pointer-events: auto;
}

.overlay-shell {
  position: absolute;
  inset: 0;
  display: grid;
  grid-template-columns: 108px 36px 620px;
  grid-template-areas: "toolbar gap drawer";
  align-items: stretch;
  width: 100%;
  height: 100%;
  box-sizing: border-box;
}

.overlay-shell.is-collapsed {
  grid-template-columns: 108px;
  grid-template-areas: "toolbar";
}

.overlay-shell.is-side-right {
  grid-template-columns: 620px 36px 108px;
  grid-template-areas: "drawer gap toolbar";
}

.overlay-shell.is-side-right.is-collapsed {
  grid-template-columns: 108px;
  grid-template-areas: "toolbar";
}

.overlay-toolbar,
.overlay-drawer {
  box-sizing: border-box;
  border: 1px solid rgb(226 185 93 / 0.42);
  border-radius: 6px;
  background: rgb(5 8 6 / 0.72);
  box-shadow:
    0 18px 46px rgb(0 0 0 / 0.28),
    inset 0 1px 0 rgb(255 255 255 / 0.1);
  color: #f4ead0;
  backdrop-filter: blur(14px);
}

.overlay-toolbar {
  display: grid;
  grid-area: toolbar;
  align-content: center;
  justify-items: center;
  gap: 12px;
  padding: 16px 10px;
  cursor: grab;
  user-select: none;
  touch-action: none;
  --wails-draggable: drag;
}

.overlay-shell.is-dragging .overlay-toolbar {
  cursor: grabbing;
}

.overlay-tool-button {
  position: relative;
  display: grid;
  width: 42px;
  height: 42px;
  place-items: center;
  border: 1px solid rgb(226 185 93 / 0.34);
  border-radius: 5px;
  background: rgb(226 185 93 / 0.14);
  color: #f0c96e;
  cursor: pointer;
  --wails-draggable: no-drag;
  transition:
    border-color 140ms ease,
    background-color 140ms ease,
    transform 140ms ease;
}

.overlay-tool-button:hover {
  border-color: rgb(240 201 110 / 0.72);
  background: rgb(226 185 93 / 0.24);
}

.overlay-tool-button:active {
  transform: translateY(1px);
}

.overlay-tool-button:focus-visible {
  outline: 2px solid rgb(240 201 110 / 0.82);
  outline-offset: 3px;
}

.overlay-toggle-icon {
  position: absolute;
  right: -7px;
  bottom: -7px;
  border: 1px solid rgb(5 8 6 / 0.72);
  border-radius: 999px;
  background: #f0c96e;
  color: #10140c;
}

.overlay-toolbar-label {
  max-width: 4em;
  text-align: center;
  color: rgb(244 234 208 / 0.86);
  font-size: 15px;
  font-weight: 800;
  line-height: 1.45;
}

.overlay-toolbar-hint {
  color: rgb(244 234 208 / 0.58);
  font-size: 12px;
}

.overlay-drawer {
  grid-area: drawer;
  padding: 18px;
}

.overlay-panel-header {
  display: flex;
  align-items: center;
  gap: 10px;
}

.overlay-icon-box {
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  border: 1px solid rgb(226 185 93 / 0.34);
  border-radius: 4px;
  background: rgb(226 185 93 / 0.14);
  color: #f0c96e;
}

.overlay-title {
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
  font-weight: 800;
}

.overlay-subtitle {
  margin: 2px 0 0;
  color: rgb(225 232 218 / 0.7);
  font-size: 12px;
}

.overlay-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  margin-top: 12px;
}

.overlay-metric {
  min-width: 0;
  border: 1px solid rgb(229 204 143 / 0.16);
  border-radius: 4px;
  background: rgb(255 255 255 / 0.06);
  padding: 8px;
}

.overlay-metric span {
  display: block;
  color: rgb(225 232 218 / 0.56);
  font-size: 11px;
}

.overlay-metric strong {
  display: block;
  overflow: hidden;
  margin-top: 4px;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 12px;
}

.overlay-target-title {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 7px;
  margin-top: 10px;
  color: rgb(225 232 218 / 0.76);
  font-size: 12px;
}

.overlay-target-title span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.overlay-footer {
  margin: 10px 0 0;
  color: rgb(225 232 218 / 0.42);
  font-size: 11px;
}
</style>
