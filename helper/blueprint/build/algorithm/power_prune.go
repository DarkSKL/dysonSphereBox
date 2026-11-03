package build

// 文件职责：在最终坐标上筛选可安全删除的普通感应塔。
// 使用：所有平移、Area 适配和锚点选择完成后，按原索引过滤 builder，再重建记录并运行最终规则审计。
// 边界：不移动消费者、不修改输入、不代替最终规则；仅减少 2201，保留每个原电网及其覆盖关系。

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"sort"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/transport"
)

// finalPowerPrune 职责：返回普通感应塔的原始建筑索引；使用：调用方过滤并重新编号；边界：索引不是 records 数组下标，不包含其他建筑。
type finalPowerPrune struct {
	// KeptPoleIndices 职责：列出保留塔；使用：诊断和集成检查；边界：仅包含 2201，按原索引递增。
	KeptPoleIndices []int32
	// RemovedPoleIndices 职责：列出删除塔；使用：过滤 builder；边界：使用重建编号前的原索引。
	RemovedPoleIndices []int32
	// NetworkCount 职责：保留原电网数量；使用：重建后的供电审计核对；边界：不代表已接入外部电源。
	NetworkCount int
}

// finalPowerPrunePoll 职责：约束长循环的取消响应；使用：建图、覆盖和连接骨架遍历；边界：取消时不返回部分删除集合。
type finalPowerPrunePoll struct {
	ctx context.Context
	// work 职责：累计扫描工作；使用：每 256 次检查取消；边界：仅属于本次去冗余。
	work uint64
}

// check 职责：按工作量检查取消；使用：长循环；边界：不改变输入或返回部分方案。
func (poll *finalPowerPrunePoll) check() error {
	poll.work++
	if poll.work%256 == 0 {
		return finalPowerPruneCancelled(poll.ctx)
	}
	return nil
}

// finalPowerPruneCancelled 职责：统一取消错误；使用：去冗余入口和出口；边界：只读取上下文。
func finalPowerPruneCancelled(ctx context.Context) error {
	if ctx.Err() != nil {
		return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "最终电塔去冗余已取消或超时"}
	}
	return nil
}

