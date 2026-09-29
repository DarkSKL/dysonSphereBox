import { NOISE, PREMULT_OUTPUT } from '../../internal/shaders';

/** 光晕 / 进度环面片半边长（以天体半径为单位） */
export const HALO_EXTENT = 4.0;

export const BODY_VERT = /* glsl */ `
varying vec3 vNormalV;
varying vec3 vViewDir;
varying vec3 vLocal;
void main() {
  vLocal = normalize(position);
  vec4 mv = modelViewMatrix * vec4(position, 1.0);
  vNormalV = normalize(normalMatrix * normal);
  vViewDir = normalize(-mv.xyz);
  gl_Position = projectionMatrix * mv;
}
`;

// 成就星球：按等级使用不同地貌
// - 铜：熔岩岩石星球（暗色地壳 + 发光裂隙）
// - 银：冰晶星球（冰原 + 蓝白冰裂纹）
// - 金：气态巨行星（纬向条带 + 风暴）
// 未解锁：暗色、去饱和的岩石表面 + 全息扫描线
export const BODY_FRAG = /* glsl */ `
uniform vec3 uTier;
uniform vec3 uLocked;
uniform vec3 uLightDir;
uniform float uUnlock;
uniform float uHover;
uniform float uFlash;
uniform float uKind;
uniform float uSeed;
uniform float uTime;
varying vec3 vNormalV;
varying vec3 vViewDir;
varying vec3 vLocal;
${NOISE}
void main() {
  vec3 n = normalize(vNormalV);
  vec3 V = normalize(vViewDir);
  vec3 p = vLocal * 2.2 + uSeed * 17.0;
  float mu = clamp(dot(n, V), 0.0, 1.0);
  float ndl = dot(n, uLightDir);
  // 柔和的明暗交界线
  float light = smoothstep(-0.25, 0.6, ndl);

  vec3 surf;
  float glowMask = 0.0;
  if (uKind < 0.5) {
    float h = fbm(p * 1.4);
    float crack = 1.0 - smoothstep(0.0, 0.08, abs(snoise(p * 3.2 + h)));
    surf = mix(uTier * 0.22, uTier * 0.7, smoothstep(-0.3, 0.5, h));
    glowMask = crack * 0.9;
  } else if (uKind < 1.5) {
    float h = fbm(p * 1.8);
    float crack = 1.0 - smoothstep(0.0, 0.05, abs(snoise(p * 4.5)));
    surf = mix(uTier * 0.55, vec3(0.95, 0.98, 1.0), smoothstep(-0.2, 0.6, h));
    surf = mix(surf, vec3(0.45, 0.75, 1.0), crack * 0.6);
    glowMask = crack * 0.25;
  } else {
    float lat = vLocal.y;
    float turb = fbm(vec3(vLocal.xz * 2.0, lat * 3.0) + uSeed * 9.0 + vec3(0.0, 0.0, uTime * 0.03));
    float band = sin(lat * 18.0 + turb * 3.0);
    surf = mix(uTier * 0.55, mix(uTier, vec3(1.0, 0.95, 0.8), 0.45), band * 0.5 + 0.5);
    float storm = 1.0 - smoothstep(0.0, 0.22, length(vLocal - normalize(vec3(0.6, -0.25, 0.75))));
    surf = mix(surf, uTier * 0.35 + vec3(0.25, 0.08, 0.0), storm * 0.8);
  }

  // 未解锁：灰暗岩石 + 全息扫描线
  vec3 lockedSurf = uLocked * (0.55 + 0.45 * fbm(p * 1.6));
  float scan = pow(clamp(0.5 + 0.5 * sin(vLocal.y * 40.0 - uTime * 2.0), 0.0, 1.0), 8.0);
  vec3 col = mix(lockedSurf, surf, uUnlock) * (0.1 + 0.95 * light);
  col += uTier * glowMask * uUnlock * (0.6 + 0.4 * (1.0 - light));
  col += mix(uLocked * 1.6, uTier, uUnlock) * scan * (1.0 - uUnlock) * 0.35;

  // 大气边缘光：受光侧更亮
  float rim = pow(1.0 - mu, 3.0);
  vec3 atmo = mix(uLocked * 1.4, mix(uTier, vec3(0.6, 0.85, 1.0), uKind == 1.0 ? 0.5 : 0.15), uUnlock);
  col += atmo * rim * (0.25 + 0.9 * smoothstep(-0.4, 0.4, ndl)) * (0.5 + 0.5 * uUnlock);

  col = mix(col, vec3(1.0), clamp(uFlash, 0.0, 1.0) * 0.75);
  col *= 1.0 + uHover * 0.45;
  gl_FragColor = vec4(col, 1.0);
  #include <colorspace_fragment>
}
`;

export const HALO_VERT = /* glsl */ `
varying vec2 vUv;
void main() {
  vUv = uv;
  gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
}
`;

// 光晕 + 分段进度环 + 选中旋转虚线 + 解锁冲击波
export const HALO_FRAG = /* glsl */ `
uniform vec3 uTier;
uniform vec3 uRing;
uniform vec3 uTrack;
uniform float uProgress;
uniform float uUnlock;
uniform float uHover;
uniform float uSelected;
uniform float uFlash;
uniform float uWave;
uniform float uGlow;
uniform float uOver;
uniform float uTime;
varying vec2 vUv;
void main() {
  const float EXT = ${HALO_EXTENT.toFixed(2)};
  const float TAU = 6.2831853;
  vec2 q = (vUv - 0.5) * 2.0 * EXT;
  float r = length(q);

  // 光晕：已解锁的天体发光，悬停时增强
  float glowA = exp(-max(r - 1.0, 0.0) * 2.6) * (0.04 + 0.32 * uUnlock) * uGlow * (1.0 + uHover * 0.8);
  glowA *= step(0.98, r) * (1.0 - smoothstep(EXT * 0.7, EXT, r));

  // 进度环：从正上方顺时针，24 段刻度式
  float ringR = 1.85;
  float ringW = 0.14 + uHover * 0.04;
  float ringMask = 1.0 - smoothstep(ringW * 0.5, ringW * 0.5 + 0.06, abs(r - ringR));
  float ang = atan(q.x, q.y);
  float frac = (ang < 0.0 ? ang + TAU : ang) / TAU;
  float seg = step(0.12, fract(frac * 24.0));
  float on = step(frac, uProgress);
  vec3 ringCol = mix(uTrack, mix(uRing, uTier, uUnlock), on);
  float ringA = ringMask * seg * mix(0.5, 1.0, on) * (0.65 + uHover * 0.35 + uSelected * 0.3);

  // 选中：外圈旋转虚线
  float sel = 1.0 - smoothstep(0.03, 0.08, abs(r - 2.45));
  float dash = step(0.5, fract((ang + uTime * 0.8) / TAU * 16.0));
  float selA = sel * dash * uSelected * 0.9;

  // 解锁冲击波
  float waveR = 1.0 + uWave * 2.8;
  float waveA = (1.0 - smoothstep(0.0, 0.18, abs(r - waveR))) * (1.0 - uWave) * step(0.001, uWave);
  float flashA = exp(-r * 0.9) * uFlash * 1.2;

  float alpha = glowA + ringA + selA + waveA + flashA;
  vec3 col = (uTier * (glowA + selA + waveA) + ringCol * ringA + mix(uTier, vec3(1.0), 0.6) * flashA)
    / max(alpha, 1e-4);
  ${PREMULT_OUTPUT}
}
`;
