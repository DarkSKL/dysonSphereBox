import { QUALITY_PRESETS } from '../defaults';
import type { DysonInteraction, DysonQuality, DysonSphereParams } from '../types';

type NumericKey = { [K in keyof DysonSphereParams]: DysonSphereParams[K] extends number ? K : never }[keyof DysonSphereParams];

/** 数值参数的合法范围；int 为 true 时四舍五入为整数 */
const NUMERIC_RANGES: Record<NumericKey, [min: number, max: number, int?: boolean]> = {
  progress: [0, 1],
  shellLevel: [1, 5, true],
  particleCount: [0, 100000, true],
  orbitLayers: [1, 12, true],
  spinSpeed: [0, 10],
  attractStrength: [0, 3],
  attractRadius: [0.1, 10],
  returnSpeed: [0.1, 20],
  bloomStrength: [0, 5],
  bloomThreshold: [0, 1],
  starPulse: [0, 1],
  starCount: [0, 20000, true],
  nebulaIntensity: [0, 3],
};

function isQuality(v: unknown): v is DysonQuality {
  return typeof v === 'string' && Object.prototype.hasOwnProperty.call(QUALITY_PRESETS, v);
}

/**
 * 合并参数补丁：忽略未知键与非法值，数值裁剪到合法范围。
 * 切换画质且补丁未指定 particleCount 时，粒子数重置为该档位默认值。
 */
export function mergeParams(prev: Readonly<DysonSphereParams>, patch: Partial<DysonSphereParams>): DysonSphereParams {
  const next: DysonSphereParams = { ...prev };
  if (!patch) return next;

  if (patch.quality !== undefined) {
    if (isQuality(patch.quality)) next.quality = patch.quality;
    else console.warn(`[dyson-sphere] 忽略无效的画质档位：${String(patch.quality)}`);
  }
  if (typeof patch.starColor === 'string') next.starColor = patch.starColor;

  const nums = next as Record<NumericKey, number>;
  for (const key of Object.keys(NUMERIC_RANGES) as NumericKey[]) {
    const v = patch[key];
    if (v === undefined) continue;
    if (typeof v !== 'number' || !Number.isFinite(v)) {
      console.warn(`[dyson-sphere] 忽略无效的参数 ${key}：${String(v)}`);
      continue;
    }
    const [min, max, int] = NUMERIC_RANGES[key];
    nums[key] = Math.min(max, Math.max(min, int ? Math.round(v) : v));
  }

  if (next.quality !== prev.quality && patch.particleCount === undefined) {
    next.particleCount = QUALITY_PRESETS[next.quality].particleCount;
  }
  return next;
}

/** 合并交互开关补丁，只接受布尔值 */
export function mergeInteraction(prev: Readonly<DysonInteraction>, patch: Partial<DysonInteraction>): DysonInteraction {
  const next: DysonInteraction = { ...prev };
  if (!patch) return next;
  for (const key of Object.keys(prev) as (keyof DysonInteraction)[]) {
    const v = patch[key];
    if (typeof v === 'boolean') next[key] = v;
  }
  return next;
}
