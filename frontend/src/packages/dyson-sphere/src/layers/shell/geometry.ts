import * as THREE from 'three';
import { mulberry32 } from '../../internal/utils';

// ---------------------------------------------------------------------------
// 测地线球几何（二十面体细分）与其对偶（六边形 Goldberg 多面体）
// ---------------------------------------------------------------------------

const PHI = (1 + Math.sqrt(5)) / 2;
const ICO_VERTS: [number, number, number][] = [
  [-1, PHI, 0], [1, PHI, 0], [-1, -PHI, 0], [1, -PHI, 0],
  [0, -1, PHI], [0, 1, PHI], [0, -1, -PHI], [0, 1, -PHI],
  [PHI, 0, -1], [PHI, 0, 1], [-PHI, 0, -1], [-PHI, 0, 1],
];
const ICO_FACES: [number, number, number][] = [
  [0, 11, 5], [0, 5, 1], [0, 1, 7], [0, 7, 10], [0, 10, 11],
  [1, 5, 9], [5, 11, 4], [11, 10, 2], [10, 7, 6], [7, 1, 8],
  [3, 9, 4], [3, 4, 2], [3, 2, 6], [3, 6, 8], [3, 8, 9],
  [4, 9, 5], [2, 4, 11], [6, 2, 10], [8, 6, 7], [9, 8, 1],
];

export type Edge = [number, number];

export interface Geodesic {
  /** 单位向量 */
  vertices: THREE.Vector3[];
  /** 顶点索引对 */
  edges: Edge[];
  /** 顶点索引三元组 */
  triangles: [number, number, number][];
}

/** 生成频率为 freq 的测地线球 */
export function buildGeodesic(freq: number): Geodesic {
  const base = ICO_VERTS.map((v) => new THREE.Vector3(...v).normalize());
  const vertices: THREE.Vector3[] = [];
  const lookup = new Map<string, number>();
  const keyOf = (v: THREE.Vector3) => `${Math.round(v.x * 1e4)},${Math.round(v.y * 1e4)},${Math.round(v.z * 1e4)}`;
  const addVertex = (v: THREE.Vector3) => {
    const k = keyOf(v);
    let idx = lookup.get(k);
    if (idx === undefined) {
      idx = vertices.length;
      vertices.push(v.clone());
      lookup.set(k, idx);
    }
    return idx;
  };

  const triangles: [number, number, number][] = [];
  const edgeSet = new Map<string, Edge>();
  const addEdge = (a: number, b: number) => {
    const k = a < b ? `${a}_${b}` : `${b}_${a}`;
    if (!edgeSet.has(k)) edgeSet.set(k, a < b ? [a, b] : [b, a]);
  };

  for (const face of ICO_FACES) {
    const [A, B, C] = face.map((i) => base[i]);
    const grid: number[][] = [];
    for (let i = 0; i <= freq; i++) {
      grid[i] = [];
      for (let j = 0; j <= freq - i; j++) {
        const p = new THREE.Vector3()
          .addScaledVector(A, 1 - (i + j) / freq)
          .addScaledVector(B, i / freq)
          .addScaledVector(C, j / freq)
          .normalize();
        grid[i][j] = addVertex(p);
      }
    }
    const pushTri = (a: number, b: number, c: number) => {
      triangles.push([a, b, c]);
      addEdge(a, b);
      addEdge(b, c);
      addEdge(c, a);
    };
    for (let i = 0; i < freq; i++) {
      for (let j = 0; j < freq - i; j++) {
        pushTri(grid[i][j], grid[i + 1][j], grid[i][j + 1]);
        if (i + j < freq - 1) pushTri(grid[i + 1][j], grid[i + 1][j + 1], grid[i][j + 1]);
      }
    }
  }

  return { vertices, edges: [...edgeSet.values()], triangles };
}

export interface GoldbergTile {
  /** 壳板中心方向（单位向量） */
  center: THREE.Vector3;
  /** 角点节点索引，按环绕顺序；六边形 6 个，五边形 5 个 */
  corners: number[];
}

export interface Goldberg {
  /** 框架节点（单位向量） */
  nodes: THREE.Vector3[];
  /** 框架梁（节点索引对） */
  edges: Edge[];
  tiles: GoldbergTile[];
}

/**
 * 测地线球的对偶：每个测地线顶点变成一块六边形（12 块五边形）壳板，
 * 三角形重心变成框架节点，相邻三角形的重心连线变成框架梁。
 */
