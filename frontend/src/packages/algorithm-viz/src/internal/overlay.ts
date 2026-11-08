/**
 * 覆盖层 DOM：阶段标题（纪念碑谷章节式）、悬停提示、空场景说明与 WebGL 降级提示。
 * 全部元素插入容器内，pointer-events 关闭，不拦截画布交互。
 */

const CAPTION_HOLD_MS = 2600;

export class Overlay {
  readonly root: HTMLDivElement;
  private readonly caption: HTMLDivElement;
  private readonly captionTitle: HTMLDivElement;
  private readonly captionDetail: HTMLDivElement;
  private readonly tooltip: HTMLDivElement;
  private readonly empty: HTMLDivElement;
  private captionTimer = 0;
  private lastCaption = '';

  constructor(container: HTMLElement, emptyText: string) {
    this.root = document.createElement('div');
    this.root.className = 'algoviz-overlay';

    this.caption = document.createElement('div');
    this.caption.className = 'algoviz-caption';
    this.caption.setAttribute('role', 'status');
    this.caption.setAttribute('aria-live', 'polite');
    const ornament = document.createElement('i');
    ornament.className = 'algoviz-caption-mark';
    this.captionTitle = document.createElement('div');
    this.captionTitle.className = 'algoviz-caption-title';
    this.captionDetail = document.createElement('div');
    this.captionDetail.className = 'algoviz-caption-detail';
    this.caption.append(ornament, this.captionTitle, this.captionDetail);

    this.tooltip = document.createElement('div');
    this.tooltip.className = 'algoviz-tooltip';

    this.empty = document.createElement('div');
    this.empty.className = 'algoviz-empty';
    this.empty.textContent = emptyText;

    this.root.append(this.caption, this.tooltip, this.empty);
    container.append(this.root);
  }

  /** 显示阶段标题；与当前标题相同时只刷新停留时间 */
  showCaption(title: string, detail = ''): void {
    const key = `${title}\n${detail}`;
    window.clearTimeout(this.captionTimer);
    if (key !== this.lastCaption) {
      this.lastCaption = key;
      this.captionTitle.textContent = title;
      this.captionDetail.textContent = detail;
      this.captionDetail.hidden = !detail;
      // 重新触发入场动画
      this.caption.classList.remove('is-visible');
      void this.caption.offsetWidth;
    }
    this.caption.classList.add('is-visible');
    this.captionTimer = window.setTimeout(() => {
      this.caption.classList.remove('is-visible');
      this.lastCaption = '';
    }, CAPTION_HOLD_MS);
  }

  showTooltip(text: string | null, x = 0, y = 0): void {
    if (!text) {
      this.tooltip.classList.remove('is-visible');
      return;
    }
    if (this.tooltip.textContent !== text) this.tooltip.textContent = text;
    this.tooltip.style.transform = `translate(${Math.round(x + 14)}px, ${Math.round(y + 14)}px)`;
    this.tooltip.classList.add('is-visible');
  }

  setEmpty(visible: boolean): void {
    this.empty.classList.toggle('is-visible', visible);
  }

  setEmptyText(text: string): void {
    this.empty.textContent = text;
  }

  dispose(): void {
    window.clearTimeout(this.captionTimer);
    this.root.remove();
  }
}
