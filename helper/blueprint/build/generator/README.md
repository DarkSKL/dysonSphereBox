# 蓝图工作流配置

公开生成入口只执行本包的YAML工作流。`factory.yaml`负责任务主链；`standard.yaml`与`dense.yaml`各自维护大模式、子模式参数、地理策略及工作链选择。`common.yaml`及各共享子模式库复用真实算法，输入、几何、逐项规则和结果整理分别有共享子流。

领域核心位于`../core/`。旧模式适配器注册、旧Generate/完整重试循环以及生产中的迁移组合桥已移除。工具使用稳定ID和Go反射白名单，所有相关状态由唯一任务实例及其受限候选/区域工作区维护。

默认使用嵌入配置。进程环境变量`DSBOX_BLUEPRINT_WORKFLOWS`可指定完整外部目录；相对路径基于可执行文件目录。新任务读取并冻结全套文件，在途任务不受后续修改影响；外部配置错误明确失败。目录最多64文件、单文件1 MiB、总4 MiB，不递归、不允许文件软链接。

参数层级为公共默认→大模式→子模式→地理覆盖，之后仍可由工具节点with显式覆盖。工具级合并为注册默认→当前库toolDefaults→with；数组整体替换，未知字段、非法类型和策略值拒绝。通过`$param`、`$input`和已登记的`$state`属性取值，不允许任意Go字段路径或脚本。

超密度按真实跨带许可、纬带类别、南北半球及经向环容量选择参数或链。当前请求没有绝对经度位置，因此不伪造经度坐标；具体规则见大模式文件。游戏原型尺寸、槽口、设备容量和物理校验不由配置随意覆盖。

共享几何链有11项实际变换，33项必需规则/度量逐项执行并产生同版本证明。`rules.complete@1`只汇合证明，不能补跑缺项；获选后区域汇合、唯一编码及统计/升档说明/结果组合也都是实际节点。

生产预算：16384展开节点、32层call、48层结构、4096累计候选、65536工具执行、最多10路计算、1 GiB候选保留状态、4分钟总时限。请求并发同时收紧实际工具槽和每组工作区派发，YAML的parallel是上限；自动模式对256台及以上计划采用单路，显式1–10仍尊重请求。局部优化时限不能延长父截止；精修通过includeCurrent保留完整已验证基线。

工具参数Schema由真实注册类型生成：

```powershell
go run ./cmd/blueprint-workflow-schema -out helper/blueprint/build/generator/schemes/workflow.schema.json
```

解析/严格解码/Schema使用yaml.v3、mapstructure/v2和jsonschema/v6；内容指纹的热点编码使用[goccy/go-json](https://github.com/goccy/go-json/tree/v0.12.0)与流式SHA-256，保持标准JSON字节兼容。版本以go.mod/go.sum为准。

现有算法实体预览保持独立。新增后台跟踪提供当前方案图、任务实际图和增量节点事件，通过已生成的Wails服务`DescribeWorkflow`、`GetWorkflowTask`、`ReadWorkflowEvents`调用。参数是JSON对象，时间是RFC3339字符串；跟踪不可用不阻断生成，丢失/淘汰与游标缺口明确报告。详见[后台接口](../../../factory/workflowjournal/README.md)。

完整方案、模块节点映射、验收与清理记录见[工作区方案目录](../../../../../docs/量化计算器重构方案/README.md)。测试在验收后按用户要求清理；验收记录保留准确输入、命令、结果和源码身份。离线检查不表示游戏内放置实测。
