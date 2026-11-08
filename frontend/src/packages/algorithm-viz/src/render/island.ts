import * as THREE from 'three';
import { easeInCubic, easeOutBack, hash01 } from '../internal/math';
import { ISLAND_FRAGMENT, ISLAND_VERTEX } from './shaders';

/**
 * 浮空平台：由边长 TILE 的方块拼成，只保留建筑下方的这一层平台。
 *
 * - 场景范围扩大时，新的方块从下方雾中升起，按离原平台的距离依次到位
 * - 范围缩小时先等待 SHRINK_DELAY 秒，确认不再回弹后，多余方块沉回雾中，避免算法搜索候选时反复抖动
 * - 方块顶面绘制建造网格，主网格线与方块边缘对齐
 */

/** 方块边长（蓝图格），与地面主网格间距一致 */
export const TILE = 4;
const THICKNESS = 1.4;
const RISE_DEPTH = 8;
const RISE_TIME = 0.7;
const SINK_TIME = 0.5;
const MARGIN = 3.5;
const MIN_HALF = 8;
const SHRINK_DELAY = 1.2;

interface CellRect {
  i0: number;
  i1: number;
  j0: number;
  j1: number;
}

interface Tile {
  i: number;
  j: number;
  slot: number;
  state: 'rise' | 'idle' | 'sink';
  t: number;
  delay: number;
  /** 升起起点（被打断的下沉从当前位置继续升起） */
  from: number;
  y: number;
}

const ZERO = new THREE.Matrix4().makeScale(0, 0, 0);
const cellKey = (i: number, j: number): string => `${i},${j}`;
const contains = (r: CellRect, i: number, j: number): boolean => i >= r.i0 && i <= r.i1 && j >= r.j0 && j <= r.j1;

export class IslandLayer {
  readonly group = new THREE.Group();
  readonly material: THREE.ShaderMaterial;
  private readonly box: THREE.BufferGeometry;
  private mesh: THREE.InstancedMesh;
  private capacity = 256;
  private used = 0;
  private readonly free: number[] = [];
  private readonly tiles = new Map<string, Tile>();
  private readonly animating = new Set<Tile>();
  /** 当前平台（未下沉方块）的外接矩形：minX, minZ, maxX, maxZ */
  private readonly bounds = new THREE.Vector4(-MIN_HALF, -MIN_HALF, MIN_HALF, MIN_HALF);
  private target: CellRect | null = null;
  private shrinkSince = -1;
  private time = 0;
  private readonly matrix = new THREE.Matrix4();

  constructor(uniforms: Record<string, THREE.IUniform>) {
    this.box = new THREE.BoxGeometry(1, 1, 1).translate(0, -0.5, 0);
    this.material = new THREE.ShaderMaterial({
      vertexShader: ISLAND_VERTEX,
      fragmentShader: ISLAND_FRAGMENT,
      uniforms: { ...uniforms, uBounds: { value: this.bounds }, uTile: { value: TILE }, uFail: { value: 0 } },
    });
    this.mesh = this.createMesh();
    this.setTarget(null);
    this.snap();
  }

  /** 设置场景包围范围（世界坐标 x/z）；空场景传 null 时在原地保留一块默认小平台 */
  setTarget(range: { minX: number; minZ: number; maxX: number; maxZ: number } | null): void {
    let minX: number;
    let minZ: number;
    let maxX: number;
    let maxZ: number;
    if (range) {
      ({ minX, minZ, maxX, maxZ } = range);
      minX -= MARGIN;
      minZ -= MARGIN;
      maxX += MARGIN;
      maxZ += MARGIN;
    } else {
      const cx = (this.bounds.x + this.bounds.z) / 2;
      const cz = (this.bounds.y + this.bounds.w) / 2;
      [minX, minZ, maxX, maxZ] = [cx - MIN_HALF, cz - MIN_HALF, cx + MIN_HALF, cz + MIN_HALF];
    }
    const cx = (minX + maxX) / 2;
    const cz = (minZ + maxZ) / 2;
    const hx = Math.max(MIN_HALF, (maxX - minX) / 2);
    const hz = Math.max(MIN_HALF, (maxZ - minZ) / 2);
    const rect: CellRect = {
      i0: Math.floor((cx - hx) / TILE),
      i1: Math.floor((cx + hx) / TILE),
      j0: Math.floor((cz - hz) / TILE),
      j1: Math.floor((cz + hz) / TILE),
    };
    const previous = this.target;
    this.target = rect;
    // 扩张立即进行：缺少的方块按离原平台的距离依次升起
    for (let i = rect.i0; i <= rect.i1; i++) {
      for (let j = rect.j0; j <= rect.j1; j++) {
        const tile = this.tiles.get(cellKey(i, j));
        if (tile && tile.state !== 'sink') continue;
        const distance = previous ? Math.max(previous.i0 - i, i - previous.i1, previous.j0 - j, j - previous.j1, 0) : 0;
        this.raise(i, j, Math.min(0.6, distance * 0.06) + hash01(i * 31 + j * 17) * 0.08, tile);
      }
    }
    // 收缩延后：若范围外仍有方块，开始计时，到期后再下沉
    let outside = false;
    for (const tile of this.tiles.values()) {
      if (tile.state !== 'sink' && !contains(rect, tile.i, tile.j)) {
        outside = true;
        break;
      }
    }
    this.shrinkSince = outside ? (this.shrinkSince >= 0 ? this.shrinkSince : this.time) : -1;
    this.refreshBounds();
  }

