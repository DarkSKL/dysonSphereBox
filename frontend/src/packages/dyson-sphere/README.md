# @dyson/sphere

首页戴森球 3D 动画渲染包（three.js）。对外只暴露一个创建函数和一个控制器，渲染细节全部封装在内部。

## 快速开始

```ts
import { createDysonSphere } from '@dyson/sphere';

const dyson = createDysonSphere(el, {
  theme: 'dark',
  achievements: list, // DysonAchievement[]
});

dyson.setParams({ progress: 0.5 });
dyson.setTheme('light');
dyson.dispose(); // 卸载时必须调用
```

- 容器需要有确定的宽高，建议 `100% × 360~480px` 的横幅
- 画布、tooltip、成就卡片都插入到容器内，样式默认自动注入到 `document.head`
- 依赖 `three`（peer，0.160 ~ 0.189）；公开类型不引用 three，使用方无需安装 `@types/three`，TypeScript 4.6 可直接使用

## 在 Vue 中使用

```vue
<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { createDysonSphere, type DysonSphereController } from '@dyson/sphere';
import { useThemeMode } from '@/composables/useThemeMode';

const el = ref<HTMLElement>();
let dyson: DysonSphereController | null = null;
const { themeMode } = useThemeMode();

onMounted(async () => {
  dyson = createDysonSphere(el.value!, {
    theme: themeMode.value === 'dark' ? 'dark' : 'light',
    achievements: await fetchAchievements(), // 后端成就数据
    onSelect: (a) => console.log('选中', a?.id),
  });
});

// 只读应用主题，不修改全局主题
watch(themeMode, (m) => dyson?.setTheme(m === 'dark' ? 'dark' : 'light'));

onBeforeUnmount(() => dyson?.dispose());
</script>

<template>
  <div ref="el" style="width: 100%; height: 420px" />
</template>
```

## 控制器接口

| 方法 | 说明 |
| --- | --- |
| `supported` | WebGL 是否可用；不可用时显示静态降级画面，其余方法仍可调用 |
| `setParams(patch)` / `getParams()` | 局部更新参数，立即生效；非法值被忽略并在控制台警告 |
| `setTheme(theme, immediate?)` / `getTheme()` | `'dark' \| 'light'`，默认约 400ms 平滑过渡 |
| `setInteraction(patch)` / `getInteraction()` | 交互开关：`wheelZoom` 滚轮缩放、`orbitDrag` 中键旋转、`pointerAttract` 鼠标吸引 |
| `setAchievements(list)` / `getAchievements()` | 替换 / 读取成就列表（场景内部只持有副本） |
| `unlockAchievement(id, unlockedAt?)` | 解锁并播放动画；不存在或已解锁返回 `false` |
| `selectAchievement(id \| null)` | 打开 / 关闭跟随卡片 |
| `replayBuild()` | 从 0 重播戴森壳建造动画 |
| `resetView()` | 缩放与旋转复位 |
| `dispose()` | 释放全部资源，调用后不可再用 |

## 配置项

```ts
interface DysonSphereOptions {
  params?: Partial<DysonSphereParams>;   // 未指定的项使用 DEFAULT_PARAMS
  theme?: 'dark' | 'light';              // 默认 'dark'
  achievements?: DysonAchievement[];     // 默认为空
  interaction?: Partial<DysonInteraction>; // 默认全部开启
  injectStyles?: boolean;                // 默认 true；为 false 时自行插入 DYSON_SPHERE_CSS
  onStats?: (s: { fps: number }) => void;
  onSelect?: (a: DysonAchievement | null) => void;
}
```

主要参数（完整说明见 `src/types.ts` 注释）：

| 参数 | 默认 | 说明 |
| --- | --- | --- |
| `quality` | `'mid'` | `low` / `mid` / `high`：像素比上限 1 / 1.5 / 2，粒子 5k / 15k / 30k，低档关闭泛光 |
| `progress` | `0.72` | 戴森壳建造进度 0~1，前进时壳板从外侧飞来落位 |
| `shellLevel` | `3` | 细分级别 1~5，壳板数 92 / 162 / 362 / 642 / 1002 |
| `particleCount` / `orbitLayers` | `15000` / `6` | 太阳帆粒子数、轨道层数 |
| `spinSpeed` | `1` | 整体转速倍率 |
| `attractStrength` / `attractRadius` / `returnSpeed` | `0.8` / `1.4` / `2.5` | 鼠标吸引与点击散开 |
| `bloomStrength` / `bloomThreshold` | `0.7` / `0.6` | 泛光 |
| `starColor` / `starPulse` | `'#ffc46b'` / `0.5` | 恒星颜色与脉动 |
| `starCount` / `nebulaIntensity` | `1800` / `1` | 背景 |

## 成就数据

```ts
interface DysonAchievement {
  id: string;             // 唯一
  title: string;
  desc?: string;
  tier: 'bronze' | 'silver' | 'gold'; // 铜 / 银 / 金，分别在由内到外三条轨道
  current: number;        // current >= threshold 视为已解锁
  threshold: number;
  unlockedAt?: string | null; // 仅展示
}
```

后端接入前可用 `createMockAchievements(count, unlockRatio)` 生成模拟数据。

## 交互

- 移动鼠标：附近太阳帆被吸引聚拢；点击空白处散开
- 滚轮：推近 / 拉远；按住中键拖动：旋转视角，松开自动复位
- 悬停成就节点：tooltip；点击：跟随卡片，所在轨道暂停；点击空白或 Esc 关闭
- 键盘：Tab 聚焦视觉隐藏的成就按钮列表，Enter 打开卡片
- `prefers-reduced-motion`：降速，关闭吸引、视差与壳板飞行
- 容器不可见或页面隐藏时自动暂停渲染

## 迁移到应用

1. 把本目录复制到 `client/frontend/src/packages/dyson-sphere/`，前端安装 `three@0.186.1`
2. 直接从源码导入：`import { createDysonSphere } from '@/packages/dyson-sphere/src'`（由应用的 Vite 编译 TS）
   - 前端 TS 4.6 解析源码会连带解析 `@types/three`，而 `@types/three@0.186` 需要 TS 5 以上。若前端要做 `vue-tsc` 类型检查，改用第 3 种方式
3. 或者在本目录 `npm run build`，从 `dist/` 导入（公开 `.d.ts` 不含 three 类型，已用 TS 4.6.4 验证：`npm run check:types-ts4`）
4. 用 `useThemeMode().themeMode` 驱动 `setTheme`，用后端成就数据调用 `setAchievements`

## 目录

```
src/
  index.ts          公开导出
  create.ts         createDysonSphere
  types.ts          公开类型
  defaults.ts       DEFAULT_PARAMS / QUALITY_PRESETS / DEFAULT_INTERACTION
  achievement.ts    isAchievementUnlocked / TIER_LABELS
  mock.ts           模拟成就数据
  styles.ts         场景 CSS 与注入
  scene.ts          场景实现（内部）
  internal/         调色板、参数校验、覆盖层 DOM、着色器片段、工具
  layers/           背景 / 恒星 / 戴森壳 / 戴森云 / 成就节点 / 拖尾 / 后处理
test/consumer-ts4.ts  TS 4.6 使用方类型检查
```

## 脚本

```bash
npm run typecheck        # 严格模式类型检查
npm run build            # 输出 dist/（JS + .d.ts）
npm run check:types-ts4  # 用 TypeScript 4.6.4 检查 dist 公开类型（需先 build）
```
