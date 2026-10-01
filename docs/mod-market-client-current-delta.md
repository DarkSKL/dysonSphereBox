# 客户端模组市场当前实现差异报告

日期：2026-10-01，16:58 起复核（UTC+8）；基准 HEAD `1ff3533824b14029fbe9aa8881e5e0a692753f37`。范围：当前 `develop/client` dirty 工作区、CodeGraph 当前源码和 `../docs/mod-market-design.md`，不修改业务代码；协调者后续授权同步客户端设计稿。本报告把事实分为“已实现”“编译检查通过”“未验证/缺能力”，当前磁盘与diff优先于历史摘要。下文短路径以 `helper/modDependence/installer/v2/` 为基准，`marketplace/` 则以 `helper/modDependence/` 为基准；完整路径另行标出。

## 1. 结论先行

- 当前 V2 安装器可作为 Thunderstore 安装基础：按 Owner-Name 识别组件，普通 Mod 依赖优先复用本地已安装版本、缺失时取最新 active；Modpacks 才按清单 key 取精确版本。这个普通 Mod 策略是产品选择，不是严格 exact 兼容。
- 已有实际传递依赖边（`Manifest.ResolvedDependencies`）用于共享引用和版本替换，但它不是可供玩家回放的跨源锁文件；声明版本与实际选中版本没有一份面向产品的显式映射 DTO。整合包必须另行实现严格 lock 入口，不能依靠普通 Mod 解析分支。
- 取消、失败和进程中断有会话 manifest、备份、清理和旧包快照恢复；但当前代码仍不能宣称完整原子更新、profile 隔离或危险合并预览。尤其 dirty diff 中冲突动作已改为默认按安装顺序覆盖，和总方案的“危险合并需预览确认”存在明确差距。

## 2. 与总方案的差异

| 总方案要求 | 当前基线 | 结论 |
| --- | --- | --- |
| 普通 Thunderstore Mod 省步骤安装 | `artifactresolve/resolve.go:21-64,67-136`：已装依赖复用，未装依赖 `ResolveLatestRemotePackage`；`marketplace/provider.go:42-63` 取 active 中创建时间最新者 | 已实现便捷策略；必须标注“未严格锁定” |
| 整合包/锁回放严格 exact | 仅 `hasModpackCategory` 在 `resolve.go:139-149` 控制 Modpacks；依赖冲突仅比较同 Owner-Name 的 key | 部分实现；缺独立 lock/profile 模式与全闭包验证 |
| 共享 wire lock | server v1 是 `roots/packages/compatibility/artifacts[].checksum`；客户端 `sessionstate.Manifest` 是本地内部会话模型 | 两者不可同称共享 v1；需要转换层 |
| 危险合并先预览再写入 | dirty `copyfiles/conflict.go:18-37` 移除了 `ResolveDecision`，冲突现在直接覆盖，非受管/受管文件均按安装顺序处理；`config` 不触发冲突检测 | 当前行为不满足总方案，不能宣称已完成安全合并 |
| 新 profile 独立安装与启动 | `install_prepare.go:21-33` 仍以单一 `gameDir` 和缓存目录承载安装；BepInEx 服务面向已配置游戏目录 | 未实现；未完成前只能导出 lock 或明确授权合并 |
| 完整原子更新/回滚 | `versionreplace/prepare.go:30-106` 会备份并记录旧版本文件，`installstore/package.go:21-60` 在事务中提交替换 | 有恢复证据和事务提交，但不是文件系统级原子交换；不能承诺完整原子回滚 |

## 3. 普通 Mod 与 Modpack 的版本、依赖和冲突

### 3.1 版本策略

`ResolveInstallArtifactsWithContext`（`helper/modDependence/installer/v2/artifactresolve/resolve.go:21-64`）按来源分派。Thunderstore 包先读取已安装组件快照（`:37-40`），根包若同版本已装则复用（`:54-58`）。递归依赖在 `:67-136`：

