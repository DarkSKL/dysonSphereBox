import * as THREE from 'three';
import { damp } from './math';

/**
 * 等距相机：正交投影、真等距俯角（约 35.26°），方位角停靠在 45° + k·90°。
 *
 * - 自动取景：跟随场景包围范围平滑移动与缩放，用户平移或缩放后暂停，resetView 恢复
 * - 左键拖动旋转视角，松手后像纪念碑谷的机关一样吸附到最近的 90° 档位
 * - 右键拖动平移，滚轮以指针位置为锚点缩放
 */

const ELEVATION = Math.atan(1 / Math.SQRT2);
const DISTANCE = 600;
const QUARTER = Math.PI / 2;
const ROTATE_SPEED = 0.008;
const ZOOM_MIN = 0.3;
const ZOOM_MAX = 8;

export class IsoCameraRig {
  readonly camera: THREE.OrthographicCamera;
  /** 相机在世界中的右方向（水平），供着色器区分左右侧面 */
  readonly viewRight = new THREE.Vector3(1, 0, 0);
  /** 由观察点指向相机的方向，供着色器计算轮廓光 */
  readonly viewDir = new THREE.Vector3(0, 1, 0);
  autoFrame = true;

  private width = 1;
  private height = 1;
  /** 画面顶部被覆盖层占用的像素高度，取景时让出这部分空间 */
  private padTop = 0;
  private azimuth = Math.PI / 4;
  private azimuthGoal = Math.PI / 4;
  private dragging = false;
  private zoom = 1;
  private zoomGoal = 1;
  /** 自动取景得到的视口高度（世界单位） */
  private viewSize = 40;
  private viewSizeGoal = 40;
  private readonly target = new THREE.Vector3();
  private readonly targetGoal = new THREE.Vector3();
  private readonly right = new THREE.Vector3();
  private readonly up = new THREE.Vector3();
  private readonly dir = new THREE.Vector3();
  private readonly corner = new THREE.Vector3();

  constructor() {
    this.camera = new THREE.OrthographicCamera(-1, 1, 1, -1, 1, DISTANCE * 3);
    this.update(1, true);
  }

  setViewport(width: number, height: number): void {
    this.width = Math.max(width, 1);
    this.height = Math.max(height, 1);
  }

  /** 设置顶部留白（CSS 像素），用于避让覆盖在画面上方的工具栏 */
  setPadding(top: number): void {
    this.padTop = Math.max(0, top);
  }

  /** 每像素对应的世界长度 */
  get worldPerPixel(): number {
    return this.viewSize / this.zoom / this.height;
  }

  /** 目标方位下的屏幕右、上方向（世界坐标），供外部按对象逐个计算投影范围 */
  screenAxes(right: THREE.Vector3, up: THREE.Vector3): void {
    this.axes(this.azimuthGoal);
    right.copy(this.right);
    up.copy(this.up);
  }

  /**
   * 按屏幕投影范围取景（仅自动取景时生效）。
   * minR/maxR、minU/maxU 为场景各点在 screenAxes 返回方向上的投影极值，比包围盒更贴合高低错落的建筑。
   */
  frameProjected(minR: number, maxR: number, minU: number, maxU: number, immediate = false): void {
    if (!this.autoFrame || !Number.isFinite(minR + maxR + minU + maxU)) return;
    const usable = Math.max(this.height - this.padTop, this.height * 0.5);
    const aspect = this.width / usable;
    const fit = Math.max(maxU - minU, (maxR - minR) / aspect) * 1.12;
    this.viewSizeGoal = Math.max(16, (fit * this.height) / usable);
    this.axes(this.azimuthGoal);
    this.targetGoal.copy(this.right).multiplyScalar((minR + maxR) / 2).addScaledVector(this.up, (minU + maxU) / 2);
    this.zoomGoal = 1;
    if (immediate) {
      this.viewSize = this.viewSizeGoal;
      this.target.copy(this.targetGoal);
      this.zoom = 1;
    }
  }

  /** 以 90° 为步进旋转（正数逆时针） */
  rotate(steps: number): void {
    this.azimuthGoal = this.snap(this.azimuthGoal) + steps * QUARTER;
  }

