# DysonSphereBox

> 一个面向《戴森球计划》的桌面工具箱，用来管理游戏路径、Mod 前置环境与后续的整合包工作流。

我是 DarkSKL，DysonSphereBox 是我为《戴森球计划》玩家做的桌面工具箱。这个项目的目标很简单：把安装前置、检查环境、管理 Mod 整合包这些原本容易出错的事情，做成一个更直观、更适合中文玩家使用的工具。

项目目前还在早期开发中。我会先把基础能力打稳，再逐步补齐整合包安装、更新、卸载和更多下载源支持。如果你也在折腾《戴森球计划》的 Mod 生态，欢迎一起把它做得更好用。

## 项目状态

我正在把这个项目整理成可长期维护的开源项目，核心框架和部分基础能力已经接入：

- 桌面端应用框架：Wails v2
- 前端：Vue 3、TypeScript、Vite、Naive UI、Tailwind CSS
- 后端：Go、GORM、SQLite
- 当前平台重点：Windows
- 当前重点能力：游戏目录检测、配置持久化、BepInEx 状态检测与安装流程

部分 Mod 整合包安装、卸载、更新等能力仍在完善中，欢迎提出 Issue 或参与开发。

## 功能特性

- 自动查找《戴森球计划》安装目录
  - 支持从已保存配置读取
  - 支持从 Steam 相关环境中查找
  - 支持手动选择游戏目录

- 本地配置管理
  - 使用 SQLite 保存配置
  - 支持游戏路径配置
  - 支持后端 HTTP BaseURL 配置

- BepInEx 前置环境管理
  - 检查 BepInEx 是否已安装
  - 检查 `winhttp.dll`、`doorstop_config.ini`、`BepInEx/core` 等关键文件
  - 读取本地 BepInEx 版本
  - 从 Thunderstore 查询并下载最新 BepInEx 包
  - 安装时会修复 Doorstop 配置

- Mod 整合包界面雏形
  - 联机 Mod 包
  - 常用辅助 Mod 包
  - 安装进度与已安装状态展示

## 预览

项目界面仍在快速迭代中，当前主要页面包括：

- 首页：项目入口与快捷导航
- Mod：整合包列表与环境检查提示
- 设置：游戏路径、连接地址、BepInEx 状态管理

后续我会在这里补充应用截图、演示 GIF 和发布版本下载地址。

## 环境要求

参与开发前，请先安装以下环境：

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

开发模式会启动 Wails 应用和 Vite 前端服务，适合本地调试前后端交互。

## 构建

构建前端：

```powershell
cd frontend
npm run build
cd ..
```

构建桌面应用：

```powershell
wails build
```

构建产物会输出到 Wails 默认构建目录中。

## 项目结构

```text
DysonSphereBox
├── app.go                    # Wails 应用生命周期与前端绑定入口
├── main.go                   # Wails 启动入口
├── core/                     # 数据库、配置、全局运行时资源
├── helper/                   # 服务辅助逻辑
├── model/                    # GORM 数据模型
├── service/                  # Wails 后端服务
├── utils/                    # 通用工具函数
├── frontend/                 # Vue 3 前端项目
│   ├── src/
│   │   ├── components/       # 可复用组件
│   │   ├── composables/      # 组合式状态与逻辑
│   │   ├── layouts/          # 页面布局
│   │   └── pages/            # 文件路由页面
│   └── wailsjs/              # Wails 生成的前端绑定文件
└── build/                    # Wails 构建资源
```

## 开发约定

- 前端使用 Vue 3 Composition API 与 `<script setup lang="ts">`
- UI 组件优先使用 Naive UI
- 样式优先使用 Tailwind CSS，复杂局部样式使用 SCSS
- 后端服务通过 Wails 绑定暴露给前端
- SQLite 通过 GORM 访问
- 修改后端导出服务后，需要重新生成 Wails 绑定

生成 Wails 绑定：

```powershell
wails generate module
```

验证 Go 代码：

```powershell
go build ./...
```

验证前端代码：

```powershell
cd frontend
npm run build
```

## 贡献

欢迎参与这个项目。你可以通过以下方式贡献：

- 提交 Bug 反馈
- 提出功能建议
- 补充文档或截图
- 优化 UI 与交互体验
- 完善 BepInEx、Mod 包安装、卸载和更新逻辑
- 适配更多平台或更多下载源

提交 Pull Request 前，建议先运行：

```powershell
go build ./...
cd frontend
npm run build
```

## 免责声明

DysonSphereBox 是社区开发工具，不隶属于 Youthcat Studio、Gamera Games、Steam、Thunderstore 或 BepInEx 官方项目。

使用 Mod 可能影响游戏体验、存档兼容性或多人联机稳定性。请在安装或更新 Mod 前备份重要存档，并自行确认 Mod 与当前游戏版本兼容。

## 开源协议

本项目使用 [MIT License](LICENSE)。

我选择 MIT，是因为它足够简单、宽松，也很适合这种个人维护的工具型项目。你可以自由使用、修改、分发或基于本项目继续开发，只需要保留原始版权与许可证声明即可。

## 作者

DarkSKL

- Email: 18214780622@163.com
- Project: DysonSphereBox
