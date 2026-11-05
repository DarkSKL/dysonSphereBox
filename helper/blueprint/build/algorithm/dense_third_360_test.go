package build

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// third360Original 职责：读取用户确认与截图对应的原图；使用：固定真实回归输入；边界：不读取或修改应用运行目录中的蓝图。
func third360Original(t *testing.T) blueprint.Document {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "docs", "超密度蓝图生成器问题", "第3档360矩阵-用户原图.txt"))
	if err != nil {
		t.Fatal(err)
	}
	document, err := blueprint.Decode(string(data), true)
	if err != nil || !document.ChecksumValid {
		t.Fatalf("用户原图解码失败：%v", err)
	}
	return document
}

// TestThird360OriginalRejectsSouthBoundary 职责：重现原图在第3档南侧边界的第三输入臂偏角；使用：防止校验被放宽而掩盖实际故障；边界：不推断截图中未记录的精确光标纬度。
func TestThird360OriginalRejectsSouthBoundary(t *testing.T) {
	document := third360Original(t)
	ctx := context.WithValue(context.Background(), denseCollisionKey{}, true)
	for _, anchor := range []int{131, 132} {
		encoding := &surfaceEncoding{areas: document.Areas, plan: &SurfacePlan{GridSegment: 200, PrimaryAreaIndex: 0, AnchorLatitudeGrid: anchor}}
		err := validateRelocatedSurface(ctx, Request{}, encoding, document.Buildings)
		if err == nil || !strings.Contains(err.Error(), "GEN_E_SURFACE_ROUTING") || !strings.Contains(err.Error(), "端点偏斜") {
			t.Fatalf("原图在纬圈%d未重现已知偏角：%v", anchor, err)
		}
	}
}

// TestDenseThirdTierBasicTransport 职责：补齐一级带/一级臂的邻近产量覆盖；使用：防止仅修复360/min单点；边界：只覆盖用户所选第3档北半球南边缘直连。
func TestDenseThirdTierBasicTransport(t *testing.T) {
	testDenseMatrixRegions(t, []float64{240, 480}, []string{"ring3-north-south"}, []int{1}, Setting{BeltLv: 1, InserterLv: 1})
}

