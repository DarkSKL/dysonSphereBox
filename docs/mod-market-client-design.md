# 模组市场客户端设计：Thunderstore 兼容与用户上传

> 状态：设计稿，不修改业务代码。按用户最新澄清，DSBox 市场是我们维护的独立源，Thunderstore 是另一个独立源；只在客户端兼容适配，不建设服务端联动、镜像或统一远端注册中心。“现状”段落已核对源码，“拟议”段落不是当前实现。
>
> 联调契约：`D:/Workspace/DysonSphere/DysonSphereBox/develop/server/aiDoc/frontend-backend/mod-market-api.md`（由 server agent 维护）；最终跨端设计由协调者合并到 `develop/docs/mod-market-design.md`。

## 1. 设计结论

1. 普通玩家默认进入“盒子市场”，不登录、不选源、不手工处理依赖。熟悉 Thunderstore 的用户可从次级入口切换到它；两个列表分别查询，来源始终可辨认。当前 `manual` 保留为本地导入入口，不进入远端分页。包格式与安装能力统一，不要求服务端统一目录或两源联动。
2. “一键安装”先做本地预检。无冲突时直接执行；仅在目标游戏目录未确认、文件冲突或合并已有环境时显示一次确认。安装完成后不自动启动游戏。
3. 整合包默认创建独立、可命名的 profile；当前环境合并是显式的危险入口，显示文件、依赖、配置覆盖和备份摘要后再确认。当前 environment 只有 AppData 资源库与真实游戏镜像同步能力，不能把它描述成已有多 profile 启动能力。
4. 包身份使用不可变 opaque `packageId` + `releaseId`；新 DTO 不使用 `versionId`，Thunderstore 的 `uuid4` 放在 `externalRefs`。版本展示字段仍保留 `version`，不能作为唯一身份。
5. 锁文件只保存可复现定位和校验信息，不保存临时下载 URL。盒子源自己核验上传字节、依赖与审核状态，生成自身 `server-verified` 锁；Thunderstore adapter 独立解析、直连下载并记录 `client-observed`。盒子服务端不抓取 Thunderstore、不代理其发布、不计算其包 hash。扫描通过不代表无恶意。
6. 作者只需“上传相关模组、填写整合包信息”，然后点击发布。单页表单，不设独立预览确认、逐依赖选源或手写 JSON；清单生成、依赖、上传、校验与审核由后台完成。

### 1.1 两个独立源如何共用客户端

```text
盒子市场 API -> Wails 后端 box adapter ----------+
                                               +-> 统一 Package/Release DTO -> 本地计划 -> installer/v2
Thunderstore 官方 API -> thunderstore adapter --+
```

这是客户端适配边界，不是服务端整合。盒子服务器只维护盒子包、团队、版本、字节、依赖、上传与审核。两个源用相同 manifest 字段、相对安装路径规则和进度接口，仍拥有各自身份、账号、目录、失败与下架状态。同名、同版本、同 hash 都不能自动获得另一源身份。

未限定来源的盒子 `Owner-Name-Version` 只在本 registry 的盒子目录解析，未找到则报缺失，不去 Thunderstore 补齐。Thunderstore 标准依赖字符串只在 Thunderstore 内解析。整合包另设 externalDependencies，允许显式固定的 Thunderstore namespace/name/version/UUID/hash 引用；服务器仅校验 schema 和引用形状，不查询、抓取、镜像或宣称已验证外源。客户端对应 adapter 负责真实解析闭包、下载、哈希核对和兼容预检。作者可发布 wrapper+配置+明确引用，不需要重新发布或上传第三方 DLL；引用不形成 box 包身份。

## 2. 当前实现证据与边界

