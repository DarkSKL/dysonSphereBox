package build

// 文件职责：检查不共享节点的球面传送带路径是否交叉。
// 使用：最终世界坐标的空间哈希和线段最近距离；边界：只检查蓝图内部带与带，通道设施和已有世界实体仍需游戏验证。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
)

// surfaceBeltSpan 职责：关联带段两端的建筑编号与世界位置；使用：交叉宽窄相位；边界：末端无后继时退化为球形节点。
type surfaceBeltSpan struct {
	indices [2]int32
	ends    [2]placement.Vector
}

// surfaceSpanDistanceSquared 职责：计算闭线段最近距离；使用：区分平面交叉与高架跨越；边界：支持零长度段，不依赖投影平面。
func surfaceSpanDistanceSquared(a, b surfaceBeltSpan) float64 {
	u, v, w := a.ends[1].Sub(a.ends[0]), b.ends[1].Sub(b.ends[0]), a.ends[0].Sub(b.ends[0])
	aa, bb, ab, aw, bw := u.Dot(u), v.Dot(v), u.Dot(v), u.Dot(w), v.Dot(w)
	s, t := 0., 0.
	if aa < 1e-12 && bb < 1e-12 {
		return w.Dot(w)
	}
	if aa < 1e-12 {
		t = max(0, min(1, bw/bb))
	} else {
		if bb < 1e-12 {
			s = max(0, min(1, -aw/aa))
		} else {
			if denominator := aa*bb - ab*ab; denominator > 1e-12 {
				s = max(0, min(1, (ab*bw-aw*bb)/denominator))
			}
			t = (ab*s + bw) / bb
			if t < 0 {
				t, s = 0, max(0, min(1, -aw/aa))
			} else if t > 1 {
				t, s = 1, max(0, min(1, (ab-aw)/aa))
			}
		}
	}
	delta := w.Add(u.Scale(s)).Sub(v.Scale(t))
	return delta.Dot(delta)
}

// validateBeltCrossings 职责：拒绝无拓扑连接的同高度带段交叉；使用：球面出口；边界：共享节点的转弯与汇流合法，高架凭真实三维距离判断，不校验 addon 通道。
func (encoding *surfaceEncoding) validateBeltCrossings(ctx context.Context, records []blueprint.Building) error {
	positions, err := encoding.surfaceWorldPositions(records)
	if err != nil {
		return err
	}
	byIndex := make(map[int32]int, len(records))
	for i, record := range records {
		byIndex[record.Index] = i
	}
	const radius = .23
	spans := make([]surfaceBeltSpan, 0, len(records))
	occupied := map[[3]int][]int{}
	for i, record := range records {
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "传送带交叉检查已取消或超时"}
		}
		if !surfaceIsBelt(record.ItemID) {
			continue
		}
		j := i
		if output, exists := byIndex[record.OutputObjIdx]; exists && surfaceIsBelt(records[output].ItemID) {
			j = output
		}
		span := surfaceBeltSpan{indices: [2]int32{record.Index, records[j].Index}}
		for end, index := range [2]int{i, j} {
			p := placement.Vector{positions[index].x, positions[index].y, positions[index].z}
			span.ends[end] = p.Add(encoding.localUp(p).Scale(.2))
		}
		low, high := [3]int{}, [3]int{}
		for dimension := range low {
			low[dimension] = int(math.Floor((min(span.ends[0][dimension], span.ends[1][dimension]) - radius) / 8))
			high[dimension] = int(math.Floor((max(span.ends[0][dimension], span.ends[1][dimension]) + radius) / 8))
		}
		seen := map[int]bool{}
		for x := low[0]; x <= high[0]; x++ {
			for y := low[1]; y <= high[1]; y++ {
				for z := low[2]; z <= high[2]; z++ {
					key := [3]int{x, y, z}
					for _, index := range occupied[key] {
						if seen[index] {
							continue
						}
						seen[index] = true
						other := spans[index]
						if span.indices[0] == other.indices[0] || span.indices[0] == other.indices[1] || span.indices[1] == other.indices[0] || span.indices[1] == other.indices[1] {
							continue
						}
						if surfaceSpanDistanceSquared(span, other) < 4*radius*radius-1e-6 {
							return &blueprint.Error{Code: "GEN_E_COLLISION", Message: fmt.Sprintf("无连接的传送带路线交叉：建筑 %d→%d 与 %d→%d，(%.1f,%.1f,%.1f)->(%.1f,%.1f,%.1f)/(%.1f,%.1f,%.1f)->(%.1f,%.1f,%.1f)", other.indices[0], other.indices[1], span.indices[0], span.indices[1], other.ends[0][0], other.ends[0][1], other.ends[0][2], other.ends[1][0], other.ends[1][1], other.ends[1][2], span.ends[0][0], span.ends[0][1], span.ends[0][2], span.ends[1][0], span.ends[1][1], span.ends[1][2])}
						}
					}
					occupied[key] = append(occupied[key], len(spans))
				}
			}
		}
		spans = append(spans, span)
	}
	return nil
}
