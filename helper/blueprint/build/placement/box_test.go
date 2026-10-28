package placement

// 文件职责：用独立几何反例验证定向盒窄相位。
// 使用：自动回归；边界：不把数学相交定义等同于 PhysX 接触容差。

import (
	"math"
	"testing"
)

// TestBoxSeparatingAxes 职责：覆盖体积交叠、相切、高度净空和轴对齐宽相位假阳性；使用：碰撞算法回归；边界：均使用有效正交盒。
func TestBoxSeparatingAxes(t *testing.T) {
	axes := [3]Vector{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}
	box := Box{HalfSize: Vector{1, 1, 1}, Axes: axes}
	for _, tc := range []struct {
		name    string
		center  Vector
		overlap bool
	}{
		{name: "same-position", overlap: true},
		{name: "positive-volume", center: Vector{1.999, 0, 0}, overlap: true},
		{name: "tangent", center: Vector{2, 0, 0}},
		{name: "vertical-clearance", center: Vector{0, 2.01, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			other := box
			other.Center = tc.center
			if got := box.Overlaps(other); got != tc.overlap || got != other.Overlaps(box) {
				t.Fatalf("overlap=%v, want=%v", got, tc.overlap)
			}
		})
	}
	s := math.Sqrt(.5)
	left := Box{HalfSize: Vector{2, .1, .1}, Axes: [3]Vector{{s, s, 0}, {-s, s, 0}, {0, 0, 1}}}
	right := left
	right.Center = Vector{-.3 * s, .3 * s, 0}
	leftLow, leftHigh := left.Bounds()
	rightLow, rightHigh := right.Bounds()
	for dimension := 0; dimension < 3; dimension++ {
		if leftLow[dimension] >= rightHigh[dimension] || rightLow[dimension] >= leftHigh[dimension] {
			t.Fatal("fixture must overlap in the broad phase")
		}
	}
	if left.Overlaps(right) || right.Overlaps(left) {
		t.Fatal("parallel rotated boxes separated across their narrow faces were reported as colliding")
	}
	left.Center, right.Center = left.Center.Add(Vector{300, -50, 200}), right.Center.Add(Vector{300, -50, 200})
	if left.Overlaps(right) {
		t.Fatal("rigid translation changed the collision result")
	}
}

// TestBoxRejectsInvalidGeometry 职责：保护失败关闭的输入边界；使用：原型数值校验；边界：未知尺寸不能当作零占位。
func TestBoxRejectsInvalidGeometry(t *testing.T) {
	box := Box{HalfSize: Vector{1, 1, 1}, Axes: [3]Vector{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}}
	if !box.Valid() {
		t.Fatal("unit box was rejected")
	}
	invalid := box
	invalid.HalfSize[0] = 0
	if invalid.Valid() {
		t.Fatal("zero extent was accepted")
	}
	invalid = box
	invalid.Axes[1] = invalid.Axes[0]
	if invalid.Valid() {
		t.Fatal("nonorthogonal axes were accepted")
	}
	invalid = box
	invalid.Center[2] = math.Inf(1)
	if invalid.Valid() {
		t.Fatal("infinite center was accepted")
	}
}
