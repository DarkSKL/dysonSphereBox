import * as THREE from 'three';
import { applyPremultBlending, damp, hemiMaterials, hemiPair, mulberry32, RENDER_ORDER, type HemiMaterials } from '../../internal/utils';
import type { Palette } from '../../internal/palette';
import { buildBeamGeometry, buildGeodesic, buildGoldberg, computeOrder, type GoldbergTile } from './geometry';
import {
  AURA_EXTENT,
  AURA_FRAG,
  AURA_VERT,
  BEAM_VERT,
  FRAME_FRAG,
  LATTICE_FRAG,
  MOTE_FRAG,
  MOTE_VERT,
  NODE_FRAG,
  NODE_VERT,
  SPARK_FRAG,
  SPARK_VERT,
  TILE_FRAG,
  TILE_VERT,
} from './shaders';

export const SHELL_RADIUS = 1.9;
const LATTICE_RADIUS = SHELL_RADIUS * 1.17;
const MOTE_COUNT = 700;

// 细分级别 1~5 对应的测地线频率；六边形壳板数 = 10 × 频率² + 2（含 12 块五边形）
const LEVEL_FREQ = [3, 4, 6, 8, 10];

// 节点先亮，框架随后，壳板最后飞来
const LAG = { node: 0.0, frame: 0.08, cell: 0.18 };
const BAND = { node: 0.03, frame: 0.05 };

// 壳板飞行：从球心到壳板的延长线外侧出发，沿径向飞回框架
const FLY_DURATION = 1.7; // 秒
const FLY_FAR = SHELL_RADIUS * 2.5; // 出发点距球心的距离
const FLY_LATERAL = SHELL_RADIUS * 0.9; // 出发时的侧向偏移，飞行路径略带弧度
const SPARKS_PER_TILE = 6;
const SPARK_LAG = 0.055; // 相邻拖尾粒子的时间滞后（秒）

/** 把总进度映射到子图层的局部进度，保证 P=0 时全灭、P=1 时全亮 */
function layerProgress(P: number, lag: number, band: number): number {
  return P * (1 + lag + band) - lag;
}

type Uniforms = Record<string, THREE.IUniform>;

/** 壳板发射状态 */
interface TileState {
  centers: THREE.Vector3[];
  tangents: THREE.Vector3[];
  /** 每块壳板的建造顺序 0~1 */
  order: Float32Array;
  /** 每块壳板在顶点缓冲中的起始位置与顶点数 */
  start: Uint32Array;
  count: Uint32Array;
  /** 按建造顺序排序的壳板索引 */
  sorted: number[];
  /** 已发射的壳板数（sorted 的前缀） */
  launched: number;
  /** 下一块壳板最早的发射时间 */
  nextLaunch: number;
  /** 相邻壳板的发射间隔（秒） */
  interval: number;
  launchAttr: THREE.BufferAttribute;
  sparkAttr: THREE.BufferAttribute | null;
}

/**
 * 戴森壳图层：
 * - 主壳：六边形 Goldberg 框架（节点 + 结构梁）+ 六边形壳板
 * - 节点与框架按建造进度点亮；壳板在进度越过自身顺序时发射，从外侧沿径向飞来落位
 * - 外层能量网格：反向缓慢旋转的稀疏测地线
 * - 壳层辉光与漂浮光尘
 * 所有透明部件都拆成前 / 后半球两次绘制，保证层次正确。
 */
export class ShellLayer {
  readonly group = new THREE.Group();

  private readonly tilt = new THREE.Group();
  private readonly spin = new THREE.Group();
  private readonly latticeTilt = new THREE.Group();

  // 首次显示时从 0 开始建造，播放一次完整的建造动画
  private targetProgress = 0.72;
  private progress = 0;
  private level = -1;
  private meshes: THREE.Object3D[] = [];
  private geometries: THREE.BufferGeometry[] = [];
  private tiles: TileState | null = null;
  private instantOnce = false;

