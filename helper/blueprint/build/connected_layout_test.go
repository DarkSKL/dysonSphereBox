package build

// 文件职责：回归直连带轮廓靠拢的凹槽利用、确定性和真实量化面积收益。
// 使用：go test 验证矩形基线与新候选使用相同机器/流边；边界：离线规则和编码成功不代替游戏内粘贴与产量验收。

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/quant"
)

// TestConnectedPackingUsesEmptyContourNotBlockRectangle 职责：证明靠拢可使用大包围盒内部的空凹槽；使用：合成占位回归；边界：仍拒绝侵入真实实体，不依赖缩小碰撞体得到面积收益。
func TestConnectedPackingUsesEmptyContourNotBlockRectangle(t *testing.T) {
	box := func(x, y, width, height float64) connectedPackingShape {
		geometry := placement.Box{
			Center:   placement.Vector{(x + width/2) * planarGridLength, 1, (y + height/2) * planarGridLength},
			HalfSize: placement.Vector{width * planarGridLength / 2, 1, height * planarGridLength / 2},
			Axes:     [3]placement.Vector{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}},
		}
		return connectedBoxShape("static", geometry)
	}
	packing := &connectedPacking{cells: map[[3]int][]int{}}
	outline := connectedBlockContour{shapes: []connectedPackingShape{box(0, 0, 2, 8), box(8, 0, 2, 8), box(0, 0, 10, 2)}}
	if _, _, err := packing.place(context.Background(), outline, vector{}, vector{10, 8, 0}, 0, 0, 30); err != nil {
		t.Fatal(err)
	}
	inner := connectedBlockContour{shapes: []connectedPackingShape{box(0, 0, 2, 2)}}
	x, y, err := packing.place(context.Background(), inner, vector{}, vector{2, 2, 0}, 0, 10, 30)
	if err != nil {
		t.Fatal(err)
	}
	if x < 2 || x > 6 || y < 2 || y+2 > 8 {
		t.Fatalf("预期占用 U 形轮廓内部空凹槽，实际位置 (%.0f,%.0f)", x, y)
	}
	if packing.high.y > 9 {
		t.Fatalf("新块不应像旧矩形排布那样增加包络高度，实际 %.0f", packing.high.y)
	}
	if packing.fits(context.Background(), inner, 0, 0) {
		t.Fatal("真实静态盒相交的候选必须被拒绝")
	}
}

// TestConnectedPackingRejectsCancelledAndUnknownGeometry 职责：锁定轮廓搜索取消和未知规则的失败关闭；使用：有界搜索边界回归；边界：不把未检查轮廓默认为空闲位置。
func TestConnectedPackingRejectsCancelledAndUnknownGeometry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	packing := &connectedPacking{cells: map[[3]int][]int{}}
	if _, _, err := packing.place(ctx, connectedBlockContour{}, vector{}, vector{2, 2, 0}, 0, 0, 60); err == nil {
		t.Fatal("已取消的候选搜索不得提交成功位置")
	}
	if !connectedShapesConflict(connectedPackingShape{kind: "unknown"}, connectedPackingShape{kind: "static"}) {
		t.Fatal("未知几何类别不得默认安全")
	}
}

// TestConnectedPackingImprovesReproducibleVanillaLayout 职责：用同一真实量化输入比较旧矩形候选和新轮廓候选；使用：空间压缩正式回归；边界：机器、规范边和 lane 总量不得为压缩面积被删减。
func TestConnectedPackingImprovesReproducibleVanillaLayout(t *testing.T) {
	var fixtures struct {
		Cases []struct {
			Name    string        `json:"name"`
			Request quant.Request `json:"request"`
			Setting Setting       `json:"setting"`
		} `json:"cases"`
	}
	raw, err := os.ReadFile("../../quant/testdata/acceptance-fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &fixtures); err != nil || len(fixtures.Cases) == 0 {
		t.Fatalf("真实量化夹具不可用: %v", err)
	}
	fixture := fixtures.Cases[0]
	plan, err := quant.Solve(context.Background(), fixture.Request)
	if err != nil {
		t.Fatal(err)
	}
	setting, err := normalizeSetting(fixture.Setting)
	if err != nil {
		t.Fatal(err)
	}
	dataset, err := quant.LoadDataset(plan.Dataset)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := quant.BuildFlowGraph(plan)
	if err != nil {
		t.Fatal(err)
	}
	req := Request{Plan: plan, Mode: 1, Setting: setting, Name: fixture.Name}
	var baseline *Result
	for _, limit := range []float64{math.Max(60, setting.BlockWidthLimit), math.Max(100, setting.BlockWidthLimit*1.5), math.Max(160, setting.BlockWidthLimit*2)} {
		candidate, err := newBuilder(setting)
		if err != nil {
			t.Fatal(err)
		}
		candidate.flowGraph, candidate.logistics = &graph, newLogisticsSummary(graph, setting)
		if err := candidate.assembleConnectedVariant(context.Background(), req, dataset, limit, false); err != nil {
			t.Fatalf("矩形基线不可行: %v", err)
		}
		out, err := candidate.finish(context.Background(), req, dataset)
		if err != nil {
			t.Fatalf("矩形基线最终规则失败: %v", err)
		}
		if baseline == nil || betterTransportCandidate(out.LogisticsSummary.Metrics, baseline.LogisticsSummary.Metrics) {
			baseline = &out
		}
	}
	first, err := Generate(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Generate(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if first.LogisticsSummary.Metrics.EnvelopeArea >= baseline.LogisticsSummary.Metrics.EnvelopeArea*.99 {
		t.Fatalf("固定候选预算应产生可重复面积收益：矩形 %.0f，轮廓 %.0f", baseline.LogisticsSummary.Metrics.EnvelopeArea, first.LogisticsSummary.Metrics.EnvelopeArea)
	}
	if first.LogisticsSummary.FlowGraphDigest != baseline.LogisticsSummary.FlowGraphDigest || len(first.LogisticsSummary.Lanes) != len(baseline.LogisticsSummary.Lanes) {
		t.Fatal("压缩不得改变规范供需图或 lane 分配")
	}
	if !reflect.DeepEqual(first.LogisticsSummary.Lanes, second.LogisticsSummary.Lanes) || first.LogisticsSummary.Metrics != second.LogisticsSummary.Metrics {
		t.Fatal("同一输入重复生成必须得到相同 lane、面积和路线度量")
	}
	decodedFirst, err := blueprint.Decode(first.Blueprint, true)
	if err != nil {
		t.Fatal(err)
	}
	decodedSecond, err := blueprint.Decode(second.Blueprint, true)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decodedFirst.Buildings, decodedSecond.Buildings) {
		t.Fatal("同一输入重复生成的实体位置、方向或连接不一致")
	}
	for _, binding := range first.Bindings {
		if len(binding.MachineIndices) != plan.Productions[binding.ProductionIndex].MachineCount {
			t.Fatalf("产线 %d 为压缩删除或复制了机器", binding.ProductionIndex)
		}
	}
	t.Logf("同输入矩形基线 area=%.0f length=%.0f blank=%.3f；轮廓候选 area=%.0f length=%.0f blank=%.3f", baseline.LogisticsSummary.Metrics.EnvelopeArea, baseline.LogisticsSummary.Metrics.InternalBeltLength, baseline.LogisticsSummary.Metrics.BlankRatio, first.LogisticsSummary.Metrics.EnvelopeArea, first.LogisticsSummary.Metrics.InternalBeltLength, first.LogisticsSummary.Metrics.BlankRatio)
}
