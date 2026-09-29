import * as THREE from 'three';
import { EffectComposer } from 'three/examples/jsm/postprocessing/EffectComposer.js';
import { RenderPass } from 'three/examples/jsm/postprocessing/RenderPass.js';
import { UnrealBloomPass } from 'three/examples/jsm/postprocessing/UnrealBloomPass.js';
import { OutputPass } from 'three/examples/jsm/postprocessing/OutputPass.js';
import type { Palette } from '../internal/palette';

/**
 * 后处理：RenderPass → Bloom → OutputPass（色彩空间转换）。
 * 低画质下仅关闭 Bloom，保持同一条渲染管线，画面色彩一致。
 */
export class PostProcess {
  private readonly composer: EffectComposer;
  private readonly bloomPass: UnrealBloomPass;
  private readonly outputPass: OutputPass;

  private userStrength = 1;
  private userThreshold = 0.1;
  private paletteScale = 1;
  private paletteThreshold = 0.1;
  private over = 0;
  private viewScale = 1;

  constructor(renderer: THREE.WebGLRenderer, scene: THREE.Scene, camera: THREE.Camera) {
    // 半浮点缓冲，保留恒星等高亮区域的 HDR 信息供 Bloom 使用
    const rt = new THREE.WebGLRenderTarget(1, 1, { type: THREE.HalfFloatType });
    this.composer = new EffectComposer(renderer, rt);
    this.bloomPass = new UnrealBloomPass(new THREE.Vector2(256, 256), 0.7, 0.45, 0.6);
    this.outputPass = new OutputPass();
    this.composer.addPass(new RenderPass(scene, camera));
    this.composer.addPass(this.bloomPass);
    this.composer.addPass(this.outputPass);
  }

  setSize(width: number, height: number, pixelRatio: number): void {
    this.composer.setPixelRatio(pixelRatio);
    this.composer.setSize(width, height);
  }

  setEnabled(enabled: boolean): void {
    this.bloomPass.enabled = enabled;
  }

  /** MSAA 采样数；后处理管线不走画布自带抗锯齿，细梁需要这里的多重采样 */
  setSamples(samples: number): void {
    for (const rt of [this.composer.renderTarget1, this.composer.renderTarget2]) {
      if (rt.samples !== samples) {
        rt.samples = samples;
        rt.dispose();
      }
    }
  }

  /** 用户设置的泛光强度 / 阈值 */
  setBloom(strength: number, threshold: number): void {
    this.userStrength = strength;
    this.userThreshold = threshold;
    this.refresh();
  }

  /**
   * 相机拉近时戴森壳占满画面，大量发光层叠加后 Bloom 会整片过曝，
   * 按相机距离比例（近 < 1）减弱泛光强度。
   */
  setViewScale(k: number): void {
    if (Math.abs(k - this.viewScale) < 1e-3) return;
    this.viewScale = k;
    this.refresh();
  }

  applyPalette(p: Palette): void {
    this.paletteScale = p.bloomScale;
    this.paletteThreshold = p.bloomThreshold;
    this.over = p.over;
    this.refresh();
  }

  private refresh(): void {
    this.bloomPass.strength = this.userStrength * this.paletteScale * this.viewScale;
    // 夜间使用用户阈值；日间向调色板阈值过渡，避免浅色背景整体发白
    this.bloomPass.threshold = THREE.MathUtils.lerp(
      this.userThreshold,
      Math.max(this.userThreshold, this.paletteThreshold),
      this.over,
    );
  }

  render(dt: number): void {
    this.composer.render(dt);
  }

  dispose(): void {
    this.bloomPass.dispose();
    this.outputPass.dispose();
    this.composer.dispose();
  }
}
