import * as THREE from 'three';
import type { BlueprintLayer } from '../../blueprint';
import { objectColor, type FrameArc } from './geometry';
import { structureMaterial } from './materials';

const up = new THREE.Vector3(0, 1, 0);

export function addNodes(
  group: THREE.Group, layer: BlueprintLayer, nodes: Map<number, THREE.Vector3>,
  radius: number, gameRadius: number, dense: boolean, daylight: THREE.IUniform<number>,
): void {
  if (!nodes.size) return;
  // 接头包含外缘平台、环形电枢及伸向恒星的锥形桅杆。密集层减少轮廓边数。
  const profile = [[0, -1.9], [0.22, -1.3], [0.29, -0.3], [0.78, -0.12], [1, 0.1], [0.85, 0.4], [0.58, 0.55], [0.40, 0.75], [0, 0.78]];
  const geometry = dense ? new THREE.OctahedronGeometry(1) :
    new THREE.LatheGeometry(profile.map(([r, h]) => new THREE.Vector2(r, h)), nodes.size > 500 ? 6 : 10);
  const mesh = new THREE.InstancedMesh(geometry, structureMaterial(false, daylight), nodes.size);
  mesh.name = '蓝图节点';
  group.add(mesh);
  const matrix = new THREE.Matrix4();
  const rotation = new THREE.Quaternion();
  const pos = new THREE.Vector3();
  const size = radius * THREE.MathUtils.clamp(75 / gameRadius, dense ? 0.005 : 0.010, 0.028);
  const scale = new THREE.Vector3(size, size, size);
  for (let i = 0; i < layer.nodes.length; i++) {
    const node = layer.nodes[i];
    const n = nodes.get(node.id)!;
    rotation.setFromUnitVectors(up, n);
    mesh.setMatrixAt(i, matrix.compose(pos.copy(n).multiplyScalar(radius), rotation, scale));
    mesh.setColorAt(i, objectColor(node.color, 0x60d6fd));
  }
}

export function addFrames(
  group: THREE.Group, frames: Map<string, FrameArc>,
  radius: number, gameRadius: number, dense: boolean, daylight: THREE.IUniform<number>,
): void {
  if (!frames.size) return;
  let count = 0;
  for (const arc of frames.values()) count += arc.points.length - 1;
  // 密集作品中梁厚通常小于一个像素，保留径向宽度、段数和真实轨迹，省去梁背面的三角形。
  const geometry = dense ? new THREE.PlaneGeometry(1, 1).rotateX(-Math.PI / 2) : new THREE.BoxGeometry(1, 1, 1);
  const styles = new THREE.InstancedBufferAttribute(new Float32Array(count * 3), 3);
  geometry.setAttribute('beamStyle', styles);
  const material = structureMaterial(true, daylight);
  if (dense) material.side = THREE.DoubleSide;
  const mesh = new THREE.InstancedMesh(geometry, material, count);
  mesh.name = '蓝图框架';
  group.add(mesh);
  const outward = new THREE.Vector3(), tangent = new THREE.Vector3(), side = new THREE.Vector3();
  const position = new THREE.Vector3(), scale = new THREE.Vector3(), matrix = new THREE.Matrix4();
  const width = radius * THREE.MathUtils.clamp(55 / gameRadius, dense ? 0.0035 : 0.0075, 0.018);
  let index = 0;
  for (const { frame, points } of frames.values()) {
    const color = objectColor(frame.color, 0xe7925a);
    // 原版按 300 米一节并取偶数；短弧采样只影响梁轮廓，接头的节距单独保留。
    const segmentCount = 2 * Math.max(1, Math.round(points[0].angleTo(points[points.length - 1]) * gameRadius / 600));
    const repeat = segmentCount / (points.length - 1);
    for (let i = 1; i < points.length; i++) {
      const a = points[i - 1], b = points[i];
      position.addVectors(a, b).multiplyScalar(radius * 0.5);
      outward.copy(position).normalize();
      tangent.subVectors(b, a);
      const length = tangent.length() * radius;
      tangent.normalize();
      side.crossVectors(outward, tangent).normalize();
      matrix.makeBasis(side, outward, tangent);
      scale.set(width * (frame.proto === 2 ? 1.3 : 1), width * 0.26, length + width * 0.025);
      matrix.scale(scale).setPosition(position);
      mesh.setMatrixAt(index, matrix);
      mesh.setColorAt(index, color);
      styles.setXYZ(index++, frame.proto, repeat, (i - 1) * repeat);
    }
  }
}
