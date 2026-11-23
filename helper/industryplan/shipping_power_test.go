package industryplan

// 文件职责：验证没有制造或采矿的发货星也必须支付每一次满载派遣的用电。

import (
	"math"
	"strings"
	"testing"
)

func TestPureSenderPaysEnergyOnce(t *testing.T) {
	for _, tc := range []struct {
		name      string
		enable    bool
		choice    EnergyChoice
		wantShort bool
	}{
		{"本地风电", true, EnergyChoice{Mode: "fixed", FacilityID: 2203}, false},
		{"已有电网足够", true, EnergyChoice{Mode: "grid", GridCapacityMW: 183}, false},
		{"已有电网不足", true, EnergyChoice{Mode: "grid", GridCapacityMW: 180}, true},
		{"玩家外接", false, EnergyChoice{Mode: "auto"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := energyCandidateForTest(t, Request{EnablePower: tc.enable, Energy: EnergySettings{Default: tc.choice}, Logistics: LogisticsSettings{CargoPerVessel: 600, CruiseSpeed: 1200, WarpSpeed: 240000, WarpDistance: 120000}}, map[int64]float64{1504: 0, 2801: 0})
			from, to := c.world[1504], c.world[2801]
			from.planet.Position = [3]float64{}
			to.planet.Position = [3]float64{64563184.748485155}
			const rate = 2871.530749789646
			const want = 182.09553349388048
			if got := c.transfer(from, to, 1109, rate); got != rate {
				t.Fatal(got)
			}
			r := c.result.Routes[0]
			if r.VesselEnergyMJ != 2282.907542 || math.Abs(r.VesselPowerMW-want) > 1e-9 || math.Abs(from.plan.OutboundVesselMW-want) > 1e-9 || to.plan.OutboundVesselMW != 0 {
				t.Fatalf("发船电费错误：%+v", r)
			}
			err := c.planPower()
			if tc.wantShort {
				if err == nil || !strings.Contains(err.Error(), "电网余量不足") {
					t.Fatalf("未拒绝180 MW已有电网：%v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.enable && (from.plan.Power == nil || math.Abs(from.plan.Power.RequiredMW-want) > 1e-9 || from.plan.Power.PowerMW+from.plan.Power.ExternalGridMW < want) {
				t.Fatalf("发货星电源合同 %+v", from.plan.Power)
			}
			if !tc.enable {
				if len(c.result.External) != 1 || c.result.External[0].Kind != "grid-power" || math.Abs(c.result.External[0].PowerMW-want) > 1e-9 {
					t.Fatalf("外接电网回执 %+v", c.result.External)
				}
			}
			if to.plan.Power != nil {
				t.Fatal("收货端重复承担发船扣电")
			}
		})
	}
}
