import * as THREE from 'three';
import type { BlueprintLayer } from '../blueprint';

type Face = [number, number, number];
type Point = THREE.Vector3;
type DrawTriangle = (cell: number, a: Point, b: Point, c: Point) => void;

const TAU = Math.PI * 2;
const north = new THREE.Vector3(0, 1, 0);
const south = new THREE.Vector3(0, -1, 0);

// 在 Unity 球层局部坐标下计算，写入 Three 缓冲时统一翻转 z。
function ring(count: number, y: number, start: number): Point[] {
  const radius = Math.sqrt(1 - y * y);
  return Array.from({ length: count }, (_, i) => {
    const angle = start + TAU * i / count;
    return new THREE.Vector3(radius * Math.sin(angle), y, radius * Math.cos(angle));
  });
}

function polyhedron(sides: number): { vertices: Point[]; faces: Face[]; frequency: number } {
  if (sides === 4) return {
    vertices: [north, ...ring(3, -1 / 3, 0)],
    faces: [[0,1,2], [0,2,3], [0,3,1], [1,3,2]], frequency: 72,
  };
  if (sides === 8) return {
    vertices: [north, ...ring(4, 0, -Math.PI / 4), south],
    faces: [[0,1,2], [0,2,3], [0,3,4], [0,4,1], [5,2,1], [5,3,2], [5,4,3], [5,1,4]], frequency: 48,
  };
  const y = 1 / Math.sqrt(5);
  return {
    vertices: [north, ...ring(5, y, 0), ...ring(5, -y, Math.PI / 5), south],
    faces: [[0,1,2], [0,2,3], [0,3,4], [0,4,5], [0,5,1],
      [6,2,1], [7,3,2], [8,4,3], [9,5,4], [10,1,5],
      [11,8,7], [11,7,6], [11,6,10], [11,10,9], [11,9,8],
      [3,7,8], [2,6,7], [1,10,6], [5,9,10], [4,8,9]], frequency: 24,
  };
}

function slerp(a: Point, b: Point, t: number): Point {
  const angle = Math.acos(THREE.MathUtils.clamp(a.dot(b), -1, 1));
  if (angle < 1e-7) return a.clone();
  return a.clone().multiplyScalar(Math.sin((1 - t) * angle))
    .addScaledVector(b, Math.sin(t * angle)).normalize();
}

// 从每个角沿大圆向对边插值，三个方向等权合成。普通重心坐标投影会扭曲高纬色块。
function radial(a: Point, b: Point, c: Point, wb: number, wc: number, n: number): Point {
  if (wb + wc === 0) return a.clone();
  return slerp(a, slerp(b, c, wc / (wb + wc)), (wb + wc) / n);
}

function geodesicCells(sides: number, draw: DrawTriangle): void {
  const { vertices, faces, frequency: n } = polyhedron(sides);
  let id = 0;
  for (const face of faces) {
    const [a, b, c] = face.map(index => vertices[index]);
    let previous: Point[] = [];
    for (let u = 0; u <= n; u++) {
      const row: Point[] = [];
      for (let v = 0; v <= n - u; v++) {
        const w = n - u - v;
        row.push(radial(a,b,c,u,v,n).add(radial(b,c,a,v,w,n)).add(radial(c,a,b,w,u,n)).normalize());
      }
      if (u > 0) {
        for (let v = 0; v < row.length; v++) draw(id++, previous[v], row[v], previous[v + 1]);
        for (let v = 0; v + 1 < row.length; v++) draw(id++, row[v], row[v + 1], previous[v + 1]);
      }
      previous = row;
    }
  }
}

// 南极至赤道每四个纬带共用同一列数，北半球镜像排列；一格对应相邻两个三角。
function graticuleCells(draw: DrawTriangle): void {
  const bands = [16, 32, 64, 80, 128, 128, 160, 160, 160, 160, 240, 240, 240, 240, 240];
  let cell = 0;
  const width = (row: number) => bands[Math.floor(Math.min(row, 119 - row) / 4)];
  function point(row: number, column: number, columns: number): Point {
    const latitude = (row / 120 - 0.5) * Math.PI;
    // 密带边界落在相邻疏带的弦上，使不同列数的纬带共用边。
    const coarse = Math.min(columns, width(Math.max(0, row - 1)), width(Math.min(119, row)));
    const fraction = column * coarse / columns;
    const left = Math.floor(fraction);
    const t = fraction - left;
    const longitude = TAU * left / coarse;
    const next = TAU * (left + 1) / coarse;
    const x = (1 - t) * Math.sin(longitude) + t * Math.sin(next);
    const z = (1 - t) * Math.cos(longitude) + t * Math.cos(next);
    return new THREE.Vector3(Math.cos(latitude) * x, Math.sin(latitude), -Math.cos(latitude) * z);
  }
  for (let row = 0; row < 120; row++) {
    const columns = width(row);
    let a = point(row, 0, columns);
    let b = point(row + 1, 0, columns);
    for (let col = 1; col <= columns; col++) {
      const c = point(row + 1, col, columns);
      const d = point(row, col, columns);
      draw(cell, a, b, c);
      draw(cell++, a, c, d);
      a = d;
      b = c;
    }
  }
}

/** 根据 Unity 绘制网格的颜色缓冲容量识别网格；预留尾部槽位没有几何。 */
export function forEachPaintTriangle(slots: number, draw: DrawTriangle): void {
  switch (slots) {
    case 54912: return graticuleCells(draw);
    case 31104: return geodesicCells(4, draw);
    case 27648: return geodesicCells(8, draw);
    case 17280: return geodesicCells(20, draw);
    default: throw new Error('此蓝图使用尚未支持的涂装网格');
  }
}

export function buildBlueprintPaint(layer: BlueprintLayer, radius: number): THREE.Mesh | null {
  if (!layer.cellColors) return null;
  const rgba = atob(layer.cellColors);
  let painted = false;
  for (let i = 3; i < rgba.length; i += 4) {
    if (rgba.charCodeAt(i)) { painted = true; break; }
  }
  if (!painted) return null;

  const positions: number[] = [];
  const colors: number[] = [];
  const tint = new THREE.Color();
  forEachPaintTriangle(rgba.length / 4, (id, a, b, c) => {
    const k = id * 4;
    const alpha = rgba.charCodeAt(k + 3);
    if (!alpha) return;
    tint.setRGB(rgba.charCodeAt(k) / 255, rgba.charCodeAt(k+1) / 255, rgba.charCodeAt(k+2) / 255, THREE.SRGBColorSpace);
    const bright = alpha > 127;
    if (bright) tint.multiplyScalar(1.6);
    for (const p of [a, b, c]) {
      positions.push(p.x * radius, p.y * radius, -p.z * radius);
      colors.push(tint.r, tint.g, tint.b, (bright ? alpha - 127 : alpha) / 127);
    }
  });
  if (!positions.length) return null;
  const geometry = new THREE.BufferGeometry();
  geometry.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
  geometry.setAttribute('color', new THREE.Float32BufferAttribute(colors, 4));
  const material = new THREE.MeshBasicMaterial({
    vertexColors: true, transparent: true, side: THREE.DoubleSide, depthWrite: false,
    stencilWrite: true, stencilWriteMask: 0, stencilFunc: THREE.EqualStencilFunc, stencilRef: 200 + layer.id,
    polygonOffset: true, polygonOffsetFactor: -2, polygonOffsetUnits: -2,
  });
  const mesh = new THREE.Mesh(geometry, material);
  mesh.scale.setScalar(1.0005);
  return mesh;
}
