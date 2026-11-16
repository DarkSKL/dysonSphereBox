# 工作流后台跟踪接口

这是后台能力，当前没有新增工作流可视化页面。实体预览继续使用原 `GenerationContext` / `progressjournal` 协议；本文接口消费 `RunOptions.ObserveExecution` 的明确事件，不解析系统日志、不在生成结束后补造实时记录。

## 已绑定服务方法

`service/factory.Service` 已由 `ServiceGroup.RegisterServices` 注册到 Wails：

| 方法 | 用途 |
| --- | --- |
| `DescribeWorkflow()` | 查询**当前配置**的冻结图；不启动生成；返回 `Description` |
| `GetWorkflowTask(taskID)` | 查询任务状态和**该任务实际绑定**的冻结图；返回 `Snapshot` |
| `ReadWorkflowEvents(taskID, after, limit)` | 查询日志游标之后的事件；返回 `Page` |

生成仍使用现有 `Generate(request)`，通过原 `request.taskId` 关联。空 TaskID 的内部调用保持不登记行为；有效 TaskID 为非空 UTF-8、最多 128 字节且不含空字符、换行或制表符。客户端应为每次调用生成新 ID，已知 ID 重用返回 `GEN_E_TASK_CONFLICT`，在取消旧任务之前拒绝，因此不会混入旧预览或日志。

`Graph.fingerprint` 是引擎的全依赖指纹，包含实际冻结配置和工具契约/实现；配置变更只影响新装载计划。`DescribeWorkflow()` 不能替代历史任务图查询。

## 图与事件关联

- `Graph.nodes[].path` 是稳定的短定义 ID，`parentPath` 和 `children` 引用同一命名空间。`id` 是原方案局部 ID，`type/relation/call/source` 描述层级、分支、候选、区域及调用来源。
- `Graph.tools` 按稳定工具 ID 共享参数声明；节点 `tool/schemaID` 引用它。参数值、声明和来源只包含工具 `TraceFields` 显式白名单，默认不暴露参数。
- 页内每项是 `{ taskId, sequence, event }`。外层 `sequence` 是 journal 接收时锁内签发的严格递增游标；内层 `event.sequence` 保留引擎序号。分页必须使用外层序号或 `page.next`。
- `event.definitionPath` 对应 `Graph.nodes[].path`。`event.path/parentPath` 是真实动态执行路径，结合 `instanceID/candidate/region` 区分嵌套候选和区域。关联身份不截断；整条事件超限会拒绝并计丢失。
- 事件包含实际 started/completed/failed/selected/skipped 状态、参数及来源、前后版本、错误、分支选择、候选/区域/组活动、时间和工具诊断。候选 completed 不代表主任务成功，应读取任务终态。

## 生命周期与降级

服务在输入校验前 `Reserve(taskID)` 原子占用身份。实际引擎装载计划后，`WithWorkflowStart` 回调调用 `Writer.BindPlan`；事件直接调用 `Writer.Append`。引擎收尾钩子提供 `RunResult.ExecutionEventsLost`，服务按最终返回结果调用 `FinishWithLost`，因此核心成功后发生的产物存储失败也不会记录成任务成功。

任务状态为 `running/succeeded/failed/canceled/failed-before-start`。输入校验、等待取消或配置装载失败时，`FinishBeforeStart` 保存 `failed-before-start`，无图、无实体、无伪造引擎事件。预留阶段查询到 `running` 且 `trackingErrorCode=pending`，表示尚未装载实际计划。

`BindPlan` 图容量不足时保留实际指纹，`trackingAvailable=false` 和 `trackingErrorCode/Message` 明确说明不可用；业务生成继续，最终业务状态仍可查询。`Read` 返回 `tracking_unavailable`，不宣称过程完整。`StartUnavailable` 为其它调用方提供相同降级登记能力。

全部活动任务数量满员或 Store 已关闭时，服务允许原生成继续，跟踪查询分别返回 `not_found` 或 `closed`。有界存储无法保证在资源完全耗尽时为无限新任务继续登记元数据。此时没有伪造的完整跟踪承诺。

