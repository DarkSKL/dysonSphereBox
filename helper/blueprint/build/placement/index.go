package placement

// 文件职责：为三维定向碰撞盒提供空间哈希宽相位。
// 使用：把适用同一碰撞规则的对象一起登记，寻找首个冲突。
// 边界：不决定建筑类别豁免；不同规则的对象应由调用方分开或另行筛选。

import (
	"context"
	"fmt"
	"math"
)

// OwnedBox 职责：把碰撞盒关联到最终建筑编号；使用：占位验证；边界：同一建筑的多个盒不会彼此构成冲突。
type OwnedBox struct {
	// Owner 职责：保存最终建筑编号；使用：冲突诊断；边界：须非负。
	Owner int
	// Geometry 职责：保存世界定向盒；使用：宽窄相位；边界：必须通过 Valid。
	Geometry Box
}

// cell 职责：索引世界八米立方格；使用：宽相位；边界：世界 XYZ 天然涵盖经度缝，不再以局部 X 取模。
type cell [3]int

// FirstConflict 职责：寻找两个不同建筑的正体积碰撞；使用：静态对象模型验证；边界：无冲突返回 -1/-1，未知或过大几何返回错误，取消立即终止。
func FirstConflict(ctx context.Context, boxes []OwnedBox) (int, int, error) {
	occupied := map[cell][]int{}
	for i, box := range boxes {
		if err := ctx.Err(); err != nil {
			return -1, -1, err
		}
		if box.Owner < 0 || !box.Geometry.Valid() {
			return -1, -1, fmt.Errorf("建筑 %d 的碰撞盒无效", box.Owner)
		}
		low, high := box.Geometry.Bounds()
		var minimum, maximum cell
		cellCount := 1
		for dimension := 0; dimension < 3; dimension++ {
			if math.Abs(low[dimension]) > 1e6 || math.Abs(high[dimension]) > 1e6 {
				return -1, -1, fmt.Errorf("建筑 %d 的碰撞范围超限", box.Owner)
			}
			minimum[dimension] = int(math.Floor(low[dimension] / 8))
			maximum[dimension] = int(math.Floor(high[dimension] / 8))
			count := maximum[dimension] - minimum[dimension] + 1
			if count > 256 || cellCount > 4096/count {
				return -1, -1, fmt.Errorf("建筑 %d 的碰撞盒覆盖格数超限", box.Owner)
			}
			cellCount *= count
		}
		seen := map[int]struct{}{}
		for x := minimum[0]; x <= maximum[0]; x++ {
			for y := minimum[1]; y <= maximum[1]; y++ {
				for z := minimum[2]; z <= maximum[2]; z++ {
					key := cell{x, y, z}
					for _, j := range occupied[key] {
						if _, checked := seen[j]; checked || box.Owner == boxes[j].Owner {
							continue
						}
						seen[j] = struct{}{}
						if box.Geometry.Overlaps(boxes[j].Geometry) {
							return boxes[j].Owner, box.Owner, nil
						}
					}
					occupied[key] = append(occupied[key], i)
				}
			}
		}
	}
	return -1, -1, nil
}
