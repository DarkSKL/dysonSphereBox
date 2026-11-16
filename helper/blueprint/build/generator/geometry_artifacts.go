package generator

// 文件职责：为每项几何操作分配独立的强类型输入输出。
// 使用：工作流编译检查真实阶段顺序；边界：快照内部不可变，不靠完成布尔值推进流程。

import (
	core "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/workflow"
)

// geometryArtifacts 职责：描述几何步骤的产物契约；使用：反射工具目录；边界：只存键，不存任务数据。
type geometryArtifacts struct {
	assembly            workflow.ArtifactKey[core.AssemblySnapshot]
	positioned          workflow.ArtifactKey[core.GeometryDraft]
	powerRepaired       workflow.ArtifactKey[core.GeometryDraft]
	powerPruned         workflow.ArtifactKey[core.GeometryDraft]
	relocated           workflow.ArtifactKey[core.GeometryDraft]
	monitorsAligned     workflow.ArtifactKey[core.GeometryDraft]
	cargoBound          workflow.ArtifactKey[core.GeometryDraft]
	rangeComputed       workflow.ArtifactKey[core.GeometryDraft]
	transportBound      workflow.ArtifactKey[core.GeometryDraft]
	prioritiesCommitted workflow.ArtifactKey[core.GeometryDraft]
	ratesAnnotated      workflow.ArtifactKey[core.GeometryDraft]
}

// newGeometryArtifacts 职责：一次登记所有阶段产物；使用：实例目录初始化；边界：任一键错误即失败。
func newGeometryArtifacts(all *[]workflow.Artifact) (geometryArtifacts, error) {
	a := geometryArtifacts{}
	var err error
	a.assembly, err = addImmutableArtifact(all, "geometry.assembled", func(v core.AssemblySnapshot) int64 { return v.EstimatedBytes() }, false)
	if err != nil {
		return a, err
	}
	stages := []struct {
		name string
		key  *workflow.ArtifactKey[core.GeometryDraft]
	}{
		{"positioned", &a.positioned}, {"power-repaired", &a.powerRepaired}, {"power-pruned", &a.powerPruned},
		{"relocated", &a.relocated}, {"monitors-aligned", &a.monitorsAligned}, {"cargo-bound", &a.cargoBound},
		{"range-computed", &a.rangeComputed}, {"transport-bound", &a.transportBound}, {"priorities-committed", &a.prioritiesCommitted}, {"rates-annotated", &a.ratesAnnotated},
	}
	for _, stage := range stages {
		*stage.key, err = addImmutableArtifact(all, "geometry."+stage.name, func(v core.GeometryDraft) int64 { return v.EstimatedBytes() }, false)
		if err != nil {
			return a, err
		}
	}
	return a, nil
}
