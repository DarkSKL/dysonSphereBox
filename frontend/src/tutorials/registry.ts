import type { TutorialDefinition, TutorialPageKey } from './types'
import { TOUR_SELECTORS } from './selectors'

// =============== 应用教程注册表 ===============
// 说明：占位阶段只保留应用框架教程，页面业务教程待新的 UI 方案确定后再恢复。
// 边界：本文件只声明教程步骤，不负责读取状态或启动 Driver.js。
export const tutorialRegistry: Record<TutorialPageKey, TutorialDefinition> = {
  global: {
    pageKey: 'global',
    version: 6,
    autoStart: true,
    steps: [
      {
        selector: TOUR_SELECTORS.mainBrand,
        title: '工作区切换',
        description: '左侧图标栏提供戴森球计划和社区入口：游戏工作区管理开服、模组与资源，社区提供联机功能。',
        side: 'right',
        align: 'center',
      },
      {
        selector: TOUR_SELECTORS.mainNav,
        title: '导航层级',
        description: '左侧列表按钮可打开日志、关于和全局设置；进入游戏或社区后，旁边的选项栏显示对应工作区的功能。',
        side: 'right',
        align: 'start',
      },
      {
        selector: TOUR_SELECTORS.mainHeaderTitle,
        title: '当前页面',
        description: '顶部会显示当前页面标题，切换功能后可以快速确认自己所在的位置。',
        side: 'bottom',
        align: 'start',
      },
      {
        selector: TOUR_SELECTORS.mainTutorialLauncher,
        title: '应用教程',
        description: '问号按钮固定在左侧导航底部，可以随时重新查看应用框架说明。',
        side: 'right',
        align: 'end',
      },
      {
        selector: TOUR_SELECTORS.mainThemeToggle,
        title: '主题切换',
        description: '这里可以在日间和夜间模式之间切换，主题偏好会保存到本地配置。',
        side: 'bottom',
        align: 'end',
      },
      {
        selector: TOUR_SELECTORS.mainWindowControls,
        title: '窗口控制',
        description: '无边框窗口的最小化、最大化和关闭都在这里，和普通桌面程序一样使用。',
        side: 'bottom',
        align: 'end',
      },
    ],
  },
}
