# Thunderstore 全链路新版适配方案

## 背景与目标

当前项目仍以 `https://thunderstore.io/c/dyson-sphere-program/api/v1/package/` 作为 Thunderstore DSP 包索引入口。该旧入口在线上已经返回 `HTTP 404`，会直接影响市场列表、依赖递归解析、BepInEx 最新版本解析和远端安装准备流程。

本方案目标是把 Thunderstore 全链路切换到当前可用的新版分片索引，同时保留旧接口作为兜底路径，并在中间增加一层适配器，把不同来源的响应统一转换为项目现有 `thunderstore.Package` / `thunderstore.Version` 契约。业务层、安装器和前端继续消费统一 DTO，不直接感知 Thunderstore 新旧接口差异。

## 已验证现状

验证时间：2026-06-16。

| 链路 | 验证结果 | 结论 |
| --- | --- | --- |
| `/c/dyson-sphere-program/api/v1/package/` | `404 text/html` | 旧全量包列表接口不可作为主链路 |
| `/c/dyson-sphere-program/api/v1/package-listing-index/` | `200 application/octet-stream` | 新索引入口可用，返回 gzip blob |
| index blob | 解压后得到 `1` 个 chunk URL | 新索引是分片入口，不是直接包列表 |
| chunk blob | 解压后得到 `607` 个 Package | 可转换为当前项目 Thunderstore DTO |
| `xiaoye97/BepInEx` | 最新可用版本 `5.4.17` | BepInEx 元数据可从新索引解析 |
| BepInEx `download_url` | `200 application/zip` | BepInEx zip 下载链路可用 |
| Nebula `download_url` | `200 application/zip` | 普通 Mod zip 下载链路可用 |
| Nebula icon | `200 image/png` | 图标资源链路可用 |
| Cyberstorm dependencies API | `200 application/json` | 可作为详情补充，不建议作为核心索引 |

新版主链路形态：

```text
GET https://thunderstore.io/c/dyson-sphere-program/api/v1/package-listing-index/
  -> 302/200 到 gcdn blob
  -> gzip(JSON([]chunkURL))
  -> GET chunkURL
  -> gzip(JSON([]Package))
  -> 聚合所有 Package
```

## 影响范围

现有代码中与 Thunderstore 相关的核心链路如下：

| 场景 | 当前入口 | 问题 | 新方案 |
| --- | --- | --- | --- |
| 市场列表 | `thunderstoreindex.LoadPackages` | 旧接口 404 | 走新版分片索引适配器 |
| 刷新市场 | `ListRemotePackages(...UseCache=false)` | 绕过缓存后必然撞旧接口 | 走新版分片索引适配器 |
| 按 key 回查包 | `marketplace.ResolveRemotePackage` | 依赖旧索引加载成功 | 复用新版统一包列表 |
| 依赖解析 | `thunderstoreview.parseDependency` | 依赖包需要从索引匹配 | 使用新版聚合包列表 |
| BepInEx 最新版本 | `bepinex/remote.fetchThunderstorePackages` | 直接请求旧接口 | 复用新版统一包列表 |
| BepInEx 下载 | `remote.DownloadPackage` | 下载 URL 本身仍可用 | 保持下载逻辑，补强错误信息 |
| V2 artifact 下载 | `artifactfetch.downloadRemoteArtifact` | 下载 URL 本身仍可用 | 保持下载逻辑，补强来源诊断 |
| 包页面/详情跳转 | `PackageURL` | 页面可能被限流或 Cloudflare 拦截 | 仅用于展示跳转，不作为安装必需链路 |
| Cyberstorm API | `/api/cyberstorm/...` | 部分接口 500/504 | 只作为补充详情，不作为主索引 |

## 目标架构

新增一层 Thunderstore 索引适配器，统一隐藏新旧接口差异。

```text
service / installer / marketplace
        |
        v
helper/modDependence/marketplace/thunderstoreindex.LoadPackages
        |
        v
ThunderstoreIndexAdapter
        |
        +-- ChunkedIndexProvider     新版主链路：package-listing-index + chunk
        |
        +-- LegacyPackageProvider    旧版兜底：api/v1/package
        |
        +-- LocalCacheFallback       本地 SQLite 缓存兜底
        |
        v
[]thunderstore.Package
```