| 事实 | 代码证据 | 对设计的影响 |
| --- | --- | --- |
| 市场服务仅暴露分页和刷新 | `service/modDependence/marketplace.go:26-55` `MarketplaceService.GetRemotePackageList/RefreshRemotePackageList` | 详情、上传、发布、安装计划需要新增契约；不要把现有列表方法当成详情 API。 |
| 来源分派支持 `thunderstore`、`manual`；manual 当前返回空列表 | `helper/modDependence/marketplace/provider.go:24-43` | manual 应走本地文件选择/导入，不应伪装成可分页远端源。拟议远端 provider 枚举为 `box|thunderstore|local`，`local` 只在客户端。 |
| Thunderstore 直连官方索引并缓存 6 小时 | `apiDefinition/modDependence/thunderstore/package.go:14`；`helper/modDependence/marketplace/thunderstoreindex/loader.go:20-66`；`helper/modDependence/marketplace/cache/thunderstore.go:21-30` | 盒子服务器离线不应阻断普通 Thunderstore 首装；缓存过期不能当成在线验证。 |
| 实测 2026-10-01：HTTP 200、685 个 DSP 包，包和版本含 `uuid4`，版本没有 SHA256 | `https://thunderstore.io/c/dyson-sphere-program/api/v1/package/`（本次采样） | 只能声称索引端点和字段已验证；不能声称官方 API 全部 schema 或官方安全校验已验证。 |
| 当前列表过滤 Tools/BepInEx，并要求依赖链最终依赖 BepInEx | `helper/modDependence/marketplace/thunderstore.go:22-84`；`helper/modDependence/marketplace/thunderstoreview/rule.go:18-61` | 普通市场默认隐藏框架/工具；整合包解析不能依赖展示过滤结果，应按 lock 精确解析。 |
| 当前 key 是 `thunderstore:owner:name:version` | `helper/modDependence/marketplace/thunderstoreview/rule.go:81-87`；`apiDefinition/modDependence/remote_package.go:68-116` | 这是 legacy 本地安装 key。迁移时保留映射，不把它当新跨源权威身份。 |
| 普通 Mod 主包按选定版本；依赖按 Owner-Name 复用实际已装组件及其清单，未装取创建时间最新 active；Modpacks 才按清单 key | `helper/modDependence/installer/v2/artifactresolve/resolve.go:31-136`；`helper/modDependence/marketplace/thunderstoremeta/version.go:18-31` | 保留普通 Thunderstore 便捷策略；整合包、社区依赖与锁回放另用严格模式，不把任意已装版本称为满足声明。 |
| 版本替换改为备份旧受管文件、记录映射、完成事务迁移共享引用和实际传递闭包；不再先卸载旧主包 | `helper/modDependence/installer/v2/versionreplace/prepare.go:34-148`；`helper/modDependence/installer/v2/installstore/version.go:27-110`；`helper/modDependence/installer/v2/installstore/package.go:25-60` | 已有恢复基础，但逐文件写入并非原子交换；完成后会话 manifest 清空，仍缺持久 lock 和用户回滚点。 |
| V2 有冲突检测 DTO，但当前 dirty 复制路径对普通文件冲突直接覆盖，config 跳过冲突检测 | `helper/modDependence/installer/v2/copyconflict/inspect.go:26-77`；`helper/modDependence/installer/v2/copyfiles/conflict.go:18-37`；`helper/modDependence/installer/v2/copyfiles/step.go:20-54` | 危险合并预览确认尚未实现；必须在完整计划阶段聚合差异并绑定摘要，不能把 DTO 存在等同已保护用户文件。 |
| 实际依赖边写入本地会话并迁移共享引用；普通模式不同版本先到者胜，循环重复命中去重跳过 | `helper/modDependence/installer/v2/installmanifest/planned.go:42-63`；`helper/modDependence/installer/v2/artifactresolve/resolve.go:87-114` | 缺显式声明/实际边映射、循环错误和独立严格 lock 入口；详细证据见[当前差异报告](mod-market-client-current-delta.md)。 |
| 安装事件为 `modDependence:installProgress`，快照恢复可见状态但不自动重启 | `helper/modDependence/installer/v2/progress/event/event.go:22-55`；`frontend/src/composables/dysonSphere/useModPageState.ts:584-592` | UI 应区分 running、conflict、cancelled、failed；刷新后给“继续/取消”，不能静默重跑。 |
| BepInEx 安装服务只针对已配置游戏目录 | `service/modDependence/bepInEx.go:24-121`；`helper/modDependence/installer/v2/install_prepare_bepinex.go:21-45` | 新 profile 必须有独立 game/profile root 或明确“共享游戏目录、隔离插件树”的实现；未实现前不能承诺多 profile 启动。 |
| environment 是资源库、镜像同步、导入和操作恢复 | `service/environment/manager.go:69-111,330-376,410-612`；`service/environment/operation.go:114-191` | 可复用预览、哈希、回滚和恢复边界；不能把 AppData/plugins 当已隔离运行环境。 |
| 导入 Mod 只识别目录或 zip，复制到 AppData/plugins，不写入游戏目录 | `service/environment/manager.go:559-612` | 用户上传可从现有环境采集，但需额外读取 manifest、配置和受管文件清单。 |
| 账号请求始终 Vue → Wails AccountService → helper → `GLOBAL.ServerClient`，token 留在后端 | `service/system/account.go:14-89`；`helper/system/account/request.go:6-66`；`frontend/src/services/account.ts:1-176` | 浏览、普通安装和本地分享不登录；发布/远端上传在最后一步要求登录，登录前保留草稿。 |

## 3. Thunderstore 兼容等级

| 等级 | 承诺 | 明确不承诺 |
| --- | --- | --- |
| L1 索引兼容 | 读取 DSP community index、搜索、分页、详情、下载；识别 `owner-name-version` 依赖；按 BepInEx 规则落位。 | 不承诺官方账号代发、官方 API 全量 schema、上游 SHA256。 |
| L2 标准包兼容（目标） | 根目录 `manifest.json`、`README.md`、`icon.png`；manifest 的 `name`、`version_number`、`description`、`website_url`、`dependencies`；保留 Team-Name-Version 声明，社区依赖/整合包/锁回放按精确 release，普通 Thunderstore Mod 使用下述便捷策略。官方文档已验证 icon 为 256x256 PNG、版本为三段 SemVer。 | 不把便捷选择称为严格 exact 兼容，不把任意 zip/DLL 自动称为 Thunderstore 包；不接受自由 URL 或跨源同名替代。 |
| L3 manifest-only 整合包 | `Modpacks` 类包或 DSBox manifest 的依赖 lock，默认新 profile 安装，保留配置覆盖和本地 payload 引用。 | 不声称兼容 r2modman/Thunderstore profile/share code，除非另行验证其格式、版本和导入恢复。 |
| L4 profile/share code | 仅作为后续适配器；需固定格式版本、来源、锁、配置和隐私规则后再开放。 | 不把 DSBox 自有 lock ZIP 冒充官方 profile。 |

