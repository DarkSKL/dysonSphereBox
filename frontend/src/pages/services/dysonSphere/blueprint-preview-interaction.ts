// =============== 蓝图预览输入适配 ===============
// 说明：当前随包预览库的控制器映射为左键旋转，这里改为左键拖动平移、右键拖动旋转，并保留滚轮缩放；同时区分拖动和建筑单击。
// 边界：仅适配已固定版本的 controls 映射，不改第三方包、相机或蓝图；实例释放前移除监听并恢复原设置。
interface PreviewControls {
  mouseButtons: {LEFT: number; MIDDLE: number; RIGHT: number}
  touches: {ONE: number}
}

const MOUSE_BUTTON = {LEFT: 0, RIGHT: 2} as const
// 控制器沿用 Three.js 的动作编号：ROTATE = 0、DOLLY = 1、PAN = 2。
const CONTROL_ACTION = {ROTATE: 0, PAN: 2} as const

const isRecord = (value: unknown): value is Record<string, unknown> => typeof value === 'object' && value !== null

// 使用运行时校验隔离预览库未在 README 中公开的映射字段；升级库时不把不兼容状态静默报告为可交互。
// 当前构建的公开类是包装层，controls 挂在内核实例（viewer.core）上；包装层与内核直挂两种形态都接受。
const resolveControls = (viewer: unknown): PreviewControls => {
  const host = isRecord(viewer) ? viewer : null
  const core = isRecord(host?.core) ? host.core : null
  const controls = host?.controls ?? core?.controls ?? null
  if (!isRecord(controls) || !isRecord(controls.mouseButtons) || !isRecord(controls.touches)
    || typeof controls.mouseButtons.LEFT !== 'number' || typeof controls.mouseButtons.RIGHT !== 'number'
    || typeof controls.touches.ONE !== 'number') {
    throw new Error('当前蓝图预览库不支持自定义拖动操作，请检查预览库版本。')
  }
  return controls as unknown as PreviewControls
}

export const enableBlueprintPreviewInteraction = (viewer: unknown, canvas: HTMLCanvasElement): (() => void) => {
  const controls = resolveControls(viewer)
  const previousLeft = controls.mouseButtons.LEFT
  const previousRight = controls.mouseButtons.RIGHT
  const previousTouch = controls.touches.ONE
  const previousTouchAction = canvas.style.touchAction
  // 左键拖动平移、右键拖动旋转；单指拖动与左键保持一致，双指缩放旋转沿用控制器默认值。
  controls.mouseButtons.LEFT = CONTROL_ACTION.PAN
  controls.mouseButtons.RIGHT = CONTROL_ACTION.ROTATE
  controls.touches.ONE = CONTROL_ACTION.PAN
  canvas.style.touchAction = 'none'

  let pointer: {id: number; x: number; y: number; moved: boolean} | null = null
  let suppressClick = false
  const onPointerDown = (event: PointerEvent): void => {
    suppressClick = false
    if (event.button !== MOUSE_BUTTON.LEFT) return
    pointer = {id: event.pointerId, x: event.clientX, y: event.clientY, moved: false}
  }
  const onPointerMove = (event: PointerEvent): void => {
    if (pointer?.id !== event.pointerId) return
    if (Math.hypot(event.clientX - pointer.x, event.clientY - pointer.y) > 5) pointer.moved = true
  }
  const onPointerUp = (event: PointerEvent): void => {
    if (pointer?.id !== event.pointerId) return
    onPointerMove(event)
    suppressClick = pointer.moved
    pointer = null
  }
  const onPointerCancel = (): void => { pointer = null; suppressClick = false }
  const onClick = (event: MouseEvent): void => {
    if (!suppressClick) return
    suppressClick = false
    event.preventDefault()
    event.stopImmediatePropagation()
  }
  canvas.addEventListener('pointerdown', onPointerDown, true)
  canvas.addEventListener('pointermove', onPointerMove, true)
  canvas.addEventListener('pointerup', onPointerUp, true)
  canvas.addEventListener('pointercancel', onPointerCancel, true)
  canvas.addEventListener('click', onClick, true)

  return () => {
    canvas.removeEventListener('pointerdown', onPointerDown, true)
    canvas.removeEventListener('pointermove', onPointerMove, true)
    canvas.removeEventListener('pointerup', onPointerUp, true)
    canvas.removeEventListener('pointercancel', onPointerCancel, true)
    canvas.removeEventListener('click', onClick, true)
    controls.mouseButtons.LEFT = previousLeft
    controls.mouseButtons.RIGHT = previousRight
    controls.touches.ONE = previousTouch
    canvas.style.touchAction = previousTouchAction
  }
}
