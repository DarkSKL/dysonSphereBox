/**
 * 类型兼容性检查：模拟应用前端（TypeScript 4.6、moduleResolution: node、strict）使用本包。
 * 只做类型检查，不运行。运行方式：npm run check:types-ts4（需要先 npm run build 生成 dist）。
 */
import {
  createDysonSphere,
  createMockAchievements,
  DEFAULT_PARAMS,
  isAchievementUnlocked,
  QUALITY_PRESETS,
  TIER_LABELS,
  type DysonAchievement,
  type DysonSphereController,
  type DysonTheme,
} from '../dist/index';

const el = document.createElement('div');
const achievements: DysonAchievement[] = createMockAchievements(8, 0.5);

const dyson: DysonSphereController = createDysonSphere(el, {
  theme: 'dark',
  params: { quality: 'mid', progress: 0.4 },
  achievements,
  interaction: { wheelZoom: false },
  onStats: ({ fps }) => console.log(fps.toFixed(0)),
  onSelect: (a) => console.log(a ? `${a.title} · ${TIER_LABELS[a.tier]}` : 'none'),
});

const theme: DysonTheme = dyson.getTheme() === 'dark' ? 'light' : 'dark';
dyson.setTheme(theme);
dyson.setParams({ shellLevel: 4, starColor: '#ffaa66' });
dyson.setInteraction({ orbitDrag: true });
dyson.setAchievements(achievements.filter(isAchievementUnlocked));
const ok: boolean = dyson.unlockAchievement('mod-collector');
dyson.selectAchievement('first-mod');
dyson.selectAchievement(null);
dyson.replayBuild();
dyson.resetView();
const particles: number = QUALITY_PRESETS[dyson.getParams().quality].particleCount + DEFAULT_PARAMS.particleCount;
console.log(ok, particles, dyson.supported, dyson.getAchievements().length, dyson.getInteraction().wheelZoom);
dyson.dispose();

// 以下应当报错：用 @ts-expect-error 确认公开类型确实在约束调用方
// @ts-expect-error 主题只能是 'dark' | 'light'
dyson.setTheme('blue');
// @ts-expect-error 画质档位只能是 low / mid / high
dyson.setParams({ quality: 'ultra' });
// @ts-expect-error 成就等级只能是 bronze / silver / gold
const bad: DysonAchievement = { id: 'x', title: 'x', tier: 'platinum', current: 0, threshold: 1 };
console.log(bad);
