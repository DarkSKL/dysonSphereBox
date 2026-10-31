package build

// 文件职责：以真实量化配方回归直连带规划与失败关闭。
// 使用：go test 运行规范物流和实体链检查；边界：不伪造真实游戏十分钟产量验收。

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// TestConnectedAcceptance 职责：复现固定真实目标并核验内部实体链与供需；使用：集成回归；边界：暂不包含游戏进程和个人存档。
func TestConnectedAcceptance(t *testing.T) {
	var fixtures struct {
		Cases []struct {
			Name    string        `json:"name"`
			Request quant.Request `json:"request"`
			Setting Setting       `json:"setting"`
		} `json:"cases"`
	}
	data, err := os.ReadFile("../../quant/testdata/acceptance-fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures.Cases {
		t.Run(fixture.Name, func(t *testing.T) {
			plan, err := quant.Solve(context.Background(), fixture.Request)
			if err != nil {
				t.Fatal(err)
			}
			out, err := Generate(context.Background(), Request{Plan: plan, Mode: 1, Setting: fixture.Setting, Name: fixture.Name})
			if err != nil {
				t.Fatal(err)
			}
			if out.LogisticsSummary == nil || len(out.LogisticsSummary.Lanes) == 0 {
				t.Fatal("缺少实际运输路线")
			}
			for _, lane := range out.LogisticsSummary.Lanes {
				if lane.RatePerMinute > lane.CapacityPerMinute+1e-6 || len(lane.BuildingIndices) == 0 || lane.FlowKind == "internal" && !lane.InternalConnected {
					t.Fatalf("无效 lane: %+v", lane)
				}
			}
			decoded, err := blueprint.Decode(out.Blueprint, true)
			if err != nil {
				t.Fatal(err)
			}
			if len(decoded.Buildings) != out.BuildingCount {
				t.Fatal("严格解码建筑数不符")
			}
			t.Logf("machines=%d buildings=%d lanes=%d inputs=%d envelope=%.0f footprint=%.0f blank=%.3f length=%.0f", plan.MachineCount, out.BuildingCount, len(out.LogisticsSummary.Lanes), out.LogisticsSummary.Metrics.ExternalInputCount, out.LogisticsSummary.Metrics.EnvelopeArea, out.LogisticsSummary.Metrics.FootprintArea, out.LogisticsSummary.Metrics.BlankRatio, out.LogisticsSummary.Metrics.InternalBeltLength)
		})
	}
}

