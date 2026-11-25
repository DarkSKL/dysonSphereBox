import {cameraFrame, type Camera, type ViewSize} from './galaxy-camera'
import {edgeTransmission, FLOW_PERIOD_MS, nebulaPaletteFor, PALETTE_SIZE, SAMPLE_STRIDE} from './galaxy-nebula'
import {GALAXY_THEMES, type GalaxyTheme} from './galaxy-theme'

// =============== 三维光尘通道 ===============
// 光尘在外、中、内三个采样点确定的曲线上单向前行。投影后的切线决定细丝的形状，星核原点只用于裁切。
const vertexSource = `
precision highp float;
attribute vec3 position;
attribute vec3 optics;
attribute vec3 origin;
attribute vec3 along;
attribute vec3 bend;
attribute vec3 life;
attribute vec3 dustLane;
uniform vec3 right, down;
uniform vec2 clipScale, shadowClock, halfViewport;
uniform float time;
uniform float pixelScale, exposure, maxSize, pixelRatio;
varying float light, tint, aspect;
varying vec2 axis;
vec2 projectPoint(vec3 p) {
  return vec2(dot(p, right), dot(p, down)) * clipScale;
}
void main() {
  vec2 anchor = projectPoint(origin);
  vec2 edge = (1.0 - abs(anchor)) * halfViewport;
  float edgeLight = smoothstep(0.0, min(32.0, .16 * min(halfViewport.x, halfViewport.y)), min(edge.x, edge.y));
  float age = fract(life.x + time * life.y);
  float u = 2.0 * age - 1.0;
  vec2 xy = projectPoint(position + u * (along + u * bend));
  bool visible = all(lessThanEqual(abs(anchor), vec2(1.0)));
  gl_Position = vec4(visible ? xy : vec2(4.0), 0.0, 1.0);
  vec3 tangent = along + 2.0 * u * bend;
  vec2 direction = vec2(dot(tangent, right), dot(tangent, down));
  float projected = length(direction);
  axis = projected > 0.01 ? direction / projected : vec2(1.0, 0.0);
  aspect = mix(1.0, life.z, clamp(projected / max(1.0, length(tangent)), 0.0, 1.0));
  float diameter = optics.x * pixelScale * sqrt(aspect);
  float materialLimit = optics.x < 640.0 ? 1.7 + optics.x * .0032 : clamp(8.0 + optics.x * .013, 10.0, 150.0);
  float limit = min(maxSize, pixelRatio * materialLimit);
  gl_PointSize = max(1.15 * pixelRatio, diameter * inversesqrt(1.0 + diameter * diameter / (limit * limit)));
  float fade = smoothstep(0.0, .18, age) * smoothstep(0.0, .18, 1.0 - age);
  float front = .5 + .5 * dot(dustLane.xy, shadowClock);
  float transmission = 1.0 - dustLane.z * (.26 + .52 * front * front);
  light = edgeLight * exposure * optics.y * 1.19 * fade * transmission * clamp(diameter / (1.15 * pixelRatio), .42, 1.0);
  tint = (optics.z + 0.5) / 64.0;
}`
const fragmentSource = `
precision mediump float;
uniform sampler2D palette;
varying float light, tint, aspect;
varying vec2 axis;
void main() {
  vec2 p = 2.0 * gl_PointCoord - 1.0;
  vec2 local = vec2(dot(p, axis), dot(p, vec2(-axis.y, axis.x)) * aspect);
  float r = dot(local, local);
  if (r >= 1.0) discard;
  float a = clamp(light * exp(-4.0 * r) * (1.0 - smoothstep(0.42, 1.0, r)), 0.0, 1.0);
  gl_FragColor = vec4(texture2D(palette, vec2(tint, 0.5)).rgb * a, a);
}`

// 深色用发光叠加（screen），浅色用预乘 alpha 的普通叠放，让云雾在白底上是灰白云团而不是发光。
function applyBlend(gl: WebGLRenderingContext, theme: GalaxyTheme): void {
  if (theme.nebula.blend === 'over') gl.blendFuncSeparate(gl.ONE, gl.ONE_MINUS_SRC_ALPHA, gl.ONE, gl.ONE_MINUS_SRC_ALPHA)
  else gl.blendFuncSeparate(gl.ONE, gl.ONE_MINUS_SRC_COLOR, gl.ONE, gl.ONE_MINUS_SRC_ALPHA)
}

