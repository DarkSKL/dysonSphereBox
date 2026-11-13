package progress

// 文件职责：验证下载失败与无监听器时的流读取边界。
// 使用：进度读取器回归测试；边界：不访问网络、不发送桌面事件、不修改更新文件。

import (
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

// TestReadAllFailureProgress 职责：验证读取失败仍报告末次实际字节；使用：防止失败下载进入校验阶段；边界：错误和已读取内容必须保留。
func TestReadAllFailureProgress(t *testing.T) {
	expectedError := errors.New("下载中断")
	reader := io.MultiReader(strings.NewReader("partial"), iotest.ErrReader(expectedError))
	var snapshots []Snapshot
	data, err := ReadAll(reader, 100, func(snapshot Snapshot) {
		snapshots = append(snapshots, snapshot)
	})
	if !errors.Is(err, expectedError) || string(data) != "partial" {
		t.Fatalf("读取结果或错误丢失：%q，%v", data, err)
	}
	for _, snapshot := range snapshots {
		if snapshot.Stage != "downloading" {
			t.Fatalf("失败下载错误地进入后续阶段：%+v", snapshot)
		}
	}
	if last := snapshots[len(snapshots)-1]; last.DownloadedBytes != int64(len(data)) {
		t.Fatalf("未报告最终实际字节：%+v", last)
	}
}

// TestReadAllWithoutReporter 职责：验证无监听器不会改变内容读取；使用：保护普通下载调用；边界：不要求 Reader 提供长度。
func TestReadAllWithoutReporter(t *testing.T) {
	data, err := ReadAll(strings.NewReader("asset"), 0, nil)
	if err != nil || string(data) != "asset" {
		t.Fatalf("无回调读取失败：%q，%v", data, err)
	}
}