  private readonly nodeUniforms: Uniforms;
  private readonly frameUniforms: Uniforms;
  private readonly tileUniforms: Uniforms;
  private readonly sparkUniforms: Uniforms;
  private readonly latticeUniforms: Uniforms;
  private readonly latticeNodeUniforms: Uniforms;
  private readonly auraUniforms: Uniforms;
  private readonly moteUniforms: Uniforms;

  private readonly nodeMaterials: HemiMaterials;
  private readonly frameMaterials: HemiMaterials;
  private readonly tileMaterials: HemiMaterials;
  private readonly sparkMaterials: HemiMaterials;
  private readonly latticeMaterials: HemiMaterials;
  private readonly latticeNodeMaterials: HemiMaterials;
  private readonly moteMaterials: HemiMaterials;
  private readonly auraMaterial: THREE.ShaderMaterial;

  private latticeGeometry!: THREE.BufferGeometry;
  private latticeNodeGeometry!: THREE.BufferGeometry;
  private moteGeometry!: THREE.BufferGeometry;
  private readonly aura: THREE.Mesh<THREE.PlaneGeometry, THREE.ShaderMaterial>;

  constructor() {
    this.tilt.rotation.set(0.32, 0, -0.18);
    this.tilt.add(this.spin);
    this.latticeTilt.rotation.set(-0.45, 0.3, 0.28);
    this.group.add(this.tilt, this.latticeTilt);

    const common = () => ({ uAlpha: { value: 1 }, uOver: { value: 0 }, uTime: { value: 0 } });
    const fly = () => ({
      uRadius: { value: SHELL_RADIUS },
      uFar: { value: FLY_FAR },
      uLateral: { value: FLY_LATERAL },
      uFlyDur: { value: FLY_DURATION },
    });
    const pointScale = { value: 400 };

    this.nodeUniforms = {
      ...common(),
      uColor: { value: new THREE.Color() },
      uHot: { value: new THREE.Color() },
      uBuild: { value: 0 },
      uBand: { value: BAND.node },
      uPointScale: pointScale,
      uSize: { value: 0.1 },
    };
    this.frameUniforms = {
      ...common(),
      uColor: { value: new THREE.Color() },
      uAlt: { value: new THREE.Color() },
      uHot: { value: new THREE.Color() },
      uBuild: { value: 0 },
      uBand: { value: BAND.frame },
      uWidth: { value: 0.03 },
    };
    this.tileUniforms = {
      ...common(),
      ...fly(),
      uCell: { value: new THREE.Color() },
      uEdge: { value: new THREE.Color() },
      uPanel: { value: new THREE.Color() },
      uRim: { value: new THREE.Color() },
      uLattice: { value: new THREE.Color() },
      uHot: { value: new THREE.Color() },
      uEmit: { value: 1 },
      uCover: { value: 1 },
    };
    this.sparkUniforms = {
      ...common(),
      ...fly(),
      uColor: { value: new THREE.Color() },
      uHot: { value: new THREE.Color() },
      uPointScale: pointScale,
      uSize: { value: 0.05 },
      uLag: { value: SPARK_LAG },
      uCount: { value: SPARKS_PER_TILE },
    };
    this.latticeUniforms = {
      ...common(),
      uColor: { value: new THREE.Color() },
      uHot: { value: new THREE.Color() },
      uBuild: { value: 10 },
      uBand: { value: 1 },
      uWidth: { value: 0.018 },
      uVis: { value: 0 },
    };
    this.latticeNodeUniforms = {
      ...common(),
      uColor: this.latticeUniforms.uColor,
      uHot: this.latticeUniforms.uHot,
      uBuild: { value: 10 },
      uBand: { value: 1 },
      uPointScale: pointScale,
      uSize: { value: 0.09 },
    };
    this.auraUniforms = {
      ...common(),
      uA: { value: new THREE.Color() },
      uB: { value: new THREE.Color() },
      uVis: { value: 0 },
    };
    this.moteUniforms = {
      ...common(),
      uA: { value: new THREE.Color() },
      uB: { value: new THREE.Color() },
      uC: { value: new THREE.Color() },
      uVis: { value: 0 },
      uRadius: { value: SHELL_RADIUS },
      uPointScale: pointScale,
      uSize: { value: 0.035 },
    };

    const doubleSide = { side: THREE.DoubleSide };
    this.nodeMaterials = hemiMaterials(this.nodeUniforms, NODE_VERT, NODE_FRAG);
    this.frameMaterials = hemiMaterials(this.frameUniforms, BEAM_VERT, FRAME_FRAG, doubleSide);
    this.tileMaterials = hemiMaterials(this.tileUniforms, TILE_VERT, TILE_FRAG, doubleSide);
    this.sparkMaterials = hemiMaterials(this.sparkUniforms, SPARK_VERT, SPARK_FRAG);
    this.latticeMaterials = hemiMaterials(this.latticeUniforms, BEAM_VERT, LATTICE_FRAG, doubleSide);
    this.latticeNodeMaterials = hemiMaterials(this.latticeNodeUniforms, NODE_VERT, NODE_FRAG);
    this.moteMaterials = hemiMaterials(this.moteUniforms, MOTE_VERT, MOTE_FRAG);
    this.auraMaterial = applyPremultBlending(
      new THREE.ShaderMaterial({ uniforms: this.auraUniforms, vertexShader: AURA_VERT, fragmentShader: AURA_FRAG }),
    );

    this.buildLattice();
    this.buildMotes();
    this.aura = new THREE.Mesh(new THREE.PlaneGeometry(2, 2), this.auraMaterial);
    this.aura.renderOrder = RENDER_ORDER.aura;
    this.group.add(this.aura);
  }

