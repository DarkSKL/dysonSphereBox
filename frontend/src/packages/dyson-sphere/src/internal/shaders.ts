/**
 * 共享 GLSL 片段。
 */

// 3D simplex 噪声（Ashima Arts / Stefan Gustavson，MIT）
export const NOISE = /* glsl */ `
vec3 mod289(vec3 x) { return x - floor(x * (1.0 / 289.0)) * 289.0; }
vec4 mod289(vec4 x) { return x - floor(x * (1.0 / 289.0)) * 289.0; }
vec4 permute(vec4 x) { return mod289(((x * 34.0) + 10.0) * x); }
vec4 taylorInvSqrt(vec4 r) { return 1.79284291400159 - 0.85373472095314 * r; }

float snoise(vec3 v) {
  const vec2 C = vec2(1.0 / 6.0, 1.0 / 3.0);
  const vec4 D = vec4(0.0, 0.5, 1.0, 2.0);
  vec3 i = floor(v + dot(v, C.yyy));
  vec3 x0 = v - i + dot(i, C.xxx);
  vec3 g = step(x0.yzx, x0.xyz);
  vec3 l = 1.0 - g;
  vec3 i1 = min(g.xyz, l.zxy);
  vec3 i2 = max(g.xyz, l.zxy);
  vec3 x1 = x0 - i1 + C.xxx;
  vec3 x2 = x0 - i2 + C.yyy;
  vec3 x3 = x0 - D.yyy;
  i = mod289(i);
  vec4 p = permute(permute(permute(
    i.z + vec4(0.0, i1.z, i2.z, 1.0))
    + i.y + vec4(0.0, i1.y, i2.y, 1.0))
    + i.x + vec4(0.0, i1.x, i2.x, 1.0));
  float n_ = 0.142857142857;
  vec3 ns = n_ * D.wyz - D.xzx;
  vec4 j = p - 49.0 * floor(p * ns.z * ns.z);
  vec4 x_ = floor(j * ns.z);
  vec4 y_ = floor(j - 7.0 * x_);
  vec4 x = x_ * ns.x + ns.yyyy;
  vec4 y = y_ * ns.x + ns.yyyy;
  vec4 h = 1.0 - abs(x) - abs(y);
  vec4 b0 = vec4(x.xy, y.xy);
  vec4 b1 = vec4(x.zw, y.zw);
  vec4 s0 = floor(b0) * 2.0 + 1.0;
  vec4 s1 = floor(b1) * 2.0 + 1.0;
  vec4 sh = -step(h, vec4(0.0));
  vec4 a0 = b0.xzyw + s0.xzyw * sh.xxyy;
  vec4 a1 = b1.xzyw + s1.xzyw * sh.zzww;
  vec3 p0 = vec3(a0.xy, h.x);
  vec3 p1 = vec3(a0.zw, h.y);
  vec3 p2 = vec3(a1.xy, h.z);
  vec3 p3 = vec3(a1.zw, h.w);
  vec4 norm = taylorInvSqrt(vec4(dot(p0, p0), dot(p1, p1), dot(p2, p2), dot(p3, p3)));
  p0 *= norm.x; p1 *= norm.y; p2 *= norm.z; p3 *= norm.w;
  vec4 m = max(0.5 - vec4(dot(x0, x0), dot(x1, x1), dot(x2, x2), dot(x3, x3)), 0.0);
  m = m * m;
  return 105.0 * dot(m * m, vec4(dot(p0, x0), dot(p1, x1), dot(p2, x2), dot(p3, x3)));
}

float fbm(vec3 p) {
  float s = 0.0;
  float a = 0.5;
  for (int i = 0; i < 5; i++) {
    s += a * snoise(p);
    p = p * 2.03 + vec3(1.7, 9.2, 3.1);
    a *= 0.5;
  }
  return s;
}
`;

export const HASH = /* glsl */ `
float hash12(vec2 p) {
  vec3 p3 = fract(vec3(p.xyx) * 0.1031);
  p3 += dot(p3, p3.yzx + 33.33);
  return fract((p3.x + p3.y) * p3.z);
}
`;

/**
 * 遮挡 + 发光输出：base 为遮挡层自身颜色，cover 为遮挡度（日夜都生效，用于真实的前后层次），
 * emit 为额外发光（线性）。配合 applyPremultBlending() 使用。
 */
export const COVER_OUTPUT = /* glsl */ `
  gl_FragColor = vec4(base * cover + emit, 1.0);
  #include <colorspace_fragment>
  gl_FragColor.a = clamp(cover, 0.0, 1.0);
`;

/** 抗锯齿细线：x 为整数处画线，线宽约 1 像素，过密时自动淡出避免摩尔纹 */
export const GRID_LINE = /* glsl */ `
float gridLine(float x) {
  float w = fwidth(x);
  float f = abs(fract(x + 0.5) - 0.5);
  return (1.0 - smoothstep(w * 0.5, w * 1.5, f)) * clamp(1.5 - w * 3.0, 0.0, 1.0);
}
`;

/**
 * 预乘输出：先做色彩空间转换，再按 uOver 在「叠加」和「普通透明」之间过渡。
 * 配合 applyPremultBlending() 使用；片段着色器需声明 uniform float uOver，并定义 col / alpha。
 *
 * 注意：GPU 上 pow 的底数为负时返回 NaN，Bloom 会把 NaN 扩散成整屏黑。
 * 所有着色器中 pow 的底数都要先 clamp 到 >= 0，求平方直接写成 x * x。
 */
export const PREMULT_OUTPUT = /* glsl */ `
  gl_FragColor = vec4(col, 1.0);
  #include <colorspace_fragment>
  // 普通透明模式下 alpha 不能超过 1，否则颜色会被放大
  float aOut = mix(alpha, min(alpha, 1.0), uOver);
  gl_FragColor = vec4(gl_FragColor.rgb * aOut, min(alpha, 1.0) * uOver);
`;