  beginRotate(): void {
    this.dragging = true;
  }

  dragRotate(dx: number): void {
    this.azimuth -= dx * ROTATE_SPEED;
    this.azimuthGoal = this.azimuth;
  }

  endRotate(): void {
    this.dragging = false;
    this.azimuthGoal = this.snap(this.azimuth);
  }

  /** 按屏幕像素平移，画面跟随指针移动 */
  pan(dx: number, dy: number): void {
    this.autoFrame = false;
    const k = this.worldPerPixel;
    this.targetGoal.addScaledVector(this.right, -dx * k).addScaledVector(this.up, dy * k);
    this.target.addScaledVector(this.right, -dx * k).addScaledVector(this.up, dy * k);
  }

  /** 以容器内坐标 (px, py) 为锚点缩放 */
  zoomAt(factor: number, px: number, py: number): void {
    this.autoFrame = false;
    const before = this.viewSize / this.zoomGoal / this.height;
    this.zoomGoal = THREE.MathUtils.clamp(this.zoomGoal * factor, ZOOM_MIN, ZOOM_MAX);
    const after = this.viewSize / this.zoomGoal / this.height;
    const sx = px - this.width / 2;
    const sy = this.height / 2 - py;
    this.targetGoal.addScaledVector(this.right, sx * (before - after)).addScaledVector(this.up, sy * (before - after));
  }

  /** 恢复自动取景与默认方位 */
  reset(): void {
    this.autoFrame = true;
    this.zoomGoal = 1;
    this.azimuthGoal = this.snap(this.azimuthGoal);
  }

  update(dt: number, immediate = false): void {
    const fast = immediate ? 1e6 : 1;
    if (!this.dragging) this.azimuth = damp(this.azimuth, this.azimuthGoal, 7 * fast, dt);
    this.zoom = damp(this.zoom, this.zoomGoal, 9 * fast, dt);
    this.viewSize = damp(this.viewSize, this.viewSizeGoal, 3 * fast, dt);
    this.target.x = damp(this.target.x, this.targetGoal.x, 4 * fast, dt);
    this.target.y = damp(this.target.y, this.targetGoal.y, 4 * fast, dt);
    this.target.z = damp(this.target.z, this.targetGoal.z, 4 * fast, dt);

    this.axes(this.azimuth);
    this.viewRight.copy(this.right);
    this.viewDir.copy(this.dir);
    const cam = this.camera;
    // 顶部留白：视点沿屏幕上方向偏移半个留白，使内容居中于剩余区域
    this.corner.copy(this.target).addScaledVector(this.up, (this.padTop / 2) * this.worldPerPixel);
    cam.position.copy(this.corner).addScaledVector(this.dir, DISTANCE);
    cam.up.set(0, 1, 0);
    cam.lookAt(this.corner);
    const halfH = this.viewSize / this.zoom / 2;
    const halfW = halfH * (this.width / this.height);
    cam.left = -halfW;
    cam.right = halfW;
    cam.top = halfH;
    cam.bottom = -halfH;
    cam.updateProjectionMatrix();
    cam.updateMatrixWorld();
  }

  /** 方位角是否仍在转动（用于判断是否需要持续渲染） */
  get moving(): boolean {
    return this.dragging || Math.abs(this.azimuth - this.azimuthGoal) > 1e-4 || Math.abs(this.zoom - this.zoomGoal) > 1e-4
      || Math.abs(this.viewSize - this.viewSizeGoal) > 1e-3 || this.target.distanceToSquared(this.targetGoal) > 1e-6;
  }

  /** 视线方向、屏幕右方向与屏幕上方向 */
  private axes(azimuth: number): void {
    const ce = Math.cos(ELEVATION);
    this.dir.set(ce * Math.sin(azimuth), Math.sin(ELEVATION), ce * Math.cos(azimuth));
    this.right.set(Math.cos(azimuth), 0, -Math.sin(azimuth));
    this.up.crossVectors(this.dir, this.right).normalize();
  }

  private snap(azimuth: number): number {
    return Math.round((azimuth - Math.PI / 4) / QUARTER) * QUARTER + Math.PI / 4;
  }
}
