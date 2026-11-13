package build

// 文件职责：回归120/min极速传送带的两种物流塔数量、物料绑定和线路开销。
// 使用：共享边界及塔口分配优化；边界：保留完整前置、最低带臂等级和全部终检，不代表游戏内实测。

import (
	"context"
	"fmt"
	"math"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// TestDenseFastBeltFactory 职责：从默认量化到导出验证两塔整厂及就近接线；使用：用户案例回归；边界：不矿化前置，不通过升级带速降低塔数。
func TestDenseFastBeltFactory(t *testing.T) {
	plan, err := quant.Solve(context.Background(), quant.Request{Targets: []quant.Target{{ItemID: 2003, RatePerMinute: 120}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []int{3, 4} {
		t.Run(fmt.Sprintf("mode%d", mode), func(t *testing.T) {
			declared := map[string]bool{}
			winner := ""
			ctx := context.Background()
			if mode == 3 {
				ctx = WithProgress(ctx, func(p Progress) {
					for _, candidate := range p.Candidates {
						declared[candidate.ID] = true
					}
					if p.Kind == ProgressKindPreviewComplete {
						winner = p.WinnerCandidateID
					}
				})
			}
			result, err := Generate(ctx, Request{Plan: plan, Mode: mode, Density: densityDense, EmitPreviewDeltas: mode == 3, PreviewProtocolVersion: 4, SurfaceLayout: &SurfaceLayout{Mode: "explicitTropic", GridSegment: 200, LatitudeZone: "ring1-center"}})
			if err != nil {
				t.Fatal(err)
			}
			if mode == 3 && (winner == "" || winner != result.WinnerCandidateID || !declared[winner]) {
				t.Fatal("追加接线候选未按预览协议声明并提交获选结果")
			}
			doc, err := blueprint.Decode(result.Blueprint, true)
			if err != nil {
				t.Fatal(err)
			}
			towers := map[int32]blueprint.Building{}
			counts := map[int]int{}
			for _, record := range doc.Buildings {
				if surfaceIsStation(record.ItemID) {
					towers[record.Index] = record
				}
				if record.RecipeID > 0 {
					counts[int(record.RecipeID)]++
				}
			}
			if len(towers) != 2 {
				t.Fatalf("8类对外物料应使用2座塔，实际%d", len(towers))
			}
			for _, p := range plan.Productions {
				if counts[p.RecipeID] != p.MachineCount {
					t.Fatalf("配方%d机器数发生变化", p.RecipeID)
				}
			}
			used := map[[2]int32]bool{}
			for _, belt := range doc.Buildings {
				if !surfaceIsBelt(belt.ItemID) {
					continue
				}
				tower, input := towers[belt.InputObjIdx]
				slot := belt.InputFromSlot
				if !input {
					tower = towers[belt.OutputObjIdx]
					slot = belt.OutputToSlot
				}
				if tower.ItemID == 0 {
					continue
				}
				if slot < 0 || slot >= 12 || used[[2]int32{tower.Index, int32(slot)}] {
					t.Fatal("塔口重复或越界")
				}
				used[[2]int32{tower.Index, int32(slot)}] = true
				if input {
					inventory := int(tower.Parameters[193+int(slot)*4]) - 1
					slots := 4
					if mode == 4 {
						slots = 5
					}
					if inventory < 0 || inventory >= slots || len(belt.Parameters) == 0 || tower.Parameters[inventory*6] != belt.Parameters[0] || tower.Parameters[inventory*6+1] != 2 {
						t.Fatal("真实供给带的物料与塔库存或供需角色不符")
					}
					if tower.Parameters[192+int(slot)*4] != 1 {
						t.Fatal("原料端口方向错误")
					}
				} else if tower.Parameters[192+int(slot)*4] != 2 {
					t.Fatal("出货端口方向错误")
				}
			}
			inputs := map[int]float64{}
			target := 0.
			for _, port := range result.LogisticsSummary.ExternalPorts {
				if port.Kind == "external-input" {
					inputs[port.ItemID] += port.RatePerMinute
				}
				if port.Kind == "target-output" && port.ItemID == 2003 {
					target += port.RatePerMinute
				}
			}
			for _, raw := range plan.RawInputs {
				if math.Abs(inputs[raw.ItemID]-raw.RatePerMinute) > 1e-6 {
					t.Fatalf("原料%d的边界份额改变", raw.ItemID)
				}
			}
			if math.Abs(target-120) > 1e-6 {
				t.Fatal("极速带对外交付量不是120/min")
			}
			baselineArea, baselineLength := 2016., 2035.701
			if mode == 4 {
				baselineArea, baselineLength = 1833, 2188.55
			}
			metrics := result.LogisticsSummary.Metrics
			if metrics.EnvelopeArea > baselineArea+1e-6 || metrics.TotalBeltLengthMeters > baselineLength+1e-6 {
				t.Fatal("接线优化增加了既有获选方案的占地或线路")
			}
			t.Logf("buildings=%d towers=%d area=%.0f length=%.2f", result.BuildingCount, len(towers), metrics.EnvelopeArea, metrics.TotalBeltLengthMeters)
		})
	}
}

// TestMinimumStationPortAssignment 职责：覆盖逐项最近会占掉他人唯一近口的全局匹配；使用：塔口分配回归；边界：分配必须唯一且不接受非法距离。
func TestMinimumStationPortAssignment(t *testing.T) {
	assignment := minimumStationPortAssignment([][]float64{{1, 2}, {1, 100}})
	if len(assignment) != 2 || assignment[0] != 1 || assignment[1] != 0 {
		t.Fatalf("未找到最小总距离双射：%v", assignment)
	}
	for _, costs := range [][][]float64{nil, {{1}, {1, 2}}, {{math.NaN()}}, {{math.MaxFloat64, math.MaxFloat64}, {math.MaxFloat64, math.MaxFloat64}}} {
		if minimumStationPortAssignment(costs) != nil {
			t.Fatal("非法距离表未被拒绝")
		}
	}
}
