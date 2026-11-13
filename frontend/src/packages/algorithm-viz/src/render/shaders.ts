/**
 * 着色器：纪念碑谷式几何构图 + 戴森球计划式太空科幻配色。
 *
 * - 三面三色：屏幕坐标导数求出平直法线，顶面最亮、左右侧面依次变暗，暗部混入冷色阴影
 * - 原生配色机身：与游戏预览器同一套公式，材质颜色只染涂装遮罩区，其余保持贴图自带的白灰色；
 *   白色主光 + 暗环境光，ORM 提供遮蔽 / 粗糙 / 金属度，法线贴图补表面凹凸，未通电的建筑不发光
 * - 全息投影：生成中的临时建筑显示为蓝色扫描线全息体，提交正式结果后实体化
 * - 地表：暗色面板叠加青色建造网格，平台边缘与侧面有流动的灯带
 * 所有建筑实例共用一张颜色索引表，主题切换只需插值 uniform。
 */

/** 公共片段：调色、雾、抖动透明、全息与自发光参数 */
const COMMON = /* glsl */ `
uniform vec3 uShadow;
uniform vec3 uLight;
uniform vec3 uHaze;
uniform vec3 uFlash;
uniform vec3 uGhost;
uniform vec3 uSelect;
uniform vec3 uSteel;
uniform vec3 uViewRight;
uniform vec3 uViewDir;
uniform float uHazeTop;
uniform float uHazeBottom;
uniform float uTime;
// x 自发光强度、y 星空强度、z 投影不透明度、w 轮廓光强度
uniform vec4 uParams;

// 4×4 Bayer 阈值，用于不透明物体的溶解（保持深度正确，无需排序）
float bayer2(vec2 a) { a = floor(a); return fract(dot(a, vec2(0.5, a.y * 0.75))); }
float bayer4(vec2 a) { return bayer2(0.5 * a) * 0.25 + bayer2(a); }

vec3 flatNormal(vec3 world) {
  return normalize(cross(dFdx(world), dFdy(world)));
}

// 主光方向：与片元里的 lightV 同一公式，供法线细节明暗使用
vec3 lightVecFor(vec3 n) {
  return normalize(normalize(uViewDir) * 0.5 - uViewRight * 0.6 + vec3(0.0, 1.0, 0.0));
}

// 法线贴图：用屏幕导数构造切线空间（无需顶点切线），返回扰动后的世界空间法线
vec3 bumpNormal(vec3 n, vec3 world, vec2 uv, vec3 tangentNormal, float strength) {
  vec3 dp1 = dFdx(world);
  vec3 dp2 = dFdy(world);
  vec2 duv1 = dFdx(uv);
  vec2 duv2 = dFdy(uv);
  vec3 dp2perp = cross(dp2, n);
  vec3 dp1perp = cross(n, dp1);
  vec3 t = dp2perp * duv1.x + dp1perp * duv2.x;
  vec3 b = dp2perp * duv1.y + dp1perp * duv2.y;
  float invmax = inversesqrt(max(max(dot(t, t), dot(b, b)), 1e-12));
  vec3 tn = tangentNormal;
  tn.xy *= strength;
  return normalize(mat3(t * invmax, b * invmax, n) * tn);
}

// 三面色调系数：顶面 1.0，左侧 ~0.84，右侧 ~0.6，底面最暗；近地面略压暗，模拟环境遮蔽
float mvTone(vec3 n, float height) {
  float up = n.y;
  float side = dot(n, uViewRight);
  float tone = mix(0.86, 0.58, smoothstep(-0.55, 0.55, side));
  tone = mix(tone, 1.0, smoothstep(0.3, 0.82, up));
  tone = mix(tone, 0.44, 1.0 - smoothstep(-0.75, -0.25, up));
  tone *= mix(0.82, 1.0, smoothstep(0.0, 2.4, height));
  return tone;
}

vec3 mvShade(vec3 base, vec3 n, float height) {
  float tone = mvTone(n, height);
  vec3 lit = base * uLight;
  vec3 dark = mix(base * 0.32, uShadow, 0.55);
  return mix(dark, lit, tone);
}

// 失败候选：保留明暗层次，整体染成警示红
vec3 failTint(vec3 col, float f) {
  float l = dot(col, vec3(0.3333));
  return mix(col, vec3(1.0, 0.16, 0.12) * (0.2 + l * 1.5), clamp(f, 0.0, 1.0) * 0.85);
}

vec3 mvHaze(vec3 col, float y) {
  float h = 1.0 - smoothstep(uHazeBottom, uHazeTop, y);
  return mix(col, uHaze, h);
}

// 全息投影：沿高度上移的亮带
float holoSweep(float y) {
  float f = fract(y * 0.32 - uTime * 0.45);
  return smoothstep(0.0, 0.06, f) * (1.0 - smoothstep(0.06, 0.22, f));
}

// 抗锯齿网格线：coord 为以线距为单位的坐标，width 为线宽（线距比例）
float gridLine(float coord, float width) {
  float d = abs(fract(coord - 0.5) - 0.5);
  float fw = fwidth(coord);
  return 1.0 - smoothstep(width * 0.5, width * 0.5 + fw, d);
}
`;

