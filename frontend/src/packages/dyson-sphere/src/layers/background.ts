import * as THREE from 'three';
import { NOISE, HASH, PREMULT_OUTPUT } from '../internal/shaders';
import { applyPremultBlending, mulberry32 } from '../internal/utils';
import type { Palette } from '../internal/palette';

// 全屏星云：直接输出裁剪空间坐标，不受相机影响
const NEBULA_VERT = /* glsl */ `
varying vec2 vUv;
void main() {
  vUv = uv;
  gl_Position = vec4(position.xy, 0.9999, 1.0);
}
`;

const NEBULA_FRAG = /* glsl */ `
uniform vec3 uBg;
uniform vec3 uNebA;
uniform vec3 uNebB;
uniform float uNebAlpha;
uniform float uIntensity;
uniform float uTime;
uniform float uAspect;
uniform vec2 uOffset;
uniform vec2 uFocus;
varying vec2 vUv;
${NOISE}
${HASH}
void main() {
  vec2 p = (vUv - 0.5) * vec2(uAspect, 1.0) + uOffset;
  vec3 q = vec3(p * 0.9, uTime * 0.012);
  // 域扭曲让云团呈现丝缕状
  float w = snoise(q * 0.7 + 3.1);
  float n = fbm(q + vec3(w * 0.7, -w * 0.4, 0.0));
  float tint = snoise(vec3(p * 0.55 - 5.0, uTime * 0.008));
  float cloud = smoothstep(-0.2, 0.75, n);
  vec3 neb = mix(uNebA, uNebB, smoothstep(-0.45, 0.55, tint));
  // 恒星附近的星云更亮
  vec2 fd = (vUv - uFocus) * vec2(uAspect, 1.0);
  float focus = exp(-dot(fd, fd) * 0.9);
  float amount = cloud * uNebAlpha * uIntensity * (0.3 + 0.7 * focus) * 0.55;
  vec3 col = mix(uBg, neb, clamp(amount, 0.0, 1.0));
  // 抖动，避免渐变色带
  col += (hash12(gl_FragCoord.xy) - 0.5) / 255.0;
  gl_FragColor = vec4(col, 1.0);
  #include <colorspace_fragment>
}
`;

const STAR_VERT = /* glsl */ `
attribute float aSize;
attribute float aSeed;
uniform float uPixelRatio;
uniform float uTime;
varying float vTwinkle;
void main() {
  vTwinkle = 0.65 + 0.35 * sin(uTime * (0.6 + aSeed * 2.2) + aSeed * 60.0);
  vec4 mv = modelViewMatrix * vec4(position, 1.0);
  gl_Position = projectionMatrix * mv;
  gl_PointSize = aSize * uPixelRatio;
}
`;

const STAR_FRAG = /* glsl */ `
uniform vec3 uColor;
uniform float uAlpha;
uniform float uOver;
varying float vTwinkle;
void main() {
  float r = length(gl_PointCoord - 0.5);
  float a = smoothstep(0.5, 0.0, r);
  vec3 col = uColor;
  float alpha = a * a * vTwinkle * uAlpha;
  ${PREMULT_OUTPUT}
}
`;

/**
 * 背景图层：全屏星云 + 远景星点。
 */
export class BackgroundLayer {
  readonly group = new THREE.Group();

  private readonly nebulaUniforms = {
    uBg: { value: new THREE.Color() },
    uNebA: { value: new THREE.Color() },
    uNebB: { value: new THREE.Color() },
    uNebAlpha: { value: 1 },
    uIntensity: { value: 1 },
    uTime: { value: 0 },
    uAspect: { value: 3 },
    uOffset: { value: new THREE.Vector2() },
    uFocus: { value: new THREE.Vector2(0.64, 0.5) },
  };
  private readonly starUniforms = {
    uColor: { value: new THREE.Color() },
    uAlpha: { value: 1 },
    uOver: { value: 0 },
    uPixelRatio: { value: 1 },
    uTime: { value: 0 },
  };
  private readonly nebula: THREE.Mesh<THREE.PlaneGeometry, THREE.ShaderMaterial>;
  private readonly starMaterial: THREE.ShaderMaterial;
  private stars: THREE.Points<THREE.BufferGeometry, THREE.ShaderMaterial> | null = null;
  private starCount = -1;

