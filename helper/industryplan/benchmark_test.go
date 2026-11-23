package industryplan

import (
	"DysonSphereBox/helper/quant"
	"context"
	"fmt"
	"testing"
)

// benchmarkWorld 职责：构造规模固定的跨星系基线；使用：标准、低负载性能对比；边界：使用模拟地表资源与真实量化数据集。
func benchmarkWorld(planets, targets int, profile string) (Request, World) {
	request := Request{Preference: "logistics", Profile: profile, DeliveryPlanetIDs: map[int]int64{}}
	world := World{}
	items := []int{1101, 1104, 1102, 1105, 1301, 1302, 1106, 1103, 1109, 1201, 1202, 1113}
	for i := 0; i < planets; i++ {
		id := int64((i/3+1)*100 + i%3 + 1)
		resources := []Resource{}
		for j := 0; j < 6; j++ {
			if (i+j)%3 != 0 {
				resources = append(resources, Resource{ItemID: 1001 + j, Kind: "solid", Count: 300})
			}
		}
		world.Planets = append(world.Planets, Planet{
			ID: id, StarID: int64(i/3 + 1), Position: [3]float64{float64(i/3)*2400000 + float64(i%3)*65000, float64(i%4) * 30000, 0},
			Buildable: true, ResourcesKnown: true, Resources: resources,
		})
	}
	for i, item := range items[:targets] {
		request.Quant.Targets = append(request.Quant.Targets, quant.Target{ItemID: item, RatePerMinute: float64((i + 1) * 12)})
		request.DeliveryPlanetIDs[item] = world.Planets[(i*5)%planets].ID
	}
	return request, world
}

// BenchmarkPlan 职责：计量纯 Go 星球分工与守恒复核；使用：性能对比；边界：复核回调为映射检查，未执行蓝图布局与桌面桥。
func BenchmarkPlan(b *testing.B) {
	for _, example := range []struct {
		name             string
		planets, targets int
	}{{"small", 3, 2}, {"medium", 12, 6}, {"large", 48, 12}} {
		for _, profile := range []string{"low", "standard"} {
			b.Run(fmt.Sprintf("%s/%s", example.name, profile), func(b *testing.B) {
				request, world := benchmarkWorld(example.planets, example.targets, profile)
				active, peak, candidates, validations := 0, 0, 0, 0
				validate := func(_ context.Context, site PlanetPlan) (PlanetActual, error) {
					active++
					peak = max(peak, active)
					validations++
					actual := PlanetActual{Ports: site.Ports}
					active--
					return actual, nil
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					result, err := Plan(context.Background(), request, world, validate, nil)
					if err != nil || !result.Verified {
						b.Fatalf("固定案例未通过：状态 %s，说明 %v，错误 %v", result.Status, result.Warnings, err)
					}
					candidates += result.Candidates
				}
				b.ReportMetric(float64(candidates)/float64(b.N), "candidates/op")
				b.ReportMetric(float64(validations)/float64(b.N), "callbacks/op")
				b.ReportMetric(float64(peak), "active-callbacks")
			})
		}
	}
}
