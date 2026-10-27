package build

// 文件职责：验证并编排布局和自检；使用：factory Generate；边界：不信任前端修改的 plan，服务需另行复核来源指纹。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"context"
	"fmt"
	"math"
	"reflect"
	"time"
)

// normalizeSetting 职责：补齐参考默认值并限制生成规模；使用：布局前；边界：不截断请求机器总数。
func normalizeSetting(s Setting) (Setting, error) {
	defaults := Setting{MaxMachineInALine: 15, BeltLv: 1, InserterLv: 1, MaxLays: 3, BeltSpeed: 7200, SmelterMaxCount: 15, WorkbenchMaxCount: 15, RefineryMaxCount: 15, ChemicalMaxCount: 15, ColliderMaxCount: 15, MatrixMaxCount: 999, MergeType: 2, BlockWidthLimit: 50, LocalStationHitRadius: 7}
	v, d := reflect.ValueOf(&s).Elem(), reflect.ValueOf(defaults)
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		switch field.Kind() {
		case reflect.Int:
			if field.Int() < 0 || field.Int() > 10000 {
				return s, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "布局整数参数超限"}
			}
			if field.Int() == 0 {
				field.Set(d.Field(i))
			}
		case reflect.Float64:
			n := field.Float()
			if n < 0 || n > 1000000 || math.IsNaN(n) || math.IsInf(n, 0) {
				return s, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "布局数值无效"}
			}
			if n == 0 {
				field.Set(d.Field(i))
			}
		}
	}
	if s.NeedPower == nil {
		value := true
		s.NeedPower = &value
	}
	if s.Mix2 == nil {
		value := true
		s.Mix2 = &value
	}
	if s.BeltLv > 3 || s.InserterLv > 4 || s.MaxLays > 15 || s.MergeType > 2 || s.BlockWidthLimit < 10 {
		return s, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "带/分拣器等级、堆叠或布局方式无效"}
	}
	return s, nil
}

// lineCounts 职责：按参考运力和列上限均匀拆列；使用：每条量化产线；边界：运力不足时减列，不丢机器。
func lineCounts(p quant.Production, r quant.Recipe, s Setting, speed float64) ([]int, error) {
	maxUse := 0.
	for _, n := range r.ItemCounts {
		maxUse = math.Max(maxUse, n)
	}
	output := 0.
	for _, n := range r.ResultCounts {
		output += n
	}
	maxUse = math.Max(maxUse, output)
	acc := 1.
	if p.ProductionMode == 1 && p.IncreaseLv > 0 {
		rates := []float64{0, .25, .5, 1}
		acc += rates[p.IncreaseLv]
	}
	theory := int(math.Floor(math.Round(s.BeltSpeed/(60/(r.TimeSpend/60)*speed*maxUse*acc)*100) / 100))
	limit := s.maxCount(p.MachineType)
	if s.MaxMachineInALine > 0 && p.MachineType != 6 {
		limit = min(limit, s.MaxMachineInALine)
	}
	limit = min(limit, theory)
	if limit <= 0 {
		return nil, &blueprint.Error{Code: "GEN_E_OVERFLOW", Message: "带速不足以支持一台机器，请升级传送带"}
	}
	lines := int(math.Ceil(float64(p.MachineCount) / float64(limit)))
	balanced := int(math.Ceil(float64(p.MachineCount) / float64(lines)))
	result := []int{}
	for left := p.MachineCount; left > 0; {
		n := min(left, balanced)
		result = append(result, n)
		left -= n
	}
	return result, nil
}

// coatSupply 职责：连接同组喷涂机供剂带；使用：有喷涂的布局；边界：默认暴露供给口，不伪造已存在的增产剂。
func (b *builder) coatSupply(lines []*line) {
	for _, l := range lines {
		if len(l.coaters) == 0 {
			continue
		}
		first, last := l.coaters[0], l.coaters[len(l.coaters)-1]
		points := []vector{}
		for y := first.a.y - 1; y <= last.a.y; y++ {
			points = append(points, vector{first.a.x - 1, y, 1})
		}
		owner := b.owner
		b.owner = l.machines[0].owner
		belts := b.byPath(points)
		b.owner = owner
		id := 1141
		if l.info.IncreaseLv == 2 {
			id = 1142
		} else if l.info.IncreaseLv >= 3 {
			id = 1143
		}
		belts[0].record.Parameters = []int32{int32(id), 0}
		b.coaterLinks[id] = append(b.coaterLinks[id], belts[0])
	}
	if len(b.coaterLinks) > 0 {
		b.warnings = append(b.warnings, "喷涂机供剂带已预留：请向带标记的入口持续提供增产剂，喷涂不足会降低产能")
	}
}

