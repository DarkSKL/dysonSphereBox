package build

// 文件职责：复现原版单次蓝图粘贴中密铺传送带的预览与节点合并边界。
// 使用：dense 最终带间检查；边界：只适用于整图单次粘贴，不取消带建筑占用、引用、端口和距离检查。
// 依据：本地 0.10.35.29151 的 BlueprintUtils、BuildTool_BlueprintPaste 及 belt-1/2/3 prefab。
// 三种 belt prefab 无 LODModelDesc，单次粘贴不创建带预览盒；禁止用全段粗管或假想节点盒代替该机制。

import (
	"DysonSphereBox/helper/blueprint"
	"context"
	"fmt"
	"math"
)

// denseCollisionKey 职责：限定本次请求使用密铺蓝图规则；使用：入口注入；边界：普通生成不隐式改变筛选。
type denseCollisionKey struct{}

// usesDenseBeltCollisions 职责：读取请求级带判定方式；使用：最终校验；边界：缺省仍使用普通路径。
func usesDenseBeltCollisions(ctx context.Context) bool {
	enabled, _ := ctx.Value(denseCollisionKey{}).(bool)
	return enabled
}

// denseBeltPositionKeys 职责：计算原版重叠蓝图预处理的两组位置桶；使用：识别会被合并的节点；边界：使用原版偏移和0.200001234567891米步长，不能用普通路由整格身份替代。
func denseBeltPositionKeys(p vector) [2][3]int64 {
	shifts := [2][3]float64{{.10123456789101112, .11234567891011121, .12345678910111212}, {.013579111315171921, .01987654321234568, .012357111317192328}}
	point := [3]float64{p.x, p.y, p.z}
	keys := [2][3]int64{}
	for i, shift := range shifts {
		for axis := range point {
			keys[i][axis] = (int64(math.Round((point[axis]+shift[axis])/.200001234567891)) + 1048576) & 0x1fffff
		}
	}
	return keys
}

// denseBeltMergePoint 职责：计算节点重合时采用的邻接加权位置；使用：节点合并判定；边界：前后两端都是实际带时才平滑，返回游戏float32精度。
func denseBeltMergePoint(index int, records []blueprint.Building, positions []vector, byIndex map[int32]int) vector {
	p := positions[index]
	a, okA := byIndex[records[index].InputObjIdx]
	c, okC := byIndex[records[index].OutputObjIdx]
	if !okA || !okC || !surfaceIsBelt(records[a].ItemID) || !surfaceIsBelt(records[c].ItemID) {
		return p
	}
	f := func(center, left, right float64) float64 {
		return float64(float32(.7)*float32(center) + float32(.15)*(float32(left)+float32(right)))
	}
	return vector{f(p.x, positions[a].x, positions[c].x), f(p.y, positions[a].y, positions[c].y), f(p.z, positions[a].z, positions[c].z)}
}

// validateDenseBeltNodes 职责：允许原版同批带交叠，并拒绝未经拓扑重映射的自动合并；使用：单次粘贴密铺出口；边界：既有场景与多次拖拽复制仍需游戏测试，未知带型号不借用该豁免。
func (encoding *surfaceEncoding) validateDenseBeltNodes(ctx context.Context, original []blueprint.Building) error {
	positions, err := encoding.surfaceWorldPositions(original)
	if err != nil {
		return err
	}
	records := append([]blueprint.Building(nil), original...)
	byIndex := make(map[int32]int, len(records))
	for i, r := range records {
		byIndex[r.Index] = i
	}
	for _, r := range records {
		if next, ok := byIndex[r.OutputObjIdx]; ok && surfaceIsBelt(r.ItemID) && surfaceIsBelt(records[next].ItemID) {
			records[next].InputObjIdx = r.Index
		}
	}
	buckets := [2]map[[3]int64]int{{}, {}}
	for i, r := range records {
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "密铺节点合并检查已取消或超时"}
		}
		if !surfaceIsBelt(r.ItemID) {
			continue
		}
		if r.ItemID < 2001 || r.ItemID > 2003 || r.ModelIndex != r.ItemID-1966 {
			return &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: "传送带原型不属于已核对的原版密铺预览规则"}
		}
		keys := denseBeltPositionKeys(positions[i])
		for pass, key := range keys {
			previous, exists := buckets[pass][key]
			if !exists {
				buckets[pass][key] = i
				continue
			}
			if records[previous].ModelIndex != r.ModelIndex {
				continue
			}
			buckets[pass][key] = i
			a, c := denseBeltMergePoint(i, records, positions, byIndex), denseBeltMergePoint(previous, records, positions, byIndex)
			distance := (a.x-c.x)*(a.x-c.x) + (a.y-c.y)*(a.y-c.y) + (a.z-c.z)*(a.z-c.z)
			if distance < .25 {
				return &blueprint.Error{Code: "GEN_E_DENSITY_MERGE", Message: fmt.Sprintf("密铺节点 %d（%v）与 %d（%v）会被游戏重叠处理合并，需要先重映射连接", r.Index, r.LocalOffset, records[previous].Index, records[previous].LocalOffset)}
			}
		}
	}
	return nil
}
