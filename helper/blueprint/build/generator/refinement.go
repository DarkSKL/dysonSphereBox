package generator

// 文件职责：显式枚举塔口精修参数，复用同一装配、几何和校验链。
// 使用：includeCurrent保留完整已验证基线；边界：工具只生产规格或单个装配图，候选选择和预算由引擎维护。
import (
	core "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/workflow"
	"context"
	"fmt"
	"reflect"
)

// refinementArgs 职责：声明是否枚举精修规格；使用：YAML参数；边界：不启动求解或选择基线。
type refinementArgs struct {
	Enabled bool `json:"enabled"`
}

// refinementSpecArgs 职责：传递冻结后的完整精修规格；使用：候选输入；边界：不读取将被本候选替换的旧geometry/choice。
type refinementSpecArgs struct {
	Specification core.StationCandidateSpec `json:"specification" required:"true"`
}

// PrepareRefinement 职责：派生合资格的精修规格；使用：显式枚举节点；边界：不复制基线实体，空集合交给includeCurrent处理。
func (t *toolset) PrepareRefinement(ctx context.Context, s *workflow.StepContext, args refinementArgs) error {
	choice, err := workflow.Read(s, t.keys.choice)
	if err != nil {
		return err
	}
	if _, err = workflow.Read(s, t.keys.verified); err != nil {
		return err
	}
	if choice.Family != "station" {
		return fmt.Errorf("station refinement requires station winner")
	}
	specs := []core.StationCandidateSpec{}
	if args.Enabled && choice.Station.Kind == "blackbox" && !choice.Station.NearbyInputPorts {
		spec, err := core.RefineStationCandidate(choice.Station)
		if err != nil {
			return domainFailure(ctx, s, err)
		}
		specs = append(specs, spec)
	}
	return workflow.Stage(s, t.keys.refinement, specs)
}

// refinementProvider 职责：把精修数据投影为候选参数；使用：items固定绑定；边界：不生成新规格，不调用工具或读取任意字段。
func refinementProvider(a artifacts) workflow.StateProvider {
	return workflow.StateProvider{Name: "station.refinement-candidates", Type: reflect.TypeFor[[]refinementSpecArgs](), Requires: []workflow.Artifact{a.refinement.Ref()}, Resolve: func(_ context.Context, s *workflow.StepContext) (any, error) {
		specs, err := workflow.Read(s, a.refinement)
		if err != nil {
			return nil, err
		}
		result := make([]refinementSpecArgs, len(specs))
		for index, spec := range specs {
			result[index] = refinementSpecArgs{Specification: spec}
		}
		return result, nil
	}}
}

// AssembleStationSpec 职责：装配一个完整传入规格；使用：精修候选；边界：与普通候选共用核心和后继子流，不调用组合Solve。
func (t *toolset) AssembleStationSpec(ctx context.Context, s *workflow.StepContext, args refinementSpecArgs) error {
	input, err := workflow.Read(s, t.keys.input)
	if err != nil {
		return err
	}
	ctx, err = core.WithCandidateObservation(ctx, s.Node(), s.Candidate())
	if err != nil {
		return err
	}
	if err = recordSpecification(s, "station-refinement", args.Specification); err != nil {
		return err
	}
	assembly, err := core.AssembleStationCandidate(ctx, input, args.Specification)
	if err != nil {
		return domainFailure(ctx, s, err)
	}
	if err = workflow.Stage(s, t.keys.choice, selection{Family: "station", Station: args.Specification}); err != nil {
		return err
	}
	return workflow.Stage(s, t.keys.geometryStages.assembly, assembly)
}

// SelectValidated 职责：封装普通候选组的完整获选结果；使用：各方案统一交接；边界：只接受指纹一致的已验证快照，不重新装配。
func (t *toolset) SelectValidated(ctx context.Context, s *workflow.StepContext, _ emptyArgs) error {
	verified, err := workflow.Read(s, t.keys.verified)
	if err != nil {
		return err
	}
	// 规格仍是注册契约中的Requires，版本依赖由事务保留；后续阶段不消费其副本。
	verified, err = core.FreezeValidatedGeometry(ctx, verified.Geometry(), verified.Validation())
	if err != nil {
		return err
	}
	return workflow.Stage(s, t.keys.selected, selectedFactory{verified})
}

// refinementComparison 职责：仅接受同塔数、面积不增且实际线长下降的精修；使用：引擎只读比较；边界：二者必须已验证，平局保留基线。
func (r *runtime) refinementComparison(a, b *workflow.FrozenSnapshot) (int, error) {
	first, err := workflow.SnapshotRead(a, r.keys.verified, "")
	if err != nil {
		return 0, err
	}
	second, err := workflow.SnapshotRead(b, r.keys.verified, "")
	if err != nil {
		return 0, err
	}
	n, err := core.StationCandidateTowerCount(first.Geometry())
	if err != nil {
		return 0, err
	}
	m, err := core.StationCandidateTowerCount(second.Geometry())
	if err != nil {
		return 0, err
	}
	if core.BetterStationRefinement(n, first.Metrics(), m, second.Metrics()) {
		return -1, nil
	}
	return 0, nil
}

// observationVerified 职责：为观察器选择当前阶段已验证的完整产物；使用：内外层候选事件；边界：只读快照查询，最终封装优先于过渡产物。
func (r *runtime) observationVerified(snapshot *workflow.FrozenSnapshot) (core.VerifiedSnapshot, error) {
	if selected, err := workflow.SnapshotRead(snapshot, r.keys.selected, ""); err == nil {
		return selected.Verified, nil
	}

	return workflow.SnapshotRead(snapshot, r.keys.verified, "")
}

// surfaceComparison 职责：在完整严格带基线与跨带方案之间保留明确改善；使用：共同球面流程；边界：不增加设施，不比较部分结果。
func (r *runtime) surfaceComparison(a, b *workflow.FrozenSnapshot) (int, error) {
	first, err := r.observationVerified(a)
	if err != nil {
		return 0, err
	}
	second, err := r.observationVerified(b)
	if err != nil {
		return 0, err
	}
	firstExpanded, err := core.UsesAdjacentSurface(first)
	if err != nil {
		return 0, err
	}
	secondExpanded, err := core.UsesAdjacentSurface(second)
	if err != nil {
		return 0, err
	}
	if firstExpanded && !secondExpanded {
		better, err := core.BetterAdjacentCandidate(first, second)
		if err != nil {
			return 0, err
		}
		if better {
			return -1, nil
		}
		return 0, nil
	}
	if !firstExpanded && secondExpanded {
		better, err := core.BetterAdjacentCandidate(second, first)
		if err != nil {
			return 0, err
		}
		if !better {
			return -1, nil
		}
		return 0, nil
	}
	return 0, nil
}
