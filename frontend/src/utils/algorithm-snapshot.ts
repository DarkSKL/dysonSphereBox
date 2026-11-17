import type { AlgoVizArea, AlgoVizBuilding, AlgoVizOperation } from '../packages/algorithm-viz/src';

// 完整字段表由类型约束：新增渲染字段必须明确加入比较，不能误判为相同快照。
const fields: Record<keyof AlgoVizBuilding, true> = {
  itemId: true, modelIndex: true, x: true, y: true, z: true, x2: true, y2: true, z2: true,
  yaw: true, areaIndex: true, group: true, temporary: true, outputId: true, index: true,
  recipeId: true, iconItemId: true,
};
const buildingKeys = Object.keys(fields) as (keyof AlgoVizBuilding)[];

/** 只认逐字段完全相同；不使用浮点容差或可能碰撞的摘要跳过正式状态。 */
export function sameSnapshotBuilding(left: AlgoVizBuilding, right: AlgoVizBuilding): boolean {
  return buildingKeys.every(key => left[key] === right[key]);
}

/** 区域坐标解释也参与身份；即使建筑局部坐标相同，区域移动仍需重新应用。 */
export function sameSnapshotAreas(left?: readonly AlgoVizArea[], right?: readonly AlgoVizArea[]): boolean {
  if (left === right) return true;
  if (!left || !right || left.length !== right.length) return false;
  return left.every((area, index) => {
    const other = right[index];
    return area.index === other.index && area.parentIndex === other.parentIndex
      && area.anchorX === other.anchorX && area.anchorY === other.anchorY
      && area.width === other.width && area.height === other.height;
  });
}

/** 分块到达时校验身份并比较镜像，最终确认不再同步转换、索引整图。 */
export class SnapshotContents {
  public readonly chunks = new Map<number, readonly AlgoVizOperation[]>();
  private readonly ids = new Set<string>();

  public constructor(public readonly revision: number, public readonly count: number,
    public readonly areas: readonly AlgoVizArea[] | undefined, public unchanged: boolean) {}

  public get size(): number { return this.ids.size; }

  /** 拒绝重复块/对象；失败方必须丢弃整个快照，不发布已接收前缀。 */
  public add(index: number, operations: readonly AlgoVizOperation[], mirror: ReadonlyMap<string, {readonly building: AlgoVizBuilding}>): boolean {
    if (!Number.isInteger(index) || index < 0 || index >= this.count || this.chunks.has(index)) return false;
    for (const op of operations) {
      if (this.ids.has(op.id)) return false;
      this.ids.add(op.id);
      const before = this.unchanged ? mirror.get(op.id)?.building : undefined;
      if (!before || op.type === 'remove' || !sameSnapshotBuilding(before, op.building)) this.unchanged = false;
    }
    this.chunks.set(index, operations);
    return true;
  }

  /** 完整且确实改变时才展平；块可能乱序到达，应用仍按原编号顺序。 */
  public operations(): AlgoVizOperation[] {
    const out: AlgoVizOperation[] = [];
    for (let index = 0; index < this.count; index++) for (const op of this.chunks.get(index) ?? []) out.push(op);
    return out;
  }
}
