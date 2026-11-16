package build

// 文件职责：检查不共享节点的球面传送带路径是否交叉。
// 使用：最终世界坐标的空间哈希和线段最近距离；边界：只检查蓝图内部带与带，通道设施和已有世界实体仍需游戏验证。

import (
	"context"
	"fmt"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
)

// surfaceBeltSpan 职责：关联带段两端的建筑编号与世界位置；使用：交叉宽窄相位；边界：末端无后继时退化为球形节点。
type surfaceBeltSpan struct {
	indices [2]int32
	ends    [2]placement.Vector
}

// ordinaryBeltRadius 职责：定义普通模式完整带段占位半径；使用：交叉校验与接头必要间距；边界：密铺节点规则独立，不用构造参数放宽本值。
const ordinaryBeltRadius = .23

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
	if usesDenseBeltCollisions(ctx) {
		return encoding.validateDenseBeltNodes(ctx, records)
	}
	positions, err := encoding.surfaceWorldPositions(records)
	if err != nil {
		return err
	}
	return encoding.validateBeltCrossingsAtWorld(ctx, records, positions)
}

// validateBeltCrossingsAtWorld 职责：共用已定位普通带段的精确交叉规则；使用：完整终检与原生单元落位预检；边界：调用方提供同序真实世界坐标，不能用端点距离代替段距离。
func (encoding *surfaceEncoding) validateBeltCrossingsAtWorld(ctx context.Context, records []blueprint.Building, positions []vector) error {
	var candidates, narrow uint64
	defer func() { recordPerformanceCollision(ctx, "beltCrossings", candidates, narrow) }()
	byIndex := make(map[int32]int, len(records))
	for i, record := range records {
		byIndex[record.Index] = i
	}
	const radius = ordinaryBeltRadius
	spans := make([]surfaceBeltSpan, 0, len(records))
	occupied := worldSpatialIndex{}
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
		low, high := polarSpanBounds(span, radius)
		cells := worldSpatialBounds(low, high, 8)
		var collision error
		occupied.any(cells, func(index int) bool {
			candidates++
			other := spans[index]
			if span.indices[0] == other.indices[0] || span.indices[0] == other.indices[1] || span.indices[1] == other.indices[0] || span.indices[1] == other.indices[1] {
				return false
			}
			narrow++
			if surfaceSpanDistanceSquared(span, other) < 4*radius*radius-1e-6 {
				collision = &blueprint.Error{Code: "GEN_E_COLLISION", Message: fmt.Sprintf("无连接的传送带路线交叉：建筑 %d→%d 与 %d→%d，(%.1f,%.1f,%.1f)->(%.1f,%.1f,%.1f)/(%.1f,%.1f,%.1f)->(%.1f,%.1f,%.1f)", other.indices[0], other.indices[1], span.indices[0], span.indices[1], other.ends[0][0], other.ends[0][1], other.ends[0][2], other.ends[1][0], other.ends[1][1], other.ends[1][2], span.ends[0][0], span.ends[0][1], span.ends[0][2], span.ends[1][0], span.ends[1][1], span.ends[1][2])}
				return true
			}
			return false
		})
		if collision != nil {
			return collision
		}
		occupied.add(cells, len(spans))
		spans = append(spans, span)
	}
	return nil
}