/** 建筑实例：游戏原生底色 / 自发光贴图 + 状态（可见度、闪光、全息、选中） */
export const BUILDING_VERTEX = /* glsl */ `
attribute vec4 aState;
attribute float aFail;
varying vec3 vWorld;
varying vec2 vUv;
varying vec4 vState;
varying float vFail;
void main() {
  vec4 world = vec4(position, 1.0);
  #ifdef USE_INSTANCING
    world = instanceMatrix * world;
  #endif
  world = modelMatrix * world;
  vWorld = world.xyz;
  vUv = uv;
  vState = aState;
  vFail = aFail;
  gl_Position = projectionMatrix * viewMatrix * world;
}
`;

export const BUILDING_FRAGMENT = /* glsl */ `
${COMMON}
uniform sampler2D uMap;
uniform sampler2D uEmissiveMap;
uniform sampler2D uOrm;
uniform vec3 uTint;
uniform vec3 uEmissiveTint;
uniform float uHasMap;
uniform float uHasEmissive;
uniform float uHasOrm;
// 该分组是否有游戏原生颜色
uniform float uNative;
// 自发光依赖供电（未通电的建筑不发光）
uniform float uUnlit;
uniform sampler2D uOriginalLight;
uniform float uHasOriginalLight;
uniform float uSolidEffect;
uniform float uLightShape;
uniform sampler2D uSecondaryLight;
uniform float uHasSecondaryLight;
uniform vec3 uLabTint;
uniform float uLabLight;
// 涂装遮罩（亮处为涂装区）与法线贴图
uniform sampler2D uMask;
uniform sampler2D uNormal;
uniform float uHasMask;
uniform float uHasNormal;
// 0 机身、1 玻璃罩、2 特效面片（加法叠加）
uniform float uKind;
varying vec3 vWorld;
varying vec2 vUv;
varying vec4 vState;
varying float vFail;
void main() {
  if (vState.x < 0.999 && vState.x <= bayer4(gl_FragCoord.xy)) discard;
  float holo = vState.z;
  // 全息体每三行像素镂空一行，形成扫描线
  if (holo > 0.5 && mod(floor(gl_FragCoord.y), 3.0) < 1.0) discard;
  // 特效面片：游戏中是黑底发光贴图，按亮度加法叠加，不写深度
  if (uKind > 1.5) {
    if (uLightShape > 0.5) {
      float band = uLightShape < 1.5
        ? exp(-pow((length(vUv - 0.5) - 0.38) * 20.0, 2.0))
        : smoothstep(0.0, 0.13, vUv.y) * (1.0 - smoothstep(0.35, 1.0, vUv.y));
      vec2 flowUv = uLightShape < 1.5 ? vUv : fract(vUv + vec2(0.0, -uTime * 0.08));
      vec4 layer = uHasOriginalLight > 0.5 ? texture2D(uOriginalLight, flowUv) : vec4(1.0);
      float pattern = dot(layer.rgb, vec3(0.2126, 0.7152, 0.0722)) * layer.a;
      float strength = band * (0.20 + pattern * 0.55) * vState.x * (1.0 - holo * 0.85);
      if (strength < 0.01) discard;
      gl_FragColor = vec4(failTint(uTint, vFail), strength);
      #include <colorspace_fragment>
      return;
    }
    if (uLabLight > 0.5 && uHasOriginalLight > 0.5) {
      vec4 lab = texture2D(uOriginalLight, vUv);
      float energy = max(lab.r, max(lab.g, lab.b)) * sqrt(lab.a);
      float opacity = energy * 1.8 * vState.x * (1.0 - holo * 0.85);
      if (opacity < 0.012) discard;
      gl_FragColor = vec4(failTint(uLabTint, vFail), min(opacity, 0.8));
      #include <colorspace_fragment>
      return;
    }
    if (uHasOriginalLight < 0.5 && uHasMap < 0.5 && uSolidEffect < 0.5) discard;
    vec4 texel = uHasOriginalLight > 0.5 ? texture2D(uOriginalLight, vUv) : vec4(1.0);
    vec3 fx = uHasOriginalLight > 0.5 ? pow(texel.rgb, vec3(2.2)) :
      (uSolidEffect > 0.5 ? uTint : texture2D(uMap, vUv).rgb);
    float strength = uSolidEffect > 0.5 ? 0.18 : max(fx.r, max(fx.g, fx.b)) * texel.a * 0.7;
    float a = strength * vState.x * (1.0 - holo * 0.85);
    if (a < 0.02) discard;
    gl_FragColor = vec4(failTint(fx, vFail) * 0.9, a);
    #include <colorspace_fragment>
    return;
  }
  vec3 n = flatNormal(vWorld);
  // 玻璃罩：纯白贴图不能当底色，改为冷色半透明（隔点镂空，保持深度正确且无需排序）
  if (uKind > 0.5) {
    if (bayer4(gl_FragCoord.xy) < 0.55) discard;
    vec3 gcol = mvShade(mix(vec3(0.42, 0.6, 0.78), uTint, 0.35), n, vWorld.y);
    if (uHasOriginalLight > 0.5) {
      vec4 e = texture2D(uOriginalLight, vUv);
      gcol += pow(e.rgb, vec3(2.2)) * e.a * 0.4;
    }
    gcol = mvHaze(failTint(gcol, vFail), vWorld.y);
    gl_FragColor = vec4(gcol, 1.0);
    #include <colorspace_fragment>
    return;
  }
  // 原生配色（与游戏预览器同一套公式）：材质 _Color 只染涂装遮罩标出的区域，其余保持贴图自带的白灰色；
  // 没有遮罩的原生材质整体染色，没有原生颜色的材质（如深色部件、占位体）直接用底色系数
  vec3 albedo = uTint;
  if (uHasMap > 0.5) {
    vec3 tex = texture2D(uMap, vUv).rgb;
    if (uNative > 0.5) {
      float paint = uHasMask > 0.5 ? clamp((texture2D(uMask, vUv).r - 0.1) * 1.25, 0.0, 1.0) : 1.0;
      albedo = tex * mix(vec3(1.0), uTint, paint);
    } else {
      albedo *= tex;
    }
  }
  // 打包贴图（glTF ORM）：R 环境遮蔽、G 粗糙度、B 金属度；缺省按半金属处理
  float ao = 1.0;
  float rough = 0.5;
  float metal = 0.35;
  if (uHasOrm > 0.5) {
    vec3 orm = texture2D(uOrm, vUv).rgb;
    ao = mix(1.0, orm.r, 0.9);
    rough = orm.g;
    metal = orm.b;
  }
  // 法线贴图：只扰动反光 / 高光与明暗的细节，三面色调仍用平直法线，保持大块面的构图
  vec3 nb = n;
  float detail = 1.0;
  if (uHasNormal > 0.5) {
    vec3 tn = texture2D(uNormal, vUv).xyz * 2.0 - 1.0;
    nb = bumpNormal(n, vWorld, vUv, tn, 1.0);
    detail = mix(0.8, 1.12, clamp(0.5 + dot(nb - n, lightVecFor(n)) * 1.6, 0.0, 1.0));
  }
  // 预览器的光照：白色主光 + 很暗的半球环境光（约 0.1），金属没有漫反射、靠反射着色
  // 预览器的主光斜射，朝上的面也只有约 0.75 的漫反射；背光面靠很暗的环境光
  // 预览器的主光斜射，朝上的面也只有约 0.8 的漫反射；背光面靠很暗的环境光
  float ambient = 0.1;
  vec3 col = albedo * mix(1.0, 0.55, metal) * ao * (ambient + (0.8 - ambient) * mvTone(n, vWorld.y)) * uLight * detail;
  // 金属反光：朝天的面映出冷色天空，侧面与背光面映出暗地面；粗糙度越高越模糊
  vec3 viewV = normalize(uViewDir);
  float ndv = clamp(dot(nb, viewV), 0.0, 1.0);
  vec3 refl = reflect(-viewV, nb);
  float sky = mix(smoothstep(-0.15, 0.85, refl.y), 0.5, rough * 0.55);
  vec3 env = mix(vec3(0.02, 0.03, 0.05), vec3(0.34, 0.5, 0.78), sky);
  float f0 = mix(0.03, 0.3, metal);
  float fres = f0 + (1.0 - f0) * pow(1.0 - ndv, 5.0);
  col += env * fres * (1.0 - rough * 0.5) * 0.3;
  // 主光高光：光源在屏幕左上方，粗糙度越低越尖锐
  vec3 lightV = normalize(viewV * 0.5 - uViewRight * 0.6 + vec3(0.0, 1.0, 0.0));
  float spec = pow(max(dot(nb, normalize(lightV + viewV)), 0.0), mix(160.0, 22.0, rough));
  spec *= (0.1 + metal * 0.6) * (1.0 - rough * 0.6);
  col += vec3(0.8, 0.88, 1.0) * spec * 0.25;
  // 有原版贴图时用完整 RGBA 自发光；GLB 缺少原通道时继续使用原来的保守降级。
  vec3 glow = uEmissiveTint * (1.0 - uUnlit);
  if (uHasOriginalLight > 0.5) {
    vec4 e = texture2D(uOriginalLight, vUv);
    glow = pow(e.rgb, vec3(2.2)) * e.a * 0.65;
  } else if (uHasEmissive > 0.5) {
    vec3 e = texture2D(uEmissiveMap, vUv).rgb;
    glow = e * uEmissiveTint * (uNative > 0.5 ? 0.5 * (1.0 - uUnlit) : 0.4);
  }
  // 实验室的专用贴图含打包通道；预览从稀疏绿通道取灯带，避免将整片洋红底当成发光。
  if (uHasSecondaryLight > 0.5) {
    float labStrip = texture2D(uSecondaryLight, vUv).g;
    glow += uLabTint * smoothstep(0.10, 0.70, labStrip) * 0.85;
  }
  col += glow;
  // 预览器的后期：饱和度 1.1、对比度 1.1，压住灰白、让涂装色更实
  col = mix(vec3(dot(col, vec3(0.2126, 0.7152, 0.0722))), col, 1.15);
  col = max((col - 0.18) * 1.1 + 0.18, 0.0);
  // 预览器的后期：饱和度 1.1、对比度 1.1，压住灰白、让涂装色更实
  if (holo > 0.001) {
    vec3 h = mvShade(uGhost, n, 2.4) * 0.85 + uGhost * (0.12 + holoSweep(vWorld.y) * 0.7);
    col = mix(col, h, holo * 0.9);
  }
  col = mix(col, uFlash, clamp(vState.y, 0.0, 1.0) * 0.85);
  col = mix(col, uSelect, vState.w * 0.6);
  col = failTint(col, vFail);
  col = mvHaze(col, vWorld.y);
  gl_FragColor = vec4(col, 1.0);
  #include <colorspace_fragment>
}
`;

