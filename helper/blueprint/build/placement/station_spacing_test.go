package placement

// 文件职责：核对大型采矿机的原版站距。使用：选址和整图终检回归。边界：模型碰撞由独立规则处理。

import "testing"

// TestVeinCollectorSpacing 区分单端新建、同批双端以及不参与专用站距的设备。
func TestVeinCollectorSpacing(t *testing.T) {
	for _, tc := range []struct {
		new, other int16
		together   bool
		squared    float32
	}{
		{2104, 2316, false, 841}, {2316, 2104, false, 841},
		{2103, 2316, false, 225}, {2316, 2103, false, 625},
		{2103, 2316, true, 625}, {2316, 2103, true, 625},
		{2316, 2316, true, 0}, {2003, 2316, false, 0}, {2104, 2301, false, 0},
	} {
		if got := StationSpacingSquared(tc.new, tc.other, tc.together); got != tc.squared {
			t.Errorf("新设施 %d 与 %d，同批=%t，间距平方=%g，期望 %g", tc.new, tc.other, tc.together, got, tc.squared)
		}
	}
}
