# 戴森球盒子

> 面向《戴森球计划》的 Windows 桌面工具合集，用来管理游戏目录、BepInEx 前置环境、模组整合包、蓝图工具、Nebula 开服环境、系统日志和应用自更新。

戴森球盒子是 DarkSKL 为《戴森球计划》玩家制作的桌面工具。项目目标是把安装前置、配置游戏路径、安装整合包、使用蓝图工具、准备联机开服环境这些容易出错的操作，收进一个更直观、可审计、适合中文玩家使用的应用里。

项目仍在持续迭代中，但当前已经不只是界面雏形：本地配置、BepInEx 管理、多渠道模组整合包列表、整合包安装/卸载、蓝图工具内嵌、Nebula 无头服务器状态检测与启停、系统日志、自更新检查等流程都已经接入。

## 本版本重点

- 新增“蓝图”页面，内嵌蓝图工具，并提供资源预加载、启动状态和失败重试提示。
- 完善模组来源适配，支持 Thunderstore 与手动导入入口。
- 模组市场列表支持按渠道搜索、刷新、分页加载、封面地址适配和本地安装状态同步。

## 当前能力

- 游戏目录管理
  - 手动选择《戴森球计划》安装目录。
  - 从已保存配置读取游戏目录。
  - 从 Steam 库和常见快捷方式位置自动查找 `DSPGAME.exe`。
  - 保存到本地 SQLite 配置项 `Game/StartupFilePath`。

- BepInEx 前置环境
  - 检查 `winhttp.dll`、`doorstop_config.ini`、`BepInEx/core` 等关键文件。
  - 读取本地 BepInEx 版本。
  - 从 Thunderstore 查询并安装 BepInEx。
  - 卸载 BepInEx、Doorstop 注入文件和本地模组安装记录。

- 模组整合包
  - 从 Thunderstore 加载模组列表，支持本地搜索、刷新和分页。
  - 支持 Thunderstore 与手动导入入口。
  - 按来源搜索、刷新和分页读取整合包市场列表。
  - 展示整合包封面、依赖、版本、简介和安装状态。
  - 安装整合包依赖到 `BepInEx/plugins`。
  - 卸载本地记录中可安全移除的整合包依赖。
  - 处理整合包安装中的组件版本冲突，并同步本地安装记录。

- 蓝图
  - 在应用内直接打开蓝图工具，减少浏览器跳转。
  - 预加载外部页面声明的脚本、样式和模块资源。
  - 展示加载进度、失败原因和重新加载入口。
  - 保留外部蓝图页面的完整编辑与生成能力。

- Nebula 开服
  - 检查 BepInEx、Nebula 相关依赖、`steam_appid.txt`、端口监听、进程状态和日志路径。
  - 准备基础开服环境，写入 `steam_appid.txt` 并创建插件目录。
  - 保存 Nebula HostPort 和存档启动参数。
  - 启动或停止 Nebula 无头服务器进程。
  - 展示 TCP/UDP 监听状态、进程 PID、网络速率采样和最近开服日志。
  - 保留 `script/install-headless.*` 与 `script/start-headless.cmd` 作为独立脚本方案。

- 系统日志
  - 后端重要操作会写入本地日志表。
  - 前端日志页支持分页、刷新、命名空间、模块、事件、等级和类型筛选。
  - 日志包含通用 `metadata`，便于追踪安装、配置、更新和开服操作。

- 应用更新
  - 设置页可检查 Gitee Release 最新版本。
  - 发布包使用 `checksums.txt` 做 SHA256 校验。
  - 更新完成后替换当前程序，用户重启后运行新版本。

## 技术栈

- 桌面框架：Wails v2
- 后端：Go、GORM、SQLite
- 前端：Vue 3、TypeScript、Vite 3
- UI：Naive UI、Tailwind CSS、SCSS、Lucide Icons
- 路由：Vue Router 4、`unplugin-vue-router`
- 图表与动效：ECharts、Lottie
- 当前重点平台：Windows

## 页面说明

- 首页：当前保留轻量占位，后续用于聚合入口或状态总览。
- 开服：Nebula 无头服务器环境检测、准备、配置、启动、停止和运行监控。
- 模组：多渠道整合包列表、BepInEx 环境提示、搜索、刷新、安装、卸载与冲突处理。
- 蓝图：内嵌蓝图工具、资源预加载、加载状态和失败重试。
- 日志：本地系统日志查询与筛选。
- 设置：游戏路径、BepInEx 检查/卸载、应用更新。
- 账号：按 `docs/ex/auth-account-api.md` 接入验证码、注册、登录、用户信息、改密和注销；认证服务当前连接本机 `127.0.0.1:8888`。
- 关于我们：项目地址、反馈入口、联系方式、鸣谢与免责声明。

## 环境要求