适配器职责：

1. 优先加载新版 `package-listing-index`。
2. 新版失败时尝试旧版 `api/v1/package/`，旧版只作为兼容兜底。
3. 远端全部失败时尝试读取本地缓存，避免用户已有缓存时完全不可用。
4. 所有成功结果统一返回 `[]thunderstore.Package`。
5. 成功加载远端后刷新本地缓存。
6. 对外不暴露 gzip、chunk、302、CDN URL 等来源细节。

## 接口契约规划

建议在 `apiDefinition/modDependence/thunderstore` 中把 API 地址拆成语义明确的常量：

```go
const (
    ChunkedIndexURL = "https://thunderstore.io/c/dyson-sphere-program/api/v1/package-listing-index/"
    LegacyIndexURL  = "https://thunderstore.io/c/dyson-sphere-program/api/v1/package/"
)
```

`IndexURL` 可以暂时保留为兼容别名，但不再作为业务层直接依赖的唯一入口。后续代码统一通过适配器选择 provider，不让业务层拼 URL。

建议新增内部结构：

```go
type indexProvider interface {
    Load(ctx context.Context) ([]thunderstore.Package, error)
    Name() string
}
```

Provider 划分：

| Provider | 职责 | 边界 |
| --- | --- | --- |
| `chunkedIndexProvider` | 请求新版索引、解压 index、拉取 chunk、解压 package | 不写缓存、不筛选包 |
| `legacyIndexProvider` | 请求旧 `api/v1/package/` 并直接解码 `[]Package` | 只作兜底 |
| `cacheFallback` | 读取 SQLite 中最近可用缓存 | 不请求远端 |

## 新版索引适配流程

`chunkedIndexProvider.Load(ctx)` 推荐流程：

1. 使用 `GET ChunkedIndexURL` 请求索引入口。
2. 跟随 Thunderstore 到 `gcdn.thunderstore.io` 的跳转。
3. 读取响应字节，使用 `gzip.NewReader` 解压。
4. JSON 解码为 `[]string`，每个元素是 chunk URL。
5. 逐个请求 chunk URL。
6. 逐个 gzip 解压并 JSON 解码为 `[]thunderstore.Package`。
7. 聚合所有 chunk 结果。
8. 如果 chunk 列表为空、gzip 损坏、JSON 不匹配或任一 chunk 请求失败，返回明确错误，让上层尝试旧接口或缓存。

请求头建议：

```text
Accept: application/json, application/octet-stream
User-Agent: DysonSphereBox/<app-version>
```

不要把网页 `package_url` 作为程序化数据来源。网页路由可能出现限流、Cloudflare 挑战或 HTML 响应，只适合前端跳转。

## 旧版兜底策略

旧版 `/c/dyson-sphere-program/api/v1/package/` 当前已经返回 404，但仍建议保留为兜底 provider，理由是：

1. Thunderstore 可能对不同社区或缓存层有阶段性恢复。
2. 旧版响应契约与项目现有 DTO 完全一致。
3. 保留 provider 可以降低切换风险，不影响新版主链路。

兜底顺序建议：

```text
允许缓存且缓存未过期
  -> 直接返回缓存

缓存不可用或强制刷新
  -> 新版 chunked provider
  -> 旧版 legacy provider
  -> 任意可读缓存
  -> 返回聚合错误
```

错误信息建议包含每个 provider 的失败原因，例如：

```text
Thunderstore index load failed: chunked returned gzip decode error; legacy returned HTTP 404; cache unavailable
```

这样前端和日志可以明确区分是索引失败、下载失败还是缓存不可用。

## BepInEx 链路改造

当前 BepInEx 安装链路中，`ResolveLatestBepInExPackage()` 会单独请求 Thunderstore 旧索引。新版方案中应取消这条独立 HTTP 请求，改为复用统一索引适配器。

建议流程：

