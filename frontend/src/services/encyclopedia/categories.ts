import robot from '@iconify-icons/game-icons/robot-antennas'
import sphere from '@iconify-icons/game-icons/mesh-ball'
import battle from '@iconify-icons/game-icons/crossed-swords'
import science from '@iconify-icons/game-icons/chemical-drop'
import trophy from '@iconify-icons/game-icons/trophy-cup'
import cargo from '@iconify-icons/game-icons/cargo-crate'
import factory from '@iconify-icons/game-icons/factory'
import book from '@iconify-icons/game-icons/book-cover'
import tools from '@iconify-icons/game-icons/anvil'
import layers from '@iconify-icons/game-icons/processor'
import crystal from '@iconify-icons/game-icons/crystal-cluster'
import gears from '@iconify-icons/game-icons/gears'
import type {EncyclopediaCategory, EncyclopediaKind} from '../../types/encyclopedia'

// =============== 图鉴分类展示 ===============
// 说明：分类沿用参考页的十宫格顺序；图标使用现有本地 CC BY 3.0 Game Icons。
export const encyclopediaCategories = [
  {key: 'setting', label: '设定', icon: robot},
  {key: 'gameplay', label: '玩法', icon: sphere},
  {key: 'combat', label: '战斗', icon: battle},
  {key: 'technology', label: '科技', icon: science},
  {key: 'achievement', label: '成就', icon: trophy},
  {key: 'item', label: '物品', icon: cargo},
  {key: 'building', label: '建筑', icon: factory},
  {key: 'tutorial', label: '教程', icon: book},
  {key: 'tool', label: '工具', icon: tools},
  {key: 'overview', label: '综合', icon: layers},
] as const satisfies readonly {key: EncyclopediaCategory; label: string; icon: typeof robot}[]

export const encyclopediaKindLabels: Record<EncyclopediaKind, string> = {building: '建筑', item: '物品', recipe: '配方', technology: '科技', resource: '资源'}
export const encyclopediaKindIcons = {building: factory, item: cargo, recipe: gears, technology: science, resource: crystal} as const
