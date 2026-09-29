import * as THREE from 'three';
import { PREMULT_OUTPUT } from '../internal/shaders';
import { hemiMaterials, hemiPair, type HemiMaterials } from '../internal/utils';

export const MAX_HEADS = 12;
const SEGMENTS = 320;
const TAU = Math.PI * 2;

// 轨道带：沿圆周的 billboard 条带，宽度按屏幕像素计算，远近粗细一致
const ORBIT_VERT = /* glsl */ `
attribute float aAngle;
attribute float aSide;
uniform mat3 uMat;
uniform float uRadius;
uniform float uWidthPx;
uniform float uPixel;
varying float vAngle;
varying float vSide;
varying vec3 vWorld;
void main() {
  vec3 local = vec3(cos(aAngle), 0.0, sin(aAngle)) * uRadius;
  vec3 wp = (modelMatrix * vec4(uMat * local, 1.0)).xyz;
  vec3 tangent = normalize(mat3(modelMatrix) * (uMat * vec3(-sin(aAngle), 0.0, cos(aAngle))));
  vec3 view = cameraPosition - wp;
  float dist = length(view);
  vec3 side = normalize(cross(tangent, view / dist));
  wp += side * aSide * uWidthPx * uPixel * dist;
  vAngle = aAngle;
  vSide = aSide;
  vWorld = wp;
  gl_Position = projectionMatrix * viewMatrix * vec4(wp, 1.0);
}
`;

// 轨道带着色：细亮芯 + 柔光；在运动体（heads）身后留下渐隐的光弧；缓慢流动的明暗起伏
const ORBIT_FRAG = /* glsl */ `
#define MAX_HEADS ${MAX_HEADS}
uniform vec3 uColor;
uniform vec3 uHot;
uniform float uAlpha;
uniform float uOver;
uniform float uTime;
uniform float uHemi;
uniform float uBase;
uniform float uTrailLen;
uniform float uTrailGain;
uniform float uHeads[MAX_HEADS];
uniform int uHeadCount;
varying float vAngle;
varying float vSide;
varying vec3 vWorld;
void main() {
  if (dot(vWorld, cameraPosition) * uHemi < 0.0) discard;
  const float TAU = 6.2831853;
  float x = abs(vSide);
  float profile = exp(-x * x * 22.0);
  float glow = exp(-x * 2.6) * 0.3;

  float trail = 0.0;
  float head = 0.0;
  for (int i = 0; i < MAX_HEADS; i++) {
    if (i >= uHeadCount) break;
    float d = mod(uHeads[i] - vAngle, TAU);
    trail = max(trail, exp(-d / uTrailLen) * (1.0 - smoothstep(TAU - 0.05, TAU, d)));
    head = max(head, exp(-min(d, TAU - d) * 60.0));
  }
  trail *= uTrailGain;

  float shimmer = 0.75 + 0.25 * sin(vAngle * 3.0 - uTime * 0.35);
  float back = uHemi > 0.0 ? 1.0 : 0.5;
  float alpha = (profile * (uBase * shimmer + trail * 1.1 + head * 0.8) + glow * (trail * 0.7 + uBase * 0.3))
    * back * uAlpha;
  vec3 col = mix(uColor, uHot, clamp(trail * 0.6 + head, 0.0, 1.0));
  ${PREMULT_OUTPUT}
}
`;

function buildRibbonGeometry(): THREE.BufferGeometry {
  const n = SEGMENTS + 1;
  const angle = new Float32Array(n * 2);
  const side = new Float32Array(n * 2);
  for (let i = 0; i < n; i++) {
    const a = (i / SEGMENTS) * TAU;
    angle[i * 2] = a;
    angle[i * 2 + 1] = a;
    side[i * 2] = -1;
    side[i * 2 + 1] = 1;
  }
  const index: number[] = [];
  for (let i = 0; i < SEGMENTS; i++) {
    const a = i * 2;
    index.push(a, a + 1, a + 2, a + 1, a + 3, a + 2);
  }
  const geo = new THREE.BufferGeometry();
  // position 仅作占位，实际位置在着色器中计算
  geo.setAttribute('position', new THREE.BufferAttribute(new Float32Array(n * 2 * 3), 3));
  geo.setAttribute('aAngle', new THREE.BufferAttribute(angle, 1));
  geo.setAttribute('aSide', new THREE.BufferAttribute(side, 1));
  geo.setIndex(index);
  return geo;
}