```text
BepInExService.Install / Update / GetLatestVersion
  -> bepinex/remote.ResolveLatestBepInExPackage
  -> thunderstoreindex.LoadPackages(ctx, useCache=true)
  -> 在 []Package 中匹配 xiaoye97/BepInEx
  -> LatestActiveVersion
  -> remote.DownloadPackage(download_url)
```

BepInEx 下载 URL 当前验证可用：

```text
https://thunderstore.io/package/download/xiaoye97/BepInEx/5.4.17/
```

下载阶段不需要切换接口，但建议补强诊断：

1. `DownloadURL` 为空时返回 “BepInEx download url is empty”。
2. HTTP 非 2xx 时保留 owner/name/version/status。
3. 下载后 zip 校验失败时保留目标路径和 Content-Type。
4. 审计日志中记录索引 provider 名称和下载 URL。

## Mod 安装链路改造

V2 安装器中普通 Thunderstore Mod 的关键步骤是：

```text
InstallRemoteModPackage
  -> prepareBepInExForRemoteInstall
  -> ResolveInstallArtifacts
  -> marketplace.ResolveRemotePackage
  -> artifactfetch.downloadRemoteArtifact
```

需要保证：

1. `ResolveRemotePackage` 使用新版统一索引适配器。
2. `parseDependency` 仍按 `namespace-name-version` 字符串解析。
3. BepInEx 依赖继续跳过普通 Mod 安装，由 BepInEx 专用安装器负责。
4. 普通依赖的 `DownloadURL` 来自 chunk 中的 `version.download_url`。
5. `artifactfetch` 继续负责 zip 下载、partial 文件、缓存复用和 zip 校验。

已验证普通 Mod 示例：

```text
nebula/NebulaMultiplayerMod 0.9.22
download_url -> 200 application/zip
dependencies -> 6
first dependency -> PhantomGamers-IlLine-1.0.0
icon -> 200 image/png
```

## Cyberstorm API 定位

Thunderstore 仓库中存在新版 `/api/cyberstorm/...` API，适合做详情补充，例如：

```text
/api/cyberstorm/package/{namespace}/{name}/v/{version}/dependencies/
```

验证中 dependencies API 返回 `200 application/json`，但列表、详情、版本等接口出现过 `500/504`。因此不建议把 Cyberstorm API 作为安装器核心链路。

推荐定位：

1. 主索引、依赖解析、安装下载全部使用 `package-listing-index` 分片索引。
2. Cyberstorm 仅用于后续详情页增强，例如 README、changelog、依赖详情展示。
3. Cyberstorm 请求失败不影响安装链路。

## 文件落点建议

建议优先改动以下文件：

| 文件 | 改动 |
| --- | --- |
| `apiDefinition/modDependence/thunderstore/package.go` | 增加新版与旧版索引 URL 常量，保留 DTO |
| `helper/modDependence/marketplace/thunderstoreindex/loader.go` | 改为统一适配器入口，负责 provider 编排与缓存兜底 |
| `helper/modDependence/marketplace/thunderstoreindex/chunked.go` | 新增新版分片 provider |
| `helper/modDependence/marketplace/thunderstoreindex/legacy.go` | 新增旧版 provider |
| `helper/modDependence/marketplace/thunderstoreindex/gzip.go` | 新增 gzip JSON 解码辅助 |
| `helper/modDependence/bepinex/remote/package.go` | BepInEx 版本解析复用 `LoadPackages` |
| `helper/modDependence/installer/v2/artifactfetch/download.go` | 补强下载失败诊断，不改变下载协议 |

文件拆分理由：

1. `apiDefinition` 只描述外部契约和 URL。
2. `thunderstoreindex` 只负责加载并返回原始包索引。
3. `marketplace/thunderstoreview` 继续负责过滤、key 和 DTO 转换。
4. `bepinex/remote` 只负责 BepInEx 包选择和 zip 下载。
5. `installer/v2` 只负责安装 artifact，不关心 Thunderstore 索引来源。

## 数据兼容与缓存

现有缓存表保存的是序列化后的 `[]thunderstore.Package`。新版分片索引聚合后仍然写入相同结构，因此不需要数据库迁移。

缓存策略建议：

