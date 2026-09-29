import * as THREE from 'three';
import { PREMULT_OUTPUT } from '../internal/shaders';
import { gaussian, hemiMaterials, hemiPair, mulberry32, type HemiMaterials } from '../internal/utils';

const PARTICLES = 520;

// 粒子拖尾：粒子从星球表面脱落，沿轨道向后飘散。
// 每个粒子的生命周期 t 在 [0, 1) 内循环：t = 0 时位于星球表面（后半球与两侧），
// 随后沿轨道向后移动、逐渐扩散，拖尾截面从星球直径开始向后展开。
// 位置完全在 vertex shader 中计算，position 属性仅作占位。
const TRAIL_VERT = /* glsl */ `
attribute float aT0;
attribute float aRate;
attribute vec3 aDir;
attribute float aSeed;
uniform mat3 uMat;
uniform float uRadius;
uniform float uHead;
uniform float uLen;
uniform float uBodyR;
uniform float uTime;
uniform float uUnlock;
uniform float uPointScale;
uniform float uSize;
uniform float uHemi;
varying float vT;
varying float vSeed;
void main() {
  float t = fract(aT0 + uTime * aRate);
  float ang = uHead - t * uLen;
  // 轨道局部坐标系：径向 / 法向 / 切向（运动方向）
  vec3 radial = vec3(cos(ang), 0.0, sin(ang));
  vec3 normal = vec3(0.0, 1.0, 0.0);
  vec3 tangent = vec3(-sin(ang), 0.0, cos(ang));

  // 从表面出发，先快后慢地向外扩散，并叠加缓慢湍流
  float expand = 1.03 + 1.6 * sqrt(t) * t + 0.35 * t;
  vec3 wob = vec3(
    sin(uTime * 1.3 + aSeed * 40.0 + t * 9.0),
    cos(uTime * 1.1 + aSeed * 23.0 + t * 7.0),
    sin(uTime * 0.9 + aSeed * 61.0 + t * 5.0)
  ) * 0.35 * t;
  vec3 d = (aDir + wob) * uBodyR * expand;
  vec3 local = radial * (uRadius + d.x) + normal * d.y + tangent * d.z;
  vec4 wp = modelMatrix * vec4(uMat * local, 1.0);

  // 未解锁的拖尾更稀疏；另一半球的粒子移出屏幕
  float density = mix(0.4, 1.0, uUnlock);
  if (aSeed > density || dot(wp.xyz, cameraPosition) * uHemi < 0.0) {
    gl_Position = vec4(2.0, 2.0, 2.0, 1.0);
    gl_PointSize = 0.0;
    return;
  }
  vT = t;
  vSeed = aSeed;
  vec4 mv = viewMatrix * wp;
  gl_Position = projectionMatrix * mv;
  // 少量亮火花更大，其余为细碎光尘；越往后越小
  float kind = aSeed < 0.08 ? 1.8 : 0.55 + 0.65 * fract(aSeed * 13.7);
  float size = uSize * kind * mix(0.9, 0.35, t);
  gl_PointSize = max(size * uPointScale / -mv.z, 1.0);
}
`;

const TRAIL_FRAG = /* glsl */ `
uniform vec3 uColor;
uniform vec3 uLocked;
uniform vec3 uTail;
uniform vec3 uCore;
uniform float uUnlock;
uniform float uGain;
uniform float uAlpha;
uniform float uOver;
uniform float uTime;
varying float vT;
varying float vSeed;
void main() {
  float r = length(gl_PointCoord - 0.5) * 2.0;
  if (r > 1.0) discard;
  float a = exp(-r * r * 5.0);
  // 刚脱离表面时淡入，避免星球表面出现一圈硬边
  float fade = pow(clamp(1.0 - vT, 0.0, 1.0), 1.3) * smoothstep(0.0, 0.08, vT);
  float twinkle = 0.7 + 0.3 * sin(uTime * (3.0 + vSeed * 5.0) + vSeed * 80.0);
  float spark = step(vSeed, 0.08);
  float gain = uGain * mix(0.45, 1.0, uUnlock);
  float alpha = a * fade * twinkle * gain * uAlpha * (1.0 + spark * 0.9);

  vec3 tier = mix(uLocked, uColor, uUnlock);
  vec3 col = mix(tier, uTail, smoothstep(0.15, 0.9, vT) * 0.75);
  col = mix(col, uCore, clamp((1.0 - vT) * (0.15 + spark * 0.6) * uUnlock, 0.0, 1.0));
  ${PREMULT_OUTPUT}
}
`;

