import * as THREE from 'three';
import { NOISE, PREMULT_OUTPUT } from '../internal/shaders';
import { applyPremultBlending, RENDER_ORDER } from '../internal/utils';
import type { Palette } from '../internal/palette';

export const STAR_RADIUS = 1;
// 日冕面片半边长（以恒星半径为单位）
const CORONA_EXTENT = 5.0;

const SURFACE_VERT = /* glsl */ `
varying vec3 vPos;
varying vec3 vNormalV;
varying vec3 vViewDir;
void main() {
  vPos = position;
  vec4 mv = modelViewMatrix * vec4(position, 1.0);
  vNormalV = normalize(normalMatrix * normal);
  vViewDir = normalize(-mv.xyz);
  gl_Position = projectionMatrix * mv;
}
`;

// 恒星表面：大尺度对流 + 米粒组织 + 太阳黑子 + 临边昏暗 + 光斑。
// 亮度刻意控制在 ~1.1 以内，只有最热的区域进入 Bloom，避免核心过曝成一团白光。
const SURFACE_FRAG = /* glsl */ `
uniform float uTime;
uniform vec3 uColor;
uniform float uBright;
varying vec3 vPos;
varying vec3 vNormalV;
varying vec3 vViewDir;
${NOISE}
void main() {
  float t = uTime;
  float c = cos(t * 0.025);
  float s = sin(t * 0.025);
  vec3 p = vec3(c * vPos.x - s * vPos.z, vPos.y, s * vPos.x + c * vPos.z);

  vec3 warp = vec3(
    snoise(p * 1.1 + t * 0.04),
    snoise(p * 1.1 + 7.1 - t * 0.03),
    snoise(p * 1.1 + 13.7 + t * 0.035)
  );
  float conv = fbm(p * 2.2 + warp * 0.6);
  // 米粒组织：两层"脊状"噪声，形成细碎的亮格与暗缝
  float g1 = 1.0 - abs(snoise(p * 9.0 + warp * 0.8 + t * 0.06));
  float g2 = 1.0 - abs(snoise(p * 19.0 - warp + t * 0.09));
  float gran = g1 * g1 * 0.6 + g2 * g2 * g2 * 0.4;
  // 太阳黑子：本影 + 半影
  float spotN = snoise(p * 1.5 + vec3(3.0, t * 0.008, 0.0)) + 0.25 * snoise(p * 5.0);
  float umbra = smoothstep(0.62, 0.8, spotN);
  float penumbra = smoothstep(0.45, 0.66, spotN);

  float heat = 0.42 + 0.32 * conv + 0.32 * gran;
  heat *= 1.0 - penumbra * 0.32 - umbra * 0.55;

  vec3 cool = uColor * vec3(0.78, 0.3, 0.1);
  vec3 hot = mix(uColor, vec3(1.0, 0.95, 0.86), 0.5);
  vec3 col = mix(cool, uColor, smoothstep(0.15, 0.62, heat));
  col = mix(col, hot, smoothstep(0.62, 1.05, heat));

  float mu = clamp(dot(normalize(vNormalV), normalize(vViewDir)), 0.0, 1.0);
  col *= 0.3 + 0.7 * pow(mu, 0.55);                           // 临边昏暗
  col *= 0.55 + 0.5 * heat;
  col += uColor * pow(1.0 - mu, 2.2) * gran * 0.35;           // 临边光斑
  col += vec3(1.0, 0.55, 0.3) * pow(1.0 - mu, 6.0) * 0.35;    // 色球层边缘
  col *= uBright;
  gl_FragColor = vec4(col, 1.0);
  #include <colorspace_fragment>
}
`;

const CORONA_VERT = /* glsl */ `
varying vec2 vUv;
void main() {
  vUv = uv;
  gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
}
`;

