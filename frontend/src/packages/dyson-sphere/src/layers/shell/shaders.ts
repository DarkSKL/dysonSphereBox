import { NOISE, PREMULT_OUTPUT, COVER_OUTPUT, GRID_LINE } from '../../internal/shaders';

/** 壳层辉光面片半边长（以壳层半径为单位） */
export const AURA_EXTENT = 1.6;

// 半球判定：以过球心、垂直于视线的平面划分，uHemi = 1 保留前半球，-1 保留后半球
const HEMI_DISCARD = /* glsl */ `if (dot(vWorld, cameraPosition) * uHemi < 0.0) discard;`;

// 飞行路径：e 为缓动后的飞行进度（0 = 出发点，1 = 落位）
const FLY_PATH = /* glsl */ `
uniform float uRadius;
uniform float uFar;
uniform float uLateral;
uniform float uFlyDur;
float flyEase(float f) {
  float k = 1.0 - clamp(f, 0.0, 1.0);
  return 1.0 - k * k * k;
}
vec3 flyPath(vec3 center, vec3 tangent, float e) {
  float k = 1.0 - e;
  return center * mix(uFar, uRadius, e) + tangent * (k * k * uLateral);
}
`;

// 节点：信标光点 = 亮芯 + 细环 + 十字星芒
export const NODE_VERT = /* glsl */ `
attribute float aOrder;
uniform float uBuild;
uniform float uBand;
uniform float uPointScale;
uniform float uSize;
uniform float uHemi;
varying float vFill;
varying float vOrder;
void main() {
  vec4 wp = modelMatrix * vec4(position, 1.0);
  vFill = clamp((uBuild - aOrder) / uBand, 0.0, 1.0);
  vOrder = aOrder;
  if (dot(wp.xyz, cameraPosition) * uHemi < 0.0) {
    gl_Position = vec4(2.0, 2.0, 2.0, 1.0);
    gl_PointSize = 0.0;
    return;
  }
  vec4 mv = viewMatrix * wp;
  gl_Position = projectionMatrix * mv;
  float s = mix(0.5, 1.0, vFill) + vFill * (1.0 - vFill) * 2.0;
  gl_PointSize = uSize * s * uPointScale / -mv.z;
}
`;

export const NODE_FRAG = /* glsl */ `
uniform vec3 uColor;
uniform vec3 uHot;
uniform float uAlpha;
uniform float uOver;
uniform float uTime;
uniform float uHemi;
varying float vFill;
varying float vOrder;
void main() {
  vec2 c = (gl_PointCoord - 0.5) * 2.0;
  float r = length(c);
  if (r > 1.0) discard;
  float core = exp(-r * r * 30.0);
  float ring = (1.0 - smoothstep(0.03, 0.08, abs(r - 0.34))) * 0.35;
  float spikes = (exp(-abs(c.x) * 24.0) + exp(-abs(c.y) * 24.0)) * (1.0 - smoothstep(0.15, 1.0, r)) * 0.3;
  float blink = 0.75 + 0.25 * sin(uTime * 1.7 + vOrder * 90.0);
  float flash = vFill * (1.0 - vFill) * 4.0;
  float back = uHemi > 0.0 ? 1.0 : 0.45;
  float lit = (core + ring + spikes * blink) * vFill * back + flash * (core + spikes) * 1.2;
  float ghost = core * 0.14 * back;
  vec3 col = mix(uColor, uHot, clamp(core * 0.7 + flash * 0.5, 0.0, 1.0));
  float alpha = max(lit, ghost) * uAlpha;
  ${PREMULT_OUTPUT}
}
`;

// 结构梁 / 能量网格共用：面向相机的四边形，沿梁生长
export const BEAM_VERT = /* glsl */ `
attribute vec3 aA;
attribute vec3 aB;
attribute vec2 aQuad;
attribute float aOrder;
attribute float aLen;
uniform float uBuild;
uniform float uBand;
uniform float uWidth;
varying float vFill;
varying float vT;
varying float vSide;
varying float vOrder;
varying float vLen;
varying vec3 vWorld;
void main() {
  vec3 wa = (modelMatrix * vec4(aA, 1.0)).xyz;
  vec3 wb = (modelMatrix * vec4(aB, 1.0)).xyz;
  vec3 wp = mix(wa, wb, aQuad.x);
  vec3 dir = normalize(wb - wa);
  vec3 view = normalize(cameraPosition - wp);
  vec3 side = normalize(cross(dir, view));
  wp += side * aQuad.y * uWidth;
  vFill = clamp((uBuild - aOrder) / uBand, 0.0, 1.0);
  vT = aQuad.x;
  vSide = aQuad.y;
  vOrder = aOrder;
  vLen = aLen;
  vWorld = wp;
  gl_Position = projectionMatrix * viewMatrix * vec4(wp, 1.0);
}
`;

