package generator

// 文件职责：把单项几何转换接入事务化工作流。
// 使用：YAML显式排列定位、供电、绑定和冻结节点；边界：接收器无任务状态，不选择后继步骤。

import (
	core "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/workflow"
	"context"
)

// geometryTransform 职责：复用纯转换的读写边界；使用：每个稳定业务ID绑定固定函数及键；边界：函数是注册时固定的只读依赖。
type geometryTransform struct {
	input  workflow.ArtifactKey[core.GeometryDraft]
	output workflow.ArtifactKey[core.GeometryDraft]
	apply  func(context.Context, core.GeometryDraft) (core.GeometryDraft, error)
}

// Apply 职责：执行当前节点的唯一几何转换；使用：反射白名单调用；边界：成功后才暂存输出，失败不改输入。
func (t *geometryTransform) Apply(ctx context.Context, s *workflow.StepContext, _ emptyArgs) error {
	input, err := workflow.Read(s, t.input)
	if err != nil {
		return err
	}
	output, err := t.apply(ctx, input)
	if err != nil {
		return domainFailure(ctx, s, err)
	}
	return workflow.Stage(s, t.output, output)
}

// PositionGeometry 职责：把装配图定位到目标坐标；使用：几何子流首节点；边界：不运行补电、重定位或规则终检。
func (t *toolset) PositionGeometry(ctx context.Context, s *workflow.StepContext, _ emptyArgs) error {
	input, err := workflow.Read(s, t.keys.geometryStages.assembly)
	if err != nil {
		return err
	}
	output, err := core.PositionGeometry(ctx, input)
	if err != nil {
		return domainFailure(ctx, s, err)
	}
	return workflow.Stage(s, t.keys.geometryStages.positioned, output)
}

// FreezeGeometry 职责：冻结注解已完成的几何图；使用：几何子流末节点；边界：不授予校验或编码权限。
func (t *toolset) FreezeGeometry(ctx context.Context, s *workflow.StepContext, _ emptyArgs) error {
	input, err := workflow.Read(s, t.keys.geometryStages.ratesAnnotated)
	if err != nil {
		return err
	}
	output, err := core.FreezeGeometry(ctx, input)
	if err != nil {
		return domainFailure(ctx, s, err)
	}
	return workflow.Stage(s, t.keys.geometry, output)
}

// geometryToolSpecs 职责：把具名几何能力绑定到唯一纯实现；使用：工具目录组装；边界：这里声明权限，不编排执行顺序。
func geometryToolSpecs(a artifacts, implementation string) []workflow.ToolSpec {
	g := a.geometryStages
	t := &toolset{keys: a}
	endpoint := func(id, method string, read, write workflow.Artifact) workflow.ToolSpec {
		return workflow.ToolSpec{ID: id, Receiver: t, Method: method, Implementation: implementation, Reads: []workflow.Artifact{read}, Requires: []workflow.Artifact{read}, Consumes: []workflow.Artifact{read}, Writes: []workflow.Artifact{write}, Provides: []workflow.Artifact{write}, Role: workflow.Transform, FailureCodes: solutionFailures}
	}
	specs := []workflow.ToolSpec{endpoint("geometry.position@1", "PositionGeometry", g.assembly.Ref(), g.positioned.Ref()), endpoint("geometry.freeze@1", "FreezeGeometry", g.ratesAnnotated.Ref(), a.geometry.Ref())}
	transforms := []struct {
		id            string
		input, output workflow.ArtifactKey[core.GeometryDraft]
		apply         func(context.Context, core.GeometryDraft) (core.GeometryDraft, error)
	}{
		{"power.repair@1", g.positioned, g.powerRepaired, core.RepairGeometryPower},
		{"power.prune@1", g.powerRepaired, g.powerPruned, core.PruneGeometryPower},
		{"geometry.relocate@1", g.powerPruned, g.relocated, core.RelocateGeometry},
		{"monitors.align@1", g.relocated, g.monitorsAligned, core.AlignGeometryMonitors},
		{"cargo.bind@1", g.monitorsAligned, g.cargoBound, core.BindGeometryCargo},
		{"geometry.measure-range@1", g.cargoBound, g.rangeComputed, core.ComputeGeometryRange},
		{"transport.bind-records@1", g.rangeComputed, g.transportBound, core.BindGeometryTransport},
		{"transport.commit-priorities@1", g.transportBound, g.prioritiesCommitted, core.CommitGeometryPriorities},
		{"production.annotate-rates@1", g.prioritiesCommitted, g.ratesAnnotated, core.AnnotateGeometryRates},
	}
	for _, transform := range transforms {
		spec := endpoint(transform.id, "Apply", transform.input.Ref(), transform.output.Ref())
		spec.Receiver = &geometryTransform{input: transform.input, output: transform.output, apply: transform.apply}
		specs = append(specs, spec)
	}
	return specs
}
