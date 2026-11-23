package galaxy

// 文件职责：按原版固态星球的碰撞网格求径向交点；使用：施工点需要射线实测高度时；边界：缺少命中三角形时返回未知。

import "math"

// TerrainSurfaceRadius 职责：返回指定方向的地表碰撞半径；使用：核对泵脚与取水口；边界：只处理 200 精度的固态星球，不用邻域平均高度替代命中结果。
func TerrainSurfaceRadius(grid HeightGrid, modData []byte, direction Vector) (float64, bool) {
	const side, stride = 201, 402
	vertices := terrainVertices()
	length := math.Hypot(math.Hypot(direction.X, direction.Y), direction.Z)
	if grid.Precision != 200 || len(grid.Heights) != len(vertices) || length == 0 || math.IsNaN(length) || math.IsInf(length, 0) {
		return 0, false
	}
	p := terrainPoint{float32(direction.X / length), float32(direction.Y / length), float32(direction.Z / length)}
	// 四块方形网格在赤道和两极共用边。先按象限选择网格，再从原版哈希给出的近邻走到最近顶点。
	panel := 0
	if p.x < 0 {
		panel = 1
	}
	if p.z < 0 {
		panel = 3 - panel
	}
	offset := (panel%2 + panel/2*stride) * side
	start := terrainIndex()[terrainPositionHash(p, 0)]
	x, y := start%stride%side, start/stride%side
	index := func(x, y int) int { return offset + x + y*stride }
	for {
		nx, ny := x, y
		best := vertices[index(x, y)].diff(p).square()
		for j := max(0, y-1); j <= min(side-1, y+1); j++ {
			for i := max(0, x-1); i <= min(side-1, x+1); i++ {
				if dist := vertices[index(i, j)].diff(p).square(); dist < best {
					best, nx, ny = dist, i, j
				}
			}
		}
		if nx == x && ny == y {
			break
		}
		x, y = nx, ny
	}
	point := func(i int) terrainPoint {
		return vertices[i].scale(float32(modifiedHeightCM(grid, modData, i)) * .01)
	}
	for j := max(0, y-1); j <= min(side-2, y); j++ {
		for i := max(0, x-1); i <= min(side-2, x); i++ {
			k := index(i, j)
			a, c := point(k), point(k+stride+1)
			// 原版两片共用 (x,y) 到 (x+1,y+1) 的对角线。
			if h, ok := radialTriangle(p, a, point(k+1), c); ok {
				return h, true
			}
			if h, ok := radialTriangle(p, a, c, point(k+stride)); ok {
				return h, true
			}
		}
	}
	return 0, false
}

// radialTriangle 职责：求球心射线与单个三角面的交点；使用：原版表面网格；边界：重心坐标在三角形外或射线平行时忽略该面。
func radialTriangle(p, a, b, c terrainPoint) (float64, bool) {
	cross := func(u, v terrainPoint) terrainPoint {
		return terrainPoint{u.y*v.z - u.z*v.y, u.z*v.x - u.x*v.z, u.x*v.y - u.y*v.x}
	}
	dot := func(u, v terrainPoint) float64 {
		return float64(u.x)*float64(v.x) + float64(u.y)*float64(v.y) + float64(u.z)*float64(v.z)
	}
	ab, ac := b.diff(a), c.diff(a)
	h := cross(p, ac)
	det := dot(ab, h)
	if math.Abs(det) < 1e-10 {
		return 0, false
	}
	u := -dot(a, h) / det
	q := cross(ab, a)
	v := dot(p, q) / det
	const edgeTolerance = 1e-5
	if u < -edgeTolerance || v < -edgeTolerance || u+v > 1+edgeTolerance {
		return 0, false
	}
	return dot(ac, q) / det, true
}
