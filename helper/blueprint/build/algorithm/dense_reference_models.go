package build

// 文件职责：按原版实际建造盒和端口几何证明不同等级可复用参考姿态。
// 使用：参考密铺装配前；边界：明确列举原型并逐次核对数值，不因同属机器类型就假定模型一致。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/blueprint/density"
	"math"
)

// referenceLineSpacing 职责：按参考行的实际切平面周期设置重复节距；使用：已核对模型的低层轨道单元；边界：保留少量数值余量及用户额外间距，仍需最终球面终检。
func referenceLineSpacing(kind, extra int) (float64, error) {
	pitch, ok := map[int]float64{1: 2.45, 2: 3.15, 4: 6.68}[kind]
	if !ok {
		return 0, &blueprint.Error{Code: "GEN_E_DENSITY_PROFILE", Message: "该设备缺少参考重复周期"}
	}
	return pitch + float64(extra), nil
}

// denseReferenceSource 职责：选择待验证的参考原型；使用：能力选择；边界：仅白名单机型，最终仍须 validateReferencePrototype。
func denseReferenceSource(item int) int {
	switch item {
	case 2302, 2315, 2319:
		return 2319
	case 2303, 2304, 2305, 2318:
		return 2318
	case 2309, 2317:
		return 2317
	default:
		return 0
	}
}

// validateReferencePrototype 职责：核对所选模型和来源模型的实际建造盒及分拣槽位；使用：替换机器等级前；边界：只有float32舍入量级的差异可接受，不修改原始碰撞体。
func (b *builder) validateReferencePrototype(item int, unit density.ReferenceUnit) error {
	fail := func() error {
		return &blueprint.Error{Code: "GEN_E_DENSITY_PROFILE", Message: "所选设备的建造盒或端口与密铺参考不匹配"}
	}
	model, ok := b.models[itoa(item)]
	if !ok || denseReferenceSource(item) != unit.MachineItemID {
		return fail()
	}
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return err
	}
	a, err := catalog.Shapes(int16(item), model)
	if err != nil {
		return err
	}
	c, err := catalog.Shapes(int16(unit.MachineItemID), unit.ModelIndex)
	if err != nil {
		return err
	}
	if len(a) != len(c) {
		return fail()
	}
	equal := func(x, y float32) bool { return math.Abs(float64(x-y)) <= 1e-6 }
	for i := range a {
		for j := 0; j < 3; j++ {
			if !equal(a[i].Position[j], c[i].Position[j]) || !equal(a[i].Extents[j], c[i].Extents[j]) {
				return fail()
			}
		}
		for j := 0; j < 4; j++ {
			if !equal(a[i].Rotation[j], c[i].Rotation[j]) {
				return fail()
			}
		}
	}
	g, err := catalog.Geometry(int16(item), model)
	if err != nil {
		return err
	}
	h, err := catalog.Geometry(int16(unit.MachineItemID), unit.ModelIndex)
	if err != nil {
		return err
	}
	if len(g.SlotPoses) != len(h.SlotPoses) {
		return fail()
	}
	for i := range g.SlotPoses {
		for j := 0; j < 3; j++ {
			if !equal(g.SlotPoses[i].Position[j], h.SlotPoses[i].Position[j]) || !equal(g.SlotPoses[i].LocalScale[j], h.SlotPoses[i].LocalScale[j]) {
				return fail()
			}
		}
		for j := 0; j < 4; j++ {
			if !equal(g.SlotPoses[i].Rotation[j], h.SlotPoses[i].Rotation[j]) {
				return fail()
			}
		}
	}
	return nil
}
