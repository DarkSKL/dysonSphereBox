package diagnostics

// 文件职责：读取已保留的独立日志证据。使用：数据库重建、降级检索与诊断导出；边界：损坏行跳过并报告告警，不读取任意用户路径。

import (
	"DysonSphereBox/constant/logging"
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// ReadEntries 职责：读取自身日志文件与未落盘内存；使用：索引回放和降级；边界：按 ID 去重，损坏行不阻断其余记录。
func (s *Store) ReadEntries() ([]logging.Entry, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	paths, err := filepath.Glob(filepath.Join(s.dir, "client-*.jsonl"))
	if err != nil {
		return append([]logging.Entry{}, s.memory...), CleanText(err.Error())
	}
	seen := map[string]bool{}
	items := []logging.Entry{}
	warnings := []string{}
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			warnings = append(warnings, CleanText(err.Error()))
			continue
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 65536), 1024*1024)
		for scanner.Scan() {
			var entry logging.Entry
			if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
				warnings = append(warnings, filepath.Base(path)+": 损坏日志行")
				continue
			}
			if entry.ID != "" && !seen[entry.ID] {
				seen[entry.ID] = true
				items = append(items, entry)
			}
		}
		if err := scanner.Err(); err != nil {
			warnings = append(warnings, CleanText(err.Error()))
		}
		file.Close()
	}
	for _, entry := range s.memory {
		if !seen[entry.ID] {
			items = append(items, entry)
			seen[entry.ID] = true
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, _ := time.Parse(time.RFC3339Nano, items[i].Time)
		b, _ := time.Parse(time.RFC3339Nano, items[j].Time)
		return a.After(b)
	})
	warning := ""
	if len(warnings) > 0 {
		warning = fmt.Sprintf("读取日志出现 %d 项异常：%s", len(warnings), warnings[0])
	}
	return items, warning
}
