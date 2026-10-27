package build

// 文件职责：验证生成结构和归属；使用：go test；边界：不替代 Windows/游戏粘贴验收。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"context"
	"fmt"
	"strings"
	"testing"
)

// TestGenerateModes 职责：核对各模式配方机器、连接和站点；使用：生成回归；边界：仅结构级验收。
func TestGenerateModes(t *testing.T) {
	plan, err := quant.Solve(context.Background(), quant.Request{Targets: []quant.Target{{ItemID: 6001, RatePerMinute: 60}}})
	if err != nil {
		t.Fatal(err)
	}
	for mode := 1; mode <= 6; mode++ {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			r, err := Generate(context.Background(), Request{Plan: plan, Mode: mode})
			if err != nil {
				t.Fatal(err)
			}
			d, err := blueprint.Decode(r.Blueprint, true)
			if err != nil {
				t.Fatal(err)
			}
			if r.BuildingCount != len(d.Buildings) || r.SourcePlanID != plan.PlanID || len(r.Bindings) != len(plan.Productions) {
				t.Fatal("summary/bindings mismatch")
			}
			references := map[int32]bool{}
			occupied := map[blueprint.Point]bool{}
			for _, b := range d.Buildings {
				references[b.Index] = true
				if b.ItemID > 2000 && b.ItemID < 2010 {
					if occupied[b.LocalOffset] {
						t.Fatalf("duplicate belt at %+v", b.LocalOffset)
					}
					occupied[b.LocalOffset] = true
				}
			}
			for _, b := range d.Buildings {
				if b.InputObjIdx >= 0 && !references[b.InputObjIdx] {
					t.Fatal("dangling input")
				}
				if b.OutputObjIdx >= 0 && !references[b.OutputObjIdx] {
					t.Fatal("dangling output")
				}
			}
			for _, binding := range r.Bindings {
				p := plan.Productions[binding.ProductionIndex]
				machines := 0
				for _, index := range binding.BuildingIndices {
					b := d.Buildings[index]
					if int(b.ItemID) == p.MachineID && int(b.RecipeID) == p.RecipeID {
						machines++
					}
				}
				if machines != p.MachineCount {
					t.Fatalf("production %d got=%d want=%d", binding.ProductionIndex, machines, p.MachineCount)
				}
			}
			t.Logf("mode=%d buildings=%d warnings=%v", mode, len(d.Buildings), r.Warnings)
		})
	}
}

// TestSprayNetwork 职责：核对可选供剂网络和喷涂邻接；使用：普通/塔/总线模式；边界：仅验证结构连接，不宣称实际喷涂速率。
func TestSprayNetwork(t *testing.T) {
	d, _ := quant.LoadDataset("Vanilla")
	scheme := quant.Scheme{SchemeForRecipe: make([]quant.RecipeScheme, len(d.Recipes))}
	for i, r := range d.Recipes {
		if r.Proliferator&2 != 0 {
			scheme.SchemeForRecipe[i] = quant.RecipeScheme{ProductionMode: 2, IncreasePoints: 4}
		}
	}
	plan, err := quant.Solve(context.Background(), quant.Request{Targets: []quant.Target{{ItemID: 6001, RatePerMinute: 60}}, Scheme: scheme})
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []int{1, 3, 6} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			r, err := Generate(context.Background(), Request{Plan: plan, Mode: mode, Setting: Setting{NeedCoaterNet: true, BeltLv: 3, InserterLv: 4}})
			if err != nil {
				t.Fatal(err)
			}
			d, err := blueprint.Decode(r.Blueprint, true)
			if err != nil {
				t.Fatal(err)
			}
			if len(r.SharedBuildingIndices) == 0 {
				t.Fatal("missing common supply network")
			}
			for _, c := range d.Buildings {
				if c.ItemID != 2313 {
					continue
				}
				found := false
				for _, belt := range d.Buildings {
					if belt.ItemID > 2000 && belt.ItemID < 2010 && belt.LocalOffset.X == c.LocalOffset.X-1 && belt.LocalOffset.Y == c.LocalOffset.Y && belt.LocalOffset.Z == 1 {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("coater without supply belt %+v", c.LocalOffset)
				}
			}
		})
	}
}

// TestGeometryAndDirections 职责：固定几何表及方向 0 约定；使用：参考常量回归；边界：不是反推游戏槽位。
func TestGeometryAndDirections(t *testing.T) {
	s, err := normalizeSetting(Setting{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := newBuilder(s)
	if err != nil {
		t.Fatal(err)
	}
	line := b.straight(0, 0, 2, 0)
	if line[0].record.Yaw != -90 {
		t.Fatal(line[0].record.Yaw)
	}
	for slot := 0; slot < 12; slot++ {
		path := stationInner(vector{}, slot)
		if len(path) < 2 {
			t.Fatal(slot)
		}
	}
}

// TestMixedAndWhiteMatrix 职责：验证六原料及混料路径可写回；使用：高原料数量回归；边界：不声明游戏运行通过。
func TestMixedAndWhiteMatrix(t *testing.T) {
	for _, item := range []int{6001, 6006} {
		plan, err := quant.Solve(context.Background(), quant.Request{Targets: []quant.Target{{ItemID: item, RatePerMinute: 1}}})
		if err != nil {
			t.Fatal(err)
		}
		for _, mode := range []int{1, 3, 4, 6} {
			t.Run(fmt.Sprintf("%d-%d", item, mode), func(t *testing.T) {
				res, err := Generate(context.Background(), Request{Plan: plan, Mode: mode, Setting: Setting{UseMixed: true, BeltLv: 3, InserterLv: 4}})
				if err != nil {
					t.Fatal(err)
				}
				d, err := blueprint.Decode(res.Blueprint, true)
				if err != nil {
					t.Fatal(err)
				}
				count := 0
				occupied := map[blueprint.Point]bool{}
				firstIndex := map[blueprint.Point]int{}
				for bi, b := range d.Buildings {
					if b.ItemID > 2000 && b.ItemID < 2010 {
						if occupied[b.LocalOffset] {
							t.Fatalf("mixed duplicate belt first=%+v second=%+v", d.Buildings[firstIndex[b.LocalOffset]], b)
						}
						occupied[b.LocalOffset] = true
						firstIndex[b.LocalOffset] = bi
					}
					if _, ok := plan.Amounts["宇宙矩阵"]; ok && b.RecipeID == 75 {
						count++
					}
				}
				t.Logf("buildings=%d whiteRecipeRecords=%d warnings=%v", len(d.Buildings), count, res.Warnings)
			})
		}
	}
}

// TestBusOverflowWarning 职责：确保带宽不足不会静默截断；使用：高负荷蓝糖总线；边界：告警不是自动保证实际运力。
func TestBusOverflowWarning(t *testing.T) {
	plan, err := quant.Solve(context.Background(), quant.Request{Targets: []quant.Target{{ItemID: 6001, RatePerMinute: 600}}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Generate(context.Background(), Request{Plan: plan, Mode: 6})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "带宽") {
			found = true
		}
	}
	if !found {
		t.Fatal("missing bandwidth warning")
	}
	if len(result.Bindings) != len(plan.Productions) {
		t.Fatal("silently dropped production")
	}
}
