/**
 * 公开类型。声明中不引用 three 的类型，使用方无需安装 @types/three。
 *
 * 坐标约定（与戴森球计划蓝图一致）：
 * - x 向东、y 向北，单位为蓝图格；z 为高度层，单位为传送带层高（约 1.333 米）
 * - yaw 为朝向角（度），0 指向北、顺时针增加
 */

/** 场景主题：light 为纪念碑谷式暖色日景，dark 为暮色夜景 */
export type AlgoVizTheme = 'light' | 'dark';

/** 画质档位：影响像素比上限与是否开启抗锯齿 */
export type AlgoVizQuality = 'low' | 'mid' | 'high';

/** 坐标系：layout 为生成过程中的平面装配格；area 为正式蓝图的区域局部坐标 */
export type AlgoVizCoordinates = 'layout' | 'area';

/** 单个建筑的渲染姿态。只描述外观与位置，不携带蓝图参数。 */
export interface AlgoVizBuilding {
  /** 建筑物品 ID（如 2303 制造台 Mk.I），用于选择模型 */
  readonly itemId: number;
  /** 蓝图模型序号；缺省时按 itemId 推断 */
  readonly modelIndex?: number;
  /** 首端位置 */
  readonly x: number;
  readonly y: number;
  readonly z: number;
  /** 次端位置（分拣器等双端建筑）；缺省等于首端 */
  readonly x2?: number;
  readonly y2?: number;
  readonly z2?: number;
  /** 首端朝向（度） */
  readonly yaw?: number;
  /** 所属区域下标，仅 area 坐标系使用 */
  readonly areaIndex?: number;
  /** 产线分组（量化产线下标）；-1 或缺省为共享设施。只用于展示归属，建筑一律使用模型原生颜色 */
  readonly group?: number;
  /** 是否为生成过程中的临时对象（显示为未定稿的浅色） */
  readonly temporary?: boolean;
  /** 输出连接对象 ID，传送带据此绘制走向 */
  readonly outputId?: string;
  /** 正式记录下标，仅展示用 */
  readonly index?: number;
  /** 机器配方；由 options.icon 解析为机器上方的配方图标，缺省或 0 表示没有 */
  readonly recipeId?: number;
  /** 传送带标记物品（原料入口、成品出口等带节点）；由 options.icon 解析为带节点上方的物品图标 */
  readonly iconItemId?: number;
}

/** 图标精灵图中的一格：左上角像素坐标，格边长见 AlgoVizIconSheet.cell */
export interface AlgoVizIcon {
  readonly x: number;
  readonly y: number;
}

/** 图标精灵图规格：整图像素尺寸与单格边长，格按行列紧密排布 */
export interface AlgoVizIconSheet {
  readonly url: string;
  readonly width: number;
  readonly height: number;
  readonly cell: number;
}

/** 图标分组：belt 为带标记的传送带入口/出口节点（显示标记物品），building 为机器建筑（显示配方产物） */
export type AlgoVizIconGroup = 'belt' | 'building';

/** 各分组的显示开关；只影响图标的绘制，不改变场景对象与播放 */
export type AlgoVizIconVisibility = Readonly<Record<AlgoVizIconGroup, boolean>>;

/** 场景操作：新增、替换（位置/外观变化，保留身份）、撤销 */
export type AlgoVizOperation =
  | { readonly type: 'add'; readonly id: string; readonly building: AlgoVizBuilding }
  | { readonly type: 'replace'; readonly id: string; readonly building: AlgoVizBuilding }
  | { readonly type: 'remove'; readonly id: string };

/** 蓝图区域：用于把 area 局部坐标拼接为整体平面 */
export interface AlgoVizArea {
  readonly index: number;
  readonly parentIndex: number;
  readonly anchorX: number;
  readonly anchorY: number;
  readonly width: number;
  readonly height: number;
}

/** 一批操作。批内按顺序播放，批与批按入队顺序播放。 */
export interface AlgoVizBatch {
  readonly candidateId?: string;
  readonly snapshot?: boolean;
  readonly operations: readonly AlgoVizOperation[];
  /** 坐标系，缺省沿用上一批 */
  readonly coordinates?: AlgoVizCoordinates;
  /** area 坐标系的区域表 */
  readonly areas?: readonly AlgoVizArea[];
  /** 阶段文案；与上一批不同时在播放到本批时显示阶段标题 */
  readonly stage?: string;
  /**
   * 播放节奏：
   * - 'replay'（缺省）：回放已有结果，按节奏逐个落位，长队列在数秒内播完
   * - 'follow'：跟随正在运行的算法，最多落后约 1 秒；积压时按顺序立即应用较早的操作，只为最新变化播放动画
   */
  readonly pace?: 'follow' | 'replay' | 'cinematic';
}

/** 选中的建筑 */
export interface AlgoVizSelection {
  readonly candidateId?: string;
  readonly candidateOrdinal?: number;
  readonly id: string;
  readonly building: AlgoVizBuilding;
  /** 选中时建筑在容器内的屏幕坐标（CSS 像素） */
  readonly screen: { readonly x: number; readonly y: number };
}

