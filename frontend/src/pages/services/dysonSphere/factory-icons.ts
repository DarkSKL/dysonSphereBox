import {computed, shallowRef} from 'vue'
import itemData from '../../../data/encyclopedia/item.json'
import buildingData from '../../../data/encyclopedia/building.json'
import resourceData from '../../../data/encyclopedia/resource.json'
import {encyclopediaManifest, encyclopediaSpriteReady, loadLocalIconPack} from '../encyclopedia/catalog'

// =============== 量化物品图标 ===============
// 职责：按中文名把量化数据集物品映射到图鉴精灵坐标；使用：节点图与选择器。
// 边界：量化与图鉴来自不同上游，按名称或明确的原版图鉴 ID 匹配；未收录名称返回 null，由调用方使用通用图标。
type IconEntry = {id: string; name?: {zh?: string}; icon?: {x: number; y: number}}
const iconIndex = shallowRef<Map<string, {x: number; y: number}> | null>(null)
// 本地追加到图鉴精灵的格子：来源与坐标记录在 public/encyclopedia-icons/NOTICE.txt，按稳定键索引供量化界面引用。
// 边界：重新导入图鉴素材会覆盖图集，需按 NOTICE 补回对应格子，否则该键只显示空白格。
const appendedIcons: Record<string, {x: number; y: number}> = {
  // 禁用增产剂：增产剂 Mk.I 叠加禁用标记，表示不喷涂。
  'proliferator-disabled': {x: 896, y: 1408},
}
const buildIndex = (): Map<string, {x: number; y: number}> => {
  const map = new Map<string, {x: number; y: number}>()
  for (const list of [itemData, buildingData, resourceData] as unknown as IconEntry[][]) {
    for (const entry of list) {
      if (!entry.icon) continue
      map.set(entry.id, entry.icon)
      if (entry.name?.zh && !map.has(entry.name.zh.replace(/\s/g, ' '))) map.set(entry.name.zh.replace(/\s/g, ' '), entry.icon)
    }
  }
  for (const [key, icon] of Object.entries(appendedIcons)) map.set(key, icon)
  return map
}

// 原版译名差异通过别名对齐；机枪弹箱的图鉴中文名误标，按其稳定 ID 取本地铜色弹箱精灵。
const nameAliases: Record<string, string> = {
  '极速分拣器': '急速分拣器',
  '极速传送带': '急速传送带',
  '钢材': '钢',
  '机枪弹箱': 'df-magnum-ammo-box',
  '电弧熔炉': '电磁熔炉',
  '蓄电器（满）': '蓄电器 (满)',
  '全息信标': '信号塔',
  '近程电浆塔': '近程等离子体塔',
  // 统计目录沿用游戏原型名（低速传送带、流速器等），与图鉴译名不同的建筑在此对齐，统计页面的图标才能解析。
  '低速传送带': '传送带',
  '低速分拣器': '分拣器',
  '流速器': '流速监测器',
  '物流运输站': '行星内物流运输站',
  '制造台 Mk.IV': '重组式制造台',
  '熔炉 Mk.III': '负熵熔炉',
  '化工厂 Mk.II': '量子化工厂',
  '矩阵研究站 Mk.II': '自演化研究站',
  '激光灯塔': '信号塔',
}
const normalize = (name: string): string => { const value = name.replace(/\s/g, ' '); return nameAliases[value] ?? value }
/** 量化物品名对应的图鉴中文名或稳定 ID；图标与图鉴属性（如建筑功率）按同一规则对齐 */
export const factoryEncyclopediaName = normalize

export const factoryIconsReady = computed(() => encyclopediaSpriteReady.value)
export function ensureFactoryIcons(): void {
  if (!iconIndex.value) iconIndex.value = buildIndex()
  void loadLocalIconPack()
}
/** 返回名称对应的图鉴精灵格坐标（像素，左上角为原点）；索引未建立、精灵未就绪或名称未收录时返回 null。 */
export function factoryIconCell(name: string): {x: number; y: number} | null {
  const icon = iconIndex.value?.get(normalize(name))
  return icon && encyclopediaSpriteReady.value ? icon : null
}
/** 返回指定尺寸下的精灵背景样式；缺失时返回 null。 */
export function factoryIconStyle(name: string, size: number): Record<string, string> | null {
  // 图鉴精灵仍是旧版；新增透镜使用从原版安装提取的独立图标。
  if (name === '黑雾引力透镜') {
    return {width: `${size}px`, height: `${size}px`, backgroundImage: 'url("/quant-icons/darkfog-lens.png")', backgroundSize: 'contain'}
  }
  const icon = factoryIconCell(name)
  if (!icon) return null
  const sprite = encyclopediaManifest.sprite
  const ratio = size / sprite.cell
  return {width: `${size}px`, height: `${size}px`, backgroundImage: `url("${sprite.path}")`, backgroundSize: `${sprite.width * ratio}px ${sprite.height * ratio}px`, backgroundPosition: `${-icon.x * ratio}px ${-icon.y * ratio}px`}
}

// =============== Canvas 图标 ===============
// 职责：把图鉴精灵中的一格裁成独立画布，供 ECharts 富文本等 Canvas 场景使用；使用：量化分析图表的行标签。
// 边界：精灵未加载完成或名称未收录时返回 null，调用方只显示文字；按格子与像素尺寸缓存，主题切换不影响。
let spriteImage: HTMLImageElement | null = null
/** 精灵原图可供 Canvas 裁切时为 true；在计算属性里读取以便加载完成后重新生成图表配置。 */
export const factoryIconImageReady = shallowRef(false)
const iconCanvasCache = new Map<string, HTMLCanvasElement>()
export function factoryIconCanvas(name: string, size: number): HTMLCanvasElement | null {
  ensureFactoryIcons()
  if (!spriteImage) {
    spriteImage = new Image()
    spriteImage.onload = () => { factoryIconImageReady.value = true }
    spriteImage.src = encyclopediaManifest.sprite.path
  }
  const cell = factoryIconCell(name)
  if (!cell || !factoryIconImageReady.value) return null
  const pixels = Math.round(size * Math.min(3, Math.max(1, window.devicePixelRatio || 1)))
  const key = `${cell.x}:${cell.y}:${pixels}`
  const cached = iconCanvasCache.get(key)
  if (cached) return cached
  const canvas = document.createElement('canvas')
  canvas.width = pixels
  canvas.height = pixels
  const context = canvas.getContext('2d')
  if (!context) return null
  context.imageSmoothingQuality = 'high'
  const cellSize = encyclopediaManifest.sprite.cell
  context.drawImage(spriteImage, cell.x, cell.y, cellSize, cellSize, 0, 0, pixels, pixels)
  iconCanvasCache.set(key, canvas)
  return canvas
}
