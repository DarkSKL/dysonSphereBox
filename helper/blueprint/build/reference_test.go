package build

// 文件职责：对拍独立 Node 参考产线的几何及连接端口；使用：DSBOX_LAYOUT_GOLDEN；边界：不代替游戏实测。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"sort"
	"testing"
)

// TestReferenceLines 职责：忽略数组顺序后逐建筑对比；使用：六类机器 mode1/2/5；边界：参考解析器截四位，因此几何按三位比较。
func TestReferenceLines(t *testing.T) {
	path := os.Getenv("DSBOX_LAYOUT_GOLDEN")
	if path == "" {
		t.Skip("设置 DSBOX_LAYOUT_GOLDEN 运行参考几何对拍")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Production quant.Production `json:"production"`
		Mode       int              `json:"mode"`
		Blueprint  string           `json:"blueprint"`
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	d, _ := quant.LoadDataset("Vanilla")
	for _, tc := range cases {
		t.Run(fmt.Sprintf("type%d-mode%d-count%d", tc.Production.MachineType, tc.Mode, tc.Production.MachineCount), func(t *testing.T) {
			want, err := blueprint.Decode(tc.Blueprint, true)
			if err != nil {
				t.Fatal(err)
			}
			s, _ := normalizeSetting(Setting{})
			b, _ := newBuilder(s)
			if _, err = b.buildLine(tc.Production, d.Recipes[d.RecipeByID[tc.Production.RecipeID]], tc.Production.MachineCount, tc.Mode, 0, true); err != nil {
				t.Fatal(err)
			}
			got, _, _, err := b.records()
			if err != nil {
				t.Fatal(err)
			}
			a, c := canonicalBuildings(got), canonicalBuildings(want.Buildings)
			if len(a) != len(c) {
				t.Fatalf("building count got=%d reference=%d", len(a), len(c))
			}
			differences := 0
			for i := range a {
				if a[i] != c[i] {
					if differences < 3 {
						t.Errorf("got %s\nref %s", a[i], c[i])
					}
					differences++
				}
			}
			if differences > 0 {
				t.Logf("%d geometry/port differences", differences)
			}
		})
	}
}

// TestReferenceStationParameters 职责：逐字对拍物流塔 2048 字参数；使用：行星/星际模式；边界：不证明塔的平面布局可在游戏中粘贴。
func TestReferenceStationParameters(t *testing.T) {
	path := os.Getenv("DSBOX_LAYOUT_GOLDEN")
	if path == "" {
		t.Skip("需外部 Node golden")
	}
	raw, err := os.ReadFile(path + ".stations.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Mode      int    `json:"mode"`
		Blueprint string `json:"blueprint"`
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(fmt.Sprint(tc.Mode), func(t *testing.T) {
			document, err := blueprint.Decode(tc.Blueprint, true)
			if err != nil {
				t.Fatal(err)
			}
			setting, _ := normalizeSetting(Setting{})
			b, _ := newBuilder(setting)
			station := b.createStation(vector{}, []stationItem{{id: 1101, provide: true}, {id: 1001}}, tc.Mode)
			want := document.Buildings[0]
			if station.record.ItemID != want.ItemID || station.record.ModelIndex != want.ModelIndex {
				t.Fatal("station prototype mismatch")
			}
			if !reflect.DeepEqual(station.record.Parameters, want.Parameters) {
				for i, v := range station.record.Parameters {
					if v != want.Parameters[i] {
						t.Errorf("parameter[%d] got=%d reference=%d", i, v, want.Parameters[i])
					}
				}
			}
		})
	}
}

// TestReferenceMixAllocations 职责：对拍每个原版配方的比例分配；使用：混带限速基础；边界：数值正确不等同于物流时序仿真。
func TestReferenceMixAllocations(t *testing.T) {
	path := os.Getenv("DSBOX_LAYOUT_GOLDEN")
	if path == "" {
		t.Skip("需外部 Node golden")
	}
	raw, err := os.ReadFile(path + ".mix.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		RecipeID int `json:"recipeId"`
		Plans    []struct {
			ItemID   int     `json:"itemId"`
			Expected float64 `json:"expected"`
			Real     float64 `json:"real"`
		} `json:"plans"`
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	d, _ := quant.LoadDataset("Vanilla")
	for _, tc := range cases {
		got := mixAllocation(d.Recipes[d.RecipeByID[tc.RecipeID]])
		if len(got) != len(tc.Plans) {
			t.Fatal(tc.RecipeID, "plan count")
		}
		for i, want := range tc.Plans {
			if got[i].id != want.ItemID || math.Abs(got[i].expected-want.Expected) > 1e-7 || math.Abs(got[i].real-want.Real) > 1e-7 {
				t.Errorf("recipe %d got=%+v reference=%+v", tc.RecipeID, got[i], want)
			}
		}
	}
}

// canonicalBuildings 职责：把连接下标转换为端点坐标再排序；使用：消除装配顺序差异；边界：保留设备、参数与端口语义。
func canonicalBuildings(buildings []blueprint.Building) []string {
	key := func(b blueprint.Building) string {
		return fmt.Sprintf("%d:(%.3f,%.3f,%.3f)-(%.3f,%.3f,%.3f)", b.ItemID, math.Round(float64(b.LocalOffset.X)*1000)/1000, math.Round(float64(b.LocalOffset.Y)*1000)/1000, math.Round(float64(b.LocalOffset.Z)*1000)/1000, math.Round(float64(b.LocalOffset2.X)*1000)/1000, math.Round(float64(b.LocalOffset2.Y)*1000)/1000, math.Round(float64(b.LocalOffset2.Z)*1000)/1000)
	}
	keys := map[int32]string{}
	for _, b := range buildings {
		keys[b.Index] = key(b)
	}
	out := []string{}
	for _, b := range buildings {
		out = append(out, fmt.Sprintf("%s recipe%d model%d yaw%.2f/%.2f in%s/%d out%s/%d filter%d params%v", key(b), b.RecipeID, b.ModelIndex, b.Yaw, b.Yaw2, keys[b.InputObjIdx], b.InputFromSlot, keys[b.OutputObjIdx], b.OutputToSlot, b.FilterID, b.Parameters))
	}
	sort.Strings(out)
	return out
}