1. 依赖缺少 namespace/name 时从 `thunderstore:owner:name:version` key 回退（`:78-84,160-169`）。
2. 以小写 Owner-Name 作为 identity（`:86-93,152-157`），同 identity 已访问时去重。
3. 已安装组件优先复用，并递归读取它保存的依赖树，避免远端新清单替换本地共享关系（`:94-114`）。
4. 普通 Mod 缺失依赖调用 `ResolveLatestRemotePackage`（`:117-123`）；该函数只从 active 版本选创建时间最新者（`marketplace/provider.go:42-63`、`thunderstoremeta/version.go:14-31`）。
5. Modpacks 调用 `ResolveRemotePackage` 使用清单 key（`resolve.go:117-123`），同一 Owner-Name 出现不同 key 返回冲突（`:87-90`）。

因此，普通 Mod 的声明依赖（例如 `Team-Name-Version`）与实际选中的本地/最新版本可能不同；现有 `Artifact.Version`、数据库组件 `Version` 和 `Raw` 能保留实际版本，但没有向用户或共享 wire 暴露“声明 -> 实际”映射。Modpack 有版本冲突检查，但 visited 重复命中本身是去重，循环不会被明确报告为循环错误；当前应标为“未验证/缺严格闭包错误”。

双记录是部分基础而非完整能力：`artifact/convert.go:19-33` 保留选中版本自身的原始依赖声明；`installstore/store.go:33-46` 将它存入组件Raw和Version，`installmanifest/planned.go:42-63` 把依赖身份映射到实际component key。但最终完成在 `installstore/package.go:40-55` 清空manifest，缺持久的逐边声明/实际/选择理由和兼容结论。导出必须从稳定包组件关系、实际版本Raw和文件记录重建闭包并验证，不能直接导出已经清空的会话。

### 3.2 传递依赖归属和共享卸载

- `installmanifest/planned.go:42-63` 把 artifact 队列转成 `ResolvedDependencies`，并要求每条非 BepInEx 依赖在已解析队列中存在（`:49-58`）。这是真实安装闭包边，供后续共享迁移使用。
- `installstore/components.go:17-48` 按 Owner-Name 读取稳定安装组件，历史重复版本取最近更新记录；`installstore/package.go:21-60` 保存完成状态并调用版本替换提交。
- `installstore/version.go:23-90` 在事务内迁移旧组件入口、合并包组件关系、展开 `resolvedDependencyClosure`（`:93-110`），再删除旧组件记录。该闭包用于引用归属，不等同 server lock。
- 卸载 `uninstall/entry/uninstall.go:24-99` 先 `BuildPlan`，共享文件跳过物理删除（`:70-77`），事务中释放当前包记录并删除无引用组件（`:80-94`）。这支持共享依赖安全卸载，但只基于当前 installed 表和文件清单。
- `uninstall/records.go:17-39` 在原安装包卸载时把共享组件文件记录移交给仍引用的包，防止“文件留下、最后卸载时清单丢失”；共享判断还使用组件引用和目标路径记录（`uninstall/shared.go:39-63`）。文件删除先于记录事务，卸载本身不是整体可回滚事务（`uninstall/entry/uninstall.go:70-95`）。

### 3.3 循环、缺失和版本替换

- 空 key/BepInEx key 在 `resolve.go:71-76` 跳过；无法解析 identity 返回错误（`:78-84`）。
- 普通策略遇到同 Owner-Name 的不同版本会因 visited 去重而选先到者，不能声称检测并解决所有版本冲突；Modpack 才在 `:87-90` 明确报冲突。
- 版本替换只对 Thunderstore、同 Owner-Name 旧组件执行（`versionreplace/prepare.go:30-37,46-65`），旧文件先备份、写入 manifest 后移除（`:68-105,109-149`）。`commitVersionReplacements` 对身份、映射链和闭包做事务校验（`installstore/version.go:27-66`）。这可复用为后续 exact lock，但不是现成严格解析器。

