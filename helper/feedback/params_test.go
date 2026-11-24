package feedback

// 文件职责：检验批量预览的输入约束。
// 使用：运行反馈 helper 测试时覆盖数量、去重和 ID 表示边界。
// 边界：测试仅校验本机输入，不读取账号或星图。

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// TestPreviewIDs 职责：验证规范十进制和原始数量限制。使用：批量输入回归。边界：合法 ID 始终保留原字符串。
func TestPreviewIDs(t *testing.T) {
	const maximum = "18446744073709551615"
	full := slices.Repeat([]string{"1"}, 128)
	for _, tc := range []struct{ raw, want []string }{
		{[]string{maximum, "9007199254740993", maximum, "1"}, []string{maximum, "9007199254740993", "1"}},
		{full, []string{"1"}},
	} {
		got, err := PreviewIDs(tc.raw)
		if err != nil || !slices.Equal(got, tc.want) {
			t.Fatalf("PreviewIDs(%v) = %v, %v", tc.raw, got, err)
		}
	}
	invalid := [][]string{nil, {}, append(full, "1")}
	for _, id := range []string{"", "0", "01", "+1", "-1", "１", "1e2", "1.0", " 1", "1\n", "18446744073709551616", strings.Repeat("9", 100)} {
		invalid = append(invalid, []string{"1", id})
	}
	for _, raw := range invalid {
		got, err := PreviewIDs(raw)
		var rejected *requestError
		if got != nil || !errors.As(err, &rejected) || rejected.Reason != "invalidInput" {
			t.Fatalf("PreviewIDs(%q) = %v, %v, want invalidInput", raw, got, err)
		}
	}
}
