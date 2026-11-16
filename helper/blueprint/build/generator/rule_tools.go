package generator

// 文件职责：为每项规则登记独立证据和工具，再由显式汇合节点批准。
// 使用：共享rules/validate子流程；边界：单项工具只执行当前规则，汇合不补跑缺失检查。

import (
	core "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/workflow"
	"context"
	"encoding/json"
)

// ruleArtifacts 职责：描述完整必需证据目录；使用：固定注册元数据；边界：目录只在初始化创建，不保存任务结果。
type ruleArtifacts struct {
	proofs map[core.RuleKind]workflow.ArtifactKey[core.RuleProof]
}

// newRuleArtifacts 职责：登记不可伪造的单项规则产物；使用：实例初始化；边界：每个规则独立键且诊断不推进实体版本。
func newRuleArtifacts(all *[]workflow.Artifact) (ruleArtifacts, error) {
	a := ruleArtifacts{proofs: make(map[core.RuleKind]workflow.ArtifactKey[core.RuleProof])}
	for _, kind := range core.RequiredRuleKinds() {
		key, err := addImmutableArtifact(all, "rule."+string(kind), func(v core.RuleProof) int64 { return v.EstimatedBytes() }, true)
		if err != nil {
			return a, err
		}
		a.proofs[kind] = key
	}
	return a, nil
}

// ruleTool 职责：绑定一个规则及其准确前置证据；使用：稳定业务ID的反射接收器；边界：无实例状态，不能自行选择下一规则。
type ruleTool struct {
	geometry     workflow.ArtifactKey[core.GeometrySnapshot]
	dependencies []workflow.ArtifactKey[core.RuleProof]
	output       workflow.ArtifactKey[core.RuleProof]
	check        func(context.Context, core.GeometrySnapshot, ...core.RuleProof) (core.RuleProof, error)
}

// Check 职责：执行本节点绑定的唯一规则；使用：各rules.*工具；边界：失败时不发布证据，不修改几何。
func (t *ruleTool) Check(ctx context.Context, s *workflow.StepContext, _ emptyArgs) error {
	geometry, err := workflow.Read(s, t.geometry)
	if err != nil {
		return err
	}
	proofs := make([]core.RuleProof, len(t.dependencies))
	for index, key := range t.dependencies {
		proof, err := workflow.Read(s, key)
		if err != nil {
			return err
		}
		proofs[index] = proof
	}
	proof, err := t.check(ctx, geometry, proofs...)
	if err != nil {
		return domainFailure(ctx, s, err)
	}
	parameters, err := json.Marshal(proof.Parameters())
	if err != nil {
		return err
	}
	if err = s.Diagnostic("rule-parameters:" + string(parameters)); err != nil {
		return err
	}
	return workflow.Stage(s, t.output, proof)
}

// CompleteRules 职责：汇合完整同版本规则证明；使用：候选最终批准；边界：不执行遗漏规则、不接受旧几何证明。
func (t *toolset) CompleteRules(ctx context.Context, s *workflow.StepContext, _ emptyArgs) error {
	geometry, err := workflow.Read(s, t.keys.geometry)
	if err != nil {
		return err
	}
	proofs := make([]core.RuleProof, 0, len(t.keys.ruleStages.proofs))
	for _, kind := range core.RequiredRuleKinds() {
		proof, err := workflow.Read(s, t.keys.ruleStages.proofs[kind])
		if err != nil {
			return err
		}
		proofs = append(proofs, proof)
	}
	verified, err := core.CompleteValidation(ctx, geometry, proofs...)
	if err != nil {
		return domainFailure(ctx, s, err)
	}
	return workflow.Stage(s, t.keys.verified, verified)
}

// ruleToolSpecs 职责：以唯一真实入口登记每项规则；使用：注册初始化；边界：循环只创建白名单，不在执行时隐藏规则链。
func ruleToolSpecs(a artifacts, implementation string) []workflow.ToolSpec {
	checks := map[core.RuleKind]func(context.Context, core.GeometrySnapshot, ...core.RuleProof) (core.RuleProof, error){
		core.RuleReferences: core.CheckReferences, core.RuleLatitude: core.CheckLatitude, core.RuleMachines: core.CheckMachines,
		core.RulePastedSlots: core.CheckPastedSlots, core.RuleBeltDistance: core.CheckBeltDistance, core.RuleStaticColliders: core.CheckStaticColliders,
		core.RuleTankPorts: core.CheckTankPorts, core.RulePowerSpacing: core.CheckPowerSpacing, core.RuleStationSpacing: core.CheckStationSpacing,
		core.RuleBeltCrossings: core.CheckBeltCrossings, core.RuleBeltBuildingOccupancy: core.CheckBeltBuildingOccupancy, core.RuleInserterGeometry: core.CheckInserterGeometry,
		core.RuleSplitterClearance: core.CheckSplitterClearance, core.RuleStationPorts: core.CheckStationPorts, core.RuleStorageRoof: core.CheckStorageRoof,
		core.RulePowerNetwork: core.CheckPowerNetwork, core.RuleSprayChannels: core.CheckSprayChannels, core.RuleTransportTopology: core.CheckTransportTopology,
		core.RuleByproductPriority: core.CheckByproductPriority, core.RuleBuffers: core.CheckBuffers, core.RuleDeliveries: core.CheckDeliveries,
		core.RuleProducerSplits: core.CheckProducerSplits, core.RuleMaterialNetworks: core.CheckMaterialNetworks, core.RuleRoutes: core.CheckRoutes,
		core.RuleRecycles: core.CheckRecycles, core.RuleSelfSpray: core.CheckSelfSpray, core.RuleLeftovers: core.CheckLeftovers,
		core.RuleSharedCapacity: core.CheckSharedCapacity, core.RuleConservation: core.CheckConservation, core.RuleBoundaries: core.CheckBoundaries,
		core.RuleMonitors: core.CheckMonitors, core.RuleLayoutMetrics: core.MeasureLayout, core.RuleDensityReport: core.MeasureDensity,
	}
	specs := make([]workflow.ToolSpec, 0, len(checks)+1)
	all := []workflow.Artifact{a.geometry.Ref()}
	for _, kind := range core.RequiredRuleKinds() {
		output := a.ruleStages.proofs[kind]
		all = append(all, output.Ref())
		receiver := &ruleTool{geometry: a.geometry, output: output, check: checks[kind]}
		reads := []workflow.Artifact{a.geometry.Ref()}
		for _, dependency := range core.RuleDependencies(kind) {
			key := a.ruleStages.proofs[dependency]
			receiver.dependencies = append(receiver.dependencies, key)
			reads = append(reads, key.Ref())
		}
		specs = append(specs, workflow.ToolSpec{ID: "rules." + string(kind) + "@1", Receiver: receiver, Method: "Check", Implementation: implementation, Reads: reads, Requires: reads, Writes: []workflow.Artifact{output.Ref()}, Provides: []workflow.Artifact{output.Ref()}, Role: workflow.Diagnostic, FailureCodes: solutionFailures})
	}
	// VerifiedSnapshot已经拥有几何及完整证明集合；汇合后消费中间槽，避免同一封闭对象被重复保留和计量。
	specs = append(specs, workflow.ToolSpec{ID: "rules.complete@1", Receiver: &toolset{keys: a}, Method: "CompleteRules", Implementation: implementation, Reads: all, Requires: all, Consumes: all, Writes: []workflow.Artifact{a.verified.Ref()}, Provides: []workflow.Artifact{a.verified.Ref()}, Role: workflow.Validate, FailureCodes: solutionFailures})
	return specs
}
