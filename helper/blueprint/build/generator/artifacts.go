package generator

// 文件职责：定义生产工作流使用的强类型产物及复制边界。
// 使用：工具目录和任务实例共享键定义；边界：只有封闭不可变核心快照允许共享句柄，可变请求和候选列表必须深复制。

import (
	"encoding/json"
	"fmt"

	core "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/workflow"
)

// selection 职责：保存一个被执行的具名候选规格；使用：只读比较及诊断；边界：不持有装配器，不控制调度。
type selection struct {
	Family      string
	Connected   core.ConnectedCandidateSpec
	Station     core.StationCandidateSpec
	Distributor core.DistributorCandidateSpec
	Bus         core.BusCandidateSpec
}

// selectedFactory 职责：封闭获选验证产物；使用：最终发布和编码；边界：策略及其版本仍由candidate.specification槽保留，不重复携带可写规格。
type selectedFactory struct {
	Verified core.VerifiedSnapshot
}

// artifacts 职责：固定生产域键目录；使用：每个无状态工具接收器；边界：键只描述权限，不保存任务数据。
type artifacts struct {
	modeStages     modeArtifacts
	resultStages   resultArtifacts
	ruleStages     ruleArtifacts
	inputStages    inputArtifacts
	geometryStages geometryArtifacts
	request        workflow.ArtifactKey[core.Request]
	base           workflow.ArtifactKey[*core.PreparedInput]
	transport      workflow.ArtifactKey[*core.PreparedInput]
	input          workflow.ArtifactKey[*core.PreparedInput]
	transportSpecs workflow.ArtifactKey[[]core.TransportCandidateSpec]
	searchWindow   workflow.ArtifactKey[candidateWindowArgs]
	searchPasses   workflow.ArtifactKey[[]candidateWindowArgs]
	surfaceSpecs   workflow.ArtifactKey[[]core.SurfaceCandidateSpec]
	connected      workflow.ArtifactKey[[]core.ConnectedCandidateSpec]
	station        workflow.ArtifactKey[[]core.StationCandidateSpec]
	distributor    workflow.ArtifactKey[[]core.DistributorCandidateSpec]
	bus            workflow.ArtifactKey[[]core.BusCandidateSpec]
	choice         workflow.ArtifactKey[selection]
	geometry       workflow.ArtifactKey[core.GeometrySnapshot]
	verified       workflow.ArtifactKey[core.VerifiedSnapshot]
	encoded        workflow.ArtifactKey[core.EncodedSnapshot]
	selected       workflow.ArtifactKey[selectedFactory]
	refinement     workflow.ArtifactKey[[]core.StationCandidateSpec]
	regions        workflow.ArtifactKey[[]core.RegionDescriptor]
	regionReport   workflow.ArtifactKey[core.RegionValidation]
	approved       workflow.ArtifactKey[selectedFactory]
	all            []workflow.Artifact
}

// cloneValue 职责：复制纯值领域容器；使用：请求与公开规格列表；边界：不用于含私有证据/预览身份的封闭核心快照。
func cloneValue[T any](value T) (T, error) {
	var copied T
	raw, err := json.Marshal(value)
	if err != nil {
		return copied, err
	}
	err = json.Unmarshal(raw, &copied)
	return copied, err
}

// valueBytes 职责：保守估计公开容器保留内存；使用：引擎状态预算；边界：序列化失败以超大值失败关闭，不能谎报零成本。
func valueBytes[T any](value T) int64 {
	raw, err := json.Marshal(value)
	if err != nil {
		return 1 << 62
	}
	return int64(len(raw))*4 + 256
}

// addArtifact 职责：一次创建键并追加实例目录；使用：启动装配；边界：错误保留给调用者，不运行算法。
func addArtifact[T any](all *[]workflow.Artifact, name string, clone func(T) (T, error), size func(T) int64, diagnostic bool) (workflow.ArtifactKey[T], error) {
	key, err := workflow.NewArtifactKey(name, clone, size, workflow.ArtifactOptions{Diagnostic: diagnostic})
	if err == nil {
		*all = append(*all, key.Ref())
	}
	return key, err
}

// addImmutableArtifact 职责：注册封闭核心快照的共享克隆契约；使用：不可变几何与验证产物；边界：公开可写容器继续使用addArtifact深复制。
func addImmutableArtifact[T any](all *[]workflow.Artifact, name string, size func(T) int64, diagnostic bool) (workflow.ArtifactKey[T], error) {
	key, err := workflow.NewImmutableArtifactKey(name, size, workflow.ArtifactOptions{Diagnostic: diagnostic})
	if err == nil {
		*all = append(*all, key.Ref())
	}
	return key, err
}

