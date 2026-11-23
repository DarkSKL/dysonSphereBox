package planet

// 文件职责：记住同一存档、模板和实际占地下已排除的格与已通过的落点，供运输或电力调整后重新装配。

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"math"
	"sync"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/saveanalysis"
)

// placeDirectShapes 首选已穷尽可用纬行时才读取并试放一次备用。超时或成功都直接交回该轮结果。
func placeDirectShapes(ctx context.Context, primary moduleTemplate, place func(moduleTemplate, bool) ([]Object, bool)) ([]Object, moduleTemplate) {
	objects, exhausted := place(primary, false)
	if len(objects) == 0 && exhausted && primary.alternative != nil && ctx.Err() == nil {
		other := primary.alternative(ctx)
		if other.generated.SurfacePlan != nil {
			if objects, _ := place(other, true); len(objects) > 0 {
				return objects, other
			}
		}
	}
	return objects, primary
}

type directSiteKey struct {
	planetID int64
	// 同一任务的不可变模板。即使两次生成输入相同，实际形状不同也不共享记录。
	first *blueprint.Building
	block int // 同一模板可在本星重复使用，各工区保留自己的搜索起点。
}

type directSitePrefix struct {
	snapshot, gameVersion, name string
	tick                        int64
	occupancy                   [sha256.Size]byte
	startRow, startX            int
	row, x                      int
	last                        error // 跳过已经穷尽的位置时保留原来的几何诊断。
	placedAt                    Position
	placed                      bool
}

// resumeModuleSite 复用同一工区、占地和搜索起点的成功落点，或续查此前排除的纬行。每个实际形状保留最近一次记录。
func resumeModuleSite(cache *sync.Map, req Request, resources saveanalysis.PlanetResourceDetail, template moduleTemplate, block int, cursor *moduleSiteCursor) func(bool) {
	if resources.SHA256 == "" || cursor.exhausted || len(template.document.Buildings) == 0 {
		return nil
	}
	key := directSiteKey{req.PlanetID, &template.document.Buildings[0], block}
	prefix := directSitePrefix{snapshot: resources.SHA256, gameVersion: req.GameVersion, name: req.Name, tick: resources.Tick, startRow: cursor.row, startX: cursor.x}
	var hashed bool
	if value, ok := cache.Load(key); ok {
		old := value.(directSitePrefix)
		if old.snapshot == prefix.snapshot && old.tick == prefix.tick && old.gameVersion == prefix.gameVersion && old.name == prefix.name && old.startRow == prefix.startRow && old.startX == prefix.startX {
			prefix.occupancy, hashed = directSiteOccupancy(req), true
			if old.occupancy == prefix.occupancy {
				cursor.row, cursor.x, cursor.last = old.row, old.x, old.last
				cursor.placedAt, cursor.certified = old.placedAt, old.placed
			}
		}
	}
	return func(placed bool) {
		if !placed && cursor.row <= prefix.startRow {
			return
		}
		if !hashed {
			prefix.occupancy = directSiteOccupancy(req)
		}
		prefix.row, prefix.x, prefix.last = cursor.row, cursor.x, cursor.last
		if placed {
			// 下次从同一格投影独立的实体及参数，运输端口由当前合同重新分配。
			prefix.x -= moduleLongitudeStride
			prefix.placedAt, prefix.placed = cursor.placedAt, true
		}
		cache.Store(key, prefix)
	}
}

type manufacturingOccupancy struct {
	hash    hash.Hash
	objects int
}

// directSiteOccupancy 按顺序登记存档障碍及前序工区双端点，追加工区只散列新增实体。单独调用时从零计算。
func directSiteOccupancy(req Request) [sha256.Size]byte {
	state := req.manufacturingOccupancy
	if state == nil {
		state = &manufacturingOccupancy{}
	}
	fresh := state.hash == nil || state.objects > len(req.Objects)
	if fresh {
		state.hash, state.objects = sha256.New(), 0
	}
	h := state.hash
	var word [8]byte
	write := func(value uint64) {
		binary.LittleEndian.PutUint64(word[:], value)
		h.Write(word[:])
	}
	position := func(p Position) {
		write(math.Float64bits(p.Longitude))
		write(math.Float64bits(p.Latitude))
		write(math.Float64bits(p.Height))
	}
	if fresh {
		for _, obstacles := range [][]Obstacle{req.Obstacles, req.minerals} {
			write(uint64(len(obstacles)))
			for _, obstacle := range obstacles {
				position(obstacle.Position)
				write(math.Float64bits(obstacle.Radius))
				flags := uint64(uint16(obstacle.stationItem))
				if obstacle.surfaceOnly {
					flags |= 1 << 16
				}
				write(flags)
			}
		}
	}
	for _, obj := range req.Objects[state.objects:] {
		write(uint64(uint16(obj.Building.ItemID))<<16 | uint64(uint16(obj.Building.ModelIndex)))
		position(obj.First)
		position(obj.Second)
	}
	state.objects = len(req.Objects)
	return [sha256.Size]byte(h.Sum(nil))
}
