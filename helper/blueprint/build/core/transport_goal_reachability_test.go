package build

// 文件职责：独立证明目标短通道预检不会误删可达路线；使用：随机BFS、冻结A*和封闭通道基准；边界：不替代最终蓝图规则。

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"testing"
)

// goalTestInside 职责：提供原整数主路的边界与预留许可；使用：预检单独验证；边界：不省略z轴边界。
func goalTestInside(env *transportEnvironment) func(vector) bool {
	return func(p vector) bool {
		return p.x >= env.minimum.x && p.x <= env.maximum.x && p.y >= env.minimum.y && p.y <= env.maximum.y && p.z >= 0 && p.z <= transportMaxHeight && !env.reservedByOther(p, nil, nil)
	}
}

// goalTestEnv 职责：建立独立普通整数网格；使用：随机与封闭通道；边界：不构造几何碰撞豁免。
func goalTestEnv(size int) *transportEnvironment {
	return &transportEnvironment{minimum: vector{}, maximum: vector{float64(size), float64(size), transportMaxHeight}, blocked: map[vector]bool{}, reserved: map[vector]map[*placed]bool{}}
}

// goalTestSeal 职责：仅保留给定通道内的邻接；使用：验证有一个入边仍不可达的反例；边界：通道外更远空间保持开放。
func goalTestSeal(env *transportEnvironment, cells []vector) {
	kept := map[vector]bool{}
	for _, v := range cells {
		kept[v] = true
	}
	for _, v := range cells {
		for _, d := range []vector{{1, 0, 0}, {-1, 0, 0}, {0, 1, 0}, {0, -1, 0}, {0, 0, 1}, {0, 0, -1}} {
			p := vector{v.x + d.x, v.y + d.y, v.z + d.z}
			if !kept[p] {
				env.blocked[p] = true
			}
		}
	}
}

// TestTransportGoalClosedCorridor 职责：排除两格死路、保持打开出口和起点例外；使用：真实A*入口；边界：占位变化必须即时生效。
func TestTransportGoalClosedCorridor(t *testing.T) {
	env := goalTestEnv(40)
	start, goal, next := vector{1, 1, 0}, vector{20, 20, 0}, vector{21, 20, 0}
	goalTestSeal(env, []vector{goal, next})
	if !frozenGoalEntryPossible(env, start, goal, nil, nil, goalTestInside(env)) {
		t.Fatal("fixture lacks old legal entrance")
	}
	path, _, steps := env.findTransportDestinationPathWeighted(context.Background(), start, []transportDestination{{point: goal}}, nil, nil, 1)
	if len(path) != 0 || steps != 0 {
		t.Fatalf("closed corridor expanded %d", steps)
	}
	delete(env.blocked, vector{22, 20, 0})
	path, _, _ = env.findTransportDestinationPathWeighted(context.Background(), start, []transportDestination{{point: goal}}, nil, nil, 1)
	if len(path) == 0 || path[len(path)-1] != goal {
		t.Fatal("opening did not invalidate dead end")
	}
	goalTestSeal(env, []vector{goal, next})
	env.blocked[next] = true
	path, _, _ = env.findTransportDestinationPathWeighted(context.Background(), next, []transportDestination{{point: goal}}, nil, nil, 1)
	if len(path) != 2 {
		t.Fatal("occupied source exception lost")
	}
	long := goalTestEnv(100)
	cells := make([]vector, 70)
	for i := range cells {
		cells[i] = vector{float64(i + 10), 50, 0}
	}
	goalTestSeal(long, cells)
	if !long.transportGoalHasApproach(context.Background(), start, cells[0], nil, nil, goalTestInside(long)) {
		t.Fatal("budget exhaustion treated as proof")
	}
}

// TestTransportGoalRandomReachability 职责：以独立BFS检查2000个固定种子网格的必要性，并对可达样本比较冻结A*代价；使用：防止反向遍历引入误剪。
func TestTransportGoalRandomReachability(t *testing.T) {
	rng := rand.New(rand.NewSource(20261116))
	for trial := 0; trial < 2000; trial++ {
		env := goalTestEnv(5)
		for x := 0; x <= 5; x++ {
			for y := 0; y <= 5; y++ {
				for z := 0; z <= int(transportMaxHeight); z++ {
					if z > 2 || rng.Float64() < .6 {
						env.blocked[vector{float64(x), float64(y), float64(z)}] = true
					}
				}
			}
		}
		start := vector{float64(rng.Intn(6)), float64(rng.Intn(6)), float64(rng.Intn(3))}
		goal := vector{float64(rng.Intn(6)), float64(rng.Intn(6)), float64(rng.Intn(3))}
		delete(env.blocked, start)
		delete(env.blocked, goal)
		inside := goalTestInside(env)
		seen := map[vector]bool{start: true}
		queue := []vector{start}
		for head := 0; head < len(queue); head++ {
			p := queue[head]
			for _, d := range []vector{{1, 0, 0}, {-1, 0, 0}, {0, 1, 0}, {0, -1, 0}, {0, 0, 1}, {0, 0, -1}} {
				n := vector{p.x + d.x, p.y + d.y, p.z + d.z}
				if inside(n) && !seen[n] && !env.blocked[n] {
					seen[n] = true
					queue = append(queue, n)
				}
			}
		}
		possible := env.transportGoalHasApproach(context.Background(), start, goal, nil, nil, inside)
		if seen[goal] && !possible {
			t.Fatalf("reachable goal removed trial%d", trial)
		}
		if trial < 200 {
			goals := []transportDestination{{point: goal}}
			before, _, _ := env.frozenGoalEntryPathWeighted(context.Background(), start, goals, nil, nil, 1)
			after, _, _ := env.findTransportDestinationPathWeighted(context.Background(), start, goals, nil, nil, 1)
			cost := func(path []vector) float64 {
				v := 0.
				for i := 1; i < len(path); i++ {
					v += env.transportStepCost(path[i-1], path[i])
				}
				return v
			}
			if (len(before) > 0) != seen[goal] || (len(after) > 0) != seen[goal] || math.Abs(cost(before)-cost(after)) > 1e-9 {
				t.Fatalf("route outcome changed trial%d", trial)
			}
		}
	}
}

// BenchmarkTransportClosedCorridor 职责：比较相同大网格封闭两格端口的重复搜索成本；使用：冻结单入边与新必要预检；边界：是局部反例，不代表整厂耗时。
func BenchmarkTransportClosedCorridor(b *testing.B) {
	for _, old := range []bool{true, false} {
		b.Run(fmt.Sprint(old), func(b *testing.B) {
			env := goalTestEnv(40)
			start, goal := vector{1, 1, 0}, vector{20, 20, 0}
			goalTestSeal(env, []vector{goal, {21, 20, 0}})
			goals := []transportDestination{{point: goal}}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var path []vector
				if old {
					path, _, _ = env.frozenGoalEntryPathWeighted(context.Background(), start, goals, nil, nil, 1)
				} else {
					path, _, _ = env.findTransportDestinationPathWeighted(context.Background(), start, goals, nil, nil, 1)
				}
				if len(path) != 0 {
					b.Fatal("sealed corridor reached")
				}
			}
		})
	}
}
