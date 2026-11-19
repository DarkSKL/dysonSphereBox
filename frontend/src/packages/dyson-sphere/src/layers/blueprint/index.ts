import * as THREE from 'three';
import type { BlueprintAppearance, BlueprintLayer, BlueprintOrbit, SphereBlueprint } from '../../blueprint';
import { RENDER_ORDER } from '../../internal/utils';
import { buildBlueprintPaint } from '../blueprint-paint';
import { SHELL_RADIUS } from '../shell';
import { layerRotation, layerTopology } from './geometry';
import { panelMaterial } from './materials';
import { PanelGeometry } from './panels';
import { addFrames, addNodes } from './structure';

const SWARM_RADIUS = 3.35;

/** 轨道蓝图只有轨道设计。按 HSVA、半径和倾角绘制路径，停用轨道使用暗色虚线。 */
function orbitLine(orbit: BlueprintOrbit, radius: number): THREE.LineLoop {
  const [h, s, v, alpha] = orbit.hsva.length === 4 ? orbit.hsva : [0.069444, 0.35, 1, 1];
  const value = (offset: number) => {
    const k = THREE.MathUtils.euclideanModulo(h * 6 + offset, 6);
    return v * (1 - s * Math.max(0, Math.min(k, 4 - k, 1)));
  };
  const color = new THREE.Color().setRGB(value(5), value(3), value(1), THREE.SRGBColorSpace);
  const points: THREE.Vector3[] = [];
  for (let i = 0; i < 256; i++) {
    const angle = i * Math.PI * 2 / 256;
    points.push(new THREE.Vector3(radius * Math.cos(angle), 0, radius * Math.sin(angle)));
  }
  const params = { color, transparent: true, opacity: Math.min(1, alpha) * (orbit.enabled ? 0.46 : 0.20), depthWrite: false };
  const rotation = layerRotation(orbit.rotation);
  const geometry = new THREE.BufferGeometry().setFromPoints(points);
  const material = orbit.enabled ? new THREE.LineBasicMaterial(params) :
    new THREE.LineDashedMaterial({ ...params, dashSize: radius * 0.075, gapSize: radius * 0.045 });
  const line = new THREE.LineLoop(geometry, material);
  line.name = `太阳帆轨道 ${orbit.id}`;
  line.quaternion.copy(rotation);
  line.renderOrder = RENDER_ORDER.cellsFront + 1;
  if (!orbit.enabled) line.computeLineDistances();
  return line;
}

/** 一次蓝图切换持有的全部资源。在离屏试绘通过后才把 group 移入正式场景。 */
export class BlueprintDisplay {
  readonly group = new THREE.Group();
  readonly daylight = { value: 0 };
  readonly starScale: number;
  private readonly structures: THREE.Group[] = [];
  private readonly orbits = new THREE.Group();
  private readonly surfaceDetail = { value: 1 };

  setAppearance(appearance: BlueprintAppearance): void {
    for (const group of this.structures) group.visible = appearance.structure;
    this.surfaceDetail.value = appearance.surfaceDetail ? 1 : 0;
    this.orbits.visible = appearance.orbits;
  }

  constructor(model: SphereBlueprint) {
    this.group.name = '玩家蓝图';
    const layers = model.layers.filter(layer => layer.visible && layer.nodes.length);
    const orbits = model.orbits.filter(orbit => orbit.visible);
    const radii = [...layers, ...orbits].map(item => item.radius);
    if (radii.some(r => !Number.isFinite(r) || r <= 0)) throw new Error('蓝图轨道半径无效');
    const largestLayer = layers.reduce((max, layer) => Math.max(max, layer.radius), 0);
    const largestOrbit = orbits.reduce((max, orbit) => Math.max(max, orbit.radius), 0);
    const scale = Math.min(SHELL_RADIUS / largestLayer, SWARM_RADIUS / largestOrbit);
    this.starScale = radii.length ? Math.min(1, Math.min(...radii) * scale * 0.54) : 1;
    try {
      const dense = layers.length > 4 || layers.reduce((count, layer) => count + layer.nodes.length, 0) > 2000;
      for (const layer of layers) {
        // 单层导入会投射到游戏选中的轨道。生成器常导出单位向量，此时以一万米展示层确定板格和梁宽。
        const gameRadius = model.type === 1 && layer.radius < 100 ? 10000 : layer.radius;
        this.addLayer(layer, layer.radius * scale, gameRadius, dense);
      }
      this.group.add(this.orbits);
      for (const orbit of orbits) this.orbits.add(orbitLine(orbit, orbit.radius * scale));
    } catch (error) {
      this.dispose();
      throw error;
    }
  }

