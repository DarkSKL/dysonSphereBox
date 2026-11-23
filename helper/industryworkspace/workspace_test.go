package industryworkspace

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/saveanalysis"
)

// TestResourceCapacityAvailability 职责：验证规划选址使用可选矿点；使用：覆盖当前和旧资源概要；边界：显式零代表没有可选矿。
func TestResourceCapacityAvailability(t *testing.T) {
	count, occupied, available := int64(10), int64(2), int64(0)
	amount, occupiedAmount, availableAmount := int64(500), int64(100), int64(0)
	ore := saveanalysis.PlanetResource{Kind: "solid", Unit: "items", Count: &count, OccupiedCount: &occupied, AvailableCount: &available,
		Amount: &amount, OccupiedAmount: &occupiedAmount, AvailableAmount: &availableAmount}
	if got := resourceCapacity(ore); got.Count != 0 || got.RemainingItems == nil || *got.RemainingItems != 0 {
		t.Fatalf("显式零可选矿仍有供给：%+v", got)
	}
	ore.AvailableCount, ore.AvailableAmount = nil, nil
	if got := resourceCapacity(ore); got.Count != 8 || got.RemainingItems == nil || *got.RemainingItems != 400 {
		t.Fatalf("旧概要未按占用扣减：%+v", got)
	}
	rate, usedRate, availableRate := 5.0, 1.0, 2.0
	oil := saveanalysis.PlanetResource{Kind: "oil", Unit: "oil_amount_raw", Rate: &rate, OccupiedRate: &usedRate, AvailableRate: &availableRate}
	if got := resourceCapacity(oil); got.BaseRatePerMinute != 120 || got.RemainingItems != nil {
		t.Fatalf("可选油速换算错误：%+v", got)
	}
	ore.Source = "saved"
	liquid := saveanalysis.PlanetResource{Kind: "liquid", Source: "generated"}
	resources := summaryCapacities(saveanalysis.PlanetResourceSummary{Source: "candidate", Resources: []saveanalysis.PlanetResource{ore, liquid}})
	if len(resources) != 2 || resources[0].Source != "candidate" || resources[0].Count != 0 || resources[0].RemainingItems != nil || resources[1].UnitRatePerMinute != 50 {
		t.Fatalf("扫描候选混入未验证的矿点供给或丢失已知液体：%+v", resources)
	}
}

// TestIndustryPack 检查预览分片与完整包必须来自同一星球，并检查导出失败保留原文件。
func TestIndustryPack(t *testing.T) {
	code := "BLUEPRINT:" + strings.Repeat("A", 200000)
	name := "电磁矩阵-600每分钟"
	sheet := BlueprintSheet{ID: 101, Name: name, File: name + "/" + name + ".txt"}
	bundle := Bundle{Report: Report{Exportable: true, ExportName: name, BlueprintCount: 1, Planets: []PlanetSheet{{ID: 101, File: sheet.File, Sheets: []BlueprintSheet{sheet}}}}, Codes: map[int64]string{101: code}}
	first, err := bundle.ReadChunk(101, 0)
	if err != nil || first.Done {
		t.Fatalf("首片 %v %v", first.Done, err)
	}
	last, err := bundle.ReadChunk(101, first.NextOffset)
	if err != nil || !last.Done || first.Text+last.Text != code {
		t.Fatal("无法复原蓝图", err)
	}
	if _, err = bundle.ReadChunk(202, 0); err == nil {
		t.Fatal("接受了未知星球")
	}
	var out bytes.Buffer
	if err = bundle.WriteZIP(&out); err != nil {
		t.Fatal(err)
	}
	pack, err := zip.NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()))
	if err != nil || len(pack.File) != 3 {
		t.Fatal("包成员数不符", err)
	}
	entry, err := pack.Open(sheet.File)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := io.ReadAll(entry)
	entry.Close()
	if err != nil || string(actual) != code {
		t.Fatal("蓝图包和预览内容不同")
	}
	target := filepath.Join(t.TempDir(), "已存在.txt")
	if err = bundle.Export(target); err != nil {
		t.Fatal(err)
	}
	if saved, err := os.ReadFile(target); err != nil || string(saved) != code {
		t.Fatal("单张导出与预览不同", err)
	}
	if err = os.WriteFile(target, []byte("原有内容"), 0600); err != nil {
		t.Fatal(err)
	}
	bundle.Report.Exportable = false
	if err = bundle.Export(target); err == nil {
		t.Fatal("不可行方案导出成功")
	}
	existing, err := os.ReadFile(target)
	if err != nil || string(existing) != "原有内容" {
		t.Fatal("覆盖了原有目标", err)
	}
}

