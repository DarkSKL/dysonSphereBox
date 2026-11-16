package build

// 文件职责：预计算来源单元的记录索引、机器臂分组及货物采样；使用：所有模式的来源型机器行；边界：只共享固定参考数据，装配实体和参数切片仍由候选独立创建。

import (
	"fmt"
	"sync"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/density"
)

// referenceTemplate 职责：保存来源图在其固定原点的只读解析结果；使用：重复机器与运输单元；边界：不能存入目标行星坐标或请求相关净空。
type referenceTemplate struct {
	unit         density.ReferenceUnit
	byIndex      map[int32]blueprint.Building
	arms         map[int32][]blueprint.Building
	frame        referenceProjector
	cargoTargets map[int32]vector
}

// referenceTemplates 职责：为有限且版本锁定的来源建立独立一次缓存；使用：按来源原型查询；边界：不按任意请求键增长，运行中数据版本不会变化。
var referenceTemplates = func() map[int]func() (*referenceTemplate, error) {
	loaders := make(map[int]func() (*referenceTemplate, error), 4)
	for _, source := range []int{2319, 2318, 2317, 2308} {
		loaders[source] = sync.OnceValues(func() (*referenceTemplate, error) {
			var unit density.ReferenceUnit
			var err error
			if source == 2308 {
				unit, err = density.RefineryReference()
			} else {
				var ok bool
				unit, _, ok, err = density.Reference(source)
				if err == nil && !ok {
					err = fmt.Errorf("缺少原型%d的参考单元", source)
				}
			}
			if err != nil {
				return nil, err
			}
			return compileReferenceTemplate(unit)
		})
	}
	return loaders
}()

// loadReferenceTemplate 职责：提供内嵌来源的同一预计算结果；使用：双排低轨和炼油同侧轨；边界：未知来源明确拒绝，候选不能改写返回内容。
func loadReferenceTemplate(source int) (*referenceTemplate, error) {
	loader, ok := referenceTemplates[source]
	if !ok {
		return nil, &blueprint.Error{Code: "GEN_E_DENSITY_PROFILE", Message: fmt.Sprintf("缺少原型%d的参考单元", source)}
	}
	return loader()
}

// compileReferenceTemplate 职责：把固定来源编译为索引和局部采样事实；使用：首次加载；边界：保留原记录顺序及严格货物规则，不对不同目标纬带作可行性承诺。
func compileReferenceTemplate(unit density.ReferenceUnit) (*referenceTemplate, error) {
	template := &referenceTemplate{unit: unit, byIndex: make(map[int32]blueprint.Building, len(unit.Records)), arms: map[int32][]blueprint.Building{}}
	for _, record := range unit.Records {
		template.byIndex[record.Index] = record
	}
	if len(unit.MachineIndices) == 0 {
		return nil, fmt.Errorf("参考单元缺少机器")
	}
	first, ok := template.byIndex[unit.MachineIndices[0]]
	if !ok {
		return nil, fmt.Errorf("参考机器编号不存在")
	}
	template.frame = newReferenceProjector(first.LocalOffset)
	for _, machine := range unit.MachineIndices {
		for _, arm := range unit.Records {
			if arm.ItemID >= 2011 && arm.ItemID <= 2014 && (arm.InputObjIdx == machine || arm.OutputObjIdx == machine) {
				template.arms[machine] = append(template.arms[machine], arm)
			}
		}
	}
	var err error
	template.cargoTargets, err = referenceCargoTargets(unit, template.frame)
	if err != nil {
		return nil, err
	}
	return template, nil
}
