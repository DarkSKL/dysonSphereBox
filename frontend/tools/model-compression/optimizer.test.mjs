import assert from 'node:assert/strict';
import test from 'node:test';
import {createRequire} from 'node:module';
import {mkdtemp, mkdir, readFile, readdir, realpath, rm, writeFile} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {resolve, sep} from 'node:path';
import {readGlb, bufferViewData, optimizeGlb} from './glb.mjs';
import {readKtx2, trimKtx2} from './ktx2.mjs';
import {FRONTEND_ROOT, prepareModel, resolveMaxSize, resolveCompressionMode, COMPRESSION_MODES} from './prepare.mjs';
import {modelCompressionPlugin} from './vite-plugin.mjs';

// 职责：以真实资源检查 mip 裁剪、几何完整性、缓存和构建替换；不生成或修改源模型。
const sourceDir = resolve(FRONTEND_ROOT, 'src/data/models');
const names = (await readdir(sourceDir)).filter(name => name.endsWith('.glb')).sort();
const source = await readFile(resolve(sourceDir, 'assembler-1.glb'));

function semantics(document) {
  const value = structuredClone(document);
  if (value.buffers?.[0]) delete value.buffers[0].byteLength;
  const images = new Set((value.images ?? []).map(image => image.bufferView));
  for (let i = 0; i < (value.bufferViews?.length ?? 0); i++) {
    delete value.bufferViews[i].byteOffset;
    if (images.has(i)) delete value.bufferViews[i].byteLength;
  }
  return value;
}

for (const size of [1024, 512, 256]) test(`${size}px：全部模型的网格字节、材质、节点与保留的压缩 mip 不变`, async () => {
  for (const name of names) {
    const input = await readFile(resolve(sourceDir, name));
    const original = Buffer.from(input);
    const before = readGlb(input);
    const result = optimizeGlb(input, size);
    const after = readGlb(result.buffer);
    assert.deepEqual(input, original, `${name} 输入被修改`);
    assert.deepEqual(semantics(after.document), semantics(before.document), `${name} 模型语义改变`);
    const imageViews = new Set((before.document.images ?? []).map(image => image.bufferView));
    for (let index = 0; index < (before.document.bufferViews?.length ?? 0); index++) {
      const a = bufferViewData(before, index), b = bufferViewData(after, index);
      if (!imageViews.has(index)) { assert.deepEqual(b, a, `${name} 网格数据改变`); continue; }
      const oldTexture = readKtx2(a), newTexture = readKtx2(b);
      const drop = oldTexture.levels.length - newTexture.levels.length;
      assert.deepEqual(newTexture.dfd, oldTexture.dfd);
      for (let i = 0; i < newTexture.levels.length; i++) assert.deepEqual(newTexture.levels[i].data, oldTexture.levels[i + drop].data);
      assert.equal(newTexture.width % 4, 0); assert.equal(newTexture.height % 4, 0);
    }
    assert.ok(result.buffer.length <= input.length, `${name} 体积增大`);
  }
});

test('真实 Basis WASM 可转码所有压缩贴图的首尾 mip，像素与原文件对应 mip 一致', async () => {
  const basisDir = resolve(FRONTEND_ROOT, 'node_modules/three/examples/jsm/libs/basis');
  const code = await readFile(resolve(basisDir, 'basis_transcoder.js'), 'utf8');
  const factory = new Function('require', '__dirname', `${code};return BASIS;`)(createRequire(import.meta.url), basisDir);
  const basis = await factory({wasmBinary: await readFile(resolve(basisDir, 'basis_transcoder.wasm'))});
  basis.initializeBasis();
  const decode = (file, level) => {
    const data = new Uint8Array(file.getImageTranscodedSizeInBytes(level, 0, 0, 13));
    assert.ok(file.transcodeImage(data, level, 0, 0, 13, 0, -1, -1));
    return data;
  };
  let count = 0;
  for (const name of names) {
    const glb = readGlb(await readFile(resolve(sourceDir, name)));
    for (const image of glb.document.images ?? []) {
      if (image.mimeType !== 'image/ktx2') continue;
      const original = bufferViewData(glb, image.bufferView);
      const before = new basis.KTX2File(new Uint8Array(original));
      try {
        assert.ok(before.isValid() && before.startTranscoding(), name);
        for (const size of [1024, 512, 256]) {
          const result = trimKtx2(original, size);
          const after = new basis.KTX2File(new Uint8Array(result.buffer));
          try {
            assert.ok(after.isValid() && after.startTranscoding(), `${name}/${size}`);
            assert.equal(after.getWidth(), result.width); assert.equal(after.getHeight(), result.height);
            for (const mip of new Set([0, after.getLevels() - 1])) assert.deepEqual(decode(after, mip), decode(before, mip + result.removedLevels));
          } finally { after.close(); after.delete(); }
        }
        count++;
      } finally { before.close(); before.delete(); }
    }
  }
  assert.ok(count >= 300, `仅验证了 ${count} 张贴图`);
});

