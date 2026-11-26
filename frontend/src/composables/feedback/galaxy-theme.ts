import type {StarProgress} from '../../types/feedback'

// =============== 星图双主题调色 ===============
// 说明：画布绘制不能读取 CSS 变量，星云、星核与背景尘点的日夜色值集中在此。
// 深色保持通透的星云丝缕与三层景深；浅色采用宣纸浅底、松烟墨灰浓淡晕染的近单色水墨星图，
// 中心温润浅宣纸向边缘自然衔接 --ds-bg-shell（#ebebeb）外壳，墨分五色而克制留白。
// 边界：舞台底色与文字使用 style.scss 的 --ds-* token 及主题模式类，本文件负责画布内的颜色与曝光参数。
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
    wispNebula: string
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
    dust: '#b8d0f2',
    warmDust: '#edd2ae',
    wispCool: '#5280d4',
    wispNebula: '#84509a',
    wispWarm: '#cfa06c',
    wispAlpha: .062,
    glow: '#e8c488',
    glowMid: '#6882c8',
    glowAlpha: .15,
    dustScale: 1.38,
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
  // 宣纸浅底中位参考色（舞台由 #f4f2ed 中心向 #ebebeb 边缘极柔和过渡，无缝衔接浅灰壳体）。
  stage: '#f2f0eb',
  // 近单色水墨星点：纯正焦墨（普通反馈）、极克制青黛深墨（已采纳）、一抹淡朱砂深墨印泥点睛（已实现）。
  // 三色对宣纸底对比度均远超 9:1，配合柔和宣纸托底在密集墨带中清晰跳出。
  starColors: {feedback: '#1a1e24', accepted: '#243646', implemented: '#723026'},
  privateColor: '#5c626a',
  coreHighlight: '#efede8',
  coreCushion: '#f5f3ee',
  focusOutline: 'rgba(245, 243, 238, 0.90)',
  composite: 'source-over',
  glow: {base: .20, slope: .20},
  sky: {
    dust: '#747a82',
    warmDust: '#646970',
    wispCool: '#8e949c',
    wispNebula: '#848a92',
    wispWarm: '#7d838b',
    wispAlpha: .060,
    // 极轻清墨银心，不向外扩散成圈，保持宣纸留白与水墨呼吸感。
    glow: '#9aa0a8',
    glowMid: '#aeb3ba',
    glowAlpha: .10,
    dustScale: 1.15,
  },
  nebula: {
    blend: 'over',
    gain: .74,
    // 近单色松烟墨灰阶（墨分五色：淡重墨中轴 → 松烟中墨 → 清墨羽化 → 纸色留白），
    // 控制峰值墨色深度在中浅灰墨阶，密集 200 条不糊成黑团，焦墨星点清晰跃出。
    stops: [
      [0, 146, 150, 156], [.16, 134, 139, 146], [.32, 114, 120, 128],
      [.50, 108, 114, 122], [.68, 124, 130, 138], [.85, 156, 161, 168], [1, 194, 198, 202],
    ],
  },
}

export const GALAXY_THEMES: Readonly<Record<GalaxyMode, GalaxyTheme>> = {dark: DARK, light: LIGHT}
