package build

// 文件职责：让相邻闭合配送行反向交错，保留端点身份后作为较紧凑的球面块。
// 使用：自动球面的独立有限候选；边界：不增加建筑、不修改物流份额，不旋转非对称占地或副产物回收行。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
)

// pairFlySurfaceBlocks 职责：以真实轮廓把相邻两行构成闭合组合块；使用：完整配送行与回收路装配后；边界：原单行映射保持不变，失败只丢弃当前独立 builder 候选。
func (b *builder) pairFlySurfaceBlocks(ctx context.Context) error {
	protected := map[*placed]bool{}
	for _, endpoint := range b.flyEndpoints {
		if endpoint.leftoverAnchor != nil {
			protected[endpoint.box] = true
		}
	}
	merged := make([]surfaceBlockRange, 0, (len(b.surfaceBlocks)+1)/2)
	rotated := 0
	for start := 0; start < len(b.surfaceBlocks); start += 2 {
		rows := b.surfaceBlocks[start:min(start+2, len(b.surfaceBlocks))]
		packing := &connectedPacking{cells: map[[3]int][]int{}}
		nextY := 0.
		for index, block := range rows {
			if ctx.Err() != nil {
				return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "配送行交错装箱已取消或超时"}
			}
			entities := b.all[block.begin:block.end]
			rotate := index == 1
			for _, entity := range entities {
				low, high := footprint(entity.record.ItemID)
				// 固定朝向 footprint 只有中心对称时才能在半周旋转后原样使用。
				// 回收行还依赖原料箱的方向与身份，不在本候选内改写该约束。
				if low.x != -high.x || low.y != -high.y || protected[entity] {
					rotate = false
				}
			}
			if rotate {
				for _, entity := range entities {
					entity.a.x, entity.a.y = -entity.a.x, -entity.a.y
					entity.b.x, entity.b.y = -entity.b.x, -entity.b.y
					entity.record.Yaw = float32(math.Mod(float64(entity.record.Yaw)+180, 360))
					entity.record.Yaw2 = float32(math.Mod(float64(entity.record.Yaw2)+180, 360))
				}
				rotated++
			}
			low, high := bounds(entities)
			contour, err := b.connectedBlockShape(ctx, &transportRow{}, entities)
			if err != nil {
				return err
			}
			x, y, err := packing.place(ctx, contour, low, high, 0, nextY, 300)
			if err != nil {
				return err
			}
			if index == 1 {
				// 切平面刚好可行的电杆间距在球面上会缩短，预留一格；
				// 最终纬带仍逐块执行真实占位、间距与供电检查。
				y++
			}
			shift(entities, x-low.x, y-low.y, 0)
			nextY = y + math.Ceil(high.y-low.y) + 3
		}
		merged = append(merged, surfaceBlockRange{begin: rows[0].begin, end: rows[len(rows)-1].end})
	}
	b.surfaceBlocks = merged
	b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "fly-surface-paired-rows", Status: "passed", Message: fmt.Sprintf("完整配送单行按相邻两行组成 %d 个球面闭合块，其中 %d 行刚性旋转半周；非对称设备和回收行保持原方向，原单行身份、物料与所有真实实体保持不变；组合块仍须完整球面终检", len(merged), rotated)})
	return nil
}
