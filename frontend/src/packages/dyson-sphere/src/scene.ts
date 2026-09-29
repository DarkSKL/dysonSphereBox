import * as THREE from 'three';
import { DEFAULT_INTERACTION, DEFAULT_PARAMS, QUALITY_PRESETS } from './defaults';
import { isAchievementUnlocked } from './achievement';
import { acquireStyles, releaseStyles } from './styles';
import { copyAchievement, formatNow, normalizeAchievements } from './internal/data';
import { mergeInteraction, mergeParams } from './internal/params';
import { clonePalette, lerpPalette, PALETTES, resolvePalette, type Palette } from './internal/palette';
import { damp, easeInOutCubic } from './internal/utils';
import { Overlay, type ScreenPoint } from './internal/overlay';
import { BackgroundLayer } from './layers/background';
import { StarLayer } from './layers/star';
import { ShellLayer } from './layers/shell';
import { SwarmLayer } from './layers/swarm';
import { AchievementLayer, ACHIEVEMENT_OUTER_RADIUS } from './layers/achievements';
import { PostProcess } from './layers/postprocess';
import type {
  DysonAchievement,
  DysonInteraction,
  DysonSphereController,
  DysonSphereOptions,
  DysonSphereParams,
  DysonTheme,
} from './types';

// 构图：恒星位于横幅宽度 64% 处，左侧留白供叠加标题
const FOCUS_X = 0.64;
const FOV = 30;
const BASE_ELEVATION = 0.3; // 相机俯视角（弧度）
// 默认取景：相机最近距离；右侧至少保留最外层成就轨道外缘的宽度
const BASE_MIN_DISTANCE = 10.5;
const FRAME_HALF_WIDTH = ACHIEVEMENT_OUTER_RADIUS - 0.08;
// 滚轮缩放：相机距离倍率范围；最近不进入最外层成就轨道
const ZOOM_MIN = 0.5;
const ZOOM_MAX = 1.8;
const MIN_DISTANCE = 6.2;
const WHEEL_SPEED = 0.0012;
// 中键拖拽旋转：每像素旋转弧度、俯仰角上限、松开后复位速度
const DRAG_SPEED = 0.006;
const ELEVATION_LIMIT = 1.3;
const DRAG_RETURN = 3.5;
const THEME_DURATION = 0.4;

const CANVAS_LABEL = '戴森球动画：恒星外围正在建造的戴森壳、环绕的太阳帆粒子云与成就节点';
const FALLBACK_LABEL = '戴森球动画（当前环境不支持 WebGL，显示静态画面）';

/**
 * 戴森球场景实现。不对外导出，使用方通过 createDysonSphere 获得 DysonSphereController。
 *
 * 结构：
 * - 状态（参数、主题、交互开关、成就列表）在 WebGL 不可用时依然有效，getter 始终可用
 * - 渲染相关字段只在 supported 为 true 时初始化
 */
export class DysonSphereScene implements DysonSphereController {
  readonly supported: boolean;

  private readonly container: HTMLElement;
  private readonly options: DysonSphereOptions;
  private params: DysonSphereParams;
  private themeName: DysonTheme;
  private interaction: DysonInteraction;
  private achievementList: DysonAchievement[] = [];
  private disposed = false;
  private stylesAcquired = false;
  private fallback: HTMLDivElement | null = null;

  // --- 渲染 ---
  private renderer!: THREE.WebGLRenderer;
  private scene!: THREE.Scene;
  private camera!: THREE.PerspectiveCamera;
  private background!: BackgroundLayer;
  private star!: StarLayer;
  private shell!: ShellLayer;
  private swarm!: SwarmLayer;
  private achievements!: AchievementLayer;
  private post!: PostProcess;
  private overlay!: Overlay;

  // --- 调色板：palette 为当前插值结果，主题切换时从 themeFrom 过渡到目标主题 ---
  private palettes!: Record<DysonTheme, Palette>;
  private palette!: Palette;
  private themeFrom: Palette | null = null;
  private themeT = 1;

