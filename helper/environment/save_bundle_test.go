package environment

// 文件职责：验证主存档过滤、联机附加数据配对及成组文件操作的冲突保护。
// 使用：go test ./helper/environment；所有文件均写入测试临时目录。
// 边界：不触碰用户存档、数据库或 Wails 运行时。

import (
	"os"
	"path/filepath"
	"testing"
)

// writeSaveFixture 创建测试文件；用于构造临时存档目录，失败时终止当前测试。
func writeSaveFixture(t *testing.T, root, name, content string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// assertSaveFixture 校验测试文件内容；供复制与移动断言使用，不修改文件。
func assertSaveFixture(t *testing.T, path, expected string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil || string(content) != expected {
		t.Fatalf("%s: content=%q, err=%v, want=%q", path, content, err, expected)
	}
}

// TestScanSavesOnlyDSV 验证只列出主存档；覆盖大小写、子目录和临时目录，不把附加文件计入列表。
func TestScanSavesOnlyDSV(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"world.dsv", "world.server", "world.moddsv", "notes.txt", "world.dsv.tmp", "nested/other.DSV", ".dsbox-tmp/hidden.dsv"} {
		writeSaveFixture(t, root, name, name)
	}
	saves, err := ScanSaves(root)
	if err != nil || len(saves) != 2 {
		t.Fatalf("saves=%v, err=%v; want two .dsv files", saves, err)
	}
	for _, save := range saves {
		if save.FileName != "world.dsv" && save.FileName != "other.DSV" {
			t.Fatalf("unexpected save: %s", save.FileName)
		}
	}
}

// TestCopySaveBundle 验证导入/导出同时复制同名联机数据，保留源文件且忽略其它同名扩展。
func TestCopySaveBundle(t *testing.T) {
	source, target := t.TempDir(), t.TempDir()
	main := writeSaveFixture(t, source, "world.dsv", "save")
	writeSaveFixture(t, source, "WORLD.SERVER", "multiplayer")
	writeSaveFixture(t, source, "world.moddsv", "unrelated")
	if err := CopySaveBundleSafely(main, target); err != nil {
		t.Fatal(err)
	}
	assertSaveFixture(t, filepath.Join(target, "world.dsv"), "save")
	assertSaveFixture(t, filepath.Join(target, "WORLD.SERVER"), "multiplayer")
	assertSaveFixture(t, main, "save")
	if _, err := os.Stat(filepath.Join(target, "world.moddsv")); !os.IsNotExist(err) {
		t.Fatalf("unexpected .moddsv copy: %v", err)
	}
	if err := CopySaveBundleSafely(main, target); err != nil {
		t.Fatalf("same-content export should succeed: %v", err)
	}
}

// TestCopySaveBundleConflict 验证附加文件冲突时预检拒绝整个文件组，不先复制主存档或覆盖目标。
func TestCopySaveBundleConflict(t *testing.T) {
	source, target := t.TempDir(), t.TempDir()
	main := writeSaveFixture(t, source, "world.dsv", "save")
	writeSaveFixture(t, source, "world.server", "multiplayer")
	companion := writeSaveFixture(t, target, "world.server", "existing")
	if err := CopySaveBundleSafely(main, target); err == nil {
		t.Fatal("expected companion conflict")
	}
	if _, err := os.Stat(filepath.Join(target, "world.dsv")); !os.IsNotExist(err) {
		t.Fatalf("main save must not be copied on conflict: %v", err)
	}
	assertSaveFixture(t, companion, "existing")
	assertSaveFixture(t, main, "save")
}

// TestMoveSaveBundle 验证重命名及嵌套快照删除保持主存档与联机数据配对，不移动无关文件。
func TestMoveSaveBundle(t *testing.T) {
	root, snapshot := t.TempDir(), t.TempDir()
	main := writeSaveFixture(t, root, "world.dsv", "save")
	companion := writeSaveFixture(t, root, "world.server", "multiplayer")
	unrelated := writeSaveFixture(t, root, "world.moddsv", "unrelated")
	renamed := filepath.Join(root, "renamed.dsv")
	if err := MoveSaveBundleSafely(main, renamed); err != nil {
		t.Fatal(err)
	}
	assertSaveFixture(t, renamed, "save")
	assertSaveFixture(t, filepath.Join(root, "renamed.server"), "multiplayer")
	for _, old := range []string{main, companion} {
		if _, err := os.Stat(old); !os.IsNotExist(err) {
			t.Fatalf("old path still exists: %s, %v", old, err)
		}
	}
	target := filepath.Join(snapshot, "nested", "renamed.dsv")
	if err := MoveSaveBundleSafely(renamed, target); err != nil {
		t.Fatal(err)
	}
	assertSaveFixture(t, target, "save")
	assertSaveFixture(t, filepath.Join(snapshot, "nested", "renamed.server"), "multiplayer")
	assertSaveFixture(t, unrelated, "unrelated")
}