/** 播放进度与累计统计（自上次 reset 起） */
export interface AlgoVizProgress {
  readonly candidates: number;
  readonly falling: number;
  /** 队列中尚未开始的操作数 */
  readonly pending: number;
  /** 正在播放动画的对象数 */
  readonly animating: number;
  /** 场景中的对象数 */
  readonly objects: number;
  readonly added: number;
  readonly replaced: number;
  readonly removed: number;
  /** 当前阶段文案 */
  readonly stage: string;
  readonly paused: boolean;
}

/** 渲染统计 */
export interface AlgoVizStats {
  readonly fps: number;
  readonly drawCalls: number;
  readonly triangles: number;
}

export interface AlgoVizOptions {
  /** 默认 'light' */
  theme?: AlgoVizTheme;
  /** 默认 'mid' */
  quality?: AlgoVizQuality;
  /** 播放速度倍率，默认 1，范围 0.25 ~ 8 */
  speed?: number;
  /** 模型键（如 'assembler-1'）到 GLB 地址的映射；缺失的模型以几何体占位 */
  modelUrls?: Readonly<Record<string, string>>;
  /**
   * Basis 转码器文件地址（模型贴图为 KTX2 格式）。构建工具会重命名资源文件时必须提供，
   * 例如 Vite 中以 `three/examples/jsm/libs/basis/basis_transcoder.js?url` 与 `.wasm?url` 导入。
   */
  transcoderUrls?: { readonly js: string; readonly wasm: string };
  /**
   * 涂装遮罩编号（见 render/native-colors.ts 的 mask 字段）到图片地址的映射。
   * 游戏只把材质颜色染在遮罩标出的涂装区；未提供时，带原生颜色的材质整体染色。
   */
  maskUrls?: Readonly<Record<string, string>>;
  /** 可选的自定义对象上限；默认不限制数量，完整展示建筑。应用无需传入此选项。 */
  maxObjects?: number;
  /** 空场景提示文案 */
  emptyText?: string;
  /** 画面顶部被外部工具栏覆盖的像素高度；取景与阶段标题会让出这部分空间，默认 0 */
  viewPaddingTop?: number;
  /** 默认 true；为 false 时需自行插入 ALGORITHM_VIZ_CSS */
  injectStyles?: boolean;
  /** 悬停提示文案；缺省时不显示悬停提示 */
  describe?: (building: AlgoVizBuilding) => string;
  /** 图标精灵图；缺省或加载失败时不绘制任何图标，其余功能不受影响 */
  iconSheet?: AlgoVizIconSheet;
  /** 解析建筑上方要显示的图标；返回 null 表示该建筑没有图标。数据迟到时由 refreshIcons 重新解析 */
  icon?: (building: AlgoVizBuilding) => AlgoVizIcon | null;
  /** 初始的图标显示开关，缺省的分组默认显示 */
  iconsVisible?: Partial<AlgoVizIconVisibility>;
  onSelect?: (selection: AlgoVizSelection | null) => void;
  /** 进度变化时回调（节流到每帧最多一次） */
  onProgress?: (progress: AlgoVizProgress) => void;
  onStats?: (stats: AlgoVizStats) => void;
}

/** 场景控制器：对外的全部接口 */
export interface AlgorithmVizController {
  /** 切换生成任务，只保留当前及上次任务使用的缓存；同一任务回放无需重复切换。 */
  beginGeneration(taskId: string): void;
  beginCandidate(id: string, ordinal: number): void;
  finishCandidate(id: string, status: 'valid' | 'rejected' | 'pruned'): void;
  finalizeSearch(winnerId: string): void;
  failSearch(): void;
  /** WebGL 是否可用；不可用时显示降级提示，其余方法仍可安全调用 */
  readonly supported: boolean;
  /** 入队一批操作，按播放速度逐个呈现 */
  enqueue(batch: AlgoVizBatch): void;
  /** 立即显示阶段标题（纪念碑谷章节式），detail 为副标题 */
  caption(title: string, detail?: string): void;
  /** 清空场景与队列；animate 为 true 时已有对象播放撤销动画 */
  reset(options?: { animate?: boolean }): void;
  /** 跳过过渡动画；大量操作按整帧预算持续应用，避免跳过按钮阻塞页面。 */
  flush(): void;
  setSpeed(speed: number): void;
  getSpeed(): number;
  setPaused(paused: boolean): void;
  isPaused(): boolean;
  setTheme(theme: AlgoVizTheme, immediate?: boolean): void;
  getTheme(): AlgoVizTheme;
  /** 以 90° 为步进旋转视角（正数逆时针） */
  rotate(steps: number): void;
  /** 视角复位：恢复自动取景 */
  resetView(): void;
  /** 视角锁定：为 true 时立即强制复位（默认方位 + 自动取景），并禁止旋转、平移、缩放；复位操作仍可用 */
  setViewLocked(locked: boolean): void;
  isViewLocked(): boolean;
  /** 显示或隐藏某一分组的图标；只切换可见性，不影响场景对象与播放 */
  setIconsVisible(group: AlgoVizIconGroup, visible: boolean): void;
  getIconsVisible(): AlgoVizIconVisibility;
  /** 重新解析全部对象的图标：精灵图、物品数据等外部数据迟到后调用 */
  refreshIcons(): void;
  /** 选中或取消选中建筑 */
  select(id: string | null): void;
  getProgress(): AlgoVizProgress;
  /** 释放全部资源，调用后不可再用 */
  dispose(): void;
}