function uploadPalette(gl: WebGLRenderingContext, theme: GalaxyTheme): void {
  gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, PALETTE_SIZE, 1, 0, gl.RGBA, gl.UNSIGNED_BYTE, nebulaPaletteFor(theme))
}

function createPipeline(gl: WebGLRenderingContext, theme: GalaxyTheme) {
  const program = gl.createProgram(), buffer = gl.createBuffer(), palette = gl.createTexture()
  const shaders: WebGLShader[] = []
  const release = () => {
    for (const shader of shaders) gl.deleteShader(shader)
    gl.deleteProgram(program)
    gl.deleteBuffer(buffer)
    gl.deleteTexture(palette)
  }
  if (!program || !buffer || !palette) { release(); return null }
  for (const [type, source] of [[gl.VERTEX_SHADER, vertexSource], [gl.FRAGMENT_SHADER, fragmentSource]] as const) {
    const shader = gl.createShader(type)
    if (!shader) { release(); return null }
    shaders.push(shader)
    gl.shaderSource(shader, source)
    gl.compileShader(shader)
    if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) { release(); return null }
    gl.attachShader(program, shader)
  }
  const attributes = ['position', 'optics', 'origin', 'along', 'bend', 'life', 'dustLane']
  attributes.forEach((name, i) => gl.bindAttribLocation(program, i, name))
  gl.linkProgram(program)
  if (!gl.getProgramParameter(program, gl.LINK_STATUS)) { release(); return null }
  gl.useProgram(program)
  gl.bindBuffer(gl.ARRAY_BUFFER, buffer)
  attributes.forEach((_, i) => {
    gl.enableVertexAttribArray(i)
    gl.vertexAttribPointer(i, 3, gl.FLOAT, false, SAMPLE_STRIDE * 4, i * 12)
  })
  gl.bindTexture(gl.TEXTURE_2D, palette)
  uploadPalette(gl, theme)
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
  gl.enable(gl.BLEND)
  applyBlend(gl, theme)
  const uniform = (name: string) => gl.getUniformLocation(program, name)
  const right = uniform('right'), down = uniform('down'), clipScale = uniform('clipScale')
  const pixelScale = uniform('pixelScale'), exposure = uniform('exposure'), pixelRatio = uniform('pixelRatio')
  const phaseTime = uniform('time'), shadowClock = uniform('shadowClock'), halfViewport = uniform('halfViewport')
  gl.uniform1f(uniform('maxSize'), Math.min(256, gl.getParameter(gl.ALIASED_POINT_SIZE_RANGE)[1]))
  let uploaded: Float32Array | undefined
  return {
    setTheme(next: GalaxyTheme) {
      gl.bindTexture(gl.TEXTURE_2D, palette)
      uploadPalette(gl, next)
      applyBlend(gl, next)
    },
    draw(samples: Float32Array, camera: Camera, size: ViewSize, dpr: number, gain: number, time: number, shadow: readonly [number, number]) {
      if (samples !== uploaded) {
        gl.bufferData(gl.ARRAY_BUFFER, samples, gl.STATIC_DRAW)
        uploaded = samples
      }
      const frame = cameraFrame(camera, size)
      gl.viewport(0, 0, gl.drawingBufferWidth, gl.drawingBufferHeight)
      gl.clear(gl.COLOR_BUFFER_BIT)
      gl.uniform3f(right, ...frame.right)
      gl.uniform3f(down, ...frame.down)
      gl.uniform2f(clipScale, 2 * frame.scale / size.width, -2 * frame.scale / size.height)
      gl.uniform2f(halfViewport, size.width / 2, size.height / 2)
      gl.uniform1f(pixelScale, frame.scale * dpr / Math.pow(camera.zoom, .23))
      gl.uniform1f(pixelRatio, dpr)
      gl.uniform1f(exposure, gain)
      gl.uniform1f(phaseTime, time)
      gl.uniform2f(shadowClock, ...shadow)
      gl.drawArrays(gl.POINTS, 0, samples.length / SAMPLE_STRIDE)
    },
    release,
  }
}

