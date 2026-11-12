import * as THREE from 'three';
import { DEFAULT_MAX_OBJECTS, QUALITY_PRESETS, SPEED_MAX, SPEED_MIN, type AlgoVizQualityPreset } from './defaults';
import { acquireStyles, releaseStyles } from './styles';
import { IsoCameraRig } from './internal/camera';
import { clamp01, easeInCubic, easeInOutCubic, easeOutBack, easeOutCubic, hashString, lerp, shortestAngle, smoothstep } from './internal/math';
import { castsShadow, copyPose, describeObject, dropHeight, emptyPose, worldPose, type VizObject } from './internal/objects';
import { Overlay } from './internal/overlay';
import { clonePalette, lerpPalette, resolvePalette, type Palette } from './internal/palette';
import { areaOffsets, OperationQueue, type Offset, type QueueFrame, type QueueItem } from './internal/queue';
import { FrameWorkBudget } from './internal/work-budget';
import { CandidateSnapshot } from './internal/snapshot';
import { BeltFlow, BeltGeometry, BELT_THICKNESS, BELT_WIDTH } from './internal/belts';
import { BackdropLayer } from './render/backdrop';
import { RingLayer, ShadowLayer } from './render/effects';
import { IconLayer } from './render/icons';
import { IslandLayer } from './render/island';
import { ModelLibrary, type PartKind } from './render/models';
import { InstancePool, type InstanceAttributeSpec } from './render/pool';
import { BELT_FRAGMENT, BELT_VERTEX, BUILDING_FRAGMENT, BUILDING_VERTEX } from './render/shaders';
import type {
  AlgorithmVizController,
  AlgoVizBatch,
  AlgoVizBuilding,
  AlgoVizCoordinates,
  AlgoVizIconGroup,
  AlgoVizIconVisibility,
  AlgoVizOptions,
  AlgoVizProgress,
  AlgoVizTheme,
} from './types';

/** 1 倍速下每秒播放的操作数 */
const BASE_RATE = 26;
/** 积压追赶时间（秒，1 倍速）：短队列约 8 秒播完，超过 800 / 3000 项时分别缩短到约 5 / 3 秒 */
const TARGET_LAG = 8;
/** 跟随模式：画面最多落后算法的秒数 */
const FOLLOW_LAG = 1;
/** 跟随模式：最多保留多少项排队播放动画，更早的操作立即应用 */
const FOLLOW_KEEP = 160;
/** 跟随模式：最低出队速度（每秒，1 倍速），保证队尾线性收敛而不是指数拖尾 */
const FOLLOW_MIN_RATE = 160;
/** 跳过动画且仍有工作时降低中间画面的重绘频率，保留主线程处理数据；完成帧始终立即绘制。 */
const INSTANT_DRAW_INTERVAL_MS = 100;
const INSTANT_MAX_DRAW_COOLDOWN_MS = 500;
const DUR_ENTER = 0.78;
const DUR_MOVE = 0.7;
const DUR_EXIT = 0.5;
const THEME_DURATION = 0.45;
/** 打开页面即预加载的常用模型 */
const PRELOAD = ['assembler-1', 'assembler-2', 'assembler-3', 'smelter', 'inserter-1', 'inserter-2', 'inserter-3', 'tesla-coil'];
const DRAG_THRESHOLD = 4;
/** 图标基准边长（蓝图格）：传送带入口固定；建筑按占地短边的 0.7 倍取值并夹在上下限之间。屏幕边长另由图标层夹取 */
const ICON_BELT_SIZE = 1.5;
const ICON_BUILDING_MIN = 1.9;
const ICON_BUILDING_MAX = 3.4;
const CANVAS_LABEL = '蓝图生成算法可视化：等距视角的浮空石台上，建筑按算法步骤逐个落位、移动与撤销';
const DEFAULT_EMPTY = '产线计算完成后，算法会在这里逐步搭建蓝图';
const NO_OFFSETS: ReadonlyMap<number, Offset> = new Map();

const STATE_SPEC: InstanceAttributeSpec = { name: 'aState', itemSize: 4 };
const COLOR_SPEC: InstanceAttributeSpec = { name: 'aColor', itemSize: 1 };
const BELT_SPECS: InstanceAttributeSpec[] = [
  { name: 'aFlow', itemSize: 2 },
  { name: 'aStartSide', itemSize: 3 }, { name: 'aStartUp', itemSize: 3 },
  { name: 'aEndSide', itemSize: 3 }, { name: 'aEndUp', itemSize: 3 },
];
/** 失败候选标红的逐实例强度 */
const FAIL_SPEC: InstanceAttributeSpec = { name: 'aFail', itemSize: 1 };

/** 结果确定后，全部候选播完到落选候选一起下坠之间的停留秒数（让失败标注可见） */
const SETTLE_HOLD = 1.1;
/** 每个候选至少展示的秒数（视觉时钟） */
const CANDIDATE_MIN_SHOW = 1.2;

const clampSpeed = (value: number): number => (Number.isFinite(value) ? THREE.MathUtils.clamp(value, SPEED_MIN, SPEED_MAX) : 1);

interface PointerState {
  id: number;
  button: number;
  startX: number;
  startY: number;
  lastX: number;
  lastY: number;
  mode: 'pending' | 'rotate' | 'pan';
}

interface CandidateStage {
  snapshot?: CandidateSnapshot;
  /** 包含尚在退场的对象；按实体身份登记，同 ID 换型不会混淆新旧对象。 */
  readonly objects: Set<VizObject>;
  readonly id: string;
  readonly ordinal: number;
  readonly island: IslandLayer;
  readonly queue: OperationQueue;
  budget: number;
  x: number;
  z: number;
  targetX: number;
  targetZ: number;
  positioned: boolean;
  y: number;
  age: number;
  shownAt?: number;
  /** 失败标注的渐变进度（0 正常，1 全红） */
  failMix: number;
  falling: number;
  status?: 'valid' | 'rejected' | 'pruned';
}

/**
 * 算法可视化场景实现。不对外导出，使用方通过 createAlgorithmViz 获得控制器。
 *
 * 数据流：enqueue 把操作放入队列 => 每帧按速度预算出队 => 新增/替换/撤销转为对象动画
 * => 动画对象逐帧写入实例池 => 统一提交 GPU 并渲染。WebGL 不可用时所有方法安全空转。
 */
export class AlgorithmVizScene implements AlgorithmVizController {
  readonly supported: boolean;

  private readonly container: HTMLElement;
  private readonly options: AlgoVizOptions;
  private readonly maxObjects: number;
  private themeName: AlgoVizTheme;
  private speed: number;
  private paused = false;
  private skipping = false;
  private disposed = false;
  private stylesAcquired = false;
  private fallback: HTMLDivElement | null = null;

  // --- 渲染 ---
  private renderer!: THREE.WebGLRenderer;
  private quality!: AlgoVizQualityPreset;
  private scene!: THREE.Scene;
  private world!: THREE.Group;
  private rig!: IsoCameraRig;
  private uniforms!: Record<string, THREE.IUniform>;
  private palettes!: Record<AlgoVizTheme, Palette>;
  private palette!: Palette;
  private themeFrom: Palette | null = null;
  private themeT = 1;
  private backdrop!: BackdropLayer;
  private island!: IslandLayer;
  private shadows!: ShadowLayer;
  private rings!: RingLayer;
  private icons!: IconLayer;
  /** 各分组的图标显示开关：只决定图标是否绘制，对象、动画与队列不受影响 */
  private iconVisibility: { belt: boolean; building: boolean } = { belt: true, building: true };
  private overlay!: Overlay;
  private models!: ModelLibrary;
  private buildingMaterial!: THREE.ShaderMaterial;
  private beltMaterial!: THREE.ShaderMaterial;
  private unitBox!: THREE.BufferGeometry;
  private beltBox!: THREE.BufferGeometry;
  private readonly pools = new Map<string, InstancePool>();
  /** 自有材质的模型池：释放场景时一并释放其材质 */
  private readonly ownedPools = new Set<InstancePool>();
  private readonly owners = new Map<string, (VizObject | undefined)[]>();
  /** 阴影实例压实时同步建筑的阴影槽号，避免退场留下空绘制范围。 */
  private readonly shadowOwners: (VizObject | undefined)[] = [];
  private unsubscribeModels: (() => void) | null = null;

  // --- 场景对象 ---
  private readonly objects = new Map<string, VizObject>();
  /** 视角锁定：计算期间强制复位并禁止旋转、平移、缩放 */
  private viewLocked = false;
  private readonly animating = new Set<VizObject>();
  /** 正在衰减闪白但不在动画中的对象 */
  private readonly glowing = new Set<VizObject>();
  private readonly dirty = new Set<VizObject>();
  /** 下游对象 ID → 指向它的传送带 */
  private readonly beltInputs = new Map<string, Set<VizObject>>();
  private readonly beltFlow = new BeltFlow();
  private readonly beltGeometry = new BeltGeometry();
  private queue = new OperationQueue();
  private readonly candidates = new Map<string, CandidateStage>();
  /** 淘汰身份在任务内保留，防止迟到批次重新创建已经退场的候选。 */
  private readonly retiredCandidates = new Set<string>();
  private readonly objectCandidates = new Map<string, string>();
  private memberships = new WeakMap<VizObject, CandidateStage>();
  /** 模型就绪时只迁移对应占位体，分帧复用同一工作预算。 */
  private readonly placeholders = new Map<string, Set<VizObject>>();
  private readonly readyModels = new Set<string>();
  /** 每帧从上次让出的候选继续，避免大队列独占操作预算。 */
  private candidateCursor = 0;
  private winnerId = '';
  /** 搜索已有结论（选出获选候选，或整体失败）：此后全部播完即让落选候选同时下坠 */
  private settled = false;
  private settleReadyAt = -1;
  private candidateSpacing = 32;
  private budget = 0;
  private animScale = 1;
  private lastCoordinates: AlgoVizCoordinates = 'layout';
  private lastOffsets: ReadonlyMap<number, Offset> = NO_OFFSETS;
  private lastStage = '';
  private stage = '';
  private added = 0;
  private replaced = 0;
  private removed = 0;
  private boundsDirty = true;
  private boundsAt = -1;
  private firstFrame = true;

  // --- 交互 ---
  private pointer: PointerState | null = null;
  private hoverX = 0;
  private hoverY = 0;
  private hoverPending = false;
  private hovered: VizObject | null = null;
  private selected: VizObject | null = null;
  private readonly raycaster = new THREE.Raycaster();
  private readonly ndc = new THREE.Vector2();

