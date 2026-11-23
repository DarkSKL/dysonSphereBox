package transport

// 职责：对照原版船运函数的独立数值；使用：运输估算回归；边界：不要求本机安装游戏，不把估算当作实际航路仿真。

import (
	"math"
	"testing"
)

// TestVesselTripNativeGolden 职责：核验原版各飞行阶段的帧数；使用：常速、阈值和跨星回归；边界：普通 200 米星球，中心距离近似两塔距离。
func TestVesselTripNativeGolden(t *testing.T) {
	// 固定值由原 DLL 的 StationComponent.CalcRemoteSingleTripTime 直接运行取得，非本测试重算。
	// DLL SHA256：EF314425DADE51F529F184F07BE136CD3C2660A6A81515603C7226FBE7AB1D8C。
	// 两颗星球半径均 200，旋转为单位四元数，两塔本地位置均为 (0,200,0)，星心沿 X 轴相距 distance；去回 direction 分别为 1/-1。
	tests := []struct {
		name              string
		distance, sail    float64
		threshold         float64
		outbound, inbound int
		warped            bool
	}{
		{"极短常速", 1, 300, 120000, 605, 605, false},
		{"短程加减速", 100, 300, 120000, 1194, 1170, false},
		{"中程未达巡航", 1000, 300, 120000, 1935, 1783, false},
		{"低于曲速阈值", 10000, 300, 120000, 3830, 3686, false},
		{"曲速未启用", 120000, 300, 120001, 25830, 25686, false},
		{"恰达曲速阈值", 120000, 300, 120000, 3940, 3796, true},
		{"典型跨星", 2400000, 300, 120000, 4056, 3912, true},
		{"跨星常速", 2400000, 300, 0, 481830, 481686, false},
		{"升级巡航速度", 2400000, 600, 120000, 2684, 2575, true},
		{"高巡航速度", 2400000, 3000, 120000, 1409, 1356, true},
		{"短程曲速升降", 20000, 6000, 20000, 1036, 995, true},
		{"进站段直接减速", 2400000, 6000, 120000, 1206, 1166, true},
		// 小于原版 UI 最小 20000 米的阈值不能产生负曲速段，按原 DLL canWarp=false 的结果核验。
		{"超短阈值仍需离星", 10000, 300, 1, 3830, 3686, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := VesselProfile{Cargo: 200, CruiseSpeed: tt.sail, WarpSpeed: 1000000, WarpDistance: tt.threshold, TurnaroundSeconds: 10}
			got, warped, err := profile.VesselTrip(tt.distance)
			want := float64(tt.outbound+tt.inbound)/60 + 10
			if err != nil || math.Abs(got-want) > 1e-9 || warped != tt.warped {
				t.Fatalf("VesselTrip = %.12f, %v, %v; 原版往返 %.12f, 曲速 %v", got, warped, err, want, tt.warped)
			}
			for _, leg := range []struct {
				outbound bool
				want     int
			}{{true, tt.outbound}, {false, tt.inbound}} {
				ticks, _ := vesselSingleTripTicks(tt.distance, float32(tt.sail), 1000000, tt.warped, leg.outbound)
				if ticks != float64(leg.want) {
					t.Fatalf("outbound=%v: got %v ticks, want %d", leg.outbound, ticks, leg.want)
				}
			}
		})
	}
}

// TestVesselTripThresholdAndTurnaround 职责：防止阈值重新变成巡航路程；使用：同一远航线对比；边界：周转时间仅额外添加一次。
func TestVesselTripThresholdAndTurnaround(t *testing.T) {
	profile := VesselProfile{Cargo: 200, CruiseSpeed: 300, WarpSpeed: 1000000, WarpDistance: 20000, TurnaroundSeconds: 10}
	first, firstWarp, err := profile.VesselTrip(2400000)
	if err != nil || !firstWarp || math.Abs(first-142.8) > 1e-9 {
		t.Fatalf("最小启用距离: %v, %v, %v", first, firstWarp, err)
	}
	profile.WarpDistance = 480000
	second, secondWarp, err := profile.VesselTrip(2400000)
	if err != nil || !secondWarp || second != first {
		t.Fatalf("同一曲速航线被阈值改变: %v -> %v, %v, %v", first, second, secondWarp, err)
	}
	profile.TurnaroundSeconds = 25
	third, _, err := profile.VesselTrip(2400000)
	if err != nil || math.Abs(third-second-15) > 1e-9 {
		t.Fatalf("周转等待未独立添加: %v -> %v, %v", second, third, err)
	}
	if vessels := math.Ceil(2000 * first / (60 * profile.Cargo)); vessels != 24 {
		t.Fatalf("2000/min 航线船数 = %v, want 24", vessels)
	}
}

// TestVesselTripInvalidParameters 职责：保持无效参数拒绝；使用：新增浮点阶段的错误出口；边界：不裁剪或默默替换玩家输入。
func TestVesselTripInvalidParameters(t *testing.T) {
	profile := VesselProfile{Cargo: 200, CruiseSpeed: 300, WarpSpeed: 1000000, WarpDistance: 120000}
	for _, distance := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if _, _, err := profile.VesselTrip(distance); err == nil {
			t.Fatalf("接受无效航程 %v", distance)
		}
	}
	profile.CruiseSpeed = math.MaxFloat64
	if _, _, err := profile.VesselTrip(2400000); err == nil {
		t.Fatal("接受超出原版单精度速度范围的参数")
	}
}