export interface ParticleTrailOptions {
  /** 轨道平面旋转 */
  matrix: THREE.Matrix3;
  /** 轨道半径 */
  radius: number;
  /** 星球半径，拖尾截面从星球直径开始展开 */
  bodyRadius: number;
  /** 0~1 的随机种子 */
  seed: number;
  /** 与星球共享的解锁 uniform，解锁动画时拖尾同步由暗转亮、变密 */
  unlock: { value: number };
  orderFront: number;
  orderBack: number;
}

export interface TrailStyle {
  color: THREE.Color;
  locked: THREE.Color;
  tail: THREE.Color;
  core: THREE.Color;
  alpha: number;
  over: number;
}

/**
 * 成就节点的粒子拖尾（前 / 后半球两个 Points）。
 */
export class ParticleTrail {
  readonly meshes: [THREE.Points, THREE.Points];

  private readonly geometry: THREE.BufferGeometry;
  private readonly materials: HemiMaterials;
  private readonly uniforms;

  constructor(opts: ParticleTrailOptions) {
    const rand = mulberry32(Math.floor(opts.seed * 1e6) + 11);
    const t0 = new Float32Array(PARTICLES);
    const rate = new Float32Array(PARTICLES);
    const dir = new Float32Array(PARTICLES * 3);
    const seed = new Float32Array(PARTICLES);
    const v = new THREE.Vector3();
    for (let i = 0; i < PARTICLES; i++) {
      t0[i] = rand();
      // 生命周期约 3.5~5.5 秒
      rate[i] = 0.18 + rand() * 0.1;
      // 球面上的随机方向（径向 / 法向 / 切向），切向分量压到后半球：
      // 粒子从星球背面与两侧脱落，不会从前进方向穿过星球
      v.set(gaussian(rand), gaussian(rand), gaussian(rand)).normalize();
      v.z = -Math.abs(v.z) * 0.8;
      v.normalize().multiplyScalar(0.92 + rand() * 0.16);
      v.toArray(dir, i * 3);
      seed[i] = rand();
    }
    const geo = new THREE.BufferGeometry();
    geo.setAttribute('position', new THREE.BufferAttribute(new Float32Array(PARTICLES * 3), 3));
    geo.setAttribute('aT0', new THREE.BufferAttribute(t0, 1));
    geo.setAttribute('aRate', new THREE.BufferAttribute(rate, 1));
    geo.setAttribute('aDir', new THREE.BufferAttribute(dir, 3));
    geo.setAttribute('aSeed', new THREE.BufferAttribute(seed, 1));
    this.geometry = geo;

    this.uniforms = {
      uMat: { value: opts.matrix.clone() },
      uRadius: { value: opts.radius },
      uHead: { value: 0 },
      uLen: { value: 1 },
      uBodyR: { value: opts.bodyRadius },
      uTime: { value: 0 },
      uUnlock: opts.unlock,
      uPointScale: { value: 400 },
      uSize: { value: 0.045 },
      uColor: { value: new THREE.Color() },
      uLocked: { value: new THREE.Color() },
      uTail: { value: new THREE.Color() },
      uCore: { value: new THREE.Color() },
      uGain: { value: 1 },
      uAlpha: { value: 1 },
      uOver: { value: 0 },
    };
    this.materials = hemiMaterials(this.uniforms, TRAIL_VERT, TRAIL_FRAG);
    this.meshes = hemiPair((m) => new THREE.Points(geo, m), this.materials, opts.orderFront, opts.orderBack);
  }

  /**
   * @param head 运动体所在角度
   * @param length 拖尾弧长（弧度）
   * @param gain 亮度倍率
   * @param bodyRadius 星球当前半径（悬停放大时拖尾截面同步变大）
   */
  set(head: number, length: number, gain: number, time: number, bodyRadius: number): void {
    const u = this.uniforms;
    u.uHead.value = head;
    u.uLen.value = length;
    u.uGain.value = gain;
    u.uTime.value = time;
    u.uBodyR.value = bodyRadius;
  }

  setStyle(s: TrailStyle): void {
    const u = this.uniforms;
    u.uColor.value.copy(s.color);
    u.uLocked.value.copy(s.locked);
    u.uTail.value.copy(s.tail);
    u.uCore.value.copy(s.core);
    u.uAlpha.value = s.alpha;
    u.uOver.value = s.over;
  }

  setPointScale(pointScale: number): void {
    this.uniforms.uPointScale.value = pointScale;
  }

  dispose(): void {
    this.geometry.dispose();
    this.materials.forEach((m) => m.dispose());
  }
}
