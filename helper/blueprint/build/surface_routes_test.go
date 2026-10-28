package build

// 文件职责：验证已知游戏带距离硬约束与经度周期映射。
// 使用：球面出口回归；边界：不把接线距离测试当作完整碰撞或科技验收。

import (
	"testing"

	"DysonSphereBox/helper/blueprint"
)

// TestSurfaceBeltDistanceUsesWorldUnits 职责：拦截格网中看似相邻但世界距离过远的接线；使用：硬约束回归；边界：验证空场景几何，不读取游戏科技。
func TestSurfaceBeltDistanceUsesWorldUnits(t *testing.T) {
	encoding := &surfaceEncoding{areas: []blueprint.Area{{Index: 0, ParentIndex: -1, AreaSegments: 200, Width: 1000, Height: 1}}, plan: &SurfacePlan{GridSegment: 200}}
	cases := []struct {
		name      string
		from, to  blueprint.Point
		target    int16
		wantError bool
	}{
		{name: "one-grid", to: blueprint.Point{X: 1}, target: 2001},
		{name: "two-grids-too-far", to: blueprint.Point{X: 2}, target: 2001, wantError: true},
		{name: "seam-neighbors", from: blueprint.Point{X: 999}, to: blueprint.Point{X: 0}, target: 2001},
		{name: "one-height-step", to: blueprint.Point{Z: 1}, target: 2001},
		{name: "two-height-steps-too-far", to: blueprint.Point{Z: 2}, target: 2001, wantError: true},
		{name: "ordinary-port-range", to: blueprint.Point{X: 3}, target: 2103},
		{name: "ordinary-port-too-far", to: blueprint.Point{X: 4}, target: 2103, wantError: true},
		{name: "fractionator-extra-range", to: blueprint.Point{X: 4}, target: 2307},
		{name: "fractionator-too-far", to: blueprint.Point{X: 5}, target: 2307, wantError: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			records := []blueprint.Building{
				{Index: 0, ItemID: 2001, LocalOffset: tc.from, OutputObjIdx: 1, InputObjIdx: -1},
				{Index: 1, ItemID: tc.target, LocalOffset: tc.to, OutputObjIdx: -1, InputObjIdx: -1},
			}
			if err := encoding.validateBeltDistances(records); (err != nil) != tc.wantError {
				t.Fatalf("distance constraint error=%v, wantError=%v", err, tc.wantError)
			}
		})
	}
}

// TestSurfaceCoordinatesRejectUnknownGrid 职责：防止未核对的行星网格沿用原版尺度；使用：版本边界回归；边界：其它网格需独立核对后才能启用。
func TestSurfaceCoordinatesRejectUnknownGrid(t *testing.T) {
	encoding := &surfaceEncoding{plan: &SurfacePlan{GridSegment: 400}}
	if _, err := encoding.surfaceWorldPositions(nil); err == nil {
		t.Fatal("unsupported planet radius was silently mapped to vanilla world units")
	}
}