// 框架梁：细亮芯 + 柔光 + 桁架节段刻痕 + 流动能量脉冲 + 生长端头
export const FRAME_FRAG = /* glsl */ `
uniform vec3 uColor;
uniform vec3 uAlt;
uniform vec3 uHot;
uniform float uAlpha;
uniform float uOver;
uniform float uTime;
uniform float uHemi;
varying float vFill;
varying float vT;
varying float vSide;
varying float vOrder;
varying float vLen;
varying vec3 vWorld;
${GRID_LINE}
void main() {
  ${HEMI_DISCARD}
  float x = abs(vSide);
  float core = 1.0 - smoothstep(0.08, 0.34, x);
  float glow = exp(-x * 3.8) * 0.35;
  float built = step(vT, vFill);
  float tip = exp(-abs(vT - vFill) * vLen * 20.0) * step(0.001, vFill) * step(vFill, 0.999);
  float ticks = gridLine(vT * vLen * 16.0) * core;
  float pulse = pow(fract(vT * 0.6 - uTime * 0.3 + vOrder * 13.0), 22.0);
  float back = uHemi > 0.0 ? 1.0 : 0.5;
  float lit = built * back * (core * 0.38 + glow * 0.5 + ticks * 0.3 + pulse * (core + glow) * 1.2)
    + tip * (core + glow) * 2.0;
  float ghost = (core * 0.1 + glow * 0.05) * back;
  // 框架颜色沿壳面在两种蓝色之间缓慢流转
  float hue = 0.5 + 0.5 * sin(dot(normalize(vWorld), vec3(0.4, 1.0, 0.25)) * 3.0 + uTime * 0.25);
  vec3 tint = mix(uColor, uAlt, hue * 0.8);
  vec3 col = mix(tint, uHot, clamp(pulse + tip, 0.0, 1.0));
  float alpha = max(lit, ghost) * uAlpha;
  ${PREMULT_OUTPUT}
}
`;

// 外层能量网格：细线 + 流动虚线光 + 偶发光脉冲，整体随建造进度浮现
export const LATTICE_FRAG = /* glsl */ `
uniform vec3 uColor;
uniform vec3 uHot;
uniform float uAlpha;
uniform float uOver;
uniform float uTime;
uniform float uHemi;
uniform float uVis;
varying float vFill;
varying float vT;
varying float vSide;
varying float vOrder;
varying float vLen;
varying vec3 vWorld;
void main() {
  ${HEMI_DISCARD}
  float core = 1.0 - smoothstep(0.0, 1.0, abs(vSide));
  core *= core;
  float s = fract(vT * vLen * 3.0 - uTime * 0.22 + vOrder * 5.0);
  float dash = smoothstep(0.0, 0.12, s) * (1.0 - smoothstep(0.24, 0.4, s));
  float pulse = pow(fract(vT - uTime * 0.16 + vOrder * 7.0), 18.0);
  float back = uHemi > 0.0 ? 1.0 : 0.4;
  float alpha = core * (0.2 + dash * 0.4 + pulse * 1.3) * uVis * back * uAlpha;
  vec3 col = mix(uColor, uHot, clamp(pulse * 1.2 + dash * 0.25, 0.0, 1.0));
  ${PREMULT_OUTPUT}
}
`;

