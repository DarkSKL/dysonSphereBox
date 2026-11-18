package saveanalysis

// 文件职责：列出星球生产过或已配置生产的物品，查询该快照的生产设备。
// 使用：选定星球后列出物品，再查询对应诊断；边界：历史件数不能归到当前单机。

import (
	"fmt"
	"slices"
	"sort"
)

// ProductionItem 表示物品的选择依据，设备数只计已定位的当前生产组件，科研消费者不计入产物。
type ProductionItem struct {
	ItemID             int64 `json:"itemId"`
	Machines           int   `json:"machines"`
	RecordedProduction bool  `json:"recordedProduction"`
}

// ProductionItems 使用所选星球的生产统计和设备实际配置建立选项，排除仅有消耗或库存的记录。
func (s *Snapshot) ProductionItems(planet int64) ([]ProductionItem, error) {
	if !slices.ContainsFunc(s.worldView.Planets, func(p PlanetLink) bool { return p.PlanetID == planet }) {
		return nil, fmt.Errorf("未知星球 %d", planet)
	}
	options := map[int64]ProductionItem{}
	if f := s.lines[planet]; f != nil {
		for _, d := range f.devices {
			if d.ProductID > 0 {
				v := options[d.ProductID]
				v.ItemID = d.ProductID
				v.Machines++
				options[d.ProductID] = v
			}
		}
		for _, m := range f.machines {
			for i, id := range m.products {
				if id <= 0 || slices.Contains(m.products[:i], id) {
					continue
				}
				v := options[id]
				v.ItemID = id
				v.Machines++
				options[id] = v
			}
		}
	}
	for item, records := range s.items {
		for _, r := range records {
			if r.planetID == planet && len(r.totals) == 14 && slices.ContainsFunc(r.totals[:7], func(v int64) bool { return v > 0 }) {
				v := options[item]
				v.ItemID = item
				v.RecordedProduction = true
				options[item] = v
				break
			}
		}
	}
	values := make([]ProductionItem, 0, len(options))
	for _, v := range options {
		values = append(values, v)
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ItemID < values[j].ItemID })
	return values, nil
}

// ProductionLine 诊断所选星球当前配置或历史有产出的物品，拒绝跨星球借用产品。
func (s *Snapshot) ProductionLine(planet, item int64) (CircuitReport, error) {
	options, err := s.ProductionItems(planet)
	if err != nil {
		return CircuitReport{}, err
	}
	if !slices.ContainsFunc(options, func(v ProductionItem) bool { return v.ItemID == item }) {
		return CircuitReport{}, fmt.Errorf("星球 %d 没有物品 %d 的设备或产出记录", planet, item)
	}
	r, err := s.productionLine(planet, item)
	if err != nil {
		return r, err
	}
	special := s.deviceReport(planet, item)
	if len(special.Devices) > 0 {
		r.DeviceDiagnostics = &special
		r.Coverage = "配方设备与其他生产组件按组件类型返回。能量枢纽只定位充放电产物来源，科研消费者可按星球查询。共享路径仅表明可达。"
	}
	return r, nil
}
