package quant

// 文件职责：检查存档约束对自动选方、指定配方和直接采集的影响；使用：量化回归；边界：普通量化没有存档约束。

import (
	"errors"
	"strings"
	"testing"
)

// TestSavedRecipeChoices 职责：自动选择已解锁的石墨烯配方，给出显式配方锁定原因；使用：复用同一请求比较模拟与存档；边界：只核对实际制造行。
func TestSavedRecipeChoices(t *testing.T) {
	q := Request{Targets: []Target{{ItemID: 1123, RatePerMinute: 1}}, Settings: Settings{MineralizeList: map[string]bool{"高能石墨": true, "硫酸": true}}}
	restricted := q.WithUnlockedRecipes(func(id int) bool { return id == 32 })
	for _, tc := range []struct {
		req  Request
		want int
	}{{restricted, 32}, {q, 31}} {
		r, err := Solve(t.Context(), tc.req)
		if err != nil || len(r.Productions) != 1 || r.Productions[0].RecipeID != tc.want {
			t.Fatalf("预期石墨烯配方 %d，实际 %+v，错误 %v", tc.want, r.Productions, err)
		}
	}
	restricted.Scheme.ItemRecipeChoices = map[string]int{"石墨烯": 1}
	_, err := Solve(t.Context(), restricted)
	var reason *Error
	if !errors.As(err, &reason) || reason.Code != "Q_E_RECIPE_LOCKED" || !strings.Contains(reason.Message, "石墨烯") {
		t.Fatalf("指定的未解锁配方应说明原因，实际 %v", err)
	}
	restricted.Targets = []Target{{ItemID: 1003, RatePerMinute: 1}}
	restricted.Settings.DirectResourceItemIDs = []int{1003}
	r, err := Solve(t.Context(), restricted)
	if err != nil || len(r.Productions) != 0 || len(r.RawInputs) != 1 || r.RawInputs[0].ItemID != 1003 {
		t.Fatalf("直接采矿受到制造配方限制，实际 %+v，错误 %v", r.RawInputs, err)
	}
}
