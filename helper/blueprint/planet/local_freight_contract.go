package planet

// 文件职责：把同星的开采、制造、电源及进出口合同折成无人机货量上界，只核对新建且与原有库存隔离的产线。

import "DysonSphereBox/helper/industryplan"

type localCargoLimit struct {
	made, used, mined, fuel, incoming, outgoing, stored float64
	manual                                              bool
}

// localContractLimits 每件物品从本地供给到本地需求只记一次。回流到制造端后重新产出的部分已经计在制造毛流中。
func localContractLimits(req *Request, plan industryplan.PlanetPlan, actual *industryplan.PlanetActual) map[int]*localCargoLimit {
	limits := map[int]*localCargoLimit{}
	item := func(id int) *localCargoLimit {
		if limits[id] == nil {
			limits[id] = &localCargoLimit{}
		}
		return limits[id]
	}
	for _, p := range plan.Plan.Productions {
		for _, q := range p.Inputs {
			item(q.ItemID).used += q.RatePerMinute
		}
		for _, q := range p.Outputs {
			item(q.ItemID).made += q.RatePerMinute
		}
	}
	for _, p := range plan.Ports {
		v := item(p.ItemID)
		if p.Kind == "external" {
			v.manual = true
		}
		switch p.Kind {
		case "import", "external":
			v.incoming += p.RatePerMinute
		case "export", "external-export":
			v.outgoing += p.RatePerMinute
		case "storage", "delivery":
			v.stored += p.RatePerMinute
		}
	}
	for _, m := range req.mineCoverage {
		item(m.ItemID).mined += m.RatePerMinute
	}
	for _, p := range actual.Ports {
		v := item(p.ItemID)
		switch p.Kind {
		case "mining":
			v.mined = max(v.mined, p.RatePerMinute)
		case "power-fuel":
			v.fuel = max(v.fuel, p.RatePerMinute)
		}
	}
	return limits
}
