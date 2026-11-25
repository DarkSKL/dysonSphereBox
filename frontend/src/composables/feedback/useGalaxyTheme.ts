import {computed} from 'vue'

import {useThemeMode} from '../useThemeMode'
import {GALAXY_THEMES, type GalaxyTheme} from './galaxy-theme'

// =============== 星图主题跟随 ===============
// 说明：读取全局主题状态（useThemeMode 的模块级 ref），返回当前画布调色板。
// 使用：galaxy-map 与 visible-stars 调用；切换主题时依赖它的 watch 会重建光晕笔刷并重绘。
// 边界：不切换、不持久化主题，主题仍由 useThemeMode 统一管理。
export function useGalaxyTheme() {
  const {isDarkMode} = useThemeMode()
  return computed<GalaxyTheme>(() => GALAXY_THEMES[isDarkMode.value ? 'dark' : 'light'])
}
