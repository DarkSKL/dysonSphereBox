import * as THREE from 'three';
import { PREMULT_OUTPUT } from '../internal/shaders';
import { damp, gaussian, hemiMaterials, hemiPair, mulberry32, RENDER_ORDER, type HemiMaterials } from '../internal/utils';
import type { Palette } from '../internal/palette';
import { OrbitRibbon } from './orbits';

export const MAX_ORBIT_LAYERS = 12;
const INNER_RADIUS = 2.3;
const OUTER_RADIUS = 3.05;
// 每条轨道上的太阳帆簇数（游戏中太阳帆是一批批发射的，沿轨道成段分布）
const CLUMPS = 9;

/** 开普勒式角速度：内圈快、外圈慢（与着色器保持一致） */
function orbitOmega(radius: number): number {
  return 0.55 * Math.pow(radius / 2.6, -1.5);
}

const SWARM_VERT = /* glsl */ `
attribute float aOrbit;
attribute float aRadius;
attribute float aPhase;
attribute float aLift;
attribute float aSeed;
uniform mat3 uOrbitMats[${MAX_ORBIT_LAYERS}];
uniform float uOrbitTime;
uniform float uTime;
uniform vec3 uAttractor;
uniform float uStrength;
uniform float uRadius;
uniform vec3 uBurstCenter;
uniform float uBurstAge;
uniform float uBurstPower;
uniform float uBurstRadius;
uniform vec3 uViewDir;
uniform float uPointScale;
uniform float uSize;
uniform float uHemi;
varying float vHeat;
varying float vGlint;
varying float vFacing;
void main() {
  float omega = 0.55 * pow(aRadius / 2.6, -1.5) * (0.97 + aSeed * 0.06);
  float ang = aPhase + uOrbitTime * omega;
  vec3 local = vec3(cos(ang) * aRadius, aLift, sin(ang) * aRadius);
  vec3 p = uOrbitMats[int(aOrbit + 0.5)] * local;
  p = (modelMatrix * vec4(p, 1.0)).xyz;

  // 屏幕平面基向量（相机俯仰有上限，不会正对上下方，可以用世界 Y 轴构造）
  vec3 sx = normalize(cross(uViewDir, vec3(0.0, 1.0, 0.0)));
  vec3 sy = cross(sx, uViewDir);

  // 吸引：在垂直于视线的平面内，把影响范围内的粒子拉向鼠标，
  // 聚成一团绕鼠标缓慢旋转的光尘云（而不是塌缩成一个点）
  vec3 d = p - uAttractor;
  vec3 depth = dot(d, uViewDir) * uViewDir;
  vec3 dp = d - depth;
  float r2 = dot(dp, dp) / (uRadius * uRadius);
  float f = clamp(exp(-r2 * 1.2) * uStrength, 0.0, 1.0);
  float cloudAng = aSeed * 6.2831 + uTime * (0.6 + aSeed * 0.9);
  float cloudR = uRadius * 0.28 * (0.25 + 0.75 * fract(aSeed * 7.13));
  vec3 cloud = (sx * cos(cloudAng) + sy * sin(cloudAng)) * cloudR;
  p = uAttractor + depth * (1.0 - 0.75 * f) + mix(dp, cloud, f * 0.9);
  vHeat = f;

  // 散开：点击处发出冲击，粒子沿径向被推开，随后回到各自轨道
  vec3 db = p - uBurstCenter;
  vec3 dbp = db - dot(db, uViewDir) * uViewDir;
  float lb = length(dbp);
  float env = (1.0 - exp(-uBurstAge * 12.0)) * exp(-uBurstAge * 1.7);
  float fall = exp(-lb * lb / (uBurstRadius * uBurstRadius));
  // 径向方向加一点随机偏转，避免散开的形状过于规整
  float jit = (aSeed - 0.5) * 0.9;
  vec3 radial = lb > 1e-4 ? dbp / lb : sx;
  vec3 tang = cross(uViewDir, radial);
  vec3 dir = radial * cos(jit) + tang * sin(jit);
  float kick = uBurstPower * env * fall * (0.55 + 0.9 * fract(aSeed * 3.71));
  p += dir * kick + uViewDir * (fract(aSeed * 5.3) - 0.5) * kick * 0.6;
  vHeat = max(vHeat, clamp(kick * 1.5, 0.0, 1.0));

  // 前 / 后半球分开绘制，后半球的太阳帆会被壳板遮挡
  if (dot(p, cameraPosition) * uHemi < 0.0) {
    gl_Position = vec4(2.0, 2.0, 2.0, 1.0);
    gl_PointSize = 0.0;
    return;
  }

  // 太阳帆翻转时的反光闪烁
  vGlint = pow(max(sin(uTime * (0.4 + aSeed * 0.8) + aSeed * 173.0), 0.0), 80.0);
  vFacing = dot(normalize(p), -uViewDir);
  vec4 mv = viewMatrix * vec4(p, 1.0);
  gl_Position = projectionMatrix * mv;
  float size = uSize * (0.6 + 0.8 * fract(aSeed * 7.31)) * (1.0 + vGlint * 2.0 + f * 0.6);
  gl_PointSize = max(size * uPointScale / -mv.z, 1.0);
}
`;

