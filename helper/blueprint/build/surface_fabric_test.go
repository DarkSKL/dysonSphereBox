package build

// 文件职责：防止新的独立混带环路被旧三圈总线重排器截断路线证据。
// 使用：球面容量入口回归；边界：只验证明确失败及原图保护，不把容量拒绝当作完整游戏兼容性。

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"DysonSphereBox/helper/blueprint"
)

// TestSurfaceFabricRejectsLegacyFolding 职责：保持单区可行环路，拒绝未经适配的跨区重排；使用：新旧 mode6 路径边界；边界：拒绝不能移动或增删任一原建筑。
func TestSurfaceFabricRejectsLegacyFolding(t *testing.T) {
	for _, count := range []int{10, 301} {
		b, err := newBuilder(Setting{BeltLv: 3})
		if err != nil {
			t.Fatal(err)
		}
		b.straight(17, 23, count, 1)
		b.busFabric = []*busFabricRoute{}
		states := make([]placed, len(b.all))
		for i, p := range b.all {
			states[i] = *p
		}
		err = b.foldSurfaceBus(context.Background(), &SurfaceLayout{Mode: "autoTropic", GridSegment: 200, ExpandAdjacent: true})
		if count == 10 && err != nil {
			t.Fatalf("单 Area 的独立混带环路应保留: %v", err)
		}
		var failure *blueprint.Error
		if count > 10 && (!errors.As(err, &failure) || failure.Code != "GEN_E_SURFACE_CAPACITY") {
			t.Fatalf("独立环路不能被旧总线折叠器隐式跨区: %v", err)
		}
		if len(b.all) != len(states) {
			t.Fatal("容量检查不得创建或删除建筑")
		}
		for i, p := range b.all {
			if !reflect.DeepEqual(*p, states[i]) {
				t.Fatalf("容量检查移动了原建筑 %d", i)
			}
		}
	}
}