兼容状态在 UI 使用 `格式已适配 / 待验证 / 不支持` 三态，表示格式适配程度，不冒充游戏运行保证。Thunderstore API docs 页面本次可达，但 schema 未固定，因此只记录端点可达与实际索引字段证据。官方文档证据：`https://wiki.thunderstore.io/mods/creating-a-package` 与 `https://wiki.thunderstore.io/mods/packaging-your-mods` 本次 HTTP 200；profile/share code 尚未验证。

## 4. 玩家流程（最少步骤与异常）

### 4.1 发现、详情、一键安装

1. 打开“模组市场”（默认盒子源，0 次登录），输入关键词；客户端以统一接口 `ListPackages({provider:'box', keyword})` 调用盒子源。只有用户选择 Thunderstore 时才调用它，不后台自动替换搜索源。
2. 点击结果行查看详情（版本、作者、依赖数量、游戏版本、来源徽标、兼容等级）；详情只读，不要求先登录。
3. 点击“安装”。客户端获取/读取 release 元数据，检查游戏目录、BepInEx、进程占用、已有 package 和本地冲突；无阻塞项立即开始（共 3 个用户动作，安装按钮后无额外确认）。
4. 有阻塞项时显示一次摘要：缺失游戏目录、将安装的依赖、文件冲突数量、是否需要更新 BepInEx。用户确认后继续；取消不改文件。
5. 通过现有 `modDependence:installProgress` 显示下载/解压/复制/记录阶段。完成后显示“已安装”，启动游戏仍由用户点击。

#### 版本策略（协调者已裁决，目标规则）

普通 Thunderstore Mod 保留“主包选定版本、依赖复用本地实际版本、未装取最新 active”的便捷策略，用户无需选技术模式。计划分别记录声明版本、实际版本和选择原因，展示默认折叠；未知兼容性显示未验证，已知不兼容阻断。当前 Raw/组件版本与会话依赖边只有部分记录能力，完整产品 DTO、兼容预检仍待实施。

Thunderstore Modpacks、社区整合包及分享锁回放使用严格闭包，只有 exact 本地版本可复用；外源根即便分类为普通 Mod，子依赖也不得转便捷模式。社区普通模组按同 registry 的服务端精确契约，不继承 Thunderstore 普通策略。显式解析模式须由安装意图确定，不能只检查 Modpacks 标签；分享导出锁定实际安装版本和传递闭包，缺边时回查实际版本失败即不生成声称完整的锁。

### 4.2 整合包安装到新环境（默认）

1. 点击整合包“安装到新环境”（1 次安装意图）。客户端后台验证 lock、自动生成名称如 `包名 v1.2.3`；改名放在已创建环境的次级操作，不强制填表。
2. 客户端创建 profile 元数据和独立插件目标；profile root/启动绑定是隔离安装上线门槛，未实现版本显示“暂不支持隔离安装”，不得引导启用不存在的实验选项或偷偷写入现有游戏目录。
3. 依赖图、BepInEx 版本、配置覆盖和磁盘空间由后台预检；新环境无冲突时直接安装全部锁定组件，下载阶段可查看明细，不增加“开始安装”按钮。
4. 安装完成后 profile 状态为 ready；不自动启动，启动时传入 profileId。从结果行安装为 1 次动作；从搜索开始为“输入关键词→安装”2 次动作；查看详情额外 1 次。无需登录。

### 4.3 安装到已有环境/危险合并

1. 从整合包详情选择“合并到已有环境”，客户端先获取本地 inventoryDigest 和 environmentRevision。
2. 展示新增/更新/删除文件、受管与非受管冲突、配置覆盖、备份位置、共享依赖影响；未确认不得写入。
3. 非托管覆盖、影响其它 Mod、配置改写在计划阶段聚合，默认保留现有；必要选择在同一摘要内完成，一次确认绑定 `planDigest`，不逐文件弹窗。服务端 resolution 只提供依赖结果，客户端 `BuildInstallPlan` 后 CAS 执行；当前按安装顺序覆盖的 dirty 行为不能视为该目标已完成。
4. 失败时按会话 manifest 和备份恢复，显示可重试/导出诊断/撤销；已有环境合并至少 3 次用户动作，避免把危险操作伪装成一键安装。

### 4.4 更新、回滚与已安装版本

- 更新入口显示 `current release → target release`、依赖变化和可回滚点。先建立旧版本文件/配置快照，在新文件校验通过后交换受管记录；失败恢复旧快照。
- 回滚只允许选择本地仍有完整 lock/cache 的 release；没有完整 artifact 时给“重新下载”或“无法离线回滚”，不静默取最新版本。
- 同一 profile 内同一 `(provider,registryId,community,packageId)` 只允许一个 active release；不同 profile 可保留不同 release。共享依赖按 profile 内组件引用计数处理，卸载主包不删除其它引用仍需要的文件。

