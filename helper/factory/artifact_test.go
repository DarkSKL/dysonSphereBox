package factory

// 文件职责：验证分片权限和淘汰；使用：go test；边界：仅使用 t.TempDir，不写资源库。

import (
	contract "DysonSphereBox/helper/factory/contract"
	"os"
	"strings"
	"testing"
)

// TestArtifacts 职责：核对分片重组、非法输入与磁盘清理；使用：大串桥接；边界：不接受任意路径。
func TestArtifacts(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	a := &Artifacts{}
	original := strings.Repeat("BLUEPRINT0123456789", 100000)
	token, err := a.Put(original)
	if err != nil {
		t.Fatal(err)
	}
	var result strings.Builder
	offset := 0
	for {
		part, err := a.Read(contract.ChunkRequest{Token: token, Offset: offset})
		if err != nil {
			t.Fatal(err)
		}
		result.WriteString(part.Text)
		if part.Done {
			break
		}
		if part.NextOffset <= offset {
			t.Fatal("no progress")
		}
		offset = part.NextOffset
	}
	if result.String() != original {
		t.Fatal("chunk corruption")
	}
	for _, req := range []contract.ChunkRequest{{Token: "/etc/passwd"}, {Token: token, Offset: -1}, {Token: token, Offset: len(original) + 1}} {
		if _, err := a.Read(req); err == nil {
			t.Fatal("invalid chunk accepted", req)
		}
	}
	firstPath := a.paths[token]
	for i := 0; i < 8; i++ {
		if _, err := a.Put("test"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(firstPath); !os.IsNotExist(err) {
		t.Fatal("evicted file retained", err)
	}
	if _, err := a.Read(contract.ChunkRequest{Token: token}); err == nil {
		t.Fatal("evicted token accepted")
	}
}
