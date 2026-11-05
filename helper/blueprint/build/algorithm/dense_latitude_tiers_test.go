package build

import (
	"context"
	"testing"
)

// TestDenseLatitudeFiveTiers 职责：验证五档完整覆盖原生纬带且不把极区压成一圈；使用：区域规则回归；边界：分类测试不代表高纬生成已经接入。
func TestDenseLatitudeFiveTiers(t *testing.T) {
	seen := map[int]int{}
	for _, sample := range []struct{ tier, hemisphere, low, high, count int }{
		{1, 0, -80, 80, 1}, {2, 1, 81, 130, 1}, {2, -1, -130, -81, 1},
		{3, 1, 131, 155, 1}, {3, -1, -155, -131, 1}, {4, 1, 156, 180, 1}, {4, -1, -180, -156, 1},
		{5, 1, 181, 249, 8}, {5, -1, -249, -181, 8},
	} {
		bands := densityLatitudeTierBands(sample.tier, sample.hemisphere, 200)
		if len(bands) != sample.count || bands[0].low != sample.low || bands[len(bands)-1].high != sample.high {
			t.Fatalf("第%d档半球%d范围错误：%+v", sample.tier, sample.hemisphere, bands)
		}
		for _, band := range bands {
			for row := band.low; row <= band.high; row++ {
				seen[row]++
				if longitudeSegmentCount(row, 200) != band.segments {
					t.Fatalf("纬圈%d经度段数不匹配", row)
				}
			}
		}
	}
	for row := -249; row <= 249; row++ {
		if seen[row] != 1 {
			t.Fatalf("纬圈%d归属%d档，应恰好归属一档", row, seen[row])
		}
	}
	for _, zone := range []string{"ring0-center", "ring6-north-center", "ring2-center", "ring1-north-center", "ring2-west-center", "ring2-north-unknown", "ring2-north-center-extra"} {
		if _, ok := densityLatitudeSelectionFor(zone); ok {
			t.Errorf("非法区域被接受：%s", zone)
		}
	}
	for _, sample := range []struct{ old, canonical string }{{"equator", "ring1-center"}, {"equator-north", "ring1-north"}, {"equator-south", "ring1-south"}, {"north-mid", "ring2-north-north"}, {"south-mid", "ring2-south-south"}, {"north-high", "ring3-north-north"}, {"south-high", "ring3-south-south"}} {
		old, ok := densityLatitudeSelectionFor(sample.old)
		current, valid := densityLatitudeSelectionFor(sample.canonical)
		if !ok || !valid || old != current {
			t.Errorf("旧选项%s未保持含义", sample.old)
		}
	}
}

// TestDenseSecondTierMatrix 职责：验证第2档两半球三位置的完整矩阵布局；使用：逐档交付；边界：与第1档共用严格产能及单塔回归。
func TestDenseSecondTierMatrix(t *testing.T) {
	testDenseMatrixRegions(t, []float64{240, 480}, []string{"ring2-north-center", "ring2-north-south", "ring2-north-north", "ring2-south-center", "ring2-south-south", "ring2-south-north"}, []int{1, 2, 3, 4})
}

// TestDenseLatitudeOptionContracts 职责：保证所有展示选项可被统一入口解析，旧键保持位置且未接入档位不会伪装可用；使用：元信息契约回归；边界：不替代生成验收。
func TestDenseLatitudeOptionContracts(t *testing.T) {
	seen := map[string]bool{}
	for _, option := range DensityLatitudeOptions() {
		if seen[option.Value] {
			t.Fatalf("重复选项%s", option.Value)
		}
		seen[option.Value] = true
		input := SurfaceLayout{Mode: "autoTropic", LatitudeZone: option.Value}
		normalized, err := normalizeSurfaceLayout(&input)
		if err != nil || normalized == nil {
			t.Fatalf("展示选项%s不能被解析：%v", option.Value, err)
		}
		if input.Mode != "autoTropic" || input.GridSegment != 0 {
			t.Fatal("解析修改了调用方请求")
		}
		if normalized.LatitudeZone != option.Value || normalized.ExpandAdjacent {
			t.Fatalf("选区%s被隐式换区", option.Value)
		}
	}
	for _, zone := range []string{"ring4-south-north", "ring5-north-center"} {
		if _, err := normalizeSurfaceLayout(&SurfaceLayout{Mode: "autoTropic", LatitudeZone: zone}); err == nil {
			t.Fatalf("未接入的档位%s被作为单纬带接受", zone)
		}
	}
	for _, zone := range []string{"ring1-center", "ring1-north", "ring1-south", "ring2-north-north", "ring2-south-south", "ring3-north-center", "ring3-south-south"} {
		if _, err := normalizeSurfaceLayout(&SurfaceLayout{Mode: "autoTropic", LatitudeZone: zone}); err != nil {
			t.Fatalf("规范选项%s被拒绝：%v", zone, err)
		}
	}
}

// TestDenseThirdTierMatrix 职责：验证25格环宽内的第3档矩阵；使用：逐档交付；边界：只限制纬向环宽，经向按实际产能展开。
func TestDenseThirdTierMatrix(t *testing.T) {
	testDenseMatrixRegions(t, []float64{240, 480}, []string{"ring3-north-center", "ring3-north-south", "ring3-north-north", "ring3-south-center", "ring3-south-south", "ring3-south-north"}, []int{1, 2, 3, 4})
}

// TestDenseThirdTierAnchorBudget 职责：允许窄环合法长矩形，同时拒绝纬向超出25格或绕满整圈；使用：容量边界回归；边界：容量通过不代表碰撞通过。
func TestDenseThirdTierAnchorBudget(t *testing.T) {
	ctx := context.WithValue(context.Background(), densityLatitudeKey{}, "ring3-north-center")
	if denseLatitudeHeightLimit(ctx, 100) != 17 {
		t.Fatal("普通模式的原接线预算被密铺调整改变")
	}
	if denseLatitudeHeightLimit(context.WithValue(ctx, denseCollisionKey{}, true), 100) != 21 {
		t.Fatal("密铺没有获得25格环宽内的21格工作预算")
	}
	for _, sample := range []struct {
		zone   string
		anchor int
	}{{"ring3-north-center", 131}, {"ring3-south-center", -155}} {
		layout := &SurfaceLayout{Mode: "explicitTropic", GridSegment: 200, LatitudeZone: sample.zone}
		anchors := surfaceLayoutAnchorCandidates(layout, 90, 25)
		if len(anchors) != 1 || anchors[0] != sample.anchor {
			t.Fatalf("25格完整窄环被额外扣除容量：%s %v", sample.zone, anchors)
		}
		if len(surfaceLayoutAnchorCandidates(layout, 90, 26)) != 0 {
			t.Fatal("纬向26格超出第3档却被接受")
		}
		if len(surfaceLayoutAnchorCandidates(layout, 601, 25)) != 0 {
			t.Fatal("经向超出原生整圈却被接受")
		}
	}
}