  private addLayer(layer: BlueprintLayer, radius: number, gameRadius: number, dense: boolean): void {
    const group = new THREE.Group();
    group.name = `球层 ${layer.id}`;
    group.quaternion.copy(layerRotation(layer.rotation));
    this.group.add(group);
    const { nodes, frames } = layerTopology(layer);
    if (layer.shells.length) {
      const panels = new PanelGeometry();
      for (const shell of layer.shells) panels.add(shell, frames, radius, gameRadius);
      const mesh = new THREE.Mesh(panels.build(), panelMaterial(layer.id, this.daylight, this.surfaceDetail));
      mesh.name = '蓝图壳面';
      // 恒星、塔和梁先写深度，只有最终可见的板才能留下涂装模板。
      mesh.renderOrder = 1;
      group.add(mesh);
      const paint = buildBlueprintPaint(layer, radius);
      if (paint) {
        this.addPaint(group, mesh, paint, layer.id);
      }
    }
    const structure = new THREE.Group();
    group.add(structure);
    this.structures.push(structure);
    addFrames(structure, frames, radius, gameRadius, dense, this.daylight);
    addNodes(structure, layer, nodes, radius, gameRadius, dense, this.daylight);
  }

  /** 内外表面各自记录层 ID；从近侧镂空看到内壁时，只取远侧格点的颜色。 */
  private addPaint(
    group: THREE.Group, surface: THREE.Mesh<THREE.BufferGeometry, THREE.ShaderMaterial>,
    paint: THREE.Mesh, id: number,
  ): void {
    const innerSurface = new THREE.Mesh(surface.geometry, panelMaterial(id, this.daylight, this.surfaceDetail));
    surface.material.side = THREE.FrontSide;
    innerSurface.material.side = THREE.BackSide;
    innerSurface.material.stencilRef = 128 + id;
    innerSurface.renderOrder = surface.renderOrder;
    innerSurface.name = '涂装壳内壁';
    group.add(innerSurface);

    // 原版绘制网格的绕序在 Z 反射后朝内。可见性已由壳板模板决定，以原半径投影可避免边缘色偏。
    const material = paint.material as THREE.MeshBasicMaterial;
    material.side = THREE.BackSide;
    material.depthTest = false;
    paint.scale.setScalar(1);
    paint.name = '逐格涂装';
    paint.renderOrder = RENDER_ORDER.cellsFront;
    const innerPaint = new THREE.Mesh(paint.geometry, material.clone());
    innerPaint.material.side = THREE.FrontSide;
    innerPaint.material.stencilRef = innerSurface.material.stencilRef;
    innerPaint.renderOrder = paint.renderOrder;
    innerPaint.name = '内壁逐格涂装';
    group.add(paint, innerPaint);
  }

  dispose(): void {
    this.group.removeFromParent();
    const geometries = new Set<THREE.BufferGeometry>();
    this.group.traverse(obj => {
      if (!(obj instanceof THREE.Mesh || obj instanceof THREE.Line)) return;
      if (obj instanceof THREE.InstancedMesh) obj.dispose();
      geometries.add(obj.geometry);
      const materials = Array.isArray(obj.material) ? obj.material : [obj.material];
      for (const material of materials) material.dispose();
    });
    for (const geometry of geometries) geometry.dispose();
    this.group.clear();
  }
}
