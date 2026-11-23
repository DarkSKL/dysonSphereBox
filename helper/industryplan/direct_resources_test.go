package industryplan

// 文件职责：验证直接采集必须经过真实来源解析和逐星布局回执。使用小型星区测试规划契约，存档几何由整星测试覆盖。

import (
	"context"
	"errors"
	"strings"
	"testing"

	"DysonSphereBox/helper/quant"
)

// TestDirectResourceResolution 校验固矿候选必须经过解析与落位验证，已知液体会交给抽水布局。
func TestDirectResourceResolution(t *testing.T) {
	req := Request{CenterPlanetID: 101, Quant: quant.Request{
		Targets:  []quant.Target{{ItemID: 1003, RatePerMinute: 30}, {ItemID: 1116, RatePerMinute: 25}},
		Settings: quant.Settings{DirectResourceItemIDs: []int{1003, 1116}},
	}}
	resources := []Resource{{ItemID: 1003, Kind: "solid", Source: "candidate"}, {ItemID: 1116, Kind: "liquid", Source: "generated", UnitRatePerMinute: 50}}
	world := World{Planets: []Planet{{ID: 101, StarID: 1, Buildable: true, Resources: resources}}}
	resolved, validated := 0, 0
	world.ResolveResources = func(_ context.Context, id int64) ([]Resource, bool, error) {
		resolved++
		return []Resource{{ItemID: 1003, Kind: "solid", Source: "saved", Count: 1}, resources[1]}, true, nil
	}
	validator := func(_ context.Context, site PlanetPlan) (PlanetActual, error) {
		validated++
		if site.PlanetID != 101 || len(site.Mines) != 2 || site.Mines[0].FacilityID != 2301 || site.Mines[1].FacilityID != 2306 {
			t.Fatalf("真实布局必须收到矿机与抽水设备任务：%+v", site.Mines)
		}
		return PlanetActual{Ports: site.Ports, Artifact: new(int)}, nil
	}
	result, err := Plan(context.Background(), req, world, validator, nil)
	if err != nil || resolved != 1 || validated == 0 || result.Status != "conditional" {
		t.Fatalf("直接采集绕过精化/校验或变成外供：%s %d/%d %v %v", result.Status, resolved, validated, result.External, err)
	}
	for _, external := range result.External {
		if external.Kind != "grid-power" {
			t.Fatalf("直接采集项目被改成外供：%+v", external)
		}
	}
	result, err = Plan(context.Background(), req, world, func(_ context.Context, _ PlanetPlan) (PlanetActual, error) {
		return PlanetActual{}, &SourcePlacementError{ItemID: 1116, Err: errors.New("没有合规的抽水落点")}
	}, nil)
	if err != nil || result.Status == "ready" || result.Status == "conditional" {
		t.Fatalf("物理落位失败的直接抽水仍可交付：%s %v", result.Status, err)
	}
	world.ResolveResources = nil
	if _, err = Plan(context.Background(), req, world, nil, nil); err == nil || !strings.Contains(err.Error(), "硅石") {
		t.Fatalf("无法精化的候选不应成为采集选择：%v", err)
	}
	disabled := false
	req.EnableMining = &disabled
	if _, err = Plan(context.Background(), req, world, nil, nil); err == nil || !strings.Contains(err.Error(), "启用新建采矿设施") {
		t.Fatalf("关闭采集后不应变成外供：%v", err)
	}
}