// 日冕：贴边的细色球层 + 丝缕状冕流 + 极淡的大范围辉光 + 一道很淡的横向镜头光条
const CORONA_FRAG = /* glsl */ `
uniform float uTime;
uniform float uIntensity;
uniform float uOver;
uniform vec3 uColor;
varying vec2 vUv;
${NOISE}
void main() {
  const float EXTENT = ${CORONA_EXTENT.toFixed(2)};
  vec2 q = (vUv - 0.5) * 2.0 * EXTENT;
  float r = length(q);
  float ang = atan(q.y, q.x);
  vec2 dir = vec2(cos(ang), sin(ang));
  float d = max(r - 1.0, 0.0);

  float chromo = exp(-d * 14.0) * 0.55;
  float rays = snoise(vec3(dir * 3.0, r * 0.35 - uTime * 0.05)) * 0.5 + 0.5;
  rays *= snoise(vec3(dir * 9.0, uTime * 0.04)) * 0.5 + 0.5;
  // snoise 可能略超出 [-1, 1]，先钳制，避免 pow 负底数产生 NaN
  float streamers = pow(clamp(rays, 0.0, 1.0), 1.6) * exp(-d * 1.9) * 0.4;
  float wide = exp(-d * 0.9) * 0.05;
  float streak = exp(-abs(q.y) * 9.0) * exp(-abs(q.x) * 0.55) * 0.07;
  float corona = (chromo + streamers + wide + streak) * step(0.98, r);
  corona *= 1.0 - smoothstep(EXTENT * 0.6, EXTENT, r);

  vec3 col = mix(uColor, vec3(1.0, 0.9, 0.78), exp(-d * 6.0) * 0.5);
  col = mix(col, vec3(0.75, 0.85, 1.0), clamp(streak / max(corona, 1e-3), 0.0, 1.0) * 0.6);
  float alpha = corona * uIntensity;
  ${PREMULT_OUTPUT}
}
`;

/**
 * 恒星图层：噪声流动表面 + 面向相机的日冕面片。
 */
export class StarLayer {
  readonly group = new THREE.Group();

  private readonly color = new THREE.Color('#ffc46b');
  private pulse = 0.5;
  private glowScale = 1;
  private readonly surfaceUniforms = {
    uTime: { value: 0 },
    uColor: { value: this.color },
    uBright: { value: 1 },
  };
  private readonly coronaUniforms = {
    uTime: { value: 0 },
    uIntensity: { value: 1 },
    uOver: { value: 0 },
    uColor: { value: this.color },
  };
  private readonly surface: THREE.Mesh<THREE.SphereGeometry, THREE.ShaderMaterial>;
  private readonly corona: THREE.Mesh<THREE.PlaneGeometry, THREE.ShaderMaterial>;

  constructor() {
    this.surface = new THREE.Mesh(
      new THREE.SphereGeometry(STAR_RADIUS, 96, 64),
      new THREE.ShaderMaterial({
        uniforms: this.surfaceUniforms,
        vertexShader: SURFACE_VERT,
        fragmentShader: SURFACE_FRAG,
      }),
    );
    this.group.add(this.surface);

    const size = CORONA_EXTENT * 2 * STAR_RADIUS;
    this.corona = new THREE.Mesh(
      new THREE.PlaneGeometry(size, size),
      applyPremultBlending(
        new THREE.ShaderMaterial({
          uniforms: this.coronaUniforms,
          vertexShader: CORONA_VERT,
          fragmentShader: CORONA_FRAG,
        }),
      ),
    );
    this.corona.renderOrder = RENDER_ORDER.corona;
    this.group.add(this.corona);
  }

  /** 恒星颜色，CSS 色值 */
  setColor(color: string): void {
    this.color.set(color);
  }

  setPulse(v: number): void {
    this.pulse = v;
  }

  applyPalette(p: Palette): void {
    this.glowScale = p.starGlow;
    this.coronaUniforms.uOver.value = p.over;
  }

  /** motion：动画幅度倍率（减少动态效果时降低） */
  update(time: number, camera: THREE.Camera, motion: number): void {
    // 缓慢脉动：两个频率叠加，避免机械感
    const wave = Math.sin(time * 1.1) * 0.6 + Math.sin(time * 2.7 + 1.3) * 0.4;
    const k = this.pulse * motion;
    this.surface.scale.setScalar(1 + wave * 0.02 * k);
    this.surfaceUniforms.uTime.value = time;
    this.surfaceUniforms.uBright.value = 1.05 + wave * 0.06 * k;
    this.coronaUniforms.uTime.value = time;
    this.coronaUniforms.uIntensity.value = this.glowScale * (1 + wave * 0.2 * k);
    this.corona.quaternion.copy(camera.quaternion);
  }

  dispose(): void {
    this.surface.geometry.dispose();
    this.surface.material.dispose();
    this.corona.geometry.dispose();
    this.corona.material.dispose();
  }
}
