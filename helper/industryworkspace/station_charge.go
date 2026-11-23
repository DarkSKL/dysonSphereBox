package industryworkspace

// 文件职责：用每颗星已冻结的施工图更新充电条件，保留同星其他物料、电源及机队条件。

import (
	"slices"

	"DysonSphereBox/helper/industryplan"
)

// recordRequiredGrid 用实装后仍需外接的持续功率替换原分工估值，包括只有采集或收货塔的星球。
func (b *Bundle) recordRequiredGrid(id int64, mw float64) {
	b.Report.External = slices.DeleteFunc(b.Report.External, func(e industryplan.External) bool { return e.PlanetID == id && e.Kind == "grid-power" })
	b.Report.External = append(b.Report.External, industryplan.External{PlanetID: id, Kind: "grid-power", PowerMW: mw})
	if b.Report.Status == "ready" {
		b.Report.Status = "conditional"
	}
}

// recordStationCharging 将分工阶段预估替换为实际设置，覆盖制造块、采矿、收发、能源与舰队模块内的全部物流塔。
func (b *Bundle) recordStationCharging(site *PlanetSheet, peakMW float64) {
	const needsGrid = "本星蓝图没有可供物流塔使用的发电容量。请先接入可用电网，再为物流塔充电。实际平均耗电取决于发船、配送和充电设定。"
	site.Warnings = slices.DeleteFunc(site.Warnings, func(s string) bool { return s == needsGrid })
	external := slices.DeleteFunc(b.Report.External, func(e industryplan.External) bool {
		return e.PlanetID == site.ID && e.Kind == "charging-peak"
	})
	if peakMW > 0 {
		external = append(external, industryplan.External{PlanetID: site.ID, Kind: "charging-peak", PowerMW: peakMW})
		if site.Power == nil || site.Power.PowerMW <= 0 {
			site.Warnings = slices.DeleteFunc(site.Warnings, func(s string) bool { return s == "当前规划需要玩家接入可用电网" })
			site.Warnings = append(site.Warnings, needsGrid)
		}
		if b.Report.Status == "ready" {
			b.Report.Status = "conditional"
		}
	}
	b.Report.External = external
	b.Plan.External = external
}
