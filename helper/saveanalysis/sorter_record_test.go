package saveanalysis

// 文件职责：验证解析器保留的分拣器投放观测。
// 使用：运行解析器回归测试；边界：只验证直接带口条件，不覆盖已移除的产线诊断。

import "testing"

// TestDropRefusal 职责：验证普通投放点拒收条件；使用：解析器回归；边界：存在空位时保留未知。
func TestDropRefusal(t *testing.T) {
	buf := make([]byte, 20)
	for i := 5; i <= 9; i++ {
		buf[i] = 1
	}
	if got := dropRefusal(buf, 4); got != "occupied-window" {
		t.Fatal(got)
	}
	buf[5] = 0
	if got := dropRefusal(buf, 4); got != "path-boundary" {
		t.Fatal(got)
	}
	clear(buf)
	for _, pos := range []int64{-1, 0, 4, 14, 18} {
		if got := dropRefusal(buf, pos); got != "" {
			t.Fatal(pos, got)
		}
	}
}
