import type { AlgoVizQuality } from './types';

/** 蓝图平面格长（米）：与生成器 planarGridLength 一致，200.2 × 2π / 1000 */
export const GRID_METERS = (200.2 * 2 * Math.PI) / 1000;

/** 高度层高（米）：与生成器 1.3333333 一致 */
export const LAYER_METERS = 1.3333333;

/** 世界坐标以蓝图格为单位；z 层高换算为格 */
export const HEIGHT_SCALE = LAYER_METERS / GRID_METERS;

/** 播放速度范围 */
export const SPEED_MIN = 0.25;
export const SPEED_MAX = 8;

/** 场景对象默认上限 */
export const DEFAULT_MAX_OBJECTS = 12000;

/** 画质档位：像素比上限与抗锯齿 */
export interface AlgoVizQualityPreset {
  readonly maxDpr: number;
  readonly antialias: boolean;
}

export const QUALITY_PRESETS: Readonly<Record<AlgoVizQuality, AlgoVizQualityPreset>> = {
  low: { maxDpr: 1, antialias: false },
  mid: { maxDpr: 1.5, antialias: true },
  high: { maxDpr: 2, antialias: true },
};
