import type { DysonInteraction, DysonQuality, DysonQualityPreset, DysonSphereParams } from './types';

/** 画质档位：像素比上限、默认粒子数、是否开启泛光、MSAA 采样数 */
export const QUALITY_PRESETS: Readonly<Record<DysonQuality, Readonly<DysonQualityPreset>>> = Object.freeze({
  low: Object.freeze({ maxDpr: 1, particleCount: 5000, bloom: false, samples: 0 }),
  mid: Object.freeze({ maxDpr: 1.5, particleCount: 15000, bloom: true, samples: 4 }),
  high: Object.freeze({ maxDpr: 2, particleCount: 30000, bloom: true, samples: 4 }),
});

/** 默认场景参数 */
export const DEFAULT_PARAMS: Readonly<DysonSphereParams> = Object.freeze<DysonSphereParams>({
  quality: 'mid',
  progress: 0.72,
  shellLevel: 3,
  particleCount: 15000,
  orbitLayers: 6,
  spinSpeed: 1,
  attractStrength: 0.8,
  attractRadius: 1.4,
  returnSpeed: 2.5,
  bloomStrength: 0.7,
  bloomThreshold: 0.6,
  starColor: '#ffc46b',
  starPulse: 0.5,
  starCount: 1800,
  nebulaIntensity: 1,
});

/** 默认交互开关：全部开启 */
export const DEFAULT_INTERACTION: Readonly<DysonInteraction> = Object.freeze<DysonInteraction>({
  wheelZoom: true,
  orbitDrag: true,
  pointerAttract: true,
});