// =============== 软件绘制 ===============
// WebView 禁用硬件加速时沿用同一批 XYZ、相机和色温。
// 按材质对宽雾/气晕、丝缕与细尘分别做确定性有界抽样并用能量守恒公式补偿单粒透明度，
// 同步搭配适度降采样离屏画布与预烘 32×32 笔刷，确保 200 星下帧耗时远低于逐粒全量绘制且与 WebGL 视觉一致。
function softwareBrush(index: number, theme: GalaxyTheme): HTMLCanvasElement {
  const brush = document.createElement('canvas')
  brush.width = brush.height = 32
  const ctx = brush.getContext('2d')!
  const color = nebulaPaletteFor(theme).slice(index * 4, index * 4 + 3).join(',')
  const gradient = ctx.createRadialGradient(16, 16, 0, 16, 16, 16)
  for (const [stop, alpha] of [[0, 1], [.22, .62], [.48, .22], [.74, .05], [1, 0]]) gradient.addColorStop(stop, 'rgba(' + color + ',' + alpha + ')')
  ctx.fillStyle = gradient
  ctx.fillRect(0, 0, 32, 32)
  return brush
}

function fadeEdge(value: number): number {
  const t = Math.max(0, Math.min(1, value / .16))
  return t * t * (3 - 2 * t)
}

// 宽雾/气晕、纤维丝缕与细尘按材质各自有界抽样（稀疏 1–6 星全量不抽样，密集 200 星控在 ~7,500–8,000 粒以内）。
// 选择只取决于颗粒自身哈希，换视区或缩放仍是同一批稳定纹理。
function softwareParticles(samples: Float32Array, peak: number) {
  const fogStep = Math.max(1, Math.ceil(peak * 4 / 16 / 2800))
  const silkStep = Math.max(1, Math.ceil(peak * 3 / 16 / 2400))
  const grainStep = Math.max(1, Math.ceil(peak * 9 / 16 / 2200))
  const bits = new Uint32Array(samples.buffer, samples.byteOffset, samples.length)
  const count = samples.length / SAMPLE_STRIDE
  const offsets = new Uint32Array(count)
  const strides = new Uint16Array(count)
  let n = 0
  for (let i = 0; i < samples.length; i += SAMPLE_STRIDE) {
    const diameter = samples[i + 3]
    const stride = diameter >= 3500 ? fogStep : diameter < 640 ? grainStep : silkStep
    if (stride === 1) {
      offsets[n] = i
      strides[n++] = 1
      continue
    }
    const hash = (bits[i] ^ Math.imul(bits[i + 1], 16777619) ^ bits[i + 2] ^ bits[i + 15]) >>> 0
    if (hash % stride === 0) {
      offsets[n] = i
      strides[n++] = stride
    }
  }
  return {samples, offsets: offsets.subarray(0, n), strides: strides.subarray(0, n)}
}