// planFinalPowerPrune 职责：计算覆盖与原电网均安全的去冗余方案；使用：最终 records 和对应 encoding，nil encoding 表示旧平面；边界：确定性贪心不承诺最少塔数，调用方仍须重建后审计。
func planFinalPowerPrune(ctx context.Context, encoding *surfaceEncoding, records []blueprint.Building) (finalPowerPrune, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := finalPowerPruneCancelled(ctx); err != nil {
		return finalPowerPrune{}, err
	}
	if encoding == nil {
		encoding = newPlanarRuleEncoding()
	}
	poll := finalPowerPrunePoll{ctx: ctx}
	byIndex := make(map[int32]int, len(records))
	poles := make([]int, 0)
	for i, record := range records {
		if err := poll.check(); err != nil {
			return finalPowerPrune{}, err
		}
		if _, duplicate := byIndex[record.Index]; duplicate || record.Index < 0 {
			return finalPowerPrune{}, &blueprint.Error{Code: "GEN_E_POWER_COVERAGE", Message: "最终电塔去冗余要求唯一且非负的建筑原索引"}
		}
		byIndex[record.Index] = i
		if record.ItemID == 2201 {
			poles = append(poles, i)
		}
	}
	// 先拒绝既有缺电或无效研究站链，不能用裁剪掩盖生成错误。
	networks, err := validateFinalPowerCoverage(ctx, encoding, records)
	if err != nil {
		return finalPowerPrune{}, err
	}
	if len(poles) == 0 {
		return finalPowerPrune{}, finalPowerPruneCancelled(ctx)
	}
	sort.Slice(poles, func(i, j int) bool { return records[poles[i]].Index < records[poles[j]].Index })
	positions, err := encoding.surfaceWorldPositions(records)
	if err != nil {
		return finalPowerPrune{}, err
	}
	centers, err := finalPowerPruneCenters(&poll, records, byIndex, positions)
	if err != nil {
		return finalPowerPrune{}, err
	}
	coverRadius, err := transport.TeslaPowerCoverRadius()
	if err != nil {
		return finalPowerPrune{}, err
	}
	connectDistance, err := transport.TeslaPowerConnectDistance()
	if err != nil {
		return finalPowerPrune{}, err
	}
	cellAt := func(point vector) [3]int {
		return [3]int{int(math.Floor(point.x / connectDistance)), int(math.Floor(point.y / connectDistance)), int(math.Floor(point.z / connectDistance))}
	}
	occupied := make(map[[3]int][]int)
	neighbors := make([][]int, len(poles))
	poleByIndex := make(map[int32]int, len(poles))
	pinned := make([]bool, len(poles))
	for p, i := range poles {
		if err := poll.check(); err != nil {
			return finalPowerPrune{}, err
		}
		record := records[i]
		poleByIndex[record.Index] = p
		// 自带连接或未知附加内容的实体保守保留，不让去冗余删掉用户语义。
		pinned[p] = finalPowerConsumer(record) || record.InputObjIdx >= 0 || record.OutputObjIdx >= 0 || len(record.Parameters) > 0 || record.Content != "" || len(record.RawTail) > 0
		cell := cellAt(positions[i])
		for x := -1; x <= 1; x++ {
			for y := -1; y <= 1; y++ {
				for z := -1; z <= 1; z++ {
					for _, q := range occupied[[3]int{cell[0] + x, cell[1] + y, cell[2] + z}] {
						if err := poll.check(); err != nil {
							return finalPowerPrune{}, err
						}
						if float64(surfacePowerDistanceSquared(positions[i], positions[poles[q]])) <= connectDistance*connectDistance {
							neighbors[p] = append(neighbors[p], q)
							neighbors[q] = append(neighbors[q], p)
						}
					}
				}
			}
		}
		occupied[cell] = append(occupied[cell], p)
	}
	for _, record := range records {
		if err := poll.check(); err != nil {
			return finalPowerPrune{}, err
		}
		for _, index := range [2]int32{record.InputObjIdx, record.OutputObjIdx} {
			if p, exists := poleByIndex[index]; exists {
				pinned[p] = true
			}
		}
	}
	component := make([]int, len(poles))
	for i := range component {
		component[i] = -1
	}
	components := make([][]int, 0, networks)
	for p := range poles {
		if component[p] >= 0 {
			continue
		}
		component[p] = len(components)
		queue := []int{p}
		for head := 0; head < len(queue); head++ {
			for _, q := range neighbors[queue[head]] {
				if err := poll.check(); err != nil {
					return finalPowerPrune{}, err
				}
				if component[q] < 0 {
					component[q] = component[p]
					queue = append(queue, q)
				}
			}
		}
		components = append(components, queue)
	}
	// 每个消费者在每个原电网内的覆盖均保留；不允许偷偷切换其接入的电网。
	coverGroups := make([]int, 0)
	poleGroups := make([][]int, len(poles))
	for i, record := range records {
		if err := poll.check(); err != nil {
			return finalPowerPrune{}, err
		}
		if !finalPowerConsumer(record) {
			continue
		}
		groups := make(map[int]int)
		cell := cellAt(centers[i])
		for x := -1; x <= 1; x++ {
			for y := -1; y <= 1; y++ {
				for z := -1; z <= 1; z++ {
					for _, p := range occupied[[3]int{cell[0] + x, cell[1] + y, cell[2] + z}] {
						if err := poll.check(); err != nil {
							return finalPowerPrune{}, err
						}
						if float64(surfacePowerDistanceSquared(centers[i], positions[poles[p]])) > coverRadius*coverRadius {
							continue
						}
						group, exists := groups[component[p]]
						if !exists {
							group = len(coverGroups)
							groups[component[p]] = group
							coverGroups = append(coverGroups, 0)
						}
						coverGroups[group]++
						poleGroups[p] = append(poleGroups[p], group)
					}
				}
			}
		}
	}
	kept, componentCounts := make([]bool, len(poles)), make([]int, len(components))
	for p := range poles {
		kept[p] = true
		componentCounts[component[p]]++
	}
	removable := func(p int) bool {
		if pinned[p] || componentCounts[component[p]] <= 1 {
			return false
		}
		for _, group := range poleGroups[p] {
			if coverGroups[group] <= 1 {
				return false
			}
		}
		return true
	}
	remove := func(p int) {
		kept[p] = false
		componentCounts[component[p]]--
		for _, group := range poleGroups[p] {
			coverGroups[group]--
		}
	}
	for p := len(poles) - 1; p >= 0; p-- {
		if err := poll.check(); err != nil {
			return finalPowerPrune{}, err
		}
		if removable(p) {
			remove(p)
		}
	}
	// 用原图的广度优先树恢复所选覆盖塔之间的路径；只恢复原塔，不创建连接或移动建筑。
	parents, connected := make([]int, len(poles)), make([]bool, len(poles))
	for p := range parents {
		parents[p] = -2
	}
	for _, members := range components {
		root := -1
		for _, p := range members {
			if err := poll.check(); err != nil {
				return finalPowerPrune{}, err
			}
			if kept[p] && (root < 0 || p < root) {
				root = p
			}
		}
		parents[root], connected[root] = -1, true
		queue := []int{root}
		for head := 0; head < len(queue); head++ {
			for _, q := range neighbors[queue[head]] {
				if err := poll.check(); err != nil {
					return finalPowerPrune{}, err
				}
				if parents[q] == -2 {
					parents[q] = queue[head]
					queue = append(queue, q)
				}
			}
		}
	}
	for p := range poles {
		if !kept[p] {
			continue
		}
		for q := p; !connected[q]; q = parents[q] {
			if err := poll.check(); err != nil {
				return finalPowerPrune{}, err
			}
			if !kept[q] {
				kept[q] = true
				componentCounts[component[q]]++
				for _, group := range poleGroups[q] {
					coverGroups[group]++
				}
			}
			connected[q] = true
		}
	}
	// 只剥离连接骨架的叶节点，保证连续裁剪也不会同时删除互为替代的桥接塔。
	witness, degree := make([][]int, len(poles)), make([]int, len(poles))
	for p, parent := range parents {
		if err := poll.check(); err != nil {
			return finalPowerPrune{}, err
		}
		if kept[p] && parent >= 0 {
			witness[p] = append(witness[p], parent)
			witness[parent] = append(witness[parent], p)
			degree[p]++
			degree[parent]++
		}
	}
	queue := make([]int, 0, len(poles))
	for p := len(poles) - 1; p >= 0; p-- {
		if kept[p] && degree[p] <= 1 {
			queue = append(queue, p)
		}
	}
	for head := 0; head < len(queue); head++ {
		p := queue[head]
		if err := poll.check(); err != nil {
			return finalPowerPrune{}, err
		}
		if !kept[p] || !removable(p) {
			continue
		}
		remove(p)
		for _, q := range witness[p] {
			if kept[q] {
				degree[q]--
				if degree[q] == 1 {
					queue = append(queue, q)
				}
			}
		}
	}
	result := finalPowerPrune{NetworkCount: networks}
	for p, i := range poles {
		if kept[p] {
			result.KeptPoleIndices = append(result.KeptPoleIndices, records[i].Index)
		} else {
			result.RemovedPoleIndices = append(result.RemovedPoleIndices, records[i].Index)
		}
	}
	filtered := make([]blueprint.Building, 0, len(records)-len(result.RemovedPoleIndices))
	for _, record := range records {
		if err := poll.check(); err != nil {
			return finalPowerPrune{}, err
		}
		if p, exists := poleByIndex[record.Index]; !exists || kept[p] {
			filtered = append(filtered, record)
		}
	}
	remainingNetworks, err := validateFinalPowerCoverage(ctx, encoding, filtered)
	if err != nil {
		return finalPowerPrune{}, err
	}
	if remainingNetworks != networks || len(components) != networks {
		return finalPowerPrune{}, &blueprint.Error{Code: "GEN_E_POWER_COVERAGE", Message: fmt.Sprintf("最终电塔去冗余改变了原电网数量：%d -> %d", networks, remainingNetworks)}
	}
	if err := finalPowerPruneCancelled(ctx); err != nil {
		return finalPowerPrune{}, err
	}
	return result, nil
}

