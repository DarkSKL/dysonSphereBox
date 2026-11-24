import {onScopeDispose, shallowRef, type ShallowRef} from 'vue'

import type {Camera} from './galaxy-camera'

// =============== 居中变焦 ===============
// 输入按目标倍率累积，镜头在对数空间连续追随；反向滚动立即从当前画面接管。
export function useGalaxyZoom(camera: ShallowRef<Camera>) {
  const reduced = window.matchMedia('(prefers-reduced-motion: reduce)')
  const outwardTarget = shallowRef<number | null>(null)
  let frame = 0
  let target = camera.value.zoom
  let direction = 0
  let previous = 0

  function stopZoom(): void {
    cancelAnimationFrame(frame)
    frame = 0
    direction = 0
    target = camera.value.zoom
    outwardTarget.value = null
  }

  function step(now: number): void {
    const dt = Math.max(0, now - previous)
    previous = now
    const gap = Math.log(target / camera.value.zoom)
    const remaining = gap * Math.exp(-dt / 55)
    const done = reduced.matches || Math.abs(remaining) < .0015
    camera.value = {...camera.value, zoom: done ? target : target * Math.exp(-remaining)}
    if (done) stopZoom()
    else frame = requestAnimationFrame(step)
  }

  function zoomBy(factor: number, limit: number): boolean {
    if (!Number.isFinite(factor) || factor <= 0 || factor === 1) return false
    const sign = Math.sign(factor - 1)
    const base = frame && direction === sign ? target : camera.value.zoom
    const ceiling = sign > 0 ? Math.max(camera.value.zoom, limit) : base
    const next = Math.max(1, Math.min(ceiling, base * factor))
    if (next === camera.value.zoom) { stopZoom(); return false }
    direction = sign
    target = next
    outwardTarget.value = sign < 0 ? next : null
    if (reduced.matches) { camera.value = {...camera.value, zoom: target}; stopZoom(); return true }
    if (!frame) { previous = performance.now(); frame = requestAnimationFrame(step) }
    return true
  }

  onScopeDispose(stopZoom)
  return {zoomBy, stopZoom, outwardTarget}
}
