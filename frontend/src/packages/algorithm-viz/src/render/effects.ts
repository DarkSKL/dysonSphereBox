import * as THREE from 'three';
import { InstancePool } from './pool';
import { RING_FRAGMENT, RING_VERTEX, SHADOW_FRAGMENT, SHADOW_VERTEX } from './shaders';

/**
 * 地面效果：建筑投影与落位涟漪。两者都是贴地的实例化方片，透明混合、不写深度。
 */

/** 朝上的单位方片（中心在原点） */
function groundQuad(): THREE.BufferGeometry {
  return new THREE.PlaneGeometry(1, 1).rotateX(-Math.PI / 2);
}

/** 建筑投影：每个有体积的建筑占用一个槽位，强度随可见度与抬升高度变化 */
export class ShadowLayer {
  private readonly geometry = groundQuad();
  private readonly material: THREE.ShaderMaterial;
  private readonly pool: InstancePool;
  private readonly matrix = new THREE.Matrix4();

  constructor(parent: THREE.Object3D, uniforms: Record<string, THREE.IUniform>) {
    this.material = new THREE.ShaderMaterial({
      vertexShader: SHADOW_VERTEX,
      fragmentShader: SHADOW_FRAGMENT,
      uniforms: { uShadowColor: uniforms.uShadow, uParams: uniforms.uParams },
      transparent: true,
      depthWrite: false,
    });
    this.pool = new InstancePool('shadow', this.geometry, this.material, [{ name: 'aShadow', itemSize: 1 }], parent, 128, -5);
  }

  alloc(): number {
    return this.pool.alloc();
  }

  /** 返回补位阴影原槽号，由场景同步持有该阴影的建筑引用。 */
  release(slot: number): number | undefined {
    return this.pool.release(slot);
  }

  get size(): number { return this.pool.size; }

  /** 无动画整场重建时回收全部投影槽位，避免按候选数量反复释放。 */
  clear(): void {
    this.pool.clear();
  }

  beginGeneration(): void {
    this.pool.beginGeneration();
  }

  /** 写入投影：中心 (x, z)、旋转 yaw、尺寸 (w, d)、强度 */
  set(slot: number, x: number, z: number, yaw: number, w: number, d: number, strength: number): void {
    const c = Math.cos(yaw);
    const s = Math.sin(yaw);
    // 绕 Y 旋转后按 w/d 缩放，地面高度略高于平台以避免深度冲突
    this.matrix.set(c * w, 0, s * d, x, 0, 1, 0, 0.03, -s * w, 0, c * d, z, 0, 0, 0, 1);
    this.pool.setMatrix(slot, this.matrix);
    this.pool.set('aShadow', slot, strength);
  }

  commit(): void {
    this.pool.commit();
  }

  dispose(): void {
    this.pool.dispose();
    this.geometry.dispose();
    this.material.dispose();
  }
}

interface Ring {
  slot: number;
  t: number;
  duration: number;
  strength: number;
}

/** 落位涟漪：方形光圈扩散后自动回收 */
export class RingLayer {
  private readonly geometry = groundQuad();
  private readonly material: THREE.ShaderMaterial;
  private readonly pool: InstancePool;
  private readonly rings: Ring[] = [];
  private readonly matrix = new THREE.Matrix4();

  constructor(parent: THREE.Object3D, uniforms: Record<string, THREE.IUniform>) {
    this.material = new THREE.ShaderMaterial({
      vertexShader: RING_VERTEX,
      fragmentShader: RING_FRAGMENT,
      uniforms: { uRingColor: uniforms.uFlash },
      transparent: true,
      depthWrite: false,
      blending: THREE.AdditiveBlending,
    });
    this.pool = new InstancePool('ring', this.geometry, this.material, [{ name: 'aRing', itemSize: 2 }], parent, 32, 15);
  }

  /** 在 (x, y, z) 处生成边长约 size×1.8 的涟漪 */
  spawn(x: number, y: number, z: number, size: number, strength = 1, duration = 0.7): void {
    if (this.rings.length > 160) return;
    const slot = this.pool.alloc();
    const s = Math.max(size, 0.8) * 1.8;
    this.matrix.makeScale(s, 1, s).setPosition(x, y + 0.05, z);
    this.pool.setMatrix(slot, this.matrix);
    this.pool.set('aRing', slot, 0.001, strength);
    this.rings.push({ slot, t: 0, duration, strength });
  }

  update(dt: number): void {
    for (let i = this.rings.length - 1; i >= 0; i--) {
      const ring = this.rings[i];
      ring.t += dt / ring.duration;
      if (ring.t >= 1) {
        const from = this.pool.release(ring.slot);
        if (from !== undefined) {
          const moved = this.rings.find(other => other.slot === from);
          if (moved) moved.slot = ring.slot;
        }
        this.rings.splice(i, 1);
      } else {
        this.pool.set('aRing', ring.slot, ring.t, ring.strength);
      }
    }
    this.pool.commit();
  }

  clear(): void {
    this.rings.length = 0;
    this.pool.clear();
    this.pool.commit();
  }

  dispose(): void {
    this.pool.dispose();
    this.geometry.dispose();
    this.material.dispose();
  }
}