/** 传送带：共享端点截面连接带面，箭头沿整条路径连续流动。 */
export const BELT_VERTEX = /* glsl */ `
attribute vec4 aState;
attribute float aColor;
attribute vec2 aFlow;
attribute vec3 aStartSide;
attribute vec3 aStartUp;
attribute vec3 aEndSide;
attribute vec3 aEndUp;
attribute float aFail;
uniform vec3 uTiers[3];
varying vec3 vWorld;
varying vec3 vLocal;
varying vec3 vTier;
varying vec4 vState;
varying float vFlow;
varying float vFail;
varying float vSteep;
void main() {
  vec4 world = vec4(position, 1.0);
  float along = position.x + 0.5;
  #ifdef USE_INSTANCING
    // 只在半个带宽以内斜接；中间两排顶点保持本段的正交截面，避免整段扭曲。
    float join = min(0.25, 0.33 / max(length(instanceMatrix[0].xyz), 1e-4));
    if (along < 0.333334) along *= 3.0 * join;
    else if (along > 0.666666) along = 1.0 - (1.0 - along) * 3.0 * join;
    else along = join + (along - 0.333333) * 3.0 * (1.0 - 2.0 * join);
    float startJoin = 1.0 - smoothstep(0.0, join, along);
    float endJoin = smoothstep(1.0 - join, 1.0, along);
    vec3 side = instanceMatrix[2].xyz;
    vec3 up = instanceMatrix[1].xyz;
    side += (aStartSide - side) * startJoin + (aEndSide - side) * endJoin;
    up += (aStartUp - up) * startJoin + (aEndUp - up) * endJoin;
    world = instanceMatrix * vec4(along - 0.5, 0.0, 0.0, 1.0);
    world.xyz += side * position.z + up * position.y;
    // 实例矩阵第一列是 运输方向 × 长度：竖直分量占比大即为陡坡带
    vSteep = step(0.35, abs(instanceMatrix[0].y) / max(length(instanceMatrix[0].xyz), 1e-4));
  #else
    vSteep = 0.0;
  #endif
  world = modelMatrix * world;
  vWorld = world.xyz;
  vLocal = position;
  vTier = uTiers[int(clamp(aColor, 0.0, 2.0) + 0.5)];
  vState = aState;
  vFlow = aFlow.x + along * aFlow.y;
  vFail = aFail;
  gl_Position = projectionMatrix * viewMatrix * world;
}
`;

