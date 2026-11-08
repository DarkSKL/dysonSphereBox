package build

// 文件职责：验证共享原料入口不丢失规范份额、真实带宽或监测器。
// 使用：窄环大产线共享输入回归；边界：必须走完整生成及严格解码，不只比较规划数量。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"context"
	"math"
	"testing"
)

// TestDenseExternalNetworks 职责：证明同原料只暴露所需带宽数量的入口；使用：矩阵与实际监测；边界：保留全部内部规范边，断链或跨物料共享不得通过。
func TestDenseExternalNetworks(t *testing.T) {
	for _, monitor := range []bool{false, true} {
		ctx := context.WithValue(context.Background(), denseMaterialNetworksKey{}, true)
		plan, err := quant.Solve(ctx, quant.Request{Dataset: "Vanilla", Targets: []quant.Target{{ItemID: 6001, RatePerMinute: 480}}})
		if err != nil {
			t.Fatal(err)
		}
		result, err := Generate(ctx, Request{Plan: plan, Mode: 1, Density: densityDense, Setting: Setting{BeltLv: 3, InserterLv: 4, EnableFlowMonitor: monitor}, SurfaceLayout: &SurfaceLayout{Mode: "autoTropic", LatitudeZone: "ring4-north-south"}})
		if err != nil {
			t.Fatal(err)
		}
		doc, err := blueprint.Decode(result.Blueprint, true)
		if err != nil {
			t.Fatal(err)
		}
		loads := map[int]float64{}
		materials := map[int]int{}
		totals := map[int]float64{}
		for _, p := range result.LogisticsSummary.ExternalPorts {
			if p.Kind != "external-input" {
				continue
			}
			if materials[p.BuildingIndex] != 0 && materials[p.BuildingIndex] != p.ItemID {
				t.Fatal("共享了不同物料")
			}
			materials[p.BuildingIndex] = p.ItemID
			loads[p.BuildingIndex] += p.RatePerMinute
			totals[p.ItemID] += p.RatePerMinute
		}
		for _, raw := range plan.RawInputs {
			if math.Abs(raw.RatePerMinute-totals[raw.ItemID]) > 1e-6 {
				t.Fatal("共享入口缺失原料份额")
			}
		}
		for index, load := range loads {
			if load > 1800+1e-6 || !surfaceIsBelt(doc.Buildings[index].ItemID) {
				t.Fatal("入口超带宽或缺少实体")
			}
		}
		if len(loads) != 2 || result.LogisticsSummary.Metrics.ExternalInputCount != 2 {
			t.Fatalf("应该只有铁矿、铜矿两个独立带头，实际%d", len(loads))
		}
		t.Logf("monitor=%t entities=%d footprint=%dx%d inputs=%d", monitor, result.BuildingCount, result.SurfacePlan.LayoutWidth, result.SurfacePlan.LayoutHeight, len(loads))
	}
}