// TestIndustrySavedPack 用主动指定的真实快照跑完选星、求解、定位、整星编码和 ZIP 导入复读。
func TestIndustrySavedPack(t *testing.T) {
	path := os.Getenv("DSP_SAVE_SAMPLE")
	if path == "" {
		t.Skip("未指定本地真实存档")
	}
	snapshot, err := saveanalysis.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	view, world := Describe("sample", snapshot)
	t.Run("既有货槽", func(t *testing.T) {
		records, err := snapshot.Item(1001, 60)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range records.Planets {
			for _, old := range p.Slots {
				if old.LocalLogic != 1 && old.LocalLogic != 2 {
					continue
				}
				matches := existingSlots(snapshot, p.PlanetID, map[int]uint8{1001: uint8(3 - old.LocalLogic)}, nil)
				for _, got := range matches {
					if got.StationID == old.StationID && got.PlanetID == p.PlanetID && got.Slot == old.Index+1 && got.ItemID == 1001 && got.Logic == old.LocalLogic && !got.Remote {
						return
					}
				}
				t.Fatal("同星已有相反货槽遗漏")
			}
		}
		t.Fatal("样本没有可核对的既有铁矿货槽")
	})
	// 物流肯定用例选取干燥空地，液面与存量矿区由各自的验收案例覆盖。
	links := snapshot.World()
	free := map[int64]bool{}
	for _, link := range links.Planets {
		free[link.PlanetID] = !link.HasFactory
	}
	var center, near, far int64
	byStar := map[int64][]int64{}
	for _, p := range world.Planets {
		if !p.Buildable || !free[p.ID] {
			continue
		}
		dry := true
		for _, resource := range p.Resources {
			if resource.Kind == "liquid" {
				dry = false
				break
			}
		}
		if dry {
			byStar[p.StarID] = append(byStar[p.StarID], p.ID)
		}
	}
	for _, star := range links.Stars {
		if sites := byStar[star.StarID]; len(sites) >= 2 {
			center, near = sites[0], sites[1]
			for _, other := range links.Stars {
				if ids := byStar[other.StarID]; other.StarID != star.StarID && len(ids) > 0 {
					far = ids[0]
					break
				}
			}
			break
		}
	}
	if center == 0 || near == 0 || far == 0 {
		t.Fatal("样本缺少同星与不同恒星的建设场地")
	}
	external := false
	tests := []struct {
		name    string
		deliver int64
		targets []quant.Target
	}{
		{"single", 0, []quant.Target{{ItemID: 1101, RatePerMinute: 30}}},
		{"same-star", near, []quant.Target{{ItemID: 1114, RatePerMinute: 30}, {ItemID: 1120, RatePerMinute: 15}}},
		{"interstellar", far, []quant.Target{{ItemID: 1114, RatePerMinute: 30}, {ItemID: 1120, RatePerMinute: 15}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := Request{SnapshotID: view.SnapshotID, SaveID: view.SaveID, Planning: industryplan.Request{CenterPlanetID: center, Profile: "low", EnableMining: &external, Quant: quant.Request{Targets: tc.targets}}}
			if tc.deliver != 0 {
				req.Planning.DeliveryPlanetIDs = map[int]int64{1120: tc.deliver}
			}
			if err := req.Validate(); err != nil {
				t.Fatal(err)
			}
			result, err := Generate(context.Background(), req, view, world, snapshot, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !result.Report.Exportable {
				t.Fatalf("星球 %d → %d，状态 %s：%v", center, tc.deliver, result.Report.Status, result.Report.Warnings)
			}
			if len(result.Report.External) == 0 {
				t.Fatal("外供原料未计入")
			}
			if tc.deliver != 0 && (len(result.Report.Routes) == 0 || len(result.Codes) < 2) {
				t.Fatal("指定的异地交付未形成有蓝图的路线")
			}
			var encoded bytes.Buffer
			if err = result.WriteZIP(&encoded); err != nil {
				t.Fatal(err)
			}
			z, err := zip.NewReader(bytes.NewReader(encoded.Bytes()), int64(encoded.Len()))
			if err != nil {
				t.Fatal(err)
			}
			for _, sheet := range result.Report.Planets {
				zones := snapshot.GroundBaseZones(sheet.ID)
				if len(zones) > 0 && !strings.Contains(strings.Join(sheet.Warnings, "\n"), fmt.Sprintf("%d 处黑雾基地或其落地区", len(zones))) {
					t.Fatalf("星球 %d 的施工说明缺少 %d 处存档基地的避让记录", sheet.ID, len(zones))
				}
				member, err := z.Open(sheet.File)
				if err != nil {
					t.Fatal(err)
				}
				data, err := io.ReadAll(member)
				member.Close()
				if err != nil {
					t.Fatal(err)
				}
				doc, err := blueprint.Decode(string(data), true)
				if err != nil || len(doc.Buildings) != sheet.Buildings || doc.GameVersion != view.GameVersion {
					t.Fatalf("无法解码 %s，%v", sheet.File, err)
				}
				if tc.deliver != 0 {
					found := false
					for _, building := range doc.Buildings {
						if building.ItemID != 2104 || len(building.Parameters) != 2048 {
							continue
						}
						p := building.Parameters
						for i := 0; i < 5; i++ {
							if p[i*6] == 1120 && p[i*6+2] != 0 {
								mask := uint64(uint32(p[332])) | uint64(uint32(p[333]))<<32
								if p[334] != 4 || mask != snapshot.FreeRemoteGroup() {
									t.Fatal("原版物流塔未设指定专组", mask, p[334])
								}
								found = true
							}
						}
					}
					if !found {
						t.Fatal("跨星际方案缺少真实氢供需槽")
					}
				}
			}
			// 仅显式指定验收目录时留存样例；平时运行测试不输出文件。
			if dir := os.Getenv("DSP_INDUSTRY_EVIDENCE"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := result.Export(filepath.Join(dir, tc.name+result.ExportExtension())); err != nil {
					t.Fatal(err)
				}
			}
			t.Logf("已复读 ZIP：%d 星球，%d 航线，%d 字节，状态 %s，外供 %v", len(result.Codes), len(result.Report.Routes), encoded.Len(), result.Report.Status, fmt.Sprint(result.Report.External))
		})
	}
	for _, facility := range []int{2301, 2316} {
		t.Run(fmt.Sprintf("矿机-%d", facility), func(t *testing.T) {
			var mineSite int64
			for _, p := range world.Planets {
				if !p.Buildable || !free[p.ID] || !p.ResourcesKnown {
					continue
				}
				hasIron, wet := false, false
				for _, r := range p.Resources {
					if r.Kind == "liquid" {
						wet = true
					}
					if r.ItemID == 1001 && r.Count >= 8 {
						hasIron = true
					}
				}
				if hasIron && !wet {
					mineSite = p.ID
					break
				}
			}
			if mineSite == 0 {
				t.Fatal("样本中没有空闲固体矿区")
			}
			profile := "low"
			if facility == 2316 {
				profile = "standard"
			}
			req := Request{SnapshotID: view.SnapshotID, SaveID: view.SaveID, Planning: industryplan.Request{
				CenterPlanetID: mineSite, Profile: profile, Mining: industryplan.MiningSettings{FacilityID: facility, SpeedMultiplier: 1, FacilityMultiplier: 1},
				Quant: quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 30}}},
			}}
			if err := req.Validate(); err != nil {
				t.Fatal(err)
			}
			result, err := Generate(context.Background(), req, view, world, snapshot, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !result.Report.Exportable {
				t.Fatalf("星球 %d %s：%v", mineSite, result.Report.Status, result.Report.Warnings)
			}
			found := false
			for _, sheet := range result.Report.Planets {
				var reserve int64
				for _, m := range sheet.MinePlacements {
					if m.ItemID == 1001 && m.FacilityID == facility && len(m.VeinIDs) > 0 && m.RatePerMinute >= 30 {
						found = true
						if m.CoveredItems == nil || *m.CoveredItems <= 0 {
							t.Fatal("有限矿区缺少实际覆盖余量")
						}
						reserve += *m.CoveredItems
					}
				}
				for _, m := range sheet.Mines {
					if m.ItemID == 1001 && (m.CoveredItems == nil || *m.CoveredItems != reserve || m.AvailableMinutes == nil || *m.AvailableMinutes != float64(reserve)/m.RatePerMinute) {
						t.Fatal("矿区寿命使用了未覆盖矿点的储量")
					}
				}
			}
			if !found {
				t.Fatal("蓝图缺少可追溯的真实矿机覆盖")
			}
			for _, need := range result.Report.External {
				if need.ItemID == 1001 && need.Kind != "local-drones" {
					t.Fatal("矿区用例退化成了外供铁矿")
				}
			}
			if dir := os.Getenv("DSP_INDUSTRY_EVIDENCE"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := result.Export(filepath.Join(dir, fmt.Sprintf("miner-%d", facility)+result.ExportExtension())); err != nil {
					t.Fatal(err)
				}
			}
			t.Logf("实际矿机 %d，星球 %d，复核通过", facility, mineSite)
		})
	}
}
