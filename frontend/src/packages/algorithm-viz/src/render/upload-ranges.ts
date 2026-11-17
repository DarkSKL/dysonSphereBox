// =============== 有界稀疏上传范围 ===============
// 说明：记录标量分块，连续块合并；覆盖密集或过度碎片化时退回单范围。
// 边界：只规划上传，不修改实例内容；尚未被renderer消费的旧范围必须一并保留。

/** 单缓冲最多十六次局部上传，避免细碎更新放大driver调用成本。 */
export const MAX_UPLOAD_RANGES = 16;
/** 标记块覆盖跨度达到六成时，直接使用精确首尾范围。 */
const DENSE_BLOCK_RATIO = 0.6;

interface UploadRange {
  readonly start: number;
  readonly count: number;
}

/** 与Three BufferAttribute的最小范围契约；单位为数组标量分量，不是字节或实例。 */
interface UploadTarget {
  readonly updateRanges: readonly UploadRange[];
  addUpdateRange(start: number, count: number): void;
  clearUpdateRanges(): void;
}

/** 可复用的脏块簿记；使用：每个矩阵/属性缓冲各持有一个；边界：分块只允许多传，不能漏掉变更。 */
export class UploadRanges {
  private readonly blocks = new Set<number>();
  /** 连续写同一块时跳过重复Set操作，降低密集装配的标记成本。 */
  private lastBlock = -1;
  low = Infinity;
  high = -Infinity;

  constructor(private readonly blockSize: number) {}

  /** 记录半开标量范围，同块重复标记只保留一个块号。 */
  mark(start: number, count: number): void {
    if (count <= 0) return;
    const end = start + count;
    this.low = Math.min(this.low, start);
    this.high = Math.max(this.high, end);
    const first = Math.floor(start / this.blockSize);
    const last = Math.floor((end - 1) / this.blockSize);
    if (first === last && first === this.lastBlock) return;
    for (let block = first; block <= last; block++) this.blocks.add(block);
    this.lastBlock = last;
  }

  /** 清空本次CPU标记；不触碰BufferAttribute中仍待GPU消费的范围。 */
  clear(): void {
    this.blocks.clear();
    this.lastBlock = -1;
    this.low = Infinity;
    this.high = -Infinity;
  }

  /** 合并尚未上传的旧范围，重建有界列表；返回本次实际发布的范围数。 */
  flush(target: UploadTarget, activeComponents: number): number {
    if (!this.blocks.size) return 0;
    for (const range of target.updateRanges) {
      const start = Math.max(0, range.start);
      const end = Math.min(activeComponents, range.start + range.count);
      if (end > start) this.mark(start, end - start);
    }
    const ranges = this.plan(activeComponents);
    this.clear();
    if (!ranges.length) return 0;
    target.clearUpdateRanges();
    for (const range of ranges) target.addUpdateRange(range.start, range.count);
    return ranges.length;
  }

  /** 首尾精确裁到活动缓冲；中间块可多传，但不会超过旧min/max跨度。 */
  private plan(activeComponents: number): UploadRange[] {
    const low = Math.max(0, this.low);
    const high = Math.min(activeComponents, this.high);
    if (high <= low) return [];
    const whole = { start: low, count: high - low };
    const first = Math.floor(low / this.blockSize);
    const last = Math.floor((high - 1) / this.blockSize);
    if (this.blocks.size >= (last - first + 1) * DENSE_BLOCK_RATIO) return [whole];

    const sorted = Array.from(this.blocks).filter(block => block >= first && block <= last).sort((a, b) => a - b);
    const ranges: UploadRange[] = [];
    let begin = -1;
    let end = -1;
    const append = (): void => {
      if (begin < 0) return;
      const start = Math.max(low, begin * this.blockSize);
      const stop = Math.min(high, (end + 1) * this.blockSize);
      ranges.push({ start, count: stop - start });
    };
    for (const block of sorted) {
      if (begin < 0) { begin = end = block; continue; }
      if (block === end + 1) { end = block; continue; }
      append();
      if (ranges.length >= MAX_UPLOAD_RANGES) return [whole];
      begin = end = block;
    }
    append();
    return ranges.length > MAX_UPLOAD_RANGES ? [whole] : ranges;
  }
}
