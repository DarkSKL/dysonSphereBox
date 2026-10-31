package build

// 文件职责：证明直连物流图在多原生纬带重排后仍逐带闭合，并锁定失败回滚。
// 使用：安全切面、跨 Area 连接和严格蓝图协议回归；边界：离线通过不代表游戏内地形、科技或连续产能已验收。

import (
	"context"
	"reflect"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// TestConnectedSurfaceChunksPadBeforeUnsafeBoundary 职责：确认设施阻挡天然边界时提前切分并显式留出补带区；使用：确定性切面规划回归；边界：所有切面被阻挡时必须失败。
func TestConnectedSurfaceChunksPadBeforeUnsafeBoundary(t *testing.T) {
	bands := []surfacePackedBand{{low: 0, high: 9, segments: 200}, {low: 10, high: 19, segments: 160}}
	cuts := make([]bool, 17)
	for i := range cuts {
		cuts[i] = true
	}
	cuts[10] = false
	chunks, ok := planConnectedSurfaceChunks(bands, cuts, 0, 30, 16)
	if !ok || len(chunks) != 2 || chunks[0].sourceHigh != 9 || chunks[1].sourceLow != 9 || chunks[1].targetLow != 10 {
		t.Fatalf("设施切面应提前一行并产生真实补带需求: %+v, %v", chunks, ok)
	}
	for i := range cuts {
		cuts[i] = false
	}
	if _, ok := planConnectedSurfaceChunks(bands, cuts, 0, 30, 16); ok {
		t.Fatal("不存在安全切面时不得拆开设施")
	}
}

// TestConnectedSurfacePreservesMultiAreaRoute 职责：以跨越多个原生纬带的完整带链证明实体引用和路径证据一致；使用：连接图感知多 Area 回归；边界：仅构造带图，不伪造生产配方或游戏运行。
func TestConnectedSurfacePreservesMultiAreaRoute(t *testing.T) {
	noPower := false
	b, err := newBuilder(Setting{BeltLv: 3, InserterLv: 3, NeedPower: &noPower})
	if err != nil {
		t.Fatal(err)
	}
	path := b.straight(2, 0, 301, 1)
	// 在首选自然带界之前加入一个合法半格节点，迫使规划器提前切面并真实补一格带。
	cuts := make([]bool, 302)
	for i := range cuts {
		cuts[i] = true
	}
	baseline, ok := planConnectedSurfaceChunks(surfaceLatitudeBands(200), cuts, -301/2, 1, 301)
	if !ok || len(baseline) < 2 {
		t.Fatal("原生纬带基线不可用")
	}
	cut := baseline[0].sourceHigh
	half := b.make(2003, 2, float64(cut)-.5, 0)
	connect(path[cut-1], half)
	connect(half, path[cut])
	path = append(append(append([]*placed{}, path[:cut]...), half), path[cut:]...)
	route := &transportRoute{path: path, sharedPath: append([]*placed{}, path...)}
	b.transportRoutes = []*transportRoute{route}
	layout := &SurfaceLayout{Mode: "autoTropic", GridSegment: 200, ExpandAdjacent: true}
	encoding, err := b.prepareConnectedSurfaceEncoding(context.Background(), layout)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoding.areas) < 2 {
		t.Fatal("测试必须真实使用多个原生 Area")
	}
	if len(route.path) <= len(path) || len(b.all) != len(route.path) {
		t.Fatal("提前切分留下的空行必须由真实建筑补带，并同步进入路线证据")
	}
	if !reflect.DeepEqual(route.path, route.sharedPath) {
		t.Fatal("重接后的共享带容量证据必须包含同一批新增实体")
	}
	for i := 1; i < len(route.path); i++ {
		if route.path[i-1].output != route.path[i] {
			t.Fatalf("重排后的路线证据在第 %d 条原带边中断", i)
		}
	}
	records, _, _, err := b.records()
	if err != nil {
		t.Fatal(err)
	}
	if err := encoding.applyRecords(b.all, records); err != nil {
		t.Fatal(err)
	}
	crossings := 0
	for _, record := range records {
		if record.OutputObjIdx >= 0 && record.AreaIndex != records[record.OutputObjIdx].AreaIndex {
			crossings++
		}
	}
	if crossings < 1 {
		t.Fatal("应保留跨 Area 的真实有向带边")
	}
	if _, err := validateFinalPlanetRules(context.Background(), Request{Mode: 1, Setting: b.setting}, encoding, records); err != nil {
		t.Fatal(err)
	}
	document := blueprint.Document{EnvelopeVersion: 1, BinaryVersion: 2, Patch: 1, Name: "跨纬带连接图回归", GameVersion: "0.10.34.28524", PrimaryAreaIndex: int32(encoding.plan.PrimaryAreaIndex), CursorTargetArea: int32(encoding.plan.PrimaryAreaIndex), DragBoxSize: blueprint.PointInt{X: int32(encoding.plan.LayoutWidth), Y: int32(encoding.plan.LayoutHeight)}, Areas: encoding.areas, Buildings: records}
	encoded, err := blueprint.Encode(document, 1)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := blueprint.Decode(encoded.Blueprint, true)
	if err != nil || len(decoded.Buildings) != len(records) {
		t.Fatalf("多 Area 严格往返失败: %v", err)
	}
	t.Logf("areas=%d original=%d actual=%d crossAreaEdges=%d", len(encoding.areas), len(path), len(route.path), crossings)
}