  /** 外层能量网格：频率 2 的稀疏测地线，边的顺序随机，用于错开光脉冲 */
  private buildLattice(): void {
    const { vertices, edges } = buildGeodesic(2);
    const rand = mulberry32(99);
    const geo = buildBeamGeometry(vertices, edges, LATTICE_RADIUS, (i0, i1) => [i0, i1, rand()]);
    this.latticeGeometry = geo;
    this.latticeTilt.add(
      ...hemiPair((m) => new THREE.Mesh(geo, m), this.latticeMaterials, RENDER_ORDER.latticeFront, RENDER_ORDER.latticeBack),
    );

    const pos = new Float32Array(vertices.length * 3);
    vertices.forEach((v, i) => v.clone().multiplyScalar(LATTICE_RADIUS).toArray(pos, i * 3));
    const ngeo = new THREE.BufferGeometry();
    ngeo.setAttribute('position', new THREE.BufferAttribute(pos, 3));
    ngeo.setAttribute('aOrder', new THREE.BufferAttribute(new Float32Array(vertices.length), 1));
    this.latticeNodeGeometry = ngeo;
    this.latticeTilt.add(
      ...hemiPair((m) => new THREE.Points(ngeo, m), this.latticeNodeMaterials, RENDER_ORDER.latticeFront, RENDER_ORDER.latticeBack),
    );
  }