export const BELT_FRAGMENT = /* glsl */ `
${COMMON}
uniform vec3 uBelt;
uniform float uFlow;
varying float vFail;
varying float vSteep;
varying vec3 vWorld;
varying vec3 vLocal;
varying vec3 vTier;
varying vec4 vState;
varying float vFlow;
void main() {
  if (vState.x < 0.999 && vState.x <= bayer4(gl_FragCoord.xy)) discard;
  float holo = vState.z;
  if (holo > 0.5 && mod(floor(gl_FragCoord.y), 3.0) < 1.0) discard;
  vec3 n = flatNormal(vWorld);
  // 带面按局部朝向判定（单位盒 y 轴 0..1，顶面恰在 1），而不是世界朝上：
  // 垂直升降的传送带顶面朝向侧面，箭头动画必须跟着贴在这一面；
  // 陡坡带（升降）的顶面容易背向或正对视线，顶底两面都绘制箭头，任何视角都能看见流向
  float top = max(step(0.99, vLocal.y), vSteep * step(vLocal.y, 0.01));
  float across = abs(vLocal.z) * 2.0;
  // 箭头尖端在带中线、两翼向后张开，整体沿运输方向（局部 +x）移动
  float c = fract(vFlow + across * 0.42 - uTime * uFlow);
  float chevron = smoothstep(0.0, 0.07, c) * (1.0 - smoothstep(0.26, 0.34, c));
  float inner = 1.0 - smoothstep(0.58, 0.66, across);
  vec3 col = mvShade(uBelt, n, vWorld.y + 1.6);
  col = mix(col, uSteel * 0.75, top * smoothstep(0.78, 0.86, across) * 0.7);
  col += vTier * top * (chevron * inner * (0.6 + 0.5 * uParams.x) + 0.05);
  if (holo > 0.001) col = mix(col, uGhost * (0.5 + 0.6 * top * chevron), holo);
  col = mix(col, uFlash, clamp(vState.y, 0.0, 1.0) * 0.8);
  col = mix(col, uSelect, vState.w * 0.6);
  col = failTint(col, vFail);
  col = mvHaze(col, vWorld.y);
  gl_FragColor = vec4(col, 1.0);
  #include <colorspace_fragment>
}
`;

