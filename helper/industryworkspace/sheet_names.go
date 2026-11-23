package industryworkspace

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/planet"
	"DysonSphereBox/helper/quant"
)

// productionName 按玩家的交付顺序合并同一物品，扩产时同时保留目标和可以净交付的产量。
func productionName(data *quant.Dataset, targets, achieved []quant.Target) string {
	requested, produced := map[int]float64{}, map[int]float64{}
	var order []int
	for _, t := range targets {
		if _, found := requested[t.ItemID]; !found {
			order = append(order, t.ItemID)
		}
		requested[t.ItemID] += t.RatePerMinute
	}
	for _, t := range achieved {
		produced[t.ItemID] += t.RatePerMinute
	}
	names := make([]string, 0, len(order))
	for _, id := range order {
		item := data.ByID[id].Name
		if item == "" {
			item = fmt.Sprintf("物品%d", id)
		}
		r := constructionRate(requested[id])
		if net, found := produced[id]; found && constructionRate(net) != r {
			r = "目标" + r + "-净产" + constructionRate(net)
		}
		names = append(names, item+"-"+r+"每分钟")
	}
	return cleanName(strings.Join(names, "＋"), 90)
}

// constructionRate 让文件名与施工说明采用相同的每分钟净产量精度。
func constructionRate(v float64) string {
	return strconv.FormatFloat(math.Round(v*10000)/10000, 'f', -1, 64)
}

// cleanName 保留中文星名，替换不能在 Windows 文件名里使用的字符。
func cleanName(name string, limit int) string {
	name = strings.Map(func(c rune) rune {
		if unicode.IsControl(c) || strings.ContainsRune(`<>:"/\|?*`, c) {
			return '－'
		}
		return c
	}, name)
	runes := []rune(name)
	if len(runes) > limit {
		runes = runes[:limit]
	}
	return strings.TrimRight(string(runes), ". ")
}

// addSheets 为同一几何布局的每个施工步骤写入原版游戏标题和锚点，预览和导出使用相同文本。
func (b *Bundle) addSheets(ctx context.Context, site *PlanetSheet, parts []ConstructionPart, multiPlanet bool) error {
	stationPeakMW := 0.0
	for i, part := range parts {
		if err := ctx.Err(); err != nil {
			return err
		}
		title := b.Report.ExportName
		if multiPlanet || len(parts) > 1 {
			title += fmt.Sprintf("-%s-%d", cleanName(site.Name, 20), site.ID)
		}
		if len(parts) > 1 {
			kind := "建筑"
			if part.Kind == "foundation" {
				kind = "地基"
			}
			title += fmt.Sprintf("-%02d%s", i+1, kind)
		}
		doc, err := blueprint.Decode(part.Code, true)
		if err != nil {
			return err
		}
		peak, err := planet.StationChargingMW(doc.Buildings)
		if err != nil {
			return err
		}
		stationPeakMW += peak
		doc.Name = title
		doc.Description = fmt.Sprintf("全线交付 %s。存档 %s（%d tick）。%s / %s，经度 %.5f°，纬度 %.5f°，朝向 %.2f°。第 %d/%d 张，按此顺序在同一锚点粘贴。%s", b.Report.ExportName, b.Report.SaveName, b.Report.Tick, site.Star, site.Name, site.Longitude, site.Latitude, site.Direction, i+1, len(parts), doc.Description)
		encoded, err := blueprint.Encode(doc, doc.EnvelopeVersion)
		if err != nil {
			return fmt.Errorf("%s 的蓝图标题核验失败：%w", title, err)
		}
		b.Report.BlueprintCount++
		id := site.ID
		// 兼容整星预览入口：星球编号指向最后粘贴的建筑图，其余编号在本任务中唯一。
		if i != len(parts)-1 {
			id = -int64(b.Report.BlueprintCount)
		}
		file := b.Report.ExportName + "/" + title + ".txt"
		site.Sheets = append(site.Sheets, BlueprintSheet{ID: id, Name: title, File: file, Kind: part.Kind, Buildings: part.Buildings, FoundationCells: part.FoundationCells})
		site.File = file
		b.Codes[id] = encoded.Blueprint
	}
	b.recordStationCharging(site, stationPeakMW)
	return nil
}