底层也支持独立 `Start(taskID, graph) -> *Writer`，方便内部生成调用及集成测试；与 `Reserve/BindPlan` 两种准入方式不能混用在同一 ID。Writer 是单次任务能力，完成、淘汰、关闭后不能写入；即使短期过期标记清理后 ID 被重新使用，旧 Writer 也无法污染新任务。

## 默认保留上限

| 项目 | 上限与策略 |
| --- | --- |
| 活动及已完成任务总数 | 8；满员优先移除最早登记的已结束任务，活动任务不会被挤走 |
| 每任务事件数 | 4096；超限淘汰最旧前缀 |
| 每任务图及事件 JSON | 8 MiB |
| 全部任务图及事件 JSON | 32 MiB；超限按任务登记顺序淘汰旧事件前缀，保留冻结图 |
| 单图 JSON | 4 MiB；整体拒绝，不能截断节点关系 |
| 单事件 JSON | 64 KiB；超限拒绝并计 `lostEvents` |
| 有效参数 | 16 KiB；超限整体省略并标记 `parametersTruncated` |
| 参数来源 | 最多 128 项，键/来源文本分别最多 128/256 字节 |
| 错误文本/码 | 最多 2048/128 字节，保持合法 UTF-8 |
| 工具诊断 | 最多 64 项、16 KiB，单项最多 2048 字节；截断明确标记 |
| 每页 | 1–128 项，越界直接拒绝；不隐式放大或钳制 |
| 结束后 TTL | 10 分钟；公开操作时惰性清理，读取不延期；活动任务由数量和字节预算约束 |
| 过期标记 | 最多 16 项，移除后再保留最多一个 TTL；超过数量或时间后回退 `not_found` |

字节预算统计保留的图和事件 JSON；固定元数据、最终有界错误及过期标记另由任务数、事件数、字段长度限制。输出在锁内反序列化深复制，不共享持久引用。淘汰和关闭会清空 Writer 可能仍持有的旧记录载荷。没有磁盘日志、清理协程或无界队列；`Close()` 幂等且永久关闭当前实例。

`droppedEvents` 是成功接收后因容量淘汰的条数，`lostEvents` 是 journal 拒绝/无法保存的条数，`engineEventsLost` 是引擎最终观察错误或 panic 数（含最终回调自身失败兜底）。后两者可能重叠，不能直接相加。`complete` 只表示任务结束且该页读至末尾，不代表任务成功或轨迹完整。

## 查询状态及最小示例

`Snapshot.code` 区分 `ok/not_found/expired/invalid_argument/closed`。`Page.code` 额外有 `cursor_gap/invalid_cursor/tracking_unavailable`：

- `after > lastSequence` 返回 `invalid_cursor`，拒绝超前或错误任务游标。
- `after < firstSequence - 1` 返回 `cursor_gap`，`gap=true`，同时返回当前可读的第一页；调用方明确提示缺口并继续使用 `next`。
- `not_found` 表示未知任务，或过期标记也已超过有界保留窗口；不承诺永久区分所有历史 ID。

以下方法已经生成Wails绑定，可直接调用，不需要新增页面。参数与Schema字段使用JSON对象类型，时间为RFC3339Nano字符串，未完成时间为空；不会把RawMessage误生成为number[]：

```ts
const snapshot = await GetWorkflowTask(taskId)
if (snapshot.code !== 'ok') throw new Error(snapshot.message)

const graph = snapshot.graph // 该任务的图；failed-before-start/不可用时为空
const page = await ReadWorkflowEvents(taskId, cursor, 64)
if (page.code === 'cursor_gap') showMissingPrefix(page.firstSequence)
if (page.code === 'ok' || page.code === 'cursor_gap') {
  for (const item of page.events) {
    renderExecutionEvent(item.event, graph) // definitionPath 对应 nodes[].path
  }
  cursor = page.next
}
// page.complete 时停止该轮询；业务成败读取 snapshot.state，完整性另检查 dropped/lost。
```

验收时有界日志的契约测试与真实生成/服务集成分别执行，均保存了结果。按用户要求，测试源在最终验收后清理；记录见工作区方案目录。
