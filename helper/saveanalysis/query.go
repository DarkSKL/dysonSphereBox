package saveanalysis

// 文件职责：查询一次解析的物品索引。
// 使用：服务层返回全档列表和单物品详情；边界：无磁盘访问，不在历史缺失时制造零值。

import (
	"fmt"
	"sort"
)

// window 职责：匹配游戏的固定历史窗口；使用：汇总和详情共同调用；边界：不接受任意滑动窗口。
func (s *Snapshot) window(seconds int64) (Window, int, error) {
	durations := []int64{10, 60, 600, 3600, 36000, 360000}
	for i, duration := range durations {
		if seconds == duration {
			return Window{Seconds: seconds, EndTick: s.tick, BucketTicks: duration / 10}, i, nil
		}
	}
	return Window{}, 0, fmt.Errorf("不支持的统计窗口：请选择 10、60、600、3600、36000 或 360000 秒")
}

// counts 职责：读取同一时间层的生产和消耗；使用：查询内部；边界：没有统计区段时返回 nil。
func (p planetRecord) counts(index int) *Counts {
	if p.totals == nil {
		return nil
	}
	return &Counts{Produced: p.totals[index], Consumed: p.totals[index+7]}
}

// Summary 职责：合计全档物品；使用：解析完成后或切换时间窗；边界：库存仅汇总已分配物品的物流货槽。
func (s *Snapshot) Summary(seconds int64) (Summary, error) {
	window, index, err := s.window(seconds)
	if err != nil {
		return Summary{}, err
	}
	result := Summary{SHA256: s.digest, GameVersion: s.gameVersion, Window: window, Items: make([]ItemSummary, 0, len(s.items))}
	for id, records := range s.items {
		item := ItemSummary{ItemID: id}
		for _, record := range records {
			if counts := record.counts(index); counts != nil {
				if item.History == nil {
					item.History = &Counts{}
				}
				item.History.Produced += counts.Produced
				item.History.Consumed += counts.Consumed
				item.HistoryPlanets++
			}
			for _, slot := range record.slots {
				item.Inventory += slot.Inventory
				item.Capacity += slot.Capacity
			}
		}
		result.Items = append(result.Items, item)
	}
	sort.Slice(result.Items, func(i, j int) bool { return result.Items[i].ItemID < result.Items[j].ItemID })
	return result, nil
}

// Item 职责：展开物品的星球产耗和物流货槽；使用：选中物品时调用；边界：未知 ID 返回错误，返回切片与缓存隔离。
func (s *Snapshot) Item(id, seconds int64) (ItemDetail, error) {
	window, index, err := s.window(seconds)
	if err != nil {
		return ItemDetail{}, err
	}
	records, exists := s.items[id]
	if !exists {
		return ItemDetail{}, fmt.Errorf("存档中未找到物品 %d", id)
	}
	result := ItemDetail{SHA256: s.digest, ItemID: id, Window: window, Planets: make([]PlanetItem, 0, len(records))}
	for _, record := range records {
		result.Planets = append(result.Planets, PlanetItem{PlanetID: record.planetID, History: record.counts(index), Slots: append([]Slot{}, record.slots...)})
	}
	sort.Slice(result.Planets, func(i, j int) bool { return result.Planets[i].PlanetID < result.Planets[j].PlanetID })
	return result, nil
}
