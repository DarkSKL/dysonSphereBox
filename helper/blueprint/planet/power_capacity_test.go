package planet

// 文件职责：验证燃料机组空间失败回传真实已落位容量；使用：规划容量反馈；边界：部分蓝图仅用于测试，不作为成功产物交付。

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/quant"
)

// TestFuelCapacityFeedbackUsesPlacedGenerators 职责：只容一台恒星的窄陆地上回传 72 MW；使用：真实装配、地形与严格编解码；边界：两台合同不能因一台落位而成功或提前回填燃料端口。
func TestFuelCapacityFeedbackUsesPlacedGenerators(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	input := ProductionRequest{Plan: industryplan.PlanetPlan{PlanetID: 101,
		Power: &industryplan.PowerPlan{FacilityID: 2210, FuelItemID: 1803, Count: 2, PowerMW: 144, FuelRatePerMinute: 1.2, RequiredMW: 100},
		Plan:  quant.Result{Productions: []quant.Production{{PowerMW: 90}}},
		Ports: []industryplan.Port{{ItemID: 1803, Kind: "import", PeerPlanetID: 102, RatePerMinute: 1.2}, {ItemID: 1803, Kind: "power-fuel", RatePerMinute: 1.2}}},
		Options: ProductionOptions{Setting: build.Setting{InserterLv: 3}}}
	request := Request{PlanetID: 101, wet: func(p Position) (bool, bool) {
		return math.Abs(p.Latitude) > .04 || p.Longitude < -.04 || p.Longitude > .12, true
	}}
	actual := industryplan.PlanetActual{}
	err = appendPower(ctx, &request, input, &actual, catalog)
	var feedback *industryplan.PowerLoadError
	if !errors.As(err, &feedback) || feedback.AvailableMW == nil || *feedback.AvailableMW != 72 || feedback.RequiredMW != 100 {
		t.Fatalf("未回传真实单台容量：%v，%+v", err, feedback)
	}
	var detail *Error
	if !errors.As(err, &detail) || detail.Code != "PLANET_POWER" || actual.Power != nil || len(actual.Ports) != 0 {
		t.Fatalf("部分供电被当作成功或丢失原错误：%v，%+v", err, actual)
	}
	built, err := Build(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := blueprint.Decode(built.Blueprint, true)
	if err != nil || doc.Summary.ItemCounts[2210] != 1 || doc.Summary.ItemCounts[2104] != 1 || doc.Summary.ItemCounts[2013] != 1 {
		t.Fatalf("容量反馈与实际电厂、燃料塔或连接不符：%v，%v", err, doc.Summary.ItemCounts)
	}
	request = Request{PlanetID: 101, wet: func(Position) (bool, bool) { return true, true }}
	err = appendPower(ctx, &request, input, &industryplan.PlanetActual{}, catalog)
	if !errors.As(err, &feedback) || feedback.AvailableMW == nil || *feedback.AvailableMW != 0 || len(request.Objects) != 0 {
		t.Fatalf("全水面伪造供电容量：%v，%+v", err, feedback)
	}
}

// TestInstalledPowerCapacityKeepsActualOtherSources 职责：把已装其他电源和声明外电保留在容量反馈中；使用：混合能源及充放电模式；边界：计划但未落位的设备和重复源不能扩大容量。
func TestInstalledPowerCapacityKeepsActualOtherSources(t *testing.T) {
	request := Request{Objects: []Object{
		{Building: blueprint.Building{ItemID: 2203}},
		{Building: blueprint.Building{ItemID: 2209, Parameters: []int32{-1}}},
		{Building: blueprint.Building{ItemID: 2209, Parameters: []int32{1}}},
		{Building: blueprint.Building{ItemID: 2208, Parameters: []int32{1}}},
	}}
	power := &industryplan.PowerPlan{FacilityID: 2210, ExternalGridMW: 10, Sources: []industryplan.PowerSource{
		{FacilityID: 2210, Count: 2, PowerMW: 144},
		{FacilityID: 2203, Count: 2, PowerMW: .6},
		{FacilityID: 2203, Count: 1, PowerMW: .3},
		{FacilityID: 2209, Count: 2, PowerMW: 108, Mode: "discharge"},
		{FacilityID: 2209, Count: 1, Mode: "charge"},
		{FacilityID: 2208, Count: 1, PowerMW: 15},
		{FacilityID: 2205, Count: 100, PowerMW: 36},
	}}
	if got := installedPowerCapacity(request, power, 1, 72); math.Abs(got-136.3) > 1e-9 {
		t.Fatalf("计入未装电源、充电器或光子接收器：%.9f MW", got)
	}
}