### 4.5 离线、源失败、取消和恢复

- 有完整既有 lock + 全部 hash 匹配的 cache：允许离线继续；`client-observed` 只表示与本地首次观测字节一致，不冒充官方验证。lock 中 locator 不含过期 URL，在线时由对应源 adapter 获取地址。
- 盒子源和 Thunderstore 分别记录 lastSuccessfulFetch/cacheState/errorCode。某源失败只影响该源；原来源包绝不自动换为另一源同名包。过期索引以 stale 状态只读显示，完整 lock/cache 仍可安装；新版本解析缺元数据则阻断。本功能需新增 allowExpired 读取路径，当前缓存函数不支持。
- 下载失败：保留失败快照和可重试按钮；取消触发 V2 cancel，已写文件按 manifest 清理。
- hash mismatch、zip 路径穿越、manifest 缺失或依赖无法解析：阻断该 artifact，显示原因和可移除项；不把“扫描通过”写成恶意检测结论。
- 应用崩溃/重启：沿用 `ListInstallProgressSnapshots` 与 environment operation recovery；恢复只清理已知临时目录和 `.dsbox` 文件，不触碰用户未知文件。

## 5. 作者流程

### 5.1 一个表单，上传模组并填写信息

用户任务为2阶段，操作为“添加文件→填写信息→发布”3次；一批文件算一次添加，填写信息算一次阶段，不以字段数增加向导步骤。

1. **上传相关模组**：拖入或选择多个ZIP、DLL、目录或已打好的整合包。后台识别、分组、提取已有manifest、计算hash、解析本地来源/依赖并生成lock；无manifest自动生成，无icon使用默认图，README从说明生成。多个独立模组目录默认一个整合包；多个DLL可能属于同一个Mod，资源目录保留关联，不能按DLL数量自动拆Mod。识别歧义只在原表单内修正。现有ImportMod拒绝裸DLL和多个plugins模组目录的ZIP，新增发布采集入口属于实施任务。
2. **填写整合包信息**：仅名称、版本（默认1.0.0或下个版本建议）、简短说明；已有manifest自动预填。封面/详细介绍可选折叠，namespace自动绑定盒子账号，识别结果与排除摘要只读留在本表单。作者不选技术来源、不手写依赖清单；高级明细只在主动展开时显示。发布权利声明归入发布按钮旁固定条款，不逐DLL弹授权向导。
3. **点击发布**：未登录才原位登录并续接草稿；一次publishIntent后台编排Init/chunks/Complete/scan/review，正常不二次确认。不要求用户等待各阶段并逐次点击。真缺失依赖、损坏文件或版本冲突在原表单对应项给修复动作，保留全部文件与输入，修好后再发布。

用户添加文件表达上传意图，但字节发送前后台须生成实际上传范围并排除个人路径/配置/token/日志；摘要同表单展示，不增加“确认上传”页。无法辨识的个人配置默认排除。已知第三方受管包自动转显式外源引用而非重传DLL，不要求作者重新发布全部依赖。

### 5.2 从现有环境导出整合包

1. 点击“分享当前环境”进入同一发布表单，Mod集合和已有信息自动填好，代替手工添加文件。
2. 作者只补名称/版本/说明；后台生成resolved lock、externalDependencies、安全configOverrides与localPayloads，受管Thunderstore Mod转明确引用，无需逐包选源/授权/重上传DLL。未知本地DLL缺上游身份不等于无权发布自己的代码，可作为box新payload校验审核；真实权利问题才列原表单修复项。
3. 点击“发布到盒子市场”或次级“导出本地”。正常仍为2任务阶段/3动作（分享入口→填写信息→发布），不增加独立预览确认页。登录/保存位置仅需要时发生。个人路径/token/日志/存档/未知配置默认排除，外源明细可展开但不继承box审核信任。

### 5.3 发布状态

首版新上传统一在自动校验、扫描后进入审核，审核前不公开资源或下载地址；审核通过自动公开并通知，无需再次点击发布。UI显示“正在上传 / 正在检查 / 待审核 / 已发布 / 需要修改”，拒绝原因回到原表单。正常状态路径draft→validating/scanning→reviewing→published；awaiting-confirmation仅实质变化待原表单修复，rejected保留原因和草稿。withdrawn/quarantined只改可用性，修复用新release；管理员下架不伪装成离线更新。可信作者免人工审核作为后续能力。

## 6. 身份、锁文件与契约建议

### 6.1 包身份

```json
{
  "provider": "thunderstore",
  "registryId": "thunderstore-public",
  "community": "dyson-sphere-program",
  "packageId": "opaque-package-id",
  "releaseId": "opaque-release-id",
  "namespace": "owner",
  "name": "PackageName",
  "version": "1.2.3",
  "externalRefs": {"thunderstoreUuid4": "...", "legacyKey": "thunderstore:owner:PackageName:1.2.3"}
}
```

