# @dyson/algorithm-viz

蓝图生成算法可视化渲染包（three.js）。构图借鉴《纪念碑谷》：正交等距视角、浮空平台、几何化的三面着色与机关式动画；环境采用《戴森球计划》的太空科幻风格：深空背景、青色建造网格；建筑使用游戏原生模型与材质颜色（灰白机身加涂装色，与游戏预览器同一套着色公式），生成中的临时建筑显示为蓝色全息投影。

平台由 4×4 格的方块拼成，只保留建筑下方这一层：场景范围扩大时，新方块从下方雾中依次升起；范围缩小时延迟约 1.2 秒确认后再沉回，避免算法搜索候选时反复抖动。

对外只暴露一个创建函数和一个控制器，渲染细节全部封装在内部。公开类型不引用 three，使用方无需安装 `@types/three`。

## 快速开始

```ts
import { createAlgorithmViz, ALGORITHM_VIZ_MODEL_KEYS } from '@/packages/algorithm-viz/src';
import basisJs from 'three/examples/jsm/libs/basis/basis_transcoder.js?url';
import basisWasm from 'three/examples/jsm/libs/basis/basis_transcoder.wasm?url';

const viz = createAlgorithmViz(el, {
  theme: 'dark',
  modelUrls: { 'assembler-1': '/models/assembler-1.glb' /* ... */ },
  transcoderUrls: { js: basisJs, wasm: basisWasm },
  describe: (b) => `物品 ${b.itemId}`,
});

viz.enqueue({
  coordinates: 'layout',
  stage: '布局装配',
  operations: [
    { type: 'add', id: 'm-1', building: { itemId: 2303, x: 0, y: 0, z: 0, yaw: 0, group: 0, temporary: true } },
  ],
});
viz.enqueue({ operations: [{ type: 'replace', id: 'm-1', building: { itemId: 2303, x: 4, y: 0, z: 0, group: 0 } }] });
viz.enqueue({ operations: [{ type: 'remove', id: 'm-1' }] });

viz.dispose(); // 卸载时必须调用
```

- 容器需要有确定的宽高；画布与覆盖层插入容器内，容器会被加上 `algoviz-root` 类
- 依赖 `three`（peer，0.160 ~ 0.189）与 `three/examples/jsm` 的 GLTFLoader

## 场景操作

| 操作 | 表现 |
| --- | --- |
| `add` | 建筑从上方落下并回弹落位，地面泛起方形涟漪；传送带沿运输方向生长 |
| `replace` | 同一对象抬起、平移转向、落下；由临时转为正式时从全息体实体化并闪光；更换模型时旧建筑沉入地面、新建筑重新落位 |
| `remove` | 建筑轻微抬起后沉入地面并溶解 |

操作按入队顺序播放，批次的 `pace` 决定节奏：

- `'replay'`（缺省）：回放已有结果。1 倍速约每秒 26 个操作；积压时自动加速，短队列约 8 秒、超过 800 / 3000 项时约 5 / 3 秒内播完，同时缩短单个动画
- `'follow'`：跟随正在运行的算法。画面最多落后约 1 秒：积压超过约 160 项时，按原顺序立即应用较早的操作（跳过过渡，不改变新增、替换、撤销的先后），只为最新的变化播放动画；出队按真实耗时计算，低帧率下也能跟上
- `'cinematic'`：候选演示使用的独立视觉时钟，1 倍速每个平台约每秒 60 项操作，积压不会自动加速或跳过旧过渡。计算结束后可以继续播放；各候选共享模型资源，但独立推进队列。

### 多候选平台

先按后端候选计划调用 `beginCandidate(id, ordinal)`，所有实际候选均完整铺开，不按平台数量截断建筑。未执行的条件回退组不创建假候选；启用新候选组时追加平台。每个候选的对象与连接 ID 均须带任务和候选前缀，避免跨平台串线。

```ts
viz.beginCandidate('candidate-1', 1);
viz.enqueue({candidateId: 'candidate-1', pace: 'cinematic', operations});
viz.finishCandidate('candidate-1', 'rejected');
```

`finishCandidate` 记录计算结果，不越过待播放过程。失败或剪枝候选在队列与建筑动画播放完毕后独立坠落；首次非空画面至少保留约 1.2 个视觉秒。可行候选暂留等待最终结果。

最终快照必须先完整组装并校验，再以 `enqueue({...batch, candidateId, snapshot: true})` 提交。快照等待该候选此前的操作和动画结束，再一次性更新全部正式姿态。只有正式生成调用成功且获选身份一致后，才调用 `finalizeSearch(winnerId)`；各落选平台分别退场，最终获选平台平滑归中。`failSearch()` 让本任务所有平台按各自时间线退场，不伪造获选方案。

包只管理视觉队列。应用层的 v4 事件订阅在队列积压约 20000 项时通过后端过程日志延迟消费，保留真实历史；日志最近两次任务、每任务最多 64 MiB、有效期十分钟，日志不足时明确提示并降级到正式结果。暂停不阻塞后端计算，`flush()` 可跳过已入队动画。

`flush()` 跳过动画直接应用全部待播操作。

批次的 `stage` 与上一批不同时，播放到该批会显示阶段标题；`caption(title, detail)` 可随时显示标题。

### 坐标

与蓝图一致：`x` 向东、`y` 向北，单位为蓝图格；`z` 为高度层（约 1.333 米）；`yaw` 为度，0 指北、顺时针。

