/**
 * 场景样式：容器、阶段标题、悬停提示、空场景说明与降级提示。
 * 主题通过容器上的 data-theme 切换；可覆盖 --algoviz-* 变量调整配色。
 * 默认由 createAlgorithmViz 自动注入到 document.head（injectStyles: true）。
 */
export const ALGORITHM_VIZ_CSS = `
.algoviz-root {
  --algoviz-text: #1f2a3a;
  --algoviz-muted: #5a6b82;
  --algoviz-mark: #f08a24;
  --algoviz-line: rgba(63, 151, 220, 0.55);
  --algoviz-chip-bg: rgba(248, 248, 248, 0.9);
  --algoviz-chip-border: rgba(0, 0, 0, 0.16);
  position: relative;
  overflow: hidden;
  isolation: isolate;
  background: linear-gradient(180deg, #dfe8f4 0%, #b3c2d8 100%);
}
.algoviz-root[data-theme='dark'] {
  --algoviz-text: #e6f3ff;
  --algoviz-muted: #8fb3d9;
  --algoviz-mark: #ff9d3c;
  --algoviz-line: rgba(76, 194, 255, 0.6);
  --algoviz-chip-bg: rgba(8, 16, 30, 0.82);
  --algoviz-chip-border: rgba(121, 208, 255, 0.35);
  background: linear-gradient(180deg, #01030a 0%, #0a1430 100%);
}
.algoviz-root canvas {
  display: block;
  width: 100%;
  height: 100%;
  outline: none;
  touch-action: none;
}
.algoviz-overlay {
  position: absolute;
  inset: 0;
  pointer-events: none;
  font-family: inherit;
  letter-spacing: 0;
}
.algoviz-caption {
  position: absolute;
  top: calc(var(--algoviz-pad-top, 0px) + 8%);
  left: 50%;
  display: flex;
  max-width: min(520px, 80%);
  flex-direction: column;
  align-items: center;
  gap: 6px;
  color: var(--algoviz-text);
  text-align: center;
  opacity: 0;
  transform: translate(-50%, 6px);
  filter: blur(3px);
  transition: opacity 600ms ease, transform 600ms ease, filter 600ms ease;
}
.algoviz-caption.is-visible {
  opacity: 1;
  transform: translate(-50%, 0);
  filter: blur(0);
}
.algoviz-caption-mark {
  width: 64px;
  height: 1px;
  background: linear-gradient(90deg, transparent, var(--algoviz-line), transparent);
  position: relative;
}
.algoviz-caption-mark::after {
  content: '';
  position: absolute;
  top: -3px;
  left: calc(50% - 3px);
  width: 6px;
  height: 6px;
  background: var(--algoviz-mark);
  transform: rotate(45deg);
}
.algoviz-caption-title {
  font-size: 18px;
  font-weight: 400;
  line-height: 1.4;
  text-shadow: 0 1px 10px rgba(255, 255, 255, 0.5);
}
.algoviz-root[data-theme='dark'] .algoviz-caption-title {
  text-shadow: 0 0 14px rgba(76, 194, 255, 0.45);
}
.algoviz-caption-detail {
  color: var(--algoviz-muted);
  font-size: 12px;
  line-height: 1.5;
}
.algoviz-tooltip {
  position: absolute;
  top: 0;
  left: 0;
  padding: 3px 8px;
  border: 1px solid var(--algoviz-chip-border);
  border-radius: 2px;
  background: var(--algoviz-chip-bg);
  color: var(--algoviz-text);
  font-size: 12px;
  line-height: 18px;
  white-space: nowrap;
  backdrop-filter: blur(6px);
  opacity: 0;
  transition: opacity 120ms ease;
}
.algoviz-tooltip.is-visible {
  opacity: 1;
}
.algoviz-empty {
  position: absolute;
  bottom: 18%;
  left: 50%;
  max-width: 80%;
  color: var(--algoviz-muted);
  font-size: 13px;
  line-height: 1.6;
  text-align: center;
  opacity: 0;
  transform: translateX(-50%);
  transition: opacity 400ms ease;
}
.algoviz-empty.is-visible {
  opacity: 1;
}
.algoviz-fallback {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--algoviz-muted);
  font-size: 13px;
}
@media (prefers-reduced-motion: reduce) {
  .algoviz-caption, .algoviz-empty, .algoviz-tooltip { transition: none; filter: none; }
}
`;

let refCount = 0;
let styleElement: HTMLStyleElement | null = null;

/** 注入样式（引用计数，多实例共享同一个 style 标签） */
export function acquireStyles(): void {
  refCount++;
  if (styleElement) return;
  styleElement = document.createElement('style');
  styleElement.dataset.algorithmViz = 'true';
  styleElement.textContent = ALGORITHM_VIZ_CSS;
  document.head.append(styleElement);
}

export function releaseStyles(): void {
  refCount = Math.max(0, refCount - 1);
  if (refCount === 0 && styleElement) {
    styleElement.remove();
    styleElement = null;
  }
}
