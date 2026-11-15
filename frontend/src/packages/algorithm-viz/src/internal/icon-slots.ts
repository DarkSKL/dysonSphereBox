/**
 * 图标槽位簿记与精灵图坐标换算（纯逻辑，不依赖 three，便于在 Node 下回归）。
 *
 * - 图标层的实例池在释放时用末端实例补位，簿记负责把“槽号 → 持有对象”的映射同步迁移
 * - 精灵图按行列紧密排布，坐标原点在左上角；贴图以 flipY 上传，故 v 轴需要翻转
 */

/** 持有图标槽位的对象：槽号由簿记在分配、释放与压实时回写，-1 表示没有图标 */
export interface IconHolder {
  iconSlot: number;
}

/** 实例池的最小契约：release 返回补位实例原来的槽号，释放末槽或无效槽时返回 undefined */
export interface IconSlotPool {
  readonly size: number;
  alloc(): number;
  release(slot: number): number | undefined;
}

export class IconSlots<T extends IconHolder> {
  private readonly owners: (T | undefined)[] = [];
  private readonly pool: IconSlotPool;

  // 不使用参数属性：包内回归测试用 Node 的类型擦除直接运行，参数属性需要代码生成，无法被擦除
  constructor(pool: IconSlotPool) {
    this.pool = pool;
  }

  /** 为对象分配槽位；已持有槽位时保持不变 */
  alloc(owner: T): number {
    if (owner.iconSlot >= 0) return owner.iconSlot;
    const slot = this.pool.alloc();
    owner.iconSlot = slot;
    this.owners[slot] = owner;
    return slot;
  }

  /** 释放对象的槽位，并把被补位的实例迁移到空出的槽号 */
  release(owner: T): void {
    const slot = owner.iconSlot;
    if (slot < 0) return;
    if (this.owners[slot] === owner) this.owners[slot] = undefined;
    const from = this.pool.release(slot);
    if (from !== undefined) {
      const moved = this.owners[from];
      if (moved) {
        moved.iconSlot = slot;
        this.owners[slot] = moved;
      }
    }
    this.owners.length = this.pool.size;
    owner.iconSlot = -1;
  }

  /** 整池归零前调用：全部持有者回到无图标状态 */
  reset(): void {
    for (const owner of this.owners) if (owner) owner.iconSlot = -1;
    this.owners.length = 0;
  }
}

/** 精灵图规格：只取换算所需的字段 */
export interface IconSheetSize {
  readonly width: number;
  readonly height: number;
  readonly cell: number;
}

/**
 * 一格图标在贴图中的 UV 矩形 [u0, v0, du, dv]。
 * 四周各内缩 inset 像素，避免线性过滤与 mipmap 采到相邻格的颜色。
 * 传入 out 时原地写入并返回它：逐帧写入的调用方复用同一数组，避免每帧为每个图标分配。
 */
export function iconRect(sheet: IconSheetSize, icon: { readonly x: number; readonly y: number }, inset = 0.5, out: number[] = [0, 0, 0, 0]): number[] {
  const size = sheet.cell - inset * 2;
  out[0] = (icon.x + inset) / sheet.width;
  out[1] = 1 - (icon.y + sheet.cell - inset) / sheet.height;
  out[2] = size / sheet.width;
  out[3] = size / sheet.height;
  return out;
}
