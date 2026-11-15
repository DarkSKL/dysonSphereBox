import { buildingKind, isProceduralModel, placeholderSize, resolveModelKey, type BuildingKind } from '../catalog';
import { HEIGHT_SCALE } from '../defaults';
import type { AlgoVizBuilding, AlgoVizIcon } from '../types';
import type { InstancePool } from '../render/pool';
import type { QueueFrame } from './queue';

/** 场景对象的显示姿态（世界坐标，单位蓝图格；yaw 为绕 Y 轴的弧度） */
export interface Pose {
  x: number;
  y: number;
  z: number;
  yaw: number;
  x2: number;
  y2: number;
  z2: number;
}

export type AnimKind = 'enter' | 'move' | 'exit';

/** 场景对象：一个建筑在场景中的完整状态，包括动画进度与实例槽位 */
export interface VizObject {
  id: string;
  building: AlgoVizBuilding;
  kind: BuildingKind;
  modelKey: string | null;
  colorIndex: number;
  pool: InstancePool | null;
  slot: number;
  placeholder: boolean;
  shadowSlot: number;
  /** 图标层的实例槽位，-1 表示没有图标；由 IconSlots 在分配、释放与压实时回写 */
  iconSlot: number;
  /** 已解析的图标；null 表示没有。由场景在创建、替换与 refreshIcons 时写入，逐帧只读 */
  icon: AlgoVizIcon | null;
  /** 占地宽、深与高度（蓝图格），用于投影、涟漪和取景 */
  width: number;
  depth: number;
  height: number;
  target: Pose;
  from: Pose;
  pose: Pose;
  scale: number;
  scaleFrom: number;
  grow: number;
  lift: number;
  drop: number;
  /** 移动时的抬升弧高 */
  arc: number;
  alpha: number;
  alphaFrom: number;
  flash: number;
  ghost: number;
  ghostFrom: number;
  ghostTo: number;
  highlight: number;
  anim: AnimKind | null;
  t: number;
  duration: number;
  delay: number;
  landed: boolean;
  removing: boolean;
  seed: number;
}

const DEG = Math.PI / 180;
const ORIGIN = { x: 0, y: 0 };

export const emptyPose = (): Pose => ({ x: 0, y: 0, z: 0, yaw: 0, x2: 0, y2: 0, z2: 0 });

export const copyPose = (out: Pose, src: Pose): Pose => {
  out.x = src.x;
  out.y = src.y;
  out.z = src.z;
  out.yaw = src.yaw;
  out.x2 = src.x2;
  out.y2 = src.y2;
  out.z2 = src.z2;
  return out;
};

/**
 * 蓝图坐标 → 世界坐标：x 向东为 +X，y 向北为 -Z，z 层高换算为格。
 * 朝向：游戏 yaw 0 指北、顺时针；模型前方为 +Z，因此旋转角为 π - yaw。
 */
export function worldPose(out: Pose, building: AlgoVizBuilding, frame: QueueFrame): Pose {
  const offset = frame.coordinates === 'area' ? frame.offsets.get(building.areaIndex ?? 0) ?? ORIGIN : ORIGIN;
  out.x = building.x + offset.x;
  out.y = building.z * HEIGHT_SCALE;
  out.z = -(building.y + offset.y);
  out.yaw = Math.PI - (building.yaw ?? 0) * DEG;
  out.x2 = (building.x2 ?? building.x) + offset.x;
  out.y2 = (building.z2 ?? building.z) * HEIGHT_SCALE;
  out.z2 = -((building.y2 ?? building.y) + offset.y);
  return out;
}

/** 颜色索引：只有程序化绘制的传送带使用，返回等级 0~2；建筑一律使用模型原生贴图颜色 */
export function colorIndexFor(building: AlgoVizBuilding, kind: BuildingKind): number {
  return kind === 'belt' ? Math.min(2, Math.max(0, building.itemId - 2001)) : 0;
}

/** 是否投射地面阴影：扁平的传送带与分拣器不投影 */
export const castsShadow = (kind: BuildingKind): boolean => kind !== 'belt' && kind !== 'sorter';

/** 入场下落高度：大型设施落得更高，显得更有分量 */
export function dropHeight(kind: BuildingKind, seed: number): number {
  switch (kind) {
    case 'belt': return 0;
    case 'sorter': return 1.4;
    case 'station': return 9;
    case 'splitter': return 2.5;
    default: return 4.5 + seed * 1.5;
  }
}

/** 建立对象的静态描述（不含槽位与动画） */
export function describeObject(id: string, building: AlgoVizBuilding, seed: number): Pick<VizObject, 'id' | 'building' | 'kind' | 'modelKey' | 'colorIndex' | 'width' | 'depth' | 'height' | 'seed'> {
  const kind = buildingKind(building.itemId);
  const key = resolveModelKey(building.itemId, building.modelIndex);
  const [width, depth, height] = placeholderSize(building.itemId);
  return {
    id,
    building,
    kind,
    modelKey: kind === 'belt' || isProceduralModel(key) ? null : key,
    colorIndex: colorIndexFor(building, kind),
    width,
    depth,
    height,
    seed,
  };
}