export function buildGoldberg(freq: number): Goldberg {
  const { vertices, triangles } = buildGeodesic(freq);
  const nodes = triangles.map(([a, b, c]) =>
    new THREE.Vector3().add(vertices[a]).add(vertices[b]).add(vertices[c]).normalize(),
  );

  const adjacent: number[][] = vertices.map(() => []);
  const edgeTris = new Map<string, number[]>();
  triangles.forEach((tri, ti) => {
    for (let k = 0; k < 3; k++) {
      adjacent[tri[k]].push(ti);
      const a = tri[k];
      const b = tri[(k + 1) % 3];
      const key = a < b ? `${a}_${b}` : `${b}_${a}`;
      const list = edgeTris.get(key);
      if (list) list.push(ti);
      else edgeTris.set(key, [ti]);
    }
  });
  const edges = [...edgeTris.values()].filter((l) => l.length === 2) as Edge[];

  const t1 = new THREE.Vector3();
  const t2 = new THREE.Vector3();
  const tiles = vertices.map((center, vi) => {
    const around = adjacent[vi];
    t1.copy(nodes[around[0]]).addScaledVector(center, -nodes[around[0]].dot(center)).normalize();
    t2.crossVectors(center, t1);
    const angle = (ti: number) => Math.atan2(nodes[ti].dot(t2), nodes[ti].dot(t1));
    const corners = [...around].sort((a, b) => angle(a) - angle(b));
    return { center, corners };
  });

  return { nodes, edges, tiles };
}

/** 建造顺序：从种子方向向外扩散，叠加噪声，再按排名归一化到 [0, 1] */
export function computeOrder(dirs: THREE.Vector3[], seed: number): Float32Array {
  const seedDir = new THREE.Vector3(0.35, 0.85, 0.4).normalize();
  const rand = mulberry32(seed);
  const score = dirs.map((v) => (1 - v.dot(seedDir)) * 0.5 + rand() * 0.1);
  const idx = score.map((_, i) => i).sort((a, b) => score[a] - score[b]);
  const order = new Float32Array(dirs.length);
  const n = Math.max(dirs.length - 1, 1);
  idx.forEach((vi, rank) => (order[vi] = rank / n));
  return order;
}

/**
 * 结构梁几何：每条边一个 billboard 四边形。
 * orient(i0, i1) 返回 [起点, 终点, 建造顺序]，梁从起点向终点生长。
 */
export function buildBeamGeometry(
  points: THREE.Vector3[],
  edges: Edge[],
  radius: number,
  orient: (i0: number, i1: number) => [start: number, end: number, order: number],
): THREE.BufferGeometry {
  const n = edges.length;
  const aA = new Float32Array(n * 4 * 3);
  const aB = new Float32Array(n * 4 * 3);
  const aQuad = new Float32Array(n * 4 * 2);
  const aOrd = new Float32Array(n * 4);
  const aLen = new Float32Array(n * 4);
  const index = new Uint32Array(n * 6);
  const quad = [[0, -1], [1, -1], [1, 1], [0, 1]];
  const va = new THREE.Vector3();
  const vb = new THREE.Vector3();
  edges.forEach(([i0, i1], e) => {
    const [s, t, o] = orient(i0, i1);
    va.copy(points[s]).multiplyScalar(radius);
    vb.copy(points[t]).multiplyScalar(radius);
    const len = va.distanceTo(vb);
    for (let k = 0; k < 4; k++) {
      const vi = e * 4 + k;
      va.toArray(aA, vi * 3);
      vb.toArray(aB, vi * 3);
      aQuad[vi * 2] = quad[k][0];
      aQuad[vi * 2 + 1] = quad[k][1];
      aOrd[vi] = o;
      aLen[vi] = len;
    }
    index.set([e * 4, e * 4 + 1, e * 4 + 2, e * 4, e * 4 + 2, e * 4 + 3], e * 6);
  });
  const geo = new THREE.BufferGeometry();
  // position 仅用于包围盒计算
  geo.setAttribute('position', new THREE.BufferAttribute(aA, 3));
  geo.setAttribute('aA', new THREE.BufferAttribute(aA, 3));
  geo.setAttribute('aB', new THREE.BufferAttribute(aB, 3));
  geo.setAttribute('aQuad', new THREE.BufferAttribute(aQuad, 2));
  geo.setAttribute('aOrder', new THREE.BufferAttribute(aOrd, 1));
  geo.setAttribute('aLen', new THREE.BufferAttribute(aLen, 1));
  geo.setIndex(new THREE.BufferAttribute(index, 1));
  return geo;
}
