package generator

// 文件职责：声明编码后结果工具的稳定ID和最小产物权限。
// 使用：主工具目录追加resultToolSpecs；边界：只注册，不调度，不把统计当作新的几何验证。

import "DysonSphereBox/helper/blueprint/build/workflow"

// resultToolSpecs 职责：登记三个纯结果节点；使用：newRegistry追加；边界：Diagnostic产物和角色不改变已验证编码的内容修订号，不允许访问原始请求。
func resultToolSpecs(a artifacts, implementation string) []workflow.ToolSpec {
	t := &resultTools{keys: a}
	makeSpec := func(id, method string, reads []workflow.Artifact, write workflow.Artifact) workflow.ToolSpec {
		return workflow.ToolSpec{ID: id, Receiver: t, Method: method, Implementation: implementation, Reads: reads, Requires: reads, Writes: []workflow.Artifact{write}, Provides: []workflow.Artifact{write}, Role: workflow.Diagnostic}
	}
	return []workflow.ToolSpec{
		makeSpec("stats.calculate@1", "CalculateStatistics", []workflow.Artifact{a.input.Ref(), a.encoded.Ref()}, a.resultStages.statistics.Ref()),
		makeSpec("result.transport-upgrade@1", "CalculateTransportUpgrade", []workflow.Artifact{a.base.Ref(), a.input.Ref(), a.encoded.Ref()}, a.resultStages.transportUpgrade.Ref()),
		makeSpec("result.compose@1", "Compose", []workflow.Artifact{a.encoded.Ref(), a.resultStages.statistics.Ref(), a.resultStages.transportUpgrade.Ref()}, a.resultStages.final.Ref()),
	}
}
