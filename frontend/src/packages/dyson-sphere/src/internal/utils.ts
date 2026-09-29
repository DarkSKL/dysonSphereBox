import * as THREE from 'three';

/**
 * 预乘混合：src * 1 + dst * (1 - srcAlpha)。
 * 着色器输出 alpha = a * uOver，uOver = 0 时等价于叠加，uOver = 1 时等价于普通透明叠放。
 */
export function applyPremultBlending<T extends THREE.Material>(material: T): T {
  material.transparent = true;
  material.depthWrite = false;
  material.blending = THREE.CustomBlending;
  material.blendEquation = THREE.AddEquation;
  material.blendSrc = THREE.OneFactor;
  material.blendDst = THREE.OneMinusSrcAlphaFactor;
  material.blendEquationAlpha = THREE.AddEquation;
  material.blendSrcAlpha = THREE.OneFactor;
  material.blendDstAlpha = THREE.OneMinusSrcAlphaFactor;
  return material;
}

/**
 * 渲染顺序（仅作用于透明物体；恒星与成就星球为不透明物体，靠深度缓冲自然遮挡）。
 * 以过球心、垂直于视线的平面把场景分为前后两半：后半球 → 日冕 → 前半球，
 * 前方的壳板才能正确遮挡后方的恒星、框架与粒子。
 */
export const RENDER_ORDER = {
  // 远侧：位于壳层后方，会被前后两层壳板覆盖
  achTrailFar: 0,
  swarmOrbitFar: 0,
  achFar: 1,
  swarmFar: 2,
  latticeBack: 3,
  motesBack: 4,
  // 壳层内侧（后半球）
  cellsBack: 5,
  framesBack: 6,
  nodesBack: 7,
  corona: 8,
  // 壳层外侧（前半球）
  cellsFront: 10,
  framesFront: 11,
  nodesFront: 12,
  aura: 13,
  latticeFront: 14,
  motesFront: 15,
  // 近侧
  swarmOrbitNear: 16,
  swarmNear: 17,
  achTrailNear: 18,
  achNear: 20,
} as const;

/** 前 / 后半球两份材质 */
export type HemiMaterials = [front: THREE.ShaderMaterial, back: THREE.ShaderMaterial];

/**
 * 生成前 / 后半球两份材质（共享 uniforms，仅 uHemi 不同）。
 * 着色器中用 dot(worldPos, cameraPosition) * uHemi < 0 判断并丢弃另一半。
 */
export function hemiMaterials(
  uniforms: Record<string, THREE.IUniform>,
  vertexShader: string,
  fragmentShader: string,
  extra: THREE.ShaderMaterialParameters = {},
): HemiMaterials {
  const make = (hemi: number) =>
    applyPremultBlending(
      new THREE.ShaderMaterial({
        uniforms: { ...uniforms, uHemi: { value: hemi } },
        vertexShader,
        fragmentShader,
        ...extra,
      }),
    );
  return [make(1), make(-1)];
}

/** 用前 / 后半球两份材质各生成一个对象（Mesh 或 Points），并设置渲染顺序 */
export function hemiPair<T extends THREE.Object3D>(
  create: (material: THREE.ShaderMaterial) => T,
  [front, back]: HemiMaterials,
  orderFront: number,
  orderBack: number,
): [T, T] {
  const f = create(front);
  const b = create(back);
  f.renderOrder = orderFront;
  b.renderOrder = orderBack;
  // 位置全部在着色器中计算，包围盒不可靠，关闭视锥剔除
  f.frustumCulled = false;
  b.frustumCulled = false;
  return [f, b];
}

/** 可复现的伪随机数发生器 */
export function mulberry32(seed: number): () => number {
  let a = seed >>> 0;
  return function () {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

/** 标准正态分布随机数（Box-Muller） */
export function gaussian(rand: () => number): number {
  const u = Math.max(rand(), 1e-6);
  return Math.sqrt(-2 * Math.log(u)) * Math.cos(Math.PI * 2 * rand());
}

/** 帧率无关的指数平滑 */
export function damp(current: number, target: number, lambda: number, dt: number): number {
  return THREE.MathUtils.damp(current, target, lambda, dt);
}

export function easeInOutCubic(t: number): number {
  return t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2;
}