## 4. Owner-Name 目录和 BepInEx 特殊处理

- `artifactlayout/naming.go:13-24` 的 `InstallDirName` 优先 `ModName`；`ModName`（`:26-41`）优先 namespace-name，缺失时解析 Thunderstore legacy key（`:43-52`），再回退 display/name/source ID/component key。
- `artifactlayout/resolve.go:28-35` 规则为：`BepInEx/plugins`（默认 `.dll`，按 Mod 子目录）、`core`、`patchers`、`monomod`（按 Mod 子目录）、`config`（不分 Mod 子目录）。`Target.Tracked` 在 `:37-45` 说明：config 复制但 `Tracked=false`，因此不进入普通受管文件清单。
- 物理目标由 `resolve.go:48-62,124-135` 生成；plugins/core/patchers/monomod 的目录名是 Owner-Name 作用域。多 DLL 不等于多个 Mod；若输入目录无法稳定得到 namespace/name，只能回退目录名，不能声称正确归属。
- BepInEx 前置由 `helper/modDependence/bepinex/install/install.go:22-86` 处理：配置游戏目录、已有安装时修复/确认 Doorstop，否则下载并解压专用前置包。依赖key在 `artifactresolve/resolve.go:75-76` 跳过，实际前置走独立路径；这不等于满足声明的BepInEx版本，现状也未禁止普通包通过core布局写入框架目录。BepInEx复制与Doorstop修复未纳入Mod会话artifact恢复证据，取消失败不能保证回到安装前框架状态。

## 5. 卸载、失败、取消、恢复、锁和进度

- 操作锁：`operationguard/guard.go:12-21` 进程内 `sync.Mutex.TryLock` 串行化安装/卸载；忙时立即返回错误，不排队，也不跨进程隔离。
- 安装主入口：`installerV2.go:17-67` 先取得锁、恢复旧会话、注册取消句柄，再准备 BepInEx、解析 artifact、逐文件安装和完成提交；失败走 `CleanupFailed`。
- 文件证据：`sessionstate/manifest.go:9-45` 记录计划组件、依赖边、受管/非受管文件、旧版本删除备份、previous package/component 快照；`copyfiles/step.go:15-54` 复制前保存备份和 manifest，复制后再检查取消。
- 失败/取消：`sessioncleanup/entry/cleanup.go:33-126` 读取 manifest，逆序恢复文件、恢复旧包快照，并标记 failed；`sessioncleanup/files/files.go:18-67` 对共享文件只删本次记录不删物理文件。应用重启时 `progress_snapshot.go:19-59` 只处理 running，会清理后显示 failed；conflict 状态保留给用户继续决策。
- 进度：事件名为 `modDependence:installProgress`（`constant/modDependence/install/event.go:7-11`）；`progress/event/event.go:17-126` 同时持久化和广播，前端 `useModPageState.ts:246-249,584-592,705-706` 恢复快照并监听事件，不自动重启安装。
- 关键限制：当前 dirty `copyfiles/conflict.go` 已默认覆盖冲突，config 又不参与冲突检测；因此总方案要求的危险合并预览、配置三方合并和文件系统原子交换仍未实现。

恢复证据的残余风险：文件写入和记录提交分阶段，游戏在中途可能看到混合版本；当前guard只覆盖已接入的入口且仅进程内有效（BepInEx服务也接入，`service/modDependence/bepInEx.go:29-76`），不能推定environment同步或另一盒子进程受保护。清理仍可能因权限、占用、备份丢失或数据库错误失败，并返回错误（`sessioncleanup/entry/cleanup.go:66-84,107-115`）。成功后会话/备份清除，不等于保存了用户可选历史回滚点。同版本主包复用只信稳定记录，不校验或修复用户删掉的文件（`install_reuse.go:19-36`）。这些均未做故障注入或桌面验证。

