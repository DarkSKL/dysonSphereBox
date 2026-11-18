package environment

// 文件职责：验证存档分析的授权边界及实档服务入口。
// 使用：go test；设置 DSP_SAVE_SAMPLE 可运行本机只读验收；边界：不更改存档和配置。

import (
	"DysonSphereBox/helper/saveanalysis"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestSaveAnalysisAuthorization 职责：确保未授权不读取列表和内容；使用：服务回归；边界：不依赖真实存档存在。
func TestSaveAnalysisAuthorization(t *testing.T) {
	service := &SaveAnalysisService{}
	if _, err := service.ListSaves(""); err == nil {
		t.Fatal("未授权返回列表")
	}
	if _, err := service.Analyze("any", 60); err == nil {
		t.Fatal("未授权解析")
	}
	if _, err := service.GetItem("any", 1001, 60); err == nil {
		t.Fatal("无快照查询成功")
	}
	if _, err := service.GetPlanetProductionDevices("any", 104); err == nil {
		t.Fatal("未授权返回设备清单")
	}
	if _, err := service.GetProductionInputSupply("any", 104, 6005, 1305, saveanalysis.SupplyScope{}); err == nil {
		t.Fatal("未授权返回上游设备")
	}
	if _, err := service.DiagnoseProduction("any", 104, 1203); err == nil {
		t.Fatal("未授权返回自动诊断")
	}
	if _, err := service.GetStationSupply("any", 104, 49, 1); err == nil {
		t.Fatal("未授权返回运输任务")
	}
	if _, err := service.GetProductionItems("any", 104); err == nil {
		t.Fatal("未授权返回产物列表")
	}
	if _, err := service.GetProductionLine("any", 104, 1114); err == nil {
		t.Fatal("未授权返回逐机诊断")
	}
	if _, err := service.GetCircuitLine("any", 104); err == nil {
		t.Fatal("无授权返回电路板诊断")
	}
	if _, err := service.GetWorld("any"); err == nil {
		t.Fatal("无快照星球查询成功")
	}
	if _, err := service.GetInventory("any", 0, 0); err == nil {
		t.Fatal("无快照库存查询成功")
	}
	if _, err := service.GetSphere("any", 1, 0); err == nil {
		t.Fatal("无快照球体查询成功")
	}
}

// TestSaveAnalysisSample 职责：通过真实授权、列表、saveId 和物品详情验收；使用：本机指定样本；边界：样本必须属于现有目录解析结果。
func TestSaveAnalysisSample(t *testing.T) {
	sample := os.Getenv("DSP_SAVE_SAMPLE")
	if sample == "" {
		t.Skip("未指定实档")
	}
	service := &SaveAnalysisService{}
	root := service.GetDirectory()
	items, err := service.AuthorizeRead(root.Path)
	if err != nil {
		t.Fatal(err)
	}
	var id string
	for _, item := range items {
		if item.FileName == filepath.Base(sample) {
			id = item.ID
			break
		}
	}
	if id == "" {
		t.Fatal("样本不在授权目录")
	}
	result, err := service.Analyze(id, 60)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Summary.Items) == 0 {
		t.Fatal("物品目录为空")
	}
	item, err := service.GetItem(id, result.Summary.Items[0].ItemID, 60)
	if err != nil {
		t.Fatal(err)
	}
	if item.SHA256 != result.Summary.SHA256 {
		t.Fatal("快照身份不一致")
	}
	t.Logf("items=%d companion=%v", len(result.Summary.Items), result.HasServerCompanion)
	world, err := service.GetWorld(id)
	if err != nil || world.SHA256 != result.Summary.SHA256 || len(world.Planets) == 0 {
		t.Fatalf("星球关联查询失败: %v", err)
	}
	inventory, err := service.GetInventory(id, world.Planets[0].PlanetID, 0)
	if err != nil || inventory.SHA256 != world.SHA256 {
		t.Fatalf("分类库存查询失败: %v", err)
	}
	if _, err := service.GetCircuitLine("stale", 104); err == nil {
		t.Fatal("旧身份可查询产线")
	}
	if result.Summary.SHA256 == "38b48472618c29091f7575a10bd0453e30273e287f7265e4e5c87ee739ce763c" {
		if _, err := service.GetProductionInputSupply("stale", 104, 6005, 1305, saveanalysis.SupplyScope{}); err == nil {
			t.Fatal("旧身份查询供应链")
		}
		if _, err := service.GetStationSupply("stale", 104, 49, 1); err == nil {
			t.Fatal("旧身份查询货槽")
		}
		if _, err := service.DiagnoseProduction("stale", 104, 1203); err == nil {
			t.Fatal("旧快照返回自动诊断")
		}
		chain, err := service.DiagnoseProduction(id, 104, 6005)
		if err != nil || chain.SHA256 != world.SHA256 || chain.Tick != 7174748 || len(chain.Checks) == 0 {
			t.Fatal("自动诊断身份或结果错误", err)
		}
		firstCheck := chain.Checks[0]
		slot := chain.Nodes[firstCheck.Node].Freight
		if slot == nil || slot.Slot.StationID != 49 || slot.Slot.Index != 1 || slot.CargoInFlight != 184 || slot.AwaitingPickup != 300 || len(firstCheck.RootEntities) != 38 || len(firstCheck.DirectEntities) != 51 {
			t.Fatal("自动定位氢槽失败", firstCheck)
		}
		motor, err := service.DiagnoseProduction(id, 104, 1203)
		if err != nil || motor.FirstCheckGroup != "downstream.supply.checks" || motor.Downstream == nil || len(motor.Downstream.Checks) < 2 {
			t.Fatal("未返回同档下游检查", err)
		}
		for _, c := range motor.Downstream.Checks[:2] {
			if c.Priority != 1 || c.ItemID != 1107 || (c.EntityID != 7506 && c.EntityID != 24441) || len(c.RootEntities) != 21 || len(c.DirectEntities) != 16 || !c.Shared {
				t.Fatal("电动机下游检查错误", c)
			}
		}
		joined := motor.Downstream.Supply
		if joined == nil || len(joined.Checks) == 0 {
			t.Fatal("积压接收端的钛合金供应链缺失")
		}
		count := 0
		for _, c := range joined.Checks {
			if c.Priority != 1 {
				break
			}
			count++
			if c.ItemID != 1107 || c.Status != "output-blocked" || c.Drop == nil || c.Drop.Source != joined.Nodes[c.Node].EntityID ||
				!slices.Equal(c.RootEntities, motor.Downstream.Checks[0].RootEntities) || !slices.Contains(c.ReceiverEntities, int64(7506)) || !slices.Contains(c.ReceiverEntities, int64(24441)) || !c.Shared || !c.Unresolved {
				t.Fatal("冶炼设备检查点、共线归属或投放证据错误", c)
			}
		}
		if count != 34 {
			t.Fatal("并列受阻钛合金设备数", count)
		}
		history, err := service.DiagnoseProduction(id, 104, 1003)
		if err != nil || history.Status != "history-only" || len(history.Checks) > 0 {
			t.Fatal("历史产量被归到设备", err)
		}
		supply, err := service.GetProductionInputSupply(id, 104, 6005, 1305, saveanalysis.SupplyScope{})
		if err != nil || supply.Tick != 7174748 || len(supply.Producers) != 12 {
			t.Fatalf("矩阵来料 %v", err)
		}
		empty, err := service.GetProductionInputSupply(id, 104, 6005, 1305, saveanalysis.SupplyScope{Entities: []int64{}})
		if err != nil || len(empty.Branches) != 0 {
			t.Fatal("空实体集合被扩大")
		}
		h, err := service.GetStationSupply(id, 104, 49, 1)
		if err != nil || h.SHA256 != supply.SHA256 || h.Freight == nil || h.Freight.CargoInFlight != 184 || h.Freight.AwaitingPickup != 300 {
			t.Fatalf("对应快照氢运输 %v", err)
		}
		if _, err := service.GetProductionItems("stale", 104); err == nil {
			t.Fatal("旧身份查询产物")
		}
		if _, err := service.GetProductionLine("stale", 104, 1114); err == nil {
			t.Fatal("旧身份查询诊断")
		}
		devices, err := service.GetPlanetProductionDevices(id, 104)
		if err != nil || len(devices.Devices) != 852 || devices.Tick != 7174748 || devices.SHA256 != world.SHA256 || devices.QueuedTechID != 5904 {
			t.Fatal("设备清单快照", err)
		}
		if _, err := service.GetPlanetProductionDevices("stale", 104); err == nil {
			t.Fatal("旧身份查询设备")
		}
		mined, err := service.GetProductionLine(id, 104, 1001)
		if err != nil || mined.DeviceDiagnostics == nil || len(mined.DeviceDiagnostics.Devices) != 45 {
			t.Fatal("铁矿逐机诊断", err)
		}
		options, err := service.GetProductionItems(id, 104)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, o := range options {
			if o.ItemID == 1114 && o.Machines == 73 && o.RecordedProduction {
				found = true
			}
		}
		if !found {
			t.Fatal("精炼油设备或产出依据缺失")
		}
		oil, err := service.GetProductionLine(id, 104, 1114)
		if err != nil || oil.Tick != 7174748 || oil.SHA256 != world.SHA256 || len(oil.Machines) != 73 || len(oil.Findings) == 0 {
			t.Fatal("精炼厂诊断", err)
		}
		for _, finding := range oil.Findings {
			for _, machine := range finding.Assemblers {
				switch machine {
				case 521, 522, 590, 597:
					if finding.RouteEvidence != "filter-mismatch" {
						t.Fatal(finding)
					}
				case 560:
					if finding.RouteEvidence != "no-direction-sorter" {
						t.Fatal(finding)
					}
				}
			}
		}
		firstOil := oil.Findings[0]
		if firstOil.ItemID != 1120 || firstOil.CheckBelt != 4900 || firstOil.RouteEvidence != "shared" || len(firstOil.Assemblers) != 16 {
			t.Fatal(firstOil)
		}
		report, err := service.GetCircuitLine(id, 104)
		if err != nil || report.Tick != 7174748 || report.SHA256 != world.SHA256 || len(report.Machines) != 16 || len(report.Findings) == 0 {
			t.Fatalf("电路板诊断快照错误: %v", err)
		}
		first := report.Findings[0]
		if first.Status != "output-blocked" || len(first.Assemblers) != 7 || len(first.RelatedBlockedEntities) != 20 {
			t.Fatalf("共线诊断错误: %+v", first)
		}
	}
	sphere, err := service.GetSphere(id, world.Planets[0].StarID, 0)
	if err != nil || sphere.SHA256 != world.SHA256 {
		t.Fatalf("球体查询失败: %v", err)
	}
	if _, err := service.GetWorld("stale"); err == nil {
		t.Fatal("旧页面身份查询成功")
	}
	if _, err := service.Analyze("missing", 60); err == nil {
		t.Fatal("无效 saveId 被接受")
	}
	if _, err := service.GetSummary(id, 60); err == nil {
		t.Fatal("失败后仍暴露旧快照")
	}
	service.RevokeRead()
	if _, err := service.ListSaves(""); err == nil {
		t.Fatal("撤销后仍可读取")
	}
}

