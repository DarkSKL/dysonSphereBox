package build

// 文件职责：独立逐格验证保守运动剪枝没有越过任何可行采样；使用：本轮性能回归；边界：随机三维盒、近共面盒和世界轴旋转均使用原Overlaps作为真源。
import (
	"DysonSphereBox/helper/blueprint/build/placement"
	"math"
	"math/rand"
	"testing"
)

func TestPolarMotionBoundNeverSkipsFreeStep(t *testing.T) {
	rng := rand.New(rand.NewSource(20261116))
	step := 2 * math.Pi / 1000 / 4
	checked := 0
	box := func(center placement.Vector) placement.Box {
		yaw, tilt := rng.Float64()*6.28, rng.Float64()*1.1
		if rng.Intn(5) == 0 {
			yaw = 0
			tilt = 0
		}
		sy, cy := math.Sincos(yaw)
		st, ct := math.Sincos(tilt)
		return placement.Box{Center: center, HalfSize: placement.Vector{.1 + rng.Float64()*3, .1 + rng.Float64()*4, .1 + rng.Float64()*3}, Axes: [3]placement.Vector{{cy, 0, sy}, {-sy * st, ct, cy * st}, {-sy * ct, -st, cy * ct}}}
	}
	for sample := 0; sample < 30000; sample++ {
		angle := rng.Float64() * 6.28
		a := box(placement.Vector{200 * math.Cos(angle), rng.Float64()*200 - 100, 200 * math.Sin(angle)})
		b := box(a.Center.Add(placement.Vector{rng.Float64()*6 - 3, rng.Float64()*6 - 3, rng.Float64()*6 - 3}))
		steps := polarBoxBlockedSteps(a, b, step)
		for k := 1; k <= steps; k++ {
			s, c := math.Sincos(float64(k) * step)
			moved := a
			moved.Center = rotatePolarPoint(a.Center, s, c)
			for axis := range moved.Axes {
				moved.Axes[axis] = rotatePolarPoint(a.Axes[axis], s, c)
			}
			if !moved.Overlaps(b) {
				t.Fatalf("skipped free sample=%d step=%d/%d margin=%.12f", sample, k, steps, a.OverlapMargin(b))
			}
			checked++
		}
	}
	if checked < 1000 {
		t.Fatalf("insufficient nonempty checks %d", checked)
	}
	t.Logf("verified %d skipped positions", checked)
}
