import * as THREE from 'three';
import { GLTFLoader, type GLTF } from 'three/examples/jsm/loaders/GLTFLoader.js';
import { KTX2Loader } from 'three/examples/jsm/loaders/KTX2Loader.js';
import { GRID_METERS } from '../defaults';
import { GenerationCache } from '../internal/generation-cache';
import { HIDDEN_PARTS, NATIVE_COLORS } from './native-colors';

/**
 * 模型库：按需加载 GLB，保留游戏原生贴图颜色。
 *
 * - 贴图为 KTX2/Basis 压缩格式，通过 KTX2Loader + Basis 转码器解码
 * - 同一模型的网格按材质分组合并为一个几何体（position / uv / index + groups），单位换算为蓝图格
 * - 加载失败的模型标记为 missing，场景继续使用占位体，不中断播放
 */

export type ModelStatus = 'idle' | 'loading' | 'ready' | 'missing';

/**
 * 材质分组的渲染类别：
 * - solid：机身，采样底色、自发光与金属/遮蔽贴图
 * - glass：玻璃罩，半透明着色，不画成实心体
 * - effect：游戏中的特效面片，黑底贴图加法叠加
 */
export type PartKind = 'solid' | 'glass' | 'effect';

/** 模型的一个材质分组：与几何体 groups 一一对应 */
export interface ModelPart {
  readonly kind: PartKind;
  readonly map: THREE.Texture | null;
  readonly emissiveMap: THREE.Texture | null;
  /** 遮蔽 / 粗糙 / 金属打包贴图（R 遮蔽、B 金属），仅机身材质使用 */
  readonly orm: THREE.Texture | null;
  /** 法线贴图，仅机身材质使用：提供板缝、螺栓等表面凹凸 */
  readonly normalMap: THREE.Texture | null;
  /** 底色系数（无贴图时即为底色） */
  readonly color: THREE.Color;
  /** 自发光系数 */
  readonly emissive: THREE.Color;
  /** color / emissive 是否来自游戏原生材质颜色表（只有这类分组可以按原色渲染） */
  readonly native: boolean;
  /** 涂装遮罩编号（见 native-colors.ts）；null 表示整个分组都染色 */
  readonly maskId: string | null;
  /** 自发光是否依赖供电（游戏中未供电时关闭，算法预览里的建筑都未通电，只保留微弱底光） */
  readonly unlit: boolean;
  /** 涂装遮罩贴图（单通道，亮处为涂装区）；由模型库按编号共享加载，未提供或加载失败时为 null */
  mask: THREE.Texture | null;
}

export interface ModelEntry {
  readonly key: string;
  status: ModelStatus;
  geometry: THREE.BufferGeometry | null;
  parts: ModelPart[];
  /** 包围盒尺寸（蓝图格） */
  readonly size: THREE.Vector3;
  /** 包围盒中心（蓝图格） */
  readonly center: THREE.Vector3;
}

/** Basis 转码器文件地址；缺省时使用 three 包内的相对路径 */
export interface TranscoderUrls {
  readonly js: string;
  readonly wasm: string;
}

/** 同时加载的模型数上限，避免解析集中在同一帧造成卡顿 */
const MAX_CONCURRENT = 3;

/** 名称中带有低精度层级或碰撞体标记的网格不参与合并 */
const SKIP_MESH = /(lod[1-9]|collider|-cl$|shadow)/i;

/** 按材质名判断类别：玻璃罩、特效面片（光效 / 遮罩 / 水面）之外都是机身 */
const classifyPart = (name: string): PartKind => (/glass/i.test(name) ? 'glass' : /(effect|-eff|mask|-water$)/i.test(name) ? 'effect' : 'solid');

export class ModelLibrary {
  private readonly entries = new Map<string, ModelEntry>();
  private readonly usage = new GenerationCache<string>();
  private readonly pending = new Map<ModelEntry, MergedModel>();
  private readonly discarded = new WeakSet<MergedModel>();
  private readonly loader: GLTFLoader;
  private readonly ktx2: KTX2Loader;
  private readonly waiting: string[] = [];
  private readonly listeners = new Set<(key: string) => void>();
  private readonly textures = new Set<THREE.Texture>();
  private active = 0;
  private disposed = false;

  private readonly maskLoader = new THREE.TextureLoader();
  /** 遮罩编号 → 加载结果；多个模型 / 材质共用同一张遮罩，失败记为 null 不再重试 */
  private readonly masks = new Map<string, { promise: Promise<THREE.Texture | null>; texture: THREE.Texture | null }>();

