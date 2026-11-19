package apply

// 文件职责：验证下载准备不会提前修改当前程序，失败不留下半成品。
// 使用：go test ./helper/updater/apply；边界：只操作测试临时目录。

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// failedReader 职责：模拟下载或解压中断；使用：准备失败测试；边界：不访问网络。
type failedReader struct{}

// Read 职责：返回可识别的读取失败；使用：io.Copy；边界：不生成文件内容。
func (failedReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

// TestPreparePreservesRunningExecutable 职责：验证准备成功和失败均保留旧包体；使用：回归退出前不得替换的约束；边界：不启动安装助手。
func TestPreparePreservesRunningExecutable(t *testing.T) {
	for _, test := range []struct {
		name      string
		reader    io.Reader
		wantError bool
	}{
		{"ready", strings.NewReader("new executable"), false},
		{"empty", strings.NewReader(""), true},
		{"interrupted", io.MultiReader(strings.NewReader("partial"), failedReader{}), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			target := filepath.Join(root, "Box.exe")
			if err := os.WriteFile(target, []byte("old executable"), 0755); err != nil {
				t.Fatal(err)
			}
			prepared, err := prepareExecutable(test.reader, target)
			if (err != nil) != test.wantError {
				t.Fatalf("prepare error = %v", err)
			}
			current, err := os.ReadFile(target)
			if err != nil || string(current) != "old executable" {
				t.Fatalf("current program changed: %q, %v", current, err)
			}
			if test.wantError {
				entries, err := os.ReadDir(root)
				if err != nil || len(entries) != 1 {
					t.Fatalf("partial update left behind: %v, %v", entries, err)
				}
			} else {
				staged, err := os.ReadFile(filepath.Join(prepared.directory, "update.exe"))
				if err != nil || string(staged) != "new executable" {
					t.Fatalf("staged content: %q, %v", staged, err)
				}
			}
		})
	}
}

// TestMissingStagedUpdateKeepsApplicationOpen 职责：验证暂存文件丢失时拒绝交接；使用：回归退出失败；边界：不启动任何更新进程。
func TestMissingStagedUpdateKeepsApplicationOpen(t *testing.T) {
	prepared := &PreparedUpdate{target: filepath.Join(t.TempDir(), "Box.exe"), directory: t.TempDir()}
	if err := prepared.StartAfterExit(os.Getpid()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing-file error, got %v", err)
	}
}

// TestDiscardPreparedUpdatePreservesCurrent 职责：验证交接失败后的取消只删除暂存文件；使用：失败退出回归；边界：不操作真实程序。
func TestDiscardPreparedUpdatePreservesCurrent(t *testing.T) {
	target := filepath.Join(t.TempDir(), "Box.exe")
	if err := os.WriteFile(target, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareExecutable(strings.NewReader("new"), target)
	if err != nil {
		t.Fatal(err)
	}
	if err := prepared.Discard(); err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(target)
	if err != nil || string(current) != "old" {
		t.Fatalf("current program changed: %q, %v", current, err)
	}
	if _, err := os.Stat(prepared.directory); !os.IsNotExist(err) {
		t.Fatalf("temporary update remains: %v", err)
	}
}