// TestDenseThirdTierMatrix360 职责：验证一级带/一级分拣器的真实360/min请求能贴南侧边界；使用：生成、严格解码及逐纬圈复验；边界：保留原建筑排向，离线结果不代替游戏拍建。
func TestDenseThirdTierMatrix360(t *testing.T) {
	ctx := context.Background()
	plan, err := quant.Solve(ctx, quant.Request{Dataset: "Vanilla", Targets: []quant.Target{{ItemID: 6001, RatePerMinute: 360}}})
	if err != nil {
		t.Fatal(err)
	}
	req := Request{Plan: plan, Mode: 1, Density: densityDense, Setting: Setting{BeltLv: 1, InserterLv: 1}, SurfaceLayout: &SurfaceLayout{Mode: "autoTropic", GridSegment: 200, LatitudeZone: "ring3-north-south"}, Name: "电磁矩阵360-第3档北半球南侧修复版"}
	result, err := Generate(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	document, err := blueprint.Decode(result.Blueprint, true)
	if err != nil || !document.ChecksumValid {
		t.Fatalf("修复版解码失败：%v", err)
	}
	surface := result.SurfacePlan
	if surface == nil || surface.AnchorLatitudeGrid != 131 {
		t.Fatalf("所选南边缘被换成环内起点：%+v", surface)
	}
	if surface.LayoutWidth > 50 || surface.LayoutHeight > 17 {
		t.Fatalf("修复扩大了原50x17包络：%dx%d", surface.LayoutWidth, surface.LayoutHeight)
	}
	original := third360Original(t)
	if document.CursorOffset != original.CursorOffset || document.CursorTargetArea != original.CursorTargetArea || document.PrimaryAreaIndex != original.PrimaryAreaIndex {
		t.Fatal("修复改变了蓝图光标或主区域")
	}
	if len(document.Areas) != 1 || document.Areas[0].AreaSegments != 120 {
		t.Fatal("修复改变了第3档的原生经度段数")
	}
	machines := func(records []blueprint.Building) []blueprint.Building {
		out := []blueprint.Building{}
		for _, r := range records {
			if r.RecipeID > 0 {
				out = append(out, r)
			}
		}
		return out
	}
	oldMachines, newMachines := machines(original.Buildings), machines(document.Buildings)
	if len(newMachines) != plan.MachineCount || len(newMachines) != len(oldMachines) {
		t.Fatal("修复版生产机器数量改变")
	}
	for i, r := range newMachines {
		old := oldMachines[i]
		if old.ItemID != r.ItemID || old.RecipeID != r.RecipeID || math.Abs(math.Remainder(float64(r.Yaw-old.Yaw), 360)) > .001 {
			t.Fatalf("机器%d的型号、配方或排向改变", i)
		}
	}
	for _, r := range document.Buildings {
		if surfaceIsBelt(r.ItemID) && r.ItemID != 2001 || r.ItemID >= 2011 && r.ItemID <= 2014 && r.ItemID != 2011 {
			t.Fatal("修复擅自升级了带或分拣器")
		}
	}
	if surface.SafeLatitudeMinDegrees == nil || surface.SafeLatitudeMaxDegrees == nil || math.Abs(*surface.SafeLatitudeMinDegrees-131*.36) > .0001 {
		t.Fatal("未把实际南侧边界列入可用范围")
	}
	last := int(math.Round(*surface.SafeLatitudeMaxDegrees / .36))
	if last < 132 || last+surface.LayoutHeight-1 > 155 {
		t.Fatalf("声明的可用范围无效：131..%d", last)
	}
	for anchor := 131; anchor <= last; anchor++ {
		encoding := &surfaceEncoding{areas: document.Areas, plan: &SurfacePlan{GridSegment: 200, PrimaryAreaIndex: 0, AnchorLatitudeGrid: anchor}}
		if err := validateRelocatedSurface(context.WithValue(ctx, denseCollisionKey{}, true), req, encoding, document.Buildings); err != nil {
			t.Fatalf("导出后在声明纬圈%d复验失败：%v", anchor, err)
		}
	}
	t.Logf("footprint=%dx%d buildings=%d safeAnchors=131..%d", surface.LayoutWidth, surface.LayoutHeight, len(document.Buildings), last)
	if directory := os.Getenv("DSBOX_BLUEPRINT_TEST_OUTPUT"); directory != "" {
		if !filepath.IsAbs(directory) {
			t.Fatal("导出目录必须为绝对路径")
		}
		if err := os.MkdirAll(directory, 0755); err != nil {
			t.Fatal(err)
		}
		payload, err := json.MarshalIndent(struct {
			Document    blueprint.Document `json:"document"`
			SurfacePlan *SurfacePlan       `json:"surfacePlan"`
			Metrics     LayoutMetrics      `json:"metrics"`
		}{document, surface, result.LogisticsSummary.Metrics}, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "matrix360-ring3-north-south-mode1.json"), payload, 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "matrix360-ring3-north-south-mode1.txt"), []byte(result.Blueprint), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestSurfaceInserterStrictAngles 职责：拦截过去被余弦容差放行的超限角度；使用：原生24/30度边界回归；边界：不依赖具体生成布局。
func TestSurfaceInserterStrictAngles(t *testing.T) {
	cos := func(angle float64) float64 { return math.Cos(angle * math.Pi / 180) }
	for _, sample := range []struct {
		first, second, relative float64
		reject                  bool
	}{{23.99, 0, 0, false}, {24.005, 0, 0, true}, {0, 24.005, 0, true}, {15, 15, 29.99, false}, {15, 15, 30.005, true}} {
		if got := surfaceInserterAngleLimitExceeded(cos(sample.first), cos(sample.second), cos(sample.relative)); got != sample.reject {
			t.Fatalf("角度%+v判定错误：%t", sample, got)
		}
	}
}
