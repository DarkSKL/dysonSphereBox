package progress

// 文件职责：验证界面下载进度来自实际字节，并且读取失败不能进入校验阶段。
// 使用：go test ./helper/updater/progress；边界：使用内存流，不连接远端。

import (
	"io"
	"strings"
	"testing"
)

// sizedReader 职责：模拟响应长度；使用：验证真实 HTTP 长度优先；边界：仅在测试中使用。
type sizedReader struct{ io.Reader }

// ContentLength 职责：返回夹具实际长度；使用：进度读取器；边界：不依赖发布元数据。
func (sizedReader) ContentLength() int64 { return 6 }

// brokenReader 职责：模拟下载中断；使用：失败进度回归；边界：固定返回读取错误。
type brokenReader struct{}

// Read 职责：返回中断错误；使用：io.MultiReader；边界：不返回任何新字节。
func (brokenReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

// TestDownloadReportsActualBytes 职责：验证正常、未知大小及中断下载的进度；使用：下载条回归；边界：不验证计时器动画。
func TestDownloadReportsActualBytes(t *testing.T) {
	for _, test := range []struct {
		name      string
		reader    io.Reader
		total     int64
		wantTotal int64
		wantBytes int64
		wantStage string
		wantError bool
	}{
		{"response-length", sizedReader{strings.NewReader("abcdef")}, 100, 6, 6, "verifying", false},
		{"unknown-length", strings.NewReader("abc"), 0, 0, 3, "verifying", false},
		{"interrupted", io.MultiReader(strings.NewReader("abc"), brokenReader{}), 10, 10, 3, "downloading", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var snapshots []Snapshot
			_, err := ReadAll(test.reader, test.total, func(snapshot Snapshot) { snapshots = append(snapshots, snapshot) })
			if (err != nil) != test.wantError {
				t.Fatalf("read error = %v", err)
			}
			if len(snapshots) < 2 || snapshots[0].DownloadedBytes != 0 {
				t.Fatalf("missing initial progress: %v", snapshots)
			}
			last := snapshots[len(snapshots)-1]
			if last.DownloadedBytes != test.wantBytes || last.TotalBytes != test.wantTotal || last.Stage != test.wantStage {
				t.Fatalf("unexpected final progress: %+v", last)
			}
		})
	}
}
