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
const buildIndex = (): Map<string, {x: number; y: number}> => {
  const map = new Map<string, {x: number; y: number}>()
  for (const list of [itemData, buildingData, resourceData] as unknown as IconEntry[][]) {
    for (const entry of list) {
      if (!entry.icon) continue
      map.set(entry.id, entry.icon)
      if (entry.name?.zh && !map.has(entry.name.zh.replace(/\s/g, ' '))) map.set(entry.name.zh.replace(/\s/g, ' '), entry.icon)
    }
  }
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
