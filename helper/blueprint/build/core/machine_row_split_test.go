package build

import (
	"DysonSphereBox/helper/quant"
	"context"
	"math"
	"testing"
)

func TestOrdinaryRowsUseActualMachineTemplate(t *testing.T) {
	dataset, err := quant.LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	off := false
	setting, err := normalizeSetting(Setting{BeltLv: 3, InserterLv: 4, MaxMachineInALine: 15, MaxLays: 3, NeedPower: &off})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int]bool{}
	items := []int{1101, 1201, 1114, 1123, 6001}
	if colliderProduct, ok := dataset.ByName["反物质"]; ok {
		items = append(items, colliderProduct.ID)
	} else {
		t.Fatal("missing collider product")
	}
	for _, item := range items {
		plan, err := quant.Solve(context.Background(), quant.Request{Targets: []quant.Target{{ItemID: item, RatePerMinute: 600}}})
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range plan.Productions {
			if seen[p.MachineType] {
				continue
			}
			seen[p.MachineType] = true
			recipe, err := datasetRecipeForProduction(dataset, p)
			if err != nil {
				t.Fatal(err)
			}
			for _, span := range []float64{8, 12, 18} {
				b, err := newBuilder(setting)
				if err != nil {
					t.Fatal(err)
				}
				b.policy, err = newAssemblyPolicy(1, densityStandard)
				if err != nil {
					t.Fatal(err)
				}
				count := min(15, p.MachineCount)
				groups, err := b.layoutSpatialRows(p, recipe, []int{count}, span)
				if err != nil {
					t.Fatal(err)
				}
				sum := 0
				for _, n := range groups {
					sum += n
					line, err := b.buildLine(p, recipe, n, 1, 0, true)
					if err != nil {
						t.Fatal(err)
					}
					low, high := math.Inf(1), math.Inf(-1)
					for _, machine := range line.machines {
						low = math.Min(low, machine.a.x)
						high = math.Max(high, machine.a.x)
					}
					if high-low > span+1e-7 {
						t.Fatalf("ordinary type%d claimed span%g but actual machines span%g (%d machines)", p.MachineType, span, high-low, n)
					}
				}
				if sum != count {
					t.Fatal("row split changed machine count")
				}
			}
		}
	}
	if len(seen) < 6 {
		t.Fatalf("missing factory coverage: %v", seen)
	}
}
