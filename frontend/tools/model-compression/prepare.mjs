import {createHash} from 'node:crypto';
import {mkdir, readFile, readdir, rename, writeFile} from 'node:fs/promises';
import {basename, dirname, resolve} from 'node:path';
import {fileURLToPath, pathToFileURL} from 'node:url';
import {optimizeGlb} from './glb.mjs';

// 职责：准备发布版模型与增量缓存、汇总体积；使用：Vite 构建插件及对比命令。
// 边界：只向 .cache 写入，不覆盖 src/data/models；缓存按源内容、策略版本和分辨率失效。
export const FRONTEND_ROOT = fileURLToPath(new URL('../../', import.meta.url));
export const STRATEGY_VERSION = 'ktx2-mip-trim-v2';
export const DEFAULT_MAX_SIZE = 512;
export const COMPRESSION_MODES = Object.freeze({
  quality: {label: '高画质', maxSize: 1024},
  balanced: {label: '均衡', maxSize: 512},
  compact: {label: '轻量', maxSize: 256},
});
const hash = buffer => createHash('sha256').update(buffer).digest('hex');
const mib = bytes => (bytes / 1048576).toFixed(2);

export function resolveCompressionMode(value = process.env.MODEL_COMPRESSION_MODE ?? 'balanced') {
  const mode = String(value).toLowerCase();
  if (!Object.hasOwn(COMPRESSION_MODES, mode)) throw new Error('MODEL_COMPRESSION_MODE 仅支持 quality、balanced、compact');
  return {mode, ...COMPRESSION_MODES[mode]};
}

export function resolveMaxSize(value) {
  // 发布脚本明确选择的模式优先于终端残留的数值覆盖，直接构建仍支持数值选项。
  const configured = process.env.MODEL_COMPRESSION_MODE ? resolveCompressionMode().maxSize : process.env.MODEL_TEXTURE_MAX_SIZE ?? DEFAULT_MAX_SIZE;
  const size = Number(value ?? configured);
  if (![256, 512, 1024, 2048, 4096].includes(size)) throw new Error('MODEL_TEXTURE_MAX_SIZE 仅支持 256、512、1024、2048、4096');
  return size;
}

async function atomicWrite(path, data) {
  await mkdir(dirname(path), {recursive: true});
  const temporary = `${path}.${process.pid}.tmp`;
  await writeFile(temporary, data);
  await rename(temporary, path);
}

export async function prepareModel(source, maxSize, root = FRONTEND_ROOT) {
  const input = await readFile(source);
  const sourceHash = hash(input);
  const key = `${STRATEGY_VERSION}:${maxSize}:${sourceHash}`;
  const output = resolve(root, '.cache/models', String(maxSize), basename(source));
  const metadataPath = `${output}.json`;
  try {
    const metadata = JSON.parse(await readFile(metadataPath, 'utf8'));
    const cached = await readFile(output);
    if (metadata.key === key && metadata.afterBytes === cached.length && metadata.outputHash === hash(cached)) {
      return {...metadata, output, cached: true};
    }
  } catch { /* 缓存缺失或不完整时从源文件重新生成。 */ }
  const result = optimizeGlb(input, maxSize);
  const metadata = {key, name: basename(source), sourceHash, outputHash: hash(result.buffer),
    beforeBytes: input.length, afterBytes: result.buffer.length, textures: result.textures};
  await atomicWrite(output, result.buffer);
  await atomicWrite(metadataPath, JSON.stringify(metadata, null, 2));
  return {...metadata, output, cached: false};
}

export function createReport(models, maxSize) {
  const beforeBytes = models.reduce((sum, model) => sum + model.beforeBytes, 0);
  const afterBytes = models.reduce((sum, model) => sum + model.afterBytes, 0);
  const mode = Object.entries(COMPRESSION_MODES).find(([, profile]) => profile.maxSize === maxSize)?.[0] ?? 'custom';
  return {strategy: STRATEGY_VERSION, compressionMode: mode, maxTextureSize: maxSize, modelCount: models.length, beforeBytes, afterBytes,
    reductionPercent: beforeBytes ? Number(((1 - afterBytes / beforeBytes) * 100).toFixed(2)) : 0,
    models: models.map(({name, sourceHash, outputHash, beforeBytes, afterBytes, textures}) => ({name, sourceHash, outputHash, beforeBytes, afterBytes, textures}))};
}

export async function prepareDirectory(maxSize, root = FRONTEND_ROOT) {
  const sourceDir = resolve(root, 'src/data/models');
  const names = (await readdir(sourceDir)).filter(name => name.endsWith('.glb')).sort();
  const models = [];
  for (const name of names) models.push(await prepareModel(resolve(sourceDir, name), maxSize, root));
  const report = createReport(models, maxSize);
  await atomicWrite(resolve(root, '.cache/models', String(maxSize), 'report.json'), JSON.stringify(report, null, 2));
  console.log(`[models] ${maxSize}px：${models.length} 个模型，${mib(report.beforeBytes)} → ${mib(report.afterBytes)} MiB，减少 ${report.reductionPercent}%（缓存 ${models.filter(model => model.cached).length}）`);
  return report;
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  const modeArgument = process.argv.indexOf('--mode');
  if (modeArgument >= 0) process.env.MODEL_COMPRESSION_MODE = resolveCompressionMode(process.argv[modeArgument + 1] ?? '').mode;
  const sizes = process.argv.includes('--compare') ? Object.values(COMPRESSION_MODES).map(profile => profile.maxSize) : [resolveMaxSize()];
  for (const size of sizes) await prepareDirectory(size);
}
