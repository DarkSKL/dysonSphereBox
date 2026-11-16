package build

// 文件职责：复现合法节点之间的带段横穿未来入口；使用：通道预留回归；边界：节点检查仍作为独立前置，不能只靠新增测试自身构造预期。
import (
	"testing"
)

func TestReservedSpanProtectsBetweenNodes(t *testing.T) {
	owner, other := &placed{a: vector{-4, 10, 2}}, &placed{}
	env := &transportEnvironment{reserved: map[vector]map[*placed]bool{}, densePorts: &denseRoutePortIndex{}}
	env.reserveTransportSpan(vector{-4, 10, 2}, vector{-3, 10, 2}, owner)
	env.reserveTransportSpan(vector{-3, 10, 2}, vector{-2, 10, 2}, owner)
	first, second := vector{-3.5, 9, 2}, vector{-3.5, 11, 2}
	if env.reservedByOther(first, other, nil) || env.reservedByOther(second, other, nil) {
		t.Fatal("fixture endpoints are already reserved")
	}
	if env.reservedSpanClear(first, second, other, nil) {
		t.Fatal("foreign belt crossed between reserved nodes")
	}
	if !env.reservedSpanClear(first, second, owner, nil) {
		t.Fatal("owner lost permission")
	}
	if !env.reservedSpanClear(vector{-4, 11, 2}, vector{-2, 11, 2}, other, nil) {
		t.Fatal("parallel neighboring row blocked")
	}
	if !env.reservedSpanClear(vector{-3.5, 9, 8}, vector{-3.5, 11, 8}, other, nil) {
		t.Fatal("separate high route blocked")
	}
}