  constructor() {
    this.nebula = new THREE.Mesh(
      new THREE.PlaneGeometry(2, 2),
      new THREE.ShaderMaterial({
        uniforms: this.nebulaUniforms,
        vertexShader: NEBULA_VERT,
        fragmentShader: NEBULA_FRAG,
        depthTest: false,
        depthWrite: false,
      }),
    );
    this.nebula.frustumCulled = false;
    this.nebula.renderOrder = -100;
    this.group.add(this.nebula);

    this.starMaterial = applyPremultBlending(
      new THREE.ShaderMaterial({
        uniforms: this.starUniforms,
        vertexShader: STAR_VERT,
        fragmentShader: STAR_FRAG,
      }),
    );
  }

  /** 重建星点（数量变化时调用） */
  setStarCount(count: number): void {
    if (count === this.starCount) return;
    this.starCount = count;
    if (this.stars) {
      this.group.remove(this.stars);
      this.stars.geometry.dispose();
    }
    const rand = mulberry32(1337);
    const pos = new Float32Array(count * 3);
    const size = new Float32Array(count);
    const seed = new Float32Array(count);
    for (let i = 0; i < count; i++) {
      // 均匀分布在远处球面上
      const u = rand() * 2 - 1;
      const th = rand() * Math.PI * 2;
      const s = Math.sqrt(1 - u * u);
      const r = 90;
      pos[i * 3] = Math.cos(th) * s * r;
      pos[i * 3 + 1] = u * r;
      pos[i * 3 + 2] = Math.sin(th) * s * r;
      size[i] = 0.8 + Math.pow(rand(), 6) * 3.2;
      seed[i] = rand();
    }
    const geo = new THREE.BufferGeometry();
    geo.setAttribute('position', new THREE.BufferAttribute(pos, 3));
    geo.setAttribute('aSize', new THREE.BufferAttribute(size, 1));
    geo.setAttribute('aSeed', new THREE.BufferAttribute(seed, 1));
    this.stars = new THREE.Points(geo, this.starMaterial);
    this.stars.frustumCulled = false;
    this.stars.renderOrder = -90;
    this.group.add(this.stars);
  }

  setNebulaIntensity(v: number): void {
    this.nebulaUniforms.uIntensity.value = v;
  }

  setViewport(width: number, height: number, pixelRatio: number, focusX: number): void {
    this.nebulaUniforms.uAspect.value = width / Math.max(height, 1);
    this.nebulaUniforms.uFocus.value.set(focusX, 0.5);
    this.starUniforms.uPixelRatio.value = pixelRatio;
  }

  applyPalette(p: Palette): void {
    const u = this.nebulaUniforms;
    u.uBg.value.copy(p.background);
    u.uNebA.value.copy(p.nebulaA);
    u.uNebB.value.copy(p.nebulaB);
    u.uNebAlpha.value = p.nebulaAlpha;
    this.starUniforms.uColor.value.copy(p.starPoint);
    this.starUniforms.uAlpha.value = p.starPointAlpha;
    this.starUniforms.uOver.value = p.over;
  }

  /** parallax：相机视差偏移，星云做反向微移 */
  update(time: number, parallax: { x: number; y: number }): void {
    this.nebulaUniforms.uTime.value = time;
    this.nebulaUniforms.uOffset.value.set(-parallax.x * 0.03, -parallax.y * 0.02);
    this.starUniforms.uTime.value = time;
    if (this.stars) this.stars.rotation.y = time * 0.004;
  }

  dispose(): void {
    this.nebula.geometry.dispose();
    this.nebula.material.dispose();
    this.stars?.geometry.dispose();
    this.starMaterial.dispose();
  }
}
