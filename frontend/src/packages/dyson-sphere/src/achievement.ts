import type { AchievementTier, DysonAchievement } from './types';

/** 等级显示名 */
export const TIER_LABELS: Readonly<Record<AchievementTier, string>> = Object.freeze({
  bronze: '铜',
  silver: '银',
  gold: '金',
});

/** 是否已解锁：current >= threshold */
export function isAchievementUnlocked(a: Pick<DysonAchievement, 'current' | 'threshold'>): boolean {
  return a.current >= a.threshold;
}
