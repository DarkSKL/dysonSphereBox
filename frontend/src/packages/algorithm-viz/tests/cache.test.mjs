import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { setImmediate } from 'node:timers/promises';
import test from 'node:test';
import { build } from 'esbuild';

// 职责：验证跨任务回收、异步加载竞态与实例缓冲缩容；使用真实 Three 资源，替换网络和 WebGL 能力探测。
// 使用 Vite 已安装的编译器在内存中加载 TS，不生成测试构建残留。
const compiled = await build({
  stdin: {
    contents: `
      export * as THREE from 'three';
      export { GenerationCache } from './internal/generation-cache';
      export { ModelLibrary } from './render/models';
      export { InstancePool } from './render/pool';
      export { NATIVE_COLORS } from './render/native-colors';
      export { AlgorithmVizScene } from './scene';
    `,
    resolveDir: fileURLToPath(new URL('../src', import.meta.url)),
    loader: 'ts',
  },
  bundle: true, platform: 'node', format: 'cjs', target: 'es2020', write: false, logLevel: 'silent',
  define: { 'import.meta.url': JSON.stringify(import.meta.url) },
});
const module = { exports: {} };
new Function('module', 'exports', 'require', compiled.outputFiles[0].text)(module, module.exports, createRequire(import.meta.url));
const { THREE, GenerationCache, ModelLibrary, InstancePool, NATIVE_COLORS, AlgorithmVizScene } = module.exports;

const renderer = { extensions: { has: () => false } };
const tick = () => setImmediate();
const deferred = () => {
  let resolve;
  const promise = new Promise(done => { resolve = done; });
  return { promise, resolve };
};
const disposal = resource => {
  let count = 0;
  resource.addEventListener('dispose', () => { count++; });
  return () => count;
};
const part = (map, maskId = null, mask = null) => ({
  kind: 'solid', map, emissiveMap: map, orm: null, normalMap: null,
  color: new THREE.Color(), emissive: new THREE.Color(), native: false, unlit: false, maskId, mask,
});
const cacheModel = (library, key, texture = new THREE.Texture()) => {
  const entry = library.get(key);
  const geometry = new THREE.BoxGeometry();
  library.settle(entry, { geometry, parts: [part(texture)] });
  return { entry, geometry, texture };
};
const gltf = (texture, materialName = '') => {
  const scene = new THREE.Group();
  const material = new THREE.MeshStandardMaterial({ map: texture });
  material.name = materialName;
  scene.add(new THREE.Mesh(new THREE.BoxGeometry(), material));
  return { scene };
};
const networkLibrary = (maskUrls = {}) => {
  const library = new ModelLibrary({ a: '/a.glb', b: '/b.glb' }, renderer, undefined, maskUrls);
  const loads = [], masks = [];
  library.loader.loadAsync = url => {
    const task = { url, ...deferred() };
    loads.push(task);
    return task.promise;
  };
  library.maskLoader.loadAsync = () => {
    const task = deferred();
    masks.push(task);
    return task.promise;
  };
  return { library, loads, masks };
};
const [maskedMaterial, { mask: maskId }] = Object.entries(NATIVE_COLORS).find(([, value]) => value.mask);

test('只淘汰早于上次生成的资源，同任务通知与状态查看不推进代次', () => {
  const cache = new GenerationCache();
  assert.equal(cache.begin(''), false);
  cache.begin('a');
  cache.touch('only-a');
  cache.touch('shared');
  cache.begin('b');
  cache.touch('shared');
  assert.deepEqual(cache.stale(), []);
  assert.equal(cache.begin('b'), false);
  assert.deepEqual(cache.stale(), []);
  cache.begin('c');
  assert.deepEqual(cache.stale(), ['only-a']);
  cache.forget('only-a');
  assert.deepEqual(cache.stale(), []);
});

test('第三次生成释放第一代几何体，共享贴图保留到最后一个使用者过期', () => {
  const library = new ModelLibrary({}, renderer);
  library.beginGeneration('a');
  const shared = new THREE.Texture();
  const a = cacheModel(library, 'only-a', shared);
  cacheModel(library, 'shared', shared);
  const freedA = disposal(a.geometry), freedTexture = disposal(shared);
  library.beginGeneration('b');
  library.get('shared');
  const b = cacheModel(library, 'only-b');
  const freedB = disposal(b.geometry);
  library.beginGeneration('c');
  library.prune(new Set());
  assert.equal(library.peek('only-a'), undefined);
  assert.equal(freedA(), 1);
  assert.equal(freedTexture(), 0);
  assert.equal(freedB(), 0);
  library.peek('shared');
  library.beginGeneration('d');
  library.prune(new Set());
  assert.equal(freedTexture(), 1, 'peek 不延长资源寿命；复用到多个材质的贴图只释放一次');
  assert.equal(freedB(), 1);
  library.dispose();
  assert.equal(freedTexture(), 1);
});