  // --- 交互状态 ---
  private readonly pointer = new THREE.Vector2();
  private pointerInside = false;
  private pointerHover = -1;
  private focusHover = -1;
  private hovered = -1;
  private selected = -1;
  private readonly parallax = new THREE.Vector2();
  private azimuth = -0.5;
  private zoom = 1;
  private zoomTarget = 1;
  // 中键拖拽：orbitDrag 记录拖拽中的指针；dragAz / dragEl 为叠加在自动视角上的旋转偏移
  private orbitDrag: { id: number; x: number; y: number } | null = null;
  private dragAz = 0;
  private dragEl = 0;
  private readonly raycaster = new THREE.Raycaster();
  private readonly plane = new THREE.Plane();
  private readonly hitPoint = new THREE.Vector3();
  private readonly _v = new THREE.Vector3();

  // --- 尺寸与循环 ---
  private width = 1;
  private height = 1;
  private distance = BASE_MIN_DISTANCE;
  private time = 0;
  private running = false;
  private inView = true;
  private rafId = 0;
  private lastNow = 0;
  private fpsFrames = 0;
  private fpsTime = 0;

  private reducedMotion!: MediaQueryList;
  private resizeObserver!: ResizeObserver;
  private intersectionObserver!: IntersectionObserver;

  constructor(container: HTMLElement, options: DysonSphereOptions = {}) {
    if (!container) throw new Error('[dyson-sphere] 需要传入容器元素');
    this.container = container;
    this.options = options;
    this.params = mergeParams(DEFAULT_PARAMS, options.params ?? {});
    this.themeName = options.theme && PALETTES[options.theme] ? options.theme : 'dark';
    this.interaction = mergeInteraction(DEFAULT_INTERACTION, options.interaction ?? {});
    this.achievementList = normalizeAchievements(options.achievements ?? []);

    if (options.injectStyles !== false) {
      acquireStyles();
      this.stylesAcquired = true;
    }
    container.classList.add('dyson-root');
    container.dataset.theme = this.themeName;

    let renderer: THREE.WebGLRenderer;
    try {
      renderer = new THREE.WebGLRenderer({ antialias: false, powerPreference: 'high-performance' });
    } catch (err) {
      console.warn('[dyson-sphere] WebGL 不可用，使用静态降级画面', err);
      this.supported = false;
      this.showFallback();
      return;
    }
    this.supported = true;
    this.renderer = renderer;
    this.initRenderer();
  }

  // -------------------------------------------------------------------------
  // 公共接口（DysonSphereController）
  // -------------------------------------------------------------------------

  setParams(patch: Partial<DysonSphereParams>): void {
    if (this.disposed) return;
    const prev = this.params;
    this.params = mergeParams(prev, patch);
    if (this.supported) this.applyParams(prev);
  }

  getParams(): DysonSphereParams {
    return { ...this.params };
  }

  setTheme(theme: DysonTheme, immediate = false): void {
    if (this.disposed || !PALETTES[theme]) return;
    this.themeName = theme;
    this.container.dataset.theme = theme;
    if (!this.supported) return;
    this.overlay.setTheme(theme);
    if (immediate) {
      this.palette = clonePalette(this.palettes[theme]);
      this.themeT = 1;
      this.applyPaletteToLayers();
    } else {
      this.themeFrom = clonePalette(this.palette);
      this.themeT = 0;
    }
  }

  getTheme(): DysonTheme {
    return this.themeName;
  }

  setInteraction(patch: Partial<DysonInteraction>): void {
    if (this.disposed) return;
    this.interaction = mergeInteraction(this.interaction, patch);
    if (!this.interaction.orbitDrag && this.orbitDrag) this.endOrbitDrag();
  }

  getInteraction(): DysonInteraction {
    return { ...this.interaction };
  }

