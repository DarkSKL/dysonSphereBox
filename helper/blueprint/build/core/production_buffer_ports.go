package build

// 文件职责：从最终球面记录复核储液罐的已提取端口及双向开关。
// 使用：动态设施共同检查；边界：只接受真实四槽及向外短接头，不以连接索引代替几何。

import (
	"context"
	"fmt"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
)

// validateTankBufferPorts 职责：验证所有罐口唯一性、开关和端点朝向；使用：平面及球面最终验收；边界：按当前原型目录米制几何检查。
func (encoding *surfaceEncoding) validateTankBufferPorts(ctx context.Context, records []blueprint.Building) error {
	tanks := map[int32]int{}
	for i, record := range records {
		if record.ItemID == 2106 {
			tanks[record.Index] = i
		}
	}
	if len(tanks) == 0 {
		return nil
	}
	positions, err := encoding.surfaceWorldPositions(records)
	if err != nil {
		return err
	}
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return err
	}
	geometry, err := catalog.Geometry(2106, 121)
	if err != nil {
		return err
	}
	used := map[[2]int32]bool{}
	directions := map[int32][2]int{}
	fail := func(message string) error {
		return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "储液罐端口：" + message}
	}
	for i, belt := range records {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !surfaceIsBelt(belt.ItemID) {
			continue
		}
		for end, index := range []int32{belt.InputObjIdx, belt.OutputObjIdx} {
			j, exists := tanks[index]
			if !exists {
				continue
			}
			slot := belt.InputFromSlot
			if end == 1 {
				slot = belt.OutputToSlot
			}
			key := [2]int32{index, int32(slot)}
			if slot < 0 || int(slot) >= len(geometry.PortPoses) || used[key] {
				return fail("重复或无效的实际带槽")
			}
			used[key] = true
			pose, err := encoding.recordPose(positions[j], records[j].Yaw, 0, 0)
			if err != nil {
				return err
			}
			port := surfaceSlotPose(pose, geometry.PortPoses[slot], false)
			point := placement.Vector{positions[i].x, positions[i].y, positions[i].z}
			delta := point.Sub(port.position)
			along := delta.Dot(port.axes[2])
			// 端口允许短连接延伸；横向及高度偏移不得冒充相邻槽。
			perpendicular := delta.Sub(port.axes[2].Scale(along))
			if along < -.1 || along > 1.1 || perpendicular.Dot(perpendicular) > .35*.35 {
				return fail(fmt.Sprintf("带%d未对准罐%d槽%d", belt.Index, index, slot))
			}
			count := directions[index]
			count[end]++
			directions[index] = count
		}
	}
	for index, i := range tanks {
		p := records[i].Parameters
		if len(p) != 2 || p[0] != 1 || p[1] != 1 || directions[index][0] == 0 || directions[index][1] == 0 {
			return fail("缺少开启的进带或出带")
		}
	}
	return nil
}
