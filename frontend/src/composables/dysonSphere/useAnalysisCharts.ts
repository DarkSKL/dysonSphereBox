import {computed, ref, type ComputedRef} from 'vue'
import {useThemeMode} from '../useThemeMode'
import {factoryIconStyle} from '../../pages/services/dysonSphere/factory-icons'
import type {ShareEntry, ShareMetric} from '../../pages/services/dysonSphere/factory-analysis'

// =============== 量化分析图表基础 ===============
// 职责：为 G6 图表、SVG 占比环与 three.js 平台提供随日夜主题切换的调色板、字体与通用浮层样式；使用：量化分析各图表组件。
// 边界：Canvas 不解析 CSS 变量，颜色在此显式维护并与平面预览的平台配色对齐；只负责视觉，不处理统计口径。

export interface AnalysisPalette {
  readonly dark: boolean
  readonly text: string
  readonly sub: string
  readonly muted: string
  readonly faint: string
  readonly grid: string
  readonly track: string
  /** 单色相明度梯，由弱到强：明度即数据 */
  readonly ramp: readonly string[]
  /** 用电地层按矿蓝、青瓷、珊瑚、鸢紫等色相区分建筑，留出恒星金标识总量峰值。 */
  readonly powerLayers: readonly string[]
  /** 整体占比四个口径各自的单色相明度梯：建筑蓝、电力金、生产青、空间紫，四条梯的明度逐级对齐；台地与占比环共用 */
  readonly shareRamps: Readonly<Record<ShareMetric, readonly string[]>>
  /** 等距块侧壁混入的冷色阴影：顶面最亮，侧面按受光向它混合，得到纪念碑谷式三面明暗 */
  readonly shadeTint: string
  /** 唯一强调色：持续剩余等需要处理的量 */
  readonly hero: string
  /** 纪念碑谷平台：方块双色、平台侧面与边缘光 */
  readonly tileA: string
  readonly tileB: string
  readonly tileLine: string
  readonly cliffLeft: string
  readonly cliffRight: string
  readonly rim: string
  readonly shadow: string
  readonly halo: string
  readonly tooltip: {readonly bg: string; readonly text: string; readonly border: string}
  readonly fontUi: string
  readonly fontDisplay: string
}

type PaletteSource = Omit<AnalysisPalette, 'fontUi' | 'fontDisplay'>
// 夜间：深空背景上的能量梯（色相取自应用夜间的炫蓝、恒星金、启用青与渐变紫）；日间：行星白昼下的低饱和梯。平台色取自平面预览调色板。
// 四条占比梯按 OKLCH 推导：沿用建筑梯每一级的明度与彩度，只替换色相，四张卡片因此同样醒目。
const DARK_BUILDING = ['#16253f', '#1a3a5e', '#1f5683', '#2a78b0', '#3c9fdc', '#86d3ff']
const LIGHT_BUILDING = ['#dbe3ee', '#b8cbe1', '#8daace', '#5d86bb', '#2f629f', '#173c6c']
const SOURCES: Readonly<Record<'dark' | 'light', PaletteSource>> = {
  dark: {
    dark: true,
    text: '#eceef8', sub: 'rgba(214, 216, 236, 0.78)', muted: 'rgba(170, 172, 200, 0.62)', faint: 'rgba(170, 172, 200, 0.34)',
    grid: 'rgba(140, 170, 230, 0.13)', track: 'rgba(140, 170, 230, 0.11)',
    ramp: DARK_BUILDING,
    powerLayers: ['#508cb1', '#6bb6ac', '#dc9489', '#a698ca', '#bdab79', '#7898cd', '#89af88', '#c089a9'],
    shareRamps: {
      building: DARK_BUILDING,
      power: ['#332202', '#4c3402', '#6c4c00', '#956901', '#c28b09', '#f6be57'],
      production: ['#022d21', '#004433', '#02614b', '#058668', '#0daf89', '#66e3bc'],
      space: ['#24203f', '#37315f', '#504885', '#6e65b4', '#9187e4', '#c4c0ff'],
    },
    shadeTint: '#01040b',
    hero: '#ffc15a',
    tileA: '#131b2c', tileB: '#0f1626', tileLine: 'rgba(76, 194, 255, 0.10)', cliffLeft: '#182038', cliffRight: '#10172a', rim: 'rgba(76, 194, 255, 0.55)', shadow: 'rgba(0, 0, 0, 0.5)', halo: 'rgba(3, 6, 13, 0.92)',
    tooltip: {bg: '#0f1624', text: '#eceef8', border: 'rgba(76, 194, 255, 0.32)'},
  },
  light: {
    dark: false,
    text: '#18181a', sub: 'rgba(24, 24, 26, 0.74)', muted: '#707072', faint: 'rgba(24, 24, 26, 0.34)',
    grid: 'rgba(24, 40, 70, 0.09)', track: 'rgba(24, 40, 70, 0.08)',
    ramp: LIGHT_BUILDING,
    powerLayers: ['#567e9a', '#52978e', '#be8073', '#9283ad', '#aa976b', '#7185ad', '#789675', '#ad8099'],
    shareRamps: {
      building: LIGHT_BUILDING,
      power: ['#e9e1d5', '#d7c6ad', '#bea379', '#a47c39', '#805801', '#513700'],
      production: ['#d8e6e1', '#b3d1c6', '#82b4a3', '#43957e', '#02725b', '#014838'],
      space: ['#e1e1ee', '#c6c6e2', '#a4a2cf', '#807abc', '#5e54a0', '#3a326c'],
    },
    shadeTint: '#1b2333',
    hero: '#c0841f',
    tileA: '#e9eef5', tileB: '#dfe6ef', tileLine: 'rgba(63, 151, 220, 0.14)', cliffLeft: '#c2ccdb', cliffRight: '#a9b5c8', rim: 'rgba(63, 151, 220, 0.6)', shadow: 'rgba(76, 90, 114, 0.22)', halo: 'rgba(248, 248, 248, 0.94)',
    tooltip: {bg: '#f8f8f8', text: '#18181a', border: 'rgba(0, 0, 0, 0.13)'},
  },
}

