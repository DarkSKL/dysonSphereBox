import type { Component } from 'vue'
import {
  Boxes,
  Home,
  Info,
  Orbit,
  Package,
  SlidersHorizontal,
  ScrollText,
  Server,
  Settings,
} from '@lucide/vue'

/** 描述可由侧边导航渲染的路由入口。 */
export interface NavigationEntry {
  readonly key: string
  readonly label: string
  readonly to: string
  readonly icon: Component
}

/** 描述全局快捷栏中的游戏，以及该游戏自己的第二层选项。 */
export interface GameNavigationDefinition {
  readonly key: string
  readonly label: string
  readonly shortLabel: string
  readonly to: string
  readonly icon: Component
  readonly navigation: readonly NavigationEntry[]
}

/** 全局导航入口由快捷栏渲染，不混入游戏选项列表。 */
export const globalNavigationEntries: readonly NavigationEntry[] = [
  { key: 'system-log', label: '日志', to: '/system/log', icon: ScrollText },
  { key: 'system-about', label: '关于我们', to: '/system/about', icon: Info },
  { key: 'global-settings', label: '全局设置', to: '/system/settings', icon: SlidersHorizontal },
]

/** 游戏注册表是第二层导航的唯一来源；新增游戏时在此声明游戏入口和专属选项。 */
export const gameNavigationRegistry: readonly GameNavigationDefinition[] = [
  {
    key: 'dyson-sphere-program',
    label: '戴森球计划',
    shortLabel: '戴森球',
    to: '/dysonSphere',
    icon: Orbit,
    navigation: [
      { key: 'home', label: '首页', to: '/dysonSphere', icon: Home },
      { key: 'headless', label: '开服', to: '/dysonSphere/headless', icon: Server },
      { key: 'mods', label: '模组', to: '/dysonSphere/mod', icon: Package },
      { key: 'resources', label: '资源', to: '/dysonSphere/environment', icon: Boxes },
      { key: 'settings', label: '游戏设置', to: '/dysonSphere/settings', icon: Settings },
    ],
  },
]
