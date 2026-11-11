import type { AlgoVizOperation } from '../types';
import type { VizObject } from './objects';
import type { QueueFrame, QueueItem } from './queue';
import type { FrameWorkBudget } from './work-budget';

/** 最终快照的分帧游标；使用：候选过程播完后先索引、再撤销、最后应用；边界：只调度真实操作，不能丢掉缺失对象的撤销或改变快照顺序。 */
export class CandidateSnapshot {
  private readonly present = new Set<string>();
  private phase: 'index' | 'remove' | 'apply' | 'done' = 'index';
  private cursor = 0;
  private iterator?: IterableIterator<VizObject>;

  constructor(readonly operations: readonly AlgoVizOperation[], readonly frame: QueueFrame) {}

  get done(): boolean { return this.phase === 'done'; }
  get started(): boolean { return this.phase !== 'index' || this.cursor > 0; }
  get pending(): number { return this.done ? 0 : Math.max(1, this.operations.length - (this.phase === 'apply' ? this.cursor : 0)); }

  /** 索引与对象扫描也计入预算；移除迭代期间允许 apply 删除当前集合元素。 */
  process(objects: ReadonlySet<VizObject>, apply: (item: QueueItem) => void, work: FrameWorkBudget, quota = 128): void {
    for (let count = 0; count < quota && !this.done && work.take(); count++) {
      if (this.phase === 'index') {
        if (this.cursor < this.operations.length) this.present.add(this.operations[this.cursor++].id);
        else { this.phase = 'remove'; this.cursor = 0; this.iterator = objects.values(); }
      } else if (this.phase === 'remove') {
        const next = this.iterator!.next();
        if (next.done) { this.phase = 'apply'; this.iterator = undefined; }
        else if (!next.value.removing && !this.present.has(next.value.id)) apply({op: {type: 'remove', id: next.value.id}, frame: this.frame, first: false});
      } else if (this.cursor < this.operations.length) {
        apply({op: this.operations[this.cursor++], frame: this.frame, first: false});
      } else { this.phase = 'done'; this.present.clear(); }
    }
  }
}
