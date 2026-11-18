package saveanalysis

// 文件职责：验证独立游戏加载基准与正式查询结果。
// 使用：显式指定 DSP_SAVE_SAMPLE、DSP_SAVE_NATIVE_JSON；边界：基准留在本地，不由待测解析器生成。
import (
	"DysonSphereBox/helper/saveanalysis/galaxy"
	"encoding/json"
	"math"
	"os"
	"testing"
)

// TestGameLoadedBaseline 职责：逐项核对星系、箱子和六档产耗；使用：离线重放游戏原始加载结果；边界：不是面板外观验收。
func TestGameLoadedBaseline(t *testing.T) {
	path := os.Getenv("DSP_SAVE_NATIVE_JSON")
	if path == "" {
		t.Skip("未指定游戏独立基准")
	}
	var ref struct {
		SHA256                string
		Tick                  int64
		Seed, Algorithm, LCID int64
		Stars                 []struct {
			ID                 int64
			Name, OverrideName string
			Position           galaxy.Vector
		}
		Planets []struct {
			ID, StarID, ParentPlanetID, FactoryIndex int64
			Name, OverrideName                       string
			Position                                 galaxy.Vector
		}
		Boxes []struct {
			PlanetID, ID, EntityID, Previous, Next int64
			Items, Counts                          []int64
		}
		Products []struct {
			PlanetID, ItemID int64
			Totals           []int64
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &ref); err != nil {
		t.Fatal(err)
	}
	s, err := Read(os.Getenv("DSP_SAVE_SAMPLE"))
	if err != nil {
		t.Fatal(err)
	}
	w := s.World()
	if w.SHA256 != ref.SHA256 || w.Tick != ref.Tick || w.Seed != ref.Seed || w.Algorithm != ref.Algorithm || w.NameLCID != ref.LCID {
		t.Fatal("快照身份或生成参数不一致")
	}
	if !w.CompletePlanets || len(w.Stars) != len(ref.Stars) || len(w.Planets) != len(ref.Planets) {
		t.Fatal("完整星系覆盖不一致")
	}
	distance := func(a, b galaxy.Vector) float64 {
		return math.Sqrt((a.X-b.X)*(a.X-b.X) + (a.Y-b.Y)*(a.Y-b.Y) + (a.Z-b.Z)*(a.Z-b.Z))
	}
	maxStar, maxPlanet := 0.0, 0.0
	for i, a := range w.Stars {
		b := ref.Stars[i]
		name := b.Name
		if b.OverrideName != "" {
			name = b.OverrideName
		}
		d := distance(a.Position, b.Position)
		maxStar = math.Max(maxStar, d)
		if a.StarID != b.ID || a.Name != name || a.GeneratedName != b.Name || a.OverrideName != b.OverrideName || d > 0.000001 {
			t.Fatalf("恒星 %d 不一致，距离误差 %g", b.ID, d)
		}
	}
	unbuilt := 0
	for i, a := range w.Planets {
		b := ref.Planets[i]
		name := b.Name
		if b.OverrideName != "" {
			name = b.OverrideName
		}
		d := distance(a.Position, b.Position)
		maxPlanet = math.Max(maxPlanet, d)
		if a.PlanetID != b.ID || a.StarID != b.StarID || a.ParentPlanetID != b.ParentPlanetID || a.Name != name || a.GeneratedName != b.Name || a.OverrideName != b.OverrideName || a.HasFactory != (b.FactoryIndex >= 0) || d > 0.1 {
			t.Fatalf("星球 %d 不一致，距离误差 %g: %+v / %+v", b.ID, d, a, b)
		}
		if !a.HasFactory {
			inv, e := s.Inventory(a.PlanetID, 0)
			if e != nil || len(inv.Entries) != 0 {
				t.Fatalf("未建厂星球库存异常 %d: %v", a.PlanetID, e)
			}
			unbuilt++
		}
	}
	inv, err := s.Inventory(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	actual := map[[3]int64]InventoryEntry{}
	for _, v := range inv.Entries {
		if v.Kind == "storage" {
			key := [3]int64{v.PlanetID, v.ContainerID, int64(v.SlotIndex)}
			if _, ok := actual[key]; ok {
				t.Fatal("箱子槽位重复")
			}
			actual[key] = v
		}
	}
	slots, stacked := 0, 0
	var storageTotal int64
	seenBoxes := map[[2]int64]bool{}
	for _, b := range ref.Boxes {
		key := [2]int64{b.PlanetID, b.ID}
		if seenBoxes[key] || len(b.Items) != len(b.Counts) {
			t.Fatal("基准箱子无效")
		}
		seenBoxes[key] = true
		if b.Previous != 0 || b.Next != 0 {
			stacked++
		}
		for i, id := range b.Items {
			k := [3]int64{b.PlanetID, b.ID, int64(i)}
			v, ok := actual[k]
			if !ok || v.EntityID != b.EntityID || v.ItemID != id || v.Count != b.Counts[i] {
				t.Fatalf("箱子槽位不一致 %v", k)
			}
			delete(actual, k)
			slots++
			storageTotal += b.Counts[i]
		}
	}
	if len(actual) != 0 || storageTotal != inv.Totals["storage"] {
		t.Fatal("箱子覆盖或分类汇总不一致")
	}
	windows := []int64{10, 60, 600, 3600, 36000, 360000}
	totals := map[[2]int64]Counts{}
	products := map[[2]int64]bool{}
	for _, p := range ref.Products {
		key := [2]int64{p.PlanetID, p.ItemID}
		if products[key] || len(p.Totals) != 14 {
			t.Fatal("基准统计无效")
		}
		products[key] = true
		for i, seconds := range windows {
			item, e := s.Item(p.ItemID, seconds)
			if e != nil {
				t.Fatal(e)
			}
			found := false
			for _, v := range item.Planets {
				if v.PlanetID == p.PlanetID {
					found = true
					if v.History == nil || v.History.Produced != p.Totals[i] || v.History.Consumed != p.Totals[i+7] {
						t.Fatalf("统计不一致 planet=%d item=%d window=%d", p.PlanetID, p.ItemID, seconds)
					}
				}
			}
			if !found {
				t.Fatal("统计星球缺失")
			}
			k := [2]int64{p.ItemID, seconds}
			sum := totals[k]
			sum.Produced += p.Totals[i]
			sum.Consumed += p.Totals[i+7]
			totals[k] = sum
		}
	}
	actualProducts := 0
	for id, records := range s.items {
		for _, p := range records {
			if p.totals != nil {
				actualProducts++
				if !products[[2]int64{p.planetID, id}] {
					t.Fatal("存在额外统计记录")
				}
			}
		}
	}
	if actualProducts != len(products) {
		t.Fatal("统计覆盖不一致")
	}
	for _, seconds := range windows {
		summary, _ := s.Summary(seconds)
		for _, v := range summary.Items {
			expected, ok := totals[[2]int64{v.ItemID, seconds}]
			if ok {
				if v.History == nil || *v.History != expected {
					t.Fatal("全档统计汇总不一致")
				}
			} else if v.History != nil {
				t.Fatal("缺失统计被伪装为零")
			}
		}
	}
	t.Logf("tick=%d stars=%d planets=%d unbuilt=%d maxStarMeters=%g maxPlanetMeters=%g boxes=%d stacked=%d slots=%d storageTotal=%d products=%d planetWindows=%d summaries=%d", ref.Tick, len(ref.Stars), len(ref.Planets), unbuilt, maxStar, maxPlanet, len(ref.Boxes), stacked, slots, storageTotal, len(ref.Products), len(ref.Products)*6, len(totals))
}