  setAchievements(list: readonly DysonAchievement[]): void {
    if (this.disposed) return;
    this.achievementList = normalizeAchievements(list);
    if (this.supported) this.rebuildAchievements();
  }

  getAchievements(): DysonAchievement[] {
    return this.achievementList.map(copyAchievement);
  }

  unlockAchievement(id: string, unlockedAt?: string): boolean {
    if (this.disposed) return false;
    const index = this.indexOf(id);
    const a = this.achievementList[index];
    if (!a || isAchievementUnlocked(a)) return false;
    a.current = a.threshold;
    a.unlockedAt = unlockedAt ?? formatNow();
    if (this.supported) {
      this.achievements.playUnlock(index);
      this.overlay.refresh(index, true);
    }
    return true;
  }

  selectAchievement(id: string | null): void {
    if (this.disposed || !this.supported) return;
    if (id === null) {
      this.select(-1);
      return;
    }
    const index = this.indexOf(id);
    if (index >= 0) this.select(index);
  }

  replayBuild(): void {
    if (this.disposed || !this.supported) return;
    this.shell.replay();
  }

  resetView(): void {
    if (this.disposed || !this.supported) return;
    this.zoomTarget = 1;
    if (this.orbitDrag) this.endOrbitDrag();
  }

  dispose(): void {
    if (this.disposed) return;
    this.disposed = true;
    if (this.supported) {
      this.running = false;
      cancelAnimationFrame(this.rafId);
      this.unbindEvents();
      this.overlay.dispose();
      this.background.dispose();
      this.star.dispose();
      this.shell.dispose();
      this.swarm.dispose();
      this.achievements.dispose();
      this.post.dispose();
      this.renderer.dispose();
      this.renderer.domElement.remove();
      this.container.style.cursor = '';
    }
    this.fallback?.remove();
    this.container.classList.remove('dyson-root');
    delete this.container.dataset.theme;
    if (this.stylesAcquired) releaseStyles();
  }

  // -------------------------------------------------------------------------
  // 初始化
  // -------------------------------------------------------------------------

  private initRenderer(): void {
    const renderer = this.renderer;
    renderer.toneMapping = THREE.NoToneMapping;
    const canvas = renderer.domElement;
    canvas.setAttribute('role', 'img');
    canvas.setAttribute('aria-label', CANVAS_LABEL);
    this.container.prepend(canvas);

    this.scene = new THREE.Scene();
    this.camera = new THREE.PerspectiveCamera(FOV, 1, 0.1, 200);

    this.background = new BackgroundLayer();
    this.star = new StarLayer();
    this.shell = new ShellLayer();
    this.swarm = new SwarmLayer();
    this.achievements = new AchievementLayer();
    this.scene.add(this.background.group, this.star.group, this.shell.group, this.swarm.group, this.achievements.group);
    this.post = new PostProcess(renderer, this.scene, this.camera);

    this.overlay = new Overlay(this.container, {
      onHover: (i) => (this.focusHover = i),
      onSelect: (i, viaKeyboard) => this.select(i, viaKeyboard),
      onClose: () => this.select(-1),
    });
    this.overlay.setTheme(this.themeName);

    this.palettes = { dark: resolvePalette(PALETTES.dark), light: resolvePalette(PALETTES.light) };
    this.palette = clonePalette(this.palettes[this.themeName]);

    this.reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)');
    this.bindEvents();

