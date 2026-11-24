package industryworkspace

// 文件职责：从存档地形和现有厂区估算大单可用的连续陆面。使用：仅在候选建设星需要分担制造设备时采样。
// 边界：所得比例只决定初始设备份额，逐栋施工仍检查原版地形、矿点和已有建筑。

import (
	"context"
	"math"

	"DysonSphereBox/helper/saveanalysis"
	"DysonSphereBox/helper/saveanalysis/galaxy"
)

// factoryLandShare 按原版 200 分段星球的 40×20 格工区采样。低、中纬按地表面积取样，
// 工区内每五格检查一个陆点。保留一个候选份额，使被抽样遗漏的落点仍可实际试建。
func factoryLandShare(ctx context.Context, site saveanalysis.PlanetResourceDetail) (float64, bool) {
	terrain := site.Terrain
	if terrain.WaterItemID == nil || terrain.WaterHeight == nil {
		return 0, false
	}
	dryPlanet := *terrain.WaterItemID == 0
	if dryPlanet && len(site.Buildings) == 0 {
		return 1, true
	}
	if !dryPlanet && terrain.HeightSource != "generated" {
		return 0, false
	}
	const gridStep = math.Pi / 500
	type windowRow struct {
		latitude, longitudeStep float64
		occupied                []bool
	}
	var rows []windowRow
	for row := -135; row <= 135; row += 15 {
		lat := float64(row) * gridStep
		rows = append(rows, windowRow{lat, gridStep / math.Cos(lat), make([]bool, int(math.Round(48*math.Cos(lat))))})
	}
	// 仅在现有建筑的一端落入工区时扣掉这扇窗口。地表采样与端点使用相同的经线起点。
	mark := func(v galaxy.Vector) {
		if v == (galaxy.Vector{}) {
			return
		}
		lat, lon := math.Atan2(v.Y, math.Hypot(v.X, v.Z)), math.Atan2(v.X, v.Z)
		for _, row := range rows {
			if math.Abs(lat-row.latitude) > 10*gridStep {
				continue
			}
			width := len(row.occupied)
			scale := float64(width) / (2 * math.Pi)
			first := int(math.Ceil((lon - 20*row.longitudeStep) * scale))
			last := int(math.Floor((lon + 20*row.longitudeStep) * scale))
			for column := first; column <= last; column++ {
				row.occupied[(column+width)%width] = true
			}
		}
	}
	for i, building := range site.Buildings {
		if i%1024 == 0 && ctx.Err() != nil {
			return 0, false
		}
		mark(building.Position)
		if building.EndPosition != building.Position {
			mark(building.EndPosition)
		}
	}
	clear, total := 0, 0
	for _, row := range rows {
		if ctx.Err() != nil {
			return 0, false
		}
		for column, occupied := range row.occupied {
			total++
			if occupied {
				continue
			}
			if dryPlanet {
				clear++
				continue
			}
			longitude := float64(column) * 2 * math.Pi / float64(len(row.occupied))
			dry := true
			for dy := -10; dy <= 10 && dry; dy += 5 {
				y, radial := math.Sincos(row.latitude + float64(dy)*gridStep)
				for dx := -20; dx <= 20; dx += 5 {
					sin, cos := math.Sincos(longitude + float64(dx)*row.longitudeStep)
					h, known := terrain.HeightAt(galaxy.Vector{X: radial * sin, Y: y, Z: radial * cos})
					if !known {
						return 0, false
					}
					if h <= *terrain.WaterHeight+.05 {
						dry = false
						break
					}
				}
			}
			if dry {
				clear++
			}
		}
	}
	return float64(clear+1) / float64(total+1), true
}