// newArtifacts 职责：构建全生产域目录；使用：生成器初始化；边界：任一错误禁止构建不完整目录。
func newArtifacts() (artifacts, error) {
	a := artifacts{}
	var err error
	if a.modeStages, err = newModeArtifacts(&a.all); err != nil {
		return a, err
	}
	if a.resultStages, err = newResultArtifacts(&a.all); err != nil {
		return a, err
	}
	if a.ruleStages, err = newRuleArtifacts(&a.all); err != nil {
		return a, err
	}
	if a.inputStages, err = newInputArtifacts(&a.all); err != nil {
		return a, err
	}
	if a.geometryStages, err = newGeometryArtifacts(&a.all); err != nil {
		return a, err
	}
	if a.request, err = addArtifact(&a.all, "request", core.CloneRequest, core.RequestEstimatedBytes, false); err != nil {
		return a, err
	}
	immutableInput := func(v *core.PreparedInput) (*core.PreparedInput, error) {
		if v == nil {
			return nil, fmt.Errorf("empty prepared input")
		}
		return v, nil
	}
	inputSize := func(v *core.PreparedInput) int64 { return v.EstimatedBytes() }
	if a.base, err = addArtifact(&a.all, "production.base", immutableInput, inputSize, false); err != nil {
		return a, err
	}
	if a.transport, err = addArtifact(&a.all, "production.transport", immutableInput, inputSize, false); err != nil {
		return a, err
	}
	if a.input, err = addArtifact(&a.all, "production.input", immutableInput, inputSize, false); err != nil {
		return a, err
	}
	if a.transportSpecs, err = addArtifact(&a.all, "search.transport", cloneValue[[]core.TransportCandidateSpec], valueBytes[[]core.TransportCandidateSpec], false); err != nil {
		return a, err
	}
	if a.surfaceSpecs, err = addArtifact(&a.all, "search.surface", cloneValue[[]core.SurfaceCandidateSpec], valueBytes[[]core.SurfaceCandidateSpec], false); err != nil {
		return a, err
	}
	if a.searchWindow, err = addArtifact(&a.all, "search.window", cloneValue[candidateWindowArgs], valueBytes[candidateWindowArgs], false); err != nil {
		return a, err
	}
	if a.searchPasses, err = addArtifact(&a.all, "search.passes", cloneValue[[]candidateWindowArgs], valueBytes[[]candidateWindowArgs], false); err != nil {
		return a, err
	}
	if a.connected, err = addArtifact(&a.all, "search.connected", cloneValue[[]core.ConnectedCandidateSpec], valueBytes[[]core.ConnectedCandidateSpec], false); err != nil {
		return a, err
	}
	if a.station, err = addArtifact(&a.all, "search.station", cloneValue[[]core.StationCandidateSpec], valueBytes[[]core.StationCandidateSpec], false); err != nil {
		return a, err
	}
	if a.distributor, err = addArtifact(&a.all, "search.distributor", cloneValue[[]core.DistributorCandidateSpec], valueBytes[[]core.DistributorCandidateSpec], false); err != nil {
		return a, err
	}
	if a.bus, err = addArtifact(&a.all, "search.bus", cloneValue[[]core.BusCandidateSpec], valueBytes[[]core.BusCandidateSpec], false); err != nil {
		return a, err
	}
	if a.choice, err = addArtifact(&a.all, "candidate.specification", cloneValue[selection], valueBytes[selection], false); err != nil {
		return a, err
	}
	if a.geometry, err = addImmutableArtifact(&a.all, "factory.geometry", func(v core.GeometrySnapshot) int64 { return v.EstimatedBytes() }, false); err != nil {
		return a, err
	}
	if a.verified, err = addImmutableArtifact(&a.all, "factory.verified", func(v core.VerifiedSnapshot) int64 { return v.EstimatedBytes() }, true); err != nil {
		return a, err
	}
	if a.encoded, err = addImmutableArtifact(&a.all, "factory.encoded", func(v core.EncodedSnapshot) int64 { return v.EstimatedBytes() }, true); err != nil {
		return a, err
	}
	if a.selected, err = addImmutableArtifact(&a.all, "factory.selected", func(v selectedFactory) int64 { return v.Verified.EstimatedBytes() }, true); err != nil {
		return a, err
	}
	if a.refinement, err = addArtifact(&a.all, "station.refinement", cloneValue[[]core.StationCandidateSpec], valueBytes[[]core.StationCandidateSpec], true); err != nil {
		return a, err
	}
	if a.regions, err = addArtifact(&a.all, "factory.regions", cloneValue[[]core.RegionDescriptor], valueBytes[[]core.RegionDescriptor], true); err != nil {
		return a, err
	}
	a.regionReport, err = workflow.NewArtifactKey("factory.region-report", func(v core.RegionValidation) (core.RegionValidation, error) { return v, nil }, func(v core.RegionValidation) int64 { return v.EstimatedBytes() }, workflow.ArtifactOptions{Regional: true, Diagnostic: true})
	if err != nil {
		return a, err
	}
	a.all = append(a.all, a.regionReport.Ref())
	if a.approved, err = addImmutableArtifact(&a.all, "factory.approved", func(v selectedFactory) int64 { return v.Verified.EstimatedBytes() }, true); err != nil {
		return a, err
	}
	return a, nil
}