// TestConnectedSurfaceFailureRestoresGraph 职责：拒绝不可拆分设施连接且保持原图位置和引用；使用：失败关闭与取消回归；边界：不通过清空路线规避错误。
func TestConnectedSurfaceFailureRestoresGraph(t *testing.T) {
	noPower := false
	b, err := newBuilder(Setting{BeltLv: 3, InserterLv: 3, NeedPower: &noPower})
	if err != nil {
		t.Fatal(err)
	}
	path := b.straight(0, 0, 301, 1)
	route := &transportRoute{path: path}
	b.transportRoutes = []*transportRoute{route}
	// 非带连接跨越全部纬向范围，没有任何合法切面。
	facility := b.make(2101, 2, 0, 0)
	facility.output = b.make(2101, 2, 300, 0)
	shift(b.all, 17, -23, 0)
	original := append([]*placed(nil), b.all...)
	states := make([]placed, len(original))
	for i, p := range original {
		states[i] = *p
	}
	if _, err := b.prepareConnectedSurfaceEncoding(context.Background(), &SurfaceLayout{Mode: "autoTropic", GridSegment: 200, ExpandAdjacent: true}); err == nil {
		t.Fatal("不可拆分的设施连接跨越整圈时必须失败")
	}
	if !reflect.DeepEqual(original, b.all) || !reflect.DeepEqual(route.path, path) {
		t.Fatal("失败后建筑集合或路线证据被修改")
	}
	for i, p := range original {
		if !reflect.DeepEqual(*p, states[i]) {
			t.Fatalf("失败后建筑 %d 的位置或连接被修改", i)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := b.prepareConnectedSurfaceEncoding(ctx, &SurfaceLayout{Mode: "autoTropic", GridSegment: 200, ExpandAdjacent: true}); err == nil {
		t.Fatal("取消后不得提交跨纬带候选")
	}
}

// TestConnectedSurfaceRuleFailureRollsBackInsertedBelts 职责：最终记录规则失败时回滚已经创建的接驳和原始引用；使用：拒绝未知模型的原子性回归；边界：不把未执行到编码的候选当作成功。
func TestConnectedSurfaceRuleFailureRollsBackInsertedBelts(t *testing.T) {
	noPower := false
	b, err := newBuilder(Setting{BeltLv: 3, InserterLv: 3, NeedPower: &noPower})
	if err != nil {
		t.Fatal(err)
	}
	path := b.straight(0, 0, 301, 1)
	cuts := make([]bool, 302)
	for i := range cuts {
		cuts[i] = true
	}
	chunks, ok := planConnectedSurfaceChunks(surfaceLatitudeBands(200), cuts, -301/2, 11, 301)
	if !ok {
		t.Fatal("回滚样本无法规划原生纬带")
	}
	cut := chunks[0].sourceHigh
	half := b.make(2003, 0, float64(cut)-.5, 0)
	connect(path[cut-1], half)
	connect(half, path[cut])
	path = append(append(append([]*placed{}, path[:cut]...), half), path[cut:]...)
	b.make(9999, 10, 0, 0)
	shift(b.all, 17, -23, 0)
	route := &transportRoute{path: path}
	b.transportRoutes = []*transportRoute{route}
	states := make([]placed, len(b.all))
	for i, p := range b.all {
		states[i] = *p
	}
	if _, err := b.prepareConnectedSurfaceEncoding(context.Background(), &SurfaceLayout{Mode: "autoTropic", GridSegment: 200, ExpandAdjacent: true}); err == nil {
		t.Fatal("未知原型不得通过最终规则")
	}
	if len(b.all) != len(states) || !reflect.DeepEqual(route.path, path) {
		t.Fatal("规则失败后保留了候选接驳或更改了路线")
	}
	for i, p := range b.all {
		if !reflect.DeepEqual(*p, states[i]) {
			t.Fatalf("最终规则失败后建筑 %d 未恢复", i)
		}
	}
}

// TestConnectedSurfaceRealProductionGraph 职责：以真实钢材链证明生产者到消费者的物流边跨 Area 后仍连接；使用：锁定量化输入与显式布局设置的端到端回归；边界：关闭供电只影响覆盖状态，其余容量、碰撞、连接和严格编码全部执行。
func TestConnectedSurfaceRealProductionGraph(t *testing.T) {
	fixture := loadMatrixFixtures(t)[0]
	fixture.Request.Targets = []quant.Target{{ItemID: 1103, RatePerMinute: 900}}
	fixture.Setting.MaxMachineInALine = 1
	fixture.Setting.SmelterMaxCount = 1
	fixture.Setting.BlockWidthLimit = 200
	noPower := false
	fixture.Setting.NeedPower = &noPower
	plan, err := quant.Solve(context.Background(), fixture.Request)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Generate(context.Background(), Request{Plan: plan, Mode: 1, Setting: fixture.Setting, Name: "vanilla-steel-900-multi-area", SurfaceLayout: &SurfaceLayout{Mode: "autoTropic", GridSegment: 200, ExpandAdjacent: true}})
	if err != nil {
		t.Fatal(err)
	}
	assertGeneratedTransport(t, plan, 1, 3, out)
	if out.Areas < 2 || out.LogisticsSummary.Metrics.CrossAreaConnections == 0 {
		t.Fatal("真实生产链必须使用多个原生 Area 并存在跨区带连接")
	}
	decoded, err := blueprint.Decode(out.Blueprint, true)
	if err != nil {
		t.Fatal(err)
	}
	crossAreaInternal := 0
	for _, lane := range out.LogisticsSummary.Lanes {
		if lane.FlowKind != "internal" {
			continue
		}
		members := map[int]bool{}
		for _, index := range lane.BuildingIndices {
			members[index] = true
		}
		for _, index := range lane.BuildingIndices {
			record := decoded.Buildings[index]
			next := int(record.OutputObjIdx)
			if next >= 0 && members[next] && surfaceIsBelt(record.ItemID) && surfaceIsBelt(decoded.Buildings[next].ItemID) && record.AreaIndex != decoded.Buildings[next].AreaIndex {
				crossAreaInternal++
			}
		}
	}
	if crossAreaInternal == 0 {
		t.Fatal("样本必须证明规范内部物料边跨区，不能只跨外部原料带")
	}
	t.Logf("真实钢材900/min：productions=%d machines=%d buildings=%d areas=%d internalCrossAreaEdges=%d", len(plan.Productions), plan.MachineCount, out.BuildingCount, out.Areas, crossAreaInternal)
}
