package build

// 文件职责：检验极区内部子环调度的整厂能力；使用：第5档开发验收；边界：不替代多Area或游戏放置。
import (
	"DysonSphereBox/helper/quant"
	"context"
	"fmt"
	"testing"
)

func TestPolarMatrix(t *testing.T) {
	for _, scenario := range []struct{ item, rate int }{{6001, 240}, {6001, 480}, {2304, 30}, {1301, 120}, {1202, 120}} {
		plan, err := quant.Solve(context.Background(), quant.Request{Dataset: "Vanilla", Settings: quant.Settings{MineralizeList: map[string]bool{"硅石": true}}, Targets: []quant.Target{{ItemID: scenario.item, RatePerMinute: float64(scenario.rate)}}})
		if err != nil {
			t.Fatal(err)
		}
		for _, zone := range []string{"ring5-north-center", "ring5-north-south", "ring5-north-north", "ring5-south-center", "ring5-south-south", "ring5-south-north"} {
			for _, mode := range []int{1, 2, 3, 4} {
				t.Run(fmt.Sprintf("item%d-rate%d-%s-mode%d", scenario.item, scenario.rate, zone, mode), func(t *testing.T) {
					ctx := WithProgress(context.Background(), func(p Progress) {
						if p.Stage == "polar-rejected" || p.Stage == "layout-rejected" {
							t.Log(p.Message)
						}
					})
					result, err := Generate(ctx, Request{Plan: plan, Mode: mode, Density: densityDense, Setting: Setting{BeltLv: 3, InserterLv: 4}, SurfaceLayout: &SurfaceLayout{Mode: "autoTropic", LatitudeZone: zone}})
					if err != nil {
						t.Fatal(err)
					}
					t.Logf("anchor=%d %dx%d", result.SurfacePlan.AnchorLatitudeGrid, result.SurfacePlan.LayoutWidth, result.SurfacePlan.LayoutHeight)
				})
			}
		}
	}
}
