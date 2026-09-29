/**
 * 对外公开的类型定义。
 *
 * 注意：本文件以及 index.ts 导出的全部声明都不引用 three 的类型。
 * 使用方即使是 TypeScript 4.x、没有安装 @types/three，也能直接使用本包的类型声明。
 */

/** 主题 */
export type DysonTheme = 'dark' | 'light';

/** 画质档位 */
export type DysonQuality = 'low' | 'mid' | 'high';

/** 成就等级：铜 / 银 / 金，分别位于由内到外的三条轨道 */
export type AchievementTier = 'bronze' | 'silver' | 'gold';

/** 成就数据 */
export interface DysonAchievement {
  /** 唯一标识，selectAchievement / unlockAchievement 通过它定位节点 */
  id: string;
  /** 标题，显示在 tooltip 与卡片上 */
  title: string;
  /** 描述，显示在卡片上 */
  desc?: string;
  /** 等级，决定所在轨道与配色 */
  tier: AchievementTier;
  /** 当前进度；current >= threshold 视为已解锁 */
  current: number;
  /** 解锁所需进度，需大于 0 */
  threshold: number;
  /** 解锁时间，仅用于展示的任意字符串；未解锁时为 null 或省略 */
  unlockedAt?: string | null;
}

/** 场景参数，均可通过 setParams 实时修改 */
export interface DysonSphereParams {
  /** 画质档位：限制像素比，决定默认粒子数、是否开启泛光与 MSAA */
  quality: DysonQuality;
  /** 戴森壳建造进度 0~1；前进时壳板从外侧飞来落位，回退时立即回收 */
  progress: number;
  /** 壳层细分级别 1~5，六边形壳板数 92 / 162 / 362 / 642 / 1002 */
  shellLevel: number;
  /** 太阳帆粒子总数 */
  particleCount: number;
  /** 太阳帆轨道层数 1~12 */
  orbitLayers: number;
  /** 整体转速倍率，同时影响壳层、各类轨道与相机自转 */
  spinSpeed: number;
  /** 鼠标吸引强度 0~1.5 */
  attractStrength: number;
  /** 鼠标吸引半径（世界单位，恒星半径 = 1），点击散开的范围随之变化 */
  attractRadius: number;
  /** 鼠标离开后粒子回到原轨道的速度 */
  returnSpeed: number;
  /** 泛光强度 */
  bloomStrength: number;
  /** 泛光阈值 0~1 */
  bloomThreshold: number;
  /** 恒星颜色，CSS 十六进制色值，例如 '#ffc46b' */
  starColor: string;
  /** 恒星脉动强度 0~1 */
  starPulse: number;
  /** 背景星点数量 */
  starCount: number;
  /** 背景星云强度 0~2 */
  nebulaIntensity: number;
}

/** 交互开关，可在创建时传入，也可通过 setInteraction 实时修改 */
export interface DysonInteraction {
  /** 滚轮缩放。开启后滚轮在场景上不会滚动页面，嵌入可滚动页面时可按需关闭 */
  wheelZoom: boolean;
  /** 按住鼠标中键拖动旋转视角，松开后自动复位 */
  orbitDrag: boolean;
  /** 鼠标吸引太阳帆粒子，点击空白处散开 */
  pointerAttract: boolean;
}

/** 渲染统计，约每 0.5 秒回调一次 */
export interface DysonStats {
  fps: number;
}

/** 画质档位配置 */
export interface DysonQualityPreset {
  /** devicePixelRatio 上限 */
  maxDpr: number;
  /** 切换到该档位时的默认粒子数 */
  particleCount: number;
  /** 是否开启泛光 */
  bloom: boolean;
  /** 后处理 MSAA 采样数 */
  samples: number;
}

/** createDysonSphere 的配置 */
export interface DysonSphereOptions {
  /** 初始参数，未指定的项使用 DEFAULT_PARAMS */
  params?: Partial<DysonSphereParams>;
  /** 初始主题，默认 'dark' */
  theme?: DysonTheme;
  /** 初始成就列表，默认为空（不显示成就节点） */
  achievements?: readonly DysonAchievement[];
  /** 交互开关，默认全部开启 */
  interaction?: Partial<DysonInteraction>;
  /** 是否自动向 document.head 注入场景样式，默认 true；设为 false 时需自行引入 style.css */
  injectStyles?: boolean;
  /** 渲染统计回调 */
  onStats?: (stats: DysonStats) => void;
  /** 选中 / 取消选中成就节点时回调；参数为成就数据副本，取消选中时为 null */
  onSelect?: (achievement: DysonAchievement | null) => void;
}

/**
 * createDysonSphere 返回的控制器，是场景对外的全部接口。
 * WebGL 不可用时 supported 为 false，场景显示静态降级画面；
 * 此时修改类方法不产生渲染效果，getter 仍返回当前状态，dispose 仍需调用。
 */
export interface DysonSphereController {
  /** WebGL 是否可用 */
  readonly supported: boolean;

  /** 局部更新参数，立即生效。切换画质且未同时指定 particleCount 时，粒子数重置为该档位默认值 */
  setParams(patch: Partial<DysonSphereParams>): void;
  /** 当前参数的副本 */
  getParams(): DysonSphereParams;

  /** 切换主题；默认约 400ms 平滑过渡，immediate 为 true 时立即切换 */
  setTheme(theme: DysonTheme, immediate?: boolean): void;
  getTheme(): DysonTheme;

  /** 局部更新交互开关 */
  setInteraction(patch: Partial<DysonInteraction>): void;
  getInteraction(): DysonInteraction;

  /** 替换成就列表：重建全部成就节点，并取消当前选中 */
  setAchievements(list: readonly DysonAchievement[]): void;
  /** 当前成就列表的副本 */
  getAchievements(): DysonAchievement[];
  /**
   * 解锁指定成就并播放解锁动画。
   * @param unlockedAt 展示用的解锁时间，默认取当前本地时间
   * @returns 成就不存在或已解锁时返回 false
   */
  unlockAchievement(id: string, unlockedAt?: string): boolean;
  /** 选中成就（打开跟随卡片、暂停所在轨道）；传 null 取消选中；id 不存在时不做任何事 */
  selectAchievement(id: string | null): void;

  /** 从 0 重新播放一次戴森壳建造动画，最终回到当前建造进度 */
  replayBuild(): void;
  /** 视角复位：缩放恢复默认，旋转偏移平滑归零 */
  resetView(): void;

  /** 停止渲染并释放全部 WebGL 资源、DOM 与事件监听；调用后控制器不可再使用 */
  dispose(): void;
}
