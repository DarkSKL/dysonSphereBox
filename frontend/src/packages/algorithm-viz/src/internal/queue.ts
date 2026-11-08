import type { AlgoVizArea, AlgoVizCoordinates, AlgoVizOperation } from '../types';

/**
 * 操作队列：保存待播放的场景操作及其坐标帧。
 * 播放节奏由场景按帧预算出队，队列本身只负责先进先出与区域偏移计算。
 */

export interface Offset {
  readonly x: number;
  readonly y: number;
}

/** 一批操作共享的坐标解释与播放节奏 */
export interface QueueFrame {
  readonly candidateId?: string;
  readonly coordinates: AlgoVizCoordinates;
  /** 区域下标 → 区域原点在整体平面中的偏移（蓝图格） */
  readonly offsets: ReadonlyMap<number, Offset>;
  readonly stage: string;
  readonly pace: 'follow' | 'replay' | 'cinematic';
}

export interface QueueItem {
  readonly op: AlgoVizOperation;
  readonly frame: QueueFrame;
  /** 是否为所属批次的第一项（用于在播放到该批时显示阶段标题） */
  readonly first: boolean;
}

/**
 * 把嵌套区域的锚点沿父链累加为整体偏移。
 * 子区域锚点按父区域局部坐标解释；出现环或缺失父区域时按根区域处理。
 */
export function areaOffsets(areas: readonly AlgoVizArea[] | undefined): Map<number, Offset> {
  const result = new Map<number, Offset>();
  if (!areas || !areas.length) return result;
  const byIndex = new Map(areas.map((area) => [area.index, area]));
  const resolve = (index: number, depth: number): Offset => {
    const cached = result.get(index);
    if (cached) return cached;
    const area = byIndex.get(index);
    if (!area || depth > areas.length) return { x: 0, y: 0 };
    const parent = area.parentIndex >= 0 && area.parentIndex !== index && byIndex.has(area.parentIndex)
      ? resolve(area.parentIndex, depth + 1)
      : { x: 0, y: 0 };
    const offset = area.parentIndex >= 0 && byIndex.has(area.parentIndex)
      ? { x: parent.x + area.anchorX, y: parent.y + area.anchorY }
      : { x: 0, y: 0 };
    result.set(index, offset);
    return offset;
  };
  for (const area of areas) resolve(area.index, 0);
  return result;
}

export class OperationQueue {
  private items: QueueItem[] = [];
  private head = 0;

  get length(): number {
    return this.items.length - this.head;
  }

  push(operations: readonly AlgoVizOperation[], frame: QueueFrame): void {
    operations.forEach((op, i) => this.items.push({ op, frame, first: i === 0 }));
  }

  /** 查看队首而不出队 */
  peek(): QueueItem | undefined {
    return this.head < this.items.length ? this.items[this.head] : undefined;
  }

  shift(): QueueItem | undefined {
    if (this.head >= this.items.length) return undefined;
    const item = this.items[this.head++];
    // 已出队部分过多时压缩数组，避免长时间运行占用内存
    if (this.head > 2048 && this.head * 2 > this.items.length) {
      this.items = this.items.slice(this.head);
      this.head = 0;
    }
    return item;
  }

  clear(): void {
    this.items = [];
    this.head = 0;
  }
}
