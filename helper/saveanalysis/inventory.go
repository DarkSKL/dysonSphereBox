package saveanalysis

// 文件职责：建立箱子、储液罐及物流塔的分类库存视图。
// 使用：选档快照查询；边界：不包含背包、传送带、生产设备内部与在途物品，不推算可调拨量。

import "fmt"

// InventoryEntry 职责：描述单个容器货槽；使用：库存明细定位；边界：容量为空表示没有物品专属容量。
type InventoryEntry struct {
	// PlanetID 职责：标识厂区星球；使用：分组；边界：非数组下标。
	PlanetID int64 `json:"planetId"`
	// Kind 职责：区分 station、storage、tank；使用：分类汇总；边界：不隐式包含其他容器。
	Kind string `json:"kind"`
	// ContainerID 职责：保存分类内组件标识；使用：定位；边界：需与星球和分类组合。
	ContainerID int64 `json:"containerId"`
	// EntityID 职责：标识建筑实体；使用：定位；边界：仅在星球内唯一。
	EntityID int64 `json:"entityId"`
	// SlotIndex 职责：保存货槽位置；使用：逐项对照；边界：零起始，储液罐固定为零。
	SlotIndex int `json:"slotIndex"`
	// ItemID 职责：标识已分配物品；使用：过滤；边界：零为空槽。
	ItemID int64 `json:"itemId"`
	// Count 职责：保存当前数量；使用：合计；边界：不包含在途。
	Count int64 `json:"count"`
	// Capacity 职责：保存专属货槽容量；使用：塔或罐展示；边界：箱子共享空间不推算成某物品容量。
	Capacity *int64 `json:"capacity"`
}

// InventoryView 职责：返回可追溯的分类库存；使用：按星球和物品查询；边界：仅合计明确覆盖的三类设施。
type InventoryView struct {
	// SHA256 职责：标识内容快照；使用：对照；边界：不是路径。
	SHA256 string `json:"sha256"`
	// Tick 职责：标识保存时刻；使用：快照核对；边界：不是实时查询时刻。
	Tick int64 `json:"tick"`
	// Totals 职责：按分类合计匹配数量；使用：与明细核对；边界：不代表全世界所有库存。
	Totals map[string]int64 `json:"totals"`
	// Entries 职责：列出匹配货槽，包括空槽；使用：定位与空容器验收；边界：选择具体物品时排除其他物品和空槽。
	Entries []InventoryEntry `json:"entries"`
	// Excluded 职责：列明未纳入位置；使用：解释覆盖；边界：不是数量为零。
	Excluded []string `json:"excluded"`
}

// buildInventory 职责：按厂区归属提取设施库存；使用：解码末尾；边界：只读取 FactoryStorage，不收集同结构的玩家库存。
func (r *reader) buildInventory(s *Snapshot) {
	if len(r.storages) != len(r.planets) {
		r.fail("储物设施与厂区数量不一致")
	}
	for i, factory := range r.storages {
		planet := r.planets[i]
		for index, entry := range factory.storagePool {
			box := entry.storage
			if box == nil {
				continue
			}
			if box.id != int64(index+1) || box.entityId <= 0 || box.size != int64(len(box.grids)) {
				r.fail("箱子身份或货槽数量无效")
			}
			for slot, grid := range box.grids {
				if grid.itemId < 0 || grid.count < 0 || (grid.itemId == 0 && grid.count != 0) {
					r.fail("箱子库存无效")
				}
				s.inventory = append(s.inventory, InventoryEntry{PlanetID: planet, Kind: "storage", ContainerID: box.id, EntityID: box.entityId, SlotIndex: slot, ItemID: grid.itemId, Count: grid.count})
			}
		}
		for index, tank := range factory.tankPool {
			if tank.id == 0 {
				continue
			}
			if tank.id != int64(index+1) || tank.entityId <= 0 || tank.fluidId < 0 || tank.fluidCount < 0 || tank.fluidCapacity < 0 || (tank.fluidId == 0 && tank.fluidCount != 0) {
				r.fail("储液罐库存或身份无效")
			}
			capacity := tank.fluidCapacity
			s.inventory = append(s.inventory, InventoryEntry{PlanetID: planet, Kind: "tank", ContainerID: tank.id, EntityID: tank.entityId, ItemID: tank.fluidId, Count: tank.fluidCount, Capacity: &capacity})
		}
	}
	for _, station := range r.stations {
		for i, slot := range station.storage {
			if slot.itemId < 0 || slot.count < 0 || slot.maxWithKeepMode < 0 || (slot.itemId == 0 && slot.count != 0) {
				r.fail("物流库存无效")
			}
			capacity := slot.maxWithKeepMode % 100000000
			s.inventory = append(s.inventory, InventoryEntry{PlanetID: station.planetId, Kind: "station", ContainerID: station.id, EntityID: station.entityId, SlotIndex: i, ItemID: slot.itemId, Count: slot.count, Capacity: &capacity})
		}
	}
	// 只存放在普通容器的物品也必须能从全档列表查到，旧 Inventory 字段仍严格表示塔内库存。
	for _, entry := range s.inventory {
		if entry.ItemID == 0 {
			continue
		}
		records := s.items[entry.ItemID]
		found := false
		for _, record := range records {
			if record.planetID == entry.PlanetID {
				found = true
				break
			}
		}
		if !found {
			s.items[entry.ItemID] = append(records, planetRecord{planetID: entry.PlanetID, slots: []Slot{}})
		}
	}
}

// Inventory 职责：按星球、物品过滤分类库存；使用：零参数表示所有；边界：返回独立副本，拒绝负数或不存在的星球。
func (s *Snapshot) Inventory(planetID, itemID int64) (InventoryView, error) {
	if planetID < 0 || itemID < 0 {
		return InventoryView{}, fmt.Errorf("星球和物品标识不能为负数")
	}
	if planetID != 0 {
		found := false
		for _, planet := range s.planetIDs {
			if planet == planetID {
				found = true
				break
			}
		}
		if !found {
			for _, planet := range s.worldView.Planets {
				if planet.PlanetID == planetID {
					found = true
					break
				}
			}
			if !found {
				return InventoryView{}, fmt.Errorf("星球不在已解析星系中")
			}
		}
	}
	out := InventoryView{SHA256: s.digest, Tick: s.tick, Totals: map[string]int64{"station": 0, "storage": 0, "tank": 0}, Entries: []InventoryEntry{}, Excluded: []string{"player", "belt", "production_buffer", "in_transit"}}
	for _, entry := range s.inventory {
		if (planetID != 0 && entry.PlanetID != planetID) || (itemID != 0 && entry.ItemID != itemID) {
			continue
		}
		if entry.Capacity != nil {
			value := *entry.Capacity
			entry.Capacity = &value
		}
		out.Entries = append(out.Entries, entry)
		out.Totals[entry.Kind] += entry.Count
	}
	return out, nil
}
