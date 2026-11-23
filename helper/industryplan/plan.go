package industryplan

import (
	"DysonSphereBox/helper/industryplan/search"
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/transport"
	"context"
	"errors"
	"fmt"
	"maps"
	"math"
	"reflect"
	"slices"
	"sort"
	"strings"
	"time"
)

// factorySearchDepth 表示原合同需要恢复的制造选区；初筛、四档中心和全部纬区分别去重。
type factorySearchDepth uint8

const (
	quickFactorySearch factorySearchDepth = iota
	centerFactorySearch
	allFactoryRegions
)

// assignment 职责：记录有限搜索的一个分工索引；使用：候选构建；边界：中心固定时保持不变。
type assignment struct {
	center   int64
	strategy int
	// verifyFirst 优先复核上一轮失败星球，避免为尚未修好的方案先重建其他星球。
	verifyFirst int64
	// canOpenHost 表示跨星方案还有新的建设星可承接制造，候选仍须通过真实布局。
	canOpenHost bool
	// fullSearch 记录已恢复完整选优的星球和查到的纬区。
	fullSearch map[int64]factorySearchDepth
	moves      map[string]int64
	changed    map[string]bool
	// excludedSources 只排除已由真实落位证明不可用的星球与物品组合。
	excludedSources map[resourceSite]bool
	// sourceLimits 保留矿区已验证可用的部分产能；剩余缺口由其他来源承担。
	sourceLimits map[resourceSite]float64
	// fullFactories 保留本次修复中制造容量已满的星球；已成功前缀可以留下，但不再加入新模块。
	fullFactories map[int64]bool
	// powerMultipliers 按实装负荷修正本星供电估算，不改变既有制造分工。
	powerMultipliers map[int64]float64
}

type resourceSite struct {
	planetID int64
	itemID   int
}

// SourcePlacementError 把不可放置的采集点带回分工层，下一轮可尝试同物料的其他来源。
type SourcePlacementError struct {
	ItemID int
	Err    error
	// AvailableRatePerMinute 为失败前已接通的实际速率；零表示没有可保留的采集能力。
	AvailableRatePerMinute float64
	// Description 带有星球和物品名称的玩家提示；底层错误仍供诊断和换矿源使用。
	Description string
}

func (e *SourcePlacementError) Error() string {
	if e.Description != "" {
		return e.Description
	}
	return e.Err.Error()
}
func (e *SourcePlacementError) Unwrap() error { return e.Err }

// PowerLoadError 职责：回传实装负荷及已知供电空间容量；使用：原星补电或按容量迁移制造；边界：缺省 AvailableMW 时仅表示需要补充电源。
type PowerLoadError struct {
	RequiredMW float64
	Err        error
	// AvailableMW 职责：记录燃料机组空间失败前已落位的供电容量；使用：确定可保留制造前缀；边界：nil 不限制容量，零表示本轮未放下有效电源。
	AvailableMW *float64
}

// Error 职责：保留供电不足的具体原因；使用：玩家提示与诊断；边界：不重复包装底层文案。
func (e *PowerLoadError) Error() string { return e.Err.Error() }

// Unwrap 职责：保留供电核对失败的原始原因；使用：错误分类；边界：不将普通落位失败改为负荷反馈。
func (e *PowerLoadError) Unwrap() error { return e.Err }

// FactoryPlacementError 职责：回传首个未能落位的制造模块；使用：真实容量失败后的批量迁移；边界：FirstUnplaced 是逐星 Productions 的零基下标，不表示已穷尽所有布局。
type FactoryPlacementError struct {
	FirstUnplaced   int
	DeferredRegions bool
	// MinimumPowerMW 记录当前整份合同必需的持续负荷下限，修正分工之外还可在原星补电。
	MinimumPowerMW float64
	Err            error
}

// Error 职责：保留具体落位原因；使用：玩家提示与诊断；边界：不重复包装底层文案。
func (e *FactoryPlacementError) Error() string { return e.Err.Error() }

// Unwrap 职责：保留容量失败的原始原因；使用：错误分类；边界：不将普通生成错误改为容量不足。
func (e *FactoryPlacementError) Unwrap() error { return e.Err }

// evaluated 职责：随候选保留星球分工来源；使用：失败后定点重排；边界：不保存蓝图。
type evaluated struct {
	result Result
	policy assignment
	single bool
}

