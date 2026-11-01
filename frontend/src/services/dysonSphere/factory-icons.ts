import {computed, shallowRef} from 'vue'
import itemData from '../../data/encyclopedia/item.json'
import buildingData from '../../data/encyclopedia/building.json'
import resourceData from '../../data/encyclopedia/resource.json'
import {encyclopediaManifest, encyclopediaSpriteReady, loadLocalIconPack} from '../encyclopedia/catalog'

// =============== 量化物品图标 ===============
// 职责：按中文名把量化数据集物品映射到图鉴精灵坐标；使用：节点图与选择器。
// 边界：量化与图鉴来自不同上游，ID 与 slug 不一致，只按名称匹配；Mod 物品或名称不一致时返回 null，由调用方使用通用图标。
type IconEntry = {name?: {zh?: string}; icon?: {x: number; y: number}}
const iconIndex = shallowRef<Map<string, {x: number; y: number}> | null>(null)
const buildIndex = (): Map<string, {x: number; y: number}> => {
  const map = new Map<string, {x: number; y: number}>()
  for (const list of [itemData, buildingData, resourceData] as unknown as IconEntry[][]) {
    for (const entry of list) if (entry.name?.zh && entry.icon && !map.has(entry.name.zh.replace(/\s/g, ' '))) map.set(entry.name.zh.replace(/\s/g, ' '), entry.icon)
  }
  return map
}

// 原版量化数据集与图鉴存在译名差异；缺少独立图鉴条目的原版物品用最接近的原版图标。
const nameAliases: Record<string, string> = {
  '极速分拣器': '急速分拣器',
  '极速传送带': '急速传送带',
  '钢材': '钢',
  '机枪弹箱': '钛化弹箱',
  '电弧熔炉': '电磁熔炉',
  '蓄电器（满）': '蓄电器 (满)',
  '全息信标': '信号塔',
  '近程电浆塔': '近程等离子体塔',
}
const normalize = (name: string): string => { const value = name.replace(/\s/g, ' '); return nameAliases[value] ?? value }

export const factoryIconsReady = computed(() => encyclopediaSpriteReady.value)
export function ensureFactoryIcons(): void {
  if (!iconIndex.value) iconIndex.value = buildIndex()
  void loadLocalIconPack()
}
/** 返回指定尺寸下的精灵背景样式；缺失时返回 null。 */
export function factoryIconStyle(name: string, size: number): Record<string, string> | null {
  const icon = iconIndex.value?.get(normalize(name))
  if (!icon || !encyclopediaSpriteReady.value) return null
  const sprite = encyclopediaManifest.sprite
  const ratio = size / sprite.cell
  return {width: `${size}px`, height: `${size}px`, backgroundImage: `url("${sprite.path}")`, backgroundSize: `${sprite.width * ratio}px ${sprite.height * ratio}px`, backgroundPosition: `${-icon.x * ratio}px ${-icon.y * ratio}px`}
}