  constructor(
    private readonly urls: Readonly<Record<string, string>>,
    renderer: THREE.WebGLRenderer,
    transcoder?: TranscoderUrls,
    private readonly maskUrls: Readonly<Record<string, string>> = {},
  ) {
    // 转码器文件经构建工具重命名后无法按目录拼接，用地址改写把固定文件名映射到实际地址
    const manager = new THREE.LoadingManager();
    if (transcoder) {
      manager.setURLModifier((url) => (url.endsWith('basis_transcoder.js') ? transcoder.js : url.endsWith('basis_transcoder.wasm') ? transcoder.wasm : url));
    }
    this.ktx2 = new KTX2Loader(manager).detectSupport(renderer);
    this.loader = new GLTFLoader().setKTX2Loader(this.ktx2);
  }

  /** 是否提供了该模型的地址 */
  has(key: string): boolean {
    return Boolean(this.urls[key]);
  }

  /** 读取模型条目；首次读取时排队加载 */
  get(key: string): ModelEntry {
    this.usage.touch(key);
    let entry = this.entries.get(key);
    if (!entry) {
      entry = { key, status: 'idle', geometry: null, parts: [], size: new THREE.Vector3(1, 1, 1), center: new THREE.Vector3() };
      this.entries.set(key, entry);
      if (this.has(key)) {
        entry.status = 'loading';
        this.waiting.push(key);
        this.pump();
      } else {
        entry.status = 'missing';
      }
    }
    return entry;
  }

  /** 异步回调只查看状态，不把旧任务的迟到结果记为当前任务使用。 */
  peek(key: string): ModelEntry | undefined {
    return this.entries.get(key);
  }

  beginGeneration(taskId: string): void {
    this.usage.begin(taskId);
  }

  /** 先由场景释放实例池，再回收更早任务独占的模型及共享贴图。 */
  prune(protectedKeys: ReadonlySet<string>): void {
    for (const key of this.usage.stale()) {
      if (protectedKeys.has(key)) continue;
      const entry = this.entries.get(key);
      if (entry) {
        entry.geometry?.dispose();
        const merged = this.pending.get(entry);
        if (merged) this.discard(merged);
        this.pending.delete(entry);
        this.entries.delete(key);
      }
      this.usage.forget(key);
      for (let i = this.waiting.length - 1; i >= 0; i--) if (this.waiting[i] === key) this.waiting.splice(i, 1);
    }
    this.pruneTextures();
  }

  /** 预加载一组模型 */
  preload(keys: readonly string[]): void {
    for (const key of keys) this.get(key);
  }

  /** 模型就绪（或确认缺失）时回调，返回取消函数 */
  onSettled(listener: (key: string) => void): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  dispose(): void {
    this.disposed = true;
    this.listeners.clear();
    this.waiting.length = 0;
    for (const entry of this.entries.values()) entry.geometry?.dispose();
    for (const merged of this.pending.values()) this.discard(merged);
    this.pending.clear();
    for (const texture of this.textures) texture.dispose();
    for (const mask of this.masks.values()) mask.texture?.dispose();
    this.textures.clear();
    this.masks.clear();
    this.entries.clear();
    this.usage.clear();
    this.ktx2.dispose();
  }

  private pump(): void {
    while (!this.disposed && this.active < MAX_CONCURRENT && this.waiting.length) {
      const key = this.waiting.shift() as string;
      const entry = this.entries.get(key);
      if (!entry) continue;
      let merged: MergedModel | null = null;
      this.active++;
      this.loader
        .loadAsync(this.urls[key])
        .then(async (gltf) => {
          merged = mergeScene(gltf);
          if (merged && !this.disposed && this.entries.get(key) === entry) {
            this.pending.set(entry, merged);
            for (const texture of baseTextures(merged.parts)) this.textures.add(texture);
            await this.attachMasks(merged);
            this.pending.delete(entry);
          }
          this.settle(entry, merged);
        })
        .catch((err: unknown) => {
          this.pending.delete(entry);
          if (merged) this.discard(merged);
          this.pruneTextures();
          if (!this.disposed && this.entries.get(key) === entry) {
            console.warn(`[algorithm-viz] 模型 ${key} 加载失败，改用占位体`, err);
            this.settle(entry, null);
          }
        })
        .finally(() => {
          this.active--;
          this.pump();
        });
    }
  }

  /** 给带遮罩编号的分组挂上遮罩贴图；遮罩缺失时该分组整体染色，不影响模型显示 */
  private async attachMasks(merged: MergedModel): Promise<void> {
    await Promise.all(
      merged.parts.map(async (part) => {
        if (part.maskId) part.mask = await this.loadMask(part.maskId);
      }),
    );
  }

