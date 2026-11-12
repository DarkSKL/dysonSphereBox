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

收到实际 `candidate-start` 时调用 `beginCandidate(id, ordinal)`；候选计划只声明身份，未派发的候选不占平台或建筑资源。活动候选不按平台数量截断建筑。每个候选的对象与连接 ID 均须带任务和候选前缀，避免跨平台串线。

```ts
viz.beginCandidate('candidate-1', 1);
viz.enqueue({candidateId: 'candidate-1', pace: 'cinematic', operations});
viz.finishCandidate('candidate-1', 'rejected');
```

`finishCandidate` 收到失败或剪枝结果后立即清空该候选待播队列及快照，启动下坠并按帧预算释放已显示对象；暂停播放也不阻止这项回收。同一任务内忽略该候选的迟到入队，防止复活。可行候选保留完整队列并暂留等待最终结果。

最终快照必须先完整组装并校验，再以 `enqueue({...batch, candidateId, snapshot: true})` 提交。快照等待该候选此前的操作和动画结束，再一次性更新全部正式姿态。只有正式生成调用成功且获选身份一致后，才调用 `finalizeSearch(winnerId)`；各落选平台立即丢弃待播内容并退场，最终获选平台平滑归中。`failSearch()` 立即清空本任务所有候选的待播内容并退场，不伪造获选方案。

包只管理视觉队列。应用层的 v4 事件订阅在队列积压约 20000 项时通过后端过程日志延迟消费，保留真实历史；日志最近两次任务、每任务最多 64 MiB、有效期十分钟，日志不足时明确提示并降级到正式结果。暂停不阻塞后端计算，`flush()` 可跳过已入队动画。

`flush()` 跳过动画，按共享帧预算持续应用全部待播操作。积压期间减少中间画面的GPU重绘，数据处理结束立即绘制完整结果。

批次的 `stage` 与上一批不同时，播放到该批会显示阶段标题；`caption(title, detail)` 可随时显示标题。

### 坐标

与蓝图一致：`x` 向东、`y` 向北，单位为蓝图格；`z` 为高度层（约 1.333 米）；`yaw` 为度，0 指北、顺时针。

- `coordinates: 'layout'`：生成过程中的平面装配格，直接使用 `x/y`
- `coordinates: 'area'`：正式蓝图的区域局部坐标，配合 `areas` 把子区域锚点沿父链累加后拼成整体平面

分拣器使用首端 `x/y/z` 与次端 `x2/y2/z2` 拉伸；传送带通过 `outputId` 连向下游传送带，链首自动补半格。相邻带段共享斜接截面，使水平转弯、斜坡与垂直升降保持闭合；流动箭头沿路径累计相位，支持 1.5 格等非整数长度，闭环自动匹配整数个箭头周期。

## 控制器接口

| 方法 | 说明 |
| --- | --- |
| `supported` | WebGL 是否可用；不可用时显示降级提示，其余方法安全空转 |
| `enqueue(batch)` | 入队一批操作 |
| `beginCandidate(id, ordinal)` | 创建候选完整平台，已存在时幂等 |
| `finishCandidate(id, status)` | 保留 valid 待播内容；rejected / pruned 立即清队列并退场 |
| `finalizeSearch(winnerId)` | 正式结果确认后确定唯一获选平台 |
| `failSearch()` | 无获选结果时让所有候选分别退场 |
| `caption(title, detail?)` | 显示阶段标题 |
| `reset({ animate? })` | 清空场景与队列；`animate` 时由内向外依次撤下 |
| `flush()` | 跳过动画，按帧预算持续处理至完整最终状态 |
| `setSpeed(n)` / `getSpeed()` | 播放速度 0.25 ~ 8 |
| `setPaused(b)` / `isPaused()` | 暂停出队与动画，视角仍可操作 |
| `setTheme(theme, immediate?)` / `getTheme()` | `'dark' \| 'light'`，默认约 450ms 渐变 |
| `rotate(steps)` | 以 90° 为步进旋转视角 |
| `resetView()` | 恢复自动取景 |
| `select(id \| null)` / `getProgress()` | 选中建筑 / 读取队列与累计统计 |
| `setIconsVisible(group, b)` / `getIconsVisible()` | 显示或隐藏世界图标，`group` 为 `'belt'`（传送带入口）或 `'building'`（机器建筑）；只切换可见性，不影响对象与播放 |
| `refreshIcons()` | 重新解析全部对象的图标；精灵图、物品数据迟到后调用 |
| `dispose()` | 释放全部资源 |

