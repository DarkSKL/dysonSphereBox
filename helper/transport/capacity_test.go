package transport

// 文件职责：验证基础带容量与自动 lane 下界；使用：go test 物流容量回归；边界：不把带容量视作分拣器或塔能力。

import (
	"math"
	"testing"
)

// TestCapacityAndAutomaticLanes 职责：核对 4200/min 在三等级下的 12/6/3 路与边界；使用：容量矩阵；边界：不允许偷偷提升带等级。
func TestCapacityAndAutomaticLanes(t *testing.T) {
	for _, test := range []struct {
		level    int
		capacity float64
		count    int
	}{{1, 360, 12}, {2, 720, 6}, {3, 1800, 3}} {
		capacity, err := BeltCapacityPerMinute(test.level)
		if err != nil || capacity != test.capacity {
			t.Fatalf("level %d capacity %g: %v", test.level, capacity, err)
		}
		lanes, err := SplitLanes(4200, test.level)
		if err != nil || len(lanes) != test.count {
			t.Fatalf("level %d lanes %v: %v", test.level, lanes, err)
		}
		total := 0.
		for _, rate := range lanes {
			if rate > capacity || rate <= 0 {
				t.Fatalf("overloaded lane %g > %g", rate, capacity)
			}
			total += rate
		}
		if math.Abs(total-4200) > 1e-8 {
			t.Fatalf("lane rate lost %g", total)
		}
		exact, _ := SplitLanes(capacity, test.level)
		over, _ := SplitLanes(math.Nextafter(capacity, math.Inf(1)), test.level)
		if len(exact) != 1 || len(over) != 2 {
			t.Fatalf("capacity boundary %v / %v", exact, over)
		}
	}
}

// TestCapacityRejectsUnknownAndInvalidRate 职责：保证未知等级和无效流量失败关闭；使用：异常输入矩阵；边界：零速率返回零 lane。
func TestCapacityRejectsUnknownAndInvalidRate(t *testing.T) {
	if _, err := BeltCapacityPerMinute(0); err == nil {
		t.Fatal("unknown level accepted")
	}
	for _, rate := range []float64{-1, math.NaN(), math.Inf(1)} {
		if _, err := SplitLanes(rate, 3); err == nil {
			t.Fatalf("invalid rate accepted: %g", rate)
		}
	}
	if lanes, err := SplitLanes(0, 1); err != nil || len(lanes) != 0 {
		t.Fatalf("zero lanes %v: %v", lanes, err)
	}
}
