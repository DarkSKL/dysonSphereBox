package industryplan

import (
	"context"
	"errors"
	"slices"
	"time"

	"DysonSphereBox/helper/industryplan/search"
	"DysonSphereBox/helper/quant"
)

// LayoutMetrics 记录一次规划所有扩产轮次的实际布局调用、回执复用及分工修正耗时。
type LayoutMetrics struct {
	Rebuilds      int              `json:"rebuilds"`
	Reuses        int              `json:"reuses"`
	RebuildMillis int64            `json:"rebuildMillis"`
	RepairMillis  int64            `json:"repairMillis"`
	StageMillis   map[string]int64 `json:"stageMillis,omitempty"`
	Adjustments   map[string]int   `json:"adjustments,omitempty"`
}

// recordAdjustment 将一次退回分工的原因计入玩家能理解的统计，原始错误只供诊断。
func (m *LayoutMetrics) recordAdjustment(err error) {
	label := "收发站或其他施工条件需要调整"
	var power *PowerLoadError
	var mine *SourcePlacementError
	var factory *FactoryPlacementError
	switch {
	case errors.As(err, &power):
		label = "实装负荷需要增配电源与燃料"
		if power.AvailableMW != nil {
			label = "本星可建电源不足以承担原分工"
		}
	case errors.As(err, &mine):
		label = "可接驳矿点达不到原定采速"
	case errors.As(err, &factory):
		label = "当前选区未找到整段制造的可用落点"
	}
	if m.Adjustments == nil {
		m.Adjustments = make(map[string]int)
	}
	m.Adjustments[label]++
}

type layoutTimingKey struct{}

type layoutTiming struct {
	LayoutMetrics
	rebuild time.Duration
	repair  time.Duration
	stages  map[string]time.Duration
}

// WithLayoutMetrics 为一个串行规划任务建立计时器；调用方在 Plan 返回后读取结果。
func WithLayoutMetrics(ctx context.Context) (context.Context, *LayoutMetrics) {
	timing := &layoutTiming{stages: make(map[string]time.Duration)}
	timing.StageMillis = make(map[string]int64)
	return context.WithValue(ctx, layoutTimingKey{}, timing), &timing.LayoutMetrics
}

// NewLayoutStageTimer 累计本星各道施工工序的有效用时，失败重试也计入。传入空阶段结束本星计时。
func NewLayoutStageTimer(ctx context.Context) func(string) {
	timing, _ := ctx.Value(layoutTimingKey{}).(*layoutTiming)
	if timing == nil {
		return func(string) {}
	}
	clock := search.ActiveTimer(ctx)
	var current string
	var start time.Duration
	return func(next string) {
		now := clock()
		if current != "" {
			timing.stages[current] += now - start
			timing.StageMillis[current] = timing.stages[current].Milliseconds()
		}
		current, start = next, now
	}
}

type verifiedLayoutsKey struct{}

type siteCheckStats struct {
	failures int
	elapsed  time.Duration
}

func (s siteCheckStats) failureRate() float64 {
	return float64(s.failures) / (1 + s.elapsed.Seconds())
}

type siteCheckStatsKey struct{}

type verifiedLayout struct {
	site   PlanetPlan
	actual PlanetActual
}

// preferCandidate 比较含异星采矿或交付的方案，按玩家所选偏好排列同样可行的分工。
func preferCandidate(a, b Result, preference string) bool {
	if x, y := rank(a.Status), rank(b.Status); x != y {
		return x < y
	}
	if preference == "" || preference == "construction" {
		if len(a.Planets) != len(b.Planets) {
			return len(a.Planets) < len(b.Planets)
		}
		manufacturing := func(r Result) int {
			count := 0
			for _, site := range r.Planets {
				if site.Plan.MachineCount > 0 {
					count++
				}
			}
			return count
		}
		if first, second := manufacturing(a), manufacturing(b); first != second {
			return first < second
		}
	}
	return a.Score < b.Score
}

// singlePlanetResources 仅以必要条件筛选候选；矿物主题提供种类，精算后才比较可供速率。
func singlePlanetResources(req Request, base quant.Result, site Planet, surveyed bool) bool {
	if !enabled(req.EnableMining, true) {
		return true
	}
	for _, raw := range base.RawInputs {
		if slices.Contains(base.ExternalInputItemIDs, raw.ItemID) || raw.ItemID == 1208 && req.Dyson.Mode != "" {
			continue
		}
		capacity, found := 0.0, false
		for _, resource := range site.Resources {
			if resource.ItemID != raw.ItemID || resource.Kind == "orbital" {
				continue
			}
			found = true
			rate, _, _ := resourceCapacity(req, resource, raw.RatePerMinute)
			capacity += rate
		}
		if !found && (surveyed || len(site.Resources) > 0) ||
			surveyed && raw.RatePerMinute-capacity > quant.FlowTolerance(raw.RatePerMinute, capacity) {
			return false
		}
	}
	return true
}

// singlePlanetRequest 将能源与接收设施限定到本星；固定的异星交付或充电不能被改成自动选址。
func singlePlanetRequest(req Request, base quant.Result, id int64) (Request, bool) {
	for index, target := range req.Quant.Targets {
		if to := req.deliveryPlanet(index, target.ItemID); to != 0 && to != id {
			return req, false
		}
	}
	choice := req.energyChoice(id)
	if req.EnablePower && choice.Mode == "fixed" && choice.FacilityID == 2209 {
		return req, false
	}
	if enabled(req.EnableStorage, true) && len(base.Leftovers) > 0 && req.StoragePlanetID != 0 && req.StoragePlanetID != id {
		return req, false
	}
	local := expansionReferences(req, map[int64]bool{id: true})
	status, _ := photonPrecondition(local, base)
	return local, status == ""
}

// singlePlanetDelivery 检查本星合同是否结清，保留玩家明确允许的来料、供能与副产处理条件。
func singlePlanetDelivery(req Request, item Result, id int64) bool {
	if item.Status != "ready" && item.Status != "conditional" || len(item.Planets) != 1 || item.Planets[0].PlanetID != id || len(item.Routes) != 0 {
		return false
	}
	if item.StoragePlanetID != 0 && req.StoragePlanetID != 0 && item.StoragePlanetID != req.StoragePlanetID {
		return false
	}
	for _, external := range item.External {
		switch external.Kind {
		case "declared-input", "grid-power", "solar-conditions", "charging-peak", "surplus-removal":
		case "fuel-input":
			if req.EnableFuel {
				return false
			}
		case "mining-disabled":
			if enabled(req.EnableMining, true) {
				return false
			}
		default:
			return false
		}
	}
	return true
}
