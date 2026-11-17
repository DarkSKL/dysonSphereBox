import type { VizObject } from './objects';

// =============== 候选局部占地索引 ===============
// 说明：按对象身份维护分块包络，建筑增删、目标移动或模型换型时才更新对应块。
// 边界：只描述当前身份表中的目标姿态；不包含退场旧对象，也不把平台平移计入局部范围。
const BLOCK_SIZE = 128;

export interface CandidateRange {
  readonly minX: number;
  readonly minZ: number;
  readonly maxX: number;
  readonly maxZ: number;
}

/** 大厂每批通常只改变部分建筑；取景复用未变块，避免反复扫描几十万实体。 */
export class CandidateBounds {
  private readonly slots = new Map<VizObject, number>();
  private readonly objects: (VizObject | undefined)[] = [];
  private readonly free: number[] = [];
  private readonly blocks: (CandidateRange | undefined)[] = [];
  private readonly dirty = new Set<number>();
  private range: CandidateRange | undefined;

  /** 新对象复用空槽；已有对象仅使所属块失效，不改变稳定身份。 */
  public update(obj: VizObject): void {
    let slot = this.slots.get(obj);
    if (slot === undefined) {
      slot = this.free.pop() ?? this.objects.length;
      this.slots.set(obj, slot);
      this.objects[slot] = obj;
    }
    this.dirty.add(Math.floor(slot / BLOCK_SIZE));
  }

  /** 撤销立即移除占地，退场动画仍由场景自己的成员表维护。 */
  public delete(obj: VizObject): void {
    const slot = this.slots.get(obj);
    if (slot === undefined) return;
    this.slots.delete(obj);
    this.objects[slot] = undefined;
    this.free.push(slot);
    this.dirty.add(Math.floor(slot / BLOCK_SIZE));
  }

  /** 只重算变化块，再归并块范围；无变化时直接返回只读缓存。 */
  public get(): CandidateRange | undefined {
    if (!this.dirty.size) return this.range;
    for (const block of this.dirty) this.blocks[block] = this.measure(block);
    this.dirty.clear();
    let minX = Infinity, minZ = Infinity, maxX = -Infinity, maxZ = -Infinity;
    for (const block of this.blocks) {
      if (!block) continue;
      minX = Math.min(minX, block.minX); minZ = Math.min(minZ, block.minZ);
      maxX = Math.max(maxX, block.maxX); maxZ = Math.max(maxZ, block.maxZ);
    }
    this.range = Number.isFinite(minX) ? { minX, minZ, maxX, maxZ } : undefined;
    return this.range;
  }

  private measure(block: number): CandidateRange | undefined {
    let minX = Infinity, minZ = Infinity, maxX = -Infinity, maxZ = -Infinity;
    const end = Math.min(this.objects.length, (block + 1) * BLOCK_SIZE);
    for (let slot = block * BLOCK_SIZE; slot < end; slot++) {
      const obj = this.objects[slot];
      if (!obj) continue;
      const t = obj.target, hw = obj.width / 2, hd = obj.depth / 2;
      minX = Math.min(minX, t.x - hw, t.x2 - hw);
      maxX = Math.max(maxX, t.x + hw, t.x2 + hw);
      minZ = Math.min(minZ, t.z - hd, t.z2 - hd);
      maxZ = Math.max(maxZ, t.z + hd, t.z2 + hd);
    }
    return Number.isFinite(minX) ? { minX, minZ, maxX, maxZ } : undefined;
  }
}
