package generator

// 文件职责：显式登记模式和地理的只读属性白名单。
// 使用：YAML控制结构与共享枚举工具；边界：没有反射字段路径、策略选择或候选执行。

import (
	core "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/workflow"
)

// modeProviders 职责：暴露有明确来源和类型的参数及地理事实；使用：主registry追加；边界：请求事实读取运输输入，不读取尚未存在的球面候选。
func modeProviders(a artifacts) []workflow.StateProvider {
	key := a.modeStages.effective
	return []workflow.StateProvider{
		inputProvider("mode.density", a.transport, func(v *core.PreparedInput) (string, error) { return v.Density(), nil }),
		inputProvider("mode.family", a.transport, func(v *core.PreparedInput) (string, error) {
			switch v.Mode() {
			case 1:
				return "connected", nil
			case 2, 5:
				return "distributor", nil
			case 3, 4:
				return "station", nil
			case 6:
				return "bus", nil
			}
			return "unsupported", nil
		}),
		inputProvider("mode.geo.class", a.transport, func(v *core.PreparedInput) (string, error) {
			facts, err := v.ModeGeography()
			if err != nil {
				return "", err
			}
			return facts.Class, nil
		}),
		inputProvider("mode.geo.hemisphere", a.transport, func(v *core.PreparedInput) (string, error) {
			facts, err := v.ModeGeography()
			if err != nil {
				return "", err
			}
			return facts.Hemisphere, nil
		}),
		inputProvider("mode.geo.longitude-boundary", a.transport, func(v *core.PreparedInput) (string, error) {
			facts, err := v.ModeGeography()
			if err != nil {
				return "", err
			}
			return facts.LongitudeBoundary, nil
		}),
		inputProvider("mode.geo.cross-bands", a.transport, func(v *core.PreparedInput) (bool, error) {
			facts, err := v.ModeGeography()
			if err != nil {
				return false, err
			}
			return facts.CrossBands, nil
		}),
		modeProvider("mode.polar-window-rows", key, func(p modePatch) []int { return p.PolarWindowRows }),
		modeProvider("mode.row-spans", key, func(p modePatch) []float64 { return p.RowSpans }),
		modeProvider("mode.street-groups", key, func(p modePatch) []int { return p.StreetGroups }),
		modeProvider("mode.street-rotations", key, func(p modePatch) []int { return p.StreetRotations }),
		modeProvider("mode.dense-orders", key, func(p modePatch) []int { return p.DenseOrders }),
		modeProvider("mode.bent-spans", key, func(p modePatch) []float64 { return p.BentSpans }),
		modeProvider("mode.station-family", key, func(p modePatch) string { return *p.StationFamily }),
		modeProvider("mode.bus-tighten", key, func(p modePatch) bool { return *p.BusTighten }),
	}
}
