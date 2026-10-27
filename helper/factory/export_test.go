package factory

// 文件职责：验证导出只触碰指定文本文件；使用：go test；边界：全程临时目录，不打开桌面保存框。

import (
	"os"
	"path/filepath"
	"testing"
)

// TestExportText 职责：验证覆盖、扩展名限制及符号链接拒绝；使用：原生导出底层；边界：不证明 Windows 对话框可用。
func TestExportText(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "blueprint.txt")
	if err := os.WriteFile(target, []byte("before"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := ExportText(target, "after"); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(target); string(data) != "after" {
		t.Fatal(string(data))
	}
	if err := ExportText(filepath.Join(dir, "app.db"), "forbidden"); err == nil {
		t.Fatal("non-text accepted")
	}
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := ExportText(link, "forbidden"); err == nil {
		t.Fatal("symlink accepted")
	}
	if ExportName("../../蓝图") != ".._.._蓝图.txt" {
		t.Fatal(ExportName("../../蓝图"))
	}
}
