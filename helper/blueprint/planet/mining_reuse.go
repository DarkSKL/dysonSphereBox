package planet

import (
	"context"
	"slices"
	"sync"
)

// miningHistoryKey 按星球和接驳距离方案分别保留最近一次真实矿机序列。
type miningHistoryKey struct {
	planet         int64
	nearMineTowers bool
}

type miningHistory struct {
	snapshot            string
	tick                int64
	belt                int
	warpers             bool
	points              []veinPoint
	obstacles, minerals []Obstacle
	steps               []miningStep
}

// miningChoice 包含影响本次矿机、塔和接驳带选择的全部可变条件。剩余需求决定固矿搜索提前停止的位置。
type miningChoice struct {
	item, facility            int
	speed, setting, remaining float64
	exporting                 bool
}

type miningStep struct {
	choice  miningChoice
	objects []Object
	ids     []int
	rate    float64
}

type miningReplay struct {
	cache    *sync.Map
	key      miningHistoryKey
	previous []miningStep
	current  miningHistory
}

// beginMiningReplay 只在相同存档、星球、科技选项和初始占用下复用。矿区在本轮第一批施工；
// 后续矿机还必须逐步匹配同一前缀，确保已占矿点、避让体及连接编号相同。
func beginMiningReplay(ctx context.Context, req *Request, input ProductionRequest, points []veinPoint) *miningReplay {
	cache, _ := ctx.Value(templateCacheKey{}).(*sync.Map)
	if cache == nil || input.Resources.SHA256 == "" || len(req.Objects) != 0 || len(input.Plan.Mines) == 0 {
		return nil
	}
	r := &miningReplay{cache: cache, key: miningHistoryKey{planet: req.PlanetID, nearMineTowers: input.Options.nearMineTowers}}
	r.current = miningHistory{snapshot: input.Resources.SHA256, tick: input.Resources.Tick,
		belt: input.Options.Setting.BeltLv, warpers: input.Options.Setting.RequestStationWarpers}
	if v, ok := cache.Load(r.key); ok {
		old := v.(miningHistory)
		if old.snapshot == r.current.snapshot && old.tick == r.current.tick && old.belt == r.current.belt && old.warpers == r.current.warpers &&
			slices.Equal(old.points, points) && slices.Equal(old.obstacles, req.Obstacles) && slices.Equal(old.minerals, req.minerals) {
			r.current.points, r.current.obstacles, r.current.minerals = old.points, old.obstacles, old.minerals
			r.previous = old.steps
			return r
		}
	}
	r.current.points = slices.Clone(points)
	r.current.obstacles = slices.Clone(req.Obstacles)
	r.current.minerals = slices.Clone(req.minerals)
	return r
}

// copyMineObjects 隔离可变的蓝图参数；后续物流分组、功率和仓储设置可能会修改建筑副本。
func copyMineObjects(objects []Object) []Object {
	copied := slices.Clone(objects)
	for i := range copied {
		b := &copied[i].Building
		b.Parameters = slices.Clone(b.Parameters)
		b.RawTail = slices.Clone(b.RawTail)
	}
	return copied
}

func (r *miningReplay) restore(choice miningChoice) ([]Object, []int, float64, bool) {
	if r == nil || len(r.previous) == 0 {
		return nil, nil, 0, false
	}
	step := r.previous[0]
	old := step.choice
	// 找不到足量单机时遍历全部落点并选最大速率。两次需求都超过该速率，选址必定相同。
	// 若旧搜索提前找到足量落点，只能提高门槛至该落点仍足量；降低门槛可能提前选中别处。
	if choice.facility == 2301 || choice.facility == 2316 {
		fullRate := step.rate + 1e-6
		if old.remaining > fullRate && choice.remaining > fullRate ||
			old.remaining <= choice.remaining && choice.remaining <= fullRate {
			old.remaining = choice.remaining
		}
	}
	if old != choice {
		// 从首个差异起，后面的矿机原址、空地及编号都要重新判断。
		r.previous = nil
		return nil, nil, 0, false
	}
	r.previous = r.previous[1:]
	r.current.steps = append(r.current.steps, step)
	return copyMineObjects(step.objects), step.ids, step.rate, true
}

func (r *miningReplay) remember(choice miningChoice, objects []Object, ids []int, rate float64) {
	if r != nil {
		r.current.steps = append(r.current.steps, miningStep{choice, copyMineObjects(objects), slices.Clone(ids), rate})
	}
}

func (r *miningReplay) finish() {
	if r != nil && len(r.current.steps) != 0 {
		r.cache.Store(r.key, r.current)
	}
}
