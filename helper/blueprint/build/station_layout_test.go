package build

// 文件职责：回归物流塔多产物过滤端口的占位登记、路线交叉与按真实米制阈值补齐净空。
// 使用：站点局部装配和装箱规则测试；边界：纯数值证据，不替代 Unity 场景或物流机队产能实测。

import (
	"math"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// TestStationOutputRegistersIndependentBlockSlice 职责：防止新增过滤端口漏出占位或覆写其它产线；使用：多产物回归；边界：只核验实体登记和物品过滤，不模拟抓取时序。
func TestStationOutputRegistersIndependentBlockSlice(t *testing.T) {
	s, err := normalizeSetting(Setting{BeltLv: 3, InserterLv: 3})
	if err != nil {
		t.Fatal(err)
	}
	b, err := newBuilder(s)
	if err != nil {
		t.Fatal(err)
	}
	b.owner = 0
	source := b.make(2003, 0, 0, 0)
	machine := b.make(2308, 10, 10, 0)
	l := &line{all: b.all[:2], output: []*placed{source}, machines: []*placed{machine}, info: quant.Production{MachineCount: 1}}
	b.owner = 1
	other := b.make(2302, 20, 20, 0)
	branch, err := b.stationOutputHead(l, 1120, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.all) != 4 || len(b.all) != 5 || b.all[2] != other {
		t.Fatalf("过滤端口必须独立登记，不能覆盖后续产线：local=%d global=%d other=%p want=%p", len(l.all), len(b.all), b.all[2], other)
	}
	if l.all[2] != branch || branch.input == nil || l.all[3] != branch.input || branch.input.input != source || branch.input.output != branch || branch.input.record.FilterID != 1120 {
		t.Fatal("过滤支路没有保留本地占位或正确的逐物品实体接线")
	}
	if branch.owner != machine.owner || branch.input.owner != machine.owner || b.owner != 1 {
		t.Fatal("过滤支路归属必须来自原机器行，同时恢复调用方当前归属")
	}
}

// TestStationOutputAvoidsAdjacentRowBelt 职责：回归低等级拆短行后过滤出口与下一行入口抢占同格；使用：真实矩阵发现问题的局部边界样本；边界：换侧出口仍保留物品过滤和原共享干线。
func TestStationOutputAvoidsAdjacentRowBelt(t *testing.T) {
	setting, err := normalizeSetting(Setting{BeltLv: 1, InserterLv: 3})
	if err != nil {
		t.Fatal(err)
	}
	b, err := newBuilder(setting)
	if err != nil {
		t.Fatal(err)
	}
	source := b.make(2001, 0, 8, 0)
	machine := b.make(2308, 10, 20, 0)
	l := &line{all: []*placed{source, machine}, output: []*placed{source}, machines: []*placed{machine}}
	nextInput := b.make(2001, 0, 9, 0)
	branch, err := b.stationOutputHead(l, 1114, 0, 2, []*placed{source, machine, nextInput})
	if err != nil {
		t.Fatal(err)
	}
	if beltCell(branch.a) == beltCell(nextInput.a) || branch.a.y >= source.a.y {
		t.Fatalf("过滤出口必须选择未占用侧，实际 %+v", branch.a)
	}
	if branch.input == nil || branch.input.input != source || branch.input.output != branch || branch.input.record.FilterID != 1114 || source.output != nil {
		t.Fatal("换侧不能改变物品过滤或截断共享干线")
	}
}

// TestStationPathCrossesAxesAndHead 职责：核对正交、共线和头部第一段的未连接交叉；使用：平面候选预检；边界：不同高度的路线允许经过同一平面投影。
func TestStationPathCrossesAxesAndHead(t *testing.T) {
	previous := []stationRoute{{head: &placed{a: vector{0, 0, 0}}, path: []vector{{2, 0, 0}, {3, 0, 0}}}}
	for _, test := range []struct {
		name string
		path []vector
		want bool
	}{
		{"head-segment", []vector{{1, -1, 0}, {1, 1, 0}}, true},
		{"horizontal-through-vertical", []vector{{2, -1, 0}, {2, 1, 0}}, true},
		{"collinear-overlap", []vector{{1, 0, 0}, {4, 0, 0}}, true},
		{"separate", []vector{{1, -2, 0}, {1, -1, 0}}, false},
		{"bridge", []vector{{1, -1, 1}, {1, 1, 1}}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := stationPathCrosses(test.path, previous); got != test.want {
				t.Fatalf("cross=%t want=%t", got, test.want)
			}
		})
	}
	if !stationPathCrosses([]vector{{0, 1, 0}, {3, 1, 0}}, []stationRoute{{path: []vector{{1, 0, 0}, {1, 2, 0}}}}) {
		t.Fatal("水平候选与竖直既有路线交叉未拒绝")
	}
}

// TestStationBlockClearanceUsesPairSpecificIntervals 职责：证明沿正 Y 平移真正离开所有塔距离禁区；使用：装箱净空回归；边界：阈值来自固定构建，float32舍入保留小余量。
func TestStationBlockClearanceUsesPairSpecificIntervals(t *testing.T) {
	tower := func(id int16, x, y float64) *placed {
		return &placed{record: blueprint.Building{ItemID: id}, a: vector{x, y, 0}}
	}
	for _, test := range []struct {
		name     string
		existing []*placed
		block    []*placed
		minShift float64
		maxShift float64
	}{
		{"stellar-diagonal", []*placed{tower(2104, 0, 0)}, []*placed{tower(2104, 20, 0)}, 11.467, 11.471},
		{"planetary-center", []*placed{tower(2103, 0, 0)}, []*placed{tower(2103, 0, 0)}, 11.924, 11.928},
		{"reverse-y", []*placed{tower(2104, 0, 0)}, []*placed{tower(2104, 0, -10)}, 33.054, 33.058},
		{"safe-horizontal", []*placed{tower(2104, 0, 0)}, []*placed{tower(2104, 24, 0)}, 0, 0},
		{"multiple-intervals", []*placed{tower(2104, 0, 0), tower(2104, 0, 30)}, []*placed{tower(2104, 0, 0)}, 53.054, 53.059},
	} {
		t.Run(test.name, func(t *testing.T) {
			shift := stationBlockClearance(test.existing, test.block, 0, 0)
			if shift < test.minShift || shift > test.maxShift {
				t.Fatalf("shift=%g want[%g,%g]", shift, test.minShift, test.maxShift)
			}
			for _, left := range test.existing {
				for _, right := range test.block {
					x := (right.a.x - left.a.x) * planarGridLength
					y := (right.a.y + shift - left.a.y) * planarGridLength
					if x*x+y*y < float64(surfaceStationDistanceLimit(left.record.ItemID, right.record.ItemID))-1e-6 {
						t.Fatal("补齐净空后仍落在真实米制塔间距禁区")
					}
				}
			}
		})
	}
	if got := stationBlockClearance([]*placed{tower(2103, 0, 0)}, []*placed{tower(2103, 0, 0)}, 0, math.Sqrt(225)/planarGridLength); got != 0 {
		t.Fatalf("刚好阈值不应增加留白: %g", got)
	}
}
