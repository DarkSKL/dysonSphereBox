import type {StarProgress} from '../../types/feedback'

// =============== 星图双主题调色 ===============
// 说明：画布绘制不能读取 CSS 变量，星云、星核与背景尘点的日夜色值集中在此。
// 深色保持通透的星云丝缕与三层景深；浅色维持 --ds-bg-shell（#ebebeb）底色，对齐首页戴森球日间气质，
// 用克制的暖金灰银心过渡到淡靛蓝雾丝，避免灰墨渍涂抹与深色反相。
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
  /** 星核中心微亮芯点色。 */
  coreHighlight: string
  /** 浅色星点背后的柔光羽化托底色（无硬白边）；深色主题为 null。 */
  coreCushion: string | null
  /** 选中或悬停准星的细外轮廓，保证落在任意云雾上都可辨。 */
  focusOutline: string
  /** 星光的画布叠加方式：深色发光叠加，浅色普通叠放。 */
  composite: GlobalCompositeOperation
  /** 光晕透明度 = base + heat * slope。 */
  glow: {base: number; slope: number}
  sky: {
    dust: string
    warmDust: string
    wispCool: string
    wispWarm: string
    wispAlpha: number
    glow: string
    glowMid: string
    glowAlpha: number
    dustScale: number
  }
  nebula: {stops: readonly Stop[]; blend: NebulaBlend; gain: number}
}

const DARK: GalaxyTheme = {
  mode: 'dark',
  stage: '#03060d',
  starColors: {feedback: '#8ad8ff', accepted: '#c4a4ff', implemented: '#ffca80'},
  privateColor: '#9eb0c7',
  coreHighlight: '#f7fbff',
  coreCushion: null,
  focusOutline: 'rgba(6, 11, 22, 0.72)',
  composite: 'screen',
  glow: {base: .40, slope: .34},
  sky: {
    dust: '#a2b8db',
    warmDust: '#d8be9e',
    wispCool: '#5c7fbc',
    wispWarm: '#cb9458',
    wispAlpha: .042,
    glow: '#ffbe64',
    glowMid: '#7c92cf',
    glowAlpha: .25,
    dustScale: 1.25,
  },
  nebula: {
    blend: 'screen',
    gain: .92,
    stops: [
      [0, 255, 194, 96], [.14, 246, 148, 72], [.28, 224, 114, 96],
      [.44, 184, 108, 166], [.62, 124, 122, 232], [.80, 68, 134, 238], [1, 46, 78, 178],
    ],
  },
}

const LIGHT: GalaxyTheme = {
  mode: 'light',
  stage: '#ebebeb',
  // 克制的深靛 / 深鸢尾 / 深琥珀金星核：对 #ebebeb 对比度均高于 6.1:1，落在清透淡雾上也清晰可辨。
  starColors: {feedback: '#25527d', accepted: '#584186', implemented: '#7d4a0c'},
  privateColor: '#666b75',
  coreHighlight: '#f2f6fb',
  coreCushion: '#f5f6f8',
  focusOutline: 'rgba(246, 247, 250, 0.86)',
  composite: 'source-over',
  glow: {base: .26, slope: .24},
  sky: {
    dust: '#747f92',
    warmDust: '#938370',
    wispCool: '#95a5bc',
    wispWarm: '#bca484',
    wispAlpha: .048,
    glow: '#c2a680',
    glowMid: '#98a9c0',
    glowAlpha: .25,
    dustScale: 1.18,
  },
  nebula: {
    blend: 'over',
    gain: .84,
    // 由银心向外：柔和暖金灰 / 浅琥珀雾 → 极淡灰藕荷过渡 → 清透低饱和淡靛蓝 / 雾蓝灰，自然融入 #ebebeb。
    stops: [
      [0, 180, 158, 126], [.16, 174, 155, 134], [.32, 156, 149, 162],
      [.50, 130, 143, 168], [.70, 120, 138, 168], [.86, 146, 160, 182], [1, 188, 196, 208],
    ],
  },
}

export const GALAXY_THEMES: Readonly<Record<GalaxyMode, GalaxyTheme>> = {dark: DARK, light: LIGHT}