// TestInserterCapacityDoesNotBlockAcceptance 职责：用真实量化结果复现配方 9 的 1301 原料跨 2 格 Mk.I 分拣器超额诊断；使用：验证结构生成降级为 unverified 而不是阻断蓝图；边界：只证明已知几何和协议可导出，不宣称低等级分拣器实际吞吐达标。
func TestInserterCapacityDoesNotBlockAcceptance(t *testing.T) {
	request := quant.Request{
		Dataset:  "Vanilla",
		Targets:  []quant.Target{{ItemID: 6001, RatePerMinute: 60}},
		Scheme:   quant.Scheme{},
		Settings: quant.Settings{},
	}
	plan, err := quant.Solve(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	var recipe9 *quant.Production
	for i := range plan.Productions {
		if plan.Productions[i].RecipeID == 9 {
			recipe9 = &plan.Productions[i]
			break
		}
	}
	if recipe9 == nil {
		t.Fatal("真实量化结果未选择 Vanilla 配方 9")
	}
	var boardRate float64
	for _, rate := range recipe9.Inputs {
		if rate.ItemID == 1301 {
			boardRate = rate.RatePerMinute
			break
		}
	}
	if math.Abs(boardRate-60) > 1e-8 {
		t.Fatalf("配方 9 原料 1301 速率不符: got %.9f, want 60", boardRate)
	}
	needPower := true
	setting := Setting{MaxMachineInALine: 15, BeltLv: 3, InserterLv: 1, MaxLays: 3, NeedPower: &needPower}
	out, err := Generate(context.Background(), Request{Plan: plan, Mode: 1, Setting: setting, Name: "recipe-9-mk1"})
	if err != nil {
		t.Fatalf("分拣器额定吞吐不足不应阻止导出: %v", err)
	}
	decoded, err := blueprint.Decode(out.Blueprint, true)
	if err != nil {
		t.Fatalf("严格解码失败: %v", err)
	}
	if len(decoded.Buildings) != out.BuildingCount {
		t.Fatalf("严格解码建筑数不符: %d != %d", len(decoded.Buildings), out.BuildingCount)
	}
	if out.LogisticsSummary == nil {
		t.Fatal("生成结果缺少物流诊断")
	}
	const needle = "配方 9 原料 1301 单端口 60.000/min 超过 Mk.1 跨 2 格分拣器"
	warningFound := false
	for _, warning := range out.Warnings {
		if strings.Contains(warning, needle) {
			warningFound = true
			break
		}
	}
	if !warningFound {
		t.Fatalf("缺少真实分拣器兼容警告: %v", out.Warnings)
	}
	unverified := false
	for _, check := range out.LogisticsSummary.Checks {
		if check.RuleID == "inserter-capacity" && check.Status == "passed" {
			t.Fatalf("超额分拣器诊断被错误标记为 passed: %+v", check)
		}
		if check.RuleID == "inserter-capacity" && check.Status == "unverified" && strings.Contains(check.Message, needle) {
			unverified = true
			break
		}
	}
	if !unverified {
		t.Fatalf("分拣器容量诊断未保留为 unverified: %+v", out.LogisticsSummary.Checks)
	}
	t.Logf("真实配方 9 Mk.I 兼容导出：recipe=%+v warnings=%v", *recipe9, out.Warnings)
}

// TestFlyAcceptance 职责：覆盖真实量化夹具的配送器与四向分流两种布局；使用：验证 mode2/5 端点、份额和严格编码均可落地；边界：不模拟机队运行时吞吐。
func TestFlyAcceptance(t *testing.T) {
	var fixtures struct {
		Cases []struct {
			Name    string        `json:"name"`
			Request quant.Request `json:"request"`
			Setting Setting       `json:"setting"`
		} `json:"cases"`
	}
	data, err := os.ReadFile("../../quant/testdata/acceptance-fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures.Cases {
		plan, err := quant.Solve(context.Background(), fixture.Request)
		if err != nil {
			t.Fatal(err)
		}
		for _, mode := range []int{2, 5} {
			mode := mode
			t.Run(fixture.Name+"-mode"+fmt.Sprint(mode), func(t *testing.T) {
				out, err := Generate(context.Background(), Request{Plan: plan, Mode: mode, Setting: fixture.Setting, Name: fixture.Name})
				if err != nil {
					t.Fatal(err)
				}
				if out.LogisticsSummary == nil || len(out.LogisticsSummary.Lanes) == 0 {
					t.Fatal("配送模式结果缺少实体 lane")
				}
				for _, lane := range out.LogisticsSummary.Lanes {
					if lane.RatePerMinute > lane.CapacityPerMinute+1e-6 || len(lane.BuildingIndices) == 0 {
						t.Fatalf("配送 lane 超载或缺少实体：%+v", lane)
					}
				}
				decoded, err := blueprint.Decode(out.Blueprint, true)
				if err != nil {
					t.Fatal(err)
				}
				if len(decoded.Buildings) != out.BuildingCount {
					t.Fatalf("严格解码建筑数不符: %d != %d", len(decoded.Buildings), out.BuildingCount)
				}
			})
		}
	}
}

// TestStationAcceptance 职责：用真实量化夹具覆盖行星/星际物流塔的多产物、槽位、路线和最终规则；使用：六模式验收矩阵中的 mode3/4；边界：不模拟机队运行和游戏内持续产量。
func TestStationAcceptance(t *testing.T) {
	var fixtures struct {
		Cases []struct {
			Name    string        `json:"name"`
			Request quant.Request `json:"request"`
			Setting Setting       `json:"setting"`
		} `json:"cases"`
	}
	data, err := os.ReadFile("../../quant/testdata/acceptance-fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures.Cases {
		plan, err := quant.Solve(context.Background(), fixture.Request)
		if err != nil {
			t.Fatal(err)
		}
		for _, mode := range []int{3, 4} {
			mode := mode
			t.Run(fixture.Name+"-mode"+fmt.Sprint(mode), func(t *testing.T) {
				out, err := Generate(context.Background(), Request{Plan: plan, Mode: mode, Setting: fixture.Setting, Name: fixture.Name})
				if err != nil {
					t.Fatal(err)
				}
				if out.LogisticsSummary == nil || len(out.LogisticsSummary.Lanes) == 0 {
					t.Fatal("物流塔结果缺少实体 lane")
				}
				decoded, err := blueprint.Decode(out.Blueprint, true)
				if err != nil {
					t.Fatal(err)
				}
				if len(decoded.Buildings) != out.BuildingCount {
					t.Fatalf("严格解码建筑数不符: %d != %d", len(decoded.Buildings), out.BuildingCount)
				}
			})
		}
	}
}