  /** 立即对齐到目标范围（首次显示时避免从默认尺寸长出） */
  snap(): void {
    for (const tile of this.tiles.values()) {
      if (tile.state === 'sink' || (this.target && !contains(this.target, tile.i, tile.j))) {
        this.removeTile(tile);
        continue;
      }
      tile.state = 'idle';
      tile.delay = 0;
      tile.y = 0;
      this.animating.delete(tile);
      this.write(tile);
    }
    this.shrinkSince = -1;
    this.refreshBounds();
    this.mesh.instanceMatrix.needsUpdate = true;
  }

  /** 失败标注：0 为正常，1 为整块染成警示红 */
  setFail(value: number): void {
    this.material.uniforms.uFail.value = value;
  }

  /** 当前平台顶面范围（世界坐标） */
  getBounds(): THREE.Vector4 {
    return this.bounds;
  }

  update(dt: number, time: number, reduced: boolean): void {
    this.time = time;
    if (this.shrinkSince >= 0 && time - this.shrinkSince >= SHRINK_DELAY && this.target) {
      this.shrinkSince = -1;
      for (const tile of this.tiles.values()) {
        if (tile.state !== 'sink' && !contains(this.target, tile.i, tile.j)) this.sink(tile, hash01(tile.i * 13 + tile.j * 7) * 0.2);
      }
      this.refreshBounds();
    }
    if (!this.animating.size) return;
    for (const tile of Array.from(this.animating)) {
      if (tile.delay > 0) {
        tile.delay -= dt;
        continue;
      }
      if (tile.state === 'rise') {
        tile.t = reduced ? 1 : Math.min(1, tile.t + dt / RISE_TIME);
        tile.y = tile.from * (1 - easeOutBack(tile.t, 1.2));
        if (tile.t >= 1) {
          tile.y = 0;
          tile.state = 'idle';
          this.animating.delete(tile);
        }
      } else if (tile.state === 'sink') {
        tile.t = reduced ? 1 : Math.min(1, tile.t + dt / SINK_TIME);
        tile.y = -RISE_DEPTH * easeInCubic(tile.t);
        if (tile.t >= 1) {
          this.removeTile(tile);
          continue;
        }
      }
      this.write(tile);
    }
    this.mesh.instanceMatrix.needsUpdate = true;
  }

  dispose(): void {
    this.group.remove(this.mesh);
    this.mesh.dispose();
    this.box.dispose();
    this.material.dispose();
  }

  private raise(i: number, j: number, delay: number, existing?: Tile): void {
    const tile: Tile = existing ?? { i, j, slot: this.alloc(), state: 'rise', t: 0, delay, from: -RISE_DEPTH, y: -RISE_DEPTH };
    if (existing) {
      // 正在下沉的方块原地折返升起
      tile.from = tile.y;
      tile.delay = 0;
    } else {
      this.tiles.set(cellKey(i, j), tile);
    }
    tile.state = 'rise';
    tile.t = 0;
    this.animating.add(tile);
    this.write(tile);
  }

  private sink(tile: Tile, delay: number): void {
    tile.state = 'sink';
    tile.t = 0;
    tile.delay = delay;
    this.animating.add(tile);
  }

  private removeTile(tile: Tile): void {
    this.tiles.delete(cellKey(tile.i, tile.j));
    this.animating.delete(tile);
    this.mesh.setMatrixAt(tile.slot, ZERO);
    if (tile.slot === this.used - 1) this.used--;
    else this.free.push(tile.slot);
    this.mesh.count = this.used;
  }

  private write(tile: Tile): void {
    this.matrix.makeScale(TILE, THICKNESS, TILE).setPosition((tile.i + 0.5) * TILE, tile.y, (tile.j + 0.5) * TILE);
    this.mesh.setMatrixAt(tile.slot, this.matrix);
  }

  /** 外接矩形只统计未下沉的方块，供网格包边与取景使用 */
  private refreshBounds(): void {
    let i0 = Infinity;
    let i1 = -Infinity;
    let j0 = Infinity;
    let j1 = -Infinity;
    for (const tile of this.tiles.values()) {
      if (tile.state === 'sink') continue;
      i0 = Math.min(i0, tile.i);
      i1 = Math.max(i1, tile.i);
      j0 = Math.min(j0, tile.j);
      j1 = Math.max(j1, tile.j);
    }
    if (Number.isFinite(i0)) this.bounds.set(i0 * TILE, j0 * TILE, (i1 + 1) * TILE, (j1 + 1) * TILE);
  }

  private alloc(): number {
    const slot = this.free.length ? (this.free.pop() as number) : this.used++;
    if (slot >= this.capacity) this.grow();
    this.mesh.count = this.used;
    return slot;
  }

  private createMesh(): THREE.InstancedMesh {
    const mesh = new THREE.InstancedMesh(this.box, this.material, this.capacity);
    mesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
    mesh.count = this.used;
    mesh.frustumCulled = false;
    mesh.renderOrder = -10;
    this.group.add(mesh);
    return mesh;
  }

  private grow(): void {
    const old = this.mesh;
    const oldCapacity = this.capacity;
    this.capacity *= 2;
    this.mesh = this.createMesh();
    for (let i = 0; i < oldCapacity; i++) {
      old.getMatrixAt(i, this.matrix);
      this.mesh.setMatrixAt(i, this.matrix);
    }
    for (let i = oldCapacity; i < this.capacity; i++) this.mesh.setMatrixAt(i, ZERO);
    this.group.remove(old);
    old.dispose();
  }
}
