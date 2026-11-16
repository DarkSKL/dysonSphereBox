import type { AlgoVizArea, AlgoVizCoordinates, AlgoVizOperation } from '../types';

/**
 * 操作队列：保存待播放的场景操作及其坐标帧。
 * 实时跟随合并同对象尚未播放的完整状态；回放和电影节奏仍逐项保序，原事件数组不会改写。
 */

export interface Offset {
  readonly x: number;
  readonly y: number;
}

/** 一批操作共享的坐标解释与播放节奏 */
export interface QueueFrame {
  /** 入队时固定任务身份，避免延迟播放的建筑通知被归到新任务。 */
  readonly taskId?: string;
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
  /** 只复制操作引用；follow 的未播旧状态可释放，原始事件保持只读。 */
  private items: Array<OperationBatch | undefined> = [];
  private follow = new Map<string, {batch: OperationBatch; index: number}>();
  private head = 0;
  private pending = 0;

  get length(): number {
    return this.pending;
  }

  push(operations: readonly AlgoVizOperation[], frame: QueueFrame): void {
    if (!operations.length) return;
    const batch: OperationBatch = {operations: [], frame, cursor: 0, started: false};
    if (frame.pace === 'follow') {
      for (const operation of operations) {
        const previous = this.follow.get(operation.id);
        let latest = operation;
        if (previous) {
          const old = previous.batch.operations[previous.index];
          // 尚未显示的对象仍须创建；后续完整姿态覆盖旧位置，不能把 replace 应用到不存在的对象。
          if (old?.type === 'add' && operation.type === 'replace') latest = {type: 'add', id: operation.id, building: operation.building};
          previous.batch.operations[previous.index] = undefined;
          this.pending--;
        }
        const index = batch.operations.push(latest) - 1;
        this.follow.set(operation.id, {batch, index});
      }
    } else {
      // 回放/电影节奏是顺序屏障，不能用之后的实时状态跳过已经请求播放的历史。
      this.follow.clear();
      for (const operation of operations) batch.operations.push(operation);
    }
    this.items.push(batch);
    this.pending += operations.length;
  }

  /** 查看队首而不出队 */
  peek(): QueueItem | undefined {
    while (this.head < this.items.length) {
      const batch = this.items[this.head];
      if (!batch) { this.head++; continue; }
      while (batch.cursor < batch.operations.length) {
        const op = batch.operations[batch.cursor];
        if (op) return {op, frame: batch.frame, first: !batch.started};
        batch.cursor++;
      }
      this.items[this.head++] = undefined;
    }
    return undefined;
  }

  shift(): QueueItem | undefined {
    const item = this.peek();
    if (!item) return undefined;
    const batch = this.items[this.head]!;
    const latest = this.follow.get(item.op.id);
    if (latest?.batch === batch && latest.index === batch.cursor) this.follow.delete(item.op.id);
    batch.operations[batch.cursor] = undefined;
    batch.started = true;
    this.pending--;
    if (++batch.cursor === batch.operations.length) this.items[this.head++] = undefined;
    // 批次播完即释放原数组；整队播完不保留最后几千个建筑引用。
    if (!this.pending) this.clear();
    else if (this.head > 256 && this.head * 2 > this.items.length) {
      this.items = this.items.slice(this.head);
      this.head = 0;
    }
    return item;
  }

  clear(): void {
    this.items = [];
    this.head = 0;
    this.pending = 0;
    this.follow.clear();
  }
}

interface OperationBatch {
  readonly operations: Array<AlgoVizOperation | undefined>;
  readonly frame: QueueFrame;
  cursor: number;
  started: boolean;
}