// TestAuthorizeReadFailureClearsState 职责：验证重新授权失败不保留旧会话；使用：授权生命周期回归；边界：不写配置或存档。
func TestAuthorizeReadFailureClearsState(t *testing.T) {
	service := &SaveAnalysisService{root: "stale", saveID: "stale"}
	if _, err := service.AuthorizeRead("invalid-directory"); err == nil {
		t.Fatal("无效目录被授权")
	}
	if service.root != "" || service.saveID != "" || service.snapshot != nil {
		t.Fatal("失败授权仍保留旧状态")
	}
}

// TestSnapshotPublicationRechecksDirectory 职责：确保解析期间失效的授权不能发布快照；使用：模拟旧目录的发布状态；边界：不修改真实目录配置。
func TestSnapshotPublicationRechecksDirectory(t *testing.T) {
	snapshot := &saveanalysis.Snapshot{}
	service := &SaveAnalysisService{root: t.TempDir(), saveID: "old", snapshot: snapshot}
	if err := service.publishSnapshot("new", snapshot); err == nil {
		t.Fatal("配置目录已不同仍发布快照")
	}
	if service.root != "" || service.saveID != "" || service.snapshot != nil {
		t.Fatal("发布失败仍保留旧状态")
	}
	service = &SaveAnalysisService{root: t.TempDir(), saveID: "old", snapshot: snapshot}
	service.RevokeRead()
	if service.root != "" || service.saveID != "" || service.snapshot != nil {
		t.Fatal("撤销未释放实际快照")
	}
}

// TestSaveListUsesCapturedRoot 职责：验证列表只扫描已选目录并沿用搜索行为；使用：临时存档列表；边界：文件只用于列档，不解析也不访问玩家存档。
func TestSaveListUsesCapturedRoot(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"allowed.dsv", "other.dsv"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	service := &EnvironmentManagerService{}
	items, err := service.listSavesAtRoot(root, "ALLOWED")
	if err != nil || len(items) != 1 || items[0].FileName != "allowed.dsv" {
		t.Fatalf("目录或过滤被重新配置替换: %+v %v", items, err)
	}
}
