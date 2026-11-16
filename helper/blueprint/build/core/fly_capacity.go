package build

// 文件职责：为配送箱体及产物过滤支路规划有界真实并联分拣器。
// 使用：mode2/5行末装配；边界：固定所选等级、不增加计划流量，超过有限端口能力仍保留结构并披露未验证。

import (
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/transport"
)

// flyStorageArmPlan 职责：保存箱体西侧真实槽位与带侧跨度；使用：箱体接驳尾带；边界：最多使用2101的9/10/11三个独立槽位。
type flyStorageArmPlan struct {
	slots []int
	reach int
}

// flyParallelArmCount 职责：计算有限候选内应创建的真实臂数；使用：箱体和过滤并联规划；边界：达到上限不伪装覆盖，实际实体能力仍由完整性阶段独立重算。
func flyParallelArmCount(rate float64, level, reach, limit int) (int, error) {
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 || limit < 1 {
		return 0, &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: "配送并联分拣器计划流量或候选上限无效"}
	}
	capacity, err := transport.InserterCapacityPerMinute(level, reach)
	if err != nil {
		return 0, err
	}
	needed := math.Ceil(math.Max(0, rate-1e-6) / capacity)
	return int(math.Min(float64(limit), math.Max(1, needed))), nil
}

// planFlyStorageArms 职责：按流量选择西侧1..3个槽外一格的所选等级臂；使用：生产机器行的配送箱体；边界：零流量纯几何调用保留旧槽10跨2格基线，不影响既有独立调用。
func planFlyStorageArms(rate float64, level int) (flyStorageArmPlan, error) {
	reach := 1
	if rate == 0 {
		reach = 2
	}
	count, err := flyParallelArmCount(rate, level, reach, 3)
	if err != nil {
		return flyStorageArmPlan{}, err
	}
	slots := []int{10}
	if count > 1 {
		slots = []int{9, 10, 11}[:count]
	}
	return flyStorageArmPlan{slots: slots, reach: reach}, nil
}

// beltPoints 职责：使同一真实有向尾带依次通过全部西侧臂；使用：装配前路径占位检查；边界：箱体朝向固定0，不跨高度或从同坐标推导连接。
func (plan flyStorageArmPlan) beltPoints(center vector) []vector {
	points := make([]vector, len(plan.slots))
	for i, slot := range plan.slots {
		point, _ := storageSlotPoint(center, 0, slot)
		points[i] = vector{point.x - float64(plan.reach), point.y, center.z}
	}
	return points
}

// flyOutputFilters 职责：在产物专属六格干线旁创建逐格并联过滤臂并合流到同一支路；使用：两种配送模式的产物端点；边界：只接所选等级真实1格臂，超过六臂能力由诊断告警，不覆盖干线output引用。
func (b *builder) flyOutputFilters(trunk []*placed, item int, rate float64) (*placed, []*placed, error) {
	count, err := flyParallelArmCount(rate, b.setting.InserterLv, 1, len(trunk))
	if err != nil {
		return nil, nil, err
	}
	points := make([]vector, count)
	for i, source := range trunk[len(trunk)-count:] {
		points[i] = vector{source.a.x, source.a.y + 1, source.a.z}
	}
	branch := b.byPath(points)
	filters := make([]*placed, count)
	for i, source := range trunk[len(trunk)-count:] {
		filters[i], err = b.beltTransfer(source, branch[i], b.setting.InserterLv, item)
		if err != nil {
			return nil, nil, err
		}
	}
	return branch[len(branch)-1], filters, nil
}