/** 浮空平台：顶面为暗色面板与发光建造网格，边缘灯带；侧面为舱壁与流动的航行灯 */
export const ISLAND_VERTEX = /* glsl */ `
varying vec3 vWorld;
varying vec3 vIslandLocal;
void main() {
  vec4 world = vec4(position, 1.0);
  #ifdef USE_INSTANCING
    world = instanceMatrix * world;
  #endif
  vIslandLocal = world.xyz;
  world = modelMatrix * world;
  vWorld = world.xyz;
  gl_Position = projectionMatrix * viewMatrix * world;
}
`;

export const ISLAND_FRAGMENT = /* glsl */ `
${COMMON}
uniform vec3 uTileA;
uniform vec3 uTileB;
uniform vec3 uGrid;
uniform vec3 uRim;
uniform vec3 uCliff;
uniform float uTile;
uniform vec4 uBounds;
uniform float uFail;
varying vec3 vWorld;
varying vec3 vIslandLocal;
void main() {
  vec3 n = flatNormal(vWorld);
  vec3 col;
  if (n.y > 0.5) {
    vec2 cell = floor(vWorld.xz / uTile);
    vec3 base = mix(uTileA, uTileB, mod(cell.x + cell.y, 2.0));
    col = mvShade(base, n, 9.0);
    float fade = 1.0 - smoothstep(0.12, 0.35, fwidth(vWorld.x));
    float fine = max(gridLine(vWorld.x, 0.05), gridLine(vWorld.z, 0.05)) * fade;
    float major = max(gridLine(vWorld.x / uTile, 0.016), gridLine(vWorld.z / uTile, 0.016));
    col += uGrid * (fine * 0.08 + major * 0.32) * uParams.x;
    float edge = min(min(vIslandLocal.x - uBounds.x, uBounds.z - vIslandLocal.x), min(vIslandLocal.z - uBounds.y, uBounds.w - vIslandLocal.z));
    col = mix(col, uRim * 0.9, (1.0 - smoothstep(0.0, 0.22, edge)) * 0.85);
    col += uRim * (1.0 - smoothstep(0.22, 1.4, edge)) * 0.08 * uParams.x;
  } else {
    col = mvShade(uCliff, n, 9.0);
    float seam = gridLine(vWorld.y * 0.5, 0.03);
    col *= 1.0 - seam * 0.25;
    float lights = gridLine(vWorld.y * 0.25 + 0.5, 0.05) * step(0.6, fract((vWorld.x + vWorld.z) * 0.16 - uTime * 0.35));
    col += uRim * lights * 0.75 * uParams.x;
  }
  col = failTint(col, uFail);
  col = mvHaze(col, vWorld.y);
  gl_FragColor = vec4(col, 1.0);
  #include <colorspace_fragment>
}
`;

