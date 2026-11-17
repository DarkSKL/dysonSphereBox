package generator

// 文件职责：把领域核心接入受限反射工具入口。
// 使用：YAML注册目录；边界：入口仅取数、调用一个核心、暂存，不访问完整实例、引擎或其他工具。

import (
	"context"
	"fmt"

	core "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/workflow"
)

// toolset 职责：保存只读产物键；使用：反射绑定；边界：无任务状态或当前候选字段。
type toolset struct{ keys artifacts }

// emptyArgs 职责：表达无参数工具；使用：统一反射签名；边界：未知with字段拒绝。
type emptyArgs struct{}

// indexArgs 职责：引用当前只读规格列表的一项；使用：引擎传递稳定候选下标；边界：下标不允许访问其他工作区。
type indexArgs struct {
	Index int `json:"index" required:"true" min:"0"`
}

// busArgs 职责：配置是否枚举总线收紧；使用：内部mode6方案；边界：不改变公开模式能力。
type busArgs struct {
	Tighten bool `json:"tighten"`
}

// domainFailure 职责：保留取消并把明确领域无解交给引擎；使用：核心调用边界；边界：未知事实、陈旧输入及内核错误不变成可重试无解。
func domainFailure(ctx context.Context, step *workflow.StepContext, err error) error {
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	code, preparation, known := classifyDomainFailure(err, false)
	if known {
		if recordErr := step.Diagnostic("failure-code:" + code); recordErr != nil {
			return recordErr
		}
		if preparation {
			code = "GEN_E_CONNECTED_PREPARATION"
		}
		return &workflow.Failure{Code: code, Message: err.Error(), Cause: err}
	}
	return err
}

// solutionFailures 职责：集中声明真实求解失败范围；使用：全部工具白名单；边界：不包括输入、未知物理事实、反射异常或取消。
var solutionFailures = []string{
	"GEN_E_CONNECTED_PREPARATION",
	"GEN_E_SHAPE",
	"GEN_E_LAYOUT", "GEN_E_COLLISION", "GEN_E_SURFACE_ROUTING", "GEN_E_SURFACE_CAPACITY", "GEN_E_DENSITY_LAYOUT", "GEN_E_DENSITY_STREET_SEAM",
	"GEN_E_INSERTER_CAPACITY", "GEN_E_BELT_CAPACITY", "GEN_E_PORT_CAPACITY", "GEN_E_OVERFLOW", "GEN_E_FLOW", "GEN_E_STATION_PORT", "GEN_E_DENSITY_CARGO",
	"GEN_E_SPRAY_ORDER", "GEN_E_POWER_NETWORK", "GEN_E_POWER_COVERAGE", "GEN_E_MONITOR", "GEN_E_MACHINE_INTEGRITY", "GEN_I_DENSITY_DOMINATED", "GEN_I_DENSITY_EQUIVALENT",
	"GEN_E_SURFACE", "GEN_E_CONNECTIVITY", "GEN_E_ROUTE", "GEN_E_STORAGE_CLEARANCE", "GEN_E_FLOW_MONITOR", "GEN_E_DENSITY_FACILITIES",
	"GEN_E_DENSITY_MERGE", "GEN_E_DENSITY_PORT", "GEN_E_DENSITY_STREET_CARGO", "GEN_E_DENSITY_STREET_PARITY", "GEN_E_DENSITY_STREET_PHASE", "GEN_E_DENSITY_STREET_SCOPE",
}

// PlanTransport 职责：产生有限档位规格；使用：外层升档候选前；边界：不执行规格。
func (t *toolset) PlanTransport(ctx context.Context, s *workflow.StepContext, args core.TransportSearchOptions) error {
	input, err := workflow.Read(s, t.keys.base)
	if err != nil {
		return err
	}
	specs, err := core.EnumerateTransportCandidates(ctx, input, args)
	if err != nil {
		return domainFailure(ctx, s, err)
	}
	return workflow.Stage(s, t.keys.transportSpecs, specs)
}

// PlanSurface 职责：产生窗口/子环规格；使用：明确球面流程；边界：无几何成功承诺。
func (t *toolset) PlanSurface(ctx context.Context, s *workflow.StepContext, args core.SurfaceSearchOptions) error {
	input, err := workflow.Read(s, t.keys.transport)
	if err != nil {
		return err
	}
	specs, err := core.EnumerateSurfaceCandidates(ctx, input, args)
	if err != nil {
		return domainFailure(ctx, s, err)
	}
	return workflow.Stage(s, t.keys.surfaceSpecs, specs)
}

