package build

// 文件职责：验证输出走廊的单层兼容、实际占位拒绝及多份额真实链提交。
// 使用：内存几何回归；边界：不加载记录请求、不运行整厂生成、不写入诊断证据。

import (
	"context"
	"testing"
)

// TestDenseOutputPhysicalCorridorScope 职责：保证没有多层边界时保持原单层交付端点；使用：防止其它case的合流行为回归；边界：不创建额外实体或虚拟预留。
func TestDenseOutputPhysicalCorridorScope(t *testing.T) {
	for _, plan := range []*denseBoundaryLayout{nil, {staggerInputs: false}} {
		endpoint := &placed{}
		route := &transportRoute{end: endpoint}
		group := &mode1DeliveryGroup{head: endpoint, members: []*transportRoute{route}}
		b := &builder{denseBoundary: plan}
		if err := b.prepareDenseOutputCorridors(context.Background(), []*mode1DeliveryGroup{group}); err != nil {
			t.Fatal(err)
		}
		if route.end != endpoint || group.approachSteps != 0 || len(b.all) != 0 {
			t.Fatal("单层交付被输出走廊改变")
		}
	}
}

// TestDenseOutputPhysicalCorridorBlocked 职责：证明固定短带仍拒绝已有独立带节点；使用：物理规则回归；边界：失败候选不会把规范端点改为未完成接入头。
func TestDenseOutputPhysicalCorridorBlocked(t *testing.T) {
	b := &builder{setting: Setting{BeltLv: 1}, denseBoundary: &denseBoundaryLayout{minimumX: -7, levelLimitX: -3.5, maximumX: -.5, minimumY: 0, maximumY: 40, staggerInputs: true}}
	g := &mode1DeliveryGroup{head: b.make(2001, -6, 27, 0), end: b.make(2001, -7, 27, 0)}
	connect(g.head, g.end)
	r := &transportRoute{end: g.head, boundary: g.end}
	g.members = []*transportRoute{r}
	b.make(2001, -3, 27, 0)
	if err := b.prepareDenseOutputCorridors(context.Background(), []*mode1DeliveryGroup{g}); err == nil {
		t.Fatal("固定输出短带穿过已有独立节点")
	}
	if r.end != g.head || r.boundary != g.end || g.approachSteps != 0 {
		t.Fatal("失败候选改写规范交付端点")
	}
}

// TestDenseOutputPhysicalCorridorCommit 职责：核实真实输出短带避开错列上层原料且支持同组多份额提交；使用：剩余产物修复回归；边界：固定地面头、实际出货方向和所有份额链保持不变。
func TestDenseOutputPhysicalCorridorCommit(t *testing.T) {
	b := &builder{setting: Setting{BeltLv: 1}, denseBoundary: &denseBoundaryLayout{handoffX: -4, levelLimitX: -3.5, maximumX: -.5, minimumX: -7, minimumY: 0, maximumY: 40, staggerInputs: true}}
	input := &transportRoute{start: b.make(2001, -7, 27, 2)}
	input.path = []*placed{input.start}
	b.appendDenseInputStem(input)
	g := &mode1DeliveryGroup{head: b.make(2001, -6, 27, 0), end: b.make(2001, -7, 27, 0)}
	connect(g.head, g.end)
	r1, r2 := &transportRoute{end: g.head, boundary: g.end}, &transportRoute{end: g.head, boundary: g.end}
	g.members = []*transportRoute{r1, r2}
	if err := b.prepareDenseOutputCorridors(context.Background(), []*mode1DeliveryGroup{g}); err != nil {
		t.Fatal(err)
	}
	arrival := r1.end
	if arrival != r2.end || arrival.a != (vector{0, 26.5, 12}) || g.approachSteps != 19 || g.head.output != g.end || input.routingEntry.a != (vector{-2, 27, 8}) {
		t.Fatal("真实接入头、原料错列端点或交付尾链改变")
	}
	for i, r := range g.members {
		r.start = b.make(2001, 1+float64(i), 26.5, 12)
		connect(r.start, arrival)
		r.path = []*placed{r.start, arrival}
	}
	b.mode1Deliveries = &mode1DeliveryPlan{groups: []*mode1DeliveryGroup{g}}
	if err := b.commitMode1Deliveries(); err != nil {
		t.Fatal(err)
	}
	for _, r := range g.members {
		if r.end != g.end || len(r.path) != 22 {
			t.Fatal("份额未包含完整真实固定接入短带")
		}
		for i := 1; i < len(r.path); i++ {
			if r.path[i-1].output != r.path[i] {
				t.Fatal("份额实际output图断开")
			}
		}
	}
	broken := arrival.output
	arrival.output = nil
	if _, err := mode1DeliveryApproach(g, arrival, g.head); err == nil {
		t.Fatal("固定接入短带断开未拒绝")
	}
	arrival.output = broken
}
