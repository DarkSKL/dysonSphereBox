package planet

// 文件职责：对照原版角色施工时测到的碰撞半径；使用：1203 星岸线回归；边界：仅使用原版地形种子和射线测量，无需私有存档。

import (
	"math"
	"testing"

	"DysonSphereBox/helper/saveanalysis"
	"DysonSphereBox/helper/saveanalysis/galaxy"
)

// TestPump180SurfaceRays 职责：区分能取水的深点与同一台泵上的浅点；使用：校验原版碰撞网格的对角线和高度；边界：容差包含记录方向的六位小数舍入。
func TestPump180SurfaceRays(t *testing.T) {
	grid, ok := galaxy.TerrainHeights(galaxy.Planet{GenSeed: 1997323822}, galaxy.SurfaceTheme{ID: 10, Algorithm: 3, ModY: .19924971061128288})
	if !ok {
		t.Fatal("1203 星基础地形缺失")
	}
	for _, ray := range []struct {
		position galaxy.Vector
		radius   float64
	}{
		{galaxy.Vector{X: -.332417, Y: .844551, Z: .419800}, 198.9365},
		{galaxy.Vector{X: -.329275, Y: .844551, Z: .422269}, 198.9279},
		{galaxy.Vector{X: -.337832, Y: .842390, Z: .419820}, 199.7854},
		{galaxy.Vector{X: -.328013, Y: .842390, Z: .427535}, 199.7675},
	} {
		got, known := galaxy.TerrainSurfaceRadius(grid, nil, ray.position)
		if !known || math.Abs(got-ray.radius) > .001 {
			t.Errorf("取水方向 %+v 的碰撞半径 %.5f，应为 %.5f", ray.position, got, ray.radius)
		}
	}
	water := 0.0
	terrain := saveanalysis.PlanetTerrain{HeightPrecision: grid.Precision, HeightCentimeters: grid.Heights, WaterHeight: &water}
	for _, tc := range []struct {
		origin galaxy.Vector
		yaw    float64
		want   bool
	}{
		{galaxy.Vector{X: -67.586520, Y: 167.673100, Z: 86.010610}, 180, false},
		{galaxy.Vector{X: 189.440200, Y: -20.092420, Z: -61.552810}, 0, true},
	} {
		p, err := savePosition(tc.origin)
		if err != nil || pumpSurfaceFits(p, tc.yaw, terrain) != tc.want {
			t.Errorf("原版泵位 %+v 朝向 %.0f 的可建状态应为 %t，错误 %v", p, tc.yaw, tc.want, err)
		}
	}
}
