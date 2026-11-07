import * as THREE from 'three';
import { SKY_FRAGMENT, SKY_VERTEX } from './shaders';

/**
 * 背景：全屏天空渐变、星云、恒星与行星。
 * 天空不参与深度测试，始终最先绘制；平台周围不再绘制环境粒子。
 */

export class BackdropLayer {
  readonly group = new THREE.Group();
  private readonly skyGeometry: THREE.BufferGeometry;
  private readonly skyMaterial: THREE.ShaderMaterial;

  constructor(uniforms: Record<string, THREE.IUniform>) {
    // 覆盖整个裁剪空间的单个三角形
    this.skyGeometry = new THREE.BufferGeometry();
    this.skyGeometry.setAttribute('position', new THREE.Float32BufferAttribute([-1, -1, 0, 3, -1, 0, -1, 3, 0], 3));
    this.skyMaterial = new THREE.ShaderMaterial({
      vertexShader: SKY_VERTEX,
      fragmentShader: SKY_FRAGMENT,
      uniforms: {
        uSkyTop: uniforms.uSkyTop,
        uSkyBottom: uniforms.uSkyBottom,
        uNebulaA: uniforms.uNebulaA,
        uNebulaB: uniforms.uNebulaB,
        uStar: uniforms.uStar,
        uPlanet: uniforms.uPlanet,
        uAtmosphere: uniforms.uAtmosphere,
        uAspect: uniforms.uAspect,
        uTime: uniforms.uTime,
        uParams: uniforms.uParams,
      },
      depthTest: false,
      depthWrite: false,
    });
    const sky = new THREE.Mesh(this.skyGeometry, this.skyMaterial);
    sky.frustumCulled = false;
    sky.renderOrder = -1000;
    this.group.add(sky);
  }

  dispose(): void {
    this.skyGeometry.dispose();
    this.skyMaterial.dispose();
  }
}
