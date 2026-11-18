package build

import (
	"fmt"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/transport"
)

const spaceWarperID = 1210

// stationInventory 在共享塔场和对外边界装箱时使用同一库存预算。运输船补给不占带口。
type stationInventory struct {
	slots       int
	warperStock int
}

func (inventory stationInventory) businessSlots(hasWarper bool) int {
	if inventory.warperStock > 0 && !hasWarper {
		return inventory.slots - 1
	}
	return inventory.slots
}

func containsWarper(items []stationItem) bool {
	for _, item := range items {
		if item.id == spaceWarperID {
			return true
		}
	}
	return false
}

func (inventory stationInventory) fits(items []stationItem) bool {
	return len(items) <= inventory.businessSlots(inventory.warperStock > 0 && containsWarper(items))
}

func validateWarperStock(stock int) error {
	if stock == 0 {
		return nil
	}
	limit, err := transport.StationItemCapacity(2104)
	if err != nil {
		return err
	}
	if stock < 0 || stock > limit {
		return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: fmt.Sprintf("翘曲器补给库存超出星际塔单槽容量 %d", limit)}
	}
	return nil
}

// validateStationWarperSlot 核对实际编码。自产供应和业务需求沿用原槽，纯船用补给须是本地仓储、星际需求。
func validateStationWarperSlot(record blueprint.Building, kinds, stock int) error {
	remoteBusiness, supplied, dedicated := false, false, false
	for i := 0; i < kinds; i++ {
		slot := record.Parameters[i*6 : i*6+4]
		if slot[0] == 0 {
			continue
		}
		if slot[0] != spaceWarperID {
			remoteBusiness = remoteBusiness || slot[2] != 0
			continue
		}
		dedicated = slot[1] == 0
		if dedicated {
			supplied = slot[2] == 2 && int(slot[3]) == stock
		} else {
			remoteBusiness = remoteBusiness || slot[2] != 0
			supplied = slot[1] == slot[2] && (slot[2] == 1 || slot[2] == 2) && int(slot[3]) >= stock
		}
	}
	if remoteBusiness && !supplied || !remoteBusiness && dedicated {
		return &blueprint.Error{Code: "GEN_E_STATION_PORT", Message: fmt.Sprintf("星际塔 %d 的翘曲器库存与远程供需角色不符", record.Index)}
	}
	return nil
}
