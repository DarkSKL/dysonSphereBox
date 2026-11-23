package planet

import (
	"context"
	"math"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/geometry"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/industryplan"
)

// TestReceivingTowerUsesLandBetweenSearchRows 验证两条常用纬线之间仅够一座收货塔的小块干地可以实际施工。
func TestReceivingTowerUsesLandBetweenSearchRows(t *testing.T) {
	ctx := context.Background()
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	latitudeStep, _ := geometry.LatitudeStep32(geometry.Vanilla200)
	latitude := float64(14 * float32(latitudeStep))
	longitudeStep, _ := geometry.PasteLongitudeStep32(geometry.Radians(latitude), geometry.Vanilla200)
	longitude := 37 * float64(longitudeStep)
	margin := groundRadius(catalog, blueprint.Building{ItemID: 2104, ModelIndex: 50})/200 + 1e-4
	wet := func(p Position) (bool, bool) {
		return math.Abs(p.Latitude-latitude) > margin || math.Abs(math.Remainder(p.Longitude-longitude, 2*math.Pi)) > margin/math.Cos(latitude), true
	}
	if _, ok := (&Request{wet: wet}).standaloneSite(nil, catalog); ok {
		t.Fatal("样本干地应位于常用搜索格之外")
	}
	const matrix = 6001
	plan := industryplan.PlanetPlan{PlanetID: 201, Ports: []industryplan.Port{
		{ItemID: matrix, Kind: "import", RatePerMinute: 6, PeerPlanetID: 204},
		{ItemID: matrix, Kind: "delivery", RatePerMinute: 6},
	}}
	req := Request{PlanetID: plan.PlanetID, TerrainKnown: true, wet: wet}
	actual := industryplan.PlanetActual{}
	if err := appendReceiving(ctx, &req, ProductionRequest{Plan: plan}, &actual, catalog); err != nil {
		t.Fatal(err)
	}
	if len(req.Objects) != 1 || len(actual.Ports) != 2 || req.Objects[0].Building.ItemID != 2104 || req.Objects[0].Building.Parameters[2] != 2 {
		t.Fatalf("收货塔或进口交付回执不符：%d 栋，%+v", len(req.Objects), actual.Ports)
	}
	if _, err := Build(ctx, req); err != nil {
		t.Fatal("带真实远程需求槽的收货塔未能通过最终蓝图核验", err)
	}
}