关键diff摘要（已比对HEAD；新增文件也直接读取）：旧 `ResolveDecision` 的覆盖/跳过/失败/暂停switch被删除，替换为“普通文件覆盖”；命名从显示名改为优先Owner-Name，布局扩到core/monomod并让config不受管；artifact新增Dependencies/ReuseExisting；计划新增ResolvedDependencies；新增versionreplace准备、共享闭包提交和卸载归属移交；入口新增TryLock和旧会话恢复；复制从写后追加证据改为写前持久化；完成由清备份后单独upsert改为事务提交版本映射后清备份。这些是源码差异，不是测试结论。

## 6. 裸 DLL、ZIP、多文件目录导入现状

`EnvironmentManagerService.ImportMod`（`service/environment/manager.go:555-612`）只把目录或 zip 导入 AppData/plugins 资源库，不直接写游戏目录。`prepareModImportSource`（`:1812-1843`）明确：目录可直接导入（`:1818-1820`）；裸 `.dll` 返回“第一版暂不支持”（`:1821-1823`）；zip 解压后调用 `discoverModRootFromExtractedZip`（`:1825-1842`）。识别逻辑（`:2627-2668`）可处理单个 plugins 子目录、单一根目录或根目录文件集，但多个 Mod 目录会报“请分别导入目录”（`:2644-2649`），并且不处理 patchers/config（`:2627-2630`）。

所以“作者一次拖入多个 ZIP、DLL、目录并填写信息发布”仍是目标发布采集流程，不是当前 ImportMod 能力；裸 DLL、跨目录多文件和完整 manifest/lock 采集需新增发布专用扫描，不应从 ImportMod 推断已支持。

## 7. 可复用能力与仍缺能力

**可复用（已实现）**：Thunderstore 直连 provider 与 latest-active 选择；Owner-Name 命名和 BepInEx 规则；artifact 递归队列与实际依赖边；文件哈希/冲突 DTO；会话 manifest、备份、失败清理、旧包快照；共享文件引用计数式卸载；版本替换事务；operation guard；取消和 progress snapshot。

**仍缺/未验证**：统一 server wire lock 到本地 `Manifest` 的明确转换；普通 Mod 的声明/实际版本双记录 DTO；跨源 externalDependencies adapter 闭包验证；循环和同 Owner-Name 多版本的严格错误；profile root/启动隔离；危险合并预览和 planDigest/CAS；文件系统级原子 swap；完整 ZIP 上传限制、符号链接/junction 拒绝、压缩炸弹/条目/总大小扫描；裸 DLL 与多 Mod 目录发布采集；发布审核状态和重试草稿。

## 8. 裁决记录、步骤与实施优先级

用户已回复“你来决定”，协调者已裁决，本轮无需重新向用户提问：普通TS保留便捷策略；整合包/分享锁定实际安装版本与闭包，未知或已知不兼容不能宣称满足声明；多独立Mod目录默认一个pack，多DLL不等于多Mod；首版自动检查后审核，批准自动公开通知；危险覆盖/影响其它Mod/配置改写聚合到同一计划摘要，一次确认绑定摘要，不逐文件弹窗。下列原需裁决的问题按此记录推荐与实施边界。

1. **普通 Mod 是否改 exact？** 推荐保留当前便捷策略，明确 UI 只显示“已选实际版本”，整合包/锁回放另走 exact；不要让普通玩家选择解析模式。
2. **profile 隔离完成前是否开放“安装到新环境”？** 推荐阻断该按钮，仅提供本地 lock 导出或危险合并；避免把 AppData 资源库误称独立运行环境。
3. **冲突是否默认覆盖？** 推荐将未来危险合并恢复为一次性预览确认；无冲突普通安装仍一步执行。当前 dirty 默认覆盖只能视为临时实现，不能作为最终 UX 契约。
4. **自有 DLL 如何发布？** 推荐默认 `self-authored` payload，统一条款登记即可，不逐 DLL 追问；检测出第三方再转显式外源引用或在原表单修复。
5. **外源解析失败怎么办？** 推荐阻断该 lock/release 并保留草稿或失败快照；不隐式换盒子源、取 latest 或要求普通玩家手工选源。