  private buildMotes(): void {
    const rand = mulberry32(5150);
    const dir = new Float32Array(MOTE_COUNT * 3);
    const rad = new Float32Array(MOTE_COUNT);
    const speed = new Float32Array(MOTE_COUNT);
    const seed = new Float32Array(MOTE_COUNT);
    for (let i = 0; i < MOTE_COUNT; i++) {
      const u = rand() * 2 - 1;
      const th = rand() * Math.PI * 2;
      const s = Math.sqrt(1 - u * u);
      dir[i * 3] = Math.cos(th) * s;
      dir[i * 3 + 1] = u;
      dir[i * 3 + 2] = Math.sin(th) * s;
      rad[i] = 0.96 + Math.pow(rand(), 2) * 0.4;
      speed[i] = (0.02 + rand() * 0.06) * (rand() < 0.5 ? 1 : -1);
      seed[i] = rand();
    }
    const geo = new THREE.BufferGeometry();
    geo.setAttribute('position', new THREE.BufferAttribute(new Float32Array(MOTE_COUNT * 3), 3));
    geo.setAttribute('aDir', new THREE.BufferAttribute(dir, 3));
    geo.setAttribute('aRad', new THREE.BufferAttribute(rad, 1));
    geo.setAttribute('aSpeed', new THREE.BufferAttribute(speed, 1));
    geo.setAttribute('aSeed', new THREE.BufferAttribute(seed, 1));
    this.moteGeometry = geo;
    this.group.add(
      ...hemiPair((m) => new THREE.Points(geo, m), this.moteMaterials, RENDER_ORDER.motesFront, RENDER_ORDER.motesBack),
    );
  }

  /** 细分级别 1~5；首次以外的细分切换直接显示建成状态，不重播飞行动画 */
  setLevel(level: number): void {
    if (level === this.level) return;
    this.instantOnce = this.level !== -1;
    this.level = level;
    this.disposeMeshes();

    const freq = LEVEL_FREQ[THREE.MathUtils.clamp(Math.round(level), 1, LEVEL_FREQ.length) - 1];
    const R = SHELL_RADIUS;
    const { nodes, edges, tiles } = buildGoldberg(freq);
    const nodeOrder = computeOrder(nodes, 42);

    // 节点
    const pos = new Float32Array(nodes.length * 3);
    nodes.forEach((v, i) => v.clone().multiplyScalar(R * 1.004).toArray(pos, i * 3));
    const nodeGeo = new THREE.BufferGeometry();
    nodeGeo.setAttribute('position', new THREE.BufferAttribute(pos, 3));
    nodeGeo.setAttribute('aOrder', new THREE.BufferAttribute(nodeOrder, 1));

    // 结构梁（六边形边）：从较早建造的节点向较晚的节点生长
    const frameGeo = buildBeamGeometry(nodes, edges, R * 1.002, (i0, i1) => {
      const [s, t] = nodeOrder[i0] <= nodeOrder[i1] ? [i0, i1] : [i1, i0];
      return [s, t, nodeOrder[t]];
    });

    const { state, geometry: tileGeo } = this.buildTiles(tiles, nodes, nodeOrder);
    const sparkGeo = this.buildSparks(state);
    this.tiles = state;

    this.meshes = [
      ...hemiPair((m) => new THREE.Mesh(tileGeo, m), this.tileMaterials, RENDER_ORDER.cellsFront, RENDER_ORDER.cellsBack),
      ...hemiPair((m) => new THREE.Mesh(frameGeo, m), this.frameMaterials, RENDER_ORDER.framesFront, RENDER_ORDER.framesBack),
      ...hemiPair((m) => new THREE.Points(nodeGeo, m), this.nodeMaterials, RENDER_ORDER.nodesFront, RENDER_ORDER.nodesBack),
      ...hemiPair((m) => new THREE.Points(sparkGeo, m), this.sparkMaterials, RENDER_ORDER.motesFront, RENDER_ORDER.motesBack),
    ];
    this.geometries = [tileGeo, frameGeo, nodeGeo, sparkGeo];

    // 节点与梁的尺寸随壳板大小变化
    this.nodeUniforms.uSize.value = 0.5 / freq;
    this.frameUniforms.uWidth.value = 0.006 + 0.13 / freq;
    this.sparkUniforms.uSize.value = 0.03 + 0.2 / freq;
    this.spin.add(...this.meshes);
  }

