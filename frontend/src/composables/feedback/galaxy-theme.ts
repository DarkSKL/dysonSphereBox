import type {StarProgress} from '../../types/feedback'

// =============== 星图双主题调色 ===============
// 说明：画布绘制不能读取 CSS 变量，星云、星核与背景尘点的日夜色值集中在此。
// 深色沿用原有参数；浅色对齐首页戴森球的日间调色板（packages/dyson-sphere 的 LIGHT：#ebebeb 底、
// 中性灰云团、深灰星点、普通透明叠放），不使用夜间的蓝色发光，也不是深色的反相。
// 边界：舞台底色与文字使用 style.scss 的 --ds-* token，本文件只负责画布内的颜色。
export type GalaxyMode = 'dark' | 'light'
export type NebulaBlend = 'screen' | 'over'
type Stop = readonly [position: number, red: number, green: number, blue: number]

export interface GalaxyTheme {
  mode: GalaxyMode
  /** 舞台底色，与 --ds-bg-shell 一致；仅用于对比度核对，DOM 背景仍读取 token。 */
  stage: string
  starColors: Record<StarProgress, string>
  privateColor: string
  /** 星核中心高光。 */
  coreHighlight: string
  /** 星核外描边：浅色下把深色星核与深色云雾分开，深色主题不需要。 */
  coreRing: string | null
  /** 选中或悬停标记的外轮廓，保证落在任意云雾上都可辨。 */
  focusOutline: string
  /** 星光的画布叠加方式：深色发光叠加，浅色普通叠放。 */
  composite: GlobalCompositeOperation
  /** 光晕透明度 = base + heat * slope。 */
  glow: {base: number; slope: number}
  sky: {dust: string; warmDust: string; glow: string; glowAlpha: number; mistAlpha: number}
  nebula: {stops: readonly Stop[]; blend: NebulaBlend; gain: number}
}

const DARK: GalaxyTheme = {
  mode: 'dark',
  stage: '#03060d',
  starColors: {feedback: '#91dbff', accepted: '#c3a0ff', implemented: '#ffc982'},
  privateColor: '#b5c1d0',
  coreHighlight: '#f3f7ff',
  coreRing: null,
  focusOutline: '#111724',
  composite: 'screen',
  glow: {base: .43, slope: .32},
  sky: {dust: '#9eaec7', warmDust: '#b5a69b', glow: '#ffbb4a', glowAlpha: .17, mistAlpha: .04},
  nebula: {
    blend: 'screen',
    gain: 1,
    stops: [
      [0, 255, 187, 74], [.14, 255, 143, 54], [.30, 238, 108, 78],
      [.45, 191, 102, 163], [.62, 129, 113, 238], [.78, 58, 125, 241], [1, 49, 72, 182],
    ],
  },
}

const LIGHT: GalaxyTheme = {
  mode: 'light',
  stage: '#ebebeb',
  // 低饱和的深色星核：对 #ebebeb 的对比度均在 4.5:1 以上。
  starColors: {feedback: '#3f6a8c', accepted: '#6b5a96', implemented: '#8a5a14'},
  privateColor: '#6a6a70',
  coreHighlight: '#ffffff',
  coreRing: '#ffffff',
  focusOutline: '#ffffff',
  composite: 'source-over',
  glow: {base: .30, slope: .22},
  sky: {dust: '#6c6c72', warmDust: '#7a7670', glow: '#4a4a52', glowAlpha: .24, mistAlpha: .028},
  nebula: {
    blend: 'over',
    gain: 1.45,
    // 由银心向外：中性深灰渐变到近白，与首页日间星云的灰白云团一致。
    stops: [
      [0, 72, 72, 78], [.18, 98, 98, 104], [.40, 134, 134, 140],
      [.62, 176, 176, 182], [.82, 214, 214, 218], [1, 244, 244, 246],
    ],
  },
}

export const GALAXY_THEMES: Readonly<Record<GalaxyMode, GalaxyTheme>> = {dark: DARK, light: LIGHT}