const SWARM_FRAG = /* glsl */ `
uniform vec3 uColor;
uniform vec3 uHot;
uniform float uAlpha;
uniform float uOver;
uniform float uHemi;
varying float vHeat;
varying float vGlint;
varying float vFacing;
void main() {
  float r = length(gl_PointCoord - 0.5) * 2.0;
  if (r > 1.0) discard;
  float a = exp(-r * r * 6.0);
  float heat = clamp(vHeat * 1.3 + vGlint, 0.0, 1.0);
  vec3 col = mix(uColor, uHot, heat);
  float face = mix(0.45, 1.0, smoothstep(-0.6, 0.6, vFacing));
  float back = uHemi > 0.0 ? 1.0 : 0.6;
  float alpha = a * face * back * (0.7 + heat * 1.6) * uAlpha;
  ${PREMULT_OUTPUT}
}
`;

/**
 * 戴森云图层：大量太阳帆粒子分布在多条倾斜的细轨道带上，轨道运动全部在 vertex shader 中计算。
 * 每条轨道配一条轨道带（细轨 + 太阳帆簇身后的拖尾光）。
 * 鼠标吸引与点击散开同样在着色器中计算，没有逐粒子速度状态。
 */
export class SwarmLayer {
  readonly group = new THREE.Group();

  private count = -1;
  private layers = -1;
  private orbitTime = 0;

  private strength = 0;
  private targetStrength = 0;
  private readonly attractor = new THREE.Vector3();
  private readonly targetAttractor = new THREE.Vector3();
  private returnSpeed = 2.5;
  private maxStrength = 1;
  // 散开后的冷却时间：期间不再吸引，让粒子真正飞散开
  private burstCooldown = 0;

  private readonly orbitMats: THREE.Matrix3[] = [];
  private readonly orbitRadii: number[] = [];
  private readonly clumpPhase: number[] = [];

  private readonly uniforms;
  private readonly materials: HemiMaterials;
  private points: THREE.Points[] = [];
  private geometry: THREE.BufferGeometry | null = null;
  private ribbons: OrbitRibbon[] = [];
  private readonly ribbonGroup = new THREE.Group();
  private palette: Palette | null = null;
  private pixel = 0.002;
  private readonly heads: number[] = [];

  constructor() {
    for (let i = 0; i < MAX_ORBIT_LAYERS; i++) this.orbitMats.push(new THREE.Matrix3());
    this.uniforms = {
      uOrbitMats: { value: this.orbitMats },
      uOrbitTime: { value: 0 },
      uTime: { value: 0 },
      uAttractor: { value: this.attractor },
      uStrength: { value: 0 },
      uRadius: { value: 1.2 },
      uBurstCenter: { value: new THREE.Vector3() },
      uBurstAge: { value: 99 },
      uBurstPower: { value: 1.1 },
      uBurstRadius: { value: 1.6 },
      uViewDir: { value: new THREE.Vector3(0, 0, -1) },
      uPointScale: { value: 400 },
      uSize: { value: 0.022 },
      uColor: { value: new THREE.Color() },
      uHot: { value: new THREE.Color() },
      uAlpha: { value: 1 },
      uOver: { value: 0 },
    };
    this.materials = hemiMaterials(this.uniforms, SWARM_VERT, SWARM_FRAG);
    this.group.add(this.ribbonGroup);
  }