  /** 壳板三角扇几何与每块壳板的发射状态 */
  private buildTiles(
    tiles: GoldbergTile[],
    nodes: THREE.Vector3[],
    nodeOrder: Float32Array,
  ): { state: TileState; geometry: THREE.BufferGeometry } {
    const R = SHELL_RADIUS;
    const rand = mulberry32(777);
    let vertCount = 0;
    for (const t of tiles) vertCount += t.corners.length * 3;

    const position = new Float32Array(vertCount * 3);
    const center = new Float32Array(vertCount * 3);
    const tangent = new Float32Array(vertCount * 3);
    const bary = new Float32Array(vertCount * 3);
    const seed = new Float32Array(vertCount);
    const spin = new Float32Array(vertCount);
    const launch = new Float32Array(vertCount).fill(-1);

    const n = tiles.length;
    const order = new Float32Array(n);
    const start = new Uint32Array(n);
    const count = new Uint32Array(n);
    const tangents: THREE.Vector3[] = [];

    const tan = new THREE.Vector3();
    const c = new THREE.Vector3();
    const pa = new THREE.Vector3();
    const pb = new THREE.Vector3();
    let vi = 0;
    const push = (p: THREE.Vector3, b: [number, number, number], tile: GoldbergTile, s: number, sp: number) => {
      p.toArray(position, vi * 3);
      tile.center.toArray(center, vi * 3);
      tan.toArray(tangent, vi * 3);
      bary.set(b, vi * 3);
      seed[vi] = s;
      spin[vi] = sp;
      vi++;
    };
    tiles.forEach((tile, ti) => {
      // 壳板在其所有角点节点建成之后才发射
      let o = 0;
      for (const k of tile.corners) o = Math.max(o, nodeOrder[k]);
      order[ti] = o;
      tan.set(rand() * 2 - 1, rand() * 2 - 1, rand() * 2 - 1);
      tan.addScaledVector(tile.center, -tan.dot(tile.center)).normalize();
      tangents.push(tan.clone());
      const s = rand();
      const sp = (rand() * 2 - 1) * 2.4;
      c.copy(tile.center).multiplyScalar(R * 0.995);

      start[ti] = vi;
      const m = tile.corners.length;
      for (let k = 0; k < m; k++) {
        pa.copy(nodes[tile.corners[k]]).multiplyScalar(R * 0.995);
        pb.copy(nodes[tile.corners[(k + 1) % m]]).multiplyScalar(R * 0.995);
        push(c, [1, 0, 0], tile, s, sp);
        push(pa, [0, 1, 0], tile, s, sp);
        push(pb, [0, 0, 1], tile, s, sp);
      }
      count[ti] = vi - start[ti];
    });

    const geo = new THREE.BufferGeometry();
    geo.setAttribute('position', new THREE.BufferAttribute(position, 3));
    geo.setAttribute('aCenter', new THREE.BufferAttribute(center, 3));
    geo.setAttribute('aTan', new THREE.BufferAttribute(tangent, 3));
    geo.setAttribute('aBary', new THREE.BufferAttribute(bary, 3));
    geo.setAttribute('aSeed', new THREE.BufferAttribute(seed, 1));
    geo.setAttribute('aSpin', new THREE.BufferAttribute(spin, 1));
    const launchAttr = new THREE.BufferAttribute(launch, 1);
    launchAttr.setUsage(THREE.DynamicDrawUsage);
    geo.setAttribute('aLaunch', launchAttr);

    const sorted = Array.from({ length: n }, (_, i) => i).sort((a, b) => order[a] - order[b]);
    const state: TileState = {
      centers: tiles.map((t) => t.center),
      tangents,
      order,
      start,
      count,
      sorted,
      launched: 0,
      nextLaunch: 0,
      // 发射间隔：一次性建造全部壳板约 4.5 秒
      interval: Math.min(0.025, 4.5 / n),
      launchAttr,
      sparkAttr: null,
    };
    return { state, geometry: geo };
  }

