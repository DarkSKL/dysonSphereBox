package generator

// 文件职责：定义编码后统计、运输说明与最终结果的独立产物键。
// 使用：主目录嵌入resultStages；边界：不可变中间句柄只读共享，公开最终Result必须强类型深复制。

import (
	core "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/workflow"
)

// resultArtifacts 职责：声明三个结果步骤的数据依赖；使用：目录及任务出口；边界：不是新生成入口，不保存当前任务结果。
type resultArtifacts struct {
	// statistics 职责：保存编码实体统计句柄；使用：组合前置；边界：不能单独作为可导出结果。
	statistics workflow.ArtifactKey[core.ResultStatistics]
	// transportUpgrade 职责：保存原始到实际档位说明；使用：组合前置；边界：没有升档也要完成此步骤。
	transportUpgrade workflow.ArtifactKey[core.ResultTransportUpgrade]
	// final 职责：保存强类型克隆的完整结果；使用：任务出口读取；边界：不再编码、不共享可写容器。
	final workflow.ArtifactKey[core.Result]
}

// newResultArtifacts 职责：登记编码后全部结果产物；使用：newArtifacts初始化；边界：报告不改变实体修订号，任一注册失败即中止。
func newResultArtifacts(all *[]workflow.Artifact) (resultArtifacts, error) {
	a := resultArtifacts{}
	var err error
	a.statistics, err = addImmutableArtifact(all, "result.statistics", func(v core.ResultStatistics) int64 { return v.EstimatedBytes() }, true)
	if err != nil {
		return a, err
	}
	a.transportUpgrade, err = addImmutableArtifact(all, "result.transport-upgrade", func(v core.ResultTransportUpgrade) int64 { return v.EstimatedBytes() }, true)
	if err != nil {
		return a, err
	}
	a.final, err = addArtifact(all, "result.completed", func(v core.Result) (core.Result, error) { return core.CloneResult(v), nil }, core.ResultEstimatedBytes, true)
	return a, err
}