test('旧实例仍在退场时保留模型，最后一个槽位归还后释放池和自有材质', () => {
  const library = new ModelLibrary({}, renderer);
  const parent = new THREE.Group();
  const scene = Object.assign(Object.create(AlgorithmVizScene.prototype), {
    disposed: false, supported: true, models: library, poolUsage: new GenerationCache(),
    pools: new Map(), owners: new Map(), ownedPools: new Set(), placeholders: new Map(), readyModels: new Set(),
    shadows: { beginGeneration() {} }, icons: { beginGeneration() {} }, touch() {},
  });
  scene.beginGeneration('a');
  const a = cacheModel(library, 'a');
  const material = new THREE.MeshBasicMaterial();
  const pool = new InstancePool('a', a.geometry, material, [], parent, 8);
  pool.alloc();
  scene.pools.set('a', pool);
  scene.owners.set('a', []);
  scene.ownedPools.add(pool);
  scene.poolUsage.touch('a');
  const freedGeometry = disposal(a.geometry), freedMaterial = disposal(material), freedMesh = disposal(pool.mesh);
  scene.beginGeneration('b');
  scene.beginGeneration('c');
  assert.equal(freedGeometry(), 0);
  assert.equal(scene.pools.has('a'), true);
  pool.clear();
  scene.pruneGenerationCache();
  assert.equal(scene.pools.has('a'), false);
  assert.equal(scene.owners.has('a'), false);
  assert.equal(parent.children.length, 0);
  assert.deepEqual([freedGeometry(), freedMaterial(), freedMesh()], [1, 1, 1]);
  library.dispose();
});

test('同名模型重新加载后，旧任务迟到的结果被释放且不覆盖新条目', async () => {
  const { library, loads } = networkLibrary();
  let settled = 0;
  library.onSettled(() => { settled++; });
  library.beginGeneration('a');
  const old = library.get('a');
  library.beginGeneration('b');
  library.beginGeneration('c');
  library.prune(new Set());
  const current = library.get('a');
  assert.notEqual(current, old);
  const currentTexture = new THREE.Texture(), oldTexture = new THREE.Texture();
  const freedCurrent = disposal(currentTexture), freedOld = disposal(oldTexture);
  loads[1].resolve(gltf(currentTexture));
  await tick();
  loads[0].resolve(gltf(oldTexture));
  await tick();
  assert.equal(library.peek('a'), current);
  assert.equal(current.status, 'ready');
  assert.equal(current.parts[0].map, currentTexture);
  assert.equal(settled, 1);
  assert.deepEqual([freedOld(), freedCurrent()], [1, 0]);
  library.dispose();
});

test('对撞机的可见网格正常加载，碰撞体仍被过滤', async () => {
  const { library, loads } = networkLibrary();
  const entry = library.get('a');
  const { scene } = gltf(new THREE.Texture());
  const body = scene.children[0];
  body.name = 'hadron-collider-2-3';
  const helper = body.clone();
  helper.name = 'hadron-collider-collider';
  scene.add(helper);
  loads[0].resolve({ scene });
  await tick();
  assert.equal(entry.status, 'ready');
  assert.equal(entry.geometry.index.count, body.geometry.index.count);
  library.dispose();
});

test('原版发光与特效贴图进入各自的材质分组，纯遮罩不参与绘制', async () => {
  const { library, loads, masks } = networkLibrary({
    'light-2914': '/emission.webp',
    'light-5085-mask-2486': '/effect.webp',
  });
  const entry = library.get('a');
  const body = gltf(new THREE.Texture(), 'hadron-collider');
  const effect = gltf(null, 'ray-receiver-eff');
  const helper = gltf(null, 'piler-mask');
  body.scene.add(effect.scene, helper.scene);
  loads[0].resolve(body);
  await tick();
  assert.equal(masks.length, 2);
  const emission = new THREE.Texture(), glow = new THREE.Texture();
  masks[0].resolve(emission);
  masks[1].resolve(glow);
  await tick();
  assert.equal(entry.status, 'ready');
  assert.deepEqual(entry.parts.map(part => [part.kind, part.light]), [['solid', emission], ['effect', glow]]);
  const freed = [disposal(emission), disposal(glow)];
  library.dispose();
  assert.deepEqual(freed.map(count => count()), [1, 1]);
});

