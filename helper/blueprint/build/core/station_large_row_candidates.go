package build

// 文件职责：为普通原生大厂提前提供较少拆行的真实行阵；边界：保留所有原候选，不改机器、配方、运输容量、规则或预算。
import (
	"context"
	"sort"
)

// prioritizeLargeStandardStationRows 职责：复用调用者已启用的行阵能力与既有长跨度，按真实拆行数量安排新增组合；边界：仅普通>=256机器且native-window，其他输入逐项保持原样。
func prioritizeLargeStandardStationRows(ctx context.Context, input *PreparedInput, options StationSearchOptions, original []StationCandidateSpec) ([]StationCandidateSpec, error) {
	if !options.StandardRowPacking || input.request.Density != densityStandard || input.request.Plan.MachineCount < 256 || input.geometry.Strategy != "native-window" {
		return original, nil
	}
	seen := map[StationCandidateSpec]bool{}
	for _, s := range original {
		seen[s] = true
	}
	added := []StationCandidateSpec{}
	counts := map[float64]int{}
	for _, s := range original {
		if s.Kind != "blackbox" || s.RowPacking || s.MaterialOrder {
			continue
		}
		s.RowPacking = true
		if seen[s] {
			continue
		}
		seen[s] = true
		if _, ok := counts[s.MaxSpan]; !ok {
			rows, err := connectedExecutionRows(ctx, input, s.MaxSpan, false)
			if err != nil {
				return nil, err
			}
			counts[s.MaxSpan] = rows.rowCount
		}
		added = append(added, s)
	}
	sort.SliceStable(added, func(i, j int) bool {
		a, b := added[i], added[j]
		if counts[a.MaxSpan] != counts[b.MaxSpan] {
			return counts[a.MaxSpan] < counts[b.MaxSpan]
		}
		if a.Refine != b.Refine {
			return !a.Refine
		}
		if a.Street != b.Street {
			return !a.Street
		}
		return a.Order < b.Order
	})
	return append(added, original...), nil
}
