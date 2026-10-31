package transport

// 文件职责：锁定独立电力/物流塔数值目录；使用：来源与字段回归；边界：不执行 Unity，也不把数值验证冒充电力或物流运行测试。

import (
	"encoding/json"
	"testing"
)

// TestInfrastructureVerifiedFacts 职责：核对节点半径、塔槽位与端口数；使用：事实目录升级审查；边界：未知构建不能仅修改 JSON 后继续沿用旧测试。
func TestInfrastructureVerifiedFacts(t *testing.T) {
	var catalog struct {
		Schema string `json:"schema"`
		Source struct {
			SteamBuildID    string `json:"steamBuildId"`
			GameVersion     string `json:"gameVersion"`
			AssemblySHA256  string `json:"assemblySha256"`
			ResourcesSHA256 string `json:"resourcesSha256"`
		} `json:"source"`
		PowerNodes []struct {
			ItemID          int     `json:"itemId"`
			Node            bool    `json:"node"`
			ConnectDistance float64 `json:"connectDistance"`
			CoverRadius     float64 `json:"coverRadius"`
		} `json:"powerNodes"`
		Stations []struct {
			ItemID        int `json:"itemId"`
			MaxItemKinds  int `json:"maxItemKinds"`
			MaxItemCount  int `json:"maxItemCount"`
			MaxDroneCount int `json:"maxDroneCount"`
			MaxShipCount  int `json:"maxShipCount"`
			PortCount     int `json:"portCount"`
		} `json:"stations"`
	}
	if err := json.Unmarshal(infrastructureFactsData, &catalog); err != nil {
		t.Fatal(err)
	}
	profile, err := InserterCapacityProfile()
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Schema != "dsp-infrastructure-facts/v1" || catalog.Source.SteamBuildID != profile.SteamBuildID || catalog.Source.GameVersion != profile.GameVersion || catalog.Source.AssemblySHA256 != profile.AssemblySHA256 || catalog.Source.ResourcesSHA256 != profile.ResourcesSHA256 {
		t.Fatalf("infrastructure source mismatch: %+v", catalog.Source)
	}
	if len(catalog.PowerNodes) != 1 || catalog.PowerNodes[0].ItemID != 2201 || !catalog.PowerNodes[0].Node || catalog.PowerNodes[0].ConnectDistance != 22.5 || catalog.PowerNodes[0].CoverRadius != 10.5 {
		t.Fatalf("power facts mismatch: %+v", catalog.PowerNodes)
	}
	if len(catalog.Stations) != 2 {
		t.Fatalf("station facts missing: %+v", catalog.Stations)
	}
	for index, want := range [][6]int{{2103, 4, 5000, 50, 0, 12}, {2104, 5, 10000, 100, 10, 12}} {
		fact := catalog.Stations[index]
		if got := [6]int{fact.ItemID, fact.MaxItemKinds, fact.MaxItemCount, fact.MaxDroneCount, fact.MaxShipCount, fact.PortCount}; got != want {
			t.Fatalf("station facts got %v want %v", got, want)
		}
		limit, err := StationItemKindLimit(want[0])
		if err != nil || limit != want[1] {
			t.Fatalf("station %d item limit %d: %v", want[0], limit, err)
		}
		ports, err := StationPortCount(want[0])
		if err != nil || ports != want[5] {
			t.Fatalf("station %d port count %d: %v", want[0], ports, err)
		}
		capacity, err := StationItemCapacity(want[0])
		if err != nil || capacity != want[2] {
			t.Fatalf("station %d item capacity %d: %v", want[0], capacity, err)
		}
	}
	cover, err := TeslaPowerCoverRadius()
	if err != nil || cover != 10.5 {
		t.Fatalf("Tesla cover radius %g: %v", cover, err)
	}
	connect, err := TeslaPowerConnectDistance()
	if err != nil || connect != 22.5 {
		t.Fatalf("Tesla connect distance %g: %v", connect, err)
	}
	for _, itemID := range []int{0, 2105, 999999} {
		if _, err := StationItemKindLimit(itemID); err == nil {
			t.Fatalf("unverified station kind limit %d accepted", itemID)
		}
		if _, err := StationPortCount(itemID); err == nil {
			t.Fatalf("unverified station port count %d accepted", itemID)
		}
		if _, err := StationItemCapacity(itemID); err == nil {
			t.Fatalf("unverified station item capacity %d accepted", itemID)
		}
	}
}
