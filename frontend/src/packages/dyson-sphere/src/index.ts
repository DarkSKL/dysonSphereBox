/**
 * @dyson/sphere —— 首页戴森球 3D 动画渲染包。
 *
 * 公开 API：
 * - createDysonSphere(container, options) → DysonSphereController
 * - 类型：DysonSphereParams / DysonAchievement / DysonTheme / ...
 * - 常量：DEFAULT_PARAMS / QUALITY_PRESETS / DEFAULT_INTERACTION / TIER_LABELS / DYSON_SPHERE_CSS
 * - 工具：isAchievementUnlocked / createMockAchievements / MOCK_ACHIEVEMENTS
 *
 * 渲染实现（layers/、internal/、scene.ts）不属于公开 API，可能随时调整。
 * 公开声明不引用 three 的类型，使用方无需安装 @types/three。
 */

export { createDysonSphere } from './create';
export { DEFAULT_INTERACTION, DEFAULT_PARAMS, QUALITY_PRESETS } from './defaults';
export { isAchievementUnlocked, TIER_LABELS } from './achievement';
export { createMockAchievements, MOCK_ACHIEVEMENTS } from './mock';
export { DYSON_SPHERE_CSS } from './styles';

export type {
  AchievementTier,
  DysonAchievement,
  DysonInteraction,
  DysonQuality,
  DysonQualityPreset,
  DysonSphereController,
  DysonSphereOptions,
  DysonSphereParams,
  DysonStats,
  DysonTheme,
} from './types';