- `coordinates: 'layout'`：生成过程中的平面装配格，直接使用 `x/y`
- `coordinates: 'area'`：正式蓝图的区域局部坐标，配合 `areas` 把子区域锚点沿父链累加后拼成整体平面

分拣器使用首端 `x/y/z` 与次端 `x2/y2/z2` 拉伸；传送带通过 `outputId` 连向下游传送带，链首自动补半格。

## 控制器接口

| 方法 | 说明 |
| --- | --- |
| `supported` | WebGL 是否可用；不可用时显示降级提示，其余方法安全空转 |
| `enqueue(batch)` | 入队一批操作 |
| `beginCandidate(id, ordinal)` | 创建候选完整平台，已存在时幂等 |
| `finishCandidate(id, status)` | 标记 valid / rejected / pruned，等待本候选播放完成 |
| `finalizeSearch(winnerId)` | 正式结果确认后确定唯一获选平台 |
| `failSearch()` | 无获选结果时让所有候选分别退场 |
| `caption(title, detail?)` | 显示阶段标题 |
| `reset({ animate? })` | 清空场景与队列；`animate` 时由内向外依次撤下 |
| `flush()` | 跳过动画，立即应用全部待播操作 |
| `setSpeed(n)` / `getSpeed()` | 播放速度 0.25 ~ 8 |
| `setPaused(b)` / `isPaused()` | 暂停出队与动画，视角仍可操作 |
| `setTheme(theme, immediate?)` / `getTheme()` | `'dark' \| 'light'`，默认约 450ms 渐变 |
| `rotate(steps)` | 以 90° 为步进旋转视角 |
| `resetView()` | 恢复自动取景 |
| `select(id \| null)` / `getProgress()` | 选中建筑 / 读取队列与累计统计 |
| `dispose()` | 释放全部资源 |

## 配置项

```ts
interface AlgoVizOptions {
  theme?: 'light' | 'dark';                  // 默认 'light'
  quality?: 'low' | 'mid' | 'high';          // 像素比上限 1 / 1.5 / 2
  speed?: number;                            // 默认 1
  modelUrls?: Record<string, string>;        // 模型键 → GLB 地址，键见 ALGORITHM_VIZ_MODEL_KEYS
  transcoderUrls?: { js: string; wasm: string }; // Basis 转码器地址；构建工具重命名资源时必须提供
  maxObjects?: number;                       // 单平台回放默认 12000；多候选不截断完整建筑
  emptyText?: string;                        // 空场景提示
  viewPaddingTop?: number;                   // 顶部被外部工具栏覆盖的像素，取景时让出
  injectStyles?: boolean;                    // 默认 true；false 时自行插入 ALGORITHM_VIZ_CSS
  describe?: (b: AlgoVizBuilding) => string; // 悬停提示文案
  onSelect?: (s: AlgoVizSelection | null) => void;
  onProgress?: (p: AlgoVizProgress) => void; // 最多每 100ms 一次
  onStats?: (s: AlgoVizStats) => void;
}
```

## 模型

模型键即 GLB 文件名去掉扩展名（如 `assembler-1`）。`resolveModelKey(itemId, modelIndex)` 按蓝图模型序号解析，缺省按物品 ID 推断（与生成器 `models.json` 一致）。

- 原生配色：底色、自发光、ORM、法线贴图为 KTX2/Basis 格式，经 KTX2Loader 解码；材质颜色（render/native-colors.ts，线性值，由游戏材质数据按材质名生成）只染涂装遮罩（data/models/masks，经 `maskUrls` 传入）标出的区域，其余保持贴图自带的白灰色；ORM 决定遮蔽 / 粗糙 / 金属度，法线贴图补表面凹凸；依赖供电的建筑自发光关闭，未通电时隐藏的特效面片（HIDDEN_PARTS）不绘制；玻璃罩半透明、特效面片加法叠加
- 同一模型的网格按材质分组合并为一个几何体，按模型实例化渲染（多材质模型使用材质数组）；米换算为蓝图格
- 传送带 GLB 为空节点，由场景程序化绘制：暗色履带上按 Mk.I 琥珀、Mk.II 绿、Mk.III 蓝流动箭头，箭头尖端朝向下游；未提供地址、加载中或失败的模型以几何体占位，加载完成后自动替换

## 交互

- 左键拖动：旋转视角，松手吸附到最近的 90° 档位；方向键 ← → 同样旋转
- 右键或 Shift + 左键拖动：平移；滚轮：以指针为锚点缩放；双击空白：复位视角
- 点击建筑：选中并回调 `onSelect`；Esc 取消选中
- `prefers-reduced-motion`：关闭下落与流动动画，操作直接落定
- 容器不可见或页面隐藏时暂停渲染

## 目录

```
src/
  index.ts          公开导出
  create.ts         createAlgorithmViz
  types.ts          公开类型
  catalog.ts        物品 / 模型序号 → 模型键、类别与占位尺寸
  defaults.ts       单位换算、速度范围、画质档位
  styles.ts         覆盖层样式与注入
  scene.ts          场景实现（队列节奏、对象动画、实例写入、交互、渲染循环）
  mock.ts           演示数据 createDemoBatches
  internal/         调色板、等距相机、操作队列、对象姿态、覆盖层、数学工具
  render/           着色器、模型库（KTX2 贴图）、实例池、方块平台、天空背景、投影与涟漪
```

## 脚本

```bash
npm run typecheck   # 严格模式类型检查（tsc -p tsconfig.json --noEmit）
```