  /** 按层数重新生成轨道平面；使用固定随机种子，层数变化时保持稳定 */
  private buildOrbits(layers: number): void {
    const rand = mulberry32(7);
    const euler = new THREE.Euler();
    const m4 = new THREE.Matrix4();
    for (let i = 0; i < MAX_ORBIT_LAYERS; i++) {
      const incl = (rand() * 2 - 1) * 0.75;
      const node = i * 2.39996 + rand() * 0.4; // 黄金角分散升交点
      euler.set(incl, node, 0, 'YXZ');
      this.orbitMats[i].setFromMatrix4(m4.makeRotationFromEuler(euler));
      const t = layers > 1 ? (i % layers) / (layers - 1) : 0.4;
      this.orbitRadii[i] = INNER_RADIUS + (OUTER_RADIUS - INNER_RADIUS) * t;
      this.clumpPhase[i] = rand() * Math.PI * 2;
    }

    this.ribbons.forEach((r) => {
      this.ribbonGroup.remove(...r.meshes);
      r.dispose();
    });
    this.ribbons = [];
    for (let i = 0; i < layers; i++) {
      const ribbon = new OrbitRibbon({
        matrix: this.orbitMats[i],
        radius: this.orbitRadii[i],
        widthPx: 2.2,
        base: 0.1,
        trailLen: 0.45,
        trailGain: 0.55,
        orderFront: RENDER_ORDER.swarmOrbitNear,
        orderBack: RENDER_ORDER.swarmOrbitFar,
      });
      ribbon.setPixel(this.pixel);
      this.ribbonGroup.add(...ribbon.meshes);
      this.ribbons.push(ribbon);
    }
    if (this.palette) this.applyPalette(this.palette);
  }

  /** 设置粒子数与轨道层数；两者都未变化时不重建 */
  setConfig(count: number, layers: number): void {
    layers = THREE.MathUtils.clamp(Math.round(layers), 1, MAX_ORBIT_LAYERS);
    if (count === this.count && layers === this.layers) return;
    if (layers !== this.layers) this.buildOrbits(layers);
    this.count = count;
    this.layers = layers;

    this.points.forEach((p) => this.group.remove(p));
    this.geometry?.dispose();

    const rand = mulberry32(2024);
    const orbit = new Float32Array(count);
    const radius = new Float32Array(count);
    const phase = new Float32Array(count);
    const lift = new Float32Array(count);
    const seed = new Float32Array(count);
    for (let i = 0; i < count; i++) {
      const l = Math.floor(rand() * layers);
      orbit[i] = l;
      // 细轨道带：径向与法向的散布都很小
      radius[i] = this.orbitRadii[l] + gaussian(rand) * 0.028;
      lift[i] = gaussian(rand) * 0.01;
      // 70% 的太阳帆成簇（簇头密、簇尾拖长），其余均匀散布在轨道上
      if (rand() < 0.7) {
        const clump = Math.floor(rand() * CLUMPS);
        const tail = -Math.pow(rand(), 1.8) * 0.9;
        phase[i] = this.clumpPhase[l] + (clump / CLUMPS) * Math.PI * 2 + tail + gaussian(rand) * 0.05;
      } else {
        phase[i] = rand() * Math.PI * 2;
      }
      seed[i] = rand();
    }
    const geo = new THREE.BufferGeometry();
    // position 仅用于占位，实际位置在着色器中计算
    geo.setAttribute('position', new THREE.BufferAttribute(new Float32Array(count * 3), 3));
    geo.setAttribute('aOrbit', new THREE.BufferAttribute(orbit, 1));
    geo.setAttribute('aRadius', new THREE.BufferAttribute(radius, 1));
    geo.setAttribute('aPhase', new THREE.BufferAttribute(phase, 1));
    geo.setAttribute('aLift', new THREE.BufferAttribute(lift, 1));
    geo.setAttribute('aSeed', new THREE.BufferAttribute(seed, 1));
    this.geometry = geo;
    this.points = hemiPair((m) => new THREE.Points(geo, m), this.materials, RENDER_ORDER.swarmNear, RENDER_ORDER.swarmFar);
    this.group.add(...this.points);
  }