/** 柔和的圆角方形投影，贴在地面上 */
export const SHADOW_VERTEX = /* glsl */ `
attribute float aShadow;
varying vec2 vUv;
varying float vStrength;
void main() {
  vUv = uv;
  vStrength = aShadow;
  vec4 world = vec4(position, 1.0);
  #ifdef USE_INSTANCING
    world = instanceMatrix * world;
  #endif
  gl_Position = projectionMatrix * viewMatrix * modelMatrix * world;
}
`;

export const SHADOW_FRAGMENT = /* glsl */ `
uniform vec3 uShadowColor;
uniform vec4 uParams;
varying vec2 vUv;
varying float vStrength;
void main() {
  vec2 q = abs(vUv - 0.5) * 2.0;
  float d = length(max(q - 0.45, 0.0)) / 0.55;
  float a = (1.0 - smoothstep(0.0, 1.0, d)) * vStrength * uParams.z;
  if (a < 0.003) discard;
  gl_FragColor = vec4(uShadowColor, a);
  #include <colorspace_fragment>
}
`;

/**
 * 世界图标：正对屏幕的公告牌，圆角底板 + 指向建筑的小尾巴 + 精灵图中的一格物品图标。
 * 锚点在尾尖；尺寸按屏幕像素夹在上下限之间，缩小取景时仍可辨认、放大时不会无限膨胀。
 * 不写深度也不测深度：图标始终浮在建筑上方，由 aIcon.x 控制整体透明度（含退场淡出）。
 */
export const ICON_VERTEX = /* glsl */ `
attribute vec4 aRect;
attribute vec2 aIcon;
uniform float uWorldPerPixel;
// x 屏幕最小边长、y 最大边长（CSS 像素）
uniform vec2 uIconPx;
// 自然屏幕边长低于 x 时完全隐藏、达到 y 时完全显示，避免远景里图标挤成一片
uniform vec2 uIconFade;
varying vec2 vLocal;
varying vec4 vRect;
varying vec2 vIcon;
varying float vWorldY;
varying float vFade;
void main() {
  vec3 anchor = vec3(0.0);
  float size = 1.0;
  #ifdef USE_INSTANCING
    anchor = instanceMatrix[3].xyz;
    size = length(instanceMatrix[0].xyz);
  #endif
  // 视图矩阵的前两行即相机在世界中的右、上方向
  vec3 right = vec3(viewMatrix[0][0], viewMatrix[1][0], viewMatrix[2][0]);
  vec3 up = vec3(viewMatrix[0][1], viewMatrix[1][1], viewMatrix[2][1]);
  float pixel = max(uWorldPerPixel, 1e-5);
  vFade = smoothstep(uIconFade.x, uIconFade.y, size / pixel);
  size = clamp(size, uIconPx.x * pixel, uIconPx.y * pixel);
  vec3 base = (modelMatrix * vec4(anchor, 1.0)).xyz;
  vec3 world = base + right * (position.x * size) + up * ((position.y + 0.5) * size);
  vLocal = position.xy;
  vRect = aRect;
  vIcon = aIcon;
  vWorldY = world.y;
  gl_Position = projectionMatrix * viewMatrix * vec4(world, 1.0);
}
`;

