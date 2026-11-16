package build

// 文件职责：按配送箱真实臂数和带宽共同拆分密铺行，并度量实际设施接驳。
// 使用：模式2独立设施装配；边界：不升级设备、不少生成机器，不以普通端点的欠能力告警作为密铺成功。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"math"
)

// denseFlyLineCounts 职责：在机器/传送带拆行基础上加入箱体真实槽位上限；使用：所有密铺配送产线；边界：自产剂按毛流检查并保留独立回料槽，整数余量不丢弃。
func denseFlyLineCounts(p quant.Production, recipe quant.Recipe, setting Setting, speed float64) ([]int, error) {
	counts, err := layoutLineCounts(p, recipe, setting, speed, densityDense)
	if err != nil {
		return nil, err
	}
	maximum := 0.
	for _, rates := range [][]quant.ItemRate{p.Inputs, p.Outputs} {
		for _, rate := range rates {
			maximum = math.Max(maximum, rate.RatePerMinute)
		}
	}
	if p.Capacity != nil && p.Capacity.OutputPerMinute > 0 {
		scale := p.OutputCount / p.Capacity.OutputPerMinute
		for _, rates := range [][]quant.ItemRate{p.Capacity.GrossInputs, p.Capacity.GrossOutputs} {
			for _, rate := range rates {
				maximum = math.Max(maximum, rate.RatePerMinute*scale)
			}
		}
	}
	if maximum <= 0 || p.MachineCount <= 0 {
		return counts, nil
	}
	slots := 3
	item := productionSprayItem(p)
	if item > 0 && productionItemRate(p.Inputs, item) == 0 && productionItemRate(p.Outputs, item) > 0 {
		slots = 2
	}
	capacity := float64(slots) * inserterCapacityPerMinute(setting.InserterLv, 1)
	limit := int(math.Floor((capacity + 1e-6) * float64(p.MachineCount) / maximum))
	if limit < 1 {
		return nil, &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: "单台机器需要的配送流量超过所选等级箱体可用独立槽位能力"}
	}
	out := []int{}
	for _, count := range counts {
		for count > 0 {
			part := min(count, limit)
			out = append(out, part)
			count -= part
		}
	}
	return out, nil
}

// denseFlyLocalBeltLength 职责：按唯一真实带边统计机器到设施的接驳长度；使用：密铺候选比较；边界：不计远程机队航线或把重复规范份额重复记为实体带。
func (b *builder) denseFlyLocalBeltLength() float64 {
	seen := map[*placed]bool{}
	length := 0.
	for _, endpoint := range b.flyEndpoints {
		members := map[*placed]bool{endpoint.link: true}
		for _, belt := range endpoint.belts {
			members[belt] = true
		}
		for _, belt := range append(append([]*placed(nil), endpoint.belts...), endpoint.link) {
			if seen[belt] || !members[belt.output] {
				continue
			}
			seen[belt] = true
			dx, dy, dz := belt.a.x-belt.output.a.x, belt.a.y-belt.output.a.y, belt.a.z-belt.output.a.z
			length += math.Sqrt(dx*dx + dy*dy + dz*dz)
		}
	}
	return length
}
