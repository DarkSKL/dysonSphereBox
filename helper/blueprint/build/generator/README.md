# 蓝图工作流配置

公开生成入口只执行本包的YAML工作流。`factory.yaml`负责任务主链；`standard.yaml`与`dense.yaml`各自维护大模式、子模式参数、地理策略及工作链选择。`common.yaml`及各共享子模式库复用真实算法，输入、几何、逐项规则和结果整理分别有共享子流。

领域核心位于`../core/`。旧模式适配器注册、旧Generate/完整重试循环以及生产中的迁移组合桥已移除。工具使用稳定ID和Go反射白名单，所有相关状态由唯一任务实例及其受限候选/区域工作区维护。

默认使用嵌入配置。进程环境变量`DSBOX_BLUEPRINT_WORKFLOWS`可指定完整外部目录；相对路径基于可执行文件目录。新任务读取并冻结全套文件，在途任务不受后续修改影响；外部配置错误明确失败。目录最多64文件、单文件1 MiB、总4 MiB，不递归、不允许文件软链接。

参数层级为公共默认→大模式→子模式→地理覆盖，之后仍可由工具节点with显式覆盖。工具级合并为注册默认→当前库toolDefaults→with；数组整体替换，未知字段、非法类型和策略值拒绝。通过`$param`、`$input`和已登记的`$state`属性取值，不允许任意Go字段路径或脚本。

布局策略在 `schemes/input.yaml` 的 `parameters` 配置，由 `input.normalize@1` 读取并冻结到任务输入指纹。使用 `DSBOX_BLUEPRINT_WORKFLOWS` 指定完整外部目录时，新任务读取修改后的 YAML。旧方案的工具节点省略新增字段时沿用注册默认值。

| 参数 | 默认值 | 用途 |
| --- | --- | --- |
| `maxAspect` | 2.5 | 最终物理包络的长宽比上限，取有限值且至少为 1。 |
| `preferredAspect` | 1.4 | 全厂软目标，至少为 1。评分为面积乘以 `max(1, 实际比例 / preferredAspect)`。候选排序、密铺剪枝和最佳预览共用冻结值。 |
| `packingPreferredAspect` | 1.8 | 密铺粗排、微调和塔场装箱的局部长宽比软目标，至少为 1。 |
| `packingWireWeight` | 2 | 启用线长代价的装箱策略在形状代价上加上估算线长乘此权重。设为 0 可关闭线长加权。 |
| `heightScales` | `[1, 0.8, 1.25]` | 依次缩放理想列高并向上取整，末尾保留旧列高退路。 |
| `adjacentWindowScales` | `[1, 0.8, 1.2]` | 居中和偏移跨纬带窗口的首批行数倍率，向下取整后裁入许可范围，继续补齐扩展窗口和上限。 |

两组倍率均接受 1 至 16 个 (0, 16] 范围内的有限数值，同一行数只保留首次出现的位置。各模式原有的候选预算和排序仍生效。

跨模式搜索策略在 `schemes/factory.yaml` 的 `parameters.layoutSearch`。其中的大厂门槛影响原生布局分流、候选窗口和布局排序。自动并发仍由同文件的 `execution.largePlanMachines` 控制。原生窗口按列表顺序尝试，窄带只取放得下的行数。密铺摆位、塔场摆位和全局供电各自有单候选预算。新任务冻结本组参数，旧外部方案缺项沿用默认值。

直连布局的基础列高、普通共享行排序、精修、街道、分流、折行和容量扩展可在 `schemes/connected.yaml` 的 `toolDefaults.connected.enumerate@1` 中调整。就近入口与集中入口共用这些默认值。折行跨度由 `common.yaml` 的 `bentSpans` 提供。

超密度按真实跨带许可、纬带类别、南北半球及经向环容量选择参数或链。当前请求没有绝对经度位置，因此不伪造经度坐标；具体规则见大模式文件。游戏原型尺寸、槽口、设备容量和物理校验不由配置随意覆盖。

共享几何链有11项实际变换，33项必需规则/度量逐项执行并产生同版本证明。`rules.complete@1`只汇合证明，不能补跑缺项；获选后区域汇合、唯一编码及统计/升档说明/结果组合也都是实际节点。

任务资源策略在 `schemes/factory.yaml` 的 `parameters.execution` 中逐项覆盖。编译工作流时校验，创建根上下文和候选调度器时生效。

| 参数 | 默认值 | 用途 |
| --- | --- | --- |
| `largePlanMachines` / `largePlanParallel` | 256 / 1 | 自动模式达到机器数阈值后使用指定并发（1 至 10）。显式并发、旧协议预览限制和节点并发上限继续生效。 |
| `baseSeconds` / `machinesPerBand` | 240 / 2048 | 第一档机器数以内使用基础秒数。 |
| `extraSecondsPerBand` / `maxSeconds` | 90 / 600 | 每跨入新的一档增加一次预算，最多到总上限。增量设为 0 时使用固定时限。 |

基础时限和总上限均为 1 至 86400 秒，总上限至少为基础时限。父上下文取消或更早截止时优先终止。工作流仍限制 16384 展开节点、32 层 call、48 层结构、4096 累计候选、65536 次工具执行、最多 10 路计算及 1 GiB 候选保留状态。

工具参数Schema由真实注册类型生成：

```powershell
go run ./cmd/blueprint-workflow-schema -out helper/blueprint/build/generator/schemes/workflow.schema.json
```

解析/严格解码/Schema使用yaml.v3、mapstructure/v2和jsonschema/v6；内容指纹的热点编码使用[goccy/go-json](https://github.com/goccy/go-json/tree/v0.12.0)与流式SHA-256，保持标准JSON字节兼容。版本以go.mod/go.sum为准。

现有算法实体预览保持独立。新增后台跟踪提供当前方案图、任务实际图和增量节点事件，通过已生成的Wails服务`DescribeWorkflow`、`GetWorkflowTask`、`ReadWorkflowEvents`调用。参数是JSON对象，时间是RFC3339字符串；跟踪不可用不阻断生成，丢失/淘汰与游标缺口明确报告。详见[后台接口](../../../factory/workflowjournal/README.md)。

完整方案、模块节点映射、验收与清理记录见[工作区方案目录](../../../../../docs/量化计算器重构方案/README.md)。测试在验收后按用户要求清理；验收记录保留准确输入、命令、结果和源码身份。离线检查不表示游戏内放置实测。