export const ICON_FRAGMENT = /* glsl */ `
${COMMON}
uniform sampler2D uIconMap;
uniform vec3 uRim;
varying vec2 vLocal;
varying vec4 vRect;
varying vec2 vIcon;
varying float vWorldY;
varying float vFade;
void main() {
  // 坠落中的候选平台带着图标一起沉入深处，随深度淡出
  float alpha = vIcon.x * vFade * smoothstep(-30.0, -6.0, vWorldY);
  if (alpha < 0.02) discard;
  vec2 p = vLocal;
  // 圆角底板占上方 0.8×0.8，下方留出指向建筑的小尾巴
  vec2 c = vec2(0.0, 0.1);
  float d = length(max(abs(p - c) - vec2(0.3), 0.0)) - 0.1;
  float aa = max(fwidth(d), 1e-4);
  float plate = 1.0 - smoothstep(-aa, aa, d);
  float rim = plate * smoothstep(-0.05 - aa, -0.05 + aa, d);
  float tailWidth = 0.13 * clamp((p.y + 0.5) / 0.22, 0.0, 1.0);
  float tail = smoothstep(-aa, aa, tailWidth - abs(p.x)) * step(p.y, -0.28) * step(-0.5, p.y);
  vec3 plateCol = mix(uShadow, uHaze, 0.18);
  vec3 col = mix(plateCol, uRim, rim * 0.9);
  col = mix(col, uRim, tail * (1.0 - plate));
  float a = max(plate * 0.88, tail * 0.95);
  // 图标占底板中央 0.6×0.6；贴图已按 flipY 上传，uv 由 iconRect 换算
  vec2 uv = clamp((p - c) / 0.6 + 0.5, 0.0, 1.0);
  float inside = step(abs(p.x - c.x), 0.3) * step(abs(p.y - c.y), 0.3);
  vec4 tex = texture2D(uIconMap, vRect.xy + uv * vRect.zw);
  float ia = tex.a * inside * plate;
  col = mix(col, tex.rgb, ia);
  a = max(a, ia);
  col = failTint(col, vIcon.y);
  col = mvHaze(col, vWorldY);
  a *= alpha;
  if (a < 0.01) discard;
  gl_FragColor = vec4(col, a);
  #include <colorspace_fragment>
}
`;

/** 落位涟漪：方形光圈向外扩散并淡出，叠加混合 */
export const RING_VERTEX = /* glsl */ `
attribute vec2 aRing;
varying vec2 vUv;
varying vec2 vRing;
void main() {
  vUv = uv;
  vRing = aRing;
  vec4 world = vec4(position, 1.0);
  #ifdef USE_INSTANCING
    world = instanceMatrix * world;
  #endif
  gl_Position = projectionMatrix * viewMatrix * modelMatrix * world;
}
`;

export const RING_FRAGMENT = /* glsl */ `
uniform vec3 uRingColor;
varying vec2 vUv;
varying vec2 vRing;
void main() {
  float p = vRing.x;
  if (p <= 0.0 || p >= 1.0) discard;
  vec2 q = abs(vUv - 0.5) * 2.0;
  float d = max(q.x, q.y);
  float r = mix(0.45, 1.0, p);
  float w = 0.07;
  float ring = smoothstep(r - w, r, d) * (1.0 - smoothstep(r, r + w * 0.6, d));
  float a = ring * (1.0 - p) * vRing.y;
  if (a < 0.003) discard;
  gl_FragColor = vec4(uRingColor, a);
  #include <colorspace_fragment>
}
`;

/** 深空：渐变、星云、星点、带十字光芒的恒星与带大气辉光的行星 */
export const SKY_VERTEX = /* glsl */ `
varying vec2 vUv;
void main() {
  vUv = position.xy * 0.5 + 0.5;
  gl_Position = vec4(position.xy, 0.9999, 1.0);
}
`;