`packageId` 表示包家族，releaseId 表示不可变发布；远端身份是 `(provider,registryId,community,packageId,releaseId)`，installed 身份再加 profileId，不能假定源或自维护服务器之间 ID 不重复。registryId 为服务器返回的稳定 registry UUID，与可变 ServerAddress 分开；切换 URL 必须重新验证 registryId，旧记录不自动改归属。Thunderstore 固定 registryId=thunderstore-public。box ID 由盒子服务器生成；Thunderstore adapter 用上游 package/version uuid4 形成源内 ID（缺 UUID 才用结构化名称回溯），不先注册盒子服务器。`namespace/name/version` 是展示与回溯字段；同 registry 的 release 不可覆盖。provider 只允许 box/thunderstore/local，local 不进入远端发布 DTO。

### 6.2 唯一共享 wire lock v1

联调及Box ZIP里的dsbox-lock.json统一使用server契约第4节的roots/packages/compatibility结构；以下是相同wire结构的最小完整例子。客户端早期root/components/game只作为内部安装模型命名，不是第二种共享schemaVersion=1；禁止以它直接请求服务器或写入共享v1文件。

```json
{
  "schemaVersion": 1,
  "registryId": "11111111-1111-4111-8111-111111111111",
  "trust": "server-verified",
  "compatibility": {"game": "dyson-sphere-program", "gameVersion": null, "platform": "windows-x64", "loader": "bepinex", "loaderVersion": "5.4.17"},
  "roots": ["rel_001"],
  "packages": [{
    "provider": "box", "packageId": "pkg_001", "releaseId": "rel_001",
    "namespace": "player_42", "name": "FactoryTools", "version": "1.0.0", "dependencies": [],
    "locator": {"provider": "box", "registryId": "11111111-1111-4111-8111-111111111111", "packageId": "pkg_001", "releaseId": "rel_001"},
    "artifacts": [{"artifactId": "art_001", "checksum": {"algorithm": "sha256", "value": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "provenance": "server", "hashVerified": true}}]
  }],
  "configOverrides": [],
  "localPayloads": [],
  "externalDependencies": [],
  "generator": "dsbox-lock-v1"
}
```

例中a哈希仅示意，不代表真实可安装字节。wire roots和每项dependencies都是同registry的releaseId字符串数组，每个引用必须在packages唯一出现；registryId由顶层及locator保留。兼容性字段与server一致，未知gameVersion/loaderVersion为null。hashVerified只放在artifacts[].checksum内部，不另设顶层同名字段；hash属于artifact，不是假定一个包只含一个文件。Box published artifact必须非空SHA256/provenance=server/hashVerified=true。

最终server契约已将externalDependencies纳入wire lock（缺省[]），RegistryPlan及Descriptor同样保留该字段。外源引用不能插入Box packages/dependencies字符串数组；含外源时整体trust=mixed，各artifact.checksum独立表示其信任，不将wrapper的server-verified传播给外源。发布wrapper+显式引用允许，server只校验形状；完整混源解析结果仍是本地内部模型，不能擅自扩充Box packages为跨registry全集。

客户端下载前内部观测checksum.value=null/provenance=none/trust=unverified，不可离线执行；下载后写真实64位SHA256，首次观测为client/local-observed/hashVerified=false。完整离线执行要求全部artifact字节、依赖闭包与已知可用状态均校验，不能用根wrapper审核代替外源检查。

#### 内部模型转换（不另宣称共享v1）

| wire字段 | 客户端内部字段/转换 |
| --- | --- |
| compatibility.game/gameVersion | game.id/version；null原样保留，同时保留platform/loader/loaderVersion，不能丢失。 |
| roots: releaseId[] | roots映射到packages中的完整身份；单根页面可取root，不能在多根安装时只保留第一个。 |
| packages[] | components[]；补顶层registryId和profile上下文，provider保留，community从capabilities/adapter上下文取得。 |
| packages[].dependencies: releaseId[] | 在同registry packages索引解析为完整组件引用，缺失/重复/循环即失败，不跨源补齐。 |
| packages[].artifacts[].checksum | 每artifact保留algorithm/value/provenance/hashVerified；不能直接收缩成包级checksum。 |
| lock.externalDependencies / RegistryPlan.externalDependencies | 同一发布版本引用必须一致；分派到client adapter解析后加入本地components，不反向写入Box packages。 |
| configOverrides/localPayloads/locator | 原样保留已校验的结构；下载URL来自downloads票据，不能写入lock。 |

本地模型加profileId/inventoryDigest/environmentRevision/operationId/planDigest，均不发送为Box wire lock字段；内部持久化使用独立格式名和localModelVersion，不使用共享schemaVersion=1。反向构造Box lock只保留当前registry的Box packages，外源在externalDependencies，含外源trust=mixed，不能替换成Box身份。最终capabilities.serverCrossSourceResolution=false仅表示server不解算跨源，externalDependencyReferences=true表示允许保存显式引用；不能据前者阻断正常整合包分享。

