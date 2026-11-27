import {Color, ShaderMaterial, Vector3} from 'three'

// 以星体编号生成稳定纹理。现有存档接口没有地表贴图，纹理仅用于区分天体和表现球体明暗。
export function celestialMaterial(kind: 'rocky' | 'gas' | 'star' | 'blackHole', tint: Color, id: number, light: Vector3): ShaderMaterial {
  const seed = (Math.abs(id) % 1009) * .137
  return new ShaderMaterial({
    uniforms: {tint: {value: tint}, seed: {value: seed}, light: {value: light}},
    defines: {SURFACE: ['rocky', 'gas', 'star', 'blackHole'].indexOf(kind)},
    vertexShader: /* glsl */ `
      varying vec3 surface;
      varying vec3 viewDirection;
      void main() {
        vec4 world = modelMatrix * vec4(position, 1.0);
        surface = normal;
        viewDirection = cameraPosition - world.xyz;
        gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
      }
    `,
    fragmentShader: /* glsl */ `
      uniform vec3 tint;
      uniform vec3 light;
      uniform float seed;
      varying vec3 surface;
      varying vec3 viewDirection;

      float hash(vec3 p) {
        p = fract(p * 0.3183099 + vec3(0.11, 0.37, 0.59));
        p *= 17.0;
        return fract(p.x * p.y * p.z * (p.x + p.y + p.z));
      }
      float noise(vec3 p) {
        vec3 i = floor(p), f = fract(p);
        f = f * f * (3.0 - 2.0 * f);
        return mix(
          mix(mix(hash(i), hash(i + vec3(1,0,0)), f.x), mix(hash(i + vec3(0,1,0)), hash(i + vec3(1,1,0)), f.x), f.y),
          mix(mix(hash(i + vec3(0,0,1)), hash(i + vec3(1,0,1)), f.x), mix(hash(i + vec3(0,1,1)), hash(i + vec3(1,1,1)), f.x), f.y), f.z);
      }
      void main() {
        vec3 n = normalize(surface);
        float facing = max(0.0, dot(n, normalize(viewDirection)));
        float rim = pow(1.0 - facing, 2.8);
        vec3 p = n + vec3(seed, seed * 0.37, seed * 0.19);
        vec3 color;
        #if SURFACE == 3
          color = tint * pow(1.0 - facing, 6.0) * 0.85;
        #elif SURFACE == 2
          float grain = noise(p * 12.0) * 0.75 + noise(p * 32.0) * 0.25;
          float limb = smoothstep(0.0, 0.85, facing);
          color = mix(tint, vec3(1.0, 0.97, 0.92), 0.18 + limb * 0.66) * (0.52 + grain * 0.46 + limb * 0.22);
        #else
          float lit = 0.09 + 0.91 * smoothstep(-0.42, 0.70, dot(n, light));
          #if SURFACE == 1
            float turbulence = noise(p * vec3(4.5, 17.0, 4.5));
            float bands = 0.5 + 0.5 * sin(n.y * 32.0 + turbulence * 2.8 + seed);
            vec3 surfaceColor = mix(tint * 0.38, mix(tint, vec3(0.92), 0.55), smoothstep(0.10, 0.92, bands));
          #else
            float terrain = noise(p * 4.2) * 0.62 + noise(p * 12.0) * 0.27 + noise(p * 31.0) * 0.11;
            vec3 surfaceColor = mix(tint * 0.18, mix(tint, vec3(0.81, 0.77, 0.71), 0.27), smoothstep(0.30, 0.68, terrain));
            float cloud = smoothstep(0.60, 0.83, noise(p * vec3(7.0, 13.0, 8.0)) + terrain * 0.15);
            surfaceColor = mix(surfaceColor, vec3(0.76, 0.84, 0.93), cloud * 0.44);
          #endif
          color = surfaceColor * lit + mix(tint, vec3(0.24, 0.63, 1.0), 0.44) * rim * (0.10 + 0.48 * lit);
        #endif
        gl_FragColor = vec4(color, 1.0);
        #include <colorspace_fragment>
      }
    `,
  })
}
