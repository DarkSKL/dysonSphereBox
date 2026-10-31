package build

// 文件职责：回归配送端点的箱体槽位、分流器模型和供电几何。
// 使用：mode2/5 装配前的纯几何契约；边界：不替代最终球面动态碰撞和游戏内放置。

import (
	"math"
	"testing"

	"DysonSphereBox/helper/blueprint"
)

func structBuilding(item int) blueprint.Building {
	return blueprint.Building{ItemID: int16(item), OutputObjIdx: -1, InputObjIdx: -1, Yaw: 90, Yaw2: 90}
}

func TestStorageSlotPointAndFlyArmGeometry(t *testing.T) {
	box := &placed{record: structBuilding(2101), a: vector{10, 20, 0}}
	box.record.Yaw, box.record.Yaw2 = 0, 0
	belt := &placed{record: structBuilding(2003), a: vector{7, 20, 0}}
	arm := &placed{record: structBuilding(2013)}
	point, ok := storageSlotPoint(box.a, 0, 10)
	if !ok || math.Abs(point.x-9) > 1e-8 || math.Abs(point.y-20) > 1e-8 {
		t.Fatalf("slot10 geometry = %+v/%v", point, ok)
	}
	if err := flyArmGeometry(arm, box, belt, 10, false); err != nil {
		t.Fatal(err)
	}
	if arm.input != belt || arm.output != box || arm.record.OutputToSlot != 10 || arm.record.InputFromSlot != -1 {
		t.Fatalf("output arm direction invalid: %+v", arm)
	}
	if arm.record.Parameters[0] < 1 || arm.record.Parameters[0] > 3 {
		t.Fatalf("arm distance invalid: %+v", arm.record.Parameters)
	}
}

// TestFlyStorageYawAndArmDirection 职责：锁定 Unity 顺时针槽位旋转和供需双向分拣器朝向；使用：防止旧数学角度约定回归；边界：最终端口修正仍由抽象规则验证。
func TestFlyStorageYawAndArmDirection(t *testing.T) {
	point, ok := storageSlotPoint(vector{10, 20, 0}, 90, 10)
	if !ok || math.Abs(point.x-10) > 1e-8 || math.Abs(point.y-21) > 1e-8 {
		t.Fatalf("yaw90 槽位应朝北：%+v", point)
	}
	box := &placed{record: structBuilding(2101), a: vector{10, 20, 0}}
	box.record.Yaw = 0
	belt := &placed{record: structBuilding(2003), a: vector{7, 20, 0}}
	for _, input := range []bool{false, true} {
		arm := &placed{record: structBuilding(2013)}
		if err := flyArmGeometry(arm, box, belt, 10, input); err != nil {
			t.Fatal(err)
		}
		want := float32(90)
		if input {
			want = 270
		}
		if arm.record.Yaw != want || arm.record.Yaw2 != want {
			t.Fatalf("input %v yaw %v want %v", input, arm.record.Yaw, want)
		}
	}
}

func TestFlyArmRejectsOutOfRangeSpan(t *testing.T) {
	box := &placed{record: structBuilding(2101), a: vector{10, 20, 0}}
	belt := &placed{record: structBuilding(2003), a: vector{10, 15, 0}}
	arm := &placed{record: structBuilding(2013)}
	if err := flyArmGeometry(arm, box, belt, 10, false); err == nil {
		t.Fatal("超过三格的箱体分拣器跨度必须失败")
	}
}

func TestEnsureFlyPowerUsesMinimumSpacing(t *testing.T) {
	needPower := true
	b, err := newBuilder(Setting{BeltLv: 3, InserterLv: 3, NeedPower: &needPower})
	if err != nil {
		t.Fatal(err)
	}
	b.ensureFlyPower(vector{0, 0, 0})
	b.ensureFlyPower(vector{1, 1, 0})
	count := 0
	for _, p := range b.all {
		if p.record.ItemID == 2201 {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("近距离端点重复补杆: %d", count)
	}
	b.ensureFlyPower(vector{10, 10, 0})
	if len(b.all) != 2 {
		t.Fatalf("远端端点未补杆: %d", len(b.all))
	}
}

func TestFlyMode5UsesVerifiedSplitterModel(t *testing.T) {
	for _, input := range []bool{false, true} {
		b, err := newBuilder(Setting{BeltLv: 3, InserterLv: 3})
		if err != nil {
			t.Fatal(err)
		}
		link := b.make(2003, 0, 0, 0)
		endpoint, err := b.fly(1101, link, input, 18, 30, 5, 0)
		if err != nil {
			t.Fatal(err)
		}
		b.flyEndpoints = append(b.flyEndpoints, endpoint)
		if endpoint.splitter.record.ModelIndex != 39 {
			t.Fatal("mode5 must retain verified splitter model39")
		}
		if err := b.validateFlyEndpointTopology(5); err != nil {
			t.Fatal(err)
		}
		end := endpoint.belts[len(endpoint.belts)-1]
		if input {
			end.record.InputFromSlot = 0
		} else {
			end.record.OutputToSlot = 0
		}
		if err := b.validateFlyEndpointTopology(5); err == nil {
			t.Fatal("西侧带错误接入东侧槽0必须失败")
		}
	}
}

// TestFlyPathChecksOccupancyBeforeCommit 职责：复现旧接驳交叉并确认候选拒绝没有追加建筑；使用：预提交占位回归；边界：未知静态几何不按空白放行。
func TestFlyPathChecksOccupancyBeforeCommit(t *testing.T) {
	b, err := newBuilder(Setting{BeltLv: 3})
	if err != nil {
		t.Fatal(err)
	}
	link := b.make(2003, 0, 0, 0)
	b.byPath([]vector{{2, -1, 0}, {2, 0, 0}, {2, 1, 0}})
	before := len(b.all)
	if err := b.validateFlyPathCandidate(manhattan(link.a, vector{4, 0, 0}, true), link, nil, b.all); err == nil {
		t.Fatal("已占位物流交叉必须在提交前失败")
	}
	if len(b.all) != before {
		t.Fatal("失败候选不应追加实体")
	}
	unknown := b.make(9999, 0, 4, 0)
	if err := b.validateFlyPathCandidate(manhattan(link.a, vector{4, 0, 0}, true), link, nil, []*placed{unknown}); err == nil {
		t.Fatal("未注册静态几何不得默认为无占位")
	}
}