// Generate 职责：生成并严格自检蓝图；使用：服务对已验证方案调用；边界：超时/布局失败不返回部分串，预览不参与判定。
func Generate(parent context.Context, req Request) (Result, error) {
	result := Result{Warnings: []string{}, Bindings: []ProductionBinding{}, SharedBuildingIndices: []int{}}
	if req.Mode < 1 || req.Mode > 6 {
		return result, &blueprint.Error{Code: "GEN_E_MODE", Message: "策略必须为 1 到 6"}
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	setting, err := normalizeSetting(req.Setting)
	if err != nil {
		return result, err
	}
	dataset, err := quant.LoadDataset(req.Plan.Dataset)
	if err != nil {
		return result, err
	}
	if req.Plan.DataVersion != dataset.Info.DataVersion || req.Plan.PlanID == "" {
		return result, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "数据集已变化，请重新量化"}
	}
	if len(req.Plan.Productions) == 0 {
		return result, &blueprint.Error{Code: "BP_E_EMPTY", Message: "当前方案没有可生成的制造产线"}
	}
	b, err := newBuilder(setting)
	if err != nil {
		return result, err
	}
	if req.Mode == 6 {
		return b.generateBus(ctx, req, dataset)
	}
	nextX, nextY, columnWidth := 0., 0., 0.
	for index, p := range req.Plan.Productions {
		if ctx.Err() != nil {
			return result, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "生成超过 30 秒"}
		}
		if p.MachineCount <= 0 || p.MachineCount > 10000 || p.IncreaseLv < 0 || p.IncreaseLv > 3 {
			return result, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "方案机器数量或等级无效"}
		}
		ri, ok := dataset.RecipeByID[p.RecipeID]
		if !ok {
			return result, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "配方已变化"}
		}
		recipe := dataset.Recipes[ri]
		valid := false
		for _, id := range recipe.Factories {
			if p.MachineID == id {
				valid = true
			}
		}
		if !valid {
			return result, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "设施与配方不匹配"}
		}
		b.owner = index
		counts, err := lineCounts(p, recipe, setting, dataset.ByID[p.MachineID].Speed)
		if err != nil {
			return result, err
		}
		perGroup := len(counts)
		if req.Mode == 3 || req.Mode == 4 {
			perGroup = max(1, 12/(len(recipe.Items)+1))
			slots := 4
			if req.Mode == 4 {
				slots = 5
			}
			if len(recipe.Items)+len(recipe.Results) > slots {
				perGroup = 1
			}
		}
		for groupStart := 0; groupStart < len(counts); groupStart += perGroup {
			groupEnd := min(groupStart+perGroup, len(counts))
			begin := len(b.all)
			lines := []*line{}
			lineY := 0.
			for i := groupStart; i < groupEnd; i++ {
				l, err := b.buildLine(p, recipe, counts[i], req.Mode, i, i == len(counts)-1)
				if err != nil {
					return result, err
				}
				minPoint, maxPoint := bounds(l.all)
				shift(l.all, -minPoint.x, lineY-minPoint.y, 0)
				lineY += math.Ceil(maxPoint.y-minPoint.y) + 1
				lines = append(lines, l)
			}
			if req.Mode == 3 || req.Mode == 4 {
				if err := b.attachStations(ctx, lines, req.Mode); err != nil {
					return result, err
				}
			}
			b.coatSupply(lines)
			block := b.all[begin:]
			minimum, maximum := bounds(block)
			width, height := math.Ceil(maximum.x-minimum.x)+1, math.Ceil(maximum.y-minimum.y)+1
			if setting.MergeType == 2 && nextY > 0 && nextY+height > setting.BlockWidthLimit {
				nextX += columnWidth + setting.LocalStationHitRadius
				nextY = 0
				columnWidth = 0
			}
			shift(block, nextX-minimum.x, nextY-minimum.y, 0)
			nextY += height + setting.LocalStationHitRadius
			columnWidth = math.Max(columnWidth, width)
			if len(b.all) > 100000 {
				return result, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "建筑超过 10 万，请拆分生成"}
			}
		}
		actualCapacity := []float64{0, 360, 720, 1800}[setting.BeltLv] * 4
		if p.OutputCount > actualCapacity {
			b.warnings = append(b.warnings, fmt.Sprintf("配方 %d 目标 %.2f/min 超过单带四层运力 %.2f/min，需多线供货并确认集装", p.RecipeID, p.OutputCount, actualCapacity))
		}
		if len(recipe.Results) > 1 {
			b.warnings = append(b.warnings, fmt.Sprintf("配方 %d 的多个产物共用输出带，请保证所有副产物有消耗出口", p.RecipeID))
		}
	}
	return b.finish(ctx, req, dataset)
}

// finish 职责：构建文档、归属和严格编码；使用：所有模式共同出口；边界：只有 Go 自检通过才返回串。
func (b *builder) finish(ctx context.Context, req Request, dataset *quant.Dataset) (Result, error) {
	result := Result{}
	if b.setting.NeedCoaterNet {
		if err := b.coaterNetwork(ctx); err != nil {
			return result, err
		}
	}
	minimumBefore, _ := bounds(b.all)
	shift(b.all, -minimumBefore.x, -minimumBefore.y, 0)
	if ctx.Err() != nil {
		return result, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "生成超过 30 秒"}
	}
	records, bindings, shared, err := b.records()
	if err != nil {
		return result, err
	}
	minimum, maximum := bounds(b.all)
	width, height := math.Ceil(maximum.x-minimum.x)+1, math.Ceil(maximum.y-minimum.y)+1
	if width > 32767 || height > 32767 {
		return result, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "蓝图尺寸超过区域整数上限"}
	}
	name := req.Name
	if name == "" {
		name = "量化产线"
	}
	document := blueprint.Document{EnvelopeVersion: 1, BinaryVersion: 2, Patch: 1, Name: name, GameVersion: dataset.Info.GameVersion, Description: "戴森球盒子生成；需确认所有原料及副产物出口", DragBoxSize: blueprint.PointInt{X: int32(width), Y: int32(height)}, Areas: []blueprint.Area{{Index: 0, ParentIndex: -1, AreaSegments: 200, Width: int16(width), Height: int16(height)}}, Buildings: records}
	encoded, err := blueprint.Encode(document, 1)
	if err != nil {
		return result, err
	}
	if len(records) > 6000 {
		b.warnings = append(b.warnings, "建筑超过 6000，预览可能较慢，可折叠预览后复制导出")
	}
	return Result{Blueprint: encoded.Blueprint, BuildingCount: len(records), Areas: 1, Warnings: b.warnings, SourcePlanID: req.Plan.PlanID, Bindings: bindings, SharedBuildingIndices: shared}, nil
}
