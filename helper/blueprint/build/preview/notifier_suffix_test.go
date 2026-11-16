package preview

// 文件职责：对照完整同步验证尾部事务；使用：本轮临时回归；边界：输入覆盖更新、撤销、追加和非法前缀写入。
import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

func TestSyncSuffixMatchesFull(t *testing.T) {
	frame := Frame{CoordinateSystem: LayoutCoordinates}
	states := [2]map[string]Building{{}, {}}
	makeNotifier := func(index int) *Notifier {
		return New(func(_ Frame, ops []Operation) {
			for _, op := range ops {
				if op.Type == Remove {
					delete(states[index], op.ObjectID)
				} else {
					states[index][op.ObjectID] = *op.Building
				}
			}
		})
	}
	full, local := makeNotifier(0), makeNotifier(1)
	full.Sync(Scene{Frame: frame})
	local.Sync(Scene{Frame: frame})
	rng := rand.New(rand.NewSource(20261116))
	objects := []Object{}
	nextID := 0
	for step := 0; step < 500; step++ {
		start := max(0, len(objects)-rng.Intn(12))
		objects = objects[:start]
		for i := 0; i < rng.Intn(16); i++ {
			nextID++
			objects = append(objects, Object{ID: fmt.Sprint(nextID), Building: Building{ItemID: 2001, Index: int32(nextID)}})
		}
		if !local.SyncSuffix(frame, start, objects[start:]) {
			t.Fatalf("valid suffix rejected at %d", step)
		}
		full.Sync(Scene{Frame: frame, Objects: objects})
		if !reflect.DeepEqual(states[0], states[1]) || !reflect.DeepEqual(full.order, local.order) {
			t.Fatalf("state/order mismatch at %d", step)
		}
		if len(objects) > 0 {
			objects[len(objects)-1].Building.ModelIndex++
			start = len(objects) - 1
			if !local.SyncSuffix(frame, start, objects[start:]) {
				t.Fatal("update rejected")
			}
			full.Sync(Scene{Frame: frame, Objects: objects})
			if !reflect.DeepEqual(states[0], states[1]) {
				t.Fatal("update mismatch")
			}
		}
	}
	if len(objects) < 2 {
		t.Fatal("fixture too small")
	}
	before := len(states[1])
	if local.SyncSuffix(frame, 1, objects[:1]) {
		t.Fatal("prefix identity accepted")
	}
	if len(states[1]) != before {
		t.Fatal("rejected input mutated state")
	}
	if local.SyncSuffix(frame, 0, []Object{objects[0], objects[0]}) {
		t.Fatal("duplicate identity accepted")
	}
}
