package saveanalysis

// 文件职责：读取稳定存档快照并建立厂区、物品和物流塔关联。
// 使用：服务层在授权和选档后调用；边界：只读，解码失败不发布部分结果，不解析联机附档中的玩家信息。

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"sort"
)

// planetRecord 职责：保存单物品的内部星球索引；使用：快照查询；边界：统计缺失与零值分开。
type planetRecord struct {
	planetID int64
	totals   []int64
	slots    []Slot
}

// Snapshot 职责：保存一次解析的紧凑索引；使用：重复查询物品；边界：创建后只读，不保留整份存档或玩家身份。
type Snapshot struct {
	stations    []*layoutStationComponent
	lines       map[int64]*factoryLine
	digest      string
	gameVersion string
	tick        int64
	items       map[int64][]planetRecord
	// inventory 职责：保留分类货槽；使用：库存查询；边界：不含玩家和在途。
	inventory []InventoryEntry
	// planetIDs 职责：保存已建厂星球；使用：归属查询；边界：非完整星系。
	planetIDs []int64
	// seed、algorithm、starCount 职责：保存星系参数；使用：关联查询；边界：不执行星系生成。
	seed, algorithm, starCount int64
	// spheres 职责：保存按恒星展开的球层；使用：只读球体查询；边界：不含戴森云。
	spheres []SphereView
	// worldView 职责：保存恢复后的完整星系；使用：所有空间关联；边界：查询返回副本。
	worldView WorldView
}

// Read 职责：从已选择的主档读取稳定快照；使用：服务层完成路径校验后调用；边界：读取期间文件变化返回错误。
func Read(path string) (*Snapshot, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	before, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, fmt.Errorf("存档不是普通文件")
	}
	data := make([]byte, before.Size())
	_, err = io.ReadFull(f, data)
	if err != nil {
		return nil, err
	}
	after, err := f.Stat()
	if err != nil {
		return nil, err
	}
	current, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !os.SameFile(before, current) || before.Size() != after.Size() || before.Size() != int64(len(data)) || !before.ModTime().Equal(after.ModTime()) || !after.ModTime().Equal(current.ModTime()) {
		return nil, fmt.Errorf("读取期间存档发生变化，请重新选档")
	}
	return Decode(data)
}

// Decode 职责：校验完整主档并建立查询索引；使用：文件入口与边界测试；边界：未知区段、截断或关联矛盾均返回错误。
// 逻辑流：1. 顺序验证所有区段 => 2. 按厂区序号关联星球 => 3. 合并非空物流货槽 => 4. 释放无关数据。
func Decode(data []byte) (snapshot *Snapshot, err error) {
	defer func() {
		if failure := recover(); failure != nil {
			if parsed, ok := failure.(decodeError); ok {
				snapshot = nil
				err = parsed
			} else {
				panic(failure)
			}
		}
	}()
	r := &reader{data: data}
	header := r.readGameSave()
	if r.offset != int64(len(data)) {
		r.fail("文件尾存在未识别数据")
	}
	if r.production == nil || len(r.production.factoryStatPool) != len(r.planets) {
		r.fail("厂区与统计数量不一致")
	}
	s := r.buildIndex(header.gameTick, fmt.Sprintf("%x", sha256.Sum256(data)))
	s.lines = r.lines
	s.stations = r.stations
	s.gameVersion = fmt.Sprintf("%d.%d.%d.%d", header.majorGameVersion, header.minorGameVersion, header.releaseGameVersion, header.buildGameVersion)
	r.buildInventory(s)
	r.buildSpheres(s)
	r.buildWorld(s)
	return s, nil
}

// buildIndex 职责：关联已验证厂区和物流数据；使用：解码完成后构建紧凑索引；边界：空货槽忽略，历史缺失保留为 nil。
func (r *reader) buildIndex(tick int64, digest string) *Snapshot {
	s := &Snapshot{digest: digest, tick: tick, items: make(map[int64][]planetRecord)}
	s.planetIDs = append([]int64(nil), r.planets...)
	sort.Slice(s.planetIDs, func(i, j int) bool { return s.planetIDs[i] < s.planetIDs[j] })
	if r.world != nil && r.world.gameDesc != nil {
		s.seed, s.algorithm, s.starCount = r.world.gameDesc.galaxySeed, r.world.gameDesc.galaxyAlgo, r.world.gameDesc.starCount
	}
	planets := make(map[int64]bool)
	for index, planet := range r.planets {
		if planet <= 0 || planets[planet] {
			r.fail("厂区星球标识无效或重复")
		}
		planets[planet] = true
		seen := make(map[int64]bool)
		for _, product := range r.production.factoryStatPool[index].productPool {
			if product.itemId <= 0 || seen[product.itemId] || len(product.total) != 14 || product.numCount != 7200 || product.numCursor != 12 {
				r.fail("物品统计结构无效")
			}
			seen[product.itemId] = true
			totals := append([]int64(nil), product.total...)
			s.items[product.itemId] = append(s.items[product.itemId], planetRecord{planetID: planet, totals: totals, slots: []Slot{}})
		}
	}
	stationIDs := make(map[[2]int64]bool)
	for _, station := range r.stations {
		key := [2]int64{station.planetId, station.id}
		if !planets[station.planetId] || station.id <= 0 || stationIDs[key] {
			r.fail("物流塔星球关联或标识无效")
		}
		stationIDs[key] = true
		for index, slot := range station.storage {
			if slot.itemId == 0 {
				continue
			}
			if slot.itemId < 0 || slot.count < 0 || slot.maxWithKeepMode < 0 || slot.localLogic < 0 || slot.localLogic > 2 || slot.remoteLogic < 0 || slot.remoteLogic > 2 {
				r.fail("物流货槽字段无效")
			}
			value := Slot{StationID: station.id, GlobalID: station.gid, EntityID: station.entityId, Index: index, Inventory: slot.count, Capacity: slot.maxWithKeepMode % 100000000, LocalLogic: slot.localLogic, RemoteLogic: slot.remoteLogic, Stellar: station.isStellar != 0, Collector: station.isCollector != 0}
			records := s.items[slot.itemId]
			found := -1
			for i := range records {
				if records[i].planetID == station.planetId {
					found = i
					break
				}
			}
			if found < 0 {
				records = append(records, planetRecord{planetID: station.planetId, slots: []Slot{}})
				found = len(records) - 1
			}
			records[found].slots = append(records[found].slots, value)
			s.items[slot.itemId] = records
		}
	}
	for _, records := range s.items {
		sort.Slice(records, func(i, j int) bool { return records[i].planetID < records[j].planetID })
	}
	return s
}