// 六边形壳板：每块壳板是一个以中心为顶点的三角扇，
// aBary 为扇形三角的重心坐标（x = 中心权重），1 - x 即"到板边的距离"，天然是六边形距离场。
// 壳板飞行时：从出发点沿径向飞来，边飞边旋转、放大，落位后闪光。
export const TILE_VERT = /* glsl */ `
attribute vec3 aCenter;
attribute vec3 aTan;
attribute vec3 aBary;
attribute float aSeed;
attribute float aSpin;
attribute float aLaunch;
uniform float uTime;
${FLY_PATH}
varying vec3 vBary;
varying float vFly;
varying float vLanded;
varying float vSeed;
varying vec3 vNormalW;
varying vec3 vWorld;
varying vec3 vLocal;
vec3 rotateAxis(vec3 v, vec3 k, float a) {
  return v * cos(a) + cross(k, v) * sin(a) + k * dot(k, v) * (1.0 - cos(a));
}
void main() {
  float age = uTime - aLaunch;
  if (aLaunch < -0.5 || age < 0.0) {
    gl_Position = vec4(2.0, 2.0, 2.0, 1.0);
    return;
  }
  float fly = clamp(age / uFlyDur, 0.0, 1.0);
  float e = flyEase(fly);
  vec3 off = position - aCenter * uRadius;
  off = rotateAxis(off, aCenter, (1.0 - e) * aSpin) * mix(0.3, 1.0, e);
  vec3 p = flyPath(aCenter, aTan, e) + off;

  vBary = aBary;
  vFly = fly;
  vLanded = age - uFlyDur;
  vSeed = aSeed;
  vLocal = aCenter;
  vec4 wp = modelMatrix * vec4(p, 1.0);
  vWorld = wp.xyz;
  vNormalW = normalize(mat3(modelMatrix) * aCenter);
  gl_Position = projectionMatrix * viewMatrix * wp;
}
`;

// 壳板着色：
// - 外表面（前半球）：深蓝色半透明面板，外缘折边发出青色光，内部有同心环、辐条、分格光伏阵列和中心发光核
// - 内表面（后半球）：被恒星照亮的蓝色内壁
// - 飞行中边缘高亮，落位瞬间闪光
export const TILE_FRAG = /* glsl */ `
uniform vec3 uCell;
uniform vec3 uEdge;
uniform vec3 uPanel;
uniform vec3 uRim;
uniform vec3 uLattice;
uniform vec3 uHot;
uniform float uAlpha;
uniform float uEmit;
uniform float uCover;
uniform float uTime;
uniform float uHemi;
varying vec3 vBary;
varying float vFly;
varying float vLanded;
varying float vSeed;
varying vec3 vNormalW;
varying vec3 vWorld;
varying vec3 vLocal;
${NOISE}
${GRID_LINE}
float ringAt(float d, float r) {
  float w = fwidth(d) * 1.5;
  return 1.0 - smoothstep(0.0, w, abs(d - r));
}
void main() {
  ${HEMI_DISCARD}
  float d = 1.0 - vBary.x;                    // 0 = 板心，1 = 板边
  if (d > 0.965) discard;                     // 板缝
  float side = min(vBary.y, vBary.z);         // 到辐条（中心 → 角点）的距离
  float u = vBary.z / max(vBary.y + vBary.z, 1e-4); // 沿板边的位置

  float lip = smoothstep(0.83, 0.88, d) * (1.0 - smoothstep(0.92, 0.955, d));
  float spoke = (1.0 - smoothstep(0.0, fwidth(side) * 1.5, side)) * smoothstep(0.2, 0.3, d) * (1.0 - smoothstep(0.52, 0.56, d));
  float ringIn = ringAt(d, 0.56);
  float ringOut = ringAt(d, 0.8);
  float cellMask = step(0.58, d) * (1.0 - step(0.8, d));
  float cells = max(gridLine(u * 3.0), ringAt(d, 0.69)) * cellMask;
  float coreDot = 1.0 - smoothstep(0.1, 0.15, d);
  float coreRing = ringAt(d, 0.2);
  // 光伏面板区域（分格内部）略亮，表现面板本身
  float pv = cellMask * (1.0 - cells);

  vec3 N = normalize(vNormalW);
  vec3 V = normalize(cameraPosition - vWorld);
  float ndv = abs(dot(N, V));
  float fres = clamp(1.0 - ndv, 0.0, 1.0);
  float rnd = vSeed;

  // 沿纬向流动的能量扫描
  float flowPhase = vLocal.y * 3.2 + snoise(vLocal * 1.7 + uTime * 0.04) * 1.6 - uTime * 0.5;
  float flow = pow(max(sin(flowPhase), 0.0), 16.0);
  float pulse = 0.75 + 0.25 * sin(uTime * (0.6 + rnd) + rnd * 40.0);
  float detail = ringIn * 0.5 + ringOut * 0.3 + cells * 0.45 + spoke * 0.4 + coreRing * 0.7;

  vec3 base;
  vec3 emit;
  float cover;
  if (uHemi > 0.0) {
    base = uPanel * (0.8 + 0.4 * rnd) * (1.0 + pv * 0.35);
    base += uRim * pow(fres, 1.6) * 0.18;
    cover = 0.68 * (0.9 + 0.2 * rnd);
    emit = uEdge * lip * (0.45 + 0.6 * pow(fres, 1.4));
    emit += uCell * detail * 0.42 * pulse;
    emit += uCell * pv * 0.04;
    emit += uEdge * coreDot * 0.6 * pulse;
    emit += uRim * pow(fres, 3.0) * 0.4;
  } else {
    base = uCell * (0.22 + 0.12 * rnd);
    cover = 0.45 * (0.85 + 0.3 * rnd);
    emit = uCell * (0.1 + 0.18 * ndv) + uEdge * lip * 0.4 + uCell * detail * 0.25;
  }
  emit += uLattice * flow * (0.08 + lip * 0.7 + cells * 0.4);

  // 飞行中：面板半透明、边缘高亮；落位：由边缘向中心扩散的闪光
  float appear = smoothstep(0.0, 0.18, vFly);
  float flying = 1.0 - step(1.0, vFly);
  // 大量壳板同时落位时闪光会叠加，只点亮边缘与扩散环，避免整片过曝
  emit += uHot * (lip + spoke * 0.4) * flying * 0.5;
  float since = max(vLanded, 0.0);
  float flash = step(0.0, vLanded) * exp(-since * 3.0);
  float wave = 1.0 - smoothstep(0.0, 0.1, abs(d - (1.0 - since * 1.8)));
  emit += uHot * flash * (0.06 + lip * 0.55 + wave * 0.45);
  cover *= mix(0.35, 1.0, smoothstep(0.6, 1.0, vFly)) * appear * uCover * uAlpha;
  emit *= appear * uEmit * uAlpha;
  ${COVER_OUTPUT}
}
`;

