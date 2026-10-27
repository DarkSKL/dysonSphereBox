import battle from '@iconify-icons/game-icons/crossed-swords'
import science from '@iconify-icons/game-icons/chemical-drop'
import trophy from '@iconify-icons/game-icons/trophy-cup'
import cargo from '@iconify-icons/game-icons/cargo-crate'
import factory from '@iconify-icons/game-icons/factory'
import crystal from '@iconify-icons/game-icons/crystal-cluster'
import gears from '@iconify-icons/game-icons/gears'
import type {EncyclopediaCategory, EncyclopediaKind} from '../../types/encyclopedia'

// =============== 图鉴分类展示 ===============
// 说明：五个入口按浏览频率排列，实体分类在前、成就最后；首项为默认分类。图标使用现有本地 CC BY 3.0 Game Icons。
export const encyclopediaCategories = [
  {key: 'building', label: '建筑', icon: factory},
  {key: 'item', label: '物品', icon: cargo},
  {key: 'technology', label: '科技', icon: science},
  {key: 'combat', label: '战斗', icon: battle},
  {key: 'achievement', label: '成就', icon: trophy},
] as const satisfies readonly {key: EncyclopediaCategory; label: string; icon: typeof battle}[]
export const defaultEncyclopediaCategory: EncyclopediaCategory = encyclopediaCategories[0].key

export const encyclopediaKindLabels: Record<EncyclopediaKind, string> = {building: '建筑', item: '物品', recipe: '配方', technology: '科技', resource: '资源'}
export const encyclopediaKindIcons = {building: factory, item: cargo, recipe: gears, technology: science, resource: crystal} as const
