import * as THREE from 'three';
import type { BlueprintLayer } from '../../blueprint';
import { frameKey, MAX_ARC, objectColor, type FrameArc } from './geometry';

type Shell = BlueprintLayer['shells'][number];

/** 按层合批。相邻三角形共享索引；不同壳面各自保留切平面、原型、种子和涂色。 */
export class PanelGeometry {
  private readonly positions: number[] = [];
  private readonly uvs: number[] = [];
  private readonly colors: number[] = [];
  private readonly styles: number[] = [];
  private readonly indices: number[] = [];

  add(shell: Shell, frames: Map<string, FrameArc>, radius: number, gameRadius: number): void {
    const rim: THREE.Vector3[] = [];
    const center = new THREE.Vector3();
    const areaNormal = new THREE.Vector3();
    const cross = new THREE.Vector3();
    const corners: number[] = [];
    for (let i = 0; i < shell.nodes.length; i++) {
      const a = shell.nodes[i];
      const b = shell.nodes[(i + 1) % shell.nodes.length];
      const arc = frames.get(frameKey(a, b));
      if (!arc) throw new Error(`壳面 ${shell.id} 的边界缺少框架 ${a}–${b}`);
      const { frame, points } = arc;
      const first = points[0], last = points[points.length - 1];
      const length = first.distanceTo(last);
      center.addScaledVector(first, length).addScaledVector(last, length);
      for (let j = 0; j < points.length - 1; j++) {
        if (j === 0 || frame.euler) corners.push(rim.length);
        rim.push(points[frame.a === a ? j : points.length - 1 - j]);
      }
    }
    if (rim.length < 3) throw new Error(`壳面 ${shell.id} 顶点不足`);
    if (center.lengthSq() < 1e-16) {
      for (let i = 0; i < rim.length; i++) {
        areaNormal.add(cross.crossVectors(rim[i], rim[(i + 1) % rim.length]));
      }
      center.copy(areaNormal);
    }
    if (center.lengthSq() < 1e-16) throw new Error(`壳面 ${shell.id} 无法确定投影方向`);
    center.normalize();
    // 原版 x 轴是 Unity 内 center × up，反射到 Three 后叉积次序反转。
    const xAxis = new THREE.Vector3(0, 1, 0).cross(center);
    if (xAxis.lengthSq() < 1e-12) xAxis.set(0, 0, -1);
    xAxis.normalize();
    const yAxis = center.clone().cross(xAxis).normalize();
    // 心射投影下大圆是直线，避免长边采样产生狭长耳片；超出切平面视域时改用球极投影。
    const wide = rim.some(p => p.dot(center) < 0.2);
    const directions = wide ? rim.slice() : corners.map(i => rim[i]);
    const flat = directions.map(p => {
      const denom = p.dot(center) + (wide ? 1 : 0);
      if (denom < 1e-7) throw new Error(`壳面 ${shell.id} 横跨投影对极`);
      return new THREE.Vector2(p.dot(xAxis) / denom, p.dot(yAxis) / denom);
    });
    const faces = THREE.ShapeUtils.triangulateShape(flat, []);
    const area = Math.abs(THREE.ShapeUtils.area(flat));
    let filled = 0;
    for (const [a, b, c] of faces) {
      filled += Math.abs((flat[b].x - flat[a].x) * (flat[c].y - flat[a].y) -
        (flat[b].y - flat[a].y) * (flat[c].x - flat[a].x)) * 0.5;
    }
    if (!faces.length || area < 1e-14 || Math.abs(filled - area) > area * 1e-5) {
      throw new Error(`壳面 ${shell.id} 的边界无法三角化`);
    }

    const gridSize = 80 * Math.max(1, Math.round(Math.pow(gameRadius / 4000, 0.75)));
    const scale = gameRadius / gridSize;
    const color = objectColor(shell.color, 0x60d6fd);
    const start = this.positions.length / 3;
    const midpoints = new Map<string, number>();
    const midpoint = (a: number, b: number): number => {
      const key = frameKey(a, b);
      const old = midpoints.get(key);
      if (old !== undefined) return old;
      let p: THREE.Vector3;
      if (wide) {
        const uv = flat[a].clone().lerp(flat[b], 0.5);
        flat.push(uv);
        const sq = uv.lengthSq();
        p = center.clone().multiplyScalar(1 - sq)
          .addScaledVector(xAxis, 2 * uv.x).addScaledVector(yAxis, 2 * uv.y).multiplyScalar(1 / (1 + sq));
      } else {
        p = directions[a].clone().add(directions[b]).normalize();
      }
      const index = directions.push(p) - 1;
      midpoints.set(key, index);
      return index;
    };
    // 二分最长边并缓存中点，球面弦差受统一角度限制；不在动画帧中重建。
    while (faces.length) {
      const [a, b, c] = faces.pop()!;
      const ab = directions[a].distanceToSquared(directions[b]);
      const bc = directions[b].distanceToSquared(directions[c]);
      const ca = directions[c].distanceToSquared(directions[a]);
      const longest = Math.max(ab, bc, ca);
      if (longest <= MAX_ARC * MAX_ARC) {
        this.indices.push(start + a, start + b, start + c);
        continue;
      }
      const [i, j, k] = longest === ab ? [a, b, c] : longest === bc ? [b, c, a] : [c, a, b];
      const m = midpoint(i, j);
      faces.push([i, m, k], [m, j, k]);
      if (directions.length + start > 1_500_000) throw new Error('球层超出预览网格容量');
    }
    for (const p of directions) {
      this.positions.push(p.x * radius, p.y * radius, p.z * radius);
      const uvScale = scale / Math.max(0.1, p.dot(center));
      this.uvs.push(p.dot(xAxis) * uvScale, p.dot(yAxis) * uvScale);
      this.colors.push(color.r, color.g, color.b);
      this.styles.push(shell.proto, (shell.seed >>> 0) % 65521, (shell.color & 255) / 255);
    }
  }

  build(): THREE.BufferGeometry {
    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute('position', new THREE.Float32BufferAttribute(this.positions, 3));
    geometry.setAttribute('uv', new THREE.Float32BufferAttribute(this.uvs, 2));
    geometry.setAttribute('color', new THREE.Float32BufferAttribute(this.colors, 3));
    geometry.setAttribute('panelStyle', new THREE.Float32BufferAttribute(this.styles, 3));
    geometry.setIndex(this.indices);
    return geometry;
  }
}