  setAttractorParams(strength: number, radius: number, returnSpeed: number): void {
    this.maxStrength = strength;
    this.uniforms.uRadius.value = radius;
    this.returnSpeed = returnSpeed;
  }

  /** 更新引力点：point 为世界坐标，active 为是否吸引 */
  setAttractor(point: THREE.Vector3 | null, active: boolean): void {
    if (point) this.targetAttractor.copy(point);
    this.targetStrength = active ? 1 : 0;
  }

  /**
   * 在 point 处触发散开：被吸引聚拢的粒子沿径向炸开，随后回到各自轨道。
   * 冲击半径跟随引力半径，保证聚拢的粒子都能被推开。
   */
  burst(point: THREE.Vector3): void {
    const u = this.uniforms;
    u.uBurstCenter.value.copy(point);
    u.uBurstAge.value = 0;
    u.uBurstRadius.value = u.uRadius.value * 1.3;
    // 冷却期内吸引快速衰减：聚拢的光尘先被炸开，再回到各自轨道
    this.burstCooldown = 0.9;
  }

  /** pointScale：点精灵换算系数；pixelWorld：距离 1 处一个 CSS 像素的世界长度 */
  setViewport(pointScale: number, pixelWorld: number): void {
    this.uniforms.uPointScale.value = pointScale;
    this.pixel = pixelWorld;
    this.ribbons.forEach((r) => r.setPixel(pixelWorld));
  }

  applyPalette(p: Palette): void {
    this.palette = p;
    this.uniforms.uColor.value.copy(p.swarm);
    this.uniforms.uHot.value.copy(p.swarmHot);
    this.uniforms.uAlpha.value = p.swarmAlpha;
    this.uniforms.uOver.value = p.over;
    this.ribbons.forEach((r) => r.setStyle(p.orbitLine, p.swarmHot, p.orbitLineAlpha, p.over));
  }

  update(dt: number, time: number, camera: THREE.Camera, spinSpeed: number, attractEnabled: boolean): void {
    this.orbitTime += dt * spinSpeed;
    this.uniforms.uOrbitTime.value = this.orbitTime;
    this.uniforms.uTime.value = time;

    // 每条轨道带的拖尾跟随太阳帆簇头
    this.ribbons.forEach((r, i) => {
      const base = this.clumpPhase[i] + this.orbitTime * orbitOmega(this.orbitRadii[i]);
      this.heads.length = 0;
      for (let c = 0; c < CLUMPS; c++) this.heads.push(base + (c / CLUMPS) * Math.PI * 2);
      r.setHeads(this.heads);
      r.update(time);
    });

    this.uniforms.uBurstAge.value = Math.min(this.uniforms.uBurstAge.value + dt, 99);
    this.burstCooldown = Math.max(this.burstCooldown - dt, 0);

    // 进入时平滑增强（散开冷却期内不吸引），离开时按回弹速度衰减
    const target = attractEnabled && this.burstCooldown <= 0 ? this.targetStrength : 0;
    const lambda = target > this.strength ? 3 : this.burstCooldown > 0 ? 5 : this.returnSpeed;
    this.strength = damp(this.strength, target, lambda, dt);
    this.attractor.lerp(this.targetAttractor, 1 - Math.exp(-12 * dt));
    this.uniforms.uStrength.value = this.strength * this.maxStrength;
    camera.getWorldDirection(this.uniforms.uViewDir.value);
  }

  dispose(): void {
    this.geometry?.dispose();
    this.ribbons.forEach((r) => r.dispose());
    this.materials.forEach((m) => m.dispose());
  }
}