externalDependencies按server ExternalDependencyRef字段：provider=thunderstore、registryId=thunderstore-public、community、namespace、name、version、upstreamPackageId/upstreamReleaseId、checksum/provenance/lastCheckedAt。checksum可为null，未知provenance=none/lastCheckedAt=null，不冒充核验；禁止自由下载URL。有UUID时保留并核对；缺UUID由固定namespace/name/version解析，不因缺UUID本身强制作者手填，解析失败或hash冲突才阻断。client外源解析不构成server源联动。

`locator` 不保存临时 URL；box 下载 ticket 带 expiresAt，Thunderstore 地址由其 adapter 查询；ticket 不进入分享锁。hashVerified 只表示预期字节匹配，不是安全保证。已知 quarantined 永远阻断，离线状态未知必须显示最后检查时间和最后已知状态，不声称在线确认；下架不会自动删除现有玩家文件。

### 6.3 安装计划与 Wails/API 边界

建议新增（名称为设计建议，不代表当前存在）：

- `Marketplace.ListPackages({provider,...})`、`Marketplace.GetPackage(provider,packageId)`、`Marketplace.GetRelease(provider,packageId,releaseId)`：Wails 后端按来源路由，公开读取、不登录；box API 只接收 box ID，Thunderstore 查询不经过盒子服务器。
- `Marketplace.ResolveInstallPlan(localIntent)`是Wails内部入口，映射到server ResolveRequest={roots:DependencyRef[],compatibility,mode,installed:DependencyRef[]}；profileId不发送，mode只取use-current/create-isolated。server返回RegistryPlan={planId,schemaVersion,state,registryId,catalogRevision,policyRevision,expiresAt,mode,lock,downloads,externalDependencies,warnings,blockers}。Thunderstore adapter独立解析；两源进入本地内部模型，不声称其接口形状相同或把root/components/game直接传server。
- `Installer.BuildInstallPlan(lock, localInventoryDigest, environmentRevision)`：客户端结合本地游戏目录、BepInEx 状态、文件哈希和进程状态生成 diff/conflicts/backup plan。
- `Installer.Execute(planId, planDigest)`：客户端 CAS 复核后落盘；服务端不得远程写本地环境。复用现有 Wails 进度事件和冲突 DTO，增加 `planDigest`、profileId、releaseId。
- `Upload.Init/Chunk/Complete/Publish`：只发布到 box，不接触 Thunderstore 账号/发布 API；login 在最终发布动作触发、实际 init/chunk 前完成；支持草稿续传、previewDigest、publishIntent、扫描与审核状态。

客户端错误/事件DTO至少含operationId/profileId/provider/registryId/community/packageId/releaseId/state/stage/sequence/bytesDone/bytesTotal/errorCode/retryable/planDigest。此为Wails内部事件，不替代server ErrorDTO/Issue。现有event name兼容保留，operationId防多profile串状态，乱序sequence不覆盖新状态；服务端Issue的errorCode/message/dependencyPath/path/action原样映射，不靠msg猜分类。最后本地状态以operation snapshot为准。

账号仍走 `AccountService`，token 不进入 DTO、lock、configOverrides 或上传日志。所有状态改变写通用 operation log 的 `namespace/module/event/level/type`，具体 release、profile、digest 放 metadata。

## 7. zip、manifest、BepInEx 与配置规则

1. zip 必须拒绝路径穿越、绝对路径、符号链接或超过大小/条目限制；现有 `fileops.ExtractZipFile` 已做分隔符清洗和目标根校验（`helper/modDependence/fileops/zip.go:30-70`），新上传扫描仍需独立限制。
2. Thunderstore标准包要求根manifest.json/README.md/icon.png。盒子自动生成相同形状：清单来自识别和表单，README由说明生成，icon用默认图或自动裁为256x256，能自动补齐的不阻断。标准dependencies仍是exact Team-Name-Version，结构化引用放独立dsbox manifest/lock，显式外源放externalDependencies。真损坏文件或非法版本在原表单提示修复，不要求作者手改JSON。
3. BepInEx 规则沿用 `helper/modDependence/installer/artifactlayout/resolve.go:28-81`：`plugins/core/patchers/monomod` 按组件目录受管，`config` 不分组件且不纳入普通文件清单；整合包必须在预览中显示此差异，避免用户误以为所有 config 可安全合并。
4. 只把 manifest 白名单键和 schema 已知可分享的 config override 导出；过滤 token、cookie、账号 ID、绝对路径、机器名、日志和存档。黑名单不能证明未知键安全，未知配置默认排除而非默认上传。override 必须绑定对应 release、相对文件路径和键；已有配置保存副本，版本变化时只合并已知键，不全文件覆盖。若作者坚持包含未识别配置则阻塞远端发布，给移除/仅本地导出动作。
5. package/release元数据、lock、配置覆盖和localPayloads分开存储。localPayloads自动计算SHA256/相对映射并关联本次发布声明；未知上游来源不阻止作者发布自己的代码，作为box新payload审核。真实权利/格式问题原表单修复，正常不逐文件问答。