// TestMoveSaveBundleConflict 验证附加目标冲突时保留完整源文件组，避免半组重命名或删除。
func TestMoveSaveBundleConflict(t *testing.T) {
	root := t.TempDir()
	main := writeSaveFixture(t, root, "world.dsv", "save")
	companion := writeSaveFixture(t, root, "world.server", "multiplayer")
	existing := writeSaveFixture(t, root, "renamed.server", "existing")
	if err := MoveSaveBundleSafely(main, filepath.Join(root, "renamed.dsv")); err == nil {
		t.Fatal("expected companion conflict")
	}
	assertSaveFixture(t, main, "save")
	assertSaveFixture(t, companion, "multiplayer")
	assertSaveFixture(t, existing, "existing")
}

// TestSaveBundleWithoutCompanion 验证单机存档可独立复制和移动，独立 .server 不能被当作主文件。
func TestSaveBundleWithoutCompanion(t *testing.T) {
	root, target := t.TempDir(), t.TempDir()
	main := writeSaveFixture(t, root, "solo.dsv", "singleplayer")
	if err := CopySaveBundleSafely(main, target); err != nil {
		t.Fatal(err)
	}
	if err := MoveSaveBundleSafely(main, filepath.Join(root, "new.dsv")); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveBundlePaths(writeSaveFixture(t, root, "orphan.server", "orphan")); err == nil {
		t.Fatal("standalone .server must not be accepted")
	}
}

// TestCopySaveBundlesSelection 验证重复主文件和显式选择的附加文件按一个存档处理，不改变源数据。
func TestCopySaveBundlesSelection(t *testing.T) {
	source, target := t.TempDir(), t.TempDir()
	main := writeSaveFixture(t, source, "world.dsv", "save")
	companion := writeSaveFixture(t, source, "world.server", "multiplayer")
	count, err := CopySaveBundlesSafely([]string{companion, main, main}, target)
	if err != nil || count != 1 {
		t.Fatalf("count=%d, err=%v; want one save", count, err)
	}
	assertSaveFixture(t, filepath.Join(target, "world.dsv"), "save")
	assertSaveFixture(t, filepath.Join(target, "world.server"), "multiplayer")
}

// TestCopySaveBundlesBatchConflict 验证整批预检会保留源文件及已有目标，不留下前一个存档的副本。
func TestCopySaveBundlesBatchConflict(t *testing.T) {
	source, target := t.TempDir(), t.TempDir()
	first := writeSaveFixture(t, source, "first.dsv", "first")
	second := writeSaveFixture(t, source, "second.dsv", "second")
	writeSaveFixture(t, source, "second.server", "multiplayer")
	existing := writeSaveFixture(t, target, "second.server", "existing")
	if _, err := CopySaveBundlesSafely([]string{first, second}, target); err == nil {
		t.Fatal("expected batch conflict")
	}
	for _, name := range []string{"first.dsv", "second.dsv"} {
		if _, err := os.Stat(filepath.Join(target, name)); !os.IsNotExist(err) {
			t.Fatalf("unexpected partial import: %s, %v", name, err)
		}
	}
	assertSaveFixture(t, existing, "existing")
}

// TestCopySaveBundlesRollback 验证后续文件提交失败只回滚本次副本；已有相同内容文件始终保留。
func TestCopySaveBundlesRollback(t *testing.T) {
	for _, existingMain := range []bool{false, true} {
		t.Run(map[bool]string{false: "new", true: "existing"}[existingMain], func(t *testing.T) {
			source, target := t.TempDir(), t.TempDir()
			main := writeSaveFixture(t, source, "world.dsv", "save")
			companion := writeSaveFixture(t, source, "world.server", "multiplayer")
			if existingMain {
				writeSaveFixture(t, target, "world.dsv", "save")
			}
			writeSaveFixture(t, target, "world.server.dsbox.tmp/blocker", "block")
			if err := CopySaveBundleSafely(main, target); err == nil {
				t.Fatal("expected temporary file write failure")
			}
			if existingMain {
				assertSaveFixture(t, filepath.Join(target, "world.dsv"), "save")
			} else if _, err := os.Stat(filepath.Join(target, "world.dsv")); !os.IsNotExist(err) {
				t.Fatalf("new main file was not rolled back: %v", err)
			}
			assertSaveFixture(t, main, "save")
			assertSaveFixture(t, companion, "multiplayer")
		})
	}
}

// TestCopySaveBundlesRejectOrphan 验证独立附加文件导致整批拒绝，不留下已选择主文件的副本。
func TestCopySaveBundlesRejectOrphan(t *testing.T) {
	source, target := t.TempDir(), t.TempDir()
	main := writeSaveFixture(t, source, "world.dsv", "save")
	orphan := writeSaveFixture(t, source, "orphan.server", "orphan")
	if _, err := CopySaveBundlesSafely([]string{main, orphan}, target); err == nil {
		t.Fatal("expected orphan rejection")
	}
	entries, err := os.ReadDir(target)
	if err != nil || len(entries) != 0 {
		t.Fatalf("target must remain empty: %v, %v", entries, err)
	}
}