  // --- 临时变量 ---
  private readonly m = new THREE.Matrix4();
  private readonly q = new THREE.Quaternion();
  private readonly v = new THREE.Vector3();
  private readonly s = new THREE.Vector3();
  private readonly ax = new THREE.Vector3();
  private readonly ay = new THREE.Vector3();
  private readonly az = new THREE.Vector3();
  private readonly yAxis = new THREE.Vector3(0, 1, 0);

  // --- 循环 ---
  private width = 1;
  private height = 1;
  private time = 0;
  private running = false;
  private inView = true;
  private rafId = 0;
  private lastNow = 0;
  private fpsFrames = 0;
  private fpsTime = 0;
  private progressVersion = 0;
  private progressSent = -1;
  private progressAt = 0;
  private nextInstantDrawAt = 0;
  private drewLastFrame = false;
  private lastDrawDurationMs = 0;
  private reducedMotion!: MediaQueryList;
  private resizeObserver!: ResizeObserver;
  private intersectionObserver!: IntersectionObserver;

  constructor(container: HTMLElement, options: AlgoVizOptions = {}) {
    if (!container) throw new Error('[algorithm-viz] 需要传入容器元素');
    this.container = container;
    this.options = options;
    this.maxObjects = Math.max(1, options.maxObjects ?? DEFAULT_MAX_OBJECTS);
    this.themeName = options.theme === 'dark' ? 'dark' : 'light';
    this.iconVisibility = { belt: options.iconsVisible?.belt !== false, building: options.iconsVisible?.building !== false };
    this.speed = clampSpeed(options.speed ?? 1);
    if (options.injectStyles !== false) {
      acquireStyles();
      this.stylesAcquired = true;
    }
    container.classList.add('algoviz-root');
    container.dataset.theme = this.themeName;

    const quality = QUALITY_PRESETS[options.quality ?? 'mid'] ?? QUALITY_PRESETS.mid;
    let renderer: THREE.WebGLRenderer;
    try {
      renderer = new THREE.WebGLRenderer({ antialias: quality.antialias, powerPreference: 'high-performance' });
    } catch (err) {
      console.warn('[algorithm-viz] WebGL 不可用，显示降级提示', err);
      this.supported = false;
      this.showFallback();
      return;
    }
    this.supported = true;
    this.renderer = renderer;
    this.quality = quality;
    this.init();
  }

  // -------------------------------------------------------------------------
  // 公共接口
  // -------------------------------------------------------------------------

  enqueue(batch: AlgoVizBatch): void {
    if (this.disposed || !this.supported || !batch?.operations?.length) return;
    if (batch.candidateId && this.retiredCandidates.has(batch.candidateId)) return;
    const coordinates = batch.coordinates ?? this.lastCoordinates;
    if (batch.areas) this.lastOffsets = areaOffsets(batch.areas);
    this.lastCoordinates = coordinates;
    const stage = batch.stage ?? this.lastStage;
    this.lastStage = stage;
    const frame: QueueFrame = { candidateId: batch.candidateId, coordinates, offsets: coordinates === 'area' ? this.lastOffsets : NO_OFFSETS, stage, pace: batch.pace ?? 'replay' };
    const queue = batch.candidateId ? this.candidates.get(batch.candidateId)?.queue : this.queue;
    const candidate = batch.candidateId ? this.candidates.get(batch.candidateId) : undefined;
    if (batch.snapshot && candidate) {
      candidate.snapshot = new CandidateSnapshot(batch.operations, frame);
    } else queue?.push(batch.operations, frame);
    this.touch();
  }

  beginCandidate(id: string, ordinal: number): void {
    if (!this.supported || this.disposed || this.candidates.has(id) || this.retiredCandidates.has(id)) return;
    const island = new IslandLayer(this.uniforms);
    this.scene.add(island.group);
    this.candidates.set(id, {id, ordinal, island, objects: new Set(), queue: new OperationQueue(), budget: 0, x: 0, z: 0, targetX: 0, targetZ: 0, positioned: false, y: 0, age: 0, failMix: 0, falling: -1});
    this.island.group.visible = false;
    this.boundsDirty = true;
    this.touch();
  }

  finishCandidate(id: string, status: 'valid' | 'rejected' | 'pruned'): void {
    const candidate = this.candidates.get(id);
    if (candidate) {
      candidate.status = status;
      if (status !== 'valid') this.retireCandidate(candidate);
    }
    this.touch();
  }

  finalizeSearch(winnerId: string): void {
    this.winnerId = winnerId;
    this.settled = true;
    for (const candidate of this.candidates.values()) if (candidate.id !== winnerId) this.retireCandidate(candidate);
    if (this.skipping) {
      const winner = this.candidates.get(winnerId);
      if (winner) {
        winner.x = winner.targetX;
        winner.z = winner.targetZ;
        winner.island.group.position.set(winner.x, winner.y, winner.z);
        this.dirtyCandidate(winner);
      }
    }
    this.touch();
  }

  failSearch(): void {
    this.winnerId = '';
    this.settled = true;
    for (const candidate of this.candidates.values()) { candidate.status = 'rejected'; this.retireCandidate(candidate); }
    if (this.skipping) {
      for (const candidate of this.candidates.values()) { candidate.age = 100; candidate.shownAt = 0; }
    }
    this.touch();
  }

  caption(title: string, detail?: string): void {
    if (this.disposed || !this.supported || !title) return;
    this.overlay.showCaption(title, detail);
  }

  /** 后端明确淘汰后立即丢弃待播数据并开始下坠，当前已显示对象仍按共享帧预算回收。 */
  private retireCandidate(candidate: CandidateStage): void {
    if (this.retiredCandidates.has(candidate.id)) return;
    this.retiredCandidates.add(candidate.id);
    candidate.queue.clear();
    candidate.snapshot = undefined;
    candidate.budget = 0;
    if (candidate.falling < 0) candidate.falling = 0;
    this.settleReadyAt = -1;
    this.touch();
  }

  reset(options: { animate?: boolean } = {}): void {
    if (this.disposed || !this.supported) return;
    const hadCandidates = this.candidates.size > 0;
    this.queue.clear();
    this.skipping = false;
    for (const candidate of this.candidates.values()) {
      this.scene.remove(candidate.island.group);
      candidate.island.dispose();
    }
    this.candidates.clear();
    this.retiredCandidates.clear();
    this.objectCandidates.clear();
    this.winnerId = '';
    this.settled = false;
    this.settleReadyAt = -1;
    this.candidateSpacing = 32;
    this.candidateCursor = 0;
    this.island.group.visible = true;
    this.budget = 0;
    this.select(null);
    const animate = Boolean(options.animate) && !this.reducedMotion.matches && !hadCandidates;
    if (!animate) {
      // 大厂或候选全场重建直接归还实例池；避免逐建筑撤销把下一轮生成阻塞在清场中。
      this.setHovered(null);
      for (const pool of this.pools.values()) pool.clear();
      for (const owners of this.owners.values()) owners.length = 0;
      this.shadows.clear();
      this.icons.clear();
      this.shadowOwners.length = 0;
      this.objects.clear();
      this.animating.clear();
      this.glowing.clear();
      this.dirty.clear();
      this.beltInputs.clear();
      this.beltFlow.clear();
      this.beltGeometry.beginFrame();
      this.objectCandidates.clear();
      this.memberships = new WeakMap();
      this.placeholders.clear();
      this.readyModels.clear();
      this.boundsDirty = true;
    }
    const live = Array.from(this.objects.values());
    let cx = 0;
    let cz = 0;
    for (const obj of live) {
      cx += obj.pose.x / live.length;
      cz += obj.pose.z / live.length;
    }
    let far = 1;
    for (const obj of live) far = Math.max(far, Math.hypot(obj.pose.x - cx, obj.pose.z - cz));
    // 动画撤销按离中心的距离错开，形成由内向外的退潮
    for (const obj of live) this.startExit(obj, animate ? (Math.hypot(obj.pose.x - cx, obj.pose.z - cz) / far) * 0.45 : 0, !animate);
    this.added = 0;
    this.replaced = 0;
    this.removed = 0;
    this.stage = '';
    this.lastStage = '';
    if (!animate) this.rings.clear();
    this.touch();
  }

  flush(): void {
    if (this.disposed || !this.supported) return;
    this.skipping = true;
    for (const candidate of this.candidates.values()) {
      candidate.age = 100;
      candidate.shownAt = 0;
    }
    this.budget = 0;
    this.touch();
  }

  setSpeed(speed: number): void {
    this.speed = clampSpeed(speed);
    this.touch();
  }

  getSpeed(): number {
    return this.speed;
  }

  setPaused(paused: boolean): void {
    this.paused = paused;
    this.touch();
  }

  isPaused(): boolean {
    return this.paused;
  }

  setTheme(theme: AlgoVizTheme, immediate = false): void {
    if (this.disposed || (theme !== 'light' && theme !== 'dark')) return;
    this.themeName = theme;
    this.container.dataset.theme = theme;
    if (!this.supported) return;
    if (immediate) {
      lerpPalette(this.palette, this.palettes[theme], this.palettes[theme], 1);
      this.themeT = 1;
      this.themeFrom = null;
      this.nextInstantDrawAt = 0;
    } else {
      this.themeFrom = clonePalette(this.palette);
      this.themeT = 0;
    }
  }

  getTheme(): AlgoVizTheme {
    return this.themeName;
  }

  setViewLocked(locked: boolean): void {
    if (this.disposed || !this.supported || locked === this.viewLocked) return;
    this.viewLocked = locked;
    if (!locked) return;
    // 锁定即强制复位：回到默认方位并恢复自动取景；正在进行的拖动立即作废
    if (this.pointer && this.pointer.mode !== 'pending') this.pointer.mode = 'pending';
    this.rig.reset(true);
    this.boundsDirty = true;
    this.boundsAt = -1;
  }

  isViewLocked(): boolean {
    return this.viewLocked;
  }

  setIconsVisible(group: AlgoVizIconGroup, visible: boolean): void {
    if (group !== 'belt' && group !== 'building') return;
    const next = Boolean(visible);
    if (this.iconVisibility[group] === next) return;
    this.iconVisibility[group] = next;
    if (this.disposed || !this.supported) return;
    // 只重写该分组已有图标的对象；槽位随写入阶段按开关分配或释放，建筑实例本身不动
    for (const obj of this.objects.values()) if (obj.icon && this.iconGroup(obj) === group) this.dirty.add(obj);
    this.touch();
  }

  getIconsVisible(): AlgoVizIconVisibility {
    return { ...this.iconVisibility };
  }