test('淘汰等待遮罩的旧模型，不影响上一代共用同一遮罩的模型', async () => {
  const { library, loads, masks } = networkLibrary({ [maskId]: '/shared.png' });
  library.beginGeneration('a');
  const a = library.get('a');
  loads[0].resolve(gltf(new THREE.Texture(), maskedMaterial));
  await tick();
  const geometryA = library.pending.get(a).geometry;
  const freedA = disposal(geometryA);
  library.beginGeneration('b');
  const b = library.get('b');
  loads[1].resolve(gltf(new THREE.Texture(), maskedMaterial));
  await tick();
  assert.equal(masks.length, 1);
  library.beginGeneration('c');
  library.prune(new Set());
  assert.equal(freedA(), 1);
  const texture = new THREE.Texture(), freedMask = disposal(texture);
  masks[0].resolve(texture);
  await tick();
  assert.equal(b.status, 'ready');
  assert.equal(b.parts[0].mask, texture);
  assert.equal(freedA(), 1, '迟到回调不能重复释放已经回收的几何体');
  assert.equal(freedMask(), 0);
  library.beginGeneration('d');
  library.prune(new Set());
  assert.equal(freedMask(), 1);
  library.dispose();
});

test('过期遮罩请求迟到时释放自身，不覆盖同编号的新请求', async () => {
  const { library, loads, masks } = networkLibrary({ [maskId]: '/shared.png' });
  library.beginGeneration('a');
  library.get('a');
  loads[0].resolve(gltf(new THREE.Texture(), maskedMaterial));
  await tick();
  library.beginGeneration('b');
  library.beginGeneration('c');
  library.prune(new Set());
  const current = library.get('a');
  loads[1].resolve(gltf(new THREE.Texture(), maskedMaterial));
  await tick();
  assert.equal(masks.length, 2);
  const oldMask = new THREE.Texture(), newMask = new THREE.Texture();
  const freedOld = disposal(oldMask), freedNew = disposal(newMask);
  masks[1].resolve(newMask);
  masks[0].resolve(oldMask);
  await tick();
  assert.equal(current.parts[0].mask, newMask);
  assert.deepEqual([freedOld(), freedNew()], [1, 0]);
  library.dispose();
});

test('卸载发生在遮罩加载期间时，几何体、底色和迟到遮罩都只释放一次', async () => {
  const { library, loads, masks } = networkLibrary({ [maskId]: '/shared.png' });
  const texture = new THREE.Texture(), mask = new THREE.Texture();
  const freedTexture = disposal(texture), freedMask = disposal(mask);
  const entry = library.get('a');
  loads[0].resolve(gltf(texture, maskedMaterial));
  await tick();
  const freedGeometry = disposal(library.pending.get(entry).geometry);
  library.dispose();
  assert.deepEqual([freedGeometry(), freedTexture()], [1, 1]);
  masks[0].resolve(mask);
  await tick();
  assert.deepEqual([freedGeometry(), freedTexture(), freedMask()], [1, 1, 1]);
});

test('第三代缩小第一代的峰值缓冲，同时保留活动矩阵、属性与后续分配能力', () => {
  const parent = new THREE.Group(), base = new THREE.BoxGeometry(), material = new THREE.MeshBasicMaterial();
  const pool = new InstancePool('test', base, material, [{ name: 'state', itemSize: 2 }], parent, 8);
  pool.beginGeneration();
  for (let i = 0; i < 2048; i++) pool.alloc();
  pool.beginGeneration();
  pool.clear();
  for (let i = 0; i < 3; i++) {
    const slot = pool.alloc();
    pool.setMatrix(slot, new THREE.Matrix4().makeTranslation(i + 1, i + 2, i + 3));
    pool.set('state', slot, i + 4, i + 5);
  }
  assert.equal(pool.mesh.instanceMatrix.count, 2048, '第二代仍保留上次任务的峰值');
  const old = pool.mesh, freed = disposal(old), freedGeometry = disposal(old.geometry);
  pool.beginGeneration();
  assert.equal(pool.mesh.instanceMatrix.count, 8);
  assert.equal(pool.mesh.count, 3);
  assert.equal(parent.children.length, 1);
  assert.deepEqual([freed(), freedGeometry()], [1, 1]);
  const matrix = new THREE.Matrix4();
  pool.mesh.getMatrixAt(2, matrix);
  assert.deepEqual(matrix.elements.slice(12, 15), [3, 4, 5]);
  assert.deepEqual(Array.from(pool.mesh.geometry.getAttribute('state').array.slice(0, 6)), [4, 5, 5, 6, 6, 7]);
  pool.commit();
  for (const range of pool.mesh.instanceMatrix.updateRanges) assert.ok(range.start + range.count <= pool.mesh.instanceMatrix.array.length);
  assert.equal(pool.release(1), 2);
  assert.equal(pool.alloc(), 2);
  assert.deepEqual(Array.from(pool.mesh.geometry.getAttribute('state').array.slice(4, 6)), [0, 0]);
  pool.dispose();
  base.dispose();
  material.dispose();
});
