import * as THREE from 'three';
import { UploadRanges } from './upload-ranges';

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
  private generationPeak = 0;
  private readonly minimumCapacity: number;
  private readonly arrays = new Map<string, Float32Array>();
  private matrixLow = Infinity;
  private matrixHigh = -1;
  /** 矩阵按128实例分块；属性按256个标量分块，不把分散动画中间的静止实例全上传。 */
  private readonly matrixUploads = new UploadRanges(128 * 16);
  private readonly attributeUploads = new Map<string, UploadRanges>();
  private readonly dirtyAttributes = new Map<string, UploadRanges>();
  /** 只在拾取时保存平移列，避免平台动画持有逐对象Matrix4副本。 */
  private pickTranslations = new Float32Array(0);

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
    this.minimumCapacity = this.capacity;
    for (const spec of specs) {
      this.arrays.set(spec.name, new Float32Array(this.capacity * spec.itemSize));
      this.attributeUploads.set(spec.name, new UploadRanges(256));
    }
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
    this.matrixUploads.clear();
    for (const range of this.dirtyAttributes.values()) range.clear();
    this.dirtyAttributes.clear();
  }

  /** 保留上次任务的容量需求；更早大蓝图留下的空闲缓冲在新任务开始时缩小。 */
  beginGeneration(): void {
    const required = Math.max(this.minimumCapacity, this.used, this.generationPeak);
    let capacity = this.minimumCapacity;
    while (capacity < required) capacity *= 2;
    if (capacity < this.capacity) this.resize(capacity);
    this.generationPeak = 0;
  }

  alloc(): number {
    const slot = this.used++;
    this.generationPeak = Math.max(this.generationPeak, this.used);
    if (slot >= this.capacity) this.resize(Math.max(this.capacity * 2, slot + 1));
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

  /** 着色器平台平移对CPU射线不可见；拾取期间临时投影平移列，复用Three原生网格求交，结束后逐位恢复且不上传GPU。 */
  raycast(raycaster: THREE.Raycaster, stages: readonly THREE.Vector4[], hits: THREE.Intersection[]): void {
    if (!this.used || !this.mesh.visible || !this.mesh.layers.test(raycaster.layers)) return;
    const assignments = this.arrays.get('aStage');
    if (!assignments) { this.mesh.raycast(raycaster, hits); return; }
    if (this.pickTranslations.length < this.used * 3) this.pickTranslations = new Float32Array(this.capacity * 3);
    const matrix = this.mesh.instanceMatrix.array;
    const bounds = this.mesh.boundingSphere;
    for (let slot = 0; slot < this.used; slot++) {
      const offset = slot * 16 + 12, saved = slot * 3;
      const stage = stages[Math.max(0, Math.min(stages.length - 1, Math.trunc(assignments[slot])))];
      this.pickTranslations[saved] = matrix[offset];
      this.pickTranslations[saved + 1] = matrix[offset + 1];
      this.pickTranslations[saved + 2] = matrix[offset + 2];
      matrix[offset] += stage.x; matrix[offset + 1] += stage.y; matrix[offset + 2] += stage.z;
    }
    // 平台每帧移动，原局部包围球不能用于世界坐标排除；每个实体仍由Three原生包围体和三角形求交。
    this.mesh.boundingSphere = new THREE.Sphere(new THREE.Vector3(), Infinity);
    try { this.mesh.raycast(raycaster, hits); }
    finally {
      for (let slot = 0; slot < this.used; slot++) {
        const offset = slot * 16 + 12, saved = slot * 3;
        matrix[offset] = this.pickTranslations[saved];
        matrix[offset + 1] = this.pickTranslations[saved + 1];
        matrix[offset + 2] = this.pickTranslations[saved + 2];
      }
      this.mesh.boundingSphere = bounds;
    }
  }

  /** 把本帧的修改提交到 GPU；包围球失效后由拾取或剔除按需重算 */
  commit(): void {
    if (this.matrixHigh >= 0) {
      if (this.matrixUploads.flush(this.mesh.instanceMatrix, this.used * 16)) this.mesh.instanceMatrix.needsUpdate = true;
      this.mesh.boundingSphere = null;
      this.mesh.boundingBox = null;
      this.matrixLow = Infinity;
      this.matrixHigh = -1;
    }
    for (const [name, range] of this.dirtyAttributes) {
      const attribute = this.mesh.geometry.getAttribute(name) as THREE.InstancedBufferAttribute | undefined;
      if (attribute) {
        if (range.flush(attribute, this.used * attribute.itemSize)) attribute.needsUpdate = true;
      } else range.clear();
    }
    this.dirtyAttributes.clear();
  }

  /** 合并本帧改动范围；只上传变更槽位，未渲染的范围由 three.js 在真正上传后清理。 */
  private markMatrix(slot: number): void {
    this.matrixLow = Math.min(this.matrixLow, slot);
    this.matrixHigh = Math.max(this.matrixHigh, slot);
    this.matrixUploads.mark(slot * 16, 16);
  }

  private markAttribute(name: string, offset: number, size: number): void {
    const active = this.dirtyAttributes.get(name);
    if (active) { active.mark(offset, size); return; }
    const range = this.attributeUploads.get(name);
    if (!range) return;
    range.mark(offset, size);
    this.dirtyAttributes.set(name, range);
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

  private resize(capacity: number): void {
    const oldMesh = this.mesh;
    this.capacity = capacity;
    this.pickTranslations = new Float32Array(0);
    for (const spec of this.specs) {
      const next = new Float32Array(capacity * spec.itemSize);
      next.set((this.arrays.get(spec.name) as Float32Array).subarray(0, next.length));
      this.arrays.set(spec.name, next);
    }
    this.mesh = this.createMesh();
    this.mesh.instanceMatrix.array.set(oldMesh.instanceMatrix.array.subarray(0, this.mesh.instanceMatrix.array.length));
    this.matrixLow = Infinity;
    this.matrixHigh = -1;
    this.matrixUploads.clear();
    for (const range of this.dirtyAttributes.values()) range.clear();
    this.dirtyAttributes.clear();
    this.parent.remove(oldMesh);
    oldMesh.geometry.dispose();
    oldMesh.dispose();
    this.parent.add(this.mesh);
    // 新GPU缓冲继承了整个活动前缀，必须全量标记，不能只用首尾两个稀疏块代表中间数据。
    if (this.used) {
      this.matrixLow = 0;
      this.matrixHigh = this.used - 1;
      this.matrixUploads.mark(0, this.used * 16);
    }
    for (const spec of this.specs) this.markAttribute(spec.name, 0, this.used * spec.itemSize);
  }
}