  /** 重新解析并重写全部对象的图标；精灵图或物品数据迟到时调用 */
  refreshIcons(): void {
    if (this.disposed || !this.supported) return;
    for (const obj of this.objects.values()) {
      this.resolveIcon(obj);
      this.dirty.add(obj);
    }
    this.touch();
  }

  rotate(steps: number): void {
    if (this.disposed || !this.supported || this.viewLocked || !Number.isFinite(steps)) return;
    this.rig.rotate(Math.round(steps));
    this.boundsDirty = true;
  }

  resetView(): void {
    if (this.disposed || !this.supported) return;
    this.rig.reset(this.viewLocked);
    this.boundsDirty = true;
    this.boundsAt = -1;
  }

  select(id: string | null): void {
    if (this.disposed || !this.supported) return;
    const next = id === null ? null : this.objects.get(id) ?? null;
    if (next === this.selected) return;
    const previous = this.selected;
    this.selected = next;
    if (previous) {
      previous.highlight = previous === this.hovered ? 0.5 : 0;
      this.dirty.add(previous);
    }
    if (!next) {
      this.options.onSelect?.(null);
      return;
    }
    const candidate = this.candidates.get(this.objectCandidates.get(next.id) ?? '');
    this.v.set(next.pose.x + (candidate?.x ?? 0), next.pose.y + next.height * 0.6 + (candidate?.y ?? 0), next.pose.z + (candidate?.z ?? 0)).project(this.rig.camera);
    this.options.onSelect?.({
      id: next.id,
      candidateId: candidate?.id,
      candidateOrdinal: candidate?.ordinal,
      building: next.building,
      screen: { x: ((this.v.x + 1) / 2) * this.width, y: ((1 - this.v.y) / 2) * this.height },
    });
  }

  getProgress(): AlgoVizProgress {
    return {
      candidates: this.candidates.size,
      falling: Array.from(this.candidates.values()).filter(candidate => candidate.falling >= 0).length,
      pending: this.queue.length + Array.from(this.candidates.values()).reduce((total, candidate) => total + candidate.queue.length + (candidate.snapshot?.pending ?? 0), 0),
      animating: this.supported ? this.animating.size : 0,
      objects: this.objects.size,
      added: this.added,
      replaced: this.replaced,
      removed: this.removed,
      stage: this.stage,
      paused: this.paused,
    };
  }

  dispose(): void {
    if (this.disposed) return;
    this.disposed = true;
    if (this.supported) {
      this.running = false;
      cancelAnimationFrame(this.rafId);
      this.unbindEvents();
      this.unsubscribeModels?.();
      for (const pool of this.pools.values()) {
        pool.dispose();
        if (this.ownedPools.has(pool)) pool.disposeMaterials();
      }
      this.pools.clear();
      this.ownedPools.clear();
      this.models.dispose();
      this.shadows.dispose();
      this.icons.dispose();
      this.rings.dispose();
      this.island.dispose();
      for (const candidate of this.candidates.values()) {
        candidate.island.dispose();
        }
      this.candidates.clear();
      this.retiredCandidates.clear();
      this.backdrop.dispose();
      this.unitBox.dispose();
      this.beltBox.dispose();
      this.buildingMaterial.dispose();
      this.beltMaterial.dispose();
      this.overlay.dispose();
      this.renderer.dispose();
      this.renderer.domElement.remove();
      this.container.style.cursor = '';
    }
    this.objects.clear();
    this.animating.clear();
    this.glowing.clear();
    this.dirty.clear();
    this.owners.clear();
    this.shadowOwners.length = 0;
    this.beltInputs.clear();
    this.beltFlow.clear();
    this.beltGeometry.beginFrame();
    this.objectCandidates.clear();
    this.placeholders.clear();
    this.readyModels.clear();
    this.queue.clear();
    this.fallback?.remove();
    this.container.classList.remove('algoviz-root');
    this.container.style.removeProperty('--algoviz-pad-top');
    delete this.container.dataset.theme;
    if (this.stylesAcquired) releaseStyles();
  }

  // -------------------------------------------------------------------------
  // 初始化
  // -------------------------------------------------------------------------