参与开发前，请安装：

- Go 1.23+
- Node.js 16+
- npm
- Wails CLI v2
- Windows WebView2 Runtime

安装 Wails CLI：

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

检查 Wails 环境：

```powershell
wails doctor
```

## 本地开发

克隆项目：

```powershell
git clone https://github.com/DarkSKL/DysonSphereBox.git
cd DysonSphereBox
```

安装前端依赖：

```powershell
cd frontend
npm install
cd ..
```

启动开发模式：

```powershell
wails dev
```

开发模式会同时启动 Wails 桌面应用和 Vite 前端服务，适合调试前后端绑定。

## 常用验证

前端构建：

```powershell
cd frontend
npm run build
cd ..
```

Go 构建：

```powershell
go build ./...
```

修改后端导出服务后，重新生成 Wails 绑定：

```powershell
wails generate module
```

注意：`frontend/wailsjs` 是生成文件，不要手动编辑。

## 发布构建

Windows 发布脚本会询问版本号；也可以在命令行直接传入：

```powershell
deploy\build.bat
deploy\build.bat v1.0.1
```

脚本会：

- 校验并规范化版本号为 `vX.Y.Z`，通过 `-ldflags` 注入 `DysonSphereBox/core.Version`。
- 构建 `windows/amd64` 和 `windows/arm64`。
- 生成 `DysonSphereBox_windows_amd64.zip`、`DysonSphereBox_windows_arm64.zip`。
- 生成 `checksums.txt`。

产物位于 `deploy/release/<版本号>/`。压缩包只包含根目录下的 `DysonSphereBox.exe`；发布到 Gitee Release 时上传两个 zip 包和 `checksums.txt`。应用自更新依赖这些附件名称和 SHA256 校验清单。

## 项目结构

```text
DysonSphereBox
├── app.go                    # Wails 应用生命周期与 App 绑定入口
├── main.go                   # Wails 启动入口
├── app.db                    # SQLite 模板数据库
├── core/                     # 数据库、配置、版本、全局 HTTP 客户端
├── helper/                   # 服务辅助逻辑、安装器、日志构造、自更新源
├── model/                    # GORM 数据模型与前端 DTO
├── service/                  # Wails 后端服务
├── utils/                    # 通用路径、Steam、快捷方式和游戏目录工具
├── apiDefinition/            # 后端接口路径定义
├── script/                   # Nebula Headless 独立脚本
├── deploy/                   # 发布构建脚本与本地发布产物
├── frontend/                 # Vue 3 前端项目
│   ├── src/
│   │   ├── components/       # 可复用组件
│   │   ├── composables/      # 组合式状态与逻辑
│   │   ├── layouts/          # 页面布局
│   │   ├── pages/            # 文件路由页面
│   │   ├── services/         # 前端 Wails 调用封装
│   │   └── types/            # 共享类型声明
│   └── wailsjs/              # Wails 生成的前端绑定文件
└── build/                    # Wails 构建资源
```

## 开发约定

- 前端页面放在 `frontend/src/pages`，使用 `definePage({ meta })` 声明布局、标题和缓存策略。
- 共享布局放在 `frontend/src/layouts`，可复用组件放在 `frontend/src/components`。
- 前端优先使用 Naive UI 组件和 Tailwind 工具类，复杂局部样式再使用 SCSS。
- 后端业务入口放在 `service/<domain>`，可复用辅助能力放在 `helper/<domain>`。
- GORM 模型放在 `model/<domain>`，统一通过 `model.RegisterModels()` 聚合迁移。
- 重要配置、安装、更新、启停等操作应写入系统日志。
- 对外展示名使用“戴森球盒子”；Go module、构建产物和自更新包名仍使用 `DysonSphereBox`。

## 贡献

欢迎提交 Issue 或 Pull Request。比较适合优先参与的方向：

- 完善远端整合包接口和数据结构。
- 改进模组安装、卸载、更新和冲突处理。
- 补充 Nebula 开服环境的一键修复能力。
- 完善蓝图工具内嵌体验和离线/镜像方案。
- 优化日志检索、日志清理和导出。
- 补充截图、使用文档和发布说明。
- 适配更多下载源或更多平台。

提交前建议至少运行：

```powershell
go build ./...
cd frontend
npm run build
```

## 免责声明

戴森球盒子是社区开发工具，不隶属于 Youthcat Studio、Gamera Games、Steam、Thunderstore、BepInEx 或 Nebula 官方项目。

使用模组可能影响游戏体验、存档兼容性或多人联机稳定性。请在安装、卸载或更新模组前备份重要存档，并自行确认模组与当前游戏版本兼容。

## 开源协议

本项目使用 [MIT License](LICENSE)。

## 作者

DarkSKL

- Email: 18214780622@163.com
- Project: 戴森球盒子