export interface OrbitRibbonOptions {
  /** 轨道平面旋转 */
  matrix: THREE.Matrix3;
  /** 轨道半径 */
  radius: number;
  /** 屏幕像素宽度 */
  widthPx?: number;
  /** 无运动体经过时的基础亮度 */
  base?: number;
  /** 拖尾长度（弧度，指数衰减常数） */
  trailLen?: number;
  /** 拖尾亮度 */
  trailGain?: number;
  orderFront: number;
  orderBack: number;
}

/**
 * 单条轨道带（前 / 后半球两个网格）。
 */
export class OrbitRibbon {
  readonly meshes: [THREE.Mesh, THREE.Mesh];

  private readonly geometry = buildRibbonGeometry();
  private readonly materials: HemiMaterials;
  private readonly uniforms: {
    uMat: { value: THREE.Matrix3 };
    uRadius: { value: number };
    uWidthPx: { value: number };
    uPixel: { value: number };
    uColor: { value: THREE.Color };
    uHot: { value: THREE.Color };
    uAlpha: { value: number };
    uOver: { value: number };
    uTime: { value: number };
    uBase: { value: number };
    uTrailLen: { value: number };
    uTrailGain: { value: number };
    uHeads: { value: number[] };
    uHeadCount: { value: number };
  };

  constructor(opts: OrbitRibbonOptions) {
    this.uniforms = {
      uMat: { value: opts.matrix.clone() },
      uRadius: { value: opts.radius },
      uWidthPx: { value: opts.widthPx ?? 3 },
      uPixel: { value: 0.002 },
      uColor: { value: new THREE.Color() },
      uHot: { value: new THREE.Color() },
      uAlpha: { value: 1 },
      uOver: { value: 0 },
      uTime: { value: 0 },
      uBase: { value: opts.base ?? 0.3 },
      uTrailLen: { value: opts.trailLen ?? 0.5 },
      uTrailGain: { value: opts.trailGain ?? 1 },
      uHeads: { value: new Array<number>(MAX_HEADS).fill(0) },
      uHeadCount: { value: 0 },
    };
    this.materials = hemiMaterials(this.uniforms, ORBIT_VERT, ORBIT_FRAG, { side: THREE.DoubleSide });
    this.meshes = hemiPair((m) => new THREE.Mesh(this.geometry, m), this.materials, opts.orderFront, opts.orderBack);
  }

  /** 设置运动体的角度（弧度，与轨道参数化一致） */
  setHeads(angles: readonly number[]): void {
    const n = Math.min(angles.length, MAX_HEADS);
    const arr = this.uniforms.uHeads.value;
    for (let i = 0; i < n; i++) arr[i] = ((angles[i] % TAU) + TAU) % TAU;
    this.uniforms.uHeadCount.value = n;
  }

  setStyle(color: THREE.Color, hot: THREE.Color, alpha: number, over: number): void {
    this.uniforms.uColor.value.copy(color);
    this.uniforms.uHot.value.copy(hot);
    this.uniforms.uAlpha.value = alpha;
    this.uniforms.uOver.value = over;
  }

  /** pixelWorld：距离为 1 处一个 CSS 像素对应的世界长度 */
  setPixel(pixelWorld: number): void {
    this.uniforms.uPixel.value = pixelWorld;
  }

  update(time: number): void {
    this.uniforms.uTime.value = time;
  }

  dispose(): void {
    this.geometry.dispose();
    this.materials.forEach((m) => m.dispose());
  }
}
