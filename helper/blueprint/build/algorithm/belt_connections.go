package build

// 文件职责：把多路汇流编码为游戏连接池中独占的传送带输入槽，并检查粘贴后的连接存续。
// 使用：统一编号出口及最终规则审计；边界：不以几何接触替代实体连接，不修改分拣器动态槽或设施端口。

import (
	"context"
	"fmt"

	"DysonSphereBox/helper/blueprint"
)

// assignBeltInputSlots 职责：为同一带的各个上游分配独立的1..3输入槽；使用：最终records完成编号之后；边界：设施进带占用slot1，超过三路拒绝，分拣器的动态-1槽保持不变。
func assignBeltInputSlots(records []blueprint.Building) error {
	byIndex := make(map[int32]int, len(records))
	for index, record := range records {
		byIndex[record.Index] = index
	}
	used := make(map[int32]uint8)
	for _, record := range records {
		if source, exists := byIndex[record.InputObjIdx]; surfaceIsBelt(record.ItemID) && exists && !surfaceIsBelt(records[source].ItemID) {
			if record.InputToSlot != 1 {
				return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: fmt.Sprintf("传送带%d设施输入必须使用主输入槽1", record.Index)}
			}
			used[record.Index] |= 1
		}
		if target, exists := byIndex[record.OutputObjIdx]; !surfaceIsBelt(record.ItemID) && exists && surfaceIsBelt(records[target].ItemID) && record.OutputToSlot >= 1 && record.OutputToSlot <= 3 {
			used[record.OutputObjIdx] |= 1 << (record.OutputToSlot - 1)
		}
	}
	assignments := make(map[int]int8)
	for index, record := range records {
		target, exists := byIndex[record.OutputObjIdx]
		if !surfaceIsBelt(record.ItemID) || !exists || !surfaceIsBelt(records[target].ItemID) {
			continue
		}
		slot := int8(1)
		for slot <= 3 && used[record.OutputObjIdx]&(1<<(slot-1)) != 0 {
			slot++
		}
		if slot > 3 {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: fmt.Sprintf("传送带%d的实体汇入超过三个独立输入槽，不能覆盖已有支线", record.OutputObjIdx)}
		}
		used[record.OutputObjIdx] |= 1 << (slot - 1)
		assignments[index] = slot
	}
	for index, slot := range assignments {
		records[index].OutputToSlot = slot
	}
	return nil
}

// connectionPoolPort 职责：标识一个建筑连接池槽；使用：粘贴写入重放；边界：每个建筑最多16槽，-1仅允许对端自动分配。
type connectionPoolPort struct {
	building int32
	slot     int8
}

// connectionPoolLink 职责：保留一次连接写入的两端及方向；使用：所有写入完成后检查是否被后续写入覆盖；边界：双向重复声明允许，真正的端口争用必须失败。
type connectionPoolLink struct {
	from, to connectionPoolPort
	output   bool
}

// connectionPoolValue 职责：保留实际连接对端及输入/输出方向；使用：重放后的双向一致性核对；边界：同样两端但相反方向不能互相冒充存续。
type connectionPoolValue struct {
	other  connectionPoolPort
	output bool
}

// validatePastedConnections 职责：按游戏粘贴的逐记录输出/输入顺序重放独占槽写入；使用：最终编码前；边界：只模拟连接池清除/回写和动态4..11分拣器槽，不声称已运行Unity或验证物料时序。
func validatePastedConnections(ctx context.Context, records []blueprint.Building) error {
	byIndex := make(map[int32]blueprint.Building, len(records))
	for _, record := range records {
		byIndex[record.Index] = record
	}
	pool := make(map[connectionPoolPort]connectionPoolValue)
	expected := make([]connectionPoolLink, 0, len(records))
	clear := func(port connectionPoolPort) {
		if other, exists := pool[port]; exists {
			delete(pool, port)
			delete(pool, other.other)
		}
	}
	write := func(source int32, slot int8, output bool, target int32, otherSlot int8) error {
		other, exists := byIndex[target]
		if !exists || target == source || slot < 0 || slot > 15 || otherSlot < -1 || otherSlot > 15 {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: fmt.Sprintf("建筑%d槽%d到建筑%d槽%d的游戏连接池引用无效", source, slot, target, otherSlot)}
		}
		if otherSlot == -1 {
			if !surfaceIsBelt(other.ItemID) {
				return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "动态分拣器连接槽只能用于传送带"}
			}
			for candidate := int8(4); candidate < 12; candidate++ {
				if _, occupied := pool[connectionPoolPort{target, candidate}]; !occupied {
					otherSlot = candidate
					break
				}
			}
			if otherSlot == -1 {
				return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: fmt.Sprintf("传送带%d的动态分拣器连接槽4..11已用尽", target)}
			}
		}
		from, to := connectionPoolPort{source, slot}, connectionPoolPort{target, otherSlot}
		clear(from)
		clear(to)
		pool[from], pool[to] = connectionPoolValue{to, output}, connectionPoolValue{from, !output}
		expected = append(expected, connectionPoolLink{from, to, output})
		return nil
	}
	for _, record := range records {
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "游戏传送带连接池核验已取消或超时"}
		}
		if record.OutputObjIdx >= 0 {
			if surfaceIsBelt(record.ItemID) && record.OutputFromSlot != 0 {
				return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: fmt.Sprintf("传送带%d必须从输出槽0出货", record.Index)}
			}
			if other, exists := byIndex[record.OutputObjIdx]; surfaceIsBelt(record.ItemID) && exists && surfaceIsBelt(other.ItemID) && (record.OutputToSlot < 1 || record.OutputToSlot > 3) {
				return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: fmt.Sprintf("传送带%d汇入带%d必须使用输入槽1..3", record.Index, record.OutputObjIdx)}
			}
			if err := write(record.Index, record.OutputFromSlot, true, record.OutputObjIdx, record.OutputToSlot); err != nil {
				return err
			}
		}
		if record.InputObjIdx >= 0 {
			// 游戏预览会清除带到带的临时input，只由上游output写入实际连接。
			if other, exists := byIndex[record.InputObjIdx]; surfaceIsBelt(record.ItemID) && exists && surfaceIsBelt(other.ItemID) {
				continue
			}
			if err := write(record.Index, record.InputToSlot, false, record.InputObjIdx, record.InputFromSlot); err != nil {
				return err
			}
		}
	}
	for _, link := range expected {
		if other, exists := pool[link.from]; !exists || other != (connectionPoolValue{link.to, link.output}) || pool[link.to] != (connectionPoolValue{link.from, !link.output}) {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: fmt.Sprintf("建筑%d槽%d与建筑%d槽%d的连接会被游戏后续写入覆盖，存在实际断带", link.from.building, link.from.slot, link.to.building, link.to.slot)}
		}
	}
	return nil
}
