package saveanalysis

import (
	"math"
	"os"
	"testing"
)

// TestSavedGroundZones 职责：使用原版导入对照快照位置；使用：本机验收；边界：仅在显式指定研究样本时运行。
func TestSavedGroundZones(t *testing.T) {
	path := os.Getenv("DSP_RELAY_SAMPLE")
	if path == "" {
		t.Skip("未指定原版中继器对照存档")
	}
	s, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.tick != 7174748 {
		t.Fatalf("不是用于原版对照的存档时刻：%d", s.tick)
	}
	if len(s.GroundBaseZones(301)) != 0 || len(s.GroundBaseZones(302)) != 0 {
		t.Fatal("无相关中继器的星球获得计划区域")
	}
	zones := s.VirtualRelayZones(401)
	if len(zones) != 5 {
		t.Fatalf("401 需要原版记录的五个虚拟落点：%+v", zones)
	}
	known := map[int64]bool{1: true, 2: true, 4: true, 5: true, 7: true}
	for _, z := range zones {
		p := z.Position
		radius := math.Sqrt(p.X*p.X + p.Y*p.Y + p.Z*p.Z)
		if math.Abs(radius-200.2) > 1e-4 || z.Radius != 40 || z.Depth != 60 || z.Realized || !known[z.RelayID] || z.HiveAstroID != 1000025 {
			t.Fatalf("虚拟落点投影或身份错误：%+v", z)
		}
		delete(known, z.RelayID)
	}
	one := zones[0]
	if math.Abs(one.Position.X-116.2945) > 0.02 || math.Abs(one.Position.Y+143.2998) > 0.02 || math.Abs(one.Position.Z-77.591) > 0.02 {
		t.Fatalf("保存坐标方向未保留：%+v", one)
	}
	zones[0].Depth = 0
	if s.VirtualRelayZones(401)[0].Depth != 60 {
		t.Fatal("修改返回值影响快照")
	}
	bases := s.GroundBaseZones(104)
	t.Logf("104 地面主基地：%+v", bases)
	found := false
	for _, base := range bases {
		if base.Realized && base.BaseID == 1 && base.HiveAstroID == 1000001 && base.RelayID == 7 {
			found = true
			if base.EnemyID == 0 || !base.Localized || base.Radius != 11.25 || base.Depth != 0 {
				t.Fatalf("本地化后的实体基地仅保留真实主基地范围：%+v", base)
			}
			if base.Position.X != 118.96903228759766 || base.Position.Y != -10.030765533447266 || base.Position.Z != 160.70408630371094 {
				t.Fatalf("存档实体坐标与原版 EnemyData.Import 不符：%+v", base)
			}
		}
	}
	if !found {
		t.Fatal("104 原版实体基地未按厂区保存坐标定位")
	}
	actual, localized, virtual := 0, 0, 0
	for _, planet := range s.World().Planets {
		for _, zone := range s.GroundBaseZones(planet.PlanetID) {
			if zone.Realized {
				actual++
				if zone.Localized {
					localized++
				} else if zone.EnemyID != 870 || zone.BaseID != 12 ||
					zone.PlanetID != 6301 || zone.Radius != 40 || zone.Depth != 60 {
					t.Fatalf("6301 待本地化实体地面基地字段错误：%+v", zone)
				}
			} else {
				virtual++
			}
		}
	}
	if actual != 34 || localized != 33 || virtual != 382 {
		t.Fatalf("与原版验证不符：已实体化 %d，已本地化 %d，虚拟 %d", actual, localized, virtual)
	}
	t.Logf("已实体化 %d，已本地化 %d，虚拟 %d", actual, localized, virtual)
}
