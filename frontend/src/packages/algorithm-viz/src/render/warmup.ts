import * as THREE from 'three';
import type { InstancePool } from './pool';

/** 模型池的异步着色器预热；使用与正式绘制相同的实例属性，准备完成前由已有占位体显示。 */
export class PoolWarmup {
  private readonly prepared = new WeakSet<InstancePool>();
  private readonly pending = new WeakSet<InstancePool>();
  private disposed = false;

  constructor(
    private readonly renderer: THREE.WebGLRenderer,
    private readonly camera: THREE.Camera,
    private readonly scene: THREE.Scene,
    private readonly ready: (pool: InstancePool) => void,
  ) {}

  isReady(pool: InstancePool): boolean { return this.prepared.has(pool); }

  /** 独立实例网格仅用于编译，绝不加入可见舞台；完成后仍由主场景按帧预算迁移模型。 */
  prepare(pool: InstancePool): void {
    if (this.disposed || this.prepared.has(pool) || this.pending.has(pool)) return;
    this.pending.add(pool);
    pool.mesh.visible = false;
    const probe = new THREE.InstancedMesh(pool.mesh.geometry, pool.mesh.material, 1);
    void this.renderer.compileAsync(probe, this.camera, this.scene).then(() => {
      if (this.disposed) return;
      this.prepared.add(pool);
      this.ready(pool);
    }).catch(() => {
      // 编译失败保留可用占位体，不能把坏模型切进正在播放的场景。
    }).finally(() => {
      this.pending.delete(pool);
      probe.dispose();
    });
  }

  dispose(): void { this.disposed = true; }
}
