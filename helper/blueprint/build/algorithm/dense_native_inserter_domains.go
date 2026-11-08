package build

// 文件职责：联合选择原生分拣器的合法端点，避免前一根臂的贪心选择堵死相邻臂。
// 使用：全部端点域通过槽位、角度和跨度检查之后；边界：回溯只选择真实合法几何，不放宽碰撞或升级设备。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"context"
	"math"
	"sort"
)

// nativeInserterEndpoints 职责：去除相同端点候选；使用：合法域去重；边界：仅本次坐标系。
type nativeInserterEndpoints struct {
	first, second blueprint.Point
	yaw, yaw2     float32
}

// nativeInserterChoice 职责：保存已核验记录和实际动态盒；使用：联合冲突检查；边界：不能以端点距离替代盒检测。
type nativeInserterChoice struct {
	record blueprint.Building
	boxes  []placement.OwnedBox
}

// nativeInserterDomain 职责：关联原实体下标与可建端点；使用：约束求解；边界：每个实体只选一个候选。
type nativeInserterDomain struct {
	index   int
	choices []nativeInserterChoice
}

// chooseNativeInserterEndpoints 职责：在最小合法域优先的有限回溯中选择互不碰撞的臂；使用：原生高纬适配；边界：成功前不提交，失败交由外层事务回滚。
func (b *builder) chooseNativeInserterEndpoints(ctx context.Context, records []blueprint.Building, domains []nativeInserterDomain) (int, error) {
	for i := range domains {
		domain := &domains[i]
		original := records[domain.index]
		cost := func(choice nativeInserterChoice) float64 {
			c := choice.record
			result := 0.
			for n, p := range []blueprint.Point{c.LocalOffset, c.LocalOffset2} {
				q := []blueprint.Point{original.LocalOffset, original.LocalOffset2}[n]
				result += math.Pow(float64(p.X-q.X), 2) + math.Pow(float64(p.Y-q.Y), 2) + math.Pow(float64(p.Z-q.Z), 2)
			}
			if len(original.Parameters) > 0 && c.Parameters[0] > original.Parameters[0] {
				result += 100
			}
			return result
		}
		sort.SliceStable(domain.choices, func(i, j int) bool { return cost(domain.choices[i]) < cost(domain.choices[j]) })
	}
	sort.SliceStable(domains, func(i, j int) bool { return len(domains[i].choices) < len(domains[j].choices) })
	selected := make([]nativeInserterChoice, len(domains))
	nodes := 0
	var search func(int) bool
	search = func(depth int) bool {
		if depth == len(domains) {
			return true
		}
		if ctx.Err() != nil || nodes >= 20000 {
			return false
		}
		for _, choice := range domains[depth].choices {
			nodes++
			conflict := false
			for previous := 0; previous < depth && !conflict; previous++ {
				for _, a := range choice.boxes {
					for _, c := range selected[previous].boxes {
						if a.Geometry.Overlaps(c.Geometry) {
							conflict = true
							break
						}
					}
					if conflict {
						break
					}
				}
			}
			if conflict {
				continue
			}
			selected[depth] = choice
			if search(depth + 1) {
				return true
			}
			if nodes >= 20000 || ctx.Err() != nil {
				return false
			}
		}
		return false
	}
	if !search(0) {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		return 0, &blueprint.Error{Code: "GEN_E_COLLISION", Message: "原生分拣器端点在本次联合搜索内没有互不碰撞的完整组合"}
	}
	adjusted := 0
	for i, domain := range domains {
		candidate, original := selected[i].record, records[domain.index]
		arm := b.all[domain.index]
		if candidate.LocalOffset != original.LocalOffset || candidate.LocalOffset2 != original.LocalOffset2 || candidate.Yaw != original.Yaw || candidate.Yaw2 != original.Yaw2 {
			adjusted++
		}
		if candidate.LocalOffset != original.LocalOffset {
			p := candidate.LocalOffset
			arm.a = vector{float64(p.X), float64(p.Y), float64(p.Z)}
		}
		if candidate.LocalOffset2 != original.LocalOffset2 {
			p := candidate.LocalOffset2
			arm.b = vector{float64(p.X), float64(p.Y), float64(p.Z)}
		}
		arm.record.Parameters = candidate.Parameters
		arm.record.Yaw, arm.record.Yaw2 = candidate.Yaw, candidate.Yaw2
		arm.nativeInserterSpan = candidate.Parameters[0]
		records[domain.index] = candidate
	}
	return adjusted, nil
}
