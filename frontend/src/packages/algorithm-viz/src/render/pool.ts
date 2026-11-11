import * as THREE from 'three';

/**
 * 实例池：一个几何体 + 一个材质对应一个 InstancedMesh，按槽位分配给场景对象。
 *
 * - 活动槽位连续存放；释放时用末端实例补位，调用方同步迁移后的对象引用，绘制数量等于活动数量
 * - 容量不足时翻倍重建 InstancedMesh，并拷贝全部实例数据
 * - 自定义的逐实例属性（状态、颜色索引、长度等）挂在池自有的几何体上，基础网格属性共享
 */

export interface InstanceAttributeSpec {
  readonly name: string;
  readonly itemSize: number;
  /** 新槽位的默认值 */
  readonly fill?: readonly number[];
}

const ZERO = new THREE.Matrix4().makeScale(0, 0, 0);

export class InstancePool {
  mesh: THREE.InstancedMesh;
  private capacity: number;
  private used = 0;
  private readonly arrays = new Map<string, Float32Array>();
  private matrixLow = Infinity;
  private matrixHigh = -1;
  private readonly dirtyAttributes = new Map<string, {low: number; high: number}>();

  constructor(
    readonly key: string,
    private readonly base: THREE.BufferGeometry,
    private readonly material: THREE.Material | THREE.Material[],
    private readonly specs: readonly InstanceAttributeSpec[],
    private readonly parent: THREE.Object3D,
    initialCapacity = 64,
    private readonly renderOrder = 0,
  ) {
    this.capacity = Math.max(8, initialCapacity);
    for (const spec of specs) this.arrays.set(spec.name, new Float32Array(this.capacity * spec.itemSize));
    this.mesh = this.createMesh();
    parent.add(this.mesh);
  }

  /** 活动槽位数，与GPU绘制数量一致。 */
  get size(): number {
    return this.used;
  }

  /** 无过渡重建时整池归零，保留 GPU 容量复用；新槽位由 alloc 重置，不逐个遍历旧建筑。 */
  clear(): void {
    this.used = 0;
    this.mesh.count = 0;
    this.matrixLow = Infinity;
    this.matrixHigh = -1;
    this.dirtyAttributes.clear();
  }

  alloc(): number {
    const slot = this.used++;
    if (slot >= this.capacity) this.grow(Math.max(this.capacity * 2, slot + 1));
    for (const spec of this.specs) {
      const array = this.arrays.get(spec.name) as Float32Array;
      for (let i = 0; i < spec.itemSize; i++) array[slot * spec.itemSize + i] = spec.fill?.[i] ?? 0;
      this.markAttribute(spec.name, slot * spec.itemSize, spec.itemSize);
    }
    this.mesh.setMatrixAt(slot, ZERO);
    this.mesh.count = this.used;
    this.markMatrix(slot);
    return slot;
  }

  /** 释放并压实实例；返回补位实例原槽号，调用方必须更新其身份映射。释放末槽或无效槽不产生迁移。 */
  release(slot: number): number | undefined {
    if (!Number.isInteger(slot) || slot < 0 || slot >= this.used) return;
    const last = --this.used;
    if (slot !== last) {
      this.mesh.instanceMatrix.array.copyWithin(slot * 16, last * 16, last * 16 + 16);
      this.markMatrix(slot);
      for (const spec of this.specs) {
        const array = this.arrays.get(spec.name) as Float32Array;
        array.copyWithin(slot * spec.itemSize, last * spec.itemSize, (last + 1) * spec.itemSize);
        this.markAttribute(spec.name, slot * spec.itemSize, spec.itemSize);
      }
    }
    this.mesh.count = this.used;
    return slot !== last ? last : undefined;
  }

  setMatrix(slot: number, matrix: THREE.Matrix4): void {
    this.mesh.setMatrixAt(slot, matrix);
    this.markMatrix(slot);
  }

  set(name: string, slot: number, a: number, b = 0, c = 0, d = 0): void {
    const array = this.arrays.get(name);
    if (!array) return;
    const size = array.length / this.capacity;
    const offset = slot * size;
    array[offset] = a;
    if (size > 1) array[offset + 1] = b;
    if (size > 2) array[offset + 2] = c;
    if (size > 3) array[offset + 3] = d;
    this.markAttribute(name, offset, size);
  }

