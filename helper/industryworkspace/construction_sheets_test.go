package industryworkspace

// 文件职责：验证同锚点施工图的科技上限、原生连接和地基逐格守恒。

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"DysonSphereBox/helper/blueprint"
)

func sheetTestBuilding(index int32, item, model int16) blueprint.Building {
	return blueprint.Building{Index: index, ItemID: item, ModelIndex: model, InputObjIdx: -1, OutputObjIdx: -1, LocalOffset: blueprint.Point{X: float32(index)*10 + 20, Y: 6}}
}

func sheetTestDocument(records ...blueprint.Building) blueprint.Document {
	return blueprint.Document{BinaryVersion: 2, EnvelopeVersion: 1, Patch: 1, Name: "施工测试",
		Description:  "按固定经纬度和零度朝向粘贴。经度 12.000000°，纬度 0.000000°。",
		Areas:        []blueprint.Area{{Index: 0, ParentIndex: -1, AreaSegments: 200, Width: 80, Height: 12}, {Index: 1, ParentIndex: 0, AnchorLocalOffsetY: 12, AreaSegments: 200, Width: 80, Height: 12}},
		CursorOffset: blueprint.PointInt{X: 7, Y: 3}, DragBoxSize: blueprint.PointInt{X: 80, Y: 24}, Buildings: records}
}

func sheetTestCode(t *testing.T, doc blueprint.Document) string {
	t.Helper()
	result, err := blueprint.Encode(doc, doc.EnvelopeVersion)
	if err != nil {
		t.Fatal(err)
	}
	return result.Blueprint
}

func sheetTestDecode(t *testing.T, part ConstructionPart) blueprint.Document {
	t.Helper()
	doc, err := blueprint.Decode(part.Code, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Buildings) != part.Buildings {
		t.Fatalf("张内建筑数 %d 与摘要 %d 不符", len(doc.Buildings), part.Buildings)
	}
	return doc
}

