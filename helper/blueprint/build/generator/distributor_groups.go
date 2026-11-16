package generator

// 文件职责：把配送核心的纯规格分组映射为引擎失败依赖描述。
// 使用：登记的distributor.groups只读提供器；边界：不读取执行结果，不自行触发备用、不运行任何候选。

import (
	"context"
	"fmt"
	"reflect"

	core "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/workflow"
)

// distributorGroups 职责：保留核心稳定组序、参数下标及备用因果；使用：状态属性；边界：拒绝同组不一致的条件而非猜测。
func distributorGroups(specs []core.DistributorCandidateSpec) ([]workflow.CandidateGroup, error) {
	groups := []workflow.CandidateGroup{}
	positions := map[string]int{}
	conditions := map[string]core.DistributorFallback{}
	for index, spec := range specs {
		if spec.GroupID == "" {
			return nil, fmt.Errorf("distributor candidate lacks group identity")
		}
		if len(spec.Fallback.AfterGroups) > 0 && (!spec.Fallback.RequireNoSuccess || !spec.Fallback.RequireFailure) {
			return nil, fmt.Errorf("unsupported distributor fallback condition")
		}
		position, exists := positions[spec.GroupID]
		if !exists {
			for _, dependency := range spec.Fallback.AfterGroups {
				if _, ok := positions[dependency]; !ok {
					return nil, fmt.Errorf("distributor group %s has missing or forward dependency %s", spec.GroupID, dependency)
				}
			}
			group := workflow.CandidateGroup{ID: spec.GroupID, Items: []map[string]any{}, AfterNoSuccess: append([]string(nil), spec.Fallback.AfterGroups...)}
			if spec.Fallback.ExcludedFailureCode != "" {
				group.ExcludeFailureCodes = []string{spec.Fallback.ExcludedFailureCode}
			}
			position = len(groups)
			groups = append(groups, group)
			positions[spec.GroupID] = position
			conditions[spec.GroupID] = spec.Fallback
		} else if !reflect.DeepEqual(conditions[spec.GroupID], spec.Fallback) {
			return nil, fmt.Errorf("distributor group %s has inconsistent failure dependencies", spec.GroupID)
		}
		groups[position].Items = append(groups[position].Items, map[string]any{"index": index})
	}
	return groups, nil
}

// distributorGroupProvider 职责：声明只读规格到引擎组参数的属性；使用：YAML groups引用；边界：无法访问工具目录、调度器或其他候选状态。
func distributorGroupProvider(key workflow.ArtifactKey[[]core.DistributorCandidateSpec]) workflow.StateProvider {
	return workflow.StateProvider{Name: "distributor.groups", Type: reflect.TypeFor[[]workflow.CandidateGroup](), Requires: []workflow.Artifact{key.Ref()}, Resolve: func(_ context.Context, s *workflow.StepContext) (any, error) {
		specs, err := workflow.Read(s, key)
		if err != nil {
			return nil, err
		}
		return distributorGroups(specs)
	}}
}