  /** 把本帧的修改提交到 GPU；包围球失效后由拾取或剔除按需重算 */
  commit(): void {
    if (this.matrixHigh >= 0) {
      this.mesh.instanceMatrix.addUpdateRange(this.matrixLow * 16, (this.matrixHigh - this.matrixLow + 1) * 16);
      this.mesh.instanceMatrix.needsUpdate = true;
      this.mesh.boundingSphere = null;
      this.mesh.boundingBox = null;
      this.matrixLow = Infinity;
      this.matrixHigh = -1;
    }
    for (const [name, range] of this.dirtyAttributes) {
      const attribute = this.mesh.geometry.getAttribute(name) as THREE.InstancedBufferAttribute | undefined;
      if (attribute) {
        attribute.addUpdateRange(range.low, range.high - range.low);
        attribute.needsUpdate = true;
      }
    }
    this.dirtyAttributes.clear();
  }

  /** 合并本帧改动范围；只上传变更槽位，未渲染的范围由 three.js 在真正上传后清理。 */
  private markMatrix(slot: number): void {
    this.matrixLow = Math.min(this.matrixLow, slot);
    this.matrixHigh = Math.max(this.matrixHigh, slot);
  }

  private markAttribute(name: string, offset: number, size: number): void {
    const range = this.dirtyAttributes.get(name);
    if (range) { range.low = Math.min(range.low, offset); range.high = Math.max(range.high, offset + size); }
    else this.dirtyAttributes.set(name, {low: offset, high: offset + size});
  }

  dispose(): void {
    this.parent.remove(this.mesh);
    this.mesh.geometry.dispose();
    this.mesh.dispose();
  }

  /** 释放池自有的材质（共享材质由创建方负责，不要对其调用） */
  disposeMaterials(): void {
    const materials = Array.isArray(this.material) ? this.material : [this.material];
    for (const material of materials) material.dispose();
  }

  private createMesh(): THREE.InstancedMesh {
    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute('position', this.base.getAttribute('position'));
    const index = this.base.getIndex();
    if (index) geometry.setIndex(index);
    const uv = this.base.getAttribute('uv');
    if (uv) geometry.setAttribute('uv', uv);
    // 多材质模型：沿用基础几何体的材质分组
    for (const group of this.base.groups) geometry.addGroup(group.start, group.count, group.materialIndex);
    if (!this.base.boundingSphere) this.base.computeBoundingSphere();
    if (!this.base.boundingBox) this.base.computeBoundingBox();
    geometry.boundingSphere = (this.base.boundingSphere as THREE.Sphere).clone();
    geometry.boundingBox = (this.base.boundingBox as THREE.Box3).clone();
    for (const spec of this.specs) {
      const attribute = new THREE.InstancedBufferAttribute(this.arrays.get(spec.name) as Float32Array, spec.itemSize);
      attribute.setUsage(THREE.DynamicDrawUsage);
      geometry.setAttribute(spec.name, attribute);
    }
    const mesh = new THREE.InstancedMesh(geometry, this.material, this.capacity);
    mesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
    mesh.count = this.used;
    // 动画期间实例位置频繁变化，包围球很快过期，关闭视锥剔除更稳妥
    mesh.frustumCulled = false;
    mesh.renderOrder = this.renderOrder;
    mesh.userData.poolKey = this.key;
    return mesh;
  }

  private grow(capacity: number): void {
    const oldMesh = this.mesh;
    this.capacity = capacity;
    for (const spec of this.specs) {
      const next = new Float32Array(capacity * spec.itemSize);
      next.set(this.arrays.get(spec.name) as Float32Array);
      this.arrays.set(spec.name, next);
    }
    this.mesh = this.createMesh();
    this.mesh.instanceMatrix.array.set(oldMesh.instanceMatrix.array);
    this.parent.remove(oldMesh);
    oldMesh.geometry.dispose();
    oldMesh.dispose();
    this.parent.add(this.mesh);
    if (this.used) { this.markMatrix(0); this.markMatrix(this.used - 1); }
    for (const spec of this.specs) this.markAttribute(spec.name, 0, this.used * spec.itemSize);
  }
}