func TestConstructionSheetsWholeAndStrict(t *testing.T) {
	record := sheetTestBuilding(0, 2201, 44)
	code := sheetTestCode(t, sheetTestDocument(record))
	p, err := SplitConstructionBlueprint(context.Background(), code, 1, 0)
	if err != nil || len(p) != 1 || p[0].Kind != "whole" || p[0].Code != code {
		t.Fatalf("可直接使用的原图发生变化：%v", err)
	}
	if _, err := SplitConstructionBlueprint(context.Background(), code[:len(code)-1], 1, 0); err == nil {
		t.Fatal("损坏的校验和不应返回蓝图")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := SplitConstructionBlueprint(ctx, code, 1, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("取消时返回 %v", err)
	}
	record.OutputObjIdx = 3
	if _, err := SplitConstructionBlueprint(context.Background(), sheetTestCode(t, sheetTestDocument(record)), 1, 0); err == nil {
		t.Fatal("导出了指向不存在建筑的蓝图")
	}
	if _, err := blueprint.Encode(sheetTestDocument(), 1); err == nil {
		t.Fatal("全空蓝图不应通过编码")
	}
	if _, err := SplitConstructionBlueprint(context.Background(), code, 0, 0); err == nil {
		t.Fatal("零建筑额度被视为无限")
	}
}

func TestConstructionSheetsFoundationUnion(t *testing.T) {
	doc := sheetTestDocument(sheetTestBuilding(0, 2201, 44))
	doc.Reform = &blueprint.ReformData{Reserved: 1, CustomReformColorMask: 3, CustomReformColors: []uint32{0x12345678, 0xabcdef01},
		Rects: []blueprint.ReformRect{{X: 3, Y: 5, W: 7, H: 3, Data: 224, Reserved: 7}, {X: 8, Y: 3, W: 3, H: 2, Data: 139, AreaIndex: 1}}}
	type cell struct{ area, x, y, data, reserved int }
	expand := func(cells map[cell]int, r blueprint.ReformRect) {
		for x := range int(r.W) {
			for y := range int(r.H) {
				cells[cell{int(r.AreaIndex), int(r.X) + x, int(r.Y) + y, int(r.Data), int(r.Reserved)}]++
			}
		}
	}
	want := make(map[cell]int)
	for _, rect := range doc.Reform.Rects {
		expand(want, rect)
	}
	for _, limit := range []int{1, 5, 11, 20} {
		code := sheetTestCode(t, doc)
		parts, err := SplitConstructionBlueprint(context.Background(), code, 1, limit)
		if err != nil {
			t.Fatal(err)
		}
		if len(parts) != (27+limit-1)/limit+1 {
			t.Fatalf("每张最多 %d 格时分成 %d 张", limit, len(parts))
		}
		got := make(map[cell]int)
		for i, part := range parts {
			sheet := sheetTestDecode(t, part)
			if !reflect.DeepEqual(doc.Areas, sheet.Areas) || sheet.PrimaryAreaIndex != doc.PrimaryAreaIndex || sheet.CursorOffset != doc.CursorOffset || sheet.DragBoxSize != doc.DragBoxSize {
				t.Fatal("分图改变了粘贴锚点或区域")
			}
			if i == len(parts)-1 {
				if part.Kind != "buildings" || part.FoundationCells != 0 || part.Buildings != 1 || sheet.Reform != nil {
					t.Fatalf("最后一张没有单独保留全部建筑")
				}
				continue
			}
			if part.Kind != "foundation" || part.Buildings != 0 || part.FoundationCells > limit || sheet.Reform == nil ||
				sheet.Reform.Reserved != doc.Reform.Reserved || sheet.Reform.CustomReformColorMask != doc.Reform.CustomReformColorMask || !reflect.DeepEqual(sheet.Reform.CustomReformColors, doc.Reform.CustomReformColors) {
				t.Fatal("地基分图的建筑、额度或颜色表不符")
			}
			count := 0
			for _, rect := range sheet.Reform.Rects {
				count += int(rect.W) * int(rect.H)
				expand(got, rect)
			}
			if count != part.FoundationCells || (i < len(parts)-2 && count != limit) {
				t.Fatalf("地基分图实际 %d 格，摘要 %d 格，上限 %d 格", count, part.FoundationCells, limit)
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatal("拆分后逐格坐标、颜色、重复次数或保留标记有差异")
		}
	}
	doc.Buildings = nil
	code := sheetTestCode(t, doc)
	parts, err := SplitConstructionBlueprint(context.Background(), code, 0, 27)
	if err != nil || len(parts) != 1 || parts[0].Code != code || parts[0].Buildings != 0 {
		t.Fatalf("原版纯地基蓝图应能独立使用：%v", err)
	}
	parts, err = SplitConstructionBlueprint(context.Background(), code, 0, 12)
	if err != nil || len(parts) != 3 || parts[2].FoundationCells != 3 || parts[2].Kind != "foundation" {
		t.Fatalf("纯地基分图异常：%v", err)
	}
	if parts, err := SplitConstructionBlueprint(context.Background(), code, 0, 0); err == nil || len(parts) != 0 {
		t.Fatal("允许铺地基上限为零的存档导出了纯地基蓝图")
	}
}

func TestConstructionSheetsConnectedBuildings(t *testing.T) {
	belt, independent, tower, monitor := sheetTestBuilding(0, 2001, 35), sheetTestBuilding(1, 2316, 256), sheetTestBuilding(2, 2104, 50), sheetTestBuilding(3, 2030, 208)
	tower.Parameters = make([]int32, 2048)
	tower.Parameters[0], tower.Parameters[193] = 1001, 1
	independent.Parameters = []int32{1001, 1, 0, 10000}
	belt.InputObjIdx, belt.InputFromSlot, belt.Parameters = 2, 4, []int32{1001, 30}
	monitor.LocalOffset = belt.LocalOffset
	pole := sheetTestBuilding(4, 2201, 44)
	doc := sheetTestDocument(belt, independent, tower, monitor, pole)
	code := sheetTestCode(t, doc)
	parts, err := SplitConstructionBlueprint(context.Background(), code, 3, 0)
	if err != nil || len(parts) != 2 {
		t.Fatalf("独立建筑组未按限额分图：%v", err)
	}
	group, other := sheetTestDecode(t, parts[0]), sheetTestDecode(t, parts[1])
	if len(group.Buildings) != 3 || len(other.Buildings) != 2 {
		t.Fatal("传送带与塔或监测器分开")
	}
	beltRecord, towerRecord, monitorRecord := group.Buildings[0], group.Buildings[1], group.Buildings[2]
	if towerRecord.ItemID != tower.ItemID || !reflect.DeepEqual(towerRecord.Parameters, tower.Parameters) ||
		beltRecord.InputObjIdx != 1 || beltRecord.Index != 0 || towerRecord.Index != 1 || beltRecord.InputFromSlot != belt.InputFromSlot || !reflect.DeepEqual(beltRecord.Parameters, belt.Parameters) ||
		monitorRecord.Index != 2 || monitorRecord.LocalOffset != beltRecord.LocalOffset || monitorRecord.OutputObjIdx != -1 ||
		other.Buildings[0].Index != 0 || other.Buildings[0].ItemID != independent.ItemID || !reflect.DeepEqual(other.Buildings[0].Parameters, independent.Parameters) {
		t.Fatal("重排改变了实体连线、建筑设置或宿主带")
	}
	if parts, err := SplitConstructionBlueprint(context.Background(), code, 2, 0); err == nil || len(parts) != 0 || !strings.Contains(err.Error(), "3 个") {
		t.Fatalf("不应切开含监测器的三建筑组：%v", err)
	}
	monitor.LocalOffset.X++
	if parts, err := SplitConstructionBlueprint(context.Background(), sheetTestCode(t, sheetTestDocument(belt, independent, tower, monitor)), 2, 0); err == nil || len(parts) != 0 || !strings.Contains(err.Error(), "宿主") {
		t.Fatalf("悬空监测器不应被导出：%v", err)
	}
	independent.ItemID = 30001
	tower.Index = 0
	if _, err := SplitConstructionBlueprint(context.Background(), sheetTestCode(t, sheetTestDocument(tower, independent)), 1, 0); err == nil || !strings.Contains(err.Error(), "未知配置") {
		t.Fatalf("未知设备参数被当成可安全拆分的原版配置：%v", err)
	}
}

func TestConstructionSheetsCoaterUsesBothBelts(t *testing.T) {
	coater := sheetTestBuilding(0, 2313, 120)
	coater.Yaw = 90
	pole := sheetTestBuilding(1, 2201, 44)
	lower, upper := sheetTestBuilding(2, 2001, 35), sheetTestBuilding(3, 2001, 35)
	lower.LocalOffset, upper.LocalOffset = coater.LocalOffset, coater.LocalOffset
	upper.LocalOffset.X--
	upper.LocalOffset.Z++
	doc := sheetTestDocument(coater, pole, lower, upper)
	parts, err := SplitConstructionBlueprint(context.Background(), sheetTestCode(t, doc), 3, 0)
	if err != nil || len(parts) != 2 {
		t.Fatalf("未能将喷涂机和两条通道的宿主带放在同一张：%v", err)
	}
	group := sheetTestDecode(t, parts[0])
	if len(group.Buildings) != 3 || group.Buildings[0].ItemID != 2313 || group.Buildings[1].LocalOffset != lower.LocalOffset || group.Buildings[2].LocalOffset != upper.LocalOffset {
		t.Fatal("喷涂机未与两条宿主带一起导出")
	}
	doc.Buildings = doc.Buildings[:3]
	if _, err := SplitConstructionBlueprint(context.Background(), sheetTestCode(t, doc), 2, 0); err == nil || !strings.Contains(err.Error(), "宿主") {
		t.Fatalf("缺少供剂带时不应宣称安全拆分：%v", err)
	}
}
