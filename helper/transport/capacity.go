package transport

// 文件职责：统一原版基础传送带每分钟额定容量和均匀拆带；使用：所有蓝图模式、预检和诊断；边界：不推测科技、堆叠或 Mod 倍率，不静默升级等级。

import (
	"fmt"
	"math"
)

// BeltCapacityPerMinute 职责：提供选定等级单条独立带的容量；使用：物流规划唯一容量入口；边界：仅接受 1..3，结果为 360/720/1800 件每分钟。
func BeltCapacityPerMinute(level int) (float64, error) {
	switch level {
	case 1:
		return 360, nil
	case 2:
		return 720, nil
	case 3:
		return 1800, nil
	default:
		return 0, fmt.Errorf("传送带等级 %d 无效，支持 1 到 3", level)
	}
}

// SplitLanes 职责：按所选等级把计划流量均分为独立 lane；使用：自动多入口下界；边界：实际端口能力可能要求更多 lane，拒绝负数、非有限值和超出有界规划的规模。
func SplitLanes(rate float64, level int) ([]float64, error) {
	capacity, err := BeltCapacityPerMinute(level)
	if err != nil {
		return nil, err
	}
	if rate < 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return nil, fmt.Errorf("物料流量必须为有限非负数")
	}
	if rate == 0 {
		return []float64{}, nil
	}
	count := math.Ceil(rate / capacity)
	if count > 100000 {
		return nil, fmt.Errorf("自动 lane 数 %.0f 超过 100000 上限，请拆分目标", count)
	}
	lanes := make([]float64, int(count))
	for i := range lanes {
		lanes[i] = rate / count
	}
	return lanes, nil
}
