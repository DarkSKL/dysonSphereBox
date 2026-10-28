package build

// 文件职责：验证整块平面蓝图在候选纬圈之间回退时的真实预检与编码一致性。
// 使用：球面生成回归，覆盖不同经度段数、索引 3999/4000 和取消路径。
// 边界：使用原版数值盒；这些样本不代替用户失败蓝图或 Unity 场景实贴。

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"DysonSphereBox/helper/blueprint"
)

// TestAnchorSearchPreservesExplicitPreference 职责：验证自动模式可全局初选且指定模式严格尊重回退开关；使用：容量边界回归；边界：不对候选声称游戏放置通过。
func TestAnchorSearchPreservesExplicitPreference(t *testing.T) {
	// 120 行 footprint 在赤道起点 0 会跨段，但可整体向南平移锚点后留在赤道纬带。
	auto := surfaceAnchorCandidates("autoTropic", 200, 0, false, 20, 120)
	if len(auto) == 0 || auto[0] >= 0 {
		t.Fatalf("auto selection was limited to the equator origin: %v", auto)
	}
	if strict := surfaceAnchorCandidates("explicitTropic", 200, 0, false, 20, 120); len(strict) != 0 {
		t.Fatalf("strict explicit anchor silently moved: %v", strict)
	}
	if expanded := surfaceAnchorCandidates("explicitTropic", 200, 0, true, 20, 120); !reflect.DeepEqual(auto, expanded) {
		t.Fatalf("adjacent search changed deterministic ordering: auto=%v expanded=%v", auto, expanded)
	}
}

// TestAnchorFallbackChangesSegmentsWithoutChangingBuildings 职责：用实际熔炉建造盒验证首候选冲突后可整体换圈；使用：跨经度段回归；边界：合成样本不代表用户 3999/4000 的实际原型。
func TestAnchorFallbackChangesSegmentsWithoutChangingBuildings(t *testing.T) {
	records := []blueprint.Building{
		{Index: 3999, ItemID: 2302, ModelIndex: 62, InputObjIdx: -1, OutputObjIdx: -1},
		{Index: 4000, ItemID: 2302, ModelIndex: 62, LocalOffset: blueprint.Point{X: 2.5}, InputObjIdx: -1, OutputObjIdx: -1},
	}
	before := append([]blueprint.Building(nil), records...)
	encoding := &surfaceEncoding{
		areas: []blueprint.Area{{ParentIndex: -1, AreaSegments: 200, Width: 6, Height: 1}},
		plan:  makeSurfacePlan("explicitTropic", 200, 80, 80, 6, 1), anchorCandidates: []int{80},
	}
	encoding.plan.Bands = []SurfaceBandPlan{{BuildingCount: 2}}
	err := encoding.validateAnchorCandidates(t.Context(), records)
	var failure *blueprint.Error
	if !errors.As(err, &failure) || failure.Code != "GEN_E_COLLISION" || !strings.Contains(failure.Message, "3999") || !strings.Contains(failure.Message, "4000") {
		t.Fatalf("first candidate must report the real synthetic collision: %v", err)
	}
	encoding.anchorCandidates = []int{80, 100}
	if err := encoding.validateAnchorCandidates(t.Context(), records); err != nil {
		t.Fatalf("safe second anchor was rejected: %v", err)
	}
	if encoding.plan.SelectedLatitudeGrid != 100 || encoding.areas[0].AreaSegments != 160 || encoding.plan.LongitudeSegments != 160 || encoding.plan.RingGridCells != 800 {
		t.Fatalf("anchor and encoded grid disagree: area=%+v plan=%+v", encoding.areas[0], encoding.plan)
	}
	if !encoding.plan.StaticCollidersChecked || !encoding.plan.BeltConnectionsChecked || encoding.plan.PlacementStatus != "partial" {
		t.Fatalf("precheck coverage was lost or overstated: %+v", encoding.plan)
	}
	if !reflect.DeepEqual(before, records) {
		t.Fatal("anchor fallback modified planar building records")
	}
}

// TestAnchorValidationRejectsEmptyOrCanceledSearch 职责：禁止无候选或取消时返回成功；使用：生成超时边界；边界：不启动后台重试。
func TestAnchorValidationRejectsEmptyOrCanceledSearch(t *testing.T) {
	encoding := &surfaceEncoding{}
	if err := encoding.validateAnchorCandidates(t.Context(), nil); err == nil {
		t.Fatal("empty candidate search was accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	encoding.anchorCandidates = []int{0}
	var failure *blueprint.Error
	if err := encoding.validateAnchorCandidates(ctx, nil); !errors.As(err, &failure) || failure.Code != "GEN_E_TIMEOUT" {
		t.Fatalf("canceled search was not stopped before geometry access: %v", err)
	}
}