  /** 飞行拖尾粒子：每块壳板 SPARKS_PER_TILE 个 */
  private buildSparks(state: TileState): THREE.BufferGeometry {
    const n = state.centers.length * SPARKS_PER_TILE;
    const center = new Float32Array(n * 3);
    const tangent = new Float32Array(n * 3);
    const k = new Float32Array(n);
    const seed = new Float32Array(n);
    const launch = new Float32Array(n).fill(-1);
    const rand = mulberry32(4242);
    state.centers.forEach((c, ti) => {
      for (let j = 0; j < SPARKS_PER_TILE; j++) {
        const i = ti * SPARKS_PER_TILE + j;
        c.toArray(center, i * 3);
        state.tangents[ti].toArray(tangent, i * 3);
        k[i] = j;
        seed[i] = rand();
      }
    });
    const geo = new THREE.BufferGeometry();
    geo.setAttribute('position', new THREE.BufferAttribute(new Float32Array(n * 3), 3));
    geo.setAttribute('aCenter', new THREE.BufferAttribute(center, 3));
    geo.setAttribute('aTan', new THREE.BufferAttribute(tangent, 3));
    geo.setAttribute('aK', new THREE.BufferAttribute(k, 1));
    geo.setAttribute('aSeed', new THREE.BufferAttribute(seed, 1));
    const launchAttr = new THREE.BufferAttribute(launch, 1);
    launchAttr.setUsage(THREE.DynamicDrawUsage);
    geo.setAttribute('aLaunch', launchAttr);
    state.sparkAttr = launchAttr;
    return geo;
  }

  private setTileLaunch(ti: number, time: number): void {
    const t = this.tiles!;
    (t.launchAttr.array as Float32Array).fill(time, t.start[ti], t.start[ti] + t.count[ti]);
    (t.sparkAttr!.array as Float32Array).fill(time, ti * SPARKS_PER_TILE, (ti + 1) * SPARKS_PER_TILE);
  }

  private markLaunchDirty(): void {
    const t = this.tiles!;
    t.launchAttr.needsUpdate = true;
    t.sparkAttr!.needsUpdate = true;
  }

  /**
   * 按当前进度发射 / 回收壳板。
   * 进度前进时按顺序排队发射（有最小间隔，形成连续的"打帆"效果）；进度后退时立即回收。
   */
  private updateLaunches(time: number, instant: boolean): void {
    const t = this.tiles;
    if (!t) return;
    const build = layerProgress(this.progress, LAG.cell, 0.02);
    let changed = false;
    while (t.launched > 0 && t.order[t.sorted[t.launched - 1]] >= build) {
      t.launched--;
      this.setTileLaunch(t.sorted[t.launched], -1);
      t.nextLaunch = time;
      changed = true;
    }
    while (t.launched < t.sorted.length && t.order[t.sorted[t.launched]] < build) {
      let at: number;
      if (instant) {
        at = time - FLY_DURATION - 10;
      } else {
        at = Math.max(time, t.nextLaunch);
        t.nextLaunch = at + t.interval;
      }
      this.setTileLaunch(t.sorted[t.launched], at);
      t.launched++;
      changed = true;
    }
    if (changed) this.markLaunchDirty();
  }

  /** 建造进度 0~1（平滑过渡） */
  setProgress(v: number): void {
    this.targetProgress = v;
  }

  /** 从 0 开始重播建造动画 */
  replay(): void {
    this.progress = 0;
    this.instantOnce = false;
    const t = this.tiles;
    if (!t) return;
    for (let i = 0; i < t.sorted.length; i++) this.setTileLaunch(i, -1);
    t.launched = 0;
    t.nextLaunch = 0;
    this.markLaunchDirty();
  }

  setViewport(pointScale: number): void {
    this.nodeUniforms.uPointScale.value = pointScale;
  }

