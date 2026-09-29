import { isAchievementUnlocked, TIER_LABELS } from '../achievement';
import type { DysonAchievement, DysonTheme } from '../types';

let uid = 0;

function el<K extends keyof HTMLElementTagNameMap>(tag: K, className?: string, text?: string): HTMLElementTagNameMap[K] {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
}

function describe(a: DysonAchievement): string {
  const state = isAchievementUnlocked(a) ? `已解锁，解锁时间 ${a.unlockedAt ?? '未知'}` : '未解锁';
  return `${a.title}，${TIER_LABELS[a.tier]}级成就，进度 ${a.current}/${a.threshold}，${state}`;
}

/** 节点在容器内的屏幕坐标（CSS 像素） */
export interface ScreenPoint {
  x: number;
  y: number;
  /** 是否在相机前方且位于画面内 */
  visible: boolean;
}

export interface OverlayHandlers {
  /** 键盘焦点移到隐藏按钮上时高亮对应节点；-1 取消 */
  onHover(index: number): void;
  /** 通过隐藏按钮打开卡片 */
  onSelect(index: number, viaKeyboard: boolean): void;
  /** 点击卡片关闭按钮 */
  onClose(): void;
}

/**
 * 场景之上的 HTML 覆盖层：悬停 tooltip、跟随节点的成就卡片、视觉隐藏的可访问按钮列表。
 * 只负责 DOM，定位数据由场景每帧提供。所有文本都通过 textContent 写入。
 */
export class Overlay {
  private list: DysonAchievement[] = [];
  private tooltipIndex = -1;
  private cardIndex = -1;
  private returnFocus: HTMLElement | null = null;
  private srButtons: HTMLButtonElement[] = [];

  private readonly root: HTMLDivElement;
  private readonly tooltip: HTMLDivElement;
  private readonly marker: HTMLDivElement;
  private readonly card: HTMLDivElement;
  private readonly cardTier: HTMLSpanElement;
  private readonly cardTitle: HTMLHeadingElement;
  private readonly cardDesc: HTMLParagraphElement;
  private readonly cardBarFill: HTMLElement;
  private readonly cardCount: HTMLSpanElement;
  private readonly cardMeta: HTMLDivElement;
  private readonly sr: HTMLDivElement;
  private readonly srList: HTMLUListElement;
  private readonly live: HTMLDivElement;

  constructor(private readonly container: HTMLElement, private readonly handlers: OverlayHandlers) {
    const id = `dyson-${++uid}`;

    this.root = el('div', 'dyson-overlay');

    this.tooltip = el('div', 'dyson-tooltip');
    this.tooltip.setAttribute('role', 'tooltip');
    this.tooltip.setAttribute('aria-hidden', 'true');

    this.marker = el('div', 'dyson-marker');

    this.card = el('div', 'dyson-card');
    this.card.setAttribute('role', 'dialog');
    this.card.setAttribute('aria-labelledby', `${id}-title`);
    this.card.setAttribute('aria-describedby', `${id}-desc`);
    this.card.tabIndex = -1;
    const head = el('div', 'dyson-card__head');
    this.cardTier = el('span', 'dyson-tier');
    this.cardTitle = el('h3', 'dyson-card__title');
    this.cardTitle.id = `${id}-title`;
    const close = el('button', 'dyson-card__close', '×');
    close.type = 'button';
    close.setAttribute('aria-label', '关闭成就卡片');
    close.addEventListener('click', (e) => {
      e.stopPropagation();
      this.handlers.onClose();
    });
    head.append(this.cardTier, this.cardTitle, close);
    this.cardDesc = el('p', 'dyson-card__desc');
    this.cardDesc.id = `${id}-desc`;
    const progress = el('div', 'dyson-card__progress');
    const bar = el('div', 'dyson-bar');
    this.cardBarFill = el('i');
    bar.append(this.cardBarFill);
    this.cardCount = el('span', 'dyson-card__count');
    progress.append(bar, this.cardCount);
    this.cardMeta = el('div', 'dyson-card__meta');
    this.card.append(head, this.cardDesc, progress, this.cardMeta);
    // 卡片内的点击不应传到场景（场景点击空白处会关闭卡片）
    this.card.addEventListener('click', (e) => e.stopPropagation());

    this.sr = el('div', 'dyson-sr');
    this.sr.append(el('h2', '', '成就节点'));
    this.srList = el('ul');
    this.sr.append(this.srList);
    this.live = el('div', 'dyson-sr');
    this.live.setAttribute('aria-live', 'polite');

    this.root.append(this.marker, this.tooltip, this.card);
    container.append(this.root, this.sr, this.live);
  }

