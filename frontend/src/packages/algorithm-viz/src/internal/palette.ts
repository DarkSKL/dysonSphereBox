import * as THREE from 'three';
import type { AlgoVizTheme } from '../types';

/**
 * 戴森球计划风格的太空科幻调色板（沿用纪念碑谷式的几何构图与三面着色）。
 *
 * - 建筑一律使用游戏原生贴图颜色，调色板只负责环境：天空、平台、光照、全息投影与状态高亮
 * - 夜间：深空背景、星云、恒星与行星；暗色平台叠加青色建造网格
 * - 日间：行星白昼大气，浅色平台与蓝色建造网格
 * - 传送带为程序化绘制，按游戏惯例分级着色：Mk.I 琥珀、Mk.II 绿、Mk.III 蓝
 */

interface PaletteSource {
  skyTop: string;
  skyBottom: string;
  nebulaA: string;
  nebulaB: string;
  star: string;
  planet: string;
  atmosphere: string;
  haze: string;
  tileA: string;
  tileB: string;
  grid: string;
  rim: string;
  cliff: string;
  steel: string;
  shadow: string;
  light: string;
  belt: string;
  ghost: string;
  flash: string;
  select: string;
  tiers: readonly [string, string, string];
  /** x 自发光强度、y 星空强度、z 投影不透明度、w 保留 */
  params: readonly [number, number, number, number];
}

const SOURCES: Readonly<Record<AlgoVizTheme, PaletteSource>> = {
  // 夜间：深空。暗色平台与青色建造网格
  dark: {
    skyTop: '#01030a',
    skyBottom: '#0a1430',
    nebulaA: '#33256f',
    nebulaB: '#0d5679',
    star: '#ffdca0',
    planet: '#10233f',
    atmosphere: '#4cc2ff',
    haze: '#050b1c',
    tileA: '#131b2c',
    tileB: '#0f1626',
    grid: '#2f8fd6',
    rim: '#4cc2ff',
    cliff: '#182038',
    steel: '#8b97ab',
    shadow: '#05070f',
    light: '#f2f7ff',
    belt: '#1a2336',
    ghost: '#4cb8ff',
    flash: '#d4f6ff',
    select: '#ff9d3c',
    tiers: ['#f4aa3c', '#3fd38f', '#3ea8ff'],
    params: [1, 1, 0.55, 0],
  },
  // 日间：行星白昼。浅色平台、蓝色建造网格
  light: {
    skyTop: '#dfe8f4',
    skyBottom: '#b3c2d8',
    nebulaA: '#cfc8ea',
    nebulaB: '#b9d9ec',
    star: '#ffffff',
    planet: '#95a9c6',
    atmosphere: '#eef8ff',
    haze: '#bfcbdc',
    tileA: '#e9eef5',
    tileB: '#dfe6ef',
    grid: '#7fa9d6',
    rim: '#3f97dc',
    cliff: '#c2ccdb',
    steel: '#c7cfdb',
    shadow: '#4c5a72',
    light: '#ffffff',
    belt: '#2b3446',
    ghost: '#2f8fd8',
    flash: '#ffffff',
    select: '#f08a24',
    tiers: ['#e09a2c', '#26ad78', '#2b86d6'],
    params: [0.55, 0, 0.32, 0],
  },
};

/** 运行时调色板：颜色为线性空间，可直接写入 uniform；params 为数值参数 */
export interface Palette {
  skyTop: THREE.Color;
  skyBottom: THREE.Color;
  nebulaA: THREE.Color;
  nebulaB: THREE.Color;
  star: THREE.Color;
  planet: THREE.Color;
  atmosphere: THREE.Color;
  haze: THREE.Color;
  tileA: THREE.Color;
  tileB: THREE.Color;
  grid: THREE.Color;
  rim: THREE.Color;
  cliff: THREE.Color;
  steel: THREE.Color;
  shadow: THREE.Color;
  light: THREE.Color;
  belt: THREE.Color;
  ghost: THREE.Color;
  flash: THREE.Color;
  select: THREE.Color;
  /** 传送带等级色：Mk.I ~ Mk.III */
  tiers: THREE.Color[];
  params: THREE.Vector4;
}

const SINGLE_KEYS = [
  'skyTop', 'skyBottom', 'nebulaA', 'nebulaB', 'star', 'planet', 'atmosphere', 'haze', 'tileA', 'tileB',
  'grid', 'rim', 'cliff', 'steel', 'shadow', 'light', 'belt', 'ghost', 'flash', 'select',
] as const;

/** 解析主题调色板 */
export function resolvePalette(theme: AlgoVizTheme): Palette {
  const src = SOURCES[theme];
  const palette = { tiers: src.tiers.map((c) => new THREE.Color(c)), params: new THREE.Vector4(...src.params) } as Palette;
  for (const key of SINGLE_KEYS) palette[key] = new THREE.Color(src[key]);
  return palette;
}

export function clonePalette(p: Palette): Palette {
  const out = { tiers: p.tiers.map((c) => c.clone()), params: p.params.clone() } as Palette;
  for (const key of SINGLE_KEYS) out[key] = p[key].clone();
  return out;
}

/** out = a → b 的插值结果（原地写入 out 的对象，保持 uniform 引用不变） */
export function lerpPalette(out: Palette, a: Palette, b: Palette, t: number): void {
  for (const key of SINGLE_KEYS) out[key].copy(a[key]).lerp(b[key], t);
  out.tiers.forEach((c, i) => c.copy(a.tiers[i]).lerp(b.tiers[i], t));
  out.params.copy(a.params).lerp(b.params, t);
}
