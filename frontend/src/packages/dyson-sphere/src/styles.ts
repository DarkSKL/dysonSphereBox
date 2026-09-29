/**
 * 场景样式：容器、tooltip、成就卡片、可访问隐藏列表、WebGL 降级画面。
 * 主题通过容器上的 data-theme 切换；可通过覆盖 --dyson-* CSS 变量调整配色。
 *
 * 默认由 createDysonSphere 自动注入到 document.head（injectStyles: true）。
 * 若关闭自动注入（例如需要 CSP nonce 或自行打包样式），请自行插入 DYSON_SPHERE_CSS。
 */
export const DYSON_SPHERE_CSS = `

.dyson-root {
  --dyson-card-bg: rgba(8, 16, 30, 0.82);
  --dyson-card-border: rgba(121, 208, 255, 0.35);
  --dyson-card-text: #e3eefb;
  --dyson-card-muted: #8ea4bf;
  --dyson-track: rgba(121, 208, 255, 0.16);
  --dyson-accent: #79d0ff;
  --dyson-bronze: #e58b4e;
  --dyson-silver: #d2def0;
  --dyson-gold: #ffd044;
  --dyson-locked: #6d7f98;

  position: relative;
  overflow: hidden;
  isolation: isolate;
  background: #03060d;
  transition: background-color 400ms ease;
}

.dyson-root[data-theme='light'] {
  --dyson-card-bg: rgba(248, 248, 248, 0.9);
  --dyson-card-border: rgba(0, 0, 0, 0.16);
  --dyson-card-text: #18181a;
  --dyson-card-muted: #707072;
  --dyson-track: rgba(0, 0, 0, 0.1);
  --dyson-accent: #1c1c1e;
  --dyson-bronze: #94682a;
  --dyson-silver: #7a7a80;
  --dyson-gold: #a88418;
  --dyson-locked: #b0b0b4;
  background: #ebebeb;
}

.dyson-root canvas {
  display: block;
  width: 100%;
  height: 100%;
  outline: none;
}

.dyson-overlay {
  position: absolute;
  inset: 0;
  pointer-events: none;
  font-family: inherit;
}

.dyson-overlay > * {
  position: absolute;
  left: 0;
  top: 0;
  will-change: transform;
}

.dyson-tooltip {
  padding: 3px 8px;
  border: 1px solid var(--dyson-card-border);
  border-radius: 3px;
  background: var(--dyson-card-bg);
  color: var(--dyson-card-text);
  font-size: 12px;
  line-height: 18px;
  white-space: nowrap;
  backdrop-filter: blur(6px);
  opacity: 0;
  transition: opacity 120ms ease, background-color 400ms ease, color 400ms ease, border-color 400ms ease;
}

.dyson-tooltip.is-visible {
  opacity: 1;
}

.dyson-tooltip[data-tier='bronze'] { border-left: 2px solid var(--dyson-bronze); }
.dyson-tooltip[data-tier='silver'] { border-left: 2px solid var(--dyson-silver); }
.dyson-tooltip[data-tier='gold'] { border-left: 2px solid var(--dyson-gold); }

/* 选中节点外的定位角标 */
.dyson-marker {
  width: 0;
  height: 0;
  opacity: 0;
  transition: opacity 160ms ease;
}

.dyson-marker::before,
.dyson-marker::after {
  content: '';
  position: absolute;
  left: -14px;
  top: -14px;
  width: 28px;
  height: 28px;
  border: 1px solid var(--dyson-accent);
  clip-path: polygon(0 0, 30% 0, 30% 12%, 12% 12%, 12% 30%, 0 30%, 0 0, 100% 0, 100% 30%, 88% 30%, 88% 12%, 70% 12%, 70% 0, 100% 0, 100% 100%, 70% 100%, 70% 88%, 88% 88%, 88% 70%, 100% 70%, 100% 100%, 0 100%, 0 70%, 12% 70%, 12% 88%, 30% 88%, 30% 100%, 0 100%);
}

.dyson-marker::after {
  border: none;
  clip-path: none;
  left: -2px;
  top: -2px;
  width: 4px;
  height: 4px;
  background: var(--dyson-accent);
  border-radius: 50%;
  opacity: 0.8;
}

.dyson-marker.is-visible {
  opacity: 1;
}

.dyson-card {
  width: 240px;
  padding: 10px 12px 11px;
  border: 1px solid var(--dyson-card-border);
  border-radius: 4px;
  background: var(--dyson-card-bg);
  color: var(--dyson-card-text);
  font-size: 12px;
  line-height: 1.5;
  backdrop-filter: blur(8px);
  pointer-events: auto;
  opacity: 0;
  visibility: hidden;
  transition: opacity 160ms ease, visibility 0s linear 160ms, background-color 400ms ease, color 400ms ease,
    border-color 400ms ease;
  outline: none;
}

.dyson-card.is-visible {
  opacity: 1;
  visibility: visible;
  transition: opacity 160ms ease, visibility 0s, background-color 400ms ease, color 400ms ease,
    border-color 400ms ease;
}

.dyson-card:focus-visible {
  box-shadow: 0 0 0 2px var(--dyson-accent);
}

/* 卡片靠近节点一侧的等级色条 */
.dyson-card::before {
  content: '';
  position: absolute;
  top: 10px;
  bottom: 10px;
  left: -1px;
  width: 2px;
  background: var(--tier-color, var(--dyson-accent));
}

.dyson-card.is-flipped::before {
  left: auto;
  right: -1px;
}

.dyson-card[data-tier='bronze'] { --tier-color: var(--dyson-bronze); }
.dyson-card[data-tier='silver'] { --tier-color: var(--dyson-silver); }
.dyson-card[data-tier='gold'] { --tier-color: var(--dyson-gold); }
.dyson-card.is-locked { --tier-color: var(--dyson-locked); }

.dyson-card__head {
  display: flex;
  align-items: center;
  gap: 6px;
}

.dyson-card__title {
  flex: 1;
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  line-height: 20px;
}

.dyson-card__close {
  width: 20px;
  height: 20px;
  padding: 0;
  border: 1px solid transparent;
  border-radius: 3px;
  background: none;
  color: var(--dyson-card-muted);
  font-size: 16px;
  line-height: 18px;
  cursor: pointer;
}

.dyson-card__close:hover,
.dyson-card__close:focus-visible {
  border-color: var(--dyson-card-border);
  color: var(--dyson-card-text);
  outline: none;
}

.dyson-tier {
  padding: 0 5px;
  border: 1px solid currentColor;
  border-radius: 2px;
  font-size: 11px;
  line-height: 16px;
}

.dyson-tier--bronze { color: var(--dyson-bronze); }
.dyson-tier--silver { color: var(--dyson-silver); }
.dyson-tier--gold { color: var(--dyson-gold); }
.is-locked .dyson-tier { color: var(--dyson-locked); }

.dyson-card__desc {
  margin: 6px 0 8px;
  color: var(--dyson-card-muted);
}

.dyson-card__progress {
  display: flex;
  align-items: center;
  gap: 8px;
}

.dyson-bar {
  flex: 1;
  height: 4px;
  border-radius: 2px;
  background: var(--dyson-track);
  overflow: hidden;
}

.dyson-bar > i {
  display: block;
  height: 100%;
  background: var(--tier-color, var(--dyson-accent));
  transition: width 600ms ease;
}

.dyson-card__count {
  font-variant-numeric: tabular-nums;
}

.dyson-card__meta {
  margin-top: 6px;
  color: var(--dyson-card-muted);
  font-size: 11px;
}

/* 视觉隐藏，但可被读屏与 Tab 聚焦 */
.dyson-sr {
  position: absolute !important;
  width: 1px;
  height: 1px;
  margin: -1px;
  padding: 0;
  overflow: hidden;
  clip: rect(0 0 0 0);
  white-space: nowrap;
  border: 0;
}

/* WebGL 不可用时的静态降级 */
.dyson-fallback {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: flex-end;
  justify-content: flex-end;
  padding: 14px 18px;
  background:
    radial-gradient(ellipse 22% 55% at 64% 50%, rgba(255, 196, 107, 0.9), rgba(255, 150, 60, 0.25) 35%, transparent 70%),
    radial-gradient(ellipse 45% 90% at 64% 50%, rgba(80, 150, 255, 0.18), transparent 70%),
    linear-gradient(120deg, #050a16, #0d1a33 55%, #1b1030);
  color: #8ea4bf;
  font-size: 12px;
}

.dyson-root[data-theme='light'] .dyson-fallback {
  background:
    radial-gradient(ellipse 22% 55% at 64% 50%, rgba(255, 190, 90, 0.85), rgba(255, 170, 80, 0.2) 35%, transparent 70%),
    linear-gradient(120deg, #f2f2f2, #e4e4e4 55%, #ececec);
  color: #707072;
}
`;

const STYLE_ID = 'dyson-sphere-styles';
let refCount = 0;

/** 注入样式（引用计数，多个场景共享同一个 <style>） */
export function acquireStyles(): void {
  refCount++;
  if (typeof document === 'undefined' || document.getElementById(STYLE_ID)) return;
  const style = document.createElement('style');
  style.id = STYLE_ID;
  style.textContent = DYSON_SPHERE_CSS;
  document.head.append(style);
}

/** 释放样式引用；最后一个场景销毁时移除 <style> */
export function releaseStyles(): void {
  refCount = Math.max(0, refCount - 1);
  if (refCount === 0 && typeof document !== 'undefined') document.getElementById(STYLE_ID)?.remove();
}