最短操作保持：玩家搜索→详情→安装为3动作，无冲突不加确认，公开浏览/安装不登录；目标整合包结果行→安装到新环境为1动作，但隔离未完成不得开放。作者添加文件→同表单填写名称/版本/说明→发布为3动作；首版审核自动公开，不追加“审核后发布”。分享环境→补信息→发布同为3动作，本地导出不登录。自有DLL不逐项授权；真实格式/归属/缺依赖问题原表单修复。社区源独立维护，server不抓取或联动TS，显式externalDependencies由client处理，身份仍需provider/registryId和本地profileId。

实施顺序建议：先补产品安装计划、声明/实际持久映射、严格解析模式和循环/冲突/兼容检查，聚合危险差异并确认；随后对接独立box源/wire转换、单表单上传采集与首版审核；整合包安装正式开放前完成profile启动隔离、完整锁回放、持久快照与故障恢复验证。部分ZIP安全能力已在 `helper/modDependence/fileops/zip.go:42-79` 实现路径越界检查和取消，但不能推定上传的条目/总大小/压缩比/符号链接/junction策略已完成。

## 9. 证据与验证口径

| 项目 | 证据 | 状态 |
| --- | --- | --- |
| 普通/Modpack 解析 | `helper/modDependence/installer/v2/artifactresolve/resolve.go:21-170` | 已实现，普通策略非 exact |
| latest active | `helper/modDependence/marketplace/provider.go:42-63`；`thunderstoremeta/version.go:14-31` | 已实现 |
| Owner-Name/BepInEx 布局 | `helper/modDependence/installer/artifactlayout/naming.go:13-65`；`resolve.go:28-135` | 已实现 |
| 共享卸载与版本替换 | `helper/modDependence/installer/v2/uninstall/entry/uninstall.go:24-99`；`helper/modDependence/installer/v2/installstore/version.go:23-110` | 已实现，范围限当前 installed 记录 |
| 失败/取消/重启恢复 | `helper/modDependence/installer/v2/sessioncleanup/entry/cleanup.go:33-126`；`helper/modDependence/installer/v2/progress_snapshot.go:19-59` | 已实现，未证明完整原子恢复 |
| 冲突处理 | dirty `helper/modDependence/installer/v2/copyfiles/conflict.go:18-37`；`helper/modDependence/installer/v2/copyconflict/inspect.go:21-77` | 已实现默认覆盖；与最终危险合并要求不一致 |
| 导入范围 | `service/environment/manager.go:555-612,1812-1843,2627-2668` | 已实现目录/部分 zip；裸 DLL 不支持 |
| 既有测试 | `go test ./helper/modDependence/installer/v2/...` | 命令成功，但所有包均为 `no test files`；不构成行为测试证据 |
| CodeGraph 指向的历史 lifecycle test | 当前磁盘 `rg --files -g '*test.go' helper/modDependence` 未找到对应文件 | 未验证，不能引用为当前测试覆盖 |

本次唯一现有检查为 `go test ./helper/modDependence/installer/v2/...`，exit 0，全为 `no test files`；没有已通过的行为测试可报告。CodeGraph历史索引列出的 `dependency_lifecycle_test.go` / `install_lifecycle_test.go` 当前磁盘不存在，已用文件枚举与Go输出核对。未测真实下载/游戏加载、共享升级卸载、取消重启/崩溃、完整原子恢复、profile、上传或审核，也未新增测试或重构。

交付新增本报告，并按协调者后续授权定向同步 `docs/mod-market-client-design.md`；没有修改业务代码、父级总方案、server 或 frontend，保留所有原dirty改动。
