# 戴森球盒子

> 面向《戴森球计划》的 Windows 桌面工作台：管理游戏目录与 BepInEx、浏览和安装 Thunderstore 模组、纳管本地模组/蓝图/存档、运行 Nebula 无头服务器、计算产线并生成蓝图、离线查阅游戏图鉴，以及通过联机大厅和 P2P 组队。

戴森球盒子（DysonSphereBox）是 DarkSKL 为《戴森球计划》玩家开发的桌面工具，使用 Go + Wails v2 + Vue 3 构建。它把安装前置、配置游戏路径、安装模组、整理蓝图和存档、准备联机开服环境、计算产线这些容易出错的操作，收进一个中文、可审计、尽量可恢复的界面里。

应用以本地能力为主；账号、公告、联机大厅和模组简介翻译需要连接应用服务器。当前版本已经形成覆盖主要流程的工作台，但量化蓝图生成、Nebula 在线玩家读取等部分仍有明确边界，见 [已知限制](#已知限制)。

- 项目主页：<https://gitee.com/Dark-Skl/dyson-sphere-box>
- 更新日志：[CHANGELOG.md](CHANGELOG.md)

## 当前重点

- **量化与蓝图**：内置 12 套游戏数据集，支持多目标产线求解、配方/设备/喷涂策略、产线节点图和球面纬圈蓝图生成，也支持导入现有蓝图反算理论产出。
- **游戏图鉴**：建筑、物品、科技、战斗和成就离线查阅，支持搜索、分组筛选、配方与条目详情。
- **社区联机**：登录后浏览公开大厅、创建公开或私密房间，通过 libp2p 直连打洞或服务器授权的中继连接游戏，并提供连接诊断。
- **本地资源库**：统一纳管客户端模组、蓝图和真实存档，可与游戏目录按需同步，处理冲突、迁移和中断恢复。
- **工作台体验**：无边框窗口、戴森球首页、日夜主题、全局新手引导、公告通知、账号资料、系统日志和应用自更新。

## 功能说明

### 工作台与全局体验

- 左侧快捷栏切换“戴森球计划”和“社区”两个工作区，第二层导航对应各功能页；日志、关于和全局设置作为全局入口独立展开。
- 无边框桌面窗口，标题栏承载页面标题、版本号、更新提示、日夜主题切换以及最小化、最大化、关闭按钮。
- 首页使用 Three.js 渲染戴森球场景，建造进度按本地时间在 08:00 至 22:00 上升、22:00 至次日 08:00 回落，只作视觉展示。
- 日夜主题写入本地配置 `System/AppTheme`，切换时使用揭示动画；系统开启“减少动态效果”时直接切换。
- 基于 Driver.js 的全局新手引导覆盖工作区切换、导航层级、当前页面、教程入口、主题和窗口控制；完成或跳过状态按页面键与教程版本保存，可从帮助按钮重新观看。

### 游戏目录与 BepInEx

- 手动选择《戴森球计划》安装目录，或读取已保存配置；也可从 Steam 库、注册表和桌面/开始菜单快捷方式自动查找 `DSPGAME.exe`。
- 游戏目录归一化后保存到配置项 `Game/StartupFilePath`；只有能定位 `DSPGAME.exe` 的目录才会写入配置。
- 检查 `winhttp.dll`、`doorstop_config.ini`、`BepInEx/core` 等关键文件和 Doorstop 配置，读取本地 BepInEx 版本。
- 从 Thunderstore 查询并安装 BepInEx，可补装缺失文件；卸载会删除游戏目录中的 BepInEx 目录和 Doorstop 注入文件，其中的插件也会被一并移除，且不会清理模组市场的安装记录。
- BepInEx 的“更新”入口目前只保证前置环境可用：未安装时安装，已安装时返回跳过，不做版本升级。

### 模组市场与安装

- 从 Thunderstore 加载模组列表，支持关键字搜索、强制刷新、分页、封面、简介、依赖、版本和本地安装状态。
- 登录后可按需把可见的模组简介翻译为中文；翻译经应用服务器执行，失败时保留原文并允许重试。
- 安装会解析依赖并写入 BepInEx 插件目录（`plugins`/`patchers`），使用来源感知的包 key 关联安装状态和安装记录，让不同来源的同名包各自独立。
- 安装过程通过 `modDependence:installProgress` 推送进度，页面重进后可读取进度快照；支持取消安装、处理文件冲突（覆盖、跳过或失败）以及恢复进程中断的安装。
- 卸载按安装清单清理已跟踪文件，并考虑共享依赖归属；已安装包的 key 用于市场卡片状态匹配，资源页读取已安装包摘要。
- 来源选择保留“手动导入”入口，但远端市场数据尚未接入；本地模组请到“资源”页导入。

### 资源库

客户端资源库位于程序同级目录，便于便携使用；模组和蓝图的同步会保留文件记录，便于禁用、恢复和冲突检查。

- **模组**
  - 扫描 `appData/plugins` 中的本地模组，支持导入目录或 zip 包、删除，以及拖入 zip。
  - 左键启用/禁用：启用时复制到游戏 `BepInEx/plugins`，禁用时按同步记录移除目标文件；冲突会标记为待处理状态。
  - 市场安装的模组单独展示，由安装器管理；资源库不会和安装器争抢同一批文件。
  - “同步本机模组”可把游戏目录里已有、尚未纳管的模组登记到资源库。
- **蓝图**
  - 以目录树管理 `appData/blueprints`，支持新建文件夹、导入文件、粘贴蓝图编码快速导入、重命名、移动和删除。
  - 启用/禁用会把蓝图镜像到游戏真实蓝图目录；移动或重命名已启用蓝图前会预览对游戏侧文件的影响。
  - 支持面包屑导航、拖拽移动、导出到指定目录，以及使用内置 3D 查看器预览蓝图。
  - 建筑数不超过 10000 时自动打开预览；超过阈值时默认折叠，仍可手动打开，不限制复制和导出。
  - “同步本机蓝图”可纳管真实蓝图目录中已有的 `.txt`、`.bp` 文件。
- **存档**
  - 扫描真实存档目录并按修改时间排序，支持重命名、导入、导出和删除。
  - 删除存档会把 `.dsv` 及同目录同名 `.server` 附加数据移入 `appData/environment/snapshots`，不是直接永久删除。
  - 存档只索引真实目录，不复制进客户端资源库；导入和导出直接作用于真实存档文件。
- 目录解析会区分配置目录、自动探测、猜测和缺失状态；需要确认的目录不会执行写入，可在页面手动选择真实蓝图或存档目录。
- 首次同步可预览并纳管游戏目录已有内容；写操作记录操作日志，启动时会恢复或标记中断的操作。

### 开服

- 开服页检测游戏目录、BepInEx、Nebula 依赖、`steam_appid.txt`、HostPort、Nebula 配置、进程状态、TCP/UDP 监听和日志路径。
- “准备环境”会先确保 BepInEx 可用，再写入基础环境和 `steam_appid.txt`，并返回仍缺失的 Nebula 依赖；不会自动安装 Nebula。
- 保存 HostPort 与启动存档；HostPort 同步写入 `nebula.cfg`，存档名保存后参与下一次启动。
- 一键启动/停止 Nebula 无头服务器，展示进程 PID、运行时长和启动命令；停止只处理确认带无头参数的服务器进程，避免误关普通游戏。
- 展示上/下行速率、峰值和近期采样趋势，以及 BepInEx/Unity 日志控制台，支持等级、关键字筛选和日志导出。
- 启动存档输入支持从真实存档目录自动补全。
- “在线连接”分区目前只显示不可用状态；Nebula 后端尚未提供玩家列表、延迟和连接控制接口。

### 量化与蓝图

- 内置 12 套量化数据集：原版，以及 GenesisBook、MoreMegaStructure、TheyComeFromVoid、FractionateEverything 的组合版本；数据随程序打包，不联网加载。
- 量化支持多个目标物品与目标产量，选择配方、设备、喷涂模式和增产点数，设置外部供应、采矿倍率、小数位、占地/电力权重、研究站堆叠和增产剂自喷涂。
- 使用内置两阶段单纯形求解产线，展示每个节点的产出、设备数、功耗、输入输出和取整余量；参数变化后旧结果会标记为过期。
- 节点图与参数面板联动，可切换配方/设备、把中间产物改为外部供应或本地生产、把剩余副产物追加为目标。
- 支持实时计算与生成，也可暂停后手动触发；生成进度由后端实际阶段驱动，并可用请求标识隔离不同任务。
- 蓝图生成支持多种布局模式、平面与球面纬圈策略、相邻纬圈扩展、单列机器数、研究站层数、街区宽度、排列方式、供电设施、喷涂供剂网络和混带输入。
- 球面模式按纬圈容量规划完整产线，容量不足时可拆分为多个原生 Area；生成结果必须通过 Go 编码器自检才会返回，并可能附带运力、供料或容量告警。
- 多目标方案可按目标分区预览，未归属单一目标的产线归入“共用”分区。
- 支持复制蓝图编码、导出文本、导出方案 JSON、重置预览视角和存入蓝图库；存入后可选择立即启用并同步到游戏蓝图目录。
- “导入反算”严格解析现有蓝图，只读识别建筑、制造设备、未喷涂理论产出和未知原型，并可把识别结果转为量化目标重新计算。
- 大型蓝图通过分片读取交给预览器；预览使用项目内置的本地查看器，不依赖外部页面。
- 生成与预览不等于游戏内放置验收；球面锚点、碰撞、物流接驳和实际运力仍需在游戏中验证。

### 游戏图鉴

- 提供建筑、物品、科技、战斗和成就五个分类，支持全局搜索、分组筛选和条目详情。
- 建筑、物品、配方、科技和资源详情包含配方、产出、消耗、科技前置和说明；成就支持按名称、条件、编号和分组检索。
- 图鉴数据、中文文本和图标随程序打包，运行时从本地加载，无需联网。
- 数据集固定对应游戏版本 `0.10.29.21950`，不包含存档加成或个人科技状态；数据来源与授权说明见 [图鉴 NOTICE](frontend/src/data/encyclopedia/NOTICE.md)。

### 联机

- 联机属于“社区”工作区，需要先登录账号；登录后进入页面会自动连接大厅，连接期间保持在线并自动退避重连，不可重连的原因需要手动重试。
- 公开大厅支持按房间名搜索、按创建时间或人数排序、分页和增量刷新；只展示公开且未满员的房间。
- 创建房间支持公开/私密可见性、4–32 位可选密码、成员上限和端口声明；端口可手动填写 TCP、UDP 或两者，也可自动识别本机游戏端口。
- 可按房间或邀请码加入；房主可复制邀请码、查看成员链路状态、踢出成员、关闭房间，成员可主动离开。
- 当前房间展示游戏连接地址、端口、邀请码、成员座位、链路状态和平均延迟；控制链路离线时不会复制旧会话的地址。
- 数据面使用 libp2p 的 QUIC/TCP 双栈，优先尝试 DCUtR 直连打洞；服务器授权时使用中继兜底，并可为成员建立本地端口转发后进入游戏。
- 连接诊断展示本机上报地址、观测节点看到的地址、房主下发地址、AutoNAT 可达性、防火墙、UPnP、IPv6、中继政策和预约限额，并保留 TCP/QUIC 拨号与打洞证据。
- 联机层只负责传输与房间协调，不注册游戏协议；玩家仍需保证游戏版本、模组和房间配置一致。

### 账号与公告

- 标题栏头像打开账号弹窗，支持验证码、注册、登录、会话恢复、强制改密、资料编辑、头像上传、修改密码和退出登录。
- 资料编辑可修改昵称、头像、手机和邮箱；登录 token 只保存在 Go 后端，不返回前端。
- 全局设置可修改应用服务器地址，保存前可做只读连通性测试；地址变化会清除账号会话并要求重新登录。默认地址为 `https://dysonbox.cn:32888`。
- 登录后接收服务端公告：启动时展示最新未读公告，标题栏公告入口可查看历史、有效期和当前账号的首次查阅时间。
- 公告正文按 Markdown 或 HTML 渲染并做安全清洗；“已查阅”需要用户主动确认，关闭弹窗不产生已读副作用。
- 社区工作区、公告和模组简介翻译都依赖账号会话；离线或不登录仍可使用本地游戏环境、资源、量化、图鉴和日志功能。

### 系统日志与诊断

- 运行日志以 JSONL 写入程序同级 `logs/`，同时建立 SQLite 索引；启动时会迁移旧日志并回放文件重建索引。
- 日志页支持按关键字、级别、命名空间、触发者、来源、操作关联号和运行会话号筛选，查看详情和删除单条日志。
- 日志条目包含命名空间、行为、原因、等级、来源、触发者、操作关联号、会话和结构化 `data`，便于串联安装、配置、更新、资源同步和开服操作。
- 可查看日志健康状态、打开日志目录，或导出脱敏诊断 ZIP（`manifest.json` + `logs.jsonl`）；导出不包含账号凭据、存档或原始数据库。

### 应用更新

- 启动后和全局设置页可检查 Gitee Release 最新版本，标题栏在发现新版本时显示入口和发布说明。
- 更新会按当前平台和架构选择 `DysonSphereBox_windows_amd64.zip` 或 `DysonSphereBox_windows_arm64.zip`。
- 下载后使用 Release 附带的 `checksums.txt` 做 SHA256 校验，再替换当前可执行文件；写入成功后需要重启应用生效。
- 检查或更新失败不会破坏当前程序，页面会保留错误原因和处理建议。

## 页面说明

| 页面 | 路由 | 说明 |
| --- | --- | --- |
| 首页 | `/dysonSphere` | 戴森球 3D 场景、欢迎信息和按本地时间变化的建造进度。 |
| 开服 | `/dysonSphere/headless` | Nebula 环境检测、准备、端口与存档配置、启动停止、网络趋势和日志控制台。 |
| 模组 | `/dysonSphere/mod` | Thunderstore 模组列表、搜索、刷新、分页、安装、卸载和冲突处理。 |
| 资源 | `/dysonSphere/environment` | 客户端模组、蓝图和真实存档的纳管、同步、迁移、导入导出与预览。 |
| 量化与蓝图 | `/dysonSphere/blueprint-generator` | 多目标产线量化、节点图、蓝图生成/导入反算、3D 预览和蓝图库。 |
| 图鉴 | `/dysonSphere/encyclopedia` | 建筑、物品、科技、战斗和成就的离线查阅。 |
| 游戏设置 | `/dysonSphere/settings` | 游戏目录与 BepInEx 前置框架。 |
| 联机 | `/community/multiplayer` | 公开大厅、房间管理、P2P 连接和网络诊断；需要登录。 |
| 系统日志 | `/system/log` | 日志检索、详情、删除、健康状态和诊断导出。 |
| 全局设置 | `/system/settings` | 应用服务器地址与应用更新。 |
| 关于我们 | `/system/about` | 版本、项目地址、反馈入口、联系方式、鸣谢与免责声明。 |

路由使用 Hash 模式。访问 `/` 会跳转到 `/dysonSphere`，`/community` 会跳转到 `/community/multiplayer`；旧的 `/dysonSphere/multiplayer` 也重定向到联机页。

## 技术栈

- 桌面框架：Wails v2.12.0（无边框窗口、WebView2、文件拖放）
- 后端：Go 1.24.11、GORM、纯 Go SQLite 驱动、Gorilla WebSocket、libp2p、go-selfupdate
- 前端：Vue 3、TypeScript 5.4、Vite 3、Vue Router 4、`unplugin-vue-router`
- UI：Naive UI、Tailwind CSS、SCSS、Lucide、Tabler Icons、Iconify、Game Icons
- 可视化与交互：Three.js、ECharts 6、Vue Flow、Lottie、Driver.js
- 内容与安全：markdown-it、DOMPurify
- 目标平台：Windows 10/11，发布 `amd64` 与 `arm64`

## 环境要求

参与开发前，请安装：

- Windows 10/11
- Go 1.24 或更高版本（`go.mod` 当前为 1.24.11）
- Node.js 18 或更高版本（建议使用当前 LTS）
- npm
- Wails CLI v2（建议 v2.12.0）
- WebView2 Runtime

安装 Wails CLI：

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
```

检查开发环境：

```powershell
wails doctor
```

## 本地开发

克隆项目：

```powershell
git clone https://gitee.com/Dark-Skl/dyson-sphere-box.git
cd dyson-sphere-box
```

安装前端依赖：

```powershell
cd frontend
npm install
cd ..
```

启动桌面开发模式：

```powershell
wails dev
```

`wails dev` 会同时启动 Wails 桌面宿主和 Vite 前端服务，适合调试前后端绑定。只运行 `frontend` 下的 `npm run dev` 时只有 Vite 页面，没有桌面 Go 运行时和 Wails 绑定。

补充说明：

- 仓库虽然保留 pnpm 工作区文件，但 `wails.json` 使用 npm 执行安装和构建；请以 `frontend/package.json` 和 `package-lock.json` 为准。
- `main.go` 通过 `go:embed` 嵌入 `frontend/dist` 和 `app.db`。全新检出后直接执行 `go build ./...` 前，需要先完成一次前端构建，确保 `frontend/dist` 存在。
- 修改后端导出服务的方法或签名后，运行 `wails generate module` 重新生成绑定。
- `frontend/wailsjs/**`、`frontend/src/components.d.ts` 和 `frontend/src/typed-router.d.ts` 都是生成文件，不要手动编辑。

## 常用验证

前端构建（包含 `vue-tsc --noEmit` 类型检查）：

```powershell
cd frontend
npm run build
cd ..
```

Go 编译：

```powershell
go build ./...
```

修改绑定后重新生成前端模块：

```powershell
wails generate module
```

项目当前没有自动化测试套件或 CI 工作流；上述命令验证的是类型、编译和打包链路，不等同于桌面行为测试。涉及窗口、文件同步、联机或游戏目录的功能，仍需通过 `wails dev` 或打包程序手动验证。

## 发布构建

Windows 发布脚本会询问版本号，也可以在命令行直接传入：

```powershell
deploy\build.bat
deploy\build.bat v2.0.1
```

脚本会：

- 校验并规范化版本号为 `vX.Y.Z`。
- 通过 `-ldflags "-X DysonSphereBox/core.Version=<版本>"` 注入应用版本。
- 使用 `wails build -clean` 构建 `windows/amd64` 和 `windows/arm64`。
- 生成 `DysonSphereBox_windows_amd64.zip`、`DysonSphereBox_windows_arm64.zip`，压缩包内只包含 `DysonSphereBox.exe`。
- 生成包含两个 zip 的 `checksums.txt`。

产物位于 `deploy/release/<版本号>/`。发布到 Gitee Release 时上传两个 zip 包和 `checksums.txt`。应用自更新依赖这些附件名称和 SHA256 校验清单，修改名称需要同步更新更新器配置。

`deploy/release` 属于本地产物并被 Git 忽略，`deploy/build.bat` 会随仓库保留。发布构建需要可用的 Wails CLI。

## 运行数据与目录

应用采用便携式数据布局，实时数据放在可执行文件旁边，不使用 Windows `%APPDATA%`：

| 路径 | 内容 |
| --- | --- |
| `<exe>/dyson-sphere-box.db` | 实时 SQLite 数据库；首次启动时从内嵌模板创建。 |
| `<exe>/logs/` | JSONL 运行日志和会话标记文件。 |
| `<exe>/appData/plugins/` | 客户端模组资源库。 |
| `<exe>/appData/blueprints/` | 客户端蓝图资源库。 |
| `<exe>/appData/environment/` | 导入临时目录、存档快照等环境管理数据。 |
| `<游戏目录>/BepInEx/plugins/` | 启用模组的同步目标。 |
| 真实蓝图目录、真实存档目录 | 由配置或自动探测解析，蓝图启用、存档导入导出直接作用于这些目录。 |

仓库根目录的 `app.db` 是首次启动使用的**模板数据库**，不是运行期数据库。开发模式下，实际数据库和日志位于构建出的可执行文件旁边。

## 项目结构

```text
DysonSphereBox
├── app.go                    # Wails 生命周期、启动恢复、窗口级对话框
├── main.go                   # Wails 启动入口，嵌入前端产物和模板数据库
├── app.db                    # SQLite 模板数据库
├── core/                     # 数据库、配置、HTTP 客户端、运行时上下文、版本
├── constant/                 # 日志、安装状态、联机协议、服务地址等稳定常量
├── service/                  # Wails 绑定服务
│   ├── system/               # 账号、公告、配置、环境目录、日志、服务器、教程、更新
│   ├── modDependence/        # BepInEx、开服、模组市场、安装器
│   ├── environment/          # 本地模组/蓝图/存档资源库
│   ├── factory/              # 量化、蓝图编解码/生成/导出
│   └── multiplayer/          # 联机控制链路、房间和状态投影
├── helper/                   # 领域实现
│   ├── modDependence/        # BepInEx、Nebula、市场、V2 安装器、翻译
│   ├── environment/          # 目录解析、扫描、镜像同步、蓝图解析
│   ├── quant/                # 内置数据集、名称图、成本模型、单纯形
│   ├── blueprint/            # 蓝图解析、编码、布局与球面生成
│   ├── factory/              # 量化/生成服务编排、进度、导出
│   ├── multiplayer/          # libp2p host、DCUtR、中继、端口转发
│   ├── system/               # 账号、公告、日志索引、服务器探测
│   └── updater/              # Gitee Release、校验与自更新
├── model/                    # GORM 模型与迁移注册
├── apiDefinition/            # 外部接口路径与请求/响应契约
├── utils/                    # 通用路径、Steam、快捷方式和游戏目录工具
├── scripts/                  # 图鉴数据导入/校验、碰撞体数据提取
├── docs/                     # 设计与实现说明
├── deploy/build.bat          # Windows 发布构建脚本
├── build/                    # Wails 构建资源
└── frontend/
    ├── public/               # 图鉴图标、Game Icons 许可文本
    ├── src/
    │   ├── pages/            # 文件路由页面
    │   ├── components/       # 可复用组件（按领域分目录）
    │   ├── composables/      # 页面状态与流程
    │   ├── services/         # Wails 调用封装
    │   ├── navigation/       # 工作区与导航注册表
    │   ├── tutorials/        # Driver.js 教程定义和选择器
    │   ├── data/             # 随程序打包的图鉴数据
    │   ├── packages/         # 内置蓝图预览/编辑库
    │   ├── layouts/          # main / blank 布局
    │   └── types/            # 共享 TypeScript 类型
    └── wailsjs/              # Wails 生成的前端绑定
```

## 架构与开发约定

调用链：

```text
Vue 页面/组合式函数 -> Wails 生成绑定 -> service 服务方法
                    -> helper 领域实现 -> core / 文件系统 / SQLite / HTTP / 进程
Go 进度与状态事件 -> Wails runtime 监听 -> Vue 状态
```

- 后端业务入口放在 `service/<domain>`，可复用实现放在 `helper/<domain>`；`utils` 只放低依赖工具，不反向引用业务包。
- 新增服务需在对应领域的 `enter.go` 注册，并由根 `service.RegisterServices()` 聚合；构造函数不得依赖已初始化的数据库。
- 新增持久化模型需在对应领域注册，并由 `model.RegisterModels()` 聚合，否则不会进入 `AutoMigrate`。
- 外部请求路径和请求/响应契约放在 `apiDefinition/`；动态路径占位符使用 `utils.ResolveAPIPath` 解析。
- 重要配置、安装、更新、资源同步、开服和账号操作应写入统一日志，使用命名空间、等级、来源、触发者、操作关联号和结构化 `data`，不要把流程细节塞进新的日志列。
- 前端页面放在 `frontend/src/pages`，通过 `definePage({ meta: { layout, title, keepAlive } })` 声明布局、标题和缓存；布局放在 `layouts`，可复用组件放在 `components`。
- 页面协调与状态优先放在 `composables`，Wails 调用封装放在 `services`；页面只负责渲染。
- 使用 Naive UI、Tailwind 工具类和现有 SCSS Token；组件目录会形成模板命名空间，脚本 API 仍需显式导入。
- 新增或调整引导步骤时，同步维护 `tutorials/registry.ts` 和 `tutorials/selectors.ts`，确保选择器仍指向实际控件。
- 用户界面与代码注释以中文为主；Go 注释沿用文件职责、使用、边界等约定，前端保留现有分区注释风格。
- 生成文件不要手改：`frontend/wailsjs/**`、`components.d.ts`、`typed-router.d.ts` 均由工具生成。

量化与蓝图的方案和验证记录见 [戴森球球面纬圈布局生成方案](docs/蓝图生成修正/戴森球球面纬圈布局生成方案.md)；该文档中的历史测试入口和脚本路径已不代表当前仓库的可执行命令。

## 数据与第三方资源

- 图鉴元数据和中文名称来自 [FactorioLab](https://github.com/factoriolab/factoriolab) 的固定提交（MIT），分类、离线摘要和成就内容由本项目整理；图标从本地资源包加载，缺失时回退到 Game Icons 类别图标。详情见 [图鉴 NOTICE](frontend/src/data/encyclopedia/NOTICE.md)。
- 量化内置 12 套数据集，行为基准为 dsp-calc（作者：神不在的黄昏）；数据集随程序分发，许可证全文、来源和差异说明见 [量化 NOTICE](helper/quant/NOTICE.md)。求解器、名称图和成本模型为本项目独立实现。
- 蓝图 3D 预览使用项目内置的 `dsp-blueprint-editor` UMD 资源，接口见 [lib-README](frontend/src/packages/dsp-blueprint-editor/lib-README.md)。
- 本地 Game Icons 图标遵循 CC BY 3.0，署名见 [game-icons-license.txt](frontend/public/game-icons-license.txt)；游戏素材、名称和商标归原权利人所有。
- 模组来源为 Thunderstore，前置框架为 BepInEx，联机框架为 Nebula Multiplayer；本项目与这些项目没有隶属关系。

## 已知限制

- 模组市场的“手动导入”来源尚未接入远端数据；本地模组导入请使用“资源”页。
- BepInEx 更新目前不做已安装版本升级；已安装时只返回跳过结果。
- 卸载 BepInEx 会删除游戏目录下的整个 `BepInEx` 文件夹（含插件、补丁器和配置）及 Doorstop 注入文件，且不会清理模组市场的安装记录；卸载前请确认并备份。
- Nebula 在线玩家列表、延迟和连接控制接口尚未接入，开服页对应分区显示不可用。
- 量化与蓝图生成仍属实验性能力：球面锚点只是粘贴建议，预览器也不做 Unity Physics 碰撞校验；生成结果必须在游戏中实际放置验证。
- 图鉴数据固定对应游戏版本 `0.10.29.21950`，不包含存档加成、个人科技状态或后续版本新增内容。
- 联机依赖账号服务器和 P2P 网络；防火墙、NAT、UPnP、IPv6、中继授权和运营商网络都可能影响连接，玩家需自行保证游戏版本与模组一致。
- 账号、公告、联机大厅和模组简介翻译需要应用服务器可达；本地功能不依赖登录。
- 自更新依赖 Gitee Release 的附件名称和 `checksums.txt`；更新写入后需要重启应用，不会自动重启。
- 当前仅以 Windows 桌面为目标，发布 `amd64` 与 `arm64` 两个架构。

## 贡献

欢迎提交 Issue 或 Pull Request。比较适合优先参与的方向：

- 完善模组安装、卸载、依赖共享和冲突恢复。
- 补充 Nebula 依赖安装与在线玩家管理能力。
- 改进球面蓝图布局、碰撞预检和游戏内放置验证。
- 完善联机打洞、中继策略和网络诊断。
- 更新图鉴数据、补充条目说明和成就内容。
- 优化日志检索、诊断导出和错误提示。
- 补充截图、使用文档和发布说明。

提交前建议至少运行：

```powershell
go build ./...
cd frontend
npm run build
```

如果修改了后端导出服务，再运行 `wails generate module`，并确认生成文件 diff 符合预期。

## 免责声明

戴森球盒子是社区开发工具，不隶属于 Youthcat Studio、Gamera Games、Steam、Thunderstore、BepInEx、Nebula 或 FactorioLab 官方项目。

使用模组可能影响游戏体验、存档兼容性或多人联机稳定性。请在安装、卸载、更新模组或批量整理资源前备份重要存档，并自行确认模组与当前游戏版本兼容。

蓝图生成结果、量化数值和图鉴数据来自本地模型或固定版本公开数据，可能随游戏版本、模组组合和实际布局变化而失效；请在游戏内自行验证。

联机会向服务器上报房间与网络信息，并在玩家之间建立 P2P 连接。请只在可信网络中使用，并自行评估防火墙、端口和隐私风险。

## 开源协议

本项目使用 [MIT License](LICENSE)。

## 作者

DarkSKL

- Email: 18214780622@163.com
- 项目主页：<https://gitee.com/Dark-Skl/dyson-sphere-box>
- QQ 群：779182569
- 微信：DarkSKL
