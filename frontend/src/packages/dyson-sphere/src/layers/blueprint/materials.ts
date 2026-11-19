import * as THREE from 'three';

/** 程序材质在壳面独立的切平面上铺六角集能板，细节密度使用游戏米制，与取景缩放无关。 */
export function panelMaterial(layerId: number, daylight: THREE.IUniform<number>, surfaceDetail: THREE.IUniform<number>): THREE.ShaderMaterial {
  return new THREE.ShaderMaterial({
    uniforms: { daylight, surfaceDetail },
    vertexColors: true,
    side: THREE.DoubleSide,
    stencilWrite: true,
    stencilRef: 200 + layerId,
    stencilZPass: THREE.ReplaceStencilOp,
    vertexShader: `
      attribute vec3 panelStyle;
      varying vec3 vColor, vStyle, vNormal, vEye;
      varying vec2 vGrid;
      void main() {
        vColor = color;
        vStyle = panelStyle;
        vGrid = uv;
        vNormal = normalize(normalMatrix * normalize(position));
        vec4 mv = modelViewMatrix * vec4(position, 1.0);
        vEye = -mv.xyz;
        gl_Position = projectionMatrix * mv;
      }
    `,
    fragmentShader: `
      uniform float daylight;
      uniform float surfaceDetail;
      varying vec3 vColor, vStyle, vNormal, vEye;
      varying vec2 vGrid;
      float line(float d, float width, float pixel) {
        return 1.0 - smoothstep(width, width + pixel, abs(d));
      }
      void main() {
        vec2 period = vec2(1.0, 1.73205);
        vec2 a = mod(vGrid + period * 0.5, period) - period * 0.5;
        vec2 b = mod(vGrid, period) - period * 0.5;
        vec2 q = dot(a,a) < dot(b,b) ? a : b;
        vec2 cell = vGrid - q;
        float hex = max(abs(q.x), dot(abs(q), vec2(0.5,0.8660254)));
        float pixel = max(max(fwidth(vGrid.x), fwidth(vGrid.y)), 0.001);
        float lod = surfaceDetail * (1.0 - smoothstep(0.12, 0.8, pixel));
        float rim = line(0.49 - hex, 0.017, pixel);
        float trace = line(0.35 - hex, 0.012, pixel);
        float bus = line(q.y, 0.018, pixel);
        float ribs = line(abs(fract((q.x + q.y * 0.58) * 7.0) - 0.5), 0.035, pixel * 3.0);
        float proto = vStyle.x;
        float emission = 2.0;
        if (proto == 1.0) { trace = max(rim, bus * step(0.0, q.x)); emission = 3.0; }
        else if (proto == 2.0) { trace = bus * step(0.24, abs(q.x)) + rim * 0.18; emission = 3.0; }
        else if (proto == 3.0) { trace = line(fract(length(q) * 3.0) - 0.5, 0.035, pixel * 3.0); emission = 1.0; }
        else if (proto == 4.0) { trace = line(q.x + q.y * 0.58, 0.025, pixel) + bus * step(0.26, q.x); emission = 2.6; }
        else if (proto == 5.0) { trace += line(0.16 - hex, 0.017, pixel); }
        else if (proto == 6.0) { trace = line(q.x - q.y * 0.58, 0.015, pixel); emission = 1.5; }
        float variance = fract(sin(dot(cell, vec2(17.13, 53.71)) + vStyle.y * 0.01) * 4375.23);
        vec3 normal = normalize(vNormal);
        float face = clamp(dot(normal,normalize(vEye)), -1.0, 1.0);
        float light = 0.31 + 0.55 * max(dot(normal,normalize(vec3(-0.38,0.7,0.6))),0.0);
        vec3 alloy = mix(vec3(0.078,0.127,0.155), vec3(0.13,0.17,0.19), daylight);
        vec3 base = mix(alloy, vColor * 0.72 + 0.009, vStyle.z);
        base *= light + (variance - 0.5) * 0.12 * lod;
        base *= 1.0 - lod * (rim * 0.38 + ribs * 0.16);
        float circuits = mix(0.075, 0.06 + trace * 0.53 + rim * 0.3, lod);
        vec3 glow = vColor * circuits * emission * mix(0.38,0.30,daylight);
        float fresnel = pow(1.0 - abs(face), 3.0);
        gl_FragColor = vec4(base + glow + vec3(0.04,0.05,0.06) * fresnel, 1.0);
        #include <colorspace_fragment>
      }
    `,
  });
}

/** 节点塔身与分段桁架共用径向照明；框架原型的两条纵梁、腹杆和接头通过局部坐标着色。 */
export function structureMaterial(frame: boolean, daylight: THREE.IUniform<number>): THREE.ShaderMaterial {
  return new THREE.ShaderMaterial({
    uniforms: { daylight },
    defines: frame ? { FRAME: 1 } : {},
    vertexShader: `
      #ifdef FRAME
        attribute vec3 beamStyle;
        varying vec3 vBeam;
      #endif
      varying vec3 vPaint, vPart, vNormal;
      void main() {
        #ifdef FRAME
          vBeam = beamStyle;
        #endif
        vPaint = instanceColor;
        vPart = position;
        vNormal = normalize(normalMatrix * mat3(instanceMatrix) * normal);
        gl_Position = projectionMatrix * modelViewMatrix * instanceMatrix * vec4(position, 1.0);
      }
    `,
    fragmentShader: `
      uniform float daylight;
      varying vec3 vPaint, vPart, vNormal;
      #ifdef FRAME
        varying vec3 vBeam;
      #endif
      void main() {
        float accent;
        #ifdef FRAME
          float x = abs(vPart.x);
          float t = (vPart.z + 0.5) * vBeam.y + vBeam.z;
          float rail = smoothstep(0.27,0.40,x);
          float brace = 1.0 - smoothstep(0.07,0.18,abs(x - abs(fract(t * 2.0) - 0.5)));
          if (vBeam.x == 2.0) brace = max(brace,1.0 - smoothstep(0.05,0.16,x));
          float joint = smoothstep(0.40,0.48,abs(fract(t) - 0.5));
          float detail = 1.0 - smoothstep(0.10,0.45,fwidth(vPart.x));
          if (detail > 0.75 && max(rail, max(brace,joint)) < 0.12) discard;
          accent = mix(0.45,rail * 0.78 + joint * 0.22,detail);
        #else
          float band = smoothstep(0.20,0.36,vPart.y) - smoothstep(0.52,0.68,vPart.y);
          accent = 0.25 + band * 0.75;
        #endif
        float diffuse = 0.53 + 0.37 * max(dot(normalize(vNormal),normalize(vec3(-0.4,0.7,0.5))),0.0);
        vec3 col = mix(vec3(0.13,0.14,0.15), vPaint, 0.60 + accent * 0.4);
        gl_FragColor = vec4(col * (diffuse + accent * mix(0.9,0.52,daylight)),1.0);
        #include <colorspace_fragment>
      }
    `,
  });
}
