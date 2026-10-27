package factory

// 文件职责：约束蓝图文本导出的文件名及原子写入；使用：桌面保存对话框之后；边界：只允许 .txt 普通文件，失败保留原文件。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ExportName 职责：清理用户提供的默认名；使用：保存对话框；边界：不把名称作为目录路径。
func ExportName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "量化产线"
	}
	name = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`\/:*?"<>|`, r) {
			return '_'
		}
		return r
	}, name)
	return strings.TrimSuffix(name, ".txt") + ".txt"
}

// ExportText 职责：将已校验文本写入用户明确选择的位置；使用：ExportBlueprint；边界：拒绝链接和非文本扩展，重命名失败不截断原文件。
func ExportText(target, text string) error {
	return exportFile(target, text, ".txt")
}

// ExportJSON 职责：导出方案记录；使用：原生保存框之后；边界：只允许 .json 普通文件。
func ExportJSON(target, text string) error { return exportFile(target, text, ".json") }

// exportFile 职责：按允许的单一扩展名执行原子写入；使用：文本/方案导出；边界：目标链接或重命名失败时不截断原文件。
func exportFile(target, text, extension string) error {
	if !strings.EqualFold(filepath.Ext(target), extension) {
		return fmt.Errorf("请选择 %s 文件", extension)
	}
	if info, err := os.Lstat(target); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("导出目标必须是普通文本文件")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(target), ".dsbox-blueprint-*.tmp")
	if err != nil {
		return err
	}
	temporary := f.Name()
	defer os.Remove(temporary)
	if _, err = f.WriteString(text); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(temporary, target)
}
