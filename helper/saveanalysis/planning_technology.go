package saveanalysis

// 文件职责：读取原版 GameHistoryData v20 的采矿、物流和蓝图科技。
// 使用：产业规划器使用存档时刻的有效能力；边界：戴森球余量、货船发航阈值仍由玩家声明。

// PlanningTechnology 是当前存档的施工限制和运输能力。速度以游戏米/秒计，包含科技倍率。
type PlanningTechnology struct {
	BlueprintLimit  int     `json:"blueprintLimit"`  // 单张允许的建筑数量，零表示未解锁。
	FoundationLimit int     `json:"foundationLimit"` // 原版 bpReformLimit，按地基格数计数。
	MiningSpeed     float64 `json:"miningSpeed"`
	MiningCostRate  float64 `json:"miningCostRate"`
	DroneCargo      int     `json:"droneCargo"`
	DroneSpeed      float64 `json:"droneSpeed"`
	ShipCargo       int     `json:"shipCargo"`
	CruiseSpeed     float64 `json:"cruiseSpeed"`
	WarpSpeed       float64 `json:"warpSpeed"`
	WarpUnlocked    bool    `json:"warpUnlocked"`
}

// PlanningTechnology 返回独立副本；手工构造且没有历史段的快照返回 nil。
func (s *Snapshot) PlanningTechnology() *PlanningTechnology {
	if s == nil || s.technology == nil {
		return nil
	}
	value := *s.technology
	return &value
}

// readPlanningTechnology 从科技队列后的 234 字节固定区段取值，保持下一段 CombatSettings 的游标。
func (r *reader) readPlanningTechnology() {
	start := r.offset
	scalar := func() float32 { return floatValue(r.number(4, false)) }
	r.skip(4) // universeObserveLevel
	t := &PlanningTechnology{BlueprintLimit: int(r.number(4, true)), FoundationLimit: int(r.number(4, true))}
	r.skip(36) // 太阳帆、分拣器和建造无人机字段，直到 manualPlantVegetation。
	droneBase, droneScale := scalar(), scalar()
	t.DroneSpeed = float64(droneBase * droneScale)
	t.DroneCargo = int(r.number(4, true))
	sail, warp, shipScale := scalar(), scalar(), scalar()
	t.CruiseSpeed, t.WarpSpeed = float64(sail*shipScale), float64(warp*shipScale)
	t.WarpUnlocked = r.number(1, false) != 0
	t.ShipCargo = int(r.number(4, true))
	r.skip(16) // 配送器速度、运量和配送角度。
	t.MiningCostRate, t.MiningSpeed = float64(scalar()), float64(scalar())
	r.technology = t
	r.skip(234 - (r.offset - start))
}