export const SKY_FRAGMENT = /* glsl */ `
uniform vec3 uSkyTop;
uniform vec3 uSkyBottom;
uniform vec3 uNebulaA;
uniform vec3 uNebulaB;
uniform vec3 uStar;
uniform vec3 uPlanet;
uniform vec3 uAtmosphere;
uniform float uAspect;
uniform float uTime;
uniform vec4 uParams;
varying vec2 vUv;

float hash21(vec2 p) {
  p = fract(p * vec2(123.34, 456.21));
  p += dot(p, p + 45.32);
  return fract(p.x * p.y);
}
float noise(vec2 p) {
  vec2 i = floor(p);
  vec2 f = fract(p);
  vec2 u = f * f * (3.0 - 2.0 * f);
  return mix(mix(hash21(i), hash21(i + vec2(1.0, 0.0)), u.x), mix(hash21(i + vec2(0.0, 1.0)), hash21(i + vec2(1.0, 1.0)), u.x), u.y);
}
float fbm(vec2 p) {
  float v = 0.0;
  float a = 0.5;
  for (int i = 0; i < 5; i++) {
    v += noise(p) * a;
    p = p * 2.03 + 11.7;
    a *= 0.5;
  }
  return v;
}
float stars(vec2 p, float threshold) {
  vec2 cell = floor(p);
  float h = hash21(cell);
  if (h < threshold) return 0.0;
  vec2 offset = vec2(hash21(cell + 1.7), hash21(cell + 4.1)) - 0.5;
  float d = length(fract(p) - 0.5 - offset * 0.6);
  float twinkle = 0.65 + 0.35 * sin(uTime * (0.8 + h * 2.5) + h * 40.0);
  return (1.0 - smoothstep(0.0, 0.14, d)) * twinkle * (h - threshold) / (1.0 - threshold);
}

void main() {
  vec2 p = vec2(vUv.x * uAspect, vUv.y);
  vec3 col = mix(uSkyBottom, uSkyTop, smoothstep(0.0, 1.0, vUv.y));
  float n1 = fbm(p * 2.1 + vec2(uTime * 0.004, 0.0));
  float n2 = fbm(p * 3.4 + vec2(7.3, 2.1));
  float lane = 1.0 - smoothstep(0.0, 0.55, abs(vUv.y - 0.25 - vUv.x * 0.45));
  col += uNebulaA * smoothstep(0.42, 0.85, n1) * (0.35 + 0.45 * lane);
  col += uNebulaB * smoothstep(0.5, 0.9, n2) * (0.25 + 0.4 * lane);
  float field = stars(p * 95.0, 0.984) + stars(p * 48.0 + 3.1, 0.99) * 1.6;
  col += vec3(0.85, 0.92, 1.0) * field * uParams.y;

  vec2 sp = vec2((vUv.x - 0.84) * uAspect, vUv.y - 0.82);
  float d = length(sp);
  float gain = 0.25 + 0.75 * uParams.y;
  col += uStar * (exp(-d * 30.0) * 1.3 + exp(-d * 7.0) * 0.22) * gain;
  col += uStar * (exp(-abs(sp.y) * 320.0) * exp(-abs(sp.x) * 7.0) + exp(-abs(sp.x) * 320.0) * exp(-abs(sp.y) * 7.0)) * 0.35 * gain;

  vec2 pp = vec2((vUv.x + 0.08) * uAspect, vUv.y + 0.78);
  float r = length(pp);
  float radius = 1.02;
  float body = 1.0 - smoothstep(radius - 0.004, radius, r);
  vec2 dirToStar = normalize(vec2(0.84 * uAspect, 0.82) - vec2(-0.08 * uAspect, -0.78));
  float lit = clamp(dot(normalize(pp), dirToStar), 0.0, 1.0);
  vec3 planet = uPlanet * (0.35 + 0.75 * lit) + uAtmosphere * pow(smoothstep(radius - 0.12, radius, r), 3.0) * 0.5 * lit;
  col = mix(col, planet, body);
  col += uAtmosphere * exp(-max(r - radius, 0.0) * 18.0) * (1.0 - body) * (0.25 + 0.55 * lit);

  vec2 v = vUv - 0.5;
  col *= 1.0 - dot(v, v) * 0.25;
  gl_FragColor = vec4(col, 1.0);
  #include <colorspace_fragment>
}
`;

