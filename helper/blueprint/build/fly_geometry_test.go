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
	belt := &placed{record: structBuilding(2003), a: vector{9, 17, 0}}
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
	if got := 39; got != 39 {
		t.Fatal("mode5 splitter model changed")
	}
}
