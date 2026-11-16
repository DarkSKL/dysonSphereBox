package build

// 文件职责：用原case05核对长行阵组合和完整候选保留；边界：仅准备及枚举，不替代完整Generate验收。
import (
	"reflect"
	"testing"
)

func TestStationCase05LargeRows(t *testing.T) {
	ctx, input := largeDiagnosticInput(t)
	options := DefaultStationSearchOptions()
	original, err := EnumerateStationCandidates(ctx, input, options)
	if err != nil {
		t.Fatal(err)
	}
	options.StandardRowPacking = true
	updated, err := EnumerateStationCandidates(ctx, input, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated) <= len(original) || !updated[0].RowPacking {
		t.Fatal("没有提前真实行阵")
	}
	if !reflect.DeepEqual(updated[len(updated)-len(original):], original) {
		t.Fatal("删除或修改了原候选")
	}
	first, err := connectedExecutionRows(ctx, input, updated[0].MaxSpan, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range updated[:len(updated)-len(original)] {
		if !s.RowPacking {
			t.Fatal("新组合未启用行阵")
		}
		r, err := connectedExecutionRows(ctx, input, s.MaxSpan, false)
		if err != nil {
			t.Fatal(err)
		}
		if r.rowCount < first.rowCount {
			t.Fatal("未优先较少真实拆行")
		}
	}
	t.Logf("原候选%d，新组合%d，首个span%.0f/order%d/rows%d", len(original), len(updated)-len(original), updated[0].MaxSpan, updated[0].Order, first.rowCount)
}