LocalPayload最终wire字段为path/sha256/bytes/license/rightsMode/rightsEvidence；rightsMode=self-authored时由账户归属、署名和统一发布条款登记，rightsEvidence可null，不逐DLL索要授权材料；rightsMode=redistributed才需要相应分发证据。已知第三方包优先自动转依赖/外源引用，真实再分发问题原表单内修复，不增加自有代码发布步骤。

### 7.1 依赖及兼容性决策

| 检查 | 默认动作 |
| --- | --- |
| 普通 Thunderstore Mod 声明版本 | 保留声明，按便捷策略记录实际版本及理由；无兼容证据显示未验证，已知不兼容阻断，不声称满足原始声明。 |
| 社区依赖、整合包/锁回放精确版本 | 按明确来源的 namespace/name/version 或不透明releaseId定位；不取latest替代，不因已装同名而跳过版本检查。 |
| 严格闭包两条边锁定不同版本 | 返回 DEPENDENCY_CONFLICT 与依赖链；box 作者发布前修复，普通玩家只见不可安装原因。当前Modpacks已有key冲突检查，普通便捷模式仅先到去重。 |
| 循环/缺失/深度超限 | 目标为显式解析失败，不把visited截断当作解决；写入前校验完整闭包。当前循环只去重跳过，深度上限/完整错误仍待实施。 |
| BepInEx 缺失 | 预检自动纳入框架安装动作，无需单独点安装；框架只由可信内置适配器安装，普通包不得覆盖 Doorstop/core。 |
| BepInEx 版本不满足声明 | 新 profile 按锁安装；已有框架重大替换需显示受影响组件并确认；不能用当前 service.Update 的“已装即跳过”当版本满足。 |
| 游戏版本/架构/Unity 后端已知不匹配 | 阻断，注明证据来源与实测范围；未知显示“未验证”，不把依赖 BepInEx 等同 DSP 或 Nebula 联机兼容保证。 |
| 两源包虽身份不同但加载 GUID 或目标路径冲突 | 本地计划报告物理冲突并预览，来源隔离不意味着运行时可共存；同 profile 每个逻辑组件只启用一个版本。 |

本地计划先解析闭包、下载/限额解包到 staging 并计算所有目标文件哈希，再完整预检，最后备份和提交；不能边复制边首次发现危险合并。进程占用或 environmentRevision 改变则返回 PLAN_STALE/PROCESS_BUSY，重新预检后保留可复用选择。新 profile 安装失败保留可重试的失败记录，清理仅此次 staging；原 profile 更新失败先恢复文件、配置和归属记录，回滚本身失败则标 recovery-required 并阻断启动。

## 8. 实施阶段与验收情景

### 阶段

1. **独立源与兼容基础**：新增 box adapter 对接自维护目录/详情/发布，默认盒子源；保留独立 Thunderstore adapter，补 UUID、manifest 校验、checksum provenance、过期缓存提示、legacy key 迁移。盒子服务端不承担 Thunderstore 联动。
2. **可复现安装**：区分普通Thunderstore便捷模式与strict lock模式，新增声明/实际版本映射和持久实际闭包；实现wire转换、精确release解析、完整差异计划及绑定摘要确认、本地inventory/CAS、原子更新与回滚；复用V2恢复证据、共享引用、版本替换与progress。
3. **profile**：设计独立 profile root、BepInEx/插件目标、启动参数和 profile 状态；在此阶段完成前，整合包只能明确标记“合并当前环境”或导出 lock，不能承诺新环境一键启动。
4. **作者工作流**：同一表单完成单Mod/多Mod采集与payload/manifest/lock生成、草稿、分片上传、previewDigest、首版校验审核和自动通知；基础交付包含用户上传自己的模组及整合包，不能只交付manifest-only引用包。r2modman/profile格式另行验证。
5. **离线与运维**：完整 lock/cache 离线复用、stale/quarantined 状态、下架提示、恢复诊断和可审计日志。

### 可验收情景

1. 未登录玩家搜索“物流”，能在 3 个动作内打开详情并安装；无冲突不弹确认。
2. 首次未配置游戏目录时，安装只引导选择一次目录；取消不写配置、不写文件。
3. 整合包安装默认新 profile；若 profile 能力未启用，按钮明确阻断并给出原因，不改当前环境。
4. 已有环境出现一个非托管 DLL 冲突时，预览显示路径、哈希和覆盖后果；用户取消后目标文件和安装记录均不变。
5. 更新下载中断后重启应用，页面显示可继续/重试/回滚；不会自动重新安装或删除未知文件。
6. Thunderstore 无 hash 且盒子服务器离线时，普通首装可直连并记录 client-observed，本地分享不访问盒子；盒子包只有盒子 source 状态，不回退到 Thunderstore 同名包。
7. 缓存过期且无 lock 时，市场显示“无法刷新”，不把旧列表标成最新；有完整 lock/cache 时可离线安装。
8. 拖入三个ZIP和作者自己的裸DLL，自动生成manifest/lock，无icon用默认图；填名称版本说明后一次发布，不额外预览/确认或选依赖。
9. 分享环境包含token配置/绝对路径时后台默认排除，摘要同表单只读；无上游来源的自有DLL进入payload审核，正常发布只填信息，真实错误才修复。
10. 盒子管理员将 box release 标为 quarantined 后，已知状态阻断新安装和离线复用，已安装实例显示告警；修复只能新 release 提供。盒子下架不改变 Thunderstore adapter 的状态。
11. 相同 Owner-Name-Version 在两源各有一个包，前端、cache、组件 ownership、卸载均用 provider+releaseId 区分，绝不相互覆盖身份。
12. 未限定依赖 owner-X-1.0.0 在盒子源缺失、Thunderstore存在时仍返回DEPENDENCY_MISSING，不隐式补齐；若整合包显式externalDependencies引用该Thunderstore版本，server只保存形状、client自动解析下载，玩家免选源和处理依赖，作者无需重上传第三方DLL。

