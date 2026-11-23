package industryworkspace

// 文件职责：按存档的原版蓝图科技分出施工图；使用：整星蓝图导出前；边界：每张图沿用同一个区域树、粘贴锚点和朝向。

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"

	"DysonSphereBox/helper/blueprint"
	buildcore "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/placement"
)

// ConstructionPart 记录施工顺序中一张已校验的游戏蓝图及其实际占用。
type ConstructionPart struct {
	Code            string // 原版游戏可读取的完整蓝图文本。
	Buildings       int    // 按原版科技计算，传送带和分拣器同样计入。
	FoundationCells int    // 按每个矩形的宽乘高计数。
	Kind            string // whole 为一张完整图，foundation 和 buildings 依施工顺序返回。
}

// SplitConstructionBlueprint 按每张允许的建筑数和地基格数分图。整张可用时保留原文，超限时先铺地基，再摆放完整连接组。
func SplitConstructionBlueprint(ctx context.Context, code string, maxBuildings, maxFoundation int) ([]ConstructionPart, error) {
	return splitConstructionBlueprintAt(ctx, code, maxBuildings, maxFoundation, nil)
}

// splitConstructionBlueprintAt 接收整星装配已知的锚点纬度（弧度），供超限建筑的跨纬带附件复核使用。
func splitConstructionBlueprintAt(ctx context.Context, code string, maxBuildings, maxFoundation int, latitude *float64) ([]ConstructionPart, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if maxBuildings < 0 || maxFoundation < 0 {
		return nil, fmt.Errorf("无法读取存档的蓝图科技上限")
	}
	doc, err := blueprint.Decode(code, true)
	if err != nil {
		return nil, err
	}
	cells := 0
	if doc.Reform != nil {
		for _, rect := range doc.Reform.Rects {
			if rect.W == 0 || rect.H == 0 || int(rect.AreaIndex) >= len(doc.Areas) {
				return nil, fmt.Errorf("地基矩形的尺寸或所属区域无效")
			}
			cells += int(rect.W) * int(rect.H)
		}
	}
	buildings := len(doc.Buildings)
	switch {
	case buildings == 0 && cells == 0:
		return nil, fmt.Errorf("蓝图中没有可施工的建筑或地基")
	case buildings > 0 && maxBuildings == 0:
		return nil, fmt.Errorf("当前科技允许的蓝图建筑数为 0，请解锁蓝图科技后重新生成")
	case cells > 0 && maxFoundation == 0:
		return nil, fmt.Errorf("当前科技允许的蓝图铺设地基数为 0，请解锁地基蓝图科技后重新生成")
	}
	// 原版 Import 用连接字段直接索引建筑数组。拒绝非原生的稀疏编号及悬空连接。
	for i, b := range doc.Buildings {
		if b.Index != int32(i) || int(b.AreaIndex) < 0 || int(b.AreaIndex) >= len(doc.Areas) ||
			b.InputObjIdx < -1 || int(b.InputObjIdx) >= buildings || b.OutputObjIdx < -1 || int(b.OutputObjIdx) >= buildings {
			return nil, fmt.Errorf("第 %d 个建筑的编号、所属区域或连接指向无效", i+1)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if buildings <= maxBuildings && cells <= maxFoundation {
		return []ConstructionPart{{Code: code, Buildings: buildings, FoundationCells: cells, Kind: "whole"}}, nil
	}
	if len(doc.RawTail) != 0 {
		return nil, fmt.Errorf("蓝图包含未知扩展，无法保证分图后保留其连接")
	}
	sheets := [][]blueprint.Building{doc.Buildings}
	if buildings > maxBuildings {
		sheets, err = partitionBuildings(ctx, doc, maxBuildings, latitude)
		if err != nil {
			return nil, err
		}
	}
	doc.OriginalGzip = nil
	var parts []ConstructionPart
	encode := func(kind string, records []blueprint.Building, reform *blueprint.ReformData, count int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		doc.Buildings, doc.Reform = records, reform
		result, err := blueprint.Encode(doc, doc.EnvelopeVersion)
		if err != nil {
			return err
		}
		parts = append(parts, ConstructionPart{Code: result.Blueprint, Buildings: len(records), FoundationCells: count, Kind: kind})
		return ctx.Err()
	}
	if cells > 0 {
		err = partitionFoundation(doc.Reform, maxFoundation, func(reform *blueprint.ReformData, count int) error {
			return encode("foundation", nil, reform, count)
		})
		if err != nil {
			return nil, err
		}
	}
	for _, sheet := range sheets {
		if len(sheet) > 0 {
			if err := encode("buildings", sheet, nil, 0); err != nil {
				return nil, err
			}
		}
	}
	return parts, nil
}

// partitionFoundation 按行分割跨越当前施工张上限的矩形，保留每格区域、涂装和颜色表。
func partitionFoundation(source *blueprint.ReformData, limit int, emit func(*blueprint.ReformData, int) error) error {
	sheet := *source
	sheet.Rects = nil
	count := 0
	for _, rect := range source.Rects {
		w, h := int(rect.W), int(rect.H)
		for offset := 0; offset < w*h; {
			x, y := offset%w, offset/w
			width, height := min(w-x, limit-count), 1
			if width == w {
				height = min(h-y, (limit-count)/w)
			}
			part := rect
			if int(rect.X)+x > math.MaxInt16 || int(rect.Y)+y > math.MaxInt16 {
				return fmt.Errorf("拆分地基后的起点超出游戏蓝图坐标范围")
			}
			part.X, part.Y = rect.X+int16(x), rect.Y+int16(y)
			part.W, part.H = uint8(width), uint8(height)
			sheet.Rects = append(sheet.Rects, part)
			offset += width * height
			count += width * height
			if count == limit {
				if err := emit(&sheet, count); err != nil {
					return err
				}
				sheet.Rects = nil
				count = 0
			}
		}
	}
	if count > 0 {
		return emit(&sheet, count)
	}
	return nil
}

// partitionBuildings 将每组实体及其输入、输出和附着设备装入同张图。重新编号仅修改原版用作对象索引的三个字段。
func partitionBuildings(ctx context.Context, doc blueprint.Document, limit int, latitude *float64) ([][]blueprint.Building, error) {
	buildings := doc.Buildings
	parents := make([]int, len(buildings))
	for i := range parents {
		parents[i] = i
	}
	root := func(i int) int {
		for parents[i] != i {
			parents[i] = parents[parents[i]]
			i = parents[i]
		}
		return i
	}
	join := func(a, b int) { parents[root(a)] = root(b) }
	for i, b := range buildings {
		if b.InputObjIdx >= 0 {
			join(i, int(b.InputObjIdx))
		}
		if b.OutputObjIdx >= 0 {
			join(i, int(b.OutputObjIdx))
		}
	}
	if err := joinBeltAttachments(ctx, doc, latitude, join); err != nil {
		return nil, err
	}
	// 先计组件，再按原图首次出现顺序分张；同张内原建筑顺序保持不变。
	sizes := make([]int, len(buildings))
	for i := range buildings {
		sizes[root(i)]++
	}
	sheetFor := make(map[int]int)
	var sheets [][]blueprint.Building
	count := 0
	nextIndex := make([]int32, len(buildings))
	for i, b := range buildings {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		group := root(i)
		sheet, found := sheetFor[group]
		if !found {
			if sizes[group] > limit {
				return nil, fmt.Errorf("一组相连建筑有 %d 个，超过存档单张上限 %d 个；请升级蓝图科技或降低交付产量", sizes[group], limit)
			}
			if len(sheets) == 0 || count+sizes[group] > limit {
				sheets = append(sheets, nil)
				count = 0
			}
			count += sizes[group]
			sheet = len(sheets) - 1
			sheetFor[group] = sheet
		}
		nextIndex[i] = int32(len(sheets[sheet]))
		sheets[sheet] = append(sheets[sheet], b)
	}
	for _, records := range sheets {
		for i := range records {
			b := &records[i]
			b.Index = int32(i)
			if b.InputObjIdx >= 0 {
				b.InputObjIdx = nextIndex[b.InputObjIdx]
			}
			if b.OutputObjIdx >= 0 {
				b.OutputObjIdx = nextIndex[b.OutputObjIdx]
			}
		}
	}
	return sheets, nil
}

// joinBeltAttachments 复核不写入输入/输出索引的原版附件。设备设置按原字节保留，遇到未知原型的私有参数则无法证明安全拆分。
func joinBeltAttachments(ctx context.Context, doc blueprint.Document, latitude *float64, join func(int, int)) error {
	type position struct {
		area  int8
		point blueprint.Point
	}
	var monitors []int
	var hasCoaters bool
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return err
	}
	verifiedModels := make(map[[2]int16]bool)
	for i, b := range doc.Buildings {
		key := [2]int16{b.ItemID, b.ModelIndex}
		if len(b.Parameters) != 0 && !verifiedModels[key] {
			if !slices.Contains(catalog.ModelsForItem(b.ItemID), b.ModelIndex) {
				return fmt.Errorf("建筑 #%d 的物品 %d、模型 %d 带有未知配置，无法验证配置中是否引用另一张图的建筑", i+1, b.ItemID, b.ModelIndex)
			}
			verifiedModels[key] = true
		}
		switch b.ItemID {
		case 2030:
			monitors = append(monitors, i)
		case 2313:
			hasCoaters = true
		}
	}
	if len(monitors) != 0 {
		belts := make(map[position]int)
		for i, b := range doc.Buildings {
			if b.ItemID >= 2001 && b.ItemID <= 2003 {
				key := position{b.AreaIndex, b.LocalOffset}
				if _, ok := belts[key]; ok {
					belts[key] = -1
				} else {
					belts[key] = i
				}
			}
		}
		for _, i := range monitors {
			b := doc.Buildings[i]
			host, ok := belts[position{b.AreaIndex, b.LocalOffset}]
			if !ok || host < 0 || math.Abs(math.Remainder(float64(b.Yaw-doc.Buildings[host].Yaw), 360)) >= 1e-2 {
				return fmt.Errorf("未能确定流速监测器 #%d 的宿主传送带，请提高蓝图科技或降低交付产量", i+1)
			}
			join(i, host)
		}
	}
	if !hasCoaters {
		return nil
	}
	if latitude == nil {
		// 已导出的整星图把固定锚点写在原生蓝图描述里，兼容再次拆分已生成的历史文件。
		marker := strings.LastIndex(doc.Description, "按固定经纬度和零度朝向粘贴。")
		var longitude, degree float64
		if marker < 0 {
			return fmt.Errorf("蓝图没有固定粘贴纬度，无法核对喷涂机的上层供剂带")
		}
		if _, err := fmt.Sscanf(doc.Description[marker:], "按固定经纬度和零度朝向粘贴。经度 %f°，纬度 %f°。", &longitude, &degree); err != nil {
			return fmt.Errorf("无法读取喷涂机供剂带所需的固定粘贴纬度")
		}
		radians := degree * math.Pi / 180
		latitude = &radians
	}
	if math.IsNaN(*latitude) || math.IsInf(*latitude, 0) || math.Abs(*latitude) >= math.Pi/2 {
		return fmt.Errorf("喷涂机的固定粘贴纬度超出原版行星可施工范围")
	}
	return buildcore.JoinPlanetCoaterBelts(ctx, doc, int(math.Round(*latitude*500/math.Pi)), join)
}