function drawSoftware(ctx: CanvasRenderingContext2D, brushes: HTMLCanvasElement[], theme: GalaxyTheme, particles: ReturnType<typeof softwareParticles>, camera: Camera, size: ViewSize, dpr: number, gain: number, time: number, shadow: readonly [number, number]) {
  const {samples, offsets, strides} = particles
  const {width, height} = size
  const {right, down, scale} = cameraFrame(camera, size)
  const r0 = right[0], r1 = right[1]
  const d0 = down[0], d1 = down[1], d2 = down[2]
  const halfW = width / 2, halfH = height / 2
  const shadowX = shadow[0], shadowY = shadow[1]
  const materialScale = scale / Math.pow(camera.zoom, .23)
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  ctx.clearRect(0, 0, width + 1, height + 1)
  ctx.globalCompositeOperation = theme.nebula.blend === 'over' ? 'source-over' : 'screen'
  let lastX = NaN, lastY = NaN, lastZ = NaN, edgeLight = 0
  let rotated = false
  for (let k = 0; k < offsets.length; k++) {
    const i = offsets[k]
    const ox = samples[i + 6], oy = samples[i + 7], oz = samples[i + 8]
    if (ox !== lastX || oy !== lastY || oz !== lastZ) {
      lastX = ox; lastY = oy; lastZ = oz
      const edgeX = halfW - Math.abs(scale * (r0 * ox + r1 * oy))
      const edgeY = halfH - Math.abs(scale * (d0 * ox + d1 * oy + d2 * oz))
      edgeLight = edgeTransmission(edgeX < edgeY ? edgeX : edgeY, width, height)
    }
    if (!edgeLight) continue
    const age = (samples[i + 15] + time * samples[i + 16]) % 1, u = 2 * age - 1
    const dx = samples[i] + u * (samples[i + 9] + u * samples[i + 12])
    const dy = samples[i + 1] + u * (samples[i + 10] + u * samples[i + 13])
    const dz = samples[i + 2] + u * (samples[i + 11] + u * samples[i + 14])
    const x = halfW + scale * (dx * r0 + dy * r1)
    const y = halfH + scale * (dx * d0 + dy * d1 + dz * d2)
    let ax = 1, ay = 0, aspect = 1
    const baseAspect = samples[i + 17]
    if (baseAspect > 1) {
      const tx = samples[i + 9] + 2 * u * samples[i + 12]
      const ty = samples[i + 10] + 2 * u * samples[i + 13]
      const tz = samples[i + 11] + 2 * u * samples[i + 14]
      const sx = tx * r0 + ty * r1, sy = tx * d0 + ty * d1 + tz * d2
      const projected = Math.hypot(sx, sy)
      if (projected > .01) {
        ax = sx / projected
        ay = sy / projected
        aspect += (baseAspect - 1) * Math.min(1, projected / Math.max(1, Math.hypot(tx, ty, tz)))
      }
    }
    const baseDiameter = samples[i + 3]
    const stride = strides[k]
    // 稀疏抽样时按面积守恒适度放宽宽雾/丝缕核半径，使邻近抽样无缝融合、消除孤立云球斑驳感。
    const spread = stride > 1
      ? (baseDiameter >= 3500 ? Math.min(1.85, 1 + .22 * Math.log2(stride)) : baseDiameter >= 640 ? Math.min(1.38, 1 + .10 * Math.log2(stride)) : 1)
      : 1
    const raw = baseDiameter * materialScale * Math.sqrt(aspect)
    // 近景让细尘保持纹理尺度，只有可点星核获得实心轮廓。
    const limit = baseDiameter < 640 ? 1.8 + baseDiameter * .0035 :
      Math.max(10, Math.min(180, 8 + baseDiameter * .014))
    const diameter = Math.max(1.2, raw / Math.sqrt(1 + (raw / limit) ** 2)) * spread
    if (x < -diameter || x > width + diameter || y < -diameter || y > height + diameter) continue
    const front = .5 + .5 * (samples[i + 18] * shadowX + samples[i + 19] * shadowY)
    const transmission = 1 - samples[i + 20] * (.29 + .59 * front * front)
    const edgeAge = age < 1 - age ? age : 1 - age
    const alpha = edgeLight * gain * samples[i + 4] * 1.19 * fadeEdge(edgeAge) * transmission
    const single = Math.min(1, alpha * Math.min(1, Math.max(.45, raw / 1.2)))
    const effectiveStride = stride === 1 ? 1 : stride / (spread * spread)
    const outAlpha = effectiveStride <= 1 ? single * effectiveStride : 1 - (1 - single) ** effectiveStride
    if (outAlpha <= .002) continue
    const index = samples[i + 5]
    const brush = brushes[index] ?? (brushes[index] = softwareBrush(index, theme))
    ctx.globalAlpha = outAlpha
    if (aspect > 1.01) {
      const minor = diameter / aspect
      ctx.setTransform(dpr * ax, dpr * ay, -dpr * ay, dpr * ax, x * dpr, y * dpr)
      ctx.drawImage(brush, -diameter / 2, -minor / 2, diameter, minor)
      rotated = true
    } else {
      if (rotated) { ctx.setTransform(dpr, 0, 0, dpr, 0, 0); rotated = false }
      ctx.drawImage(brush, x - diameter / 2, y - diameter / 2, diameter, diameter)
    }
  }
  if (rotated) ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  ctx.globalAlpha = 1
  ctx.globalCompositeOperation = 'source-over'
}

