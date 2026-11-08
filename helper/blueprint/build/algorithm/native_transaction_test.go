package build

// 文件职责：验证原生端口修正的事务边界和各级设备局部能力。
// 使用：高纬整厂接入回归；边界：局部试样不代表整厂产能或游戏实测。
import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"context"
	"fmt"
	"math"
	"reflect"
	"testing"
)

// nativeAssemblerUnit 职责：构造指定原生纬度的独立双机单元；使用：第4档端口试验；边界：不接入整厂、外部物流或供电生成。
func nativeAssemblerUnit(t *testing.T, item, level, turn, hemisphere int, position string) (*builder, *surfaceEncoding, []blueprint.Building) {
	t.Helper()
	ctx := context.WithValue(context.Background(), denseCollisionKey{}, true)
	plan, err := quant.Solve(ctx, quant.Request{Dataset: "Vanilla", Targets: []quant.Target{{ItemID: item, RatePerMinute: 120}}})
	if err != nil {
		t.Fatal(err)
	}
	disabled := false
	adapter, err := resolveModeAdapter(1, densityDense)
	if err != nil {
		t.Fatal(err)
	}
	belt := min(level, 3)
	prepared, err := prepareGeneration(ctx, Request{Plan: plan, Mode: 1, Density: densityDense, Setting: Setting{BeltLv: belt, InserterLv: level, NeedPower: &disabled}}, adapter)
	if err != nil {
		t.Fatal(err)
	}
	b := prepared.builder
	b.denseSpatial = true
	recipe := 50
	if item == 1202 {
		recipe = 6
	}
	for owner, p := range prepared.request.Plan.Productions {
		if p.RecipeID != recipe {
			continue
		}
		b.owner = owner
		r, err := datasetRecipeForProduction(prepared.dataset, p)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = b.buildReferenceLowLine(p, r, 2, 1, 0, true); err != nil {
			t.Fatal(err)
		}
	}
	if len(b.all) == 0 {
		t.Fatal("没有创建参考单元")
	}
	rotatePlacedQuarter(b.all, turn)
	low, high := bounds(b.all)
	shift(b.all, -low.x, -low.y, 0)
	w, h := int(math.Ceil(high.x-low.x))+1, int(math.Ceil(high.y-low.y))+1
	band := densityLatitudeTierBands(4, hemisphere, 200)[0]
	anchor := bandCenter(band) - (h-1)/2
	if position == "south" {
		anchor = band.low
	} else if position == "north" {
		anchor = band.high - h + 1
	}
	if h > 25 {
		t.Fatal("局部单元超出第4档环宽")
	}
	records, _, _, err := b.records()
	if err != nil {
		t.Fatal(err)
	}
	encoding := &surfaceEncoding{areas: []blueprint.Area{{Index: 0, ParentIndex: -1, AreaSegments: 100, Width: int16(w), Height: int16(h)}}, plan: &SurfacePlan{GridSegment: 200, PrimaryAreaIndex: 0, AnchorLatitudeGrid: anchor, LayoutWidth: w, LayoutHeight: h, Bands: []SurfaceBandPlan{{AreaIndex: 0, MinLatitudeGrid: anchor, MaxLatitudeGrid: anchor + h - 1, LongitudeSegments: 100, BuildingCount: len(records)}}}}
	return b, encoding, records
}

// TestNativePortTransaction 职责：验证失败后的实体、参数、记录与诊断完全回滚；使用：失败候选隔离；边界：不接受部分修正。
func TestNativePortTransaction(t *testing.T) {
	ctx := context.WithValue(context.Background(), denseCollisionKey{}, true)
	b, e, records := nativeAssemblerUnit(t, 1301, 1, 0, 1, "south")
	original := append([]blueprint.Building(nil), records...)
	aligned, err := b.alignNativeInserterPorts(ctx, e, records)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, records) {
		t.Fatal("修改了输入记录")
	}
	again, err := b.alignNativeInserterPorts(ctx, e, aligned)
	if err != nil || !reflect.DeepEqual(again, aligned) {
		t.Fatal("成功适配不幂等", err)
	}
	// 在臂修正完成后触发独立静态碰撞，以覆盖参数和货物引用已被写入的回滚路径。
	var machines []*placed
	for _, p := range b.all {
		if p.record.RecipeID > 0 {
			machines = append(machines, p)
		}
	}
	machines[1].a, machines[1].b = machines[0].a, machines[0].b
	invalid, _, _, err := b.records()
	if err != nil {
		t.Fatal(err)
	}
	snapshot := make([]placed, len(b.all))
	for i, p := range b.all {
		snapshot[i] = *p
		snapshot[i].record.Parameters = append([]int32(nil), p.record.Parameters...)
	}
	plan, checks := *e.plan, len(b.logistics.Checks)
	if _, err := b.alignNativeInserterPorts(ctx, e, invalid); err == nil {
		t.Fatal("碰撞样例不应通过")
	}
	for i, p := range b.all {
		if !reflect.DeepEqual(snapshot[i], *p) {
			t.Fatalf("实体%d未回滚", i)
		}
	}
	if !reflect.DeepEqual(plan, *e.plan) || len(b.logistics.Checks) != checks {
		t.Fatal("失败污染布局或诊断")
	}
}

// TestNativePortGrades 职责：复核四级分拣器的双机单元；使用：两半球三位置及四方向；边界：设备等级和机器朝向不改变。
func TestNativePortGrades(t *testing.T) {
	ctx := context.WithValue(context.Background(), denseCollisionKey{}, true)
	for _, level := range []int{1, 2, 3, 4} {
		for _, turn := range []int{0, 1, 2, 3} {
			for _, hemisphere := range []int{-1, 1} {
				for _, position := range []string{"center", "south", "north"} {
					t.Run(fmt.Sprintf("lv%d-turn%d-h%d-%s", level, turn, hemisphere, position), func(t *testing.T) {
						b, e, records := nativeAssemblerUnit(t, 1301, level, turn, hemisphere, position)
						if _, err := b.alignNativeInserterPorts(ctx, e, records); err != nil {
							t.Fatal(err)
						}
					})
				}
			}
		}
	}
}