## 配置项

```ts
interface AlgoVizOptions {
  theme?: 'light' | 'dark';                  // 默认 'light'
  quality?: 'low' | 'mid' | 'high';          // 像素比上限 1 / 1.5 / 2
  speed?: number;                            // 默认 1
  modelUrls?: Record<string, string>;        // 模型键 → GLB 地址，键见 ALGORITHM_VIZ_MODEL_KEYS
  transcoderUrls?: { js: string; wasm: string }; // Basis 转码器地址；构建工具重命名资源时必须提供
  maxObjects?: number;                       // 默认无限制；只有显式传入时才限制单平台对象数
  emptyText?: string;                        // 空场景提示
  viewPaddingTop?: number;                   // 顶部被外部工具栏覆盖的像素，取景时让出
  injectStyles?: boolean;                    // 默认 true；false 时自行插入 ALGORITHM_VIZ_CSS
  describe?: (b: AlgoVizBuilding) => string; // 悬停提示文案
  iconSheet?: { url: string; width: number; height: number; cell: number }; // 图标精灵图；缺省时不绘制图标
  icon?: (b: AlgoVizBuilding) => { x: number; y: number } | null;          // 解析建筑的图标格，null 为没有
  iconsVisible?: { belt?: boolean; building?: boolean };                    // 初始显示开关，默认都显示
  onSelect?: (s: AlgoVizSelection | null) => void;
  onProgress?: (p: AlgoVizProgress) => void; // 最多每 100ms 一次
  onStats?: (s: AlgoVizStats) => void;
}
```

## 世界图标

传送带入口与机器建筑上方可以显示物品图标：圆角底板 + 指向建筑的小尾巴 + 精灵图中的一格，正对屏幕，边长夹在 12 ~ 40 屏幕像素之间，远景里自然边长过小的图标淡出。全部图标共用一个实例池和一次绘制。

- 数据：`AlgoVizBuilding.iconItemId`（带标记物品，用于原料入口与成品出口的带节点）和 `recipeId`（机器配方）只是原始数据，包本身不认识物品名；由使用方的 `icon(building)` 解析为精灵图中的格坐标，返回 `null` 表示没有图标
- 精灵图：`iconSheet` 给出整图像素尺寸与单格边长，格按行列紧密排布、原点在左上角；缺省或加载失败时图标层保持为空，其余功能不受影响
- 开关：`setIconsVisible('belt' | 'building', visible)` 只切换可见性，槽位随写入阶段分配或释放，对象、动画与队列都不受影响；外部数据迟到时调用 `refreshIcons()`
- 随对象：图标跟随入场下落、平移抬升和候选平台的偏移，失败候选一并标红，坠落候选随深度淡出

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
  internal/         调色板、等距相机、操作队列、对象姿态、覆盖层、数学工具、图标槽位簿记
  render/           着色器、模型库（KTX2 贴图）、实例池、方块平台、天空背景、投影与涟漪、世界图标
```

## 脚本

```bash
npm run typecheck   # 严格模式类型检查（tsc -p tsconfig.json --noEmit）
npm run test:belts  # 传送带接缝和箭头相位回归检查，需 Node.js 22.6+ 的类型擦除支持
npm run test:icons  # 图标槽位压实与精灵图 UV 换算回归检查，同样需要类型擦除支持
```
