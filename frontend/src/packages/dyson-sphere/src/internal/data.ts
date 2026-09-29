import type { AchievementTier, DysonAchievement } from '../types';

const TIERS: ReadonlySet<string> = new Set<AchievementTier>(['bronze', 'silver', 'gold']);

/**
 * 校验并复制成就列表。场景内部只持有副本，不会修改调用方传入的对象。
 * 丢弃 id 重复、等级无效的条目（控制台给出警告）。
 */
export function normalizeAchievements(list: readonly DysonAchievement[]): DysonAchievement[] {
  const out: DysonAchievement[] = [];
  const seen = new Set<string>();
  for (const a of list ?? []) {
    if (!a || typeof a.id !== 'string' || a.id === '') {
      console.warn('[dyson-sphere] 忽略缺少 id 的成就', a);
      continue;
    }
    if (seen.has(a.id)) {
      console.warn(`[dyson-sphere] 忽略重复的成就 id：${a.id}`);
      continue;
    }
    if (!TIERS.has(a.tier)) {
      console.warn(`[dyson-sphere] 忽略等级无效的成就：${a.id}（tier = ${String(a.tier)}）`);
      continue;
    }
    seen.add(a.id);
    const threshold = Number(a.threshold);
    out.push({
      id: a.id,
      title: a.title ? String(a.title) : a.id,
      desc: a.desc ?? '',
      tier: a.tier,
      current: Number(a.current) || 0,
      threshold: threshold > 0 ? threshold : 1,
      unlockedAt: a.unlockedAt ?? null,
    });
  }
  return out;
}

export function copyAchievement(a: DysonAchievement): DysonAchievement {
  return { ...a };
}

/** 当前本地时间，格式 YYYY-MM-DD HH:mm，用作默认解锁时间 */
export function formatNow(): string {
  const d = new Date();
  const p = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
}
