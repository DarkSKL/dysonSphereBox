package environment

// 文件职责：验证存档目录优先级。
// 使用：go test 执行临时目录测试；边界：不修改用户配置或真实存档。

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSaveDirectoryOverride 职责：验证显式配置与游戏覆盖路径；使用：目录回归；边界：不存在的显式路径不偷偷回退。
func TestSaveDirectoryOverride(t *testing.T) {
	game := t.TempDir()
	document := t.TempDir()
	save := filepath.Join(document, "Dyson Sphere Program", "Save")
	if err := os.MkdirAll(filepath.Join(game, "Configs"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(save, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(game, "Configs", "path.txt"), []byte("\ufeff  "+document+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	result := ResolveSaveDirectory(game, "")
	if result.Path != save || !result.Exists {
		t.Fatalf("覆盖路径: %+v", result)
	}
	configured := filepath.Join(t.TempDir(), "missing")
	result = ResolveSaveDirectory(game, configured)
	if result.Path != configured || result.Exists {
		t.Fatalf("显式路径优先级: %+v", result)
	}
}