## 9. 决策记录与待确认项

- **2026-10-01 用户澄清，替代早期讨论**：盒子市场是独立自维护源，不做server↔Thunderstore联动；默认盒子列表、Thunderstore次级入口，客户端共用DTO/安装器。取消server聚合/抓取/镜像；整合包允许显式externalDependencies由client解析，不隐式转换身份，不要求作者重传第三方DLL。
- **用户最终简化要求**：单页发布，上传相关模组+填写信息2阶段，点击发布后后台完成其余工作；无专门预览确认/逐依赖选源/手工JSON/逐文件授权向导。危险合并安装仍须确认，不影响作者正常发布流程。
- **2026-10-01 协调者获用户授权后的裁决**：普通Thunderstore保留便捷策略，社区依赖/整合包/锁回放严格精确；锁导出固定实际闭包。多独立Mod目录默认一个pack，多DLL不等于多Mod；首版上传自动检查后审核，通过后自动发布通知。危险覆盖、其它Mod影响和配置改写须计划聚合确认，当前dirty默认覆盖不是目标能力。
- **后续wire复核**：唯一共享v1锁为server roots/packages/compatibility及嵌套artifact.checksum.hashVerified；旧root/components/game仅本地模型，转换见6.2。profileId/inventoryDigest等不进入Box wire，显式外源不被同registry releaseId[]隐式编码。
- **已决定**：provider=box/thunderstore/local；安装身份provider/registryId/community/packageId/releaseId/profileId；不可变releaseId；server解box图并保存显式外源引用，最终本地plan+CAS；新profile安装1次意图；标准manifest pack MVP，profile/sharecode延后。
- **已决定**：Thunderstore首装直连记录client-observed；box只对自身上传字节形成server-verified，外源引用逐项标client-observed/unverified；server不抓第三方源。hash mismatch/已知quarantined阻断；wrapper扫描审核不继承给外源。
- **实施参数及上线门槛**：profile root/启动隔离未完成前只提供本地 lock 导出与明确授权的合并，隔离安装显示暂不支持；box install-plan 字段以联调契约为准。本地 payload/解压/上传大小上限和重试限流由实现配置提供，客户端读取 capabilities 展示，不改变来源边界和最短流程。
- **不得宣称**：官方账号代发、已支持 r2modman profile/share code、Thunderstore 官方安全签名、当前 environment 已具备多 profile 启动、当前更新已原子可回滚。

### 9.1 双 agent 质疑与回应

| 发起者 | 具体质疑 | 回应及最终处理 |
| --- | --- | --- |
| client | namespace 可伪造？同版本能覆盖？ | server 同意账号/团队绑定 box namespace、不可变 release；仅 hash blob 去重，不合并包身份。 |
| server | 分享只有 dependencies 不能复现配置和本地 Mod | client接受configOverrides/localPayloads，默认排除个人路径/日志/存档；用户最终要求下改为单页自动采集，无额外预览。未知自有DLL走payload审核，仅真实权利问题原表单修复。 |
| client | server 看不到本地哈希/进程，远端计划不能授权写入 | server 接受 resolution 与本地 plan 分层，执行时 inventoryDigest/environmentRevision/planDigest CAS。 |
| client | 强制 server 抓 Thunderstore hash 会扩大服务器故障范围 | server 接受 TS 直连 client-observed；用户进一步澄清独立源后完全取消 server↔TS 抓取，仅保留 box 自身校验。 |
| 用户 | 我们维护另一个源，只兼容、不联动 | 本稿据此重写来源边界、同源解析、上传归属和12个验收情景，并同步给 server/协调者；早期聚合提案已撤销。 |
| 用户 | 上传相关模组、填写信息即可发布 | 单页2任务阶段/3动作，自动生成manifest/lock/README/icon，后台完成依赖和审核，无正常二次确认，已同步双端。 |

本文为文档设计，未修改业务代码。本次基线复核执行 `go test ./helper/modDependence/installer/v2/...` 成功，但所有包为 `no test files`，仅有编译检查证据；未运行frontend/Go全量build或桌面行为验证。实现阶段需验证图循环/精确版本冲突、恶意zip、同名不同provider、CAS竞态、进程占用、取消/重启恢复、config三方合并与profile启动，不能以编译成功代替行为验证。当前dirty差异见[现状报告](mod-market-client-current-delta.md)。
