package planet

// 文件职责：验证重复投影的参数所有权；使用：制造模板复用回归；边界：各投影独立配置物流槽和专组，不修改原始模板。

import (
	"context"
	"slices"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build"
	"DysonSphereBox/helper/industryplan"
)

// TestProjectDocumentOwnsStationParameters 职责：隔离模板和两份投影的可变参数；使用：真实配置入口及严格编解码；边界：位置仍逐份通过原版几何验证。
func TestProjectDocumentOwnsStationParameters(t *testing.T) {
	ctx := context.Background()
	template, err := Build(ctx, Request{PlanetID: 101, Objects: []Object{stationObject(0, Position{}, 1101, 1, 0, false)}})
	if err != nil {
		t.Fatal(err)
	}
	document, err := blueprint.Decode(template.Blueprint, true)
	if err != nil {
		t.Fatal(err)
	}
	original := slices.Clone(document.Buildings[0].Parameters)
	first, err := ProjectDocument(ctx, document, template.Anchor, "第一制造模块")
	if err != nil {
		t.Fatal(err)
	}
	otherAnchor := template.Anchor
	otherAnchor.Longitude += .5
	second, err := ProjectDocument(ctx, document, otherAnchor, "第二制造模块")
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || len(second) != 1 || !slices.Equal(first[0].Building.Parameters, original) || !slices.Equal(second[0].Building.Parameters, original) {
		t.Fatal("投影没有完整保留模板参数")
	}
	want := industryplan.Port{ItemID: 1101, Kind: "export", PeerPlanetID: 102, RatePerMinute: 30}
	actual, err := configureProductionPorts(industryplan.PlanetPlan{PlanetID: 101, Ports: []industryplan.Port{want}},
		[]blueprint.Building{first[0].Building}, []build.BoundaryPort{{ItemID: 1101, Kind: "target-output", BuildingIndex: 0, RatePerMinute: 30}})
	if err != nil || len(actual) != 1 || actual[0] != want {
		t.Fatalf("第一投影的真实制造出口配置失败：%v，%+v", err, actual)
	}
	configureRemoteGroup(&first[0].Building, (1<<45)|(1<<3))
	firstParameters := slices.Clone(first[0].Building.Parameters)
	if firstParameters[2] != 1 || firstParameters[332] != 8 || firstParameters[333] != 8192 || firstParameters[334] != 4 {
		t.Fatal("第一投影未获得独立的出口和远程专组")
	}
	if !slices.Equal(document.Buildings[0].Parameters, original) || !slices.Equal(second[0].Building.Parameters, original) {
		t.Fatal("修改第一投影污染了模板或第二投影")
	}
	second[0].Building.Parameters[2] = 2
	configureRemoteGroup(&second[0].Building, 1<<34)
	secondParameters := slices.Clone(second[0].Building.Parameters)
	if !slices.Equal(first[0].Building.Parameters, firstParameters) || !slices.Equal(document.Buildings[0].Parameters, original) {
		t.Fatal("修改第二投影污染了第一投影或模板")
	}
	document.Buildings[0].Parameters[2] = 1
	configureRemoteGroup(&document.Buildings[0], 1<<5)
	if !slices.Equal(first[0].Building.Parameters, firstParameters) || !slices.Equal(second[0].Building.Parameters, secondParameters) {
		t.Fatal("修改模板污染了已投影的建筑参数")
	}
	for _, tc := range []struct {
		name       string
		objects    []Object
		parameters []int32
	}{{"first", first, firstParameters}, {"second", second, secondParameters}} {
		t.Run(tc.name, func(t *testing.T) {
			built, err := Build(ctx, Request{PlanetID: 101, Objects: tc.objects})
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := blueprint.Decode(built.Blueprint, true)
			if err != nil {
				t.Fatal(err)
			}
			if len(decoded.Buildings) != 1 || decoded.Buildings[0].ItemID != 2104 || !slices.Equal(decoded.Buildings[0].Parameters, tc.parameters) {
				t.Fatal("投影经过整星装配后丢失了各自物流参数")
			}
		})
	}
}