  private loadMask(id: string): Promise<THREE.Texture | null> {
    let task = this.masks.get(id);
    if (!task) {
      const record = { promise: Promise.resolve<THREE.Texture | null>(null), texture: null as THREE.Texture | null };
      this.masks.set(id, record);
      const url = this.maskUrls[id];
      record.promise = url
        ? this.maskLoader
            .loadAsync(url)
            .then((texture) => {
              // 与 glTF 贴图一致：不翻转，按数据而非颜色读取
              texture.flipY = false;
              texture.colorSpace = THREE.NoColorSpace;
              texture.needsUpdate = true;
              if (this.disposed || this.masks.get(id) !== record) {
                texture.dispose();
                return null;
              }
              record.texture = texture;
              return texture;
            })
            .catch((err: unknown) => {
              console.warn(`[algorithm-viz] 涂装遮罩 ${id} 加载失败，该材质整体染色`, err);
              return null;
            })
        : Promise.resolve(null);
      task = record;
    }
    return task.promise;
  }

  /** 已淘汰任务的异步结果不能覆盖同名的新条目。 */
  private settle(entry: ModelEntry, merged: MergedModel | null): void {
    if (this.disposed || this.entries.get(entry.key) !== entry) {
      if (merged) this.discard(merged);
      this.pruneTextures();
      return;
    }
    if (merged) {
      const geometry = merged.geometry;
      geometry.computeBoundingBox();
      geometry.computeBoundingSphere();
      const box = geometry.boundingBox as THREE.Box3;
      box.getSize(entry.size);
      box.getCenter(entry.center);
      entry.geometry = geometry;
      entry.parts = merged.parts;
      for (const texture of baseTextures(merged.parts)) this.textures.add(texture);
      entry.status = 'ready';
    } else {
      entry.status = 'missing';
    }
    for (const listener of this.listeners) listener(entry.key);
  }

  private discard(merged: MergedModel): void {
    if (this.discarded.has(merged)) return;
    this.discarded.add(merged);
    merged.geometry.dispose();
    for (const texture of baseTextures(merged.parts)) this.textures.add(texture);
  }

  /** 遮罩可能被多个模型和未完成的加载共享，只有没有任何引用时才释放。 */
  private pruneTextures(): void {
    const retained = new Set<THREE.Texture>();
    const maskIds = new Set<string>();
    for (const model of [...this.entries.values(), ...this.pending.values()]) {
      for (const texture of baseTextures(model.parts)) retained.add(texture);
      for (const part of model.parts) if (part.maskId) maskIds.add(part.maskId);
    }
    for (const texture of this.textures) if (!retained.has(texture)) {
      texture.dispose();
      this.textures.delete(texture);
    }
    for (const [id, mask] of this.masks) if (!maskIds.has(id)) {
      this.masks.delete(id);
      mask.texture?.dispose();
    }
  }
}

/** 底色等贴图属于模型，遮罩另按编号共享管理；同一材质复用的贴图只计一次。 */
function baseTextures(parts: readonly ModelPart[]): Set<THREE.Texture> {
  const textures = new Set<THREE.Texture>();
  for (const part of parts) for (const texture of [part.map, part.emissiveMap, part.orm, part.normalMap]) if (texture) textures.add(texture);
  return textures;
}

interface MergedModel {
  geometry: THREE.BufferGeometry;
  parts: ModelPart[];
}

interface Bucket {
  part: ModelPart;
  positions: number[];
  uvs: number[];
  indices: number[];
}

/**
 * 把 glTF 场景内的全部网格按世界变换合并：同一材质的网格放进同一分组，只保留位置、UV 与索引，
 * 并把米换算为蓝图格。量化或交错存储的顶点属性通过 getX/getY/getZ 统一读出。
 */
