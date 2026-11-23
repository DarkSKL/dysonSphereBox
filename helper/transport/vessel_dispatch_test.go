package transport

// 文件职责：以原版 CalcTripEnergyCost 的逐焦耳读数回归真实两条航线和曲速边界。

import (
	"math"
	"testing"
)

func TestVesselDispatchGameEnergy(t *testing.T) {
	profile := VesselProfile{Cargo: 600, CruiseSpeed: 1200, WarpSpeed: 240000, WarpDistance: 120000, TurnaroundSeconds: 10}
	for _, tc := range []struct {
		name               string
		distance, from, to float64
		joules             int64
	}{
		{"1504至2801高能石墨", 64563184.748485155, 200, 200, 2282907542},
		{"1203至1202钛石", 133916.91091346936, 200, 200, 350029507},
		{"等于曲速门槛不收附加费", 119600, 200, 200, 249600000},
		{"气态星真实半径跨过门槛", 119001, 800, 200, 349600030},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, energy, err := profile.VesselDispatch(tc.distance, tc.from, tc.to)
			if err != nil || int64(math.Round(energy*1e6)) != tc.joules {
				t.Fatalf("每次派遣 %.9f MJ，期望 %d J：%v", energy, tc.joules, err)
			}
		})
	}
	for _, r := range []float64{0, -200, math.NaN(), math.Inf(1)} {
		if _, _, _, err := profile.VesselDispatch(3000, r, 200); err == nil {
			t.Fatalf("接受了无效运输星半径 %g", r)
		}
	}
}
