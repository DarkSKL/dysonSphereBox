import type { Component } from 'vue'
import {
  IconAdjustmentsHorizontal,
  IconFileText,
  IconInfoCircle,
  IconMessages,
  IconNetwork,
} from '@tabler/icons-vue'
import {gameNavigationIcons} from './game-icons'

/** 描述可由侧边导航渲染的路由入口。 */
export interface NavigationEntry {
  readonly key: string
  readonly label: string
  readonly to: string
  readonly icon: Component
  /** 需要登录后才能进入；未登录时点击改为提示并打开登录弹窗 */
  readonly requiresAuth?: boolean
}

/** 描述快捷栏中的工作区，以及该工作区的第二层选项。 */
export interface WorkspaceNavigationDefinition {
  readonly key: string
  readonly label: string
  readonly shortLabel: string
  readonly to: string
  readonly icon: Component
  readonly navigation: readonly NavigationEntry[]
  /** 需要登录后才能进入；未登录时点击改为提示并打开登录弹窗 */
  readonly requiresAuth?: boolean
}

/** 全局导航入口由快捷栏渲染，统一使用 Tabler 图标，不混入游戏选项列表。 */
export const globalNavigationEntries: readonly NavigationEntry[] = [
  { key: 'system-log', label: '日志', to: '/system/log', icon: IconFileText },
  { key: 'system-about', label: '关于我们', to: '/system/about', icon: IconInfoCircle },
  { key: 'global-settings', label: '全局设置', to: '/system/settings', icon: IconAdjustmentsHorizontal },
]

/** 工作区注册表统一声明游戏、社区入口及各自的子导航。 */
export const workspaceNavigationRegistry: readonly WorkspaceNavigationDefinition[] = [
  {
    key: 'dyson-sphere-program',
    label: '戴森球计划',
    shortLabel: '戴森球',
    to: '/dysonSphere',
    icon: gameNavigationIcons.planet,
    navigation: [
      { key: 'home', label: '首页', to: '/dysonSphere', icon: gameNavigationIcons.home },
      { key: 'headless', label: '开服', to: '/dysonSphere/headless', icon: gameNavigationIcons.headless },
      { key: 'mods', label: '模组', to: '/dysonSphere/mod', icon: gameNavigationIcons.mods },
      { key: 'resources', label: '资源', to: '/dysonSphere/environment', icon: gameNavigationIcons.resources },
      { key: 'encyclopedia', label: '图鉴', to: '/dysonSphere/encyclopedia', icon: gameNavigationIcons.encyclopedia },
      { key: 'settings', label: '游戏设置', to: '/dysonSphere/settings', icon: gameNavigationIcons.settings },
    ],
  },
  {
    key: 'community',
    label: '社区',
    shortLabel: '社区',
    to: '/community',
    icon: IconMessages,
    requiresAuth: true,
    navigation: [
      { key: 'multiplayer', label: '联机', to: '/community/multiplayer', icon: IconNetwork, requiresAuth: true },
    ],
  },
]