// finalPowerPruneCenters 职责：复用最终审计的实体接电位置与底层供电语义；使用：覆盖计数；边界：分拣器首端不取中点，研究站链记忆化避免逐层反复扫描全部建筑。
func finalPowerPruneCenters(poll *finalPowerPrunePoll, records []blueprint.Building, byIndex map[int32]int, positions []vector) ([]vector, error) {
	centers := append([]vector(nil), positions...)
	base, visiting := make([]int, len(records)), make([]bool, len(records))
	for i := range base {
		base[i] = -1
	}
	for i, record := range records {
		if err := poll.check(); err != nil {
			return nil, err
		}
		if record.ItemID != 2901 || record.RecipeID <= 0 {
			continue
		}
		current, path := i, make([]int, 0)
		for base[current] < 0 {
			if err := poll.check(); err != nil {
				return nil, err
			}
			if visiting[current] {
				return nil, &blueprint.Error{Code: "GEN_E_POWER_COVERAGE", Message: fmt.Sprintf("研究站 %d 的堆叠供电链无效", record.Index)}
			}
			visiting[current] = true
			path = append(path, current)
			if records[current].InputObjIdx < 0 {
				base[current] = current
				break
			}
			j, exists := byIndex[records[current].InputObjIdx]
			if !exists || records[j].ItemID != record.ItemID || records[j].RecipeID != record.RecipeID {
				return nil, &blueprint.Error{Code: "GEN_E_POWER_COVERAGE", Message: fmt.Sprintf("研究站 %d 的堆叠供电链无效", record.Index)}
			}
			current = j
		}
		for _, j := range path {
			base[j], visiting[j] = base[current], false
		}
		centers[i] = positions[base[current]]
	}
	return centers, nil
}

