package build

// 文件职责：用游戏数值原型验证静态三维碰撞的方向、层高和失败关闭。
// 使用：球面放置预检回归；边界：这些测试没有运行游戏物理场景。

import (
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
)

// TestStaticColliderUsesFullVerticalExtent 职责：保证 Z 不同不能绕过真实体积检查；使用：静态出口回归；边界：研究站合法堆叠另有真实盒净空。
func TestStaticColliderUsesFullVerticalExtent(t *testing.T) {
	for _, tc := range []struct {
		name      string
		id, model int16
		height    float32
		wantError bool
	}{
		{name: "assembler-overlap-despite-z-change", id: 2303, model: 65, height: 1, wantError: true},
		{name: "smelter-overlap-despite-z-change", id: 2302, model: 62, height: 1, wantError: true},
		{name: "lab-native-stack-spacing", id: 2901, model: 70, height: 3},
		{name: "lab-insufficient-spacing", id: 2901, model: 70, height: 2, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoding := &surfaceEncoding{areas: []blueprint.Area{{AreaSegments: 200, ParentIndex: -1}}, plan: &SurfacePlan{GridSegment: 200}}
			records := []blueprint.Building{{Index: 0, ItemID: tc.id, ModelIndex: tc.model}, {Index: 1, ItemID: tc.id, ModelIndex: tc.model, LocalOffset: blueprint.Point{Z: tc.height}}}
			if err := encoding.validateStaticColliders(t.Context(), records); (err != nil) != tc.wantError {
				t.Fatalf("error=%v, wantError=%v", err, tc.wantError)
			}
		})
	}
}

// TestStaticColliderRotationMatchesSphericalBasis 职责：用非对称原型核对四分之一转的中心和轴；使用：精炼朝向回归；边界：参考系选在赤道经度零，Unity up 对应世界负 Z。
func TestStaticColliderRotationMatchesSphericalBasis(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	shapes, err := catalog.Shapes(2308, 63)
	if err != nil {
		t.Fatal(err)
	}
	for _, yaw := range []float32{0, 90, 180, 270} {
		box, err := surfaceColliderBox(vector{0, 0, -200.2}, yaw, shapes[0])
		if err != nil || !box.Valid() {
			t.Fatalf("yaw=%v invalid: %v", yaw, err)
		}
		if box.Center[2] >= -200.2 || box.HalfSize[2] < box.HalfSize[0] {
			t.Fatalf("raw Unity height/directional extents lost: %+v", box)
		}
		if yaw == 90 && (box.Axes[0][1] > -.999 || box.Axes[2][0] < .999) {
			t.Fatalf("yaw90 rotated in wrong direction: %+v", box.Axes)
		}
	}
}

// TestStaticColliderRejectsUnknownIdentity 职责：防止正确模型配错物品时借用占位；使用：可信目录边界；边界：已知盒不能覆盖未知 Mod。
func TestStaticColliderRejectsUnknownIdentity(t *testing.T) {
	encoding := &surfaceEncoding{areas: []blueprint.Area{{AreaSegments: 200, ParentIndex: -1}}, plan: &SurfacePlan{GridSegment: 200}}
	if err := encoding.validateStaticColliders(t.Context(), []blueprint.Building{{Index: 0, ItemID: 9999, ModelIndex: 65}}); err == nil {
		t.Fatal("unknown item reused a known model without verification")
	}
}
