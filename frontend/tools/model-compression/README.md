# 模型构建压缩与画质对比

开发服务器读取 `src/data/models` 中的原始 GLB。`npm run build` 的 Vite 构建插件会自动为实际导入的模型准备压缩副本，再生成资源哈希；Wails 的既有前端构建命令会沿用此流程。

## 压缩方式

当前模型的内嵌 KTX2 贴图已包含完整 mip 金字塔。发布版移除超出目标分辨率的高精度层，默认上限为 512；保留层的 ETC1S/UASTC 压缩字节、色彩描述和码本保持不变，同时重建 KTX2 与 GLB 的索引和偏移。网格、材质、节点与模型尺寸不变，无需额外安装纹理编码器。

缩小后边长不满足 `KHR_texture_basisu` 四倍数约束的特殊贴图保留原样，并在报告的 `preservedReason` 中说明。缺失所需 mip、数据越界或不支持的需裁剪纹理会使构建失败。

## 命令

在 `frontend` 目录运行：

```powershell
npm run models:compare
npm run dev
```

打开开发服务器的 `/tools/model-compression/preview.html`，并排比较原版、1024 和 512 三档。页面复用真实算法渲染包、原生材质配色、KTX2 加载器与涂装遮罩，支持建筑切换、同步视角、放大检查和日夜主题。此页是独立开发工具，不进入应用路由或生产入口。

```powershell
npm run models:prepare
npm run build
```

- `models:prepare`：准备当前档位的全部模型，便于预热缓存。
- `models:compare`：准备全部模型的 1024、512、256 三档，以及各自的体积报告；对比页展示原版、1024 和 512。
- `build`：仅处理构建实际导入的 GLB，输出压缩资产与 `dist/model-compression-report.json`。

发布脚本 `deploy/build.bat` 提供三档菜单，回车选择中间的均衡档；也可直接运行 `deploy\build.bat v2.1.2 quality`、`balanced` 或 `compact`。模式通过 `MODEL_COMPRESSION_MODE` 传递，并优先于终端中残留的数值覆盖。

直接构建前端时，也可在 PowerShell 中选择模式：

```powershell
$env:MODEL_COMPRESSION_MODE = 'quality'
npm run build
$env:MODEL_COMPRESSION_MODE = 'balanced'
```

`quality`、`balanced`、`compact` 分别对应 1024、512、256。也支持 `npm run models:prepare -- --mode quality`。未设置模式时，高级数值选项 `MODEL_TEXTURE_MAX_SIZE` 支持 256、512、1024、2048、4096。数值表示最大边长，保持原图宽高比；使用现成 mip 时，实际尺寸可能小于上限。

## 缓存与验证边界

压缩副本及报告位于被 Git 忽略的 `frontend/.cache/models/<档位>/`。缓存按源文件 SHA-256、压缩策略版本和分辨率失效；输出哈希也会验证，损坏缓存自动重建。同一模型与档位复用固定缓存位置，不持续积累带哈希的旧文件。

构建报告区分单文件压缩前后体积，保存源文件和产物哈希。完整目录报告包含全部 GLB；生产报告仅包含 Vite 实际导入的模型，两者总量不同。单个 GLB 仍是自包含资源，运行时无需解压归档或下载模型包。

类型检查和构建通过不等于画质验收；需要在对比页检查近景的法线、金属细节、玻璃和轮廓。保留完整原始模型，便于后续调整档位或重新生成发布副本。