// pruneFinalPower 职责：原子应用最终电塔去冗余并重映射块区间；使用：finish 完成锚点重试后、重建 records 前；边界：不移动实体或缩小 Area，不替代重建后的最终审计。
func (b *builder) pruneFinalPower(ctx context.Context, encoding *surfaceEncoding, records []blueprint.Building) (int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := finalPowerPruneCancelled(ctx); err != nil {
		return 0, err
	}
	if b.setting.NeedPower != nil && !*b.setting.NeedPower {
		return 0, nil
	}
	if len(b.all) != len(records) {
		return 0, &blueprint.Error{Code: "GEN_E_POWER_COVERAGE", Message: "最终电塔去冗余的建筑记录与装配器数量不一致"}
	}
	poll := finalPowerPrunePoll{ctx: ctx}
	hasPole := false
	for i, entity := range b.all {
		if err := poll.check(); err != nil {
			return 0, err
		}
		if entity == nil || records[i].Index != int32(i) || records[i].ItemID != entity.record.ItemID {
			return 0, &blueprint.Error{Code: "GEN_E_POWER_COVERAGE", Message: "最终电塔去冗余要求与装配器同序的最新记录"}
		}
		hasPole = hasPole || records[i].ItemID == 2201
	}
	if !hasPole {
		// 关闭自动供电的蓝图无需进入覆盖裁剪；是否要求覆盖仍由最终请求规则决定。
		return 0, finalPowerPruneCancelled(ctx)
	}
	plan, err := planFinalPowerPrune(ctx, encoding, records)
	if err != nil {
		return 0, err
	}
	if len(plan.RemovedPoleIndices) == 0 {
		return 0, nil
	}
	removed := make(map[*placed]bool, len(plan.RemovedPoleIndices))
	for _, index := range plan.RemovedPoleIndices {
		removed[b.all[int(index)]] = true
	}
	if err := b.validateFinalPowerPruneReferences(&poll, removed); err != nil {
		return 0, err
	}
	prefix := make([]int, len(b.all)+1)
	retained := make([]*placed, 0, len(b.all)-len(removed))
	for i, entity := range b.all {
		if err := poll.check(); err != nil {
			return 0, err
		}
		if !removed[entity] {
			retained = append(retained, entity)
		}
		prefix[i+1] = len(retained)
	}
	validRange := func(begin, end int) bool { return begin >= 0 && begin <= end && end <= len(b.all) }
	zones := append([]zoneRange(nil), b.zoneRanges...)
	for i := range zones {
		if err := poll.check(); err != nil {
			return 0, err
		}
		if !validRange(zones[i].begin, zones[i].end) {
			return 0, &blueprint.Error{Code: "GEN_E_POWER_COVERAGE", Message: "最终电塔去冗余遇到无效生产分区"}
		}
		zones[i].begin, zones[i].end = prefix[zones[i].begin], prefix[zones[i].end]
	}
	surfaceBlocks := append([]surfaceBlockRange(nil), b.surfaceBlocks...)
	flyBlocks := append([]surfaceBlockRange(nil), b.flyLayoutBlocks...)
	for _, blocks := range [][]surfaceBlockRange{surfaceBlocks, flyBlocks} {
		for i := range blocks {
			if err := poll.check(); err != nil {
				return 0, err
			}
			if !validRange(blocks[i].begin, blocks[i].end) {
				return 0, &blueprint.Error{Code: "GEN_E_POWER_COVERAGE", Message: "最终电塔去冗余遇到无效产线块"}
			}
			blocks[i].begin, blocks[i].end = prefix[blocks[i].begin], prefix[blocks[i].end]
		}
	}
	encodings := []*surfaceEncoding{encoding}
	if b.surfaceBusEncoding != nil && b.surfaceBusEncoding != encoding {
		encodings = append(encodings, b.surfaceBusEncoding)
	}
	areaMaps := make([]map[*placed]int, len(encodings))
	areaPlans := make([]*SurfacePlan, len(encodings))
	for i, current := range encodings {
		if current == nil || current.byBuilding == nil {
			continue
		}
		areaMaps[i] = make(map[*placed]int, len(retained))
		for _, entity := range retained {
			if err := poll.check(); err != nil {
				return 0, err
			}
			if area, exists := current.byBuilding[entity]; exists {
				areaMaps[i][entity] = area
			}
		}
		if current.plan != nil {
			counts := make(map[int]int, len(current.plan.Bands))
			for _, area := range areaMaps[i] {
				if err := poll.check(); err != nil {
					return 0, err
				}
				counts[area]++
			}
			plan := *current.plan
			plan.Bands = append([]SurfaceBandPlan(nil), current.plan.Bands...)
			for j := range plan.Bands {
				plan.Bands[j].BuildingCount = counts[plan.Bands[j].AreaIndex]
			}
			areaPlans[i] = &plan
		}
	}
	if err := finalPowerPruneCancelled(ctx); err != nil {
		return 0, err
	}
	// 到此所有失败与取消检查均完成；提交时不修改原切片的底层数组。
	b.all, b.zoneRanges, b.surfaceBlocks, b.flyLayoutBlocks = retained, zones, surfaceBlocks, flyBlocks
	for i, current := range encodings {
		if current != nil && areaMaps[i] != nil {
			current.byBuilding = areaMaps[i]
			if areaPlans[i] != nil {
				current.plan = areaPlans[i]
			}
		}
	}
	return len(removed), nil
}