// =============== Canvas 字体 ===============
// 说明：读取全局字体栈；Oxanium 加载完成后再更新数值字体，避免首帧图表固定使用回退字体。
const fontUi = ref('')
const fontDisplay = ref('')
let fontProbe: Promise<void> | undefined
const loadAnalysisFonts = (): void => {
  if (fontProbe) return
  const style = getComputedStyle(document.documentElement)
  fontUi.value = style.getPropertyValue('--ds-font-ui').trim()
  const display = style.getPropertyValue('--ds-font-display').trim()
  fontProbe = document.fonts.load('600 12px Oxanium').then(() => { fontDisplay.value = display }).catch(() => { fontDisplay.value = display })
}

/** 当前主题的分析调色板；主题切换或字体加载完成后图表配置随之重新计算 */
export const useAnalysisPalette = (): ComputedRef<AnalysisPalette> => {
  const {themeMode} = useThemeMode()
  loadAnalysisFonts()
  return computed(() => {
    const ui = fontUi.value || 'sans-serif'
    return {...SOURCES[themeMode.value === 'dark' ? 'dark' : 'light'], fontUi: ui, fontDisplay: fontDisplay.value || ui}
  })
}

/** 用户偏好减少动态效果时关闭图表入场动画 */
export const analysisReducedMotion = (): boolean => window.matchMedia('(prefers-reduced-motion: reduce)').matches

// =============== 颜色工具 ===============
const parseHex = (hex: string): [number, number, number] => {
  const value = /^#([0-9a-f]{6})$/i.exec(hex)?.[1] ?? '000000'
  return [0, 2, 4].map(offset => Number.parseInt(value.slice(offset, offset + 2), 16)) as [number, number, number]
}
const toHex = (channels: readonly number[]): string => `#${channels.map(channel => Math.round(Math.min(255, Math.max(0, channel))).toString(16).padStart(2, '0')).join('')}`
/** #rrggbb 线性混合，t=0 为 a，t=1 为 b */
export const mixHex = (a: string, b: string, t: number): string => {
  const from = parseHex(a)
  const to = parseHex(b)
  return toHex(from.map((channel, index) => channel + (to[index] - channel) * t))
}
/** 按 0–1 在明度梯上插值 */
export const rampAt = (ramp: readonly string[], t: number): string => {
  const clamped = Math.min(1, Math.max(0, Number.isFinite(t) ? t : 0))
  const position = clamped * (ramp.length - 1)
  const index = Math.min(ramp.length - 2, Math.floor(position))
  return mixHex(ramp[index], ramp[index + 1], position - index)
}
/** 明度梯格子上的文字是否应使用浅色：夜间低值格偏暗、日间高值格偏暗时返回 true */
export const rampNeedsLightText = (palette: AnalysisPalette, t: number): boolean => (palette.dark ? t < 0.62 : t > 0.5)

/**
 * 整体占比条目的顶面色：台地与占比环共用，同一条目在两处颜色一致。
 * ratio 为该项数值 ÷ 当前口径最大项，按明度梯 0.3–1 取色，最小的块也不会融进背景；
 * 空间构成按角色取固定明度，空地返回 null，由调用方画成空置样式。
 */
export const shareEntryTone = (palette: AnalysisPalette, metric: ShareMetric, entry: ShareEntry, ratio: number): string | null => {
  const ramp = palette.shareRamps[metric]
  if (entry.kind === 'space') return entry.role === 'empty' ? null : rampAt(ramp, entry.role === 'building' ? 0.92 : 0.6)
  return rampAt(ramp, 0.3 + 0.7 * Math.min(1, Math.max(0, ratio)))
}

// =============== 通用浮层与行标签 ===============
/** 行标签文字过长时截断，图表左侧留给数据 */
export const clampLabel = (text: string, max = 9): string => (text.length > max ? `${text.slice(0, max - 1)}…` : text)

/** 浮层里的物品图标：复用图鉴精灵背景样式拼成行内 HTML；图标缺失时返回同尺寸空位，保持文字对齐 */
export const iconHtml = (name: string, size = 18): string => {
  const style = factoryIconStyle(name, size)
  const css = Object.entries(style ?? {width: `${size}px`, height: `${size}px`}).map(([key, value]) => `${key.replace(/[A-Z]/g, char => `-${char.toLowerCase()}`)}:${value}`).join(';')
  return `<span style="display:inline-block;flex:0 0 auto;vertical-align:-4px;margin-right:6px;background-repeat:no-repeat;${css}"></span>`
}
