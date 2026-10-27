package blueprint

// 文件职责：以独立社区语料验证严格读取和无损写回；使用：设置 DSBOX_BLUEPRINT_CORPUS 后 go test；边界：语料仅本地使用，不入库。

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestCommunityCorpus 职责：验证至少 28 份外部真实输入；使用：P0 门禁；边界：无语料时明确跳过，不伪称独立验收。
func TestCommunityCorpus(t *testing.T) {
	root := os.Getenv("DSBOX_BLUEPRINT_CORPUS")
	if root == "" {
		t.Skip("设置 DSBOX_BLUEPRINT_CORPUS 运行外部语料门禁")
	}
	var paths []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".txt") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	total, passed := 0, 0
	versions := map[int]int{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := strings.TrimSpace(strings.TrimPrefix(string(data), "\ufeff"))
		if !strings.HasPrefix(text, "BLUEPRINT:") {
			continue
		}
		total++
		d, err := Decode(text, true)
		if err != nil {
			t.Logf("失败 %s: %v", path, err)
		} else {
			encoded, err := Encode(d, d.EnvelopeVersion)
			if err != nil {
				t.Errorf("写回 %s: %v", path, err)
			} else if encoded.Blueprint != text {
				t.Errorf("无修改串字节不同: %s", path)
			} else {
				passed++
				versions[d.EnvelopeVersion]++
			}
		}
		if total == 28 {
			break
		}
	}
	t.Logf("社区语料 total=%d passed=%d envelopeVersions=%v", total, passed, versions)
	if total < 28 || passed < 25 {
		t.Fatalf("社区门禁未满足: %d/%d", passed, total)
	}
}
