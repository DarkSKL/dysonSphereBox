package planet

// 文件职责：验证稳定制造块只复用成功区域的尝试顺序；使用：暖重核与隔离回归；边界：全部候选仍经过真实地形、落位和严格编解码。

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/industryplan/search"
	"DysonSphereBox/helper/quant"
)

// TestManufacturingRegionHintIsolationAndFallback 职责：让同硬件稳定块跳过已知失败区域，并保留失效后的原顺序；使用：不同任务、星球、块与硬件回归；边界：用真实模板及干湿地面证明尝试顺序，不依赖计时断言。
func TestManufacturingRegionHintIsolationAndFallback(t *testing.T) {
	parent, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	parent = search.WithControl(parent, search.New(3*time.Minute, nil))
	ctx := WithTemplateCache(parent)
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	var northLow, northHigh, equatorLow, equatorHigh float64
	for _, region := range build.DensityLatitudeOptions() {
		switch region.Value {
		case "ring2-north-center":
			northLow, northHigh = region.LatitudeLow*math.Pi/180, region.LatitudeHigh*math.Pi/180
		case "equator":
			equatorLow, equatorHigh = region.LatitudeLow*math.Pi/180, region.LatitudeHigh*math.Pi/180
		}
	}
	if northHigh <= northLow || equatorHigh <= equatorLow {
		t.Fatal("生成器没有提供待核验纬带")
	}
	base := moduleRegionTestPlan(t, ctx, 300)
	needPower := true
	inputFor := func(plan quant.Result, planet int64, block string, belt int) ProductionRequest {
		input := ProductionRequest{Plan: industryplan.PlanetPlan{PlanetID: planet, Plan: plan, BlockIDs: []string{block}},
			Options: ProductionOptions{Setting: build.Setting{NeedPower: &needPower, BeltLv: belt, InserterLv: 3}}}
		for _, raw := range plan.RawInputs {
			input.Plan.Ports = append(input.Plan.Ports, industryplan.Port{ItemID: raw.ItemID, Kind: "external", RatePerMinute: raw.RatePerMinute})
		}
		input.Plan.Ports = append(input.Plan.Ports, industryplan.Port{ItemID: 1101, Kind: "export", PeerPlanetID: 999, RatePerMinute: plan.Targets[0].RatePerMinute})
		return input
	}
	assemble := func(label string, current context.Context, input ProductionRequest, low, high float64, wantRejectedRegion bool) {
		t.Helper()
		before, _ := json.Marshal(input)
		outside := 0
		req := Request{PlanetID: input.Plan.PlanetID, wet: func(p Position) (bool, bool) {
			wet := p.Latitude < low || p.Latitude > high
			if wet {
				outside++
			}
			return wet, true
		}}
		actual := industryplan.PlanetActual{}
		started := time.Now()
		if err := appendManufacturing(current, &req, input, &actual, catalog); err != nil {
			t.Fatalf("%s：%v", label, err)
		}
		if (outside > 0) != wantRejectedRegion {
			t.Fatalf("%s：拒绝地面探测%d次，预期需要重扫其他区域=%v", label, outside, wantRejectedRegion)
		}
		built, err := Build(current, req)
		if err != nil {
			t.Fatalf("%s完整编码：%v", label, err)
		}
		doc, err := blueprint.Decode(built.Blueprint, true)
		if err != nil || doc.Summary.ItemCounts[2302] != input.Plan.Plan.MachineCount {
			t.Fatalf("%s严格解码或机器数错误：%v，%v", label, err, doc.Summary.ItemCounts)
		}
		for _, want := range input.Plan.Ports {
			found := false
			for _, got := range actual.Ports {
				if got.ItemID == want.ItemID && got.Kind == want.Kind && got.PeerPlanetID == want.PeerPlanetID {
					found = math.Abs(got.RatePerMinute-want.RatePerMinute) < 1e-8
				}
			}
			if !found {
				t.Fatalf("%s复用了旧合同：期望%+v，得到%+v", label, want, actual.Ports)
			}
		}
		after, _ := json.Marshal(input)
		if !bytes.Equal(before, after) {
			t.Fatalf("%s改写了生产合同", label)
		}
		t.Logf("%s：%s，拒绝地面探测%d次，完整实体/端口/严格编解码通过", label, time.Since(started), outside)
	}
	input := inputFor(base, 101, "stable-a", 3)
	assemble("首次找到北纬区域", ctx, input, northLow, northHigh, true)
	assemble("同星同块暖重核", ctx, input, northLow, northHigh, false)
	assemble("需求变化硬件不变", ctx, inputFor(moduleRegionTestPlan(t, ctx, 260), 101, "stable-a", 3), northLow, northHigh, false)
	assemble("其他星球不沿用", ctx, inputFor(base, 102, "stable-a", 3), northLow, northHigh, true)
	assemble("普通新块不沿用", ctx, inputFor(base, 101, "stable-b", 3), northLow, northHigh, true)
	assemble("缺少稳定身份不沿用", ctx, inputFor(base, 101, "", 3), northLow, northHigh, true)
	assemble("硬件增加不沿用", ctx, inputFor(moduleRegionTestPlan(t, ctx, 375), 101, "stable-a", 3), northLow, northHigh, true)
	assemble("设置变化不沿用", ctx, inputFor(base, 101, "stable-a", 1), northLow, northHigh, true)
	assemble("其他任务不沿用", WithTemplateCache(parent), input, northLow, northHigh, true)
	assemble("原成功区域变为水面后回退", ctx, input, equatorLow, equatorHigh, true)
	assemble("更新为新的成功区域", ctx, input, equatorLow, equatorHigh, false)
}
