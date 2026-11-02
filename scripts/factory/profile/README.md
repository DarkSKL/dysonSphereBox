# 蓝图生成资源测量

本命令重放验收包中的真实 `request.json`，补充总体方案 §14.3 的进程峰值内存、Go 累计分配和指定阶段取消延迟。每次调用独立进程运行一个请求，不读存档、不启动游戏、不改变引擎原有预算、候选顺序或几何规则。

## 编译与运行

在应用根目录执行；报告的父目录须已存在，报告文件只能首次创建。

```powershell
go build -o runtime-shots/factory-profile.exe ./scripts/factory/profile

& .\runtime-shots\factory-profile.exe `
  -request 'runtime-shots/factory-acceptance/20261102-completion-audit/four-target-240-auto/request.json' `
  -report 'runtime-shots/four-target-resource.json'

& .\runtime-shots\factory-profile.exe `
  -request 'runtime-shots/factory-acceptance/20261102-completion-audit/four-target-240-auto/request.json' `
  -report 'runtime-shots/four-target-cancel-routing.json' `
  -cancel-stage routing -cancel-delay 25ms
```

`-cancel-stage` 选取实际阶段键，如直连带的 `routing` 或共同的 `power`、`rules`。到达目标阶段后异步等待 `-cancel-delay` 再调用父 context 的取消函数。报告同时记录取消时最近阶段；短阶段可能已切换，须按 `stageAtIssue` 解释。未进入目标阶段、尚未触发定时器便已生成完成、生成错误、取消未被响应或返回半成品，均留下原始报告并以非零状态退出，不能当作取消验收通过。

正常返回码为 `0`，测量结果不满足条件为 `1`，命令参数、请求读取或报告写入错误为 `2`。`-timeout` 缺省五分钟，仅提供外层截止；引擎原有四分钟期限仍先行生效。命令仅接受公开模式 1–4 和平面/自动球面，强制开启本次请求的 `collectPerformance`，其它请求参数保持原值。可用真实请求中的 `emitBlueprintProgress` 单独评估预览开销。

## 证据口径

- `requestSHA256` 是原始请求文件内容的哈希；`blueprintSHA256` 仅在生成成功时填写，可与人工包蓝图哈希直接核对。
- `generationDurationMs` 是 `Generate` 调用的外层墙钟，包含其性能回调，不包含报告编码和写盘。内部 `performance` 保留请求全部候选的阶段、寻路、碰撞及未测量边界。
- Windows 的 `processMemoryAfter.peakWorkingSetBytes`、`peakCommitBytes` 直接读取操作系统高水位，范围从独立进程启动至生成后读数，包含运行时初始化、输入解析及观测成本。它们分别为驻留集峰值和提交量峰值，不是当前工作集、Go 堆峰值或请求独占内存。`processMemoryBefore` 提供起点参照，两次高水位不能相减当作请求峰值。
- `goMemory.totalAllocatedBytes` 是 `runtime.MemStats.TotalAlloc` 的调用区间增量；前后活跃堆和 GC 次数只作解释，不冒充峰值。测量不强制 GC、不更改 GOMAXPROCS。
- `cancellation.responseMs` 从真正调用 `cancel` 到 `Generate` 返回取单调时间差。正常生成不填取消数据，未发出的取消保留空值。单次或有限矩阵不能代表通用取消 SLA。
- 当前非 Windows 平台保留内存接口缺口及空值，仍可测量生成与取消；不以零数值伪装峰值已测。默认应用生成本身仍不测峰值，因此 `performance.unmeasured` 继续保留对应项；本命令的外层报告提供单独证据。
- 比较耗时应固定输入哈希、Go 版本、机器、运行条件及代码版本，独立进程串行重复多次。一次速度变化不能证明算法加速；不得通过少建筑、降流量或跳过终检取得更快数字。

Windows 接口与结构字段依据 Microsoft 官方定义：`GetProcessMemoryInfo`（`learn.microsoft.com/en-us/windows/win32/api/psapi/nf-psapi-getprocessmemoryinfo`）及 `PROCESS_MEMORY_COUNTERS`（`learn.microsoft.com/en-us/windows/win32/api/psapi/ns-psapi-process_memory_counters`）。实现仅从系统目录加载 `psapi.dll`，使用当前进程伪句柄；不需要管理员权限或额外依赖。

## 编译与静态检查

```powershell
go build ./scripts/factory/profile
go vet ./scripts/factory/profile
```

2026-11-02 按版本提交清理要求删除本目录的两个 `*_test.go`，不再保留 Go 回归入口。此前的真实生成、产物哈希、阶段取消、输入拒绝、报告禁止覆盖及 Windows 内存结构检查仅作为历史验证背景；当前编译和静态检查不等于这些行为已重新验证，也不证明游戏放置、机队吞吐或持续产能。