  /** 列表对象被直接引用，数据变化后调用 refresh 更新文案 */
  setAchievements(list: DysonAchievement[]): void {
    this.list = list;
    this.srList.replaceChildren();
    this.srButtons = list.map((a, i) => {
      const li = el('li');
      const btn = el('button', '', a.title);
      btn.type = 'button';
      btn.setAttribute('aria-label', describe(a));
      btn.addEventListener('focus', () => this.handlers.onHover(i));
      btn.addEventListener('blur', () => this.handlers.onHover(-1));
      btn.addEventListener('click', () => {
        this.returnFocus = btn;
        this.handlers.onSelect(i, true);
      });
      li.append(btn);
      this.srList.append(li);
      return btn;
    });
    this.showTooltip(-1);
    this.closeCard();
  }

  showTooltip(index: number): void {
    if (index === this.tooltipIndex) return;
    this.tooltipIndex = index;
    const a = this.list[index];
    if (!a) {
      this.tooltip.classList.remove('is-visible');
      return;
    }
    this.tooltip.textContent = `${a.title} · ${TIER_LABELS[a.tier]}`;
    this.tooltip.dataset.tier = a.tier;
    this.tooltip.classList.add('is-visible');
  }

  /** focus：通过键盘打开时把焦点移到卡片上 */
  openCard(index: number, focus: boolean): void {
    this.cardIndex = index;
    this.refresh(index);
    this.card.classList.add('is-visible');
    this.marker.classList.add('is-visible');
    if (focus) this.card.focus({ preventScroll: true });
  }

  /** 关闭卡片；若卡片是通过键盘打开的，把焦点还给对应按钮 */
  closeCard(): void {
    const wasOpen = this.cardIndex >= 0;
    this.cardIndex = -1;
    this.card.classList.remove('is-visible');
    this.marker.classList.remove('is-visible');
    if (wasOpen && this.returnFocus) this.returnFocus.focus({ preventScroll: true });
    this.returnFocus = null;
  }

  /** 数据变化后刷新卡片与按钮文案；announce 为 true 时通过 aria-live 播报解锁 */
  refresh(index: number, announce = false): void {
    const a = this.list[index];
    if (!a) return;
    this.srButtons[index]?.setAttribute('aria-label', describe(a));
    if (announce) this.live.textContent = `成就解锁：${a.title}`;
    if (index !== this.cardIndex) return;
    const unlocked = isAchievementUnlocked(a);
    this.card.dataset.tier = a.tier;
    this.card.classList.toggle('is-locked', !unlocked);
    this.cardTier.textContent = TIER_LABELS[a.tier];
    this.cardTier.className = `dyson-tier dyson-tier--${a.tier}`;
    this.cardTitle.textContent = a.title;
    this.cardDesc.textContent = a.desc ?? '';
    this.cardBarFill.style.width = `${Math.min(a.current / a.threshold, 1) * 100}%`;
    this.cardCount.textContent = `${a.current}/${a.threshold}`;
    this.cardMeta.textContent = unlocked ? `解锁时间 ${a.unlockedAt ?? '—'}` : '未解锁';
  }

  setTheme(theme: DysonTheme): void {
    this.container.dataset.theme = theme;
  }

  /**
   * 每帧更新浮层位置。
   * @param getScreen 节点在容器内的屏幕坐标
   */
  update(getScreen: (index: number) => ScreenPoint | null, width: number, height: number): void {
    if (this.tooltipIndex >= 0) {
      const s = getScreen(this.tooltipIndex);
      const show = s !== null && s.visible && this.tooltipIndex !== this.cardIndex;
      this.tooltip.style.opacity = show ? '' : '0';
      if (s) {
        const tw = this.tooltip.offsetWidth;
        let x = s.x + 16;
        if (x + tw > width - 6) x = s.x - 16 - tw;
        this.tooltip.style.transform = `translate3d(${Math.round(x)}px, ${Math.round(s.y - 12)}px, 0)`;
      }
    }
    if (this.cardIndex >= 0) {
      const s = getScreen(this.cardIndex);
      if (!s) return;
      const cw = this.card.offsetWidth;
      const ch = this.card.offsetHeight;
      const gap = 26;
      let x = s.x + gap;
      // 右侧放不下时翻到节点左侧
      const flip = x + cw > width - 8;
      if (flip) x = s.x - gap - cw;
      x = Math.max(8, x);
      const y = Math.min(Math.max(s.y - ch * 0.35, 8), Math.max(height - ch - 8, 8));
      this.card.style.transform = `translate3d(${Math.round(x)}px, ${Math.round(y)}px, 0)`;
      this.card.classList.toggle('is-flipped', flip);
      this.marker.style.transform = `translate3d(${Math.round(s.x)}px, ${Math.round(s.y)}px, 0)`;
      this.marker.style.opacity = s.visible ? '' : '0';
    }
  }

  dispose(): void {
    this.root.remove();
    this.sr.remove();
    this.live.remove();
  }
}
