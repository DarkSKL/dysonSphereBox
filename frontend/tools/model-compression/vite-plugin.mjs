import {readFile} from 'node:fs/promises';
import {basename, resolve, sep} from 'node:path';
import {createReport, prepareModel, resolveMaxSize} from './prepare.mjs';

// 职责：只在生产构建中替换模型资产，压缩完成后再交给 Rollup 计算内容哈希。
// 边界：开发服务器不启用插件；仅处理模型目录中的 GLB URL 导入，不改变组件的路径与模型键。
export function modelCompressionPlugin() {
  let root;
  let modelRoot;
  let maxSize;
  let logger;
  const pending = new Map();
  return {
    name: 'dsbox-model-compression',
    apply: 'build',
    enforce: 'pre',
    configResolved(config) {
      root = config.root;
      modelRoot = resolve(root, 'src/data/models');
      maxSize = resolveMaxSize();
      logger = config.logger;
    },
    buildStart() { pending.clear(); },
    async load(id) {
      const [file, query = ''] = id.split('?');
      if (!file.startsWith(modelRoot + sep) && !file.startsWith(modelRoot.replaceAll('\\', '/') + '/')) return null;
      if (!file.endsWith('.glb') || !new URLSearchParams(query).has('url')) return null;
      if (!pending.has(file)) pending.set(file, prepareModel(file, maxSize, root));
      const model = await pending.get(file);
      const reference = this.emitFile({type: 'asset', name: basename(file), source: await readFile(model.output)});
      return `export default import.meta.ROLLUP_FILE_URL_${reference};`;
    },
    async generateBundle() {
      const models = await Promise.all(pending.values());
      const report = createReport(models, maxSize);
      this.emitFile({type: 'asset', fileName: 'model-compression-report.json', source: JSON.stringify(report, null, 2)});
      logger.info(`[models] ${models.length} 个发布模型：${(report.beforeBytes / 1048576).toFixed(2)} → ${(report.afterBytes / 1048576).toFixed(2)} MiB，减少 ${report.reductionPercent}%`);
    },
  };
}