test('错误文件、无效分辨率及缺失 mip 明确报错，特殊边长贴图保留兼容性', () => {
  assert.throws(() => readGlb(Buffer.alloc(20)));
  const glb = readGlb(source);
  const texture = bufferViewData(glb, glb.document.images[0].bufferView);
  assert.throws(() => readKtx2(texture.subarray(0, 90)));
  assert.throws(() => trimKtx2(texture, 0));
  assert.throws(() => resolveMaxSize('bad'));
  const missing = Buffer.from(texture); missing.writeUInt32LE(1, 40);
  assert.throws(() => trimKtx2(missing, 512), /mip/);
});

test('三档模式默认选择均衡，显式模式覆盖终端残留的分辨率设置', () => {
  const savedMode = process.env.MODEL_COMPRESSION_MODE;
  const savedSize = process.env.MODEL_TEXTURE_MAX_SIZE;
  try {
    delete process.env.MODEL_COMPRESSION_MODE;
    delete process.env.MODEL_TEXTURE_MAX_SIZE;
    assert.equal(resolveCompressionMode().mode, 'balanced');
    assert.equal(resolveMaxSize(), 512);
    process.env.MODEL_TEXTURE_MAX_SIZE = '4096';
    for (const [mode, profile] of Object.entries(COMPRESSION_MODES)) {
      process.env.MODEL_COMPRESSION_MODE = mode;
      assert.equal(resolveMaxSize(), profile.maxSize);
    }
    assert.throws(() => resolveCompressionMode('unknown'));
  } finally {
    if (savedMode === undefined) delete process.env.MODEL_COMPRESSION_MODE; else process.env.MODEL_COMPRESSION_MODE = savedMode;
    if (savedSize === undefined) delete process.env.MODEL_TEXTURE_MAX_SIZE; else process.env.MODEL_TEXTURE_MAX_SIZE = savedSize;
  }
});

test('缓存复用、损坏缓存重建以及仅构建替换均不覆盖源文件', async () => {
  const root = await mkdtemp(resolve(tmpdir(), 'dsbox-model-test-'));
  try {
    const file = resolve(root, 'src/data/models/assembler-1.glb');
    await mkdir(resolve(root, 'src/data/models'), {recursive: true});
    await writeFile(file, source);
    const first = await prepareModel(file, 512, root);
    assert.equal(first.cached, false);
    assert.equal((await prepareModel(file, 512, root)).cached, true);
    await writeFile(first.output, Buffer.alloc(4));
    assert.equal((await prepareModel(file, 512, root)).cached, false);
    const plugin = modelCompressionPlugin();
    assert.equal(plugin.apply, 'build');
    plugin.configResolved({root, logger: {info() {}}}); plugin.buildStart();
    const assets = [];
    const context = {emitFile(asset) { assets.push(asset); return `ref${assets.length}`; }};
    assert.equal(await plugin.load.call(context, file), null);
    assert.match(await plugin.load.call(context, `${file}?url`), /ROLLUP_FILE_URL/);
    assert.ok(assets[0].source.length < source.length / 2);
    await plugin.generateBundle.call(context);
    assert.equal(JSON.parse(assets[1].source).modelCount, 1);
    assert.deepEqual(await readFile(file), source);
  } finally {
    const target = await realpath(root);
    if (!target.startsWith(resolve(tmpdir()) + sep)) throw new Error('临时测试目录越界');
    await rm(target, {recursive: true, force: true});
  }
});
