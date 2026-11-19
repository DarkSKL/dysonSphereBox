package apply

// 文件职责：用真实 Windows 进程验证退出等待、替换、清理与自动重启。
// 使用：go test ./helper/updater/apply；边界：只启动测试程序副本，不操作真实盒子或远端发布。

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestMain 职责：为进程回归提供旧版和新版夹具入口；使用：测试程序复制到临时目录后启动；边界：只有显式测试环境变量存在时启用。
func TestMain(m *testing.M) {
	if root := os.Getenv("DSBOX_UPDATE_FIXTURE"); root != "" {
		if len(os.Args) > 1 && os.Args[1] == "--hold" {
			_ = os.WriteFile(filepath.Join(root, "parent-ready"), []byte("ready"), 0600)
			for {
				if _, err := os.Stat(filepath.Join(root, "exit")); err == nil {
					break
				}
				time.Sleep(20 * time.Millisecond)
			}
		} else {
			_ = os.WriteFile(filepath.Join(root, "restarted"), []byte("started"), 0600)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// waitForFile 职责：等待真实进程的文件信号；使用：更新集成回归；边界：限时失败，不无限轮询。
func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}

// TestUpdateWaitsForExitThenCleansAndRestarts 职责：验证实际 Windows 更新生命周期；使用：退出更新回归；边界：运行真实助手但目标仅为临时测试副本。
func TestUpdateWaitsForExitThenCleansAndRestarts(t *testing.T) {
	root := filepath.Join(t.TempDir(), "中文 空格'目录")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DSBOX_UPDATE_FIXTURE", root)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	oldData := append(append([]byte(nil), data...), []byte("old version")...)
	newData := append(append([]byte(nil), data...), []byte("new version")...)
	target := filepath.Join(root, "Box 更新.exe")
	if err := os.WriteFile(target, oldData, 0755); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(root, "."+filepath.Base(target)+".old")
	if err := os.WriteFile(legacy, []byte("legacy backup"), 0600); err != nil {
		t.Fatal(err)
	}
	userFile := filepath.Join(root, "user-data.txt")
	if err := os.WriteFile(userFile, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}

	parent := exec.Command(target, "--hold")
	parent.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := parent.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = parent.Wait(); close(done) }()
	t.Cleanup(func() {
		_ = parent.Process.Kill()
		<-done
	})
	waitForFile(t, filepath.Join(root, "parent-ready"))
	prepared, err := prepareExecutable(bytes.NewReader(newData), target)
	if err != nil {
		t.Fatal(err)
	}
	if err := prepared.StartAfterExit(parent.Process.Pid); err != nil {
		t.Fatal(err)
	}

	// 旧进程仍运行时，助手必须只等待，目标与旧备份都保持原样。
	time.Sleep(200 * time.Millisecond)
	before, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(before, oldData) {
		t.Fatalf("program replaced before exit: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "restarted")); !os.IsNotExist(err) {
		t.Fatal("restarted before parent exit")
	}
	if err := os.WriteFile(filepath.Join(root, "exit"), []byte("exit"), 0600); err != nil {
		t.Fatal(err)
	}
	waitForFile(t, filepath.Join(root, "restarted"))
	after, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(after, newData) {
		t.Fatalf("new program not installed: %v", err)
	}
	for _, removed := range []string{legacy, prepared.directory} {
		if _, err := os.Stat(removed); !os.IsNotExist(err) {
			t.Fatalf("old or temporary file remains: %s (%v)", removed, err)
		}
	}
	userData, err := os.ReadFile(userFile)
	if err != nil || string(userData) != "keep" {
		t.Fatalf("user data changed: %v", err)
	}
}

// TestFailedHandoffDoesNotReplaceExecutable 职责：验证等待目标不存在时交接返回错误；使用：助手失败回归；边界：不进入安装阶段、不关闭测试主进程。
func TestFailedHandoffDoesNotReplaceExecutable(t *testing.T) {
	target := filepath.Join(t.TempDir(), "Box.exe")
	if err := os.WriteFile(target, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareExecutable(bytes.NewReader([]byte("new")), target)
	if err != nil {
		t.Fatal(err)
	}
	if err := prepared.StartAfterExit(-1); err == nil || strings.Contains(err.Error(), "超时") {
		t.Fatal("expected handoff error")
	}
	current, err := os.ReadFile(target)
	if err != nil || string(current) != "old" {
		t.Fatalf("current program changed: %q, %v", current, err)
	}
}
