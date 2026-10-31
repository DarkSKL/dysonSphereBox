package build

// 文件职责：用同一真实综合计划核对配送模式的三等级实体容量矩阵。
// 使用：全局物流验收；边界：保持计划和设备不变，不把配送机队运行当作已验证。

import (
	"context"
	"fmt"
	"testing"

	"DysonSphereBox/helper/quant"
)

// TestFlyRealBeltCapacityMatrix 职责：要求低带等级真实拆行并保持规范逐边流量和端口账；使用：mode2/5 综合目标回归；边界：实际带等级从最终编码记录独立核对，不只检查摘要。
func TestFlyRealBeltCapacityMatrix(t *testing.T) {
	fixtures := loadMatrixFixtures(t)
	fixture := fixtures[len(fixtures)-1]
	plan, err := quant.Solve(context.Background(), fixture.Request)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []int{2, 5} {
		counts := [4]int{}
		for _, level := range []int{1, 2, 3} {
			t.Run(fmt.Sprintf("mode%d-Mk%d", mode, level), func(t *testing.T) {
				setting := fixture.Setting
				setting.BeltLv = level
				out, err := Generate(context.Background(), Request{Plan: plan, Mode: mode, Setting: setting, Name: fixture.Name})
				if err != nil {
					t.Fatal(err)
				}
				assertGeneratedTransport(t, plan, mode, level, out)
				counts[level] = len(out.LogisticsSummary.Lanes)
				t.Logf("machines=%d buildings=%d lanes=%d inputs=%d area=%.0f", plan.MachineCount, out.BuildingCount, counts[level], out.LogisticsSummary.Metrics.ExternalInputCount, out.LogisticsSummary.Metrics.EnvelopeArea)
			})
		}
		if counts[1] < counts[2] || counts[2] < counts[3] {
			t.Fatalf("mode%d 降级带反而减少实体路线：%v", mode, counts)
		}
	}
}
