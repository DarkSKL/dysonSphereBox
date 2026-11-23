package galaxy

// 文件职责：按原版方向哈希查询基础与改造地表高度。
// 使用：矿脉生成和球面放置；边界：此处的返回值为距离星球中心的游戏米。
import (
	"math"
	"sync"
)

var terrainIndex = sync.OnceValue(func() []int {
	const width = 200 / 4
	vertices := terrainVertices()
	indexMap := make([]int, width*width*3*8)
	for i := range indexMap {
		indexMap[i] = -1
	}
	const face, stride = 201, 402
	corners := [...]int{7, 5, 6, 4, 2, 0, 3, 1}
	for i, vertex := range vertices {
		x, y := i%stride, i/stride
		f := (x/face + y/face*2) * 2
		if x%face < y%face {
			f++
		}
		hash := terrainPositionHash(vertex, corners[f])
		if indexMap[hash] == -1 {
			indexMap[hash] = i
		}
	}
	for i := 1; i < len(indexMap); i++ {
		if indexMap[i] == -1 {
			indexMap[i] = indexMap[i-1]
		}
	}
	return indexMap
})

// terrainPositionHash 职责：定位球面位置所在哈希区；使用：地形顶点索引；边界：格精度为 200。
func terrainPositionHash(p terrainPoint, corner int) int {
	const width = 50
	if corner == 0 {
		if p.x > 0 {
			corner |= 1
		}
		if p.y > 0 {
			corner |= 2
		}
		if p.z > 0 {
			corner |= 4
		}
	}
	x, y, z := float32(math.Abs(float64(p.x))), float32(math.Abs(float64(p.y))), float32(math.Abs(float64(p.z)))
	if x < 1e-6 && y < 1e-6 && z < 1e-6 {
		return 0
	}
	coordinate := func(value float32) int {
		return min(width-1, int((float32(math.Sqrt(float64(value+.23)))-.4795832)/.6294705*width))
	}
	switch {
	case x >= y && x >= z:
		return coordinate(z/x) + width*coordinate(y/x) + width*width*3*corner
	case y >= x && y >= z:
		return coordinate(x/y) + width*coordinate(z/y) + width*width*(1+3*corner)
	default:
		return coordinate(x/z) + width*coordinate(y/z) + width*width*(2+3*corner)
	}
}

// TerrainHeightAt 职责：按局部方向查询改造后高度；使用：球面布局；边界：不支持的网格精度返回未知。
func TerrainHeightAt(grid HeightGrid, modData []byte, direction Vector) (float64, bool) {
	length := math.Hypot(math.Hypot(direction.X, direction.Y), direction.Z)
	if length <= 0 || math.IsNaN(length) || math.IsInf(length, 0) {
		return 0, false
	}
	if grid.Precision == 64 && len(grid.Heights) > 0 {
		return float64(grid.Heights[0]) / 100, true
	}
	vertices := terrainVertices()
	if grid.Precision != 200 || len(grid.Heights) != len(vertices) {
		return 0, false
	}
	p := terrainPoint{float32(direction.X), float32(direction.Y), float32(direction.Z)}.unit()
	indexMap := terrainIndex()
	start := indexMap[terrainPositionHash(p, 0)]
	radius := float32(float64(float32(math.Pi)) / 400 * float64(float32(1.2)))
	var weightSum, heightSum float32
	for x := -1; x <= 3; x++ {
		for y := -1; y <= 3; y++ {
			index := start + x + y*402
			if index < 0 || index >= len(vertices) {
				continue
			}
			v := vertices[index]
			squared := v.diff(p).square()
			if squared > radius*radius {
				continue
			}
			weight := float32(1 - float64(float32(math.Sqrt(float64(squared))))/float64(radius))
			height := modifiedHeightCM(grid, modData, index)
			weightSum += weight
			heightSum = float32(float64(heightSum) + height*float64(weight))
		}
	}
	if weightSum == 0 {
		return 0, false
	}
	return single(float64(heightSum) / float64(weightSum) * single(.01)), true
}

// modifiedHeightCM 职责：解开一个顶点的改造等级与高度平面；使用：平滑高度和施工碰撞面；边界：无改造记录时取原始厘米高度。
func modifiedHeightCM(grid HeightGrid, modData []byte, index int) float64 {
	height := float64(grid.Heights[index])
	if index/2 >= len(modData) {
		return height
	}
	mod := modData[index/2] >> (4 * (index % 2))
	level := mod & 3
	target := float64(20020 + 133*int(mod>>2&3))
	if level == 3 {
		return target
	}
	blend := single(float64(level) * single(.3333333))
	return single(height*(1-blend) + target*blend)
}