// 飞行拖尾粒子：跟在飞行中的壳板后方，沿同一路径滞后排列
export const SPARK_VERT = /* glsl */ `
attribute vec3 aCenter;
attribute vec3 aTan;
attribute float aK;
attribute float aSeed;
attribute float aLaunch;
uniform float uTime;
uniform float uPointScale;
uniform float uSize;
uniform float uHemi;
uniform float uLag;
uniform float uCount;
${FLY_PATH}
varying float vAlpha;
varying float vK;
void main() {
  float tileAge = uTime - aLaunch;
  float tileFly = tileAge / uFlyDur;
  float age = tileAge - aK * uLag;
  if (aLaunch < -0.5 || age < 0.0 || tileFly >= 1.0) {
    gl_Position = vec4(2.0, 2.0, 2.0, 1.0);
    gl_PointSize = 0.0;
    return;
  }
  float e = flyEase(age / uFlyDur);
  vec3 jitter = vec3(sin(aSeed * 91.0 + aK), cos(aSeed * 57.0 + aK * 1.7), sin(aSeed * 33.0 - aK)) * 0.02 * aK;
  vec4 wp = modelMatrix * vec4(flyPath(aCenter, aTan, e) + jitter, 1.0);
  if (dot(wp.xyz, cameraPosition) * uHemi < 0.0) {
    gl_Position = vec4(2.0, 2.0, 2.0, 1.0);
    gl_PointSize = 0.0;
    return;
  }
  float kf = aK / uCount;
  vAlpha = (1.0 - kf) * smoothstep(0.0, 0.1, tileFly) * (1.0 - smoothstep(0.8, 1.0, tileFly));
  vK = kf;
  vec4 mv = viewMatrix * wp;
  gl_Position = projectionMatrix * mv;
  gl_PointSize = max(uSize * mix(1.3, 0.45, kf) * uPointScale / -mv.z, 1.0);
}
`;

