package logging

// 文件职责：生成脱敏诊断 ZIP 并打开日志目录。使用：LogService；边界：只包含统一日志与基础运行信息，不收集账号配置、存档或全量数据库。

import (
	"DysonSphereBox/core"
	"DysonSphereBox/core/diagnostics"
	systemHelper "DysonSphereBox/helper/system"
	systemModel "DysonSphereBox/model/system"
	"archive/zip"
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// OpenDirectory 职责：使用系统文件管理器打开目录；使用：日志工具栏；边界：只接受已初始化的内部目录。
func OpenDirectory(dir string) error {
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("日志目录不可访问")
	}
	return exec.Command("explorer.exe", dir).Start()
}

// Export 职责：输出可携带的诊断证据；使用：用户选定目标后；边界：保护盒子内部目录，半成品用临时文件原子替换，ZIP 写入错误透传。
func Export(db *gorm.DB, target string, q systemHelper.LogQuery, health systemHelper.LogHealth) error {
	deleted, err := deletedIDs(db)
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	for _, protected := range []string{exe, core.GLOBAL.DBPath} {
		if protected != "" && strings.EqualFold(filepath.Clean(abs), filepath.Clean(protected)) {
			return fmt.Errorf("不能覆盖盒子程序或数据库")
		}
	}
	if dir := health.File.Directory; dir != "" {
		rel, err := filepath.Rel(dir, abs)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("不能覆盖原始日志目录中的文件")
		}
	}
	temp, err := os.CreateTemp(filepath.Dir(abs), ".dsbox-diagnostics-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	writer := zip.NewWriter(temp)
	entry, err := writer.Create("manifest.json")
	if err != nil {
		return err
	}
	manifest := map[string]interface{}{"schemaVersion": 2, "exportedAt": time.Now().Format(time.RFC3339Nano), "appVersion": core.Version, "os": runtime.GOOS, "arch": runtime.GOARCH, "goVersion": runtime.Version(), "health": health, "query": q, "scope": "盒子统一日志；不含账号凭据、存档或原始数据库"}
	encoded, _ := json.Marshal(manifest)
	var safe map[string]interface{}
	json.Unmarshal(encoded, &safe)
	if err := json.NewEncoder(entry).Encode(diagnostics.CleanData(safe)); err != nil {
		return err
	}
	logs, err := writer.Create("logs.jsonl")
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(logs)
	seen := map[string]bool{}
	indexWarning := ""
	if db != nil {
		query := ApplyQuery(db.Session(&gorm.Session{Logger: logger.Discard}).Model(&systemModel.LogEntry{}), q)
		rows, queryErr := query.Order("event_time ASC, id ASC").Rows()
		if queryErr == nil {
			for rows.Next() {
				var row systemModel.LogEntry
				if err := db.ScanRows(rows, &row); err != nil {
					indexWarning = diagnostics.CleanText(err.Error())
					break
				}
				item := systemHelper.ToLogEntries([]systemModel.LogEntry{row})[0]
				item.Data = diagnostics.CleanData(item.Data)
				if err := encoder.Encode(item); err != nil {
					rows.Close()
					return err
				}
				seen[item.ID] = true
			}
			if err := rows.Err(); err != nil {
				indexWarning = diagnostics.CleanText(err.Error())
			}
			rows.Close()
		} else {
			indexWarning = diagnostics.CleanText(queryErr.Error())
		}
	}
	items, warning := diagnostics.Default.ReadEntries()
	for _, item := range items {
		if !deleted[item.ID] && !seen[item.ID] && Match(item, q) {
			item.Data = diagnostics.CleanData(item.Data)
			if err := encoder.Encode(item); err != nil {
				return err
			}
		}
	}
	if warning != "" {
		w, err := writer.Create("file-warning.txt")
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, warning); err != nil {
			return err
		}
	}
	if indexWarning != "" {
		w, err := writer.Create("index-warning.txt")
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, indexWarning); err != nil {
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(temp.Name(), abs)
}