  applyPalette(p: Palette): void {
    this.nodeUniforms.uColor.value.copy(p.shellNode);
    this.nodeUniforms.uHot.value.copy(p.shellFrameHot);
    this.frameUniforms.uColor.value.copy(p.shellFrame);
    this.frameUniforms.uHot.value.copy(p.shellFrameHot);
    this.frameUniforms.uAlt.value.copy(p.shellLattice);
    const c = this.tileUniforms;
    c.uCell.value.copy(p.shellCell);
    c.uEdge.value.copy(p.shellCellEdge);
    c.uPanel.value.copy(p.shellPanel);
    c.uRim.value.copy(p.shellRim);
    c.uLattice.value.copy(p.shellLattice);
    c.uHot.value.copy(p.shellFrameHot);
    c.uEmit.value = p.shellEmit;
    c.uCover.value = p.shellCover;
    this.sparkUniforms.uColor.value.copy(p.shellCellEdge);
    this.sparkUniforms.uHot.value.copy(p.shellFrameHot);
    this.latticeUniforms.uColor.value.copy(p.shellLattice);
    this.latticeUniforms.uHot.value.copy(p.shellLatticeHot);
    this.auraUniforms.uA.value.copy(p.shellRim);
    this.auraUniforms.uB.value.copy(p.shellLattice);
    this.moteUniforms.uA.value.copy(p.shellRim);
    this.moteUniforms.uB.value.copy(p.shellLattice);
    this.moteUniforms.uC.value.copy(p.shellCellEdge);
    for (const u of this.allUniforms()) {
      u.uAlpha.value = p.shellAlpha;
      u.uOver.value = p.over;
    }
  }

  private allUniforms(): Uniforms[] {
    return [
      this.nodeUniforms,
      this.frameUniforms,
      this.tileUniforms,
      this.sparkUniforms,
      this.latticeUniforms,
      this.latticeNodeUniforms,
      this.auraUniforms,
      this.moteUniforms,
    ];
  }

  /** reducedMotion：减少动态效果时壳板直接落位，不播放飞行动画 */
  update(dt: number, time: number, spinSpeed: number, camera: THREE.Camera, reducedMotion: boolean): void {
    this.progress = damp(this.progress, this.targetProgress, 2, dt);
    const P = this.progress;
    this.nodeUniforms.uBuild.value = layerProgress(P, LAG.node, BAND.node);
    this.frameUniforms.uBuild.value = layerProgress(P, LAG.frame, BAND.frame);
    this.updateLaunches(time, reducedMotion || this.instantOnce);
    this.instantOnce = false;
    // 外层网格、辉光、光尘随建造进度浮现
    this.latticeUniforms.uVis.value = THREE.MathUtils.smoothstep(P, 0.25, 0.85);
    this.auraUniforms.uVis.value = 0.25 + 0.75 * THREE.MathUtils.smoothstep(P, 0.0, 0.9);
    this.moteUniforms.uVis.value = 0.3 + 0.7 * P;
    for (const u of this.allUniforms()) u.uTime.value = time;

    this.spin.rotation.y += dt * 0.05 * spinSpeed;
    this.latticeTilt.rotation.y -= dt * 0.03 * spinSpeed;

    // 辉光面片：朝向相机，按透视把轮廓半径对齐到壳层外缘
    const d = camera.position.length();
    const k = d / Math.sqrt(Math.max(d * d - SHELL_RADIUS * SHELL_RADIUS, 1e-3));
    this.aura.quaternion.copy(camera.quaternion);
    this.aura.scale.setScalar(SHELL_RADIUS * k * AURA_EXTENT);
  }

  private disposeMeshes(): void {
    for (const m of this.meshes) this.spin.remove(m);
    this.geometries.forEach((g) => g.dispose());
    this.meshes = [];
    this.geometries = [];
    this.tiles = null;
  }

  dispose(): void {
    this.disposeMeshes();
    this.latticeGeometry.dispose();
    this.latticeNodeGeometry.dispose();
    this.moteGeometry.dispose();
    this.aura.geometry.dispose();
    [
      ...this.nodeMaterials,
      ...this.frameMaterials,
      ...this.tileMaterials,
      ...this.sparkMaterials,
      ...this.latticeMaterials,
      ...this.latticeNodeMaterials,
      ...this.moteMaterials,
      this.auraMaterial,
    ].forEach((m) => m.dispose());
  }
}