function mergeScene(gltf: GLTF): MergedModel | null {
  const root = gltf.scene;
  root.updateMatrixWorld(true);
  const buckets = new Map<THREE.Material, Bucket>();
  const v = new THREE.Vector3();
  const scale = 1 / GRID_METERS;
  const used = new Set<THREE.Texture>();
  root.traverse((object) => {
    const mesh = object as THREE.Mesh;
    if (!mesh.isMesh || !mesh.geometry || SKIP_MESH.test(mesh.name)) return;
    const material = (Array.isArray(mesh.material) ? mesh.material[0] : mesh.material) as THREE.MeshStandardMaterial;
    const source = mesh.geometry;
    const position = source.getAttribute('position');
    if (!position || position.itemSize < 3) return;
    const kind = classifyPart(material.name);
    // 预览器里隐藏的特效面片（未通电）与 alpha 丢失后会画成白板的面片不绘制
    if (HIDDEN_PARTS.has(material.name)) return;
    // 没有贴图的特效面片（遮罩、水面等）没有可绘制内容，直接略过
    if (kind === 'effect' && !material.map) return;
    let bucket = buckets.get(material);
    if (!bucket) {
      // 名称含 black 的无贴图材质在游戏中为深色部件，glTF 中缺省底色为白色，这里按名称还原
      const dark = !material.map && /black/i.test(material.name);
      const orm = kind === 'solid' ? (material.metalnessMap ?? material.roughnessMap ?? null) : null;
      const normalMap = kind === 'solid' ? (material.normalMap ?? null) : null;
      // GLB 里的底色系数全是白色，游戏中的色相来自材质 _Color；按材质名查原生颜色表（深色部件按名称还原，不查表）
      const native = kind === 'solid' && !dark ? NATIVE_COLORS[material.name] : undefined;
      const hasEmissiveMap = kind === 'solid' && Boolean(material.emissiveMap);
      bucket = {
        part: {
          kind,
          map: material.map ?? null,
          orm,
          normalMap,
          emissiveMap: kind === 'solid' ? (material.emissiveMap ?? null) : null,
          color: dark ? new THREE.Color(0x1c2028) : native ? new THREE.Color().setRGB(...native.color, THREE.LinearSRGBColorSpace) : (material.color?.clone() ?? new THREE.Color(1, 1, 1)),
          emissive: hasEmissiveMap
            ? (native?.emissive ? new THREE.Color().setRGB(...native.emissive, THREE.LinearSRGBColorSpace) : new THREE.Color(1, 1, 1))
            : (kind === 'solid' ? (material.emissive?.clone() ?? new THREE.Color(0, 0, 0)) : new THREE.Color(0, 0, 0)),
          native: Boolean(native),
          maskId: native?.mask ?? null,
          unlit: Boolean(native?.unlit),
          mask: null,
        },
        positions: [],
        uvs: [],
        indices: [],
      };
      buckets.set(material, bucket);
      if (material.map) used.add(material.map);
      if (bucket.part.emissiveMap) used.add(bucket.part.emissiveMap);
      if (orm) used.add(orm);
      if (normalMap) used.add(normalMap);
    }
    const uv = source.getAttribute('uv');
    const base = bucket.positions.length / 3;
    for (let i = 0; i < position.count; i++) {
      v.set(position.getX(i), position.getY(i), position.getZ(i)).applyMatrix4(mesh.matrixWorld);
      bucket.positions.push(v.x * scale, v.y * scale, v.z * scale);
      bucket.uvs.push(uv ? uv.getX(i) : 0, uv ? uv.getY(i) : 0);
    }
    const index = source.getIndex();
    if (index) {
      for (let i = 0; i < index.count; i++) bucket.indices.push(base + index.getX(i));
    } else {
      for (let i = 0; i < position.count; i++) bucket.indices.push(base + i);
    }
  });
  // 释放 GLTFLoader 创建的几何体与未使用的贴图；用到的贴图交给模型库统一管理
  root.traverse((object) => {
    const mesh = object as THREE.Mesh;
    if (!mesh.isMesh) return;
    mesh.geometry?.dispose();
    const materials = Array.isArray(mesh.material) ? mesh.material : [mesh.material];
    for (const material of materials as THREE.MeshStandardMaterial[]) {
      for (const texture of [material.normalMap, material.metalnessMap, material.roughnessMap, material.aoMap]) {
        if (texture && !used.has(texture)) texture.dispose();
      }
      material.dispose();
    }
  });
  if (!buckets.size) return null;
  const positions: number[] = [];
  const uvs: number[] = [];
  const indices: number[] = [];
  const parts: ModelPart[] = [];
  const geometry = new THREE.BufferGeometry();
  for (const bucket of buckets.values()) {
    const offset = positions.length / 3;
    const start = indices.length;
    for (const value of bucket.positions) positions.push(value);
    for (const value of bucket.uvs) uvs.push(value);
    for (const index of bucket.indices) indices.push(index + offset);
    geometry.addGroup(start, bucket.indices.length, parts.length);
    parts.push(bucket.part);
  }
  geometry.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
  geometry.setAttribute('uv', new THREE.Float32BufferAttribute(uvs, 2));
  const vertexCount = positions.length / 3;
  geometry.setIndex(vertexCount > 65535 ? new THREE.Uint32BufferAttribute(indices, 1) : new THREE.Uint16BufferAttribute(indices, 1));
  return { geometry, parts };
}
