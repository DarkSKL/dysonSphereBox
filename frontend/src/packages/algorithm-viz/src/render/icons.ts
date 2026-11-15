import * as THREE from 'three';
import { iconRect, IconSlots } from '../internal/icon-slots';
import type { VizObject } from '../internal/objects';
import type { AlgoVizIcon, AlgoVizIconSheet } from '../types';
import { InstancePool } from './pool';
import { ICON_FRAGMENT, ICON_VERTEX } from './shaders';

/**
 * 世界图标层：传送带入口与机器建筑上方的物品图标。
 * 全部图标共用一个实例池与一次绘制；精灵图加载失败或未提供时保持空层，场景其余部分不受影响。
 */

/** 图标在屏幕上的最小、最大边长（CSS 像素）：缩小取景仍可辨认，放大时不无限膨胀 */
const MIN_PIXELS = 12;
const MAX_PIXELS = 40;
/** 自然边长（未夹取前）低于此值完全隐藏，达到上限值时完全显示 */
const FADE_PIXELS: readonly [number, number] = [4, 8];
/** 精灵图每格四周内缩的像素：mipmap 缩小采样时不会混入相邻格的颜色 */
const CELL_INSET = 2;

export class IconLayer {
  private readonly geometry = new THREE.PlaneGeometry(1, 1);
  private readonly material: THREE.ShaderMaterial;
  private readonly pool: InstancePool;
  private readonly slots: IconSlots<VizObject>;
  private readonly matrix = new THREE.Matrix4();
  /** 逐帧写入时复用的 UV 暂存，避免每个图标每帧分配数组 */
  private readonly rect = [0, 0, 0, 0];
  private readonly sheet: AlgoVizIconSheet | undefined;
  private texture: THREE.Texture | null = null;
  private disposed = false;

  /** 精灵图已加载，可以分配并绘制图标 */
  ready = false;

  /** onReady 在精灵图加载完成后调用一次，场景据此为已有对象补建图标 */
  constructor(parent: THREE.Object3D, uniforms: Record<string, THREE.IUniform>, sheet: AlgoVizIconSheet | undefined, onReady: () => void) {
    this.sheet = sheet;
    this.material = new THREE.ShaderMaterial({
      vertexShader: ICON_VERTEX,
      fragmentShader: ICON_FRAGMENT,
      uniforms: {
        ...uniforms,
        uIconMap: { value: null },
        uWorldPerPixel: { value: 1 },
        uIconPx: { value: new THREE.Vector2(MIN_PIXELS, MAX_PIXELS) },
        uIconFade: { value: new THREE.Vector2(FADE_PIXELS[0], FADE_PIXELS[1]) },
      },
      transparent: true,
      depthTest: false,
      depthWrite: false,
    });
    this.pool = new InstancePool('icon', this.geometry, this.material, [{ name: 'aRect', itemSize: 4 }, { name: 'aIcon', itemSize: 2 }], parent, 64, 30);
    this.slots = new IconSlots<VizObject>(this.pool);
    if (sheet) this.load(sheet, onReady);
  }

  private load(sheet: AlgoVizIconSheet, onReady: () => void): void {
    new THREE.TextureLoader().load(
      sheet.url,
      (texture) => {
        if (this.disposed) {
          texture.dispose();
          return;
        }
        texture.colorSpace = THREE.SRGBColorSpace;
        texture.anisotropy = 4;
        this.texture = texture;
        this.material.uniforms.uIconMap.value = texture;
        this.ready = true;
        onReady();
      },
      undefined,
      () => console.warn('[algorithm-viz] 图标精灵图加载失败，不显示世界图标', sheet.url),
    );
  }

  /** 为对象分配图标槽位；已持有时保持不变 */
  alloc(obj: VizObject): void {
    this.slots.alloc(obj);
  }

  /** 释放对象的图标槽位，补位实例的持有关系同步迁移 */
  release(obj: VizObject): void {
    this.slots.release(obj);
  }

  /** 写入图标：锚点（尾尖）世界坐标、基准边长、整体透明度与失败染红强度 */
  set(slot: number, icon: AlgoVizIcon, x: number, y: number, z: number, size: number, alpha: number, fail: number): void {
    if (!this.sheet) return;
    this.matrix.makeScale(size, size, size).setPosition(x, y, z);
    this.pool.setMatrix(slot, this.matrix);
    const [u, v, du, dv] = iconRect(this.sheet, icon, CELL_INSET, this.rect);
    this.pool.set('aRect', slot, u, v, du, dv);
    this.pool.set('aIcon', slot, alpha, fail);
  }

  /** 每帧同步取景比例，用于把图标边长夹在屏幕像素范围内 */
  setView(worldPerPixel: number): void {
    this.material.uniforms.uWorldPerPixel.value = worldPerPixel;
  }

  commit(): void {
    this.pool.commit();
  }

  /** 整场重建时回收全部槽位，持有者回到无图标状态 */
  clear(): void {
    this.slots.reset();
    this.pool.clear();
    this.pool.commit();
  }

  beginGeneration(): void {
    this.pool.beginGeneration();
  }

  dispose(): void {
    this.disposed = true;
    this.slots.reset();
    this.pool.dispose();
    this.geometry.dispose();
    this.material.dispose();
    this.texture?.dispose();
    this.texture = null;
  }
}
