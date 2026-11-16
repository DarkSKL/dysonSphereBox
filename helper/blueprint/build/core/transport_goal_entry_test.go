package build

// 文件职责：验证封闭目标在展开前被证明不可达，同时保留合法入边与同点起终点；使用：共享寻路优化回归。
import (
	"context"
	"testing"
)

func TestTransportGoalEntryPreflight(t *testing.T) {
	env := &transportEnvironment{minimum: vector{}, maximum: vector{20, 20, transportMaxHeight}, blocked: map[vector]bool{}, reserved: map[vector]map[*placed]bool{}}
	start, goal := vector{1, 1, 0}, vector{10, 10, 1}
	for _, d := range transportNeighborOffsets {
		env.blocked[vector{goal.x + d.x, goal.y + d.y, goal.z + d.z}] = true
	}
	path, _, expanded := env.findTransportDestinationPathWeighted(context.Background(), start, []transportDestination{{point: goal}}, nil, nil, 1)
	if len(path) != 0 || expanded != 0 {
		t.Fatalf("sealed goal expanded %d", expanded)
	}
	delete(env.blocked, vector{9, 10, 1})
	path, _, expanded = env.findTransportDestinationPathWeighted(context.Background(), start, []transportDestination{{point: goal}}, nil, nil, 1)
	if len(path) == 0 || path[len(path)-1] != goal {
		t.Fatalf("legal entry lost, expanded%d", expanded)
	}
	env.blocked[goal] = true
	path, _, _ = env.findTransportDestinationPathWeighted(context.Background(), goal, []transportDestination{{point: goal, reused: &placed{a: goal}}}, nil, nil, 1)
	if len(path) != 1 {
		t.Fatal("same-point reused goal lost")
	}
}
