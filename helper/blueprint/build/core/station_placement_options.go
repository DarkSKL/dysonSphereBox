package build

// 文件职责：为单星直连施工保留普通塔布局择优已经验证的另一种外形；不调度新候选。

import "sync"

// StationPlacementOptions 每种锚纬与宽高只保留评分最好的方案，总计至多两个。快照不可变，成功事件可能并发到达。
type StationPlacementOptions struct {
	mu   sync.Mutex
	best [2]stationPlacementOption
}

type stationPlacementOption struct {
	verified VerifiedSnapshot
	shape    [3]int
	towers   int
	path     string
}

func stationPlacementShape(s *SurfacePlan) [3]int {
	return [3]int{s.AnchorLatitudeGrid, s.LayoutWidth, s.LayoutHeight}
}

func (a stationPlacementOption) better(b stationPlacementOption) bool {
	if b.path == "" {
		return true
	}
	x, y := a.verified.Metrics(), b.verified.Metrics()
	if BetterLayoutCandidate(x, a.towers, y, b.towers) {
		return true
	}
	return a.path < b.path && !BetterLayoutCandidate(y, b.towers, x, a.towers)
}

// Offer 复用塔候选的评分，按稳定执行路径打破同分；只保留通过最终规则的球面几何。
func (choices *StationPlacementOptions) Offer(verified VerifiedSnapshot, path string) {
	if verified.validation.data == nil || verified.validation.data.surfacePlan == nil {
		return
	}
	towers, err := StationCandidateTowerCount(verified.geometry)
	if err != nil {
		return
	}
	candidate := stationPlacementOption{verified, stationPlacementShape(verified.validation.data.surfacePlan), towers, path}
	choices.mu.Lock()
	defer choices.mu.Unlock()
	for i := range choices.best {
		current := choices.best[i]
		if !candidate.better(current) {
			if current.shape == candidate.shape {
				return
			}
			continue
		}
		choices.best[i] = candidate
		if i == 0 && current.path != "" && current.shape != candidate.shape {
			choices.best[1] = current
		}
		return
	}
}

// Alternative 返回与最终首选的锚纬或包络不同的最优已验证方案。调用方仅在首选遍历完可用落点后编码。
func (choices *StationPlacementOptions) Alternative(primary *SurfacePlan) *VerifiedSnapshot {
	if primary != nil {
		for _, candidate := range choices.best {
			if candidate.path != "" && candidate.shape != stationPlacementShape(primary) {
				return &candidate.verified
			}
		}
	}
	return nil
}
