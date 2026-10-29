package build

// 文件职责：复核球面最终位置上的电力节点间距。
// 使用：动态设施预检；边界：按当前原版蓝图预览规则检查蓝图内部节点，不检查既有工厂节点。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
)

// surfaceIsPowerNode 职责：识别已核对 PowerDesc.node 的原版电力原型；使用：电力间距约束；边界：蓄电器自身不发起检查，但其他节点仍检查与它的距离。
func surfaceIsPowerNode(itemID int16) bool {
	switch itemID {
	case 2201, 2202, 2203, 2204, 2205, 2206, 2207, 2208, 2209, 2210, 2211, 2212, 2213:
		return true
	default:
		return false
	}
}

// surfacePowerDistanceLimit 职责：返回原版蓝图内部节点对的距离平方阈值；使用：间距窄相位；边界：仅两风机、两地热采用更大间距，两蓄电器不执行该规则。
func surfacePowerDistanceLimit(left, right int16) float32 {
	if (left == 2206 || left == 2207) && (right == 2206 || right == 2207) {
		return 0
	}
	if left == 2203 && right == 2203 {
		return 110.25
	}
	if left == 2213 && right == 2213 {
		return 144
	}
	return 12.25
}

// surfacePowerDistanceSquared 职责：复现游戏 Vector3 的单精度距离平方；使用：电力节点硬阈值；边界：不额外增加容差。
func surfacePowerDistanceSquared(a, b vector) float32 {
	x, y, z := float32(a.x)-float32(b.x), float32(a.y)-float32(b.y), float32(a.z)-float32(b.z)
	return x*x + y*y + z*z
}

// surfacePowerNodeConflict 职责：以三维空间哈希返回首对低于游戏阈值的节点；使用：球面世界坐标校验；边界：严格小于对应阈值，恰好等于阈值不构成冲突。
func surfacePowerNodeConflict(ctx context.Context, records []blueprint.Building, positions []vector) (int, int, error) {
	if len(records) != len(positions) {
		return -1, -1, fmt.Errorf("电力节点坐标数量不一致")
	}
	occupied := map[[3]int][]int{}
	for i, left := range records {
		if ctx.Err() != nil {
			return -1, -1, ctx.Err()
		}
		if !surfaceIsPowerNode(left.ItemID) {
			continue
		}
		point := positions[i]
		cell := [3]int{int(math.Floor(point.x / 12)), int(math.Floor(point.y / 12)), int(math.Floor(point.z / 12))}
		for x := -1; x <= 1; x++ {
			for y := -1; y <= 1; y++ {
				for z := -1; z <= 1; z++ {
					for _, j := range occupied[[3]int{cell[0] + x, cell[1] + y, cell[2] + z}] {
						if surfacePowerDistanceSquared(point, positions[j]) < surfacePowerDistanceLimit(left.ItemID, records[j].ItemID) {
							return j, i, nil
						}
					}
				}
			}
		}
		occupied[cell] = append(occupied[cell], i)
	}
	return -1, -1, nil
}

// validateSurfacePowerNodeSpacing 职责：用游戏纬圈映射后的三维弦长检查电力节点间距；使用：动态设施预检；边界：不把平面坐标距离误当球面距离，也不检查地形和已有设施。
func (encoding *surfaceEncoding) validateSurfacePowerNodeSpacing(ctx context.Context, records []blueprint.Building) error {
	positions, err := encoding.surfaceWorldPositions(records)
	if err != nil {
		return err
	}
	left, right, err := surfacePowerNodeConflict(ctx, records, positions)
	if err != nil {
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "电力节点间距检查已取消或超时"}
		}
		return err
	}
	if left >= 0 {
		limit := math.Sqrt(float64(surfacePowerDistanceLimit(records[left].ItemID, records[right].ItemID)))
		return &blueprint.Error{Code: "GEN_E_COLLISION", Message: fmt.Sprintf("电力设施相隔太近：建筑 %d（物品 %d）与建筑 %d（物品 %d），球面间距小于 %g 米", records[left].Index, records[left].ItemID, records[right].Index, records[right].ItemID, limit)}
	}
	return nil
}
