package build

// 文件职责：独立复核监测方向、失败回滚和已交付纬带布局。
// 使用：原生高纬接入回归；边界：不代替游戏内放置。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"context"
	"fmt"
	"math"
	"reflect"
	"testing"
)

// TestMonitorTracksDirectedBelt 职责：证明默认Yaw相等不再掩盖实际南北/反向输送；使用：四个方向及球面局部切线；边界：宿主缺失必须报错。
func TestMonitorTracksDirectedBelt(t *testing.T) {
	for _, direction := range []struct {
		x, y float64
		yaw  float32
	}{{0, 1, 0}, {1, 0, 90}, {0, -1, 180}, {-1, 0, 270}} {
		b, err := newBuilder(Setting{BeltLv: 3, InserterLv: 4})
		if err != nil {
			t.Fatal(err)
		}
		chain := b.byPath([]vector{{10 - direction.x, 10 - direction.y, 0}, {10, 10, 0}, {10 + direction.x, 10 + direction.y, 0}})
		monitor := b.make(2030, 10, 10, 0)
		records, _, _, err := b.records()
		if err != nil {
			t.Fatal(err)
		}
		if math.Abs(math.Remainder(float64(monitor.record.Yaw-direction.yaw), 360)) > 1e-3 {
			t.Fatalf("方向应为%g，实际%g", direction.yaw, monitor.record.Yaw)
		}
		if !boundaryFlowMonitorMatches(chain[1], monitor) {
			t.Fatal("真实宿主朝向不一致")
		}
		plan := makeSurfacePlan("explicitTropic", 200, 156, 156, 30, 20)
		plan.AnchorLatitudeGrid = 156
		encoding := &surfaceEncoding{plan: plan, areas: []blueprint.Area{{Index: 0, ParentIndex: -1, AreaSegments: 100, Width: 30, Height: 20}}}
		if err := b.alignFlowMonitorDirections(context.Background(), encoding, records, true); err != nil {
			t.Fatal(err)
		}
		if math.Abs(math.Remainder(float64(records[3].Yaw-direction.yaw), 360)) > .01 {
			t.Fatal("球面直线监测方向不符")
		}
		original := append([]blueprint.Building(nil), records...)
		if err := b.alignFlowMonitorDirections(context.Background(), encoding, records, true); err != nil || !reflect.DeepEqual(original, records) {
			t.Fatal("方向修正不幂等", err)
		}
		b.all = b.all[3:]
		if _, _, _, err := b.records(); err == nil {
			t.Fatal("孤立监测器不应通过")
		}
	}
}

// TestEstablishedLatitudeRegression 职责：防止高纬和监测器改动影响前三档及用户360一级物流反馈；使用：完整生成；边界：只检查明确的基准包络和实体数量，不要求候选ID固定。
func TestEstablishedLatitudeRegression(t *testing.T) {
	for _, zone := range []string{"equator", "equator-north", "equator-south", "ring2-north-north", "ring2-south-south", "ring3-north-south", "ring3-south-north"} {
		for _, mode := range []int{1, 2, 3, 4} {
			t.Run(fmt.Sprintf("%s-mode%d", zone, mode), func(t *testing.T) {
				plan, err := quant.Solve(context.Background(), quant.Request{Dataset: "Vanilla", Targets: []quant.Target{{ItemID: 6001, RatePerMinute: 480}}})
				if err != nil {
					t.Fatal(err)
				}
				result, err := Generate(context.Background(), Request{Plan: plan, Mode: mode, Density: densityDense, Setting: Setting{BeltLv: 3, InserterLv: 4}, SurfaceLayout: &SurfaceLayout{Mode: "autoTropic", LatitudeZone: zone}})
				if err != nil {
					t.Fatal(err)
				}
				doc, err := blueprint.Decode(result.Blueprint, true)
				if err != nil {
					t.Fatal(err)
				}
				facilities := 0
				for _, r := range doc.Buildings {
					if surfaceIsStation(r.ItemID) || r.ItemID == 2107 {
						facilities++
					}
					if r.RecipeID > 0 && math.Abs(math.Remainder(float64(r.Yaw), 90)) > 1e-3 {
						t.Fatal("机器倾斜")
					}
				}
				if mode >= 3 && facilities != 1 || mode == 2 && facilities != 3 {
					t.Fatal("设施数量回退", facilities)
				}
				if zone == "equator" && (result.LogisticsSummary.Metrics.EnvelopeArea > 1500 || result.SurfacePlan.LayoutWidth > 50 || result.SurfacePlan.LayoutHeight > 50) {
					t.Fatal("赤道包络退化")
				}
			})
		}
	}
	t.Run("用户360一级物流", func(t *testing.T) {
		plan, err := quant.Solve(context.Background(), quant.Request{Dataset: "Vanilla", Targets: []quant.Target{{ItemID: 6001, RatePerMinute: 360}}})
		if err != nil {
			t.Fatal(err)
		}
		result, err := Generate(context.Background(), Request{Plan: plan, Mode: 1, Density: densityDense, Setting: Setting{BeltLv: 1, InserterLv: 1}, SurfaceLayout: &SurfaceLayout{Mode: "autoTropic", LatitudeZone: "ring3-north-south"}})
		if err != nil {
			t.Fatal(err)
		}
		if result.SurfacePlan.AnchorLatitudeGrid != 131 || result.SurfacePlan.LayoutWidth > 50 || result.SurfacePlan.LayoutHeight > 17 {
			t.Fatal("360实测修复回退")
		}
	})
}