// planOnce 职责：联合量化、分工、运力分配与实际蓝图复核；使用：共享同一父 context；边界：时间窗暂停保留搜索栈。
func planOnce(ctx context.Context, req Request, world World, validate Validator, progress func(Progress)) (out Result, err error) {
	submitted := req
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if progress == nil {
		progress = func(Progress) {}
	}
	req, profile, err := normalize(req)
	if err != nil {
		return Result{}, err
	}
	planets, ids, buildable, err := validateWorld(req, world)
	if err != nil {
		return Result{}, err
	}
	if err := validateEnergy(req, planets); err != nil {
		return Result{}, err
	}
	hadSiliconSource := slices.Contains(req.Quant.Settings.DirectResourceItemIDs, 1003)
	req = planningDefaults(req, planets, world.ResolveResources != nil)
	autoSilicon := !hadSiliconSource && slices.Contains(req.Quant.Settings.DirectResourceItemIDs, 1003)
	progress(Progress{Phase: "quant", Total: 1})
	base, err := quant.Solve(ctx, req.Quant)
	if err != nil {
		return Result{}, err
	}
	if trial, next, use := compareOilCracking(ctx, req, world, base); use {
		req, base = trial, next
		defer func() {
			if ctx.Err() != nil || err != nil {
				return
			}
			if out.Verified || validate == nil && (out.Status == "ready" || out.Status == "conditional") {
				if out.Status == "ready" {
					out.Status = "conditional"
				}
				out.Warnings = append(out.Warnings, "采用已解锁的 X 射线裂解消纳精炼油并生产氢和高能石墨。启动时给各裂解炼油厂预充氢，每台开始一轮需 2 个，之后由优先回氢带循环供料。")
				return
			}
			submitted.keepOilRecipe = true
			out, err = planOnce(ctx, submitted, world, validate, progress)
		}()
	}
	if status, message := photonPrecondition(req, base); status != "" {
		blocked := Result{Plan: base, Status: status, Warnings: []string{message}}
		if status != "infeasible" || req.Quant.Scheme.CostWeight.Power != 0 {
			return blocked, nil
		}
		// 戴森球余量是硬条件。先用已有的省电配方选择重算一次，保留玩家指定的配方、外供和分工偏好。
		alternative := req
		alternative.Quant.Scheme.CostWeight.Power = energyRecipeWeight
		plan, solveErr := quant.Solve(ctx, alternative.Quant)
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		if solveErr != nil {
			return blocked, nil
		}
		if status, _ := photonPrecondition(alternative, plan); status != "" {
			return blocked, nil
		}
		req, base = alternative, plan
		defer func() {
			out.Warnings = append(out.Warnings, "原自动配方的临界光子用量超过所选戴森球的余量，已按可用供给重新选择配方。")
		}()
	}
	progress(Progress{Phase: "quant", Completed: 1, Total: 1, WorkTotal: 1 + base.MachineCount, SimpleLayout: simpleLayout(base)})
	if err := validateDirectResources(req, planets, world.ResolveResources != nil); err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	// 候选覆盖全部可建设星球；算力档只限制并发，时间窗由可恢复检查点控制。
	limit := 4 * len(buildable)
	inspected, limited, err := resolveCandidateResources(ctx, req, world, base, planets, ids, limit, progress)
	if err != nil {
		return Result{}, err
	}
	req = allowSharedMining(req, base, planets, ids, validate)
	if autoSilicon {
		for _, gap := range miningSupplyGaps(req, base, planets, ids) {
			if gap.ItemID != 1003 || gap.PendingPlanets > 0 || gap.UnknownPlanets > 0 {
				continue
			}
			// 自动选择的硅矿不足时回退制造；玩家明确选择的直接采集仍保留真实缺口。
			req.Quant.Settings.DirectResourceItemIDs = slices.DeleteFunc(slices.Clone(req.Quant.Settings.DirectResourceItemIDs), func(id int) bool { return id == 1003 })
			base, err = quant.Solve(ctx, req.Quant)
			if err != nil {
				return Result{}, err
			}
			count, capped, resolveErr := resolveCandidateResources(ctx, req, world, base, planets, ids, limit, progress)
			if resolveErr != nil {
				return Result{}, resolveErr
			}
			inspected += count
			limited = limited || capped
			req = allowSharedMining(req, base, planets, ids, validate)
			break
		}
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	bound, hasAuxiliary, err := mandatoryInputsLowerBound(ctx, req, base)
	if err != nil {
		return Result{}, err
	}
	if hasAuxiliary && world.ResolveResources != nil {
		count, _, resolveErr := resolveCandidateResources(ctx, req, world, bound, planets, ids, limit, progress)
		if resolveErr != nil {
			return Result{}, resolveErr
		}
		inspected += count
		req = allowSharedMining(req, bound, planets, ids, validate)
		gaps := miningSupplyGaps(req, bound, planets, ids)
		for _, gap := range gaps {
			if gap.PendingPlanets == 0 && gap.UnknownPlanets == 0 {
				return Result{Plan: bound, CenterPlanetID: req.CenterPlanetID, Preference: req.Preference, Logistics: req.Logistics,
					Status: "infeasible", ResourceGaps: gaps, ResourcesInspected: inspected,
					Warnings: []string{"当前配方连同光子接收所必需的透镜，最低原料需求已超过已确认矿源；请调整采矿设备或倍率、配方，或关闭透镜"}}, nil
			}
		}
	}
	base, err = SplitProductions(ctx, base, 32)
	if err != nil {
		return Result{}, err
	}
	defer func() {
		out.ResourcesInspected = inspected
	}()
	if world.ResolveResources != nil && enabled(req.EnableMining, true) {
		if gaps := miningSupplyGaps(req, base, planets, ids); len(gaps) > 0 {
			// 矿点总供给已经是分工与运输能达到的上界，重排星球无法填平这个缺口。
			blocked := Result{Plan: base, CenterPlanetID: req.CenterPlanetID, Preference: req.Preference, Logistics: req.Logistics, Status: "missing_data", ResourceGaps: gaps}
			for _, gap := range gaps {
				if gap.PendingPlanets == 0 && gap.UnknownPlanets == 0 {
					blocked.Status, blocked.LimitStage = "infeasible", ""
					break
				}
				if limited && gap.PendingPlanets > 0 {
					blocked.Status, blocked.LimitStage = "budget_exhausted", "resources"
				}
			}
			return blocked, nil
		}
	}
	centers := centerOrder(req, base, planets, buildable)
	ctx = context.WithValue(ctx, verifiedLayoutsKey{}, map[int64]verifiedLayout{})
	ctx = context.WithValue(ctx, siteCheckStatsKey{}, map[int64]siteCheckStats{})
	// 真正生成一颗星的完整合同前，先排除矿种、速率、交付与射线接收的硬缺口。
	required := base
	if hasAuxiliary {
		required = bound
	}
	singleReq := req
	singleReq.probeSharedMining = validate != nil
	singleIndex, singleEvaluations := 0, 0
	nextSingle := func() (evaluated, bool, error) {
		for singleIndex < len(centers) {
			if err := search.Checkpoint(ctx); err != nil {
				return evaluated{}, false, err
			}
			id := centers[singleIndex]
			singleIndex++
			p := planets[id]
			localReq, ok := singlePlanetRequest(singleReq, base, id)
			if !ok || !singlePlanetResources(localReq, required, p, p.ResourcesKnown || p.resourcesChecked) {
				continue
			}
			localIDs := []int64{id}
			if !p.ResourcesKnown && !p.resourcesChecked && world.ResolveResources != nil {
				count, _, err := resolveCandidateResources(ctx, localReq, world, required, planets, localIDs, 1, progress)
				if err != nil {
					return evaluated{}, false, err
				}
				inspected += count
				p = planets[id]
			}
			if !singlePlanetResources(localReq, required, p, true) {
				continue
			}
			policy := assignment{center: id}
			singleEvaluations++
			item := evaluateEnergy(ctx, localReq, base, world, planets, localIDs, localIDs, profile, policy, progress)
			progress(Progress{Phase: "assign", Completed: singleEvaluations, Total: len(centers), PlanetID: id})
			if singlePlanetDelivery(req, item, id) {
				return evaluated{result: item, policy: policy, single: true}, true, nil
			}
		}
		return evaluated{}, false, nil
	}
	assignments := make([]assignment, 0, limit)
	// 在同一交付中心比较集中制造、就近加工和外围分工，再按玩家偏好选取实体布局。
	for _, center := range centers {
		for strategy := 0; strategy < 4; strategy++ {
			assignments = append(assignments, assignment{center: center, strategy: strategy})
		}
	}
	candidates := make([]evaluated, 0, len(assignments))
	seen := map[string]bool{}
	var rejected Result
	inspectedAssignments := 0
	nextCandidate := func(end int) (evaluated, bool) {
		for inspectedAssignments < end && ctx.Err() == nil {
			if search.Checkpoint(ctx) != nil {
				return evaluated{}, false
			}
			assignment := assignments[inspectedAssignments]
			inspectedAssignments++
			item := evaluateEnergy(ctx, req, base, world, planets, ids, buildable, profile, assignment, progress)
			progress(Progress{Phase: "assign", Completed: inspectedAssignments, Total: len(assignments), PlanetID: assignment.center})
			if item.Status != "ready" && item.Status != "conditional" || len(item.Planets) == 0 {
				if rejected.Status == "" || item.Status == "missing_data" && rejected.Status != "missing_data" || item.Status == rejected.Status && item.Score < rejected.Score {
					rejected = item
				}
				continue
			}
			key := fingerprint(item, assignment.fullSearch)
			if !seen[key] {
				seen[key] = true
				return evaluated{result: item, policy: assignment}, true
			}
		}
		return evaluated{}, false
	}
	// 单星条件不满足时才比较同一交付中心的不同跨星分工。
	seedCandidates := func() {
		for len(candidates) == 0 && inspectedAssignments < len(assignments) && ctx.Err() == nil {
			end := min(inspectedAssignments+4, len(assignments))
			for {
				candidate, ok := nextCandidate(end)
				if !ok {
					break
				}
				candidates = append(candidates, candidate)
			}
		}
		sort.SliceStable(candidates, func(i, j int) bool {
			return preferCandidate(candidates[i].result, candidates[j].result, req.Preference)
		})
	}
	defer func() { out.Candidates = inspectedAssignments + singleEvaluations }()
	current, single, err := nextSingle()
	if err != nil {
		return Result{}, err
	}
	next := 0
	if !single {
		seedCandidates()
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		if len(candidates) == 0 {
			if limited && rejected.Status == "missing_data" {
				rejected.Status, rejected.LimitStage = "budget_exhausted", "resources"
			}
			return rejected, nil
		}
		current, next = candidates[0], 1
	}
	var previous, failed Result
	var lastFailure int64
	var failures []string
	defer func() {
		if !out.Verified && len(failures) > 0 {
			out.Warnings = append(out.Warnings, failures...)
		}
	}()
	tried := map[string]bool{fingerprint(current.result, nil): true}
	timing, _ := ctx.Value(layoutTimingKey{}).(*layoutTiming)
	var repairElapsed func() time.Duration
	finishRepair := func() {
		if repairElapsed != nil {
			timing.repair += repairElapsed()
			timing.RepairMillis = timing.repair.Milliseconds()
			repairElapsed = nil
		}
	}
	defer finishRepair()
	verify := func(item *Result, previous Result, policy assignment) (int64, error) {
		if world.CheckSourceSites != nil && len(item.Planets) > 1 {
			for _, site := range item.Planets {
				if len(site.Mines) == 0 && site.Photons == nil {
					continue
				}
				if err := search.Checkpoint(ctx); err != nil {
					return site.PlanetID, err
				}
				if err := world.CheckSourceSites(ctx, site); err != nil {
					return site.PlanetID, fmt.Errorf("星球 %d：%w", site.PlanetID, err)
				}
			}
		}
		return verifyCandidate(ctx, item, previous, policy, validate, progress)
	}
	for repairs := 0; ; repairs++ {
		finishRepair()
		if err := search.Checkpoint(ctx); err != nil {
			return Result{}, err
		}
		item := current.result
		item.Repairs = repairs
		if validate == nil {
			return item, nil
		}
		current.policy.verifyFirst = lastFailure
		current.policy.canOpenHost = !current.single && len(item.Planets) > 1 && len(buildable) > len(current.policy.fullFactories)+1
		badSite, verifyErr := verify(&item, previous, current.policy)
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		if verifyErr == nil {
			if !current.single && len(item.Planets) > 1 && (req.Preference == "energy" || req.Preference == "logistics") {
				// 候选只在刚刚验证过的建设星中重新配平，保留已有的矿区容量和电力反馈。
				fixed := make([]int64, 0, len(item.Planets))
				for _, site := range item.Planets {
					fixed = append(fixed, site.PlanetID)
				}
				incumbent := item
				incumbent.Verified = true
				item = improveVerifiedFreight(ctx, incumbent, current.policy, func(p assignment) Result {
					if timing != nil {
						repairElapsed = search.ActiveTimer(ctx)
						defer finishRepair()
					}
					return evaluateEnergy(ctx, req, base, world, planets, fixed, fixed, profile, p, progress)
				}, func(next *Result, p assignment) (int64, error) {
					return verify(next, incumbent, p)
				}, func(p assignment, next Result, id int64, err error) (assignment, bool) {
					return repairAssignment(p, next, id, err, planets, fixed)
				})
				if err := ctx.Err(); err != nil {
					return Result{}, err
				}
			}
			item.Verified = true
			for i := range item.Planets {
				site := &item.Planets[i]
				if fleet := item.Actuals[site.PlanetID].Fleet; len(fleet) > 0 {
					site.VesselCount = 0
					for _, station := range fleet {
						site.VesselCount += station.Vessels
					}
				}
			}
			progress(Progress{Phase: "verified", Completed: len(item.Planets), Total: len(item.Planets)})
			return item, nil
		}
		lastFailure = badSite
		if timing != nil {
			timing.recordAdjustment(verifyErr)
			repairElapsed = search.ActiveTimer(ctx)
		}
		previous = item
		item.Actuals = nil
		item.Status = "layout_not_found"
		// 原布局工作流会合并各候选相同的错误，保留每轮首个差异以解释真正的局部瓶颈。
		seenLines := map[string]bool{}
		var lines []string
		for _, line := range strings.Split(verifyErr.Error(), "\n") {
			if line != "" && !seenLines[line] {
				lines = append(lines, line)
				seenLines[line] = true
			}
		}
		failures = append(failures, fmt.Sprintf("第 %d 次布局　%s", repairs+1, strings.Join(lines, "\n")))
		failed = item
		firstReason, _, _ := strings.Cut(verifyErr.Error(), "\n")
		progress(Progress{Phase: "repair", Completed: repairs, PlanetID: badSite, RepairReason: firstReason})
		if current.single {
			var power *PowerLoadError
			if errors.As(verifyErr, &power) && power.AvailableMW == nil {
				localIDs := []int64{badSite}
				if policy, ok := repairAssignment(current.policy, previous, badSite, verifyErr, planets, localIDs); ok {
					localReq, _ := singlePlanetRequest(singleReq, base, badSite)
					item := evaluateEnergy(ctx, localReq, base, world, planets, localIDs, localIDs, profile, policy, progress)
					key := fingerprint(item, nil)
					if singlePlanetDelivery(req, item, badSite) && !tried[key] {
						tried[key] = true
						current = evaluated{result: item, policy: policy, single: true}
						continue
					}
				}
			}
			alternative, ok, err := nextSingle()
			if err != nil {
				return Result{}, err
			}
			if ok {
				current = alternative
				tried[fingerprint(current.result, nil)] = true
				continue
			}
		}
		var placement *FactoryPlacementError
		if errors.As(verifyErr, &placement) && placement.MinimumPowerMW > 0 {
			// 当前电源不足以运行全部制造块。保留原星补电候选，以便同星已试放的前缀分工失败时恢复完整选区。
			power := &PowerLoadError{RequiredMW: placement.MinimumPowerMW, Err: verifyErr}
			if powered, ok := repairAssignment(current.policy, previous, badSite, power, planets, buildable); ok {
				// 保留原星补电作为后备。跨中心的待核分工也可能复用本中心已开工的上下游。
				assignments = append(assignments, powered)
			}
		}
		if errors.As(verifyErr, &placement) && placement.DeferredRegions && placement.MinimumPowerMW == 0 && current.policy.fullSearch[badSite] < allFactoryRegions {
			// 同星初筛后先试四档完整选优；原合同仍可恢复全部纬区。单星无承接点，直接查全。
			fallback := current.policy
			fallback.fullSearch = maps.Clone(fallback.fullSearch)
			if fallback.fullSearch == nil {
				fallback.fullSearch = map[int64]factorySearchDepth{}
			}
			fallback.fullSearch[badSite]++
			if current.single || len(item.Planets) == 1 {
				fallback.fullSearch[badSite] = allFactoryRegions
				current.policy = fallback
				continue
			}
			assignments = slices.Insert(assignments, inspectedAssignments, fallback)
		}
		current.policy.fullSearch = nil
		policy, ok := repairAssignment(current.policy, previous, badSite, verifyErr, planets, buildable)
		if ok {
			options := []assignment{policy}
			if !current.policy.fullFactories[badSite] {
				if spread, changed := distributeFactoryOverflow(policy, previous, badSite, planets); changed {
					options = []assignment{spread, policy}
				}
			}
			repaired := false
			for i, option := range options {
				local := evaluateEnergy(ctx, req, base, world, planets, ids, buildable, profile, option, progress)
				key := fingerprint(local, option.fullSearch)
				if (local.Status != "ready" && local.Status != "conditional") || len(local.Planets) == 0 || tried[key] {
					continue
				}
				// 分摊方案先核验。其余方案在需要时重新量化。
				assignments = slices.Insert(assignments, inspectedAssignments, options[i+1:]...)
				current = evaluated{result: local, policy: option}
				tried[key], repaired = true, true
				break
			}
			if repaired {
				continue
			}
		}
		if len(candidates) == 0 {
			seedCandidates()
		}
		for {
			if next == len(candidates) {
				candidate, ok := nextCandidate(len(assignments))
				if !ok {
					break
				}
				candidates = append(candidates, candidate)
			}
			if !tried[fingerprint(candidates[next].result, candidates[next].policy.fullSearch)] {
				break
			}
			next++
		}
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		if next == len(candidates) {
			break
		}
		current = candidates[next]
		tried[fingerprint(current.result, current.policy.fullSearch)] = true
		next++
	}
	if failed.LimitStage == "" && len(assignments) < 4*len(centers) {
		failed.Status, failed.LimitStage = "budget_exhausted", "assign"
	}
	return failed, nil
}

// verifyCandidate 职责：逐星球回收蓝图反馈；使用：初选和修复；边界：仅复用完整合同相同的成功回执。
func verifyCandidate(ctx context.Context, item *Result, previous Result, policy assignment, validate Validator, progress func(Progress)) (int64, error) {
	item.Actuals = map[int64]PlanetActual{}
	var done, total, available int
	for _, site := range item.Planets {
		total += layoutWork(site)
		if !policy.fullFactories[site.PlanetID] {
			available++
		}
	}
	// 每颗星只保留最近一次成功的合同。中途先在另一颗星失败也不会丢掉可复用的实物。
	verified, _ := ctx.Value(verifiedLayoutsKey{}).(map[int64]verifiedLayout)
	// 换位可能覆盖某颗星的最近回执，整线保留的原施工合同仍可直接使用。
	var incumbent map[int64]verifiedLayout
	if previous.Verified || verified == nil {
		incumbent = make(map[int64]verifiedLayout, len(previous.Actuals))
		for _, site := range previous.Planets {
			if actual, ok := previous.Actuals[site.PlanetID]; ok {
				incumbent[site.PlanetID] = verifiedLayout{site: site, actual: actual}
			}
		}
		if verified == nil {
			verified, incumbent = incumbent, nil
		}
	}
	timing, _ := ctx.Value(layoutTimingKey{}).(*layoutTiming)
	checks, _ := ctx.Value(siteCheckStatsKey{}).(map[int64]siteCheckStats)
	sites := item.Planets
	if (policy.verifyFirst != 0 || previous.Verified) && len(sites) > 1 {
		sites = slices.Clone(sites)
		// 先复查刚失败的星球，再按每秒实查时间发现的失败数排序。无记录及同分时先查小工地。
		sort.SliceStable(sites, func(i, j int) bool {
			a, b := sites[i], sites[j]
			if a.PlanetID == policy.verifyFirst || b.PlanetID == policy.verifyFirst {
				return a.PlanetID == policy.verifyFirst && b.PlanetID != policy.verifyFirst
			}
			if x, y := checks[a.PlanetID].failureRate(), checks[b.PlanetID].failureRate(); x != y {
				return x > y
			}
			return layoutWork(a) < layoutWork(b)
		})
	}
	for n, site := range sites {
		if err := ctx.Err(); err != nil {
			return site.PlanetID, err
		}
		p := Progress{Phase: "blueprint", Completed: n, Total: len(item.Planets), PlanetID: site.PlanetID, WorkDone: done, WorkTotal: total, SimpleLayout: simpleLayout(item.Plan)}
		progress(p)
		old, ok := verified[site.PlanetID]
		if !ok || !reflect.DeepEqual(old.site, site) {
			old, ok = incumbent[site.PlanetID]
			ok = ok && reflect.DeepEqual(old.site, site)
		}
		actual := old.actual
		if ok {
			if timing != nil {
				timing.Reuses++
			}
		} else {
			var err error
			layoutCtx := context.WithValue(ctx, placedMachinesKey{}, func(machines int) {
				p.WorkDone += machines
				progress(p)
			})
			input := site
			depth := policy.fullSearch[site.PlanetID]
			input.CanRelocate = depth == quickFactorySearch && (policy.canOpenHost || available > 1 || available == 1 && policy.fullFactories[site.PlanetID])
			input.CenterRegionsOnly = depth == centerFactorySearch && len(item.Planets) > 1
			input.SinglePlanet = len(item.Planets) == 1
			var elapsed func() time.Duration
			if timing != nil || checks != nil {
				elapsed = search.ActiveTimer(ctx)
			}
			actual, err = validate(layoutCtx, input)
			if err == nil {
				err = verifyPorts(site, actual)
			}
			var duration time.Duration
			if elapsed != nil {
				duration = elapsed()
			}
			if checks != nil {
				s := checks[site.PlanetID]
				s.elapsed += duration
				if err != nil {
					s.failures++
				}
				checks[site.PlanetID] = s
			}
			if timing != nil {
				timing.Rebuilds++
				timing.rebuild += duration
				timing.RebuildMillis = timing.rebuild.Milliseconds()
			}
			if err != nil {
				return site.PlanetID, fmt.Errorf("星球 %d：%w", site.PlanetID, err)
			}
			verified[site.PlanetID] = verifiedLayout{site: site, actual: actual}
		}
		item.Actuals[site.PlanetID] = actual
		done += layoutWork(site)
	}
	return 0, nil
}

// repairAssignment 职责：负荷不足原星补电，容量失败迁移未落位后缀，其他失败迁移单条最大产线；使用：有限局部修复；边界：保留成功前缀和其他分工，不声称穷尽所有布局。
func repairAssignment(policy assignment, previous Result, badSite int64, cause error, planets map[int64]Planet, buildable []int64) (assignment, bool) {
	var powerError *PowerLoadError
	if errors.As(cause, &powerError) && powerError.AvailableMW != nil {
		var capacityError *FactoryPlacementError
		for _, site := range previous.Planets {
			if site.PlanetID != badSite {
				continue
			}
			first, ok := powerCapacityPrefix(site, powerError)
			if !ok {
				return policy, false
			}
			capacityError = &FactoryPlacementError{FirstUnplaced: first, Err: cause}
			break
		}
		if capacityError == nil {
			return policy, false
		}
		cause = capacityError
	} else if powerError != nil {
		for _, site := range previous.Planets {
			if site.PlanetID != badSite || site.Power == nil {
				continue
			}
			old := max(1, policy.powerMultipliers[badSite])
			planned := site.Power.RequiredMW
			if !finite(planned) || planned <= 0 || !finite(powerError.RequiredMW) || powerError.RequiredMW <= planned || !finite(old) {
				return policy, false
			}
			next := old * powerError.RequiredMW / planned
			if !finite(next) || next <= old {
				return policy, false
			}
			policy.powerMultipliers = maps.Clone(policy.powerMultipliers)
			if policy.powerMultipliers == nil {
				policy.powerMultipliers = map[int64]float64{}
			}
			policy.powerMultipliers[badSite] = next
			policy.moves = maps.Clone(policy.moves)
			if policy.moves == nil {
				policy.moves = map[string]int64{}
			}
			for _, current := range previous.Planets {
				for _, blockID := range current.BlockIDs {
					policy.moves[blockID] = current.PlanetID
				}
			}
			return policy, true
		}
		return policy, false
	}
	var sourceError *SourcePlacementError
	if errors.As(cause, &sourceError) && sourceError.ItemID > 0 {
		key := resourceSite{planetID: badSite, itemID: sourceError.ItemID}
		if !policy.excludedSources[key] {
			available := sourceError.AvailableRatePerMinute
			old, limited := policy.sourceLimits[key]
			if finite(available) && available > 0 && (!limited || available < old) {
				policy.sourceLimits = maps.Clone(policy.sourceLimits)
				if policy.sourceLimits == nil {
					policy.sourceLimits = map[resourceSite]float64{}
				}
				policy.sourceLimits[key] = available
				return policy, true
			}
			policy.excludedSources = maps.Clone(policy.excludedSources)
			if policy.excludedSources == nil {
				policy.excludedSources = map[resourceSite]bool{}
			}
			policy.excludedSources[key] = true
			return policy, true
		}
		return policy, false
	}
	var factoryError *FactoryPlacementError
	if errors.As(cause, &factoryError) {
		used := map[int64]bool{}
		var failed *PlanetPlan
		for i := range previous.Planets {
			site := &previous.Planets[i]
			used[site.PlanetID] = true
			if site.PlanetID == badSite {
				failed = site
			}
		}
		if failed == nil || factoryError.FirstUnplaced < 0 || factoryError.FirstUnplaced >= len(failed.Plan.Productions) || len(failed.BlockIDs) != len(failed.Plan.Productions) {
			return policy, false
		}
		selected := connectedFactoryDestination(badSite, failed.Plan.Productions[factoryError.FirstUnplaced:], previous, used, policy.fullFactories, planets, buildable)
		if selected == 0 {
			return policy, false
		}
		policy.moves = map[string]int64{}
		for _, site := range previous.Planets {
			for _, blockID := range site.BlockIDs {
				policy.moves[blockID] = site.PlanetID
			}
		}
		for _, blockID := range failed.BlockIDs[factoryError.FirstUnplaced:] {
			policy.moves[blockID] = selected
		}
		policy.fullFactories = maps.Clone(policy.fullFactories)
		if policy.fullFactories == nil {
			policy.fullFactories = map[int64]bool{}
		}
		policy.fullFactories[badSite] = true
		return policy, true
	}
	var move quant.Production
	moveID := ""
	used := map[int64]bool{}
	for _, site := range previous.Planets {
		used[site.PlanetID] = true
		if site.PlanetID != badSite {
			continue
		}
		for i, line := range site.Plan.Productions {
			blockID := site.BlockIDs[i]
			if !policy.changed[blockID] && line.MachineCount > move.MachineCount {
				move = line
				moveID = blockID
			}
		}
	}
	if move.MachineCount == 0 {
		return policy, false
	}
	selected := connectedFactoryDestination(badSite, []quant.Production{move}, previous, used, policy.fullFactories, planets, buildable)
	if selected == 0 {
		return policy, false
	}
	policy.moves = map[string]int64{}
	for _, site := range previous.Planets {
		for _, blockID := range site.BlockIDs {
			policy.moves[blockID] = site.PlanetID
		}
	}
	policy.moves[moveID] = selected
	policy.changed = maps.Clone(policy.changed)
	if policy.changed == nil {
		policy.changed = map[string]bool{}
	}
	policy.changed[moveID] = true
	return policy, true
}

// powerCapacityPrefix 职责：按实际可放供电容量保留制造前缀；使用：燃料机组空间反馈；边界：仅扣除迁移必定释放的机器额定负荷，辅助设备和其他基础设施仍保留在固定需求中，下一轮重新核算并严格布局。
func powerCapacityPrefix(site PlanetPlan, feedback *PowerLoadError) (int, bool) {
	if site.Power == nil || feedback.AvailableMW == nil || len(site.Plan.Productions) == 0 || len(site.BlockIDs) != len(site.Plan.Productions) {
		return 0, false
	}
	available, required := *feedback.AvailableMW, max(site.Power.RequiredMW, feedback.RequiredMW)
	if !finite(available) || available < 0 || !finite(required) || required <= available+1e-6 {
		return 0, false
	}
	loads := make([]float64, len(site.Plan.Productions))
	manufacturing := 0.0
	for i, production := range site.Plan.Productions {
		load := production.PowerMW
		if production.Capacity != nil {
			load = production.Capacity.PowerMW
		}
		if !finite(load) || load < 0 {
			return 0, false
		}
		loads[i] = load
		manufacturing += load
	}
	if !finite(manufacturing) || manufacturing <= 0 || required+1e-6 < manufacturing || required-manufacturing > available+1e-6 {
		return 0, false
	}
	first := len(loads)
	for first > 0 && required > available+1e-6 {
		first--
		required -= loads[first]
	}
	return first, first < len(loads) && required <= available+1e-6
}

// factoryDestination 先复用已开工的星球，再按可建设地表、距离和星球编号选择制造承接星。
func factoryDestination(from int64, used, full map[int64]bool, planets map[int64]Planet, buildable []int64) int64 {
	var selected int64
	for _, id := range buildable {
		if id == from || full[id] {
			continue
		}
		if selected == 0 || preferFactoryHost(planets[from], planets[id], planets[selected], used) {
			selected = id
		}
	}
	return selected
}

// validateWorld 职责：校验星球主键、坐标和实际建设目标；使用：Plan；边界：未知资源可在求解结果中诊断。
func validateWorld(req Request, world World) (map[int64]Planet, []int64, []int64, error) {
	planets := make(map[int64]Planet, len(world.Planets))
	buildable := []int64{}
	for _, planet := range world.Planets {
		if !planet.ResourcesKnown && enabled(req.EnableMining, true) && world.CachedResources != nil {
			if resources, known, found := world.CachedResources(planet.ID); found {
				planet.Resources, planet.ResourcesKnown, planet.resourcesChecked = resources, known, true
			}
		}
		if planet.ID <= 0 || planet.StarID <= 0 || !finite(planet.Position[0]) || !finite(planet.Position[1]) || !finite(planet.Position[2]) {
			return nil, nil, nil, fmt.Errorf("星球身份或坐标无效")
		}
		if planet.Radius < 0 || !finite(planet.Radius) {
			return nil, nil, nil, fmt.Errorf("星球 %d 的运输半径无效", planet.ID)
		}
		if _, exists := planets[planet.ID]; exists {
			return nil, nil, nil, fmt.Errorf("存在重复星球 %d", planet.ID)
		}
		resources := map[int]bool{}
		for _, resource := range planet.Resources {
			if resources[resource.ItemID] || resource.ItemID <= 0 {
				return nil, nil, nil, fmt.Errorf("星球 %d 存在重复或无效资源 %d", planet.ID, resource.ItemID)
			}
			resources[resource.ItemID] = true
			for _, value := range []float64{resource.BaseRatePerMinute, resource.Count, resource.CapacityPerMinute, resource.UnitRatePerMinute} {
				if !finite(value) || value < 0 {
					return nil, nil, nil, fmt.Errorf("星球 %d 的资源能力无效", planet.ID)
				}
			}
			if resource.RemainingItems != nil && (!finite(*resource.RemainingItems) || *resource.RemainingItems < 0) {
				return nil, nil, nil, fmt.Errorf("星球 %d 的资源储量无效", planet.ID)
			}
		}
		planets[planet.ID] = planet
		if planet.Buildable {
			buildable = append(buildable, planet.ID)
		}
	}
	if len(buildable) == 0 {
		return nil, nil, nil, fmt.Errorf("存档中没有可建设星球")
	}
	if req.CenterPlanetID != 0 && !planets[req.CenterPlanetID].Buildable {
		return nil, nil, nil, fmt.Errorf("中心星球不可建设或不存在")
	}
	if req.StoragePlanetID != 0 && !planets[req.StoragePlanetID].Buildable {
		return nil, nil, nil, fmt.Errorf("仓储星球不可建设或不存在")
	}
	targets := map[int]bool{}
	if len(req.TargetPlanetIDs) > 0 && len(req.TargetPlanetIDs) != len(req.Quant.Targets) {
		return nil, nil, nil, fmt.Errorf("每个交付目标须对应一颗目的星或自动选择")
	}
	for i, id := range req.TargetPlanetIDs {
		if id != 0 && !planets[id].Buildable {
			return nil, nil, nil, fmt.Errorf("第 %d 项交付星球不可建设或不存在", i+1)
		}
	}
	for _, target := range req.Quant.Targets {
		targets[target.ItemID] = true
	}
	for item, id := range req.DeliveryPlanetIDs {
		if !targets[item] || id != 0 && !planets[id].Buildable {
			return nil, nil, nil, fmt.Errorf("物品 %d 的交付星球不适用", item)
		}
	}
	slices.Sort(buildable)
	return planets, slices.Sorted(maps.Keys(planets)), buildable, nil
}

// centerOrder 职责：优先考虑靠近目标交付及原料的中心；使用：有限候选顺序；边界：固定中心不参与排序。
func centerOrder(req Request, base quant.Result, planets map[int64]Planet, buildable []int64) []int64 {
	if req.CenterPlanetID != 0 {
		return []int64{req.CenterPlanetID}
	}
	demand := map[int]float64{}
	for _, item := range base.RawInputs {
		demand[item.ItemID] += item.RatePerMinute
	}
	score := map[int64]float64{}
	for _, id := range buildable {
		planet := planets[id]
		for _, resource := range planet.Resources {
			capacity, _, _ := resourceCapacity(req, resource, demand[resource.ItemID])
			score[id] += math.Min(capacity, demand[resource.ItemID])
		}
		for index, target := range base.Targets {
			if location := req.deliveryPlanet(index, target.ItemID); location != 0 {
				score[id] -= math.Log1p(distance(planet, planets[location])) * target.RatePerMinute
			}
		}
	}
	order := slices.Clone(buildable)
	sort.SliceStable(order, func(i, j int) bool {
		a, b := order[i], order[j]
		if score[a] != score[b] {
			return score[a] > score[b]
		}
		return !planets[a].HasFactory && planets[b].HasFactory
	})
	return order
}

// evaluate 职责：对一个分工同时结清所有物料；使用：候选搜索；边界：只输出可重建逐星球守恒图的切片。
func evaluate(ctx context.Context, req Request, base quant.Result, planets map[int64]Planet, ids, buildable []int64, profile transport.VesselProfile, assignment assignment, rules func(int64, int64, int64) (bool, bool)) Result {
	result := Result{CenterPlanetID: assignment.center, Plan: base, Preference: req.Preference, Status: "ready", Logistics: req.Logistics}
	c := candidate{ctx: ctx, req: req, center: assignment.center, ids: ids, world: make(map[int64]*location, len(ids)), profile: profile, result: result}
	c.excludedSources = assignment.excludedSources
	c.sourceLimits = assignment.sourceLimits
	c.powerMultipliers = assignment.powerMultipliers
	c.remoteRules = rules
	for _, id := range ids {
		c.world[id] = &location{planet: planets[id], balance: map[int]float64{}, plan: PlanetPlan{PlanetID: id}}
	}
	items, raw, explicit := map[int]bool{}, map[int]bool{}, map[int]bool{}
	for _, item := range base.RawInputs {
		raw[item.ItemID] = true
	}
	for _, item := range base.ExternalInputItemIDs {
		explicit[item] = true
	}
	blockIDs := productionBlockIDs(base.Productions)
	usedFactories := map[int64]bool{}
	for _, id := range assignment.moves {
		usedFactories[id] = true
	}
	for index, p := range base.Productions {
		blockID := blockIDs[index]
		site := c.world[assignment.moves[blockID]]
		if site == nil {
			site = c.placement(p, buildable, assignment.strategy, raw, explicit)
			if assignment.fullFactories[site.planet.ID] {
				destination := factoryDestination(site.planet.ID, usedFactories, assignment.fullFactories, planets, buildable)
				if destination == 0 {
					c.result.Status = "layout_not_found"
					c.result.Warnings = append(c.result.Warnings, "本轮已尝试的星球没有剩余制造容量，无法安放新增模块")
					return c.result
				}
				site = c.world[destination]
			}
		}
		usedFactories[site.planet.ID] = true
		site.plan.BlockIDs = append(site.plan.BlockIDs, blockID)
		site.plan.Plan.Productions = append(site.plan.Plan.Productions, p)
		site.plan.Plan.MachineCount += p.MachineCount
		site.plan.Plan.PowerMW += p.PowerMW
		for _, input := range p.Inputs {
			items[input.ItemID] = true
			site.balance[input.ItemID] -= input.RatePerMinute
		}
		for _, output := range p.Outputs {
			items[output.ItemID] = true
			site.balance[output.ItemID] += output.RatePerMinute
		}
	}
	for index, target := range req.Quant.Targets {
		id := req.deliveryPlanet(index, target.ItemID)
		if id == 0 {
			id = assignment.center
		}
		c.world[id].balance[target.ItemID] -= target.RatePerMinute
		c.world[id].add(Port{ItemID: target.ItemID, Kind: "delivery", RatePerMinute: target.RatePerMinute})
		items[target.ItemID] = true
	}
	// 先采集/制造辅助燃料并汇集至中心，确保这部分采矿负荷进入本轮电源计算。
	for _, supply := range req.energySupply {
		c.world[c.center].balance[supply.ItemID] -= supply.RatePerMinute
		items[supply.ItemID] = true
	}
	c.storage = req.StoragePlanetID
	if c.storage == 0 {
		c.storage = c.center
		maxSurplus := 0.0
		for _, id := range buildable {
			total := 0.0
			for _, item := range base.Leftovers {
				total += math.Max(0, math.Min(item.RatePerMinute, c.world[id].balance[item.ItemID]))
			}
			if total > maxSurplus {
				c.storage, maxSurplus = id, total
			}
		}
	}
	itemOrder := slices.Sorted(maps.Keys(items))
	for _, item := range itemOrder {
		if ctx.Err() != nil {
			return c.result
		}
		c.missing = false
		c.allocate(item, raw[item], explicit[item], false)
		if c.failure != "" {
			c.result.Status = "infeasible"
			if c.missing {
				c.result.Status = "missing_data"
			}
			c.result.Warnings = append(c.result.Warnings, c.failure)
			return c.result
		}
	}
	for _, supply := range req.energySupply {
		c.world[c.center].balance[supply.ItemID] += supply.RatePerMinute
	}
	beforePower := make(map[int64]float64, len(c.world))
	for id, site := range c.world {
		beforePower[id] = site.plan.OutboundVesselMW
	}
	if err := c.planPower(); err != nil {
		c.result.Status = "infeasible"
		c.result.Warnings = append(c.result.Warnings, err.Error())
		return c.result
	}
	if !matchingEnergySupply(req.energySupply, c.result.EnergyNeeds) {
		if !energyCovered(c.result.EnergyNeeds, req.energySupply) {
			// 换电或减少机组后先改辅助目标，再结算仓储。保留本轮已核准的自动电源以免去煤后重新选煤。
			c.result.settledAutoPower = map[int64]int{}
			for _, id := range c.ids {
				power := c.world[id].plan.Power
				if power != nil && power.FacilityID > 0 && req.energyChoice(id).Mode == "auto" {
					c.result.settledAutoPower[id] = power.FacilityID
				}
			}
		}
		return c.result
	}
	for _, item := range itemOrder {
		c.missing = false
		c.allocateSurplus(item)
		if c.failure != "" {
			c.result.Status = "infeasible"
			if c.missing {
				c.result.Status = "missing_data"
			}
			c.result.Warnings = append(c.result.Warnings, c.failure)
			return c.result
		}
	}
	// 确定燃料、透镜、电池循环和副产物的真实航线后，重做本候选的供电和燃料量化。
	for id, site := range c.world {
		extra := site.plan.OutboundVesselMW - beforePower[id]
		if extra > req.postPowerShipping[id]+quant.FlowTolerance(extra, req.postPowerShipping[id]) {
			if c.result.postPowerShipping == nil {
				c.result.postPowerShipping = make(map[int64]float64, len(req.postPowerShipping)+1)
				maps.Copy(c.result.postPowerShipping, req.postPowerShipping)
			}
			c.result.postPowerShipping[id] = extra
		}
	}
	if len(c.result.postPowerShipping) > 0 {
		return c.result
	}
	if err := c.finalize(ctx, base); err != nil {
		c.result.Status = "infeasible"
		c.result.Planets = nil
		c.result.Warnings = append(c.result.Warnings, err.Error())
		return c.result
	}
	if err := c.checkGrid(); err != nil {
		c.result.Status = "infeasible"
		c.result.Planets = nil
		c.result.Warnings = append(c.result.Warnings, err.Error())
		return c.result
	}
	c.result.Score, c.result.ScoreBasis = c.score()
	if enabled(req.EnableStorage, true) && len(base.Leftovers) > 0 {
		var sitesWithStorage int
		for _, site := range c.result.Planets {
			if slices.ContainsFunc(site.Ports, func(port Port) bool { return port.Kind == "storage" }) {
				c.result.StoragePlanetID = site.PlanetID
				sitesWithStorage++
			}
		}
		if sitesWithStorage > 1 {
			c.result.StoragePlanetID = 0
			c.result.Warnings = append(c.result.Warnings, fmt.Sprintf("余料按施工清单分别暂存在 %d 颗星球，请逐星留出仓容并持续消纳。", sitesWithStorage))
		}
	}
	for _, external := range c.result.External {
		if external.Kind == "resource-data" {
			c.result.Status = "missing_data"
			break
		}
		if external.Kind == "resource-shortfall" || external.Kind == "collector-placement" {
			c.result.Status = "infeasible"
		} else if c.result.Status == "ready" {
			c.result.Status = "conditional"
		}
	}
	if len(c.result.Routes) > 0 {
		c.result.Warnings = append(c.result.Warnings, "航时和船队吞吐采用存档坐标及可调船运参数估算")
	}
	if enabled(req.EnableStorage, true) && len(base.Leftovers) > 0 {
		c.result.Warnings = append(c.result.Warnings, fmt.Sprintf("副产物仓储覆盖 %.4g 分钟，装满前须清理或持续消耗", req.StorageMinutes))
		if c.result.Status == "ready" {
			c.result.Status = "conditional"
		}
	}
	return c.result
}

// placement 职责：在固定中心及外围就近策略中为整条产线选址；使用：候选构建；边界：不拆分整数机器或重复生产。
func (c *candidate) placement(production quant.Production, buildable []int64, strategy int, raw, external map[int]bool) *location {
	center := c.world[c.center]
	if strategy == 0 {
		return center
	}
	if strategy < 3 {
		var receiver int64
		var largestDelivery float64
		for index, target := range c.req.Quant.Targets {
			if id := c.req.deliveryPlanet(index, target.ItemID); target.ItemID == production.MainItemID && id != 0 && target.RatePerMinute > largestDelivery {
				receiver, largestDelivery = id, target.RatePerMinute
			}
		}
		if receiver != 0 {
			return c.world[receiver]
		}
	}
	best, bestScore := center, math.Inf(-1)
	for _, id := range buildable {
		site := c.world[id]
		score := 0.0
		for _, in := range production.Inputs {
			balance := site.balance[in.ItemID]
			localOutput := min(in.RatePerMinute, max(0, balance))
			score += localOutput * float64(strategy)
			if raw[in.ItemID] && !external[in.ItemID] {
				score += c.miningContribution(site, in.ItemID, max(0, -balance), in.RatePerMinute-localOutput)
			}
		}
		for _, out := range production.Outputs {
			score += math.Min(out.RatePerMinute, math.Max(0, -site.balance[out.ItemID]))
		}
		score -= math.Log1p(distance(center.planet, site.planet)) / float64(strategy)
		if score > bestScore || score == bestScore && id == c.center {
			best, bestScore = site, score
		}
	}
	return best
}

// rank 职责：优先无附加前提的候选；使用：排序；边界：缺资源数据的方案仍可作为带条件草案验证。
func rank(status string) int {
	switch status {
	case "ready":
		return 0
	case "conditional":
		return 1
	default:
		return 2
	}
}

// fingerprint 职责：避免为相同产线布局重复生成；使用：候选去重；边界：保留分工与端口差异。
func fingerprint(item Result, fullSearch map[int64]factorySearchDepth) string {
	var key strings.Builder
	fmt.Fprint(&key, item.CenterPlanetID)
	for _, id := range slices.Sorted(maps.Keys(fullSearch)) {
		fmt.Fprintf(&key, "/选区%d=%d", id, fullSearch[id])
	}
	for _, site := range item.Planets {
		fmt.Fprint(&key, "/", site.PlanetID, ":", site.Plan.PlanID, site.BlockIDs, site.Ports, site.Power, site.Photons, site.OutboundFleet)
	}
	return key.String()
}
