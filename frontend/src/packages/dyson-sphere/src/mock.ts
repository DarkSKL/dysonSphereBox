import { isAchievementUnlocked } from './achievement';
import type { AchievementTier, DysonAchievement } from './types';

/**
 * 模拟成就数据（静态），用于原型与后端接入前的演示。
 * 正式接入时改为由后端成就服务提供，通过 setAchievements 传入。
 */
export const MOCK_ACHIEVEMENTS: readonly Readonly<DysonAchievement>[] = Object.freeze([
  { id: 'first-mod', title: '初次改装', desc: '安装并启用第一个模组。', tier: 'bronze', current: 1, threshold: 1, unlockedAt: '2026-06-02 21:14' },
  { id: 'mod-collector', title: '模组收藏家', desc: '累计安装 10 个模组。', tier: 'silver', current: 6, threshold: 10, unlockedAt: null },
  { id: 'mod-master', title: '模组大师', desc: '累计安装 30 个模组。', tier: 'gold', current: 6, threshold: 30, unlockedAt: null },
  { id: 'first-blueprint', title: '第一张蓝图', desc: '导入第一张蓝图。', tier: 'bronze', current: 1, threshold: 1, unlockedAt: '2026-06-05 19:40' },
  { id: 'blueprint-engineer', title: '蓝图工程师', desc: '累计导入 20 张蓝图。', tier: 'silver', current: 20, threshold: 20, unlockedAt: '2026-08-11 22:03' },
  { id: 'blueprint-master', title: '蓝图大师', desc: '累计导入 100 张蓝图。', tier: 'gold', current: 20, threshold: 100, unlockedAt: null },
  { id: 'star-traveler', title: '星际旅者', desc: '创建第一个存档配置。', tier: 'bronze', current: 1, threshold: 1, unlockedAt: '2026-06-02 21:30' },
  { id: 'multiverse', title: '多元宇宙', desc: '同时维护 10 个存档配置。', tier: 'silver', current: 3, threshold: 10, unlockedAt: null },
] as DysonAchievement[]);

const EXTRA_TIERS: readonly AchievementTier[] = ['bronze', 'silver', 'gold'];
const UNLOCK_TIME_FALLBACK = '2026-09-01 20:00';

/** 按数量生成成就列表：前 8 个为模拟数据，超出部分自动补充占位成就 */
function buildList(count: number): DysonAchievement[] {
  const list: DysonAchievement[] = [];
  for (let i = 0; i < count; i++) {
    if (i < MOCK_ACHIEVEMENTS.length) {
      list.push({ ...MOCK_ACHIEVEMENTS[i] });
    } else {
      const n = i - MOCK_ACHIEVEMENTS.length + 1;
      const threshold = [5, 20, 50][n % 3];
      list.push({
        id: `extra-${n}`,
        title: `占位成就 #${n}`,
        desc: '原型补充的占位成就，用于测试节点数量。',
        tier: EXTRA_TIERS[n % 3],
        current: Math.floor((threshold * ((n * 37) % 100)) / 100),
        threshold,
        unlockedAt: null,
      });
    }
  }
  return list;
}

/** 按解锁比例调整成就数据：进度越高的越优先视为已解锁 */
function applyUnlockRatio(list: DysonAchievement[], ratio: number): void {
  const target = Math.round(ratio * list.length);
  const score = (a: DysonAchievement) => (isAchievementUnlocked(a) ? 2 : a.current / a.threshold);
  const order = list.map((_, i) => i).sort((i, j) => score(list[j]) - score(list[i]) || i - j);
  order.forEach((idx, rank) => {
    const a = list[idx];
    if (rank < target) {
      if (!isAchievementUnlocked(a)) {
        a.current = a.threshold;
        a.unlockedAt = UNLOCK_TIME_FALLBACK;
      }
    } else if (isAchievementUnlocked(a)) {
      a.current = Math.max(0, a.threshold - 1);
      a.unlockedAt = null;
    }
  });
}

/**
 * 生成一份模拟成就列表（每次返回新的副本）。
 * @param count 数量，默认 8；超过 8 个时补充占位成就
 * @param unlockRatio 已解锁比例 0~1，按进度高低分配；省略时保持模拟数据原样
 */
export function createMockAchievements(count: number = MOCK_ACHIEVEMENTS.length, unlockRatio?: number): DysonAchievement[] {
  const list = buildList(Math.max(0, Math.floor(count)));
  if (unlockRatio !== undefined) applyUnlockRatio(list, unlockRatio);
  return list;
}
