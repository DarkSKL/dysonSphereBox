import * as THREE from 'three';
import type { DysonTheme } from '../types';

/**
 * 日 / 夜两套调色板。
 * 颜色字段为十六进制字符串，数值字段直接插值。
 * over：混合模式系数，0 = 叠加（夜间发光感），1 = 普通透明叠放（日间浅色背景下保持可读）。
 */
const DARK = {
  background: '#03060d',
  nebulaA: '#16305e',
  nebulaB: '#4b1d52',
  nebulaAlpha: 0.9,
  starPoint: '#d6e6ff',
  starPointAlpha: 1.0,
  starGlow: 1.0,
  shellFrame: '#3a9cff', // 框架梁
  shellFrameHot: '#dff4ff', // 框架梁生长端头 / 能量脉冲 / 壳板落位闪光
  shellNode: '#9fe2ff',
  shellCell: '#2f8cff', // 壳板内部纹路 / 内表面受光
  shellCellEdge: '#5fe0ff', // 壳板折边（青色）
  shellPanel: '#061526', // 壳板外表面（深蓝）
  shellRim: '#5fb4ff', // 壳板边缘反光 / 壳层辉光
  shellLattice: '#6f8dff', // 外层能量网格 / 框架第二色
  shellLatticeHot: '#e6ecff',
  shellAlpha: 1.0,
  shellEmit: 1.0,
  shellCover: 1.0,
  swarm: '#63b0ff',
  swarmHot: '#e2f2ff',
  swarmAlpha: 0.5,
  orbitLine: '#3f8fe0',
  orbitLineAlpha: 0.55,
  trailAlpha: 0.85, // 成就粒子拖尾整体亮度
  trailTail: '#7a8cff', // 拖尾尾端渐变色
  trailCore: '#fff3dc', // 拖尾亮芯
  tierBronze: '#e58b4e',
  tierSilver: '#c9d6ea',
  tierGold: '#ffc93a',
  locked: '#3e4d63',
  ring: '#5aaefc',
  ringTrack: '#1e3550',
  bloomScale: 1.0,
  bloomThreshold: 0.72,
  over: 0.0,
};

type PaletteSource = typeof DARK;

// 日间：白灰黑无彩色，与应用日间主题（style.scss 中 --ds-bg-shell: #ebebeb）保持一致，不使用蓝色
const LIGHT: PaletteSource = {
  background: '#ebebeb',
  nebulaA: '#c4c4c8',
  nebulaB: '#f7f7f7',
  nebulaAlpha: 0.75,
  starPoint: '#5a5a5e',
  starPointAlpha: 0.5,
  starGlow: 0.7,
  shellFrame: '#3a3a3e',
  shellFrameHot: '#111113',
  shellNode: '#2a2a2e',
  shellCell: '#55555a',
  shellCellEdge: '#3d3d42',
  shellPanel: '#c9c9cd',
  shellRim: '#4a4a4f',
  shellLattice: '#5c5c62',
  shellLatticeHot: '#1c1c1e',
  shellAlpha: 0.95,
  shellEmit: 0.45,
  shellCover: 0.85,
  swarm: '#48484d',
  swarmHot: '#18181a',
  swarmAlpha: 0.7,
  orbitLine: '#6a6a70',
  orbitLineAlpha: 0.6,
  trailAlpha: 1.0,
  // 日间浅色背景：亮芯改为深色，保证拖尾可读
  trailTail: '#6a6a70',
  trailCore: '#1c1c1e',
  tierBronze: '#94682a',
  tierSilver: '#7a7a80',
  tierGold: '#a88418',
  locked: '#b0b0b4',
  ring: '#3a3a3e',
  ringTrack: '#cfcfd2',
  bloomScale: 0.35,
  bloomThreshold: 0.92,
  over: 1.0,
};

export const PALETTES: Readonly<Record<DysonTheme, PaletteSource>> = { dark: DARK, light: LIGHT };

/** 运行时调色板：颜色字段解析为 THREE.Color，数值字段保持 number */
export type Palette = { [K in keyof PaletteSource]: PaletteSource[K] extends string ? THREE.Color : number };

type LoosePalette = Record<string, THREE.Color | number>;

/** 把调色板解析为运行时对象 */
export function resolvePalette(src: PaletteSource): Palette {
  const out: LoosePalette = {};
  for (const [key, value] of Object.entries(src)) {
    out[key] = typeof value === 'string' ? new THREE.Color(value) : value;
  }
  return out as unknown as Palette;
}

/** 复制一份运行时调色板 */
export function clonePalette(p: Palette): Palette {
  const out: LoosePalette = {};
  for (const [key, value] of Object.entries(p as unknown as LoosePalette)) {
    out[key] = value instanceof THREE.Color ? value.clone() : value;
  }
  return out as unknown as Palette;
}

/** out = lerp(a, b, t)，颜色在线性空间插值 */
export function lerpPalette(out: Palette, a: Palette, b: Palette, t: number): void {
  const o = out as unknown as LoosePalette;
  const pa = a as unknown as LoosePalette;
  const pb = b as unknown as LoosePalette;
  for (const key of Object.keys(o)) {
    const va = pa[key];
    const vb = pb[key];
    const target = o[key];
    if (target instanceof THREE.Color && va instanceof THREE.Color && vb instanceof THREE.Color) {
      target.copy(va).lerp(vb, t);
    } else if (typeof va === 'number' && typeof vb === 'number') {
      o[key] = va + (vb - va) * t;
    }
  }
}