  private init(): void {
    const renderer = this.renderer;
    renderer.toneMapping = THREE.NoToneMapping;
    renderer.outputColorSpace = THREE.SRGBColorSpace;
    const canvas = renderer.domElement;
    canvas.setAttribute('role', 'img');
    canvas.setAttribute('aria-label', CANVAS_LABEL);
    canvas.tabIndex = 0;
    this.container.prepend(canvas);

    this.scene = new THREE.Scene();
    this.rig = new IsoCameraRig();
    const padTop = Math.max(0, this.options.viewPaddingTop ?? 0);
    this.rig.setPadding(padTop);
    this.container.style.setProperty('--algoviz-pad-top', `${padTop}px`);
    this.palettes = { light: resolvePalette('light'), dark: resolvePalette('dark') };
    this.palette = clonePalette(this.palettes[this.themeName]);
    this.uniforms = this.createUniforms();

    this.backdrop = new BackdropLayer(this.uniforms);
    this.island = new IslandLayer(this.uniforms);
    this.world = new THREE.Group();
    this.scene.add(this.backdrop.group, this.island.group, this.world);
    this.shadows = new ShadowLayer(this.world, this.uniforms);
    this.rings = new RingLayer(this.world, this.uniforms);
    // 图标层在全部建筑之后绘制；精灵图缺失或加载失败时保持空层，不影响其余渲染
    this.icons = new IconLayer(this.world, this.uniforms, this.options.iconSheet, () => this.refreshIcons());
    // 占位体：无贴图的钢灰色机身，跟随主题插值
    this.buildingMaterial = this.createBuildingMaterial(null, null, this.palette.steel, new THREE.Color(0, 0, 0));
    this.beltMaterial = new THREE.ShaderMaterial({ vertexShader: BELT_VERTEX, fragmentShader: BELT_FRAGMENT, uniforms: this.uniforms, side: THREE.DoubleSide });
    this.unitBox = new THREE.BoxGeometry(1, 1, 1).translate(0, 0.5, 0);
    // 三段分别负责入口接头、等宽直段和出口接头，接头不会拉伸整条履带。
    this.beltBox = new THREE.BoxGeometry(1, 1, 1, 3, 1, 1).translate(0, 0.5, 0);

    this.models = new ModelLibrary(this.options.modelUrls ?? {}, renderer, this.options.transcoderUrls, this.options.maskUrls);
    this.unsubscribeModels = this.models.onSettled((key) => this.onModelSettled(key));
    this.models.preload(PRELOAD.filter((key) => this.models.has(key)));

    this.overlay = new Overlay(this.container, this.options.emptyText ?? DEFAULT_EMPTY);
    this.reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)');
    this.bindEvents();
    this.resize();
    this.touch();
    this.updateRunning();
  }

  /** 全部着色器共享的 uniform；颜色直接引用调色板对象，主题插值原地生效 */
  private createUniforms(): Record<string, THREE.IUniform> {
    const p = this.palette;
    return {
      uShadow: { value: p.shadow },
      uLight: { value: p.light },
      uHaze: { value: p.haze },
      uFlash: { value: p.flash },
      uGhost: { value: p.ghost },
      uSelect: { value: p.select },
      uSteel: { value: p.steel },
      uViewRight: { value: this.rig.viewRight },
      uViewDir: { value: this.rig.viewDir },
      uHazeTop: { value: -1.6 },
      uHazeBottom: { value: -24 },
      uTime: { value: 0 },
      uParams: { value: p.params },
      uTiers: { value: p.tiers },
      uBelt: { value: p.belt },
      uFlow: { value: 1.1 },
      uTileA: { value: p.tileA },
      uTileB: { value: p.tileB },
      uGrid: { value: p.grid },
      uRim: { value: p.rim },
      uCliff: { value: p.cliff },
      uSkyTop: { value: p.skyTop },
      uSkyBottom: { value: p.skyBottom },
      uNebulaA: { value: p.nebulaA },
      uNebulaB: { value: p.nebulaB },
      uStar: { value: p.star },
      uPlanet: { value: p.planet },
      uAtmosphere: { value: p.atmosphere },
      uAspect: { value: 1 },
      uPixel: { value: 1 },
    };
  }

  /**
   * 建筑材质：共享 uniform + 本材质的原生贴图。tint 直接引用颜色对象，
   * 占位体传入调色板中的钢灰色以随主题插值。
   */
  private createBuildingMaterial(map: THREE.Texture | null, emissiveMap: THREE.Texture | null, tint: THREE.Color, emissive: THREE.Color, kind: PartKind = 'solid', orm: THREE.Texture | null = null, native = false, mask: THREE.Texture | null = null, normalMap: THREE.Texture | null = null, unlit = false): THREE.ShaderMaterial {
    const effect = kind === 'effect';
    return new THREE.ShaderMaterial({
      vertexShader: BUILDING_VERTEX,
      fragmentShader: BUILDING_FRAGMENT,
      uniforms: {
        ...this.uniforms,
        uMap: { value: map },
        uEmissiveMap: { value: emissiveMap },
        uOrm: { value: orm },
        uTint: { value: tint },
        uEmissiveTint: { value: emissive },
        uHasMap: { value: map ? 1 : 0 },
        uHasEmissive: { value: emissiveMap ? 1 : 0 },
        uHasOrm: { value: orm ? 1 : 0 },
        uNative: { value: native ? 1 : 0 },
        uUnlit: { value: unlit ? 1 : 0 },
        uMask: { value: mask },
        uHasMask: { value: mask ? 1 : 0 },
        uNormal: { value: normalMap },
        uHasNormal: { value: normalMap ? 1 : 0 },
        uKind: { value: kind === 'glass' ? 1 : effect ? 2 : 0 },
      },
      side: THREE.DoubleSide,
      // 特效面片加法叠加，不写深度也不遮挡机身；其余类别保持不透明绘制
      transparent: effect,
      depthWrite: !effect,
      blending: effect ? THREE.AdditiveBlending : THREE.NormalBlending,
    });
  }

  private showFallback(): void {
    const div = document.createElement('div');
    div.className = 'algoviz-fallback';
    div.textContent = '当前环境不支持 WebGL，无法显示算法可视化';
    this.container.append(div);
    this.fallback = div;
  }

  // -------------------------------------------------------------------------
  // 操作应用
  // -------------------------------------------------------------------------

  private processQueue(dt: number, work: FrameWorkBudget): void {
    const pending = this.queue.length;
    if (!pending) {
      this.budget = 0;
      this.animScale = 1 / Math.sqrt(this.speed);
      return;
    }
    let processed = 0;
    if (this.skipping) {
      // 仍受全场4ms/1024项预算限制；单队列不再额外卡在128项，候选同时存在时保留轮转份额。
      const quota = this.candidates.size ? 128 : Infinity;
      while (this.queue.length && processed < quota && work.take()) {
        this.applyItem(this.queue.shift() as QueueItem, true);
        processed++;
      }
      this.budget = 0;
      if (processed) this.touch();
      return;
    }
    let rate: number;
    if (this.queue.peek()?.frame.pace === 'cinematic') {
      rate = 60 * this.speed;
      this.animScale = 1 / Math.sqrt(this.speed);
    } else if (this.queue.peek()?.frame.pace === 'follow') {
      // 跟随算法：积压超过可动画的上限时，按原顺序立即应用较早的操作（跳过过渡），只为最新的变化播放动画，
      // 使画面始终落后算法约 FOLLOW_LAG 秒以内
      while (this.queue.length > FOLLOW_KEEP && this.queue.peek()?.frame.pace === 'follow' && processed < 128 && work.take()) {
        this.applyItem(this.queue.shift() as QueueItem, true);
        processed++;
      }
      rate = Math.max(FOLLOW_MIN_RATE * this.speed, this.queue.length / FOLLOW_LAG);
      this.animScale = 1 / THREE.MathUtils.clamp(Math.sqrt(rate / BASE_RATE), 1.6, 5);
    } else {
      // 回放结果：积压越多追赶越快，长队列在数秒内播完，短队列保持逐个可辨的节奏
      const lag = (pending > 3000 ? TARGET_LAG * 0.4 : pending > 800 ? TARGET_LAG * 0.65 : TARGET_LAG) / this.speed;
      rate = Math.max(BASE_RATE * this.speed, pending / lag);
      this.animScale = 1 / THREE.MathUtils.clamp(Math.sqrt(rate / BASE_RATE), 0.5, 5);
    }
    this.budget += dt * rate;
    while (this.budget >= 1 && this.queue.length && processed < 128 && work.take()) {
      this.applyItem(this.queue.shift() as QueueItem, false);
      this.budget -= 1;
      processed++;
    }
    if (!this.queue.length) this.budget = 0;
    if (processed) this.touch();
  }

  private applyItem(item: QueueItem, instant: boolean): void {
    const { op, frame } = item;
    if (item.first && frame.stage && frame.stage !== this.stage) {
      this.stage = frame.stage;
      if (!instant) this.overlay.showCaption(frame.stage);
    }
    if (op.type === 'remove') {
      const obj = this.objects.get(op.id);
      if (obj) {
        this.removed++;
        this.startExit(obj, 0, instant);
      }
      return;
    }
    const existing = this.objects.get(op.id);
    if (!existing) {
      if (!frame.candidateId && this.objects.size >= this.maxObjects) return;
      if (frame.candidateId) this.objectCandidates.set(op.id, frame.candidateId);
      const candidate = frame.candidateId ? this.candidates.get(frame.candidateId) : undefined;
      if (candidate && candidate.shownAt === undefined) candidate.shownAt = candidate.age;
      this.added++;
      this.createObject(op.id, op.building, frame, instant);
      return;
    }
    this.replaced++;
    this.replaceObject(existing, op.building, frame, instant);
  }

  private createObject(id: string, building: AlgoVizBuilding, frame: QueueFrame, instant: boolean): VizObject {
    const desc = describeObject(id, building, hashString(id));
    const ghost = building.temporary ? 1 : 0;
    const obj: VizObject = {
      ...desc,
      pool: null,
      slot: -1,
      placeholder: false,
      shadowSlot: -1,
      iconSlot: -1,
      icon: null,
      target: worldPose(emptyPose(), building, frame),
      from: emptyPose(),
      pose: emptyPose(),
      scale: 1,
      scaleFrom: 1,
      grow: 1,
      lift: 0,
      drop: dropHeight(desc.kind, desc.seed),
      arc: 0,
      alpha: 1,
      alphaFrom: 1,
      flash: 0,
      ghost,
      ghostFrom: ghost,
      ghostTo: ghost,
      highlight: 0,
      anim: null,
      t: 1,
      duration: 1,
      delay: 0,
      landed: true,
      removing: false,
    };
    copyPose(obj.pose, obj.target);
    copyPose(obj.from, obj.target);
    this.allocSlot(obj);
    if (castsShadow(obj.kind)) {
      obj.shadowSlot = this.shadows.alloc();
      this.shadowOwners[obj.shadowSlot] = obj;
    }
    this.objects.set(id, obj);
    const candidate = frame.candidateId ? this.candidates.get(frame.candidateId) : undefined;
    if (candidate) {
      candidate.objects.add(obj);
      this.memberships.set(obj, candidate);
      this.objectCandidates.set(id, candidate.id);
    }
    this.resolveIcon(obj);
    this.linkBelt(obj);
    this.markNeighbors(obj);
    if (!instant && !this.reducedMotion.matches) this.startEnter(obj);
    this.dirty.add(obj);
    this.boundsDirty = true;
    return obj;
  }

  private replaceObject(obj: VizObject, building: AlgoVizBuilding, frame: QueueFrame, instant: boolean): void {
    const desc = describeObject(obj.id, building, obj.seed);
    if (desc.kind !== obj.kind || desc.modelKey !== obj.modelKey) {
      // 换型：旧建筑沉入地面，新建筑在同一身份下重新落位
      const id = obj.id;
      this.startExit(obj, 0, instant);
      this.createObject(id, building, frame, instant);
      return;
    }
    this.unlinkBelt(obj);
    obj.building = building;
    this.resolveIcon(obj);
    if (obj.colorIndex !== desc.colorIndex) {
      obj.colorIndex = desc.colorIndex;
      obj.pool?.set('aColor', obj.slot, obj.colorIndex);
    }
    // 从当前显示位置（含抬升）出发，避免打断入场动画时跳变
    copyPose(obj.from, obj.pose);
    obj.from.y += obj.lift;
    worldPose(obj.target, building, frame);
    obj.ghostFrom = obj.ghost;
    obj.ghostTo = building.temporary ? 1 : 0;
    this.linkBelt(obj);
    this.markNeighbors(obj);
    this.boundsDirty = true;
    const f = obj.from;
    const t = obj.target;
    const distance = Math.hypot(t.x - f.x, t.z - f.z, t.x2 - f.x2, t.z2 - f.z2) + Math.abs(t.y - f.y);
    const turned = Math.abs(shortestAngle(f.yaw, t.yaw)) > 0.01;
    if (instant || this.reducedMotion.matches) {
      this.finishPose(obj);
    } else if (distance > 0.01 || turned || obj.ghostFrom !== obj.ghostTo || obj.anim) {
      this.startMove(obj, distance);
    } else {
      obj.flash = Math.max(obj.flash, 0.35);
      this.startMove(obj, 0);
    }
    this.dirty.add(obj);
  }

  private startEnter(obj: VizObject): void {
    obj.anim = 'enter';
    obj.t = 0;
    obj.delay = 0;
    obj.duration = DUR_ENTER * this.animScale * (obj.kind === 'belt' ? 0.6 : 1);
    obj.landed = false;
    obj.alpha = 0;
    obj.scale = 0.6;
    obj.lift = obj.drop;
    obj.grow = obj.kind === 'belt' ? 0 : 1;
    this.animating.add(obj);
  }

  private startMove(obj: VizObject, distance: number): void {
    obj.anim = 'move';
    obj.t = 0;
    obj.delay = 0;
    obj.duration = DUR_MOVE * this.animScale * (distance > 12 ? 1.25 : 1);
    obj.scaleFrom = obj.scale;
    obj.alphaFrom = obj.alpha;
    obj.lift = 0;
    obj.grow = 1;
    obj.landed = true;
    // 纪念碑谷式机关：先抬起，平移转向，再落下
    obj.arc = obj.kind === 'belt' ? 0 : Math.min(1.8, distance > 0.01 ? 0.35 + distance * 0.07 : 0);
    this.animating.add(obj);
  }

  /** 开始撤销：对象立即脱离身份表（同 ID 可马上重新加入），播放完动画后释放 */
  private startExit(obj: VizObject, delay: number, instant: boolean): void {
    if (obj.removing) return;
    obj.removing = true;
    this.untrackPlaceholder(obj);
    if (this.objects.get(obj.id) === obj) this.objects.delete(obj.id);
    this.unlinkBelt(obj);
    this.markNeighbors(obj);
    if (this.selected === obj) this.select(null);
    if (this.hovered === obj) this.setHovered(null);
    this.boundsDirty = true;
    if (instant || this.reducedMotion.matches) {
      this.destroy(obj);
      return;
    }
    copyPose(obj.from, obj.pose);
    obj.from.y += obj.lift;
    copyPose(obj.pose, obj.from);
    obj.lift = 0;
    obj.anim = 'exit';
    obj.t = 0;
    obj.delay = delay;
    obj.duration = DUR_EXIT * this.animScale;
    obj.scaleFrom = obj.scale;
    obj.alphaFrom = obj.alpha;
    this.animating.add(obj);
  }

  private destroy(obj: VizObject): void {
    this.beltFlow.delete(obj);
    this.memberships.get(obj)?.objects.delete(obj);
    this.memberships.delete(obj);
    this.animating.delete(obj);
    this.glowing.delete(obj);
    this.dirty.delete(obj);
    this.freeSlot(obj);
    this.icons.release(obj);
    if (obj.shadowSlot >= 0) {
      const slot = obj.shadowSlot;
      this.shadowOwners[slot] = undefined;
      const from = this.shadows.release(slot);
      if (from !== undefined) {
        const moved = this.shadowOwners[from];
        if (moved) { moved.shadowSlot = slot; this.shadowOwners[slot] = moved; }
      }
      this.shadowOwners.length = this.shadows.size;
      obj.shadowSlot = -1;
    }
    if (this.objects.get(obj.id) === obj) this.objects.delete(obj.id);
    if (!this.objects.has(obj.id)) this.objectCandidates.delete(obj.id);
    this.unlinkBelt(obj);
  }

  /** 推进单个对象的动画；返回 true 表示本帧有变化 */
  private step(obj: VizObject, dt: number): boolean {
    if (obj.delay > 0) {
      obj.delay -= dt;
      return false;
    }
    obj.t = Math.min(1, obj.t + dt / Math.max(obj.duration, 1e-3));
    const t = obj.t;
    if (obj.anim === 'enter') {
      if (obj.kind === 'belt') {
        obj.grow = easeOutCubic(t);
        obj.alpha = clamp01(t * 4);
        obj.scale = 1;
        obj.lift = 0;
        obj.flash = (1 - t) * 0.5;
      } else {
        obj.alpha = clamp01(t / 0.2);
        obj.lift = obj.drop * (1 - easeOutBack(Math.min(1, t / 0.78), 1.3));
        obj.scale = 0.6 + 0.4 * easeOutBack(t, 1.7);
        if (!obj.landed && t >= 0.6) {
          obj.landed = true;
          if (obj.kind !== 'sorter' && !this.objectCandidates.has(obj.id)) this.rings.spawn(obj.pose.x, obj.pose.y, obj.pose.z, Math.max(obj.width, obj.depth), obj.kind === 'station' ? 1 : 0.75, 0.75 * Math.max(this.animScale, 0.5));
        }
        obj.flash = obj.landed ? (1 - t) * 1.5 : 0;
      }
      obj.ghost = obj.ghostTo;
    } else if (obj.anim === 'move') {
      const e = easeInOutCubic(t);
      const f = obj.from;
      const g = obj.target;
      const p = obj.pose;
      p.x = lerp(f.x, g.x, e);
      p.y = lerp(f.y, g.y, e);
      p.z = lerp(f.z, g.z, e);
      p.x2 = lerp(f.x2, g.x2, e);
      p.y2 = lerp(f.y2, g.y2, e);
      p.z2 = lerp(f.z2, g.z2, e);
      p.yaw = f.yaw + shortestAngle(f.yaw, g.yaw) * e;
      obj.lift = Math.sin(Math.PI * t) * obj.arc;
      obj.scale = lerp(obj.scaleFrom, 1, e);
      obj.alpha = lerp(obj.alphaFrom, 1, e);
      obj.ghost = lerp(obj.ghostFrom, obj.ghostTo, smoothstep(0.3, 1, t));
      // 由临时转为正式：落定时闪一下，像石块被点亮
      const solidify = obj.ghostFrom - obj.ghostTo > 0.5 ? smoothstep(0.5, 0.8, t) * (1 - smoothstep(0.8, 1, t)) : 0;
      obj.flash = Math.max(solidify * 0.9, obj.flash * (1 - Math.min(1, dt * 4)));
    } else if (obj.anim === 'exit') {
      const e = easeInCubic(t);
      if (obj.kind === 'belt') {
        obj.grow = 1 - e;
        obj.lift = 0;
      } else {
        obj.lift = Math.sin(Math.min(1, t * 2.2) * Math.PI * 0.5) * 0.6 - e * 3;
      }
      obj.scale = obj.scaleFrom * (1 - 0.4 * e);
      obj.alpha = obj.alphaFrom * (1 - smoothstep(0.25, 1, t));
      obj.flash = (1 - t) * 0.35;
    }
    if (t >= 1) this.finish(obj);
    return true;
  }

  private finish(obj: VizObject): void {
    if (obj.anim === 'exit') {
      this.destroy(obj);
      return;
    }
    this.finishPose(obj);
    this.animating.delete(obj);
    this.dirty.add(obj);
  }

  private finishPose(obj: VizObject): void {
    this.animating.delete(obj);
    copyPose(obj.pose, obj.target);
    obj.anim = null;
    obj.t = 1;
    obj.delay = 0;
    obj.lift = 0;
    obj.scale = 1;
    obj.grow = 1;
    obj.alpha = 1;
    obj.flash = 0;
    obj.ghost = obj.ghostTo;
    obj.landed = true;
  }

  // -------------------------------------------------------------------------
  // 实例槽位与模型
  // -------------------------------------------------------------------------

  private allocSlot(obj: VizObject): void {
    let key = 'placeholder';
    let geometry = this.unitBox;
    let material: THREE.Material | THREE.Material[] = this.buildingMaterial;
    let specs: InstanceAttributeSpec[] = [STATE_SPEC, FAIL_SPEC];
    let owned = false;
    obj.placeholder = false;
    if (obj.kind === 'belt') {
      key = 'belt';
      geometry = this.beltBox;
      material = this.beltMaterial;
      specs = [STATE_SPEC, COLOR_SPEC, FAIL_SPEC, ...BELT_SPECS];
    } else if (obj.modelKey) {
      const entry = this.models.get(obj.modelKey);
      if (entry.status === 'ready' && entry.geometry) {
        key = obj.modelKey;
        geometry = entry.geometry;
        if (!this.pools.has(key)) {
          // 每个模型一组材质：与几何体分组一一对应，采样游戏原生的底色与自发光贴图
          material = entry.parts.map((part) => this.createBuildingMaterial(part.map, part.emissiveMap, part.color, part.emissive, part.kind, part.orm, part.native, part.mask, part.normalMap, part.unlit));
          owned = true;
        }
        if (obj.kind !== 'sorter') {
          obj.width = entry.size.x;
          obj.depth = entry.size.z;
          obj.height = Math.max(0.5, entry.size.y);
        } else {
          obj.depth = Math.max(0.2, entry.size.z);
        }
      } else {
        obj.placeholder = true;
      }
    } else {
      obj.placeholder = true;
    }
    let pool = this.pools.get(key);
    if (!pool) {
      pool = new InstancePool(key, geometry, material, specs, this.world, key === 'belt' ? 256 : 32);
      this.pools.set(key, pool);
      this.owners.set(key, []);
      if (owned) this.ownedPools.add(pool);
    }
    obj.pool = pool;
    obj.slot = pool.alloc();
    (this.owners.get(key) as (VizObject | undefined)[])[obj.slot] = obj;
    pool.set('aColor', obj.slot, obj.colorIndex);
    if (obj.placeholder && obj.modelKey) {
      let placeholders = this.placeholders.get(obj.modelKey);
      if (!placeholders) this.placeholders.set(obj.modelKey, (placeholders = new Set()));
      placeholders.add(obj);
    }
  }

  private freeSlot(obj: VizObject): void {
    this.untrackPlaceholder(obj);
    if (!obj.pool || obj.slot < 0) return;
    const owners = this.owners.get(obj.pool.key);
    if (owners && owners[obj.slot] === obj) owners[obj.slot] = undefined;
    const from = obj.pool.release(obj.slot);
    if (owners) {
      const moved = from === undefined ? undefined : owners[from];
      if (moved) { moved.slot = obj.slot; owners[obj.slot] = moved; }
      owners.length = obj.pool.size;
    }
    obj.pool = null;
    obj.slot = -1;
  }

  private untrackPlaceholder(obj: VizObject): void {
    if (!obj.modelKey) return;
    const placeholders = this.placeholders.get(obj.modelKey);
    placeholders?.delete(obj);
    if (!placeholders?.size) this.placeholders.delete(obj.modelKey);
  }

  /** 模型加载回调仅登记待迁移类型，避免冷加载时同步重建整场对象。 */
  private onModelSettled(key: string): void {
    if (this.disposed) return;
    if (this.models.get(key).status === 'ready') this.readyModels.add(key);
    this.touch();
  }

  /** 每帧迁移有限占位体；退出和重置同步撤销索引，失败模型保留原占位体。 */
  private updateModelObjects(work: FrameWorkBudget): void {
    let processed = 0;
    for (const key of this.readyModels) {
      const placeholders = this.placeholders.get(key);
      if (placeholders) for (const obj of placeholders) {
        if (processed >= 128 || !work.take()) return;
        processed++;
        this.freeSlot(obj);
        this.allocSlot(obj);
        if (obj.placeholder) break;
        obj.flash = Math.max(obj.flash, 0.5);
        this.glowing.add(obj);
        this.dirty.add(obj);
        this.boundsDirty = true;
      }
      this.readyModels.delete(key);
    }
  }

  // -------------------------------------------------------------------------
  // 传送带连接
  // -------------------------------------------------------------------------

  private linkBelt(obj: VizObject): void {
    this.beltFlow.invalidate(obj);
    for (const input of this.beltInputs.get(obj.id) ?? []) this.beltFlow.invalidate(input);
    const out = obj.kind === 'belt' ? obj.building.outputId : undefined;
    if (!out) return;
    let set = this.beltInputs.get(out);
    if (!set) this.beltInputs.set(out, (set = new Set()));
    set.add(obj);
  }

  private unlinkBelt(obj: VizObject): void {
    for (const input of this.beltInputs.get(obj.id) ?? []) this.beltFlow.invalidate(input);
    const out = obj.kind === 'belt' ? obj.building.outputId : undefined;
    if (!out) return;
    const set = this.beltInputs.get(out);
    if (!set) return;
    set.delete(obj);
    if (!set.size) this.beltInputs.delete(out);
    const target = this.objects.get(out);
    if (target) {
      this.dirty.add(target);
      this.markNeighbors(target);
    }
  }

  /** 对象位置或存在性变化后，重绘与它相连的传送带 */
  private markNeighbors(obj: VizObject): void {
    this.beltGeometry.markVerticalRun(obj, this.objects, this.beltInputs, this.dirty);
    const inputs = this.beltInputs.get(obj.id);
    if (inputs) for (const belt of inputs) {
      this.dirty.add(belt);
      // 输入段的端点截面也依赖本节点的下一段朝向。
      for (const previous of this.beltInputs.get(belt.id) ?? []) this.dirty.add(previous);
    }
    if (obj.kind === 'belt' && obj.building.outputId) {
      const out = this.objects.get(obj.building.outputId);
      if (out && out.kind === 'belt') {
        this.dirty.add(out);
        for (const input of this.beltInputs.get(out.id) ?? []) this.dirty.add(input);
        const next = out.building.outputId ? this.objects.get(out.building.outputId) : undefined;
        if (next?.kind === 'belt') this.dirty.add(next);
      }
    }
  }

  // -------------------------------------------------------------------------
  // 世界图标
  // -------------------------------------------------------------------------

  /** 图标所属分组：传送带节点为 belt，其余建筑为 building */
  private iconGroup(obj: VizObject): AlgoVizIconGroup {
    return obj.kind === 'belt' ? 'belt' : 'building';
  }

  /** 解析对象的图标并交给写入阶段；数据由外部 icon 回调提供，槽位随开关与精灵图状态在写入时分配或释放 */
  private resolveIcon(obj: VizObject): void {
    obj.icon = this.options.icon?.(obj.building) ?? null;
  }

  /**
   * 写入对象上方的图标；分配与释放都由同一个判定驱动，开关切换、精灵图迟到、退场对象都会自行归位。
   * 锚点：建筑为模型顶面上方，传送带为带面上方；跟随入场下落、平移抬升与候选平台偏移。
   */
  private writeIcon(obj: VizObject, candidate: CandidateStage | undefined): void {
    const icon = obj.icon;
    if (!icon || !this.icons.ready || !this.iconVisibility[this.iconGroup(obj)]) {
      if (obj.iconSlot >= 0) this.icons.release(obj);
      return;
    }
    this.icons.alloc(obj);
    const p = obj.pose;
    let top: number;
    let size: number;
    if (obj.kind === 'belt') {
      top = BELT_THICKNESS + 0.1;
      size = ICON_BELT_SIZE;
    } else {
      const entry = obj.modelKey && !obj.placeholder ? this.models.get(obj.modelKey) : null;
      top = (entry?.geometry ? entry.center.y + entry.size.y / 2 : obj.height) * obj.scale + 0.25;
      size = THREE.MathUtils.clamp(Math.min(obj.width, obj.depth) * 0.7, ICON_BUILDING_MIN, ICON_BUILDING_MAX);
    }
    this.icons.set(obj.iconSlot, icon, p.x + (candidate?.x ?? 0), p.y + obj.lift + top + (candidate?.y ?? 0), p.z + (candidate?.z ?? 0), size, obj.alpha * (1 - obj.ghost * 0.4), candidate?.failMix ?? 0);
  }

  // -------------------------------------------------------------------------
  // 写入实例
  // -------------------------------------------------------------------------

  private writeObject(obj: VizObject): void {
    const pool = obj.pool;
    if (!pool || obj.slot < 0) return;
    if (obj.kind === 'belt') this.composeBelt(obj, pool);
    else if (obj.kind === 'sorter') this.composeSorter(obj);
    else this.composeBuilding(obj);
    const candidate = this.memberships.get(obj);
    if (candidate) {
      this.m.elements[12] += candidate.x;
      this.m.elements[13] += candidate.y;
      this.m.elements[14] += candidate.z;
    }
    pool.setMatrix(obj.slot, this.m);
    pool.set('aState', obj.slot, obj.alpha, obj.flash, obj.ghost, obj.highlight);
    pool.set('aFail', obj.slot, candidate?.failMix ?? 0);
    if (obj.shadowSlot >= 0) {
      const lift = Math.max(0, obj.lift);
      const size = 1.12 * obj.scale;
      const strength = obj.alpha * clamp01(1 - lift / 9) * (1 - obj.ghost * 0.55);
      this.shadows.set(obj.shadowSlot, obj.pose.x + (candidate?.x ?? 0), obj.pose.z + (candidate?.z ?? 0), obj.pose.yaw, obj.width * size, obj.depth * size, candidate && candidate.y < -1 ? 0 : strength);
    }
    this.writeIcon(obj, candidate);
  }

  private composeBuilding(obj: VizObject): void {
    const p = obj.pose;
    const k = obj.scale;
    this.v.set(p.x, p.y + obj.lift, p.z);
    this.q.setFromAxisAngle(this.yAxis, p.yaw);
    if (obj.placeholder) this.s.set(obj.width * 0.9 * k, obj.height * k, obj.depth * 0.9 * k);
    else this.s.set(k, k, k);
    this.m.compose(this.v, this.q, this.s);
  }

  /** 分拣器：模型沿 +Z 拉伸到首尾两端之间 */
  private composeSorter(obj: VizObject): void {
    const p = obj.pose;
    const dx = p.x2 - p.x;
    const dz = p.z2 - p.z;
    const length = Math.hypot(dx, dz);
    const yaw = length > 1e-3 ? Math.atan2(dx, dz) : p.yaw;
    const span = Math.max(length, 0.6);
    const k = obj.scale;
    this.v.set((p.x + p.x2) / 2, Math.max(p.y, p.y2) + obj.lift, (p.z + p.z2) / 2);
    this.q.setFromAxisAngle(this.yAxis, yaw);
    if (obj.placeholder) this.s.set(0.4 * k, 0.5 * k, span * k);
    else this.s.set(k, k, (span / obj.depth) * k);
    this.m.compose(this.v, this.q, this.s);
  }

  /**
   * 传送带共享节点截面，升降与转角使用同一斜接边界；箭头沿连接累计相位。
   * 生长只裁短当前带段，不改变下游的动画原点；链首向后补半格。
   */
  private composeBelt(obj: VizObject, pool: InstancePool): void {
    const shape = this.beltGeometry.shape(obj, this.objects, this.beltInputs);
    this.ax.subVectors(shape.end, shape.start);
    this.ay.copy(shape.up).multiplyScalar(BELT_THICKNESS);
    this.az.copy(shape.side).multiplyScalar(BELT_WIDTH * obj.scale);
    this.m.makeBasis(this.ax, this.ay, this.az);
    this.v.addVectors(shape.start, shape.end).multiplyScalar(0.5);
    this.m.setPosition(this.v);
    const flow = this.beltFlow.get(obj);
    pool.set('aFlow', obj.slot, flow.phase - shape.head * flow.frequency,
      (this.beltGeometry.span(obj, this.objects) + shape.head) * flow.frequency * Math.max(obj.grow, 1e-3));
    pool.set('aStartSide', obj.slot, shape.startSide.x, shape.startSide.y, shape.startSide.z);
    pool.set('aStartUp', obj.slot, shape.startUp.x, shape.startUp.y, shape.startUp.z);
    pool.set('aEndSide', obj.slot, shape.endSide.x, shape.endSide.y, shape.endSide.z);
    pool.set('aEndUp', obj.slot, shape.endUp.x, shape.endUp.y, shape.endUp.z);
  }

  // -------------------------------------------------------------------------
  // 取景与状态
  // -------------------------------------------------------------------------

  private recomputeBounds(): void {
    this.boundsDirty = false;
    this.boundsAt = this.time;
    if (this.candidates.size) {
      this.recomputeCandidateBounds();
      return;
    }
    let minX = Infinity;
    let minZ = Infinity;
    let maxX = -Infinity;
    let maxZ = -Infinity;
    for (const obj of this.objects.values()) {
      const t = obj.target;
      const hw = obj.kind === 'belt' ? 0.5 : obj.width / 2;
      const hd = obj.kind === 'belt' ? 0.5 : obj.depth / 2;
      minX = Math.min(minX, t.x - hw, t.x2 - hw);
      maxX = Math.max(maxX, t.x + hw, t.x2 + hw);
      minZ = Math.min(minZ, t.z - hd, t.z2 - hd);
      maxZ = Math.max(maxZ, t.z + hd, t.z2 + hd);
    }
    const empty = !Number.isFinite(minX);
    this.island.setTarget(empty ? null : { minX, minZ, maxX, maxZ });
    if (this.firstFrame) {
      this.island.snap();
      this.firstFrame = false;
    }
    // 取景：逐个对象投影到目标方位的屏幕方向，取极值；平台外沿与檐层一并计入，使平台完整可见
    this.rig.screenAxes(this.ax, this.ay);
    const right = this.ax;
    const up = this.ay;
    let minR = Infinity;
    let maxR = -Infinity;
    let minU = Infinity;
    let maxU = -Infinity;
    const visit = (x: number, y: number, z: number): void => {
      const r = x * right.x + y * right.y + z * right.z;
      const u = x * up.x + y * up.y + z * up.z;
      if (r < minR) minR = r;
      if (r > maxR) maxR = r;
      if (u < minU) minU = u;
      if (u > maxU) maxU = u;
    };
    for (const obj of this.objects.values()) {
      const t = obj.target;
      const hw = obj.kind === 'belt' ? 0.5 : obj.width / 2;
      const hd = obj.kind === 'belt' ? 0.5 : obj.depth / 2;
      const top = t.y + (obj.kind === 'belt' ? 0.3 : obj.height);
      for (const [px, pz] of [[t.x - hw, t.z - hd], [t.x + hw, t.z - hd], [t.x - hw, t.z + hd], [t.x + hw, t.z + hd], [t.x2, t.z2]]) {
        visit(px, t.y, pz);
        visit(px, top, pz);
      }
    }
    const b = this.island.getBounds();
    const goal = empty ? null : { minX: minX - 3.5, minZ: minZ - 3.5, maxX: maxX + 3.5, maxZ: maxZ + 3.5 };
    const ix0 = goal ? goal.minX : b.x;
    const iz0 = goal ? goal.minZ : b.y;
    const ix1 = goal ? goal.maxX : b.z;
    const iz1 = goal ? goal.maxZ : b.w;
    for (const [px, pz] of [[ix0, iz0], [ix1, iz0], [ix0, iz1], [ix1, iz1]]) {
      visit(px, 0.2, pz);
      visit(px, -1.7, pz);
    }
    this.rig.frameProjected(minR, maxR, minU, maxU, this.time < 0.05);
  }

  private recomputeCandidateBounds(): void {
    const ranges = new Map<string, {minX: number; minZ: number; maxX: number; maxZ: number}>();
    for (const obj of this.objects.values()) {
      const id = this.objectCandidates.get(obj.id);
      if (!id) continue;
      const range = ranges.get(id) ?? {minX: Infinity, minZ: Infinity, maxX: -Infinity, maxZ: -Infinity};
      range.minX = Math.min(range.minX, obj.target.x - obj.width / 2, obj.target.x2 - obj.width / 2);
      range.maxX = Math.max(range.maxX, obj.target.x + obj.width / 2, obj.target.x2 + obj.width / 2);
      range.minZ = Math.min(range.minZ, obj.target.z - obj.depth / 2, obj.target.z2 - obj.depth / 2);
      range.maxZ = Math.max(range.maxZ, obj.target.z + obj.depth / 2, obj.target.z2 + obj.depth / 2);
      ranges.set(id, range);
    }
    let span = 32;
    for (const range of ranges.values()) span = Math.max(span, range.maxX - range.minX + 24, range.maxZ - range.minZ + 24);
    this.candidateSpacing = Math.max(this.candidateSpacing, span);
    span = this.candidateSpacing;
    this.rig.screenAxes(this.ax, this.ay);
    let minR = Infinity, maxR = -Infinity, minU = Infinity, maxU = -Infinity;
    // 候选按方阵排布（4 个排 2×2，8 个排 3×3）：列数取候选总数的平方根向上取整，序号稳定故平台位置不随进度变化
    let total = 1;
    for (const candidate of this.candidates.values()) total = Math.max(total, candidate.ordinal);
    const columns = Math.max(1, Math.ceil(Math.sqrt(total)));
    for (const candidate of this.candidates.values()) {
      const range = ranges.get(candidate.id);
      candidate.island.setTarget(range ?? null);
      const bounds = candidate.island.getBounds();
      const centered = this.candidates.size === 1 && candidate.id === this.winnerId;
      const index = candidate.ordinal - 1;
      const targetX = centered ? 0 : (index % columns) * span;
      const targetZ = centered ? 0 : Math.floor(index / columns) * span;
      candidate.targetX = targetX - (bounds.x + bounds.z) / 2;
      candidate.targetZ = targetZ - (bounds.y + bounds.w) / 2;
      if (!candidate.positioned) {
        candidate.x = candidate.targetX;
        candidate.z = candidate.targetZ;
        candidate.positioned = true;
        this.dirtyCandidate(candidate);
      }
      for (const [px, pz] of [[bounds.x, bounds.y], [bounds.z, bounds.y], [bounds.x, bounds.w], [bounds.z, bounds.w]]) {
        for (const height of [-2, 12]) {
          const position = new THREE.Vector3(px + candidate.targetX, height, pz + candidate.targetZ);
          const right = position.dot(this.ax), up = position.dot(this.ay);
          minR = Math.min(minR, right); maxR = Math.max(maxR, right);
          minU = Math.min(minU, up); maxU = Math.max(maxU, up);
        }
      }
    }
    this.rig.frameProjected(minR, maxR, minU, maxU, false);
  }

  /** 只扫描本候选实体，包含同 ID 换型后尚在退场的旧对象。 */
  private candidateAnimating(candidate: CandidateStage): boolean {
    if (!this.animating.size) return false;
    for (const obj of candidate.objects) if (this.animating.has(obj)) return true;
    return false;
  }

  private dirtyCandidate(candidate: CandidateStage): void {
    for (const obj of candidate.objects) this.dirty.add(obj);
  }

  /** 候选必须完整播完过程、快照及动画后才允许统一退场。 */
  private candidatesReady(): boolean {
    for (const candidate of this.candidates.values()) {
      if (candidate.queue.length || candidate.snapshot || this.candidateAnimating(candidate)) return false;
      if (!this.skipping && candidate.age - (candidate.shownAt ?? 0) < CANDIDATE_MIN_SHOW) return false;
    }
    return true;
  }

  /** 候选轮询共用整帧预算，剩余队列与快照原序留到下帧；平台运动只更新所属对象。 */
  private updateCandidates(dt: number, work = new FrameWorkBudget(), playing = true): void {
    const originalQueue = this.queue;
    const originalBudget = this.budget;
    let release = false;
    if (this.settled && this.candidatesReady()) {
      if (this.settleReadyAt < 0) this.settleReadyAt = this.time;
      release = this.skipping || this.reducedMotion.matches || this.time - this.settleReadyAt >= SETTLE_HOLD;
    } else this.settleReadyAt = -1;
    const stages = Array.from(this.candidates.values());
    const first = this.candidateCursor % Math.max(1, stages.length);
    let nextCursor = (first + 1) % Math.max(1, stages.length);
    let budgetReached = false;
    try {
      for (let offset = 0; offset < stages.length; offset++) {
        const candidate = stages[(first + offset) % stages.length];
        if (!playing && !this.retiredCandidates.has(candidate.id)) continue;
        const moving = Math.abs(candidate.x - candidate.targetX) + Math.abs(candidate.z - candidate.targetZ) > 0.001;
        const blend = this.reducedMotion.matches || this.skipping ? 1 : 1 - Math.exp(-dt * 7);
        candidate.x = lerp(candidate.x, candidate.targetX, blend);
        candidate.z = lerp(candidate.z, candidate.targetZ, blend);
        if (moving) this.dirtyCandidate(candidate);
        candidate.age += dt * this.speed;
        this.queue = candidate.queue;
        this.budget = candidate.budget;
        if (candidate.falling < 0) this.processQueue(dt, work);
        candidate.budget = this.budget;
        if (!candidate.queue.length && candidate.snapshot && (candidate.snapshot.started || !this.candidateAnimating(candidate))) this.applyCandidateSnapshot(candidate, this.skipping, work);
        const failed = (candidate.status === 'rejected' || candidate.status === 'pruned') && (this.retiredCandidates.has(candidate.id) || !candidate.queue.length && !candidate.snapshot && !this.candidateAnimating(candidate));
        const failGoal = failed ? 1 : 0;
        if (candidate.failMix !== failGoal) {
          const step = this.reducedMotion.matches || this.skipping ? 1 : dt * 3;
          candidate.failMix += Math.sign(failGoal - candidate.failMix) * Math.min(Math.abs(failGoal - candidate.failMix), step);
          this.dirtyCandidate(candidate);
        }
        candidate.island.setFail(candidate.failMix);
        if (release && candidate.falling < 0 && candidate.id !== this.winnerId) candidate.falling = 0;
        if (candidate.falling >= 0) {
          candidate.falling += dt * this.speed;
          const y = this.reducedMotion.matches || this.skipping ? -10000 : -120 * Math.pow(candidate.falling / 0.8, 2);
          if (candidate.y !== y) { candidate.y = y; this.dirtyCandidate(candidate); }
          if (this.skipping || candidate.falling >= 0.8) {
            let removed = 0;
            for (const obj of candidate.objects) {
              if (removed >= 128 || !work.take()) break;
              if (obj.removing) this.destroy(obj);
              else this.startExit(obj, 0, true);
              removed++;
            }
            if (!candidate.objects.size) {
              this.scene.remove(candidate.island.group);
              candidate.island.dispose();
              this.candidates.delete(candidate.id);
              this.boundsDirty = true;
              this.touch();
              continue;
            }
          }
        }
        candidate.island.group.position.set(candidate.x, candidate.y, candidate.z);
        candidate.island.update(dt, this.time, this.reducedMotion.matches);
        if (!budgetReached && work.exhausted) {
          nextCursor = (first + offset + 1) % stages.length;
          budgetReached = true;
        }
      }
    } finally {
      this.queue = originalQueue;
      this.budget = originalBudget;
      this.candidateCursor = nextCursor;
    }
  }

  /** 快照的索引、撤销与应用分帧执行；尚未完成时保留游标，不能提前宣布候选已播完。 */
  private applyCandidateSnapshot(candidate: CandidateStage, instant: boolean, work: FrameWorkBudget): void {
    const snapshot = candidate.snapshot;
    if (!snapshot) return;
    snapshot.process(candidate.objects, item => this.applyItem(item, instant), work);
    if (snapshot.done) candidate.snapshot = undefined;
    this.touch();
  }

  private setHovered(obj: VizObject | null): void {
    if (obj === this.hovered) return;
    const previous = this.hovered;
    this.hovered = obj;
    if (previous && previous !== this.selected) {
      previous.highlight = 0;
      this.dirty.add(previous);
    }
    if (obj && obj !== this.selected) {
      obj.highlight = 0.5;
      this.dirty.add(obj);
    }
    const text = obj && this.options.describe ? this.options.describe(obj.building) : null;
    this.overlay.showTooltip(text, this.hoverX, this.hoverY);
    this.container.style.cursor = obj ? 'pointer' : '';
  }

  private pick(x: number, y: number): VizObject | null {
    this.ndc.set((x / this.width) * 2 - 1, -(y / this.height) * 2 + 1);
    this.raycaster.setFromCamera(this.ndc, this.rig.camera);
    const meshes: THREE.Object3D[] = [];
    for (const pool of this.pools.values()) if (pool.size) meshes.push(pool.mesh);
    const hits = this.raycaster.intersectObjects(meshes, false);
    for (const hit of hits) {
      const key = hit.object.userData.poolKey as string | undefined;
      const owner = key !== undefined && hit.instanceId !== undefined ? this.owners.get(key)?.[hit.instanceId] : undefined;
      if (owner && !owner.removing) return owner;
    }
    return null;
  }

  /** 标记状态变化：刷新空场景提示并请求发送进度 */
  private touch(): void {
    this.progressVersion++;
    if (this.supported && !this.disposed) {
      this.overlay.setEmpty(this.candidates.size === 0 && this.objects.size === 0 && this.queue.length === 0 && this.animating.size === 0);
    }
  }

  private emitProgress(): void {
    if (!this.options.onProgress || this.progressSent === this.progressVersion) return;
    const now = performance.now();
    if (now - this.progressAt < 100) return;
    this.progressAt = now;
    this.progressSent = this.progressVersion;
    this.options.onProgress(this.getProgress());
  }

  // -------------------------------------------------------------------------
  // 事件
  // -------------------------------------------------------------------------

  private localPoint(e: { clientX: number; clientY: number }): { x: number; y: number } {
    const rect = this.container.getBoundingClientRect();
    return { x: e.clientX - rect.left, y: e.clientY - rect.top };
  }

  private readonly onPointerDown = (e: PointerEvent): void => {
    if (e.target !== this.renderer.domElement || this.pointer) return;
    const { x, y } = this.localPoint(e);
    this.renderer.domElement.setPointerCapture(e.pointerId);
    const panButton = e.button === 1 || e.button === 2 || (e.button === 0 && e.shiftKey);
    this.pointer = { id: e.pointerId, button: e.button, startX: x, startY: y, lastX: x, lastY: y, mode: panButton ? 'pan' : 'pending' };
    if (e.button === 1) e.preventDefault();
  };

  private readonly onPointerMove = (e: PointerEvent): void => {
    const { x, y } = this.localPoint(e);
    const drag = this.pointer;
    if (drag && drag.id === e.pointerId) {
      const dx = x - drag.lastX;
      const dy = y - drag.lastY;
      if (this.viewLocked && drag.mode !== 'pending') drag.mode = 'pending';
      if (!this.viewLocked && drag.mode === 'pending' && Math.hypot(x - drag.startX, y - drag.startY) > DRAG_THRESHOLD) {
        drag.mode = 'rotate';
        this.rig.beginRotate();
        this.setHovered(null);
      }
      if (drag.mode === 'rotate') this.rig.dragRotate(dx);
      else if (drag.mode === 'pan') this.rig.pan(dx, dy);
      drag.lastX = x;
      drag.lastY = y;
      this.container.style.cursor = drag.mode === 'pending' ? '' : 'grabbing';
      return;
    }
    if (e.target !== this.renderer.domElement) return;
    this.hoverX = x;
    this.hoverY = y;
    this.hoverPending = true;
  };

  private readonly onPointerUp = (e: PointerEvent): void => {
    const drag = this.pointer;
    if (!drag || drag.id !== e.pointerId) return;
    this.pointer = null;
    if (this.renderer.domElement.hasPointerCapture(e.pointerId)) this.renderer.domElement.releasePointerCapture(e.pointerId);
    this.container.style.cursor = '';
    if (drag.mode === 'rotate') {
      this.rig.endRotate();
      this.boundsDirty = true;
      return;
    }
    const moved = Math.hypot(drag.lastX - drag.startX, drag.lastY - drag.startY) > DRAG_THRESHOLD;
    if (drag.mode === 'pending' && !moved && drag.button === 0 && e.type === 'pointerup') {
      const hit = this.pick(drag.startX, drag.startY);
      this.select(hit ? hit.id : null);
    }
  };

  private readonly onPointerLeave = (): void => {
    if (!this.pointer) this.setHovered(null);
    this.hoverPending = false;
  };

  private readonly onWheel = (e: WheelEvent): void => {
    if (e.target !== this.renderer.domElement) return;
    e.preventDefault();
    if (this.viewLocked) return;
    const dy = e.deltaY * (e.deltaMode === 1 ? 16 : e.deltaMode === 2 ? 400 : 1);
    const { x, y } = this.localPoint(e);
    this.rig.zoomAt(Math.exp(-dy * 0.0015), x, y);
  };

  private readonly onContextMenu = (e: MouseEvent): void => {
    if (e.target === this.renderer.domElement) e.preventDefault();
  };

  private readonly onDoubleClick = (e: MouseEvent): void => {
    if (e.target === this.renderer.domElement && !this.pick(this.localPoint(e).x, this.localPoint(e).y)) this.resetView();
  };

  private readonly onKeyDown = (e: KeyboardEvent): void => {
    if (e.key === 'Escape') this.select(null);
    else if (e.key === 'ArrowLeft') this.rotate(-1);
    else if (e.key === 'ArrowRight') this.rotate(1);
    else return;
    e.preventDefault();
  };

  private readonly onVisibility = (): void => this.updateRunning();

  private bindEvents(): void {
    const canvas = this.renderer.domElement;
    canvas.addEventListener('pointerdown', this.onPointerDown);
    canvas.addEventListener('pointermove', this.onPointerMove);
    canvas.addEventListener('pointerup', this.onPointerUp);
    canvas.addEventListener('pointercancel', this.onPointerUp);
    canvas.addEventListener('pointerleave', this.onPointerLeave);
    canvas.addEventListener('wheel', this.onWheel, { passive: false });
    canvas.addEventListener('contextmenu', this.onContextMenu);
    canvas.addEventListener('dblclick', this.onDoubleClick);
    canvas.addEventListener('keydown', this.onKeyDown);
    document.addEventListener('visibilitychange', this.onVisibility);
    this.resizeObserver = new ResizeObserver(() => this.onContainerResize());
    this.resizeObserver.observe(this.container);
    this.intersectionObserver = new IntersectionObserver((entries) => {
      this.inView = entries[entries.length - 1].isIntersecting;
      this.updateRunning();
    });
    this.intersectionObserver.observe(this.container);
  }

  private unbindEvents(): void {
    const canvas = this.renderer.domElement;
    canvas.removeEventListener('pointerdown', this.onPointerDown);
    canvas.removeEventListener('pointermove', this.onPointerMove);
    canvas.removeEventListener('pointerup', this.onPointerUp);
    canvas.removeEventListener('pointercancel', this.onPointerUp);
    canvas.removeEventListener('pointerleave', this.onPointerLeave);
    canvas.removeEventListener('wheel', this.onWheel);
    canvas.removeEventListener('contextmenu', this.onContextMenu);
    canvas.removeEventListener('dblclick', this.onDoubleClick);
    canvas.removeEventListener('keydown', this.onKeyDown);
    document.removeEventListener('visibilitychange', this.onVisibility);
    this.resizeObserver.disconnect();
    this.intersectionObserver.disconnect();
  }

  // -------------------------------------------------------------------------
  // 尺寸与循环：容器不可见或页面隐藏时暂停
  // -------------------------------------------------------------------------

  /** 调整画布会清空绘图缓冲，尺寸变化后立即重绘一帧，避免闪烁 */
  private onContainerResize(): void {
    if (this.disposed) return;
    const w = Math.max(this.container.clientWidth, 1);
    const h = Math.max(this.container.clientHeight, 1);
    if (w === this.width && h === this.height) return;
    this.resize();
    this.boundsDirty = true;
    this.boundsAt = -1;
    if (this.running) this.frame(0);
  }

  private resize(): void {
    const w = Math.max(this.container.clientWidth, 1);
    const h = Math.max(this.container.clientHeight, 1);
    const dpr = Math.min(window.devicePixelRatio || 1, this.quality.maxDpr);
    this.width = w;
    this.height = h;
    this.renderer.setPixelRatio(dpr);
    this.renderer.setSize(w, h, false);
    this.rig.setViewport(w, h);
    this.uniforms.uAspect.value = w / h;
    this.uniforms.uPixel.value = dpr;
    this.nextInstantDrawAt = 0;
  }

  private updateRunning(): void {
    const shouldRun = this.inView && !document.hidden && !this.disposed;
    if (shouldRun === this.running) return;
    this.running = shouldRun;
    if (shouldRun) {
      this.lastNow = performance.now();
      this.rafId = requestAnimationFrame(this.tick);
    } else {
      cancelAnimationFrame(this.rafId);
    }
  }

  private readonly tick = (now: number): void => {
    if (!this.running) return;
    this.rafId = requestAnimationFrame(this.tick);
    // 动画步长封顶 0.1 秒保持平滑；队列按真实耗时出队，帧率低时也能跟上算法
    const elapsed = Math.min((now - this.lastNow) / 1000, 1);
    const dt = Math.min(elapsed, 0.1);
    this.lastNow = now;
    if (this.frame(dt, elapsed)) this.fpsFrames++;
    this.fpsTime += elapsed;
    if (this.fpsTime >= 0.5) {
      const info = this.renderer.info.render;
      this.options.onStats?.({ fps: this.fpsFrames / this.fpsTime, drawCalls: info.calls, triangles: info.triangles });
      this.fpsFrames = 0;
      this.fpsTime = 0;
    }
  };

  private frame(dt: number, elapsed = dt): boolean {
    const reduced = this.reducedMotion.matches;
    this.time += dt;
    this.uniforms.uTime.value = this.time;
    this.uniforms.uFlow.value = reduced ? 0 : 1.1;

    if (this.themeT < 1 && this.themeFrom) {
      this.themeT = Math.min(this.themeT + dt / THEME_DURATION, 1);
      lerpPalette(this.palette, this.themeFrom, this.palettes[this.themeName], easeInOutCubic(this.themeT));
      if (this.themeT >= 1) this.themeFrom = null;
    }

    const work = new FrameWorkBudget();
    this.updateModelObjects(work);
    const animDt = this.paused && !this.skipping ? 0 : dt;
    if (!this.paused || this.skipping) {
      this.processQueue(elapsed, work);
      this.updateCandidates(dt, work);
    } else if (Array.from(this.candidates.values()).some(candidate => candidate.falling >= 0)) {
      // 暂停只冻结有效候选的演示，不能阻止后端已淘汰候选释放资源。
      this.updateCandidates(dt, work, false);
    }
    if (this.animating.size) {
      for (const obj of this.animating) {
        if (this.skipping) {
          if (!work.take()) break;
          this.finish(obj);
          continue;
        }
        if (this.step(obj, animDt)) {
          this.dirty.add(obj);
          if (!obj.removing) this.markNeighbors(obj);
        }
      }
      if (!this.animating.size) this.touch();
    }
    // 模型到达后的闪白：对象已不在动画中时由这里衰减，否则闪白会一直残留成一层白光
    if (this.glowing.size) {
      const keep = 1 - Math.min(1, animDt * 4);
      for (const obj of Array.from(this.glowing)) {
        if (obj.anim) {
          this.glowing.delete(obj);
          continue;
        }
        obj.flash *= keep;
        if (obj.flash < 0.01) {
          obj.flash = 0;
          this.glowing.delete(obj);
        }
        this.dirty.add(obj);
      }
    }
    if (this.selected) {
      this.selected.highlight = 0.55 + Math.sin(this.time * 5) * 0.25;
      this.dirty.add(this.selected);
    }
    this.beltFlow.update(this.objects, this.beltInputs, this.dirty);
    this.beltGeometry.beginFrame();
    for (const obj of this.dirty) if (obj.pool) this.writeObject(obj);
    this.dirty.clear();
    for (const pool of this.pools.values()) pool.commit();
    this.shadows.commit();
    this.icons.setView(this.rig.worldPerPixel);
    this.icons.commit();
    this.rings.update(animDt);

    const instantPending = this.skipping && (this.queue.length > 0 || this.animating.size > 0 || this.readyModels.size > 0
      || Array.from(this.candidates.values()).some(candidate => candidate.queue.length > 0 || candidate.snapshot || candidate.falling >= 0));
    // 慢设备的 GPU/驱动阻塞会落在下一次 RAF 间隔里；跳过期间给队列留出计算窗口，避免每次慢绘制后只处理一小批。
    // 下一帧没有绘制便不再推迟，窗口最长半秒；完成帧始终立即绘制，resize/主题强制刷新仍可把截止清零。
    if (instantPending && this.drewLastFrame && this.nextInstantDrawAt > 0) {
      const cooldown = Math.min(INSTANT_MAX_DRAW_COOLDOWN_MS, Math.max(this.lastDrawDurationMs, elapsed * 1000) * 2);
      this.nextInstantDrawAt = Math.max(this.nextInstantDrawAt, performance.now() + cooldown);
    }
    const draw = !instantPending || performance.now() >= this.nextInstantDrawAt;
    if (draw && this.boundsDirty && (this.boundsAt < 0 || this.time - this.boundsAt > 0.12)) this.recomputeBounds();
    this.island.update(dt, this.time, reduced);
    this.rig.update(dt);

    if (draw && this.hoverPending && !this.pointer) {
      this.hoverPending = false;
      this.setHovered(this.pick(this.hoverX, this.hoverY));
      if (this.hovered) this.overlay.showTooltip(this.options.describe ? this.options.describe(this.hovered.building) : null, this.hoverX, this.hoverY);
    }

    if (draw) {
      const drawStart = performance.now();
      this.renderer.render(this.scene, this.rig.camera);
      this.lastDrawDurationMs = performance.now() - drawStart;
      this.nextInstantDrawAt = performance.now() + INSTANT_DRAW_INTERVAL_MS;
    }
    this.drewLastFrame = draw;
    this.emitProgress();
    return draw;
  }
}