// ApplySurface 职责：固定当前工作区球面规格；使用：球面候选内部；边界：不切换其他工作区。
func (t *toolset) ApplySurface(ctx context.Context, s *workflow.StepContext, args indexArgs) error {
	input, err := workflow.Read(s, t.keys.transport)
	if err != nil {
		return err
	}
	specs, err := workflow.Read(s, t.keys.surfaceSpecs)
	if err != nil {
		return err
	}
	if args.Index < 0 || args.Index >= len(specs) {
		return fmt.Errorf("surface candidate index out of range")
	}
	if err = recordSpecification(s, "surface", specs[args.Index]); err != nil {
		return err
	}
	prepared, err := core.PrepareSurfaceCandidate(ctx, input, specs[args.Index])
	if err != nil {
		return domainFailure(ctx, s, err)
	}
	return workflow.Stage(s, t.keys.input, prepared)
}

// PlanConnected 职责：枚举同一核心的直连参数集合；使用：直连方案族；边界：不执行候选或备用组。
func (t *toolset) PlanConnected(ctx context.Context, s *workflow.StepContext, args core.ConnectedSearchOptions) error {
	input, err := workflow.Read(s, t.keys.input)
	if err != nil {
		return err
	}
	specs, err := core.EnumerateConnectedCandidates(ctx, input, args)
	if err != nil {
		return domainFailure(ctx, s, err)
	}
	return workflow.Stage(s, t.keys.connected, specs)
}

// AssembleConnected 职责：执行一个直连规格；使用：引擎布局候选内部；边界：不验证编码、不择优。
func (t *toolset) AssembleConnected(ctx context.Context, s *workflow.StepContext, args indexArgs) error {
	input, err := workflow.Read(s, t.keys.input)
	if err != nil {
		return err
	}
	specs, err := workflow.Read(s, t.keys.connected)
	if err != nil {
		return err
	}
	if args.Index < 0 || args.Index >= len(specs) {
		return fmt.Errorf("connected candidate index out of range")
	}
	ctx, err = core.WithCandidateObservation(ctx, s.Node(), s.Candidate())
	if err != nil {
		return err
	}
	if err = recordSpecification(s, "connected", specs[args.Index]); err != nil {
		return err
	}
	geometry, err := core.AssembleConnectedCandidate(ctx, input, specs[args.Index])
	if err != nil {
		return domainFailure(ctx, s, err)
	}
	if err = workflow.Stage(s, t.keys.choice, selection{Family: "connected", Connected: specs[args.Index]}); err != nil {
		return err
	}
	return workflow.Stage(s, t.keys.geometryStages.assembly, geometry)
}

// PlanStation 职责：枚举共享塔核心规格；使用：行星塔和星际塔共用方案；边界：不复制设备事实。
func (t *toolset) PlanStation(ctx context.Context, s *workflow.StepContext, args core.StationSearchOptions) error {
	input, err := workflow.Read(s, t.keys.input)
	if err != nil {
		return err
	}
	specs, err := core.EnumerateStationCandidates(ctx, input, args)
	if err != nil {
		return domainFailure(ctx, s, err)
	}
	return workflow.Stage(s, t.keys.station, specs)
}

// AssembleStation 职责：装配一个塔规格；使用：引擎布局候选内部；边界：不进行备用搜索或编码。
func (t *toolset) AssembleStation(ctx context.Context, s *workflow.StepContext, args indexArgs) error {
	input, err := workflow.Read(s, t.keys.input)
	if err != nil {
		return err
	}
	specs, err := workflow.Read(s, t.keys.station)
	if err != nil {
		return err
	}
	if args.Index < 0 || args.Index >= len(specs) {
		return fmt.Errorf("station candidate index out of range")
	}
	ctx, err = core.WithCandidateObservation(ctx, s.Node(), s.Candidate())
	if err != nil {
		return err
	}
	if err = recordSpecification(s, "station", specs[args.Index]); err != nil {
		return err
	}
	geometry, err := core.AssembleStationCandidate(ctx, input, specs[args.Index])
	if err != nil {
		return domainFailure(ctx, s, err)
	}
	if err = workflow.Stage(s, t.keys.choice, selection{Family: "station", Station: specs[args.Index]}); err != nil {
		return err
	}
	return workflow.Stage(s, t.keys.geometryStages.assembly, geometry)
}