export const SPARK_FRAG = /* glsl */ `
uniform vec3 uColor;
uniform vec3 uHot;
uniform float uAlpha;
uniform float uOver;
uniform float uHemi;
varying float vAlpha;
varying float vK;
void main() {
  float r = length(gl_PointCoord - 0.5) * 2.0;
  if (r > 1.0) discard;
  float back = uHemi > 0.0 ? 1.0 : 0.5;
  vec3 col = mix(uHot, uColor, vK);
  float alpha = exp(-r * r * 5.0) * vAlpha * back * uAlpha * 1.2;
  ${PREMULT_OUTPUT}
}
`;

// 壳层外缘辉光：贴着轮廓的细亮边 + 向外弥散的柔光，色相沿轮廓缓慢流转
export const AURA_VERT = /* glsl */ `
varying vec2 vUv;
void main() {
  vUv = uv;
  gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
}
`;

export const AURA_FRAG = /* glsl */ `
uniform vec3 uA;
uniform vec3 uB;
uniform float uVis;
uniform float uAlpha;
uniform float uOver;
uniform float uTime;
varying vec2 vUv;
${NOISE}
void main() {
  const float EXT = ${AURA_EXTENT.toFixed(2)};
  vec2 q = (vUv - 0.5) * 2.0 * EXT;
  float r = length(q);
  float ang = atan(q.y, q.x);
  vec2 dir = vec2(cos(ang), sin(ang));
  // 注意：GPU 上 pow 的底数为负时返回 NaN，平方一律写成乘法
  float rd = (r - 1.0) / 0.03;
  float rim = exp(-rd * rd) * 0.3;
  float outer = exp(-max(r - 1.0, 0.0) * 7.0) * smoothstep(0.97, 1.0, r) * 0.2;
  float inner = smoothstep(1.0, 0.75, r) * smoothstep(0.45, 1.0, r) * 0.05;
  float shimmer = 0.7 + 0.3 * snoise(vec3(dir * 2.5, uTime * 0.12));
  float hue = 0.5 + 0.5 * sin(ang * 2.0 + uTime * 0.15);
  vec3 col = mix(uA, uB, hue);
  float alpha = (rim + outer + inner) * shimmer * uVis * uAlpha * (1.0 - smoothstep(EXT * 0.8, EXT, r));
  ${PREMULT_OUTPUT}
}
`;

// 漂浮光尘：在壳层内外缓慢漂移、闪烁
export const MOTE_VERT = /* glsl */ `
attribute vec3 aDir;
attribute float aRad;
attribute float aSpeed;
attribute float aSeed;
uniform float uTime;
uniform float uRadius;
uniform float uPointScale;
uniform float uSize;
uniform float uHemi;
varying float vTw;
varying float vSeed;
void main() {
  float a = uTime * aSpeed;
  float c = cos(a);
  float s = sin(a);
  vec3 d = vec3(c * aDir.x - s * aDir.z, aDir.y, s * aDir.x + c * aDir.z);
  float rr = uRadius * (aRad + 0.03 * sin(uTime * 0.7 + aSeed * 40.0));
  vec4 wp = modelMatrix * vec4(d * rr, 1.0);
  if (dot(wp.xyz, cameraPosition) * uHemi < 0.0) {
    gl_Position = vec4(2.0, 2.0, 2.0, 1.0);
    gl_PointSize = 0.0;
    return;
  }
  vTw = pow(clamp(0.5 + 0.5 * sin(uTime * (0.8 + aSeed * 2.0) + aSeed * 100.0), 0.0, 1.0), 3.0);
  vSeed = aSeed;
  vec4 mv = viewMatrix * wp;
  gl_Position = projectionMatrix * mv;
  gl_PointSize = max(uSize * (0.5 + vTw) * uPointScale / -mv.z, 1.0);
}
`;

export const MOTE_FRAG = /* glsl */ `
uniform vec3 uA;
uniform vec3 uB;
uniform vec3 uC;
uniform float uVis;
uniform float uAlpha;
uniform float uOver;
uniform float uHemi;
varying float vTw;
varying float vSeed;
void main() {
  float r = length(gl_PointCoord - 0.5) * 2.0;
  if (r > 1.0) discard;
  vec3 col = vSeed > 0.88 ? uC : mix(uA, uB, step(0.5, vSeed));
  float back = uHemi > 0.0 ? 1.0 : 0.5;
  float alpha = exp(-r * r * 6.0) * (0.2 + 0.8 * vTw) * uVis * back * uAlpha;
  ${PREMULT_OUTPUT}
}
`;
