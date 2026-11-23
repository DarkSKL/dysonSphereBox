package planet

// 文件职责：按照原版基础格投影新铺地基及沙土区间；使用：整星蓝图；边界：液面、矿点、旧建筑及现成地基保留。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/geometry"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/saveanalysis"
)

// configureTerrainReform 职责：读取现成地基和原始顶点高度；使用：施工材料清单；边界：缺少数据时保持未知，不把旧地基当作新建设。
func configureTerrainReform(req *Request, terrain saveanalysis.PlanetTerrain) error {
	if len(terrain.ReformData) != 0 {
		if len(terrain.ReformOffsets) < 500 {
			return fmt.Errorf("存档地基存在，但缺少原版纬格索引")
		}
		req.isReformed = func(longitude, latitude int) bool {
			if latitude < 0 || latitude >= len(terrain.ReformOffsets) {
				return false
			}
			offset := terrain.ReformOffsets[latitude] + int64(longitude)
			return offset >= 0 && offset < int64(len(terrain.ReformData)) && terrain.ReformData[offset]>>5 > 0
		}
	}
	if len(terrain.HeightCentimeters) == 0 {
		return nil
	}
	var positive int64
	for i, height := range terrain.HeightCentimeters {
		level := 0
		if i/2 < len(terrain.ModData) {
			level = int(terrain.ModData[i/2]>>((i%2)*4)) & 3
		}
		// 原版每个顶点最多提高到改造级 3，目标半径为 200.20 米。
		positive += int64(max(0, 20020-int(height)) * (3 - level))
	}
	// 原版累计沙土点后每三个点折成一份沙土。只计全星所有可能填高的顶点，是安全上界。
	upper := (positive + 2) / 3
	req.soilUpper = &upper
	return nil
}

// foundation 职责：生成与原版 RefreshBuildPreview 一致的 v2 地基矩形；使用：有可信地形时；边界：优先避免重复铺设和受保护格。
func foundation(ctx context.Context, req Request, bands []band, areas []blueprint.Area, step, longitudeOrigin float64) (*blueprint.ReformData, int, error) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return nil, 0, err
	}
	full := req.Foundation == "planet"
	ground := obstacleIndex{}
	if !full {
		for _, obj := range req.Objects {
			radius := groundRadius(catalog, obj.Building)
			for _, p := range []Position{obj.First, obj.Second} {
				if p.Height > .5 {
					continue
				}
				p.Height = 0
				ground.add(Obstacle{Position: p, Radius: radius})
			}
		}
	}
	obstacles := append([]Obstacle(nil), req.Obstacles...)
	for _, cell := range req.Protected {
		radius := .75
		if cell.buried {
			// 原版按矿点方向最近的地形顶点决定是否翻出埋矿。三级改造影响格心周围约 2.83 米，
			// 再留出顶点到矿点的偏移和格投影误差，保持玩家已有埋矿状态。
			radius = 6
		}
		obstacles = append(obstacles, Obstacle{Position: Position{Longitude: cell.Longitude, Latitude: cell.Latitude}, Radius: radius})
	}
	// 原版铺格会连带抬高附近的地形顶点。给取水侧留出整段岸线，泵的陆脚仍由建造条件检查。
	for _, obj := range req.Objects {
		if obj.Building.ItemID == 2306 {
			pumpIntakeClear(obj.First, float64(obj.Building.Yaw), 0, func(p Position) bool {
				obstacles = append(obstacles, Obstacle{Position: p, Radius: 6})
				return true
			})
		}
	}
	protected := newObstacleIndex(obstacles)
	used := map[[2]int]bool{}
	result := &blueprint.ReformData{}
	count := 0
	for area, b := range bands {
		for row := 0; row <= b.high-b.low+1; row++ {
			if err := ctx.Err(); err != nil {
				return nil, 0, err
			}
			start, length := 0, 0
			flush := func() {
				if length != 0 {
					// build 25955035 的 kUndecalReformData 表示无装饰地基。
					result.Rects = append(result.Rects, blueprint.ReformRect{X: int16(start), Y: int16(row), W: uint8(length), H: 1, Data: 224, AreaIndex: uint8(area)})
					count += length
					length = 0
				}
			}
			for col := 0; col < int(areas[area].Width); col++ {
				key, pos, known := reformGridPosition(b.low, row, col, step, longitudeOrigin)
				include := known && !used[key] && (req.isReformed == nil || !req.isReformed(key[0], key[1]))
				if include && req.wet != nil {
					water, valid := req.wet(pos)
					include = valid && !water
				}
				if include {
					point, err := world(pos)
					if err != nil {
						return nil, 0, err
					}
					include = !protected.conflicts(point, .9) && (full || ground.conflicts(point, .65))
				}
				if !include {
					flush()
					continue
				}
				used[key] = true
				if length == 0 {
					start = col
				}
				length++
				if length == 255 {
					flush()
				}
			}
			flush()
		}
	}
	if count == 0 {
		return nil, 0, nil
	}
	return result, count, nil
}

// reformGridPosition 职责：解开游戏原版地基格的半格、纬带和南北号映射；使用：保护格及旧地基检查；边界：零度粘贴。
func reformGridPosition(areaLatitude, localY, localX int, step, origin float64) ([2]int, Position, bool) {
	areaLine := float64(areaLatitude)
	if areaLine <= 0 {
		areaLine += .4
	} else {
		areaLine -= .4
	}
	latitudeLine := areaLine + float64(localY) - .5
	latitude := int(latitudeLine)
	if latitudeLine < 0 {
		latitude = int(250 - latitudeLine)
	}
	if latitude < 0 || latitude >= 500 {
		return [2]int{}, Position{}, false
	}
	width := func(line float64) float64 {
		n := min(49, int(math.Abs(float64(int(line/5)))))
		segments, _ := geometry.LongitudeSegmentsAtGrid(n*5+1, geometry.Vanilla200)
		return float64(segments * 5)
	}
	cellWidth := width(latitudeLine)
	// 原版逐行预览要求本格与所属 Area 起点的经向格数相同。交界行由相邻、同格宽的 Area 登记。
	if cellWidth != width(areaLine) {
		return [2]int{}, Position{}, false
	}
	lonLine := origin*cellWidth/(2*math.Pi) + float64(localX) + .5
	lonLine = positiveAngle(lonLine*(2*math.Pi/cellWidth)+math.Pi)*cellWidth/(2*math.Pi) - cellWidth/2
	longitude := int(lonLine)
	if lonLine < 0 {
		longitude = int(cellWidth/2 - lonLine)
	}
	if longitude < 0 || float64(longitude) >= cellWidth {
		return [2]int{}, Position{}, false
	}
	latCenter := float64(latitude) + .5
	if latitude >= 250 {
		latCenter = float64(250-latitude) - .5
	}
	lonCenter := float64(longitude) + .5
	if float64(longitude) >= cellWidth/2 {
		lonCenter = cellWidth/2 - float64(longitude) - .5
	}
	pos := Position{Latitude: latCenter * step, Longitude: lonCenter * (2 * math.Pi / cellWidth)}
	return [2]int{longitude, latitude}, pos, true
}