// PlanDistributor 职责：枚举配送规格；使用：配送方案族；边界：备用元数据由调度层解释。
func (t *toolset) PlanDistributor(ctx context.Context, s *workflow.StepContext, args core.DistributorSearchOptions) error {
	input, err := workflow.Read(s, t.keys.input)
	if err != nil {
		return err
	}
	specs, err := core.EnumerateDistributorCandidates(ctx, input, args)
	if err != nil {
		return domainFailure(ctx, s, err)
	}
	return workflow.Stage(s, t.keys.distributor, specs)
}

// AssembleDistributor 职责：装配一个配送规格；使用：引擎布局候选内部；边界：不自行扩大规格集合。
func (t *toolset) AssembleDistributor(ctx context.Context, s *workflow.StepContext, args indexArgs) error {
	input, err := workflow.Read(s, t.keys.input)
	if err != nil {
		return err
	}
	specs, err := workflow.Read(s, t.keys.distributor)
	if err != nil {
		return err
	}
	if args.Index < 0 || args.Index >= len(specs) {
		return fmt.Errorf("distributor candidate index out of range")
	}
	ctx, err = core.WithCandidateObservation(ctx, s.Node(), s.Candidate())
	if err != nil {
		return err
	}
	if err = recordSpecification(s, "distributor", specs[args.Index]); err != nil {
		return err
	}
	geometry, err := core.AssembleDistributorCandidate(ctx, input, specs[args.Index])
	if err != nil {
		return domainFailure(ctx, s, err)
	}
	if err = workflow.Stage(s, t.keys.choice, selection{Family: "distributor", Distributor: specs[args.Index]}); err != nil {
		return err
	}
	return workflow.Stage(s, t.keys.geometryStages.assembly, geometry)
}

// PlanBus 职责：枚举内部总线规格；使用：mode6方案；边界：不扩大公开模式。
func (t *toolset) PlanBus(ctx context.Context, s *workflow.StepContext, args busArgs) error {
	input, err := workflow.Read(s, t.keys.input)
	if err != nil {
		return err
	}
	specs, err := core.EnumerateBusCandidates(ctx, input, args.Tighten)
	if err != nil {
		return domainFailure(ctx, s, err)
	}
	return workflow.Stage(s, t.keys.bus, specs)
}

// AssembleBus 职责：装配一个内部总线规格；使用：引擎布局候选；边界：无内部重试或编码。
func (t *toolset) AssembleBus(ctx context.Context, s *workflow.StepContext, args indexArgs) error {
	input, err := workflow.Read(s, t.keys.input)
	if err != nil {
		return err
	}
	specs, err := workflow.Read(s, t.keys.bus)
	if err != nil {
		return err
	}
	if args.Index < 0 || args.Index >= len(specs) {
		return fmt.Errorf("bus candidate index out of range")
	}
	ctx, err = core.WithCandidateObservation(ctx, s.Node(), s.Candidate())
	if err != nil {
		return err
	}
	if err = recordSpecification(s, "bus", specs[args.Index]); err != nil {
		return err
	}
	geometry, err := core.AssembleBusCandidate(ctx, input, specs[args.Index])
	if err != nil {
		return domainFailure(ctx, s, err)
	}
	if err = workflow.Stage(s, t.keys.choice, selection{Family: "bus", Bus: specs[args.Index]}); err != nil {
		return err
	}
	return workflow.Stage(s, t.keys.geometryStages.assembly, geometry)
}

// Encode 职责：编码与实例验证版本一致的封闭快照；使用：全部候选选定后的唯一出口；边界：不修改几何、不发布服务成功事件。
func (t *toolset) Encode(ctx context.Context, s *workflow.StepContext, _ emptyArgs) error {
	if _, err := s.FrozenVersion(); err != nil {
		return err
	}
	selected, err := workflow.Read(s, t.keys.approved)
	if err != nil {
		return err
	}
	encoded, err := core.EncodeSnapshot(ctx, selected.Verified)
	if err != nil {
		return err
	}
	return workflow.Stage(s, t.keys.encoded, encoded)
}