export function createNebulaLayer(redraw: () => void) {
  const surface = document.createElement('canvas'), fallback = document.createElement('canvas')
  const gl = surface.getContext('webgl', {alpha: true, antialias: false, depth: false, preserveDrawingBuffer: true})
  let fallbackCtx: CanvasRenderingContext2D | null = null
  let theme = GALAXY_THEMES.dark
  let pipeline = gl && createPipeline(gl, theme)
  let previous: {camera: Camera; samples: Float32Array; width: number; height: number; gain: number; dpr: number; tick: number} | undefined
  let software: ReturnType<typeof softwareParticles> | undefined
  let softwarePeak = 0
  const brushes: HTMLCanvasElement[] = []
  const lost = (event: Event) => { event.preventDefault(); pipeline = null; previous = undefined; redraw() }
  const restored = () => { pipeline = gl && createPipeline(gl, theme); previous = undefined; redraw() }
  surface.addEventListener('webglcontextlost', lost)
  surface.addEventListener('webglcontextrestored', restored)
  return {
    get flowInterval(): number { return pipeline ? 1000 / 30 : 1000 / 12 },
    render(samples: Float32Array, camera: Camera, size: ViewSize, dpr: number, level: number, theme_: GalaxyTheme, focused: boolean, elapsed: number): HTMLCanvasElement {
      if (theme_.mode !== theme.mode) {
        // 主题切换：换色带与混合方式，丢弃按旧色带缓存的软件笔刷与上一帧。
        theme = theme_
        pipeline?.setTheme(theme)
        brushes.length = 0
        previous = undefined
      }
      const output = pipeline ? surface : fallback
      // 软件回退使用柔和降采样离屏层再平滑放大回舞台，大幅降低像素填充带宽且保持水墨与云雾柔顺无锯齿。
      const resolution = pipeline ? dpr : Math.min(dpr, .68, 560 / Math.max(size.width, size.height))
      // level 为 0..1 的云雾亮度（存在感 × 数量曝光，由调用方缓动），镜头缩放只做轻微补偿。
      // 切换视区快照不会按缓冲区长度重新打光。
      const gain = 1.32 * theme.nebula.gain * level * (focused ? .12 : 1) / (1 + .30 * Math.log2(camera.zoom))
      const tick = Math.floor(elapsed / this.flowInterval)
      if (previous?.samples === samples && previous.camera === camera && previous.width === size.width &&
          previous.height === size.height && previous.dpr === dpr && previous.gain === gain && previous.tick === tick) return output
      const time = tick * this.flowInterval / FLOW_PERIOD_MS
      const angle = 2 * Math.PI * 3 * time
      const shadow = [Math.cos(angle), -Math.sin(angle)] as const
      const w = Math.max(1, Math.round(size.width * resolution)), h = Math.max(1, Math.round(size.height * resolution))
      if (output.width !== w || output.height !== h) { output.width = w; output.height = h }
      if (pipeline) pipeline.draw(samples, camera, size, dpr, gain, time, shadow)
      else {
        fallbackCtx ??= fallback.getContext('2d')
        if (fallbackCtx) {
          const count = samples.length / SAMPLE_STRIDE
          if (software?.samples !== samples) {
            // 稀疏目录（如 200→1 条）恢复全量抽样，避免沿用旧高水位导致单星雾团稀疏。
            softwarePeak = count <= 8000 || count < softwarePeak * .25 ? count : Math.max(softwarePeak, count)
            software = softwareParticles(samples, softwarePeak)
          }
          drawSoftware(fallbackCtx, brushes, theme, software, camera, size, resolution, gain, time, shadow)
        }
      }
      previous = {camera, samples, width: size.width, height: size.height, dpr, gain, tick}
      return output
    },
    dispose() {
      surface.removeEventListener('webglcontextlost', lost)
      surface.removeEventListener('webglcontextrestored', restored)
      pipeline?.release()
      gl?.getExtension('WEBGL_lose_context')?.loseContext()
      fallbackCtx = null
      surface.width = fallback.width = 1
    },
  }
}