// validateFinalPowerPruneReferences 职责：拒绝删除被关键领域对象引用的塔；使用：提交裁剪前；边界：空间快照 all 和 Area 映射不构成实体引用，领域图按指针去重且不进入外部包。
func (b *builder) validateFinalPowerPruneReferences(poll *finalPowerPrunePoll, removed map[*placed]bool) error {
	pointerType := reflect.TypeOf((*placed)(nil))
	builderType, lineType := reflect.TypeOf(*b), reflect.TypeOf(line{})
	encodingType := reflect.TypeOf(surfaceEncoding{})
	packagePath := builderType.PkgPath()
	candidates := make(map[uintptr]bool, len(removed))
	for entity := range removed {
		candidates[reflect.ValueOf(entity).Pointer()] = true
	}
	for _, entity := range b.all {
		if err := poll.check(); err != nil {
			return err
		}
		if removed[entity.input] || removed[entity.output] {
			return &blueprint.Error{Code: "GEN_E_POWER_COVERAGE", Message: "最终电塔去冗余拒绝删除被建筑连接引用的塔"}
		}
	}
	visited := make(map[uintptr]bool)
	var visit func(reflect.Value) error
	visit = func(value reflect.Value) error {
		if err := poll.check(); err != nil {
			return err
		}
		if value.Type() == pointerType {
			if !value.IsNil() && candidates[value.Pointer()] {
				return &blueprint.Error{Code: "GEN_E_POWER_COVERAGE", Message: "最终电塔去冗余拒绝删除被物流或生产对象引用的塔"}
			}
			return nil
		}
		switch value.Kind() {
		case reflect.Pointer:
			if value.IsNil() || value.Type().Elem().PkgPath() != packagePath || visited[value.Pointer()] {
				return nil
			}
			visited[value.Pointer()] = true
			return visit(value.Elem())
		case reflect.Struct:
			kind := value.Type()
			if kind.PkgPath() != packagePath || kind == encodingType {
				return nil
			}
			for i := 0; i < value.NumField(); i++ {
				// all 是装配/空间库存，包含原电塔并不表示物流使用了它；其它 *placed 引用全部检查。
				if (kind == builderType || kind == lineType) && kind.Field(i).Name == "all" {
					continue
				}
				if err := visit(value.Field(i)); err != nil {
					return err
				}
			}
		case reflect.Array, reflect.Slice:
			for i := 0; i < value.Len(); i++ {
				if err := visit(value.Index(i)); err != nil {
					return err
				}
			}
		case reflect.Map:
			iterator := value.MapRange()
			for iterator.Next() {
				if err := visit(iterator.Key()); err != nil {
					return err
				}
				if err := visit(iterator.Value()); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return visit(reflect.ValueOf(b))
}
