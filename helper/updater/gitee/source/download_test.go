package source

// 文件职责：验证更新附件的真实 HTTP 流与下载进度衔接。
// 使用：回归测试响应长度、重定向和分块读取；边界：只访问本地测试服务器，不校验或替换可执行文件。

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"DysonSphereBox/helper/updater/progress"
)

// TestDownloadReleaseAssetProgress 职责：验证真实流量决定进度及响应长度优先级；使用：更新下载回归；边界：未知长度只统计字节，下载完成不等于安装成功。
func TestDownloadReleaseAssetProgress(t *testing.T) {
	payload := strings.Repeat("更新内容", 4096)
	for _, scenario := range []struct {
		// name 职责：标识回归场景；使用：子测试命名；边界：不参与下载。
		name string
		// knownLength 职责：控制服务器是否发送内容长度；使用：模拟普通响应或分块响应；边界：不影响响应内容。
		knownLength bool
		// metadataSize 职责：模拟发布元数据长度；使用：覆盖缺失、错误与兜底；边界：不作为完整性校验。
		metadataSize int64
		// expectedTotal 职责：声明预期下载总量；使用：校验进度快照；边界：零表示无法确定总量。
		expectedTotal int64
	}{
		{name: "响应长度补齐缺失元数据", knownLength: true, expectedTotal: int64(len(payload))},
		{name: "响应长度覆盖错误元数据", knownLength: true, metadataSize: 1, expectedTotal: int64(len(payload))},
		{name: "分块响应回退发布元数据", metadataSize: int64(len(payload)), expectedTotal: int64(len(payload))},
		{name: "未知总量仍报告实际字节"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/redirect" {
					http.Redirect(writer, request, "/asset", http.StatusFound)
					return
				}
				if scenario.knownLength {
					writer.Header().Set("Content-Length", fmt.Sprint(len(payload)))
				}
				for offset := 0; offset < len(payload); offset += len(payload) / 3 {
					_, _ = writer.Write([]byte(payload[offset : offset+len(payload)/3]))
					writer.(http.Flusher).Flush()
					if offset == 0 {
						time.Sleep(150 * time.Millisecond)
					}
				}
			}))
			defer server.Close()
			updateSource, err := NewSource(Config{HTTPClient: server.Client()})
			if err != nil {
				t.Fatal(err)
			}
			updateSource.assets[1] = giteeAsset{Name: "update.zip", BrowserDownloadURL: server.URL + "/redirect"}
			reader, err := updateSource.DownloadReleaseAsset(context.Background(), nil, 1)
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			var snapshots []progress.Snapshot
			sawIntermediate := false
			data, err := progress.ReadAll(reader, scenario.metadataSize, func(snapshot progress.Snapshot) {
				snapshots = append(snapshots, snapshot)
				if snapshot.Stage == "downloading" && snapshot.DownloadedBytes > 0 && snapshot.DownloadedBytes < int64(len(payload)) {
					sawIntermediate = true
				}
			})
			if err != nil || string(data) != payload {
				t.Fatalf("下载内容不一致：长度=%d，错误=%v", len(data), err)
			}
			if !sawIntermediate {
				t.Fatal("下载结束前没有收到实际字节进度")
			}
			previousBytes := int64(0)
			for _, snapshot := range snapshots {
				if snapshot.TotalBytes != scenario.expectedTotal || snapshot.DownloadedBytes < previousBytes {
					t.Fatalf("总量错误或下载字节回退：%+v", snapshot)
				}
				previousBytes = snapshot.DownloadedBytes
			}
			first, last := snapshots[0], snapshots[len(snapshots)-1]
			if first.Stage != "downloading" || first.DownloadedBytes != 0 || last.Stage != "verifying" || last.DownloadedBytes != int64(len(payload)) {
				t.Fatalf("阶段或首尾字节错误：首=%+v，尾=%+v", first, last)
			}
		})
	}
}