1. `useCache=true` 且缓存未过期时直接返回缓存。
2. `useCache=false` 时强制请求远端，但远端失败仍允许读取任意可用缓存兜底。
3. 新版 provider 成功后覆盖缓存。
4. 旧版 provider 成功后也覆盖缓存。
5. 缓存内容不记录 provider 差异，避免影响旧数据读取。

可选增强：

1. 在缓存 metadata 中记录 `provider=chunked|legacy`。
2. 记录 `chunk_count`、`package_count`、`last_success_at` 用于诊断。
3. 前端刷新失败时展示 “已使用本地缓存”。

## 错误与日志设计

建议定义索引加载错误上下文，而不是只返回 `Thunderstore returned HTTP 404`。

错误分类：

| 类型 | 示例 | 用户提示 |
| --- | --- | --- |
| 新版索引失败 | index HTTP 526、gzip decode error | Thunderstore 新版索引暂时不可用，正在尝试兜底 |
| chunk 失败 | chunk HTTP 403、JSON decode error | Thunderstore 包分片读取失败 |
| 旧版失败 | legacy HTTP 404 | Thunderstore 旧版接口不可用 |
| 缓存失败 | database unavailable、cache expired | 本地缓存不可用 |
| 下载失败 | zip HTTP 404、invalid zip | Mod 文件下载失败 |

日志建议记录：

1. provider 名称。
2. 请求 URL 类型，避免完整记录长 CDN URL 时污染日志。
3. HTTP status。
4. chunk 数量和 package 数量。
5. 是否使用缓存兜底。
6. BepInEx owner/name/version/download_url。

## 实施步骤

第一阶段：索引适配器

1. 增加 `ChunkedIndexURL` 和 `LegacyIndexURL`。
2. 在 `thunderstoreindex` 新增 provider 抽象。
3. 实现 chunked provider。
4. 实现 legacy provider。
5. `LoadPackages` 改为 provider 编排加缓存兜底。

第二阶段：BepInEx 复用

1. 删除 BepInEx 内部单独请求旧索引的逻辑。
2. `ResolveLatestBepInExPackage` 调用统一 `LoadPackages`。
3. 下载逻辑保持不变。
4. 补充错误上下文和日志字段。

第三阶段：安装链路验证

1. 市场页加载 Thunderstore 列表。
2. 刷新 Thunderstore 列表。
3. 搜索 Nebula。
4. 解析 Nebula 依赖链。
5. 获取 BepInEx 最新版本。
6. BepInEx zip 下载并校验。
7. 普通 Mod zip 下载并校验。

第四阶段：兜底与观测

1. 模拟新版索引失败，确认旧版 provider 被调用。
2. 模拟新旧远端均失败，确认本地缓存兜底。
3. 缓存不可用时返回聚合错误。
4. 日志中能看出失败 provider 和最终使用来源。

## 验证清单

实施后至少执行：

```powershell
cd D:\Workspace\DysonSphere\DysonSphereBox
go build ./...
```

如修改 Wails 暴露方法或 DTO 字段，再执行：

```powershell
wails generate module
```

建议增加或补充的测试：

1. `chunkedIndexProvider` 能解码 gzip index。
2. `chunkedIndexProvider` 能解码 gzip chunk。
3. chunk URL 为空时返回错误。
4. chunk 请求失败时返回错误。
5. `LoadPackages` 在新版失败时尝试旧版。
6. 新旧都失败时读取缓存。
7. BepInEx 能从统一索引中解析到 `xiaoye97/BepInEx`。
8. BepInEx 依赖 key 被 V2 安装器跳过。

## 最终推荐

把 `package-listing-index` 作为 Thunderstore 唯一主链路，把旧 `api/v1/package` 降级为 provider 兜底，把 Cyberstorm API 定位为详情增强。中间适配器统一输出现有 `[]thunderstore.Package`，这样市场、依赖、BepInEx、安装器都不需要关心 Thunderstore 后端接口形态变化。

该方案可以在不迁移数据库、不改前端 DTO、不重写安装器的前提下，恢复当前 Thunderstore 市场和 BepInEx 安装链路，并保留后续扩展空间。