    this.applyParams(null);
    this.rebuildAchievements();
    this.applyPaletteToLayers();
    this.resize();
    this.updateRunning();
  }

  private showFallback(): void {
    const div = document.createElement('div');
    div.className = 'dyson-fallback';
    div.setAttribute('role', 'img');
    div.setAttribute('aria-label', FALLBACK_LABEL);
    div.textContent = '当前环境不支持 WebGL，已显示静态画面';
    this.container.append(div);
    this.fallback = div;
  }

  // -------------------------------------------------------------------------
  // 参数、主题与数据
  // -------------------------------------------------------------------------

  /** 把变化的参数应用到各图层；prev 为 null 时全部应用 */
  private applyParams(prev: DysonSphereParams | null): void {
    const p = this.params;
    const changed = (k: keyof DysonSphereParams) => prev === null || prev[k] !== p[k];

    if (changed('quality')) {
      const q = QUALITY_PRESETS[p.quality];
      this.post.setEnabled(q.bloom);
      this.post.setSamples(q.samples);
      if (prev !== null) this.resize();
    }
    if (changed('starCount')) this.background.setStarCount(p.starCount);
    if (changed('nebulaIntensity')) this.background.setNebulaIntensity(p.nebulaIntensity);
    if (changed('starColor')) this.star.setColor(p.starColor);
    if (changed('starPulse')) this.star.setPulse(p.starPulse);
    if (changed('shellLevel')) this.shell.setLevel(p.shellLevel);
    if (changed('progress')) this.shell.setProgress(p.progress);
    if (changed('particleCount') || changed('orbitLayers')) this.swarm.setConfig(p.particleCount, p.orbitLayers);
    if (changed('attractStrength') || changed('attractRadius') || changed('returnSpeed')) {
      this.swarm.setAttractorParams(p.attractStrength, p.attractRadius, p.returnSpeed);
    }
    if (changed('bloomStrength') || changed('bloomThreshold')) this.post.setBloom(p.bloomStrength, p.bloomThreshold);
  }

  private rebuildAchievements(): void {
    const hadSelection = this.selected >= 0;
    this.selected = -1;
    this.hovered = -1;
    this.pointerHover = -1;
    this.focusHover = -1;
    this.achievements.setAchievements(this.achievementList);
    this.overlay.setAchievements(this.achievementList);
    if (hadSelection) this.options.onSelect?.(null);
  }

  private applyPaletteToLayers(): void {
    const p = this.palette;
    this.background.applyPalette(p);
    this.star.applyPalette(p);
    this.shell.applyPalette(p);
    this.swarm.applyPalette(p);
    this.achievements.applyPalette(p);
    this.post.applyPalette(p);
  }

  private indexOf(id: string): number {
    return this.achievementList.findIndex((a) => a.id === id);
  }

  private select(index: number, viaKeyboard = false): void {
    if (index >= this.achievementList.length) index = -1;
    const changed = index !== this.selected;
    this.selected = index;
    this.achievements.setSelected(index);
    if (index >= 0) this.overlay.openCard(index, viaKeyboard);
    else this.overlay.closeCard();
    if (changed) {
      const a = this.achievementList[index];
      this.options.onSelect?.(a ? copyAchievement(a) : null);
    }
  }

  // -------------------------------------------------------------------------
  // 事件
  // -------------------------------------------------------------------------

  private readonly onPointerMove = (e: PointerEvent): void => {
    const rect = this.container.getBoundingClientRect();
    this.pointer.set(((e.clientX - rect.left) / rect.width) * 2 - 1, -((e.clientY - rect.top) / rect.height) * 2 + 1);
    this.pointerInside = e.target === this.renderer.domElement;
    const drag = this.orbitDrag;
    if (drag && e.pointerId === drag.id) {
      // 抓取式旋转：向右拖，戴森球表面向右转；向下拖，从更高处俯视
      this.dragAz -= (e.clientX - drag.x) * DRAG_SPEED;
      this.dragEl = THREE.MathUtils.clamp(
        this.dragEl + (e.clientY - drag.y) * DRAG_SPEED,
        -ELEVATION_LIMIT - BASE_ELEVATION,
        ELEVATION_LIMIT - BASE_ELEVATION,
      );
      drag.x = e.clientX;
      drag.y = e.clientY;
    }
  };

  private readonly onPointerLeave = (): void => {
    this.pointerInside = false;
    this.pointerHover = -1;
  };

  private readonly onPointerDown = (e: PointerEvent): void => {
    if (e.button !== 1 || !this.interaction.orbitDrag || e.target !== this.renderer.domElement) return;
    e.preventDefault();
    this.renderer.domElement.setPointerCapture(e.pointerId);
    this.orbitDrag = { id: e.pointerId, x: e.clientX, y: e.clientY };
  };

  private readonly onPointerUp = (e: PointerEvent): void => {
    if (this.orbitDrag && e.pointerId === this.orbitDrag.id) this.endOrbitDrag();
  };

  // 阻止中键按下时浏览器的自动滚动
  private readonly onMouseDown = (e: MouseEvent): void => {
    if (e.button === 1 && this.interaction.orbitDrag && e.target === this.renderer.domElement) e.preventDefault();
  };

  private readonly onWheel = (e: WheelEvent): void => {
    if (!this.interaction.wheelZoom || e.target !== this.renderer.domElement) return;
    e.preventDefault();
    // 统一滚动单位：行 / 页模式换算为像素
    const dy = e.deltaY * (e.deltaMode === 1 ? 16 : e.deltaMode === 2 ? 400 : 1);
    this.zoomTarget = THREE.MathUtils.clamp(this.zoomTarget * Math.exp(dy * WHEEL_SPEED), ZOOM_MIN, ZOOM_MAX);
  };

  private readonly onClick = (e: MouseEvent): void => {
    // 只处理画布上的点击；卡片、隐藏按钮的点击自行处理
    if (e.target !== this.renderer.domElement) return;
    // 点击空白处：关闭卡片，同时在点击位置把聚拢的粒子炸开
    if (this.pointerHover < 0 && this.interaction.pointerAttract && !this.reducedMotion.matches) {
      const hit = this.pointerOnPlane();
      if (hit) this.swarm.burst(hit);
    }
    this.select(this.pointerHover);
  };

  private readonly onKeyDown = (e: KeyboardEvent): void => {
    if (e.key === 'Escape' && this.selected >= 0) this.select(-1);
  };

  private readonly onVisibility = (): void => this.updateRunning();

  private bindEvents(): void {
    const root = this.container;
    const canvas = this.renderer.domElement;
    root.addEventListener('pointermove', this.onPointerMove);
    root.addEventListener('pointerleave', this.onPointerLeave);
    root.addEventListener('click', this.onClick);
    canvas.addEventListener('pointerdown', this.onPointerDown);
    canvas.addEventListener('pointerup', this.onPointerUp);
    canvas.addEventListener('pointercancel', this.onPointerUp);
    canvas.addEventListener('lostpointercapture', this.onPointerUp);
    canvas.addEventListener('mousedown', this.onMouseDown);
    canvas.addEventListener('wheel', this.onWheel, { passive: false });
    window.addEventListener('keydown', this.onKeyDown);
    document.addEventListener('visibilitychange', this.onVisibility);

    this.resizeObserver = new ResizeObserver(() => this.resize());
    this.resizeObserver.observe(root);
    this.intersectionObserver = new IntersectionObserver((entries) => {
      this.inView = entries[entries.length - 1].isIntersecting;
      this.updateRunning();
    });
    this.intersectionObserver.observe(root);
  }

  private unbindEvents(): void {
    const root = this.container;
    const canvas = this.renderer.domElement;
    root.removeEventListener('pointermove', this.onPointerMove);
    root.removeEventListener('pointerleave', this.onPointerLeave);
    root.removeEventListener('click', this.onClick);
    canvas.removeEventListener('pointerdown', this.onPointerDown);
    canvas.removeEventListener('pointerup', this.onPointerUp);
    canvas.removeEventListener('pointercancel', this.onPointerUp);
    canvas.removeEventListener('lostpointercapture', this.onPointerUp);
    canvas.removeEventListener('mousedown', this.onMouseDown);
    canvas.removeEventListener('wheel', this.onWheel);
    window.removeEventListener('keydown', this.onKeyDown);
    document.removeEventListener('visibilitychange', this.onVisibility);
    this.resizeObserver.disconnect();
    this.intersectionObserver.disconnect();
  }

  /** 结束中键拖拽：旋转偏移归一到 [-π, π]，沿最短路径复位 */
  private endOrbitDrag(): void {
    this.orbitDrag = null;
    this.dragAz = Math.atan2(Math.sin(this.dragAz), Math.cos(this.dragAz));
  }

  /** 鼠标射线与引力平面（过恒星中心、垂直于视线）的交点 */
  private pointerOnPlane(): THREE.Vector3 | null {
    this.raycaster.setFromCamera(this.pointer, this.camera);
    this.camera.getWorldDirection(this._v);
    this.plane.setFromNormalAndCoplanarPoint(this._v, this.star.group.position);
    return this.raycaster.ray.intersectPlane(this.plane, this.hitPoint);
  }

  /** 每帧更新鼠标拾取与引力点 */
  private updatePointer(): void {
    // 中键旋转视角时不拾取、不吸引，避免旋转过程中粒子和提示框跟着鼠标乱动
    if (!this.pointerInside || this.orbitDrag) {
      this.pointerHover = -1;
      this.swarm.setAttractor(null, false);
    } else {
      this.raycaster.setFromCamera(this.pointer, this.camera);
      this.pointerHover = this.achievements.pick(this.raycaster);
      const attract = this.interaction.pointerAttract && !this.reducedMotion.matches;
      const hit = this.pointerOnPlane();
      // 悬停成就节点时不吸引，避免节点周围粒子聚拢干扰阅读
      this.swarm.setAttractor(hit, attract && hit !== null && this.pointerHover < 0);
    }
    this.container.style.cursor = this.orbitDrag ? 'grabbing' : this.pointerHover >= 0 ? 'pointer' : '';

    const hovered = this.pointerHover >= 0 ? this.pointerHover : this.focusHover;
    if (hovered !== this.hovered) {
      this.hovered = hovered;
      this.achievements.setHovered(hovered);
      this.overlay.showTooltip(hovered);
    }
  }

  // -------------------------------------------------------------------------
  // 尺寸与相机
  // -------------------------------------------------------------------------

  private resize(): void {
    const w = Math.max(this.container.clientWidth, 1);
    const h = Math.max(this.container.clientHeight, 1);
    const q = QUALITY_PRESETS[this.params.quality];
    const dpr = Math.min(window.devicePixelRatio || 1, q.maxDpr);
    this.width = w;
    this.height = h;

    this.renderer.setPixelRatio(dpr);
    this.renderer.setSize(w, h, false);
    this.post.setSize(w, h, dpr);

    const aspect = w / h;
    this.camera.aspect = aspect;
    // 把画面中心平移到恒星右侧，使恒星落在 FOCUS_X 处
    this.camera.setViewOffset(w, h, -(FOCUS_X - 0.5) * w, 0, w, h);
    // 纵向保证壳层与轨道完整；横幅过窄时拉远相机，保证右侧成就轨道不被裁掉
    const tanHalf = Math.tan(THREE.MathUtils.degToRad(FOV / 2));
    this.distance = Math.max(BASE_MIN_DISTANCE, FRAME_HALF_WIDTH / (tanHalf * aspect * (1 - FOCUS_X) * 2));
    this.camera.updateProjectionMatrix();

    // 世界尺寸点精灵的像素换算系数（drawing buffer 像素）
    const pointScale = (h * dpr) / (2 * tanHalf);
    // 距离 1 处一个 CSS 像素对应的世界长度，用于按像素宽度绘制轨道带
    const pixelWorld = (2 * tanHalf) / h;
    this.shell.setViewport(pointScale);
    this.swarm.setViewport(pointScale, pixelWorld);
    this.achievements.setViewport(pointScale);
    this.background.setViewport(w, h, dpr, FOCUS_X);
  }

  private updateCamera(dt: number, motion: number): void {
    const parallaxOn = this.pointerInside && !this.reducedMotion.matches && !this.orbitDrag;
    this.parallax.x = damp(this.parallax.x, parallaxOn ? this.pointer.x : 0, 2.2, dt);
    this.parallax.y = damp(this.parallax.y, parallaxOn ? this.pointer.y : 0, 2.2, dt);

    // 松开中键后旋转偏移平滑归零，回到自动视角
    if (!this.orbitDrag) {
      this.dragAz = damp(this.dragAz, 0, DRAG_RETURN, dt);
      this.dragEl = damp(this.dragEl, 0, DRAG_RETURN, dt);
    }
    this.zoom = damp(this.zoom, this.zoomTarget, 8, dt);

    this.azimuth += dt * 0.03 * this.params.spinSpeed * motion;
    const az = this.azimuth + this.dragAz + this.parallax.x * 0.1;
    const el = THREE.MathUtils.clamp(BASE_ELEVATION + this.dragEl - this.parallax.y * 0.05, -ELEVATION_LIMIT, ELEVATION_LIMIT);
    const d = Math.max(this.distance * this.zoom, MIN_DISTANCE);
    this.post.setViewScale(THREE.MathUtils.clamp(Math.pow(d / this.distance, 1.3), 0.35, 1));
    this.camera.position.set(d * Math.cos(el) * Math.sin(az), d * Math.sin(el), d * Math.cos(el) * Math.cos(az));
    this.camera.lookAt(0, 0, 0);
    this.camera.updateMatrixWorld();
  }

  /** 成就节点在容器内的屏幕坐标 */
  private screenOf = (index: number): ScreenPoint | null => {
    const v = this.achievements.getWorldPosition(index, this._v);
    if (!v) return null;
    v.project(this.camera);
    return {
      x: ((v.x + 1) / 2) * this.width,
      y: ((1 - v.y) / 2) * this.height,
      visible: v.z < 1 && Math.abs(v.x) < 1.02 && Math.abs(v.y) < 1.02,
    };
  };

  // -------------------------------------------------------------------------
  // 渲染循环：容器不可见或页面隐藏时暂停
  // -------------------------------------------------------------------------

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
    const dt = Math.min((now - this.lastNow) / 1000, 0.1);
    this.lastNow = now;
    this.frame(dt);

    this.fpsFrames++;
    this.fpsTime += dt;
    if (this.fpsTime >= 0.5) {
      this.options.onStats?.({ fps: this.fpsFrames / this.fpsTime });
      this.fpsFrames = 0;
      this.fpsTime = 0;
    }
  };

  private frame(dt: number): void {
    // prefers-reduced-motion：整体降速，关闭吸引、视差与壳板飞行
    const reduced = this.reducedMotion.matches;
    const motion = reduced ? 0.2 : 1;
    this.time += dt * motion;
    const t = this.time;
    const spin = this.params.spinSpeed * motion;

    if (this.themeT < 1 && this.themeFrom) {
      this.themeT = Math.min(this.themeT + dt / THEME_DURATION, 1);
      lerpPalette(this.palette, this.themeFrom, this.palettes[this.themeName], easeInOutCubic(this.themeT));
      this.applyPaletteToLayers();
    }

    this.updateCamera(dt, motion);
    this.updatePointer();

    this.background.update(t, {
      x: (this.azimuth + this.dragAz) * 0.8 + this.parallax.x,
      y: this.parallax.y - this.dragEl * 0.8,
    });
    this.star.update(t, this.camera, motion);
    this.shell.update(dt, t, spin, this.camera, reduced);
    this.swarm.update(dt, t, this.camera, spin, this.interaction.pointerAttract && !reduced);
    this.achievements.update(dt, t, this.camera, spin);

    this.post.render(dt);
    this.overlay.update(this.screenOf, this.width, this.height);
  }
}
