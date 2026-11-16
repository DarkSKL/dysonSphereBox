package build

// 文件职责：防止普通原生装箱遗漏带段和喷涂通道的真实占位。
// 使用：case21共用修复的针对性回归；边界：只构造几何反例，整图验收由原始请求重放执行。

import (
	"context"
	"math"
	"testing"
)

// TestCase21PolarCrossUnitSpans 职责：证明节点彼此分离时仍须避让跨单元交叉带段；使用：装箱回归；边界：最终按同一普通带规则检查，不能通过减小净距放行。
func TestCase21PolarCrossUnitSpans(t *testing.T) {
	b := &builder{models: map[int]int16{2001: 35}}
	// 真实普通策略由既有规范入口提供；几何反例无需量化、供电或数据库。
	policy, err := newAssemblyPolicy(3, densityStandard)
	if err != nil {
		t.Fatal(err)
	}
	b.policy = policy
	b.polarFrame = &densePolarFrame{start: 0, height: 20, segments: 200, latitudeStep: float32(math.Pi / 500), longitudeStep: float32(math.Pi / 500)}
	a, c := b.make(2001, 3, 3, 0), b.make(2001, 5, 5, 0)
	connect(a, c)
	d, e := b.make(2001, 3, 5, 0), b.make(2001, 5, 3, 0)
	connect(d, e)
	units := []denseSpatialUnit{{entities: []*placed{a, c}}, {entities: []*placed{d, e}}}
	if err = b.validateNativeUnitBelts(context.Background(), denseSpatialUnit{entities: b.all}); err == nil {
		t.Fatal("几何反例应被普通带段规则拒绝")
	}
	if _, err = b.packPolarUnitGeometry(context.Background(), units, false); err != nil {
		t.Fatal(err)
	}
	if err = b.validateNativeUnitBelts(context.Background(), denseSpatialUnit{entities: b.all}); err != nil {
		t.Fatalf("装箱后普通带段仍冲突: %v", err)
	}
}

// TestCase21NativeAddonPreflight 职责：使无连接的侵入喷涂机带节点在纬向搜索阶段即失败；使用：附加通道回归；边界：使用真实原型盒与正式AddonPass规则。
func TestCase21NativeAddonPreflight(t *testing.T) {
	b := &builder{models: map[int]int16{2001: 35, 2313: 120}}
	b.polarFrame = &densePolarFrame{start: 0, height: 20, segments: 200, latitudeStep: float32(math.Pi / 500), longitudeStep: float32(math.Pi / 500)}
	coater := b.make(2313, 8, 8, 0)
	coater.record.Yaw = 270
	belt := b.make(2001, 8.5, 8, 1)
	unit := denseSpatialUnit{entities: b.all}
	if err := b.validateNativeUnitBelts(context.Background(), unit); err == nil {
		t.Fatal("偏离供剂通道的带节点应在装箱前被拒绝")
	}
	belt.a.x = 12
	if err := b.validateNativeUnitBelts(context.Background(), unit); err != nil {
		t.Fatalf("移出建造盒后仍被误拒: %v", err)
	}
}
