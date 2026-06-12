package core

// 文件职责：解析运行期数据库文件路径并确保模板数据库落盘。
// 使用：InitDatabase 打开 SQLite 前调用 databasePath 和 ensureDatabaseFile。
// 边界：只处理文件路径和模板复制，不打开数据库、不执行迁移、不初始化全局资源。

import (
	"fmt"
	"os"
	"path/filepath"
)

// databasePath 解析运行期数据库文件路径。
// 职责：将 exe 所在目录和 databaseFileName 组合为真实路径。
// 使用：InitDatabase 打开数据库前调用。
// 边界：依赖 os.Executable，解析失败时返回错误，不猜测工作目录。
func databasePath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}

	return filepath.Join(filepath.Dir(exePath), databaseFileName), nil
}

// ensureDatabaseFile 确保运行期数据库文件存在。
// 职责：首次启动时把嵌入的模板数据库写入 exe 同级目录。
// 使用：InitDatabase 打开 SQLite 前调用。
// 边界：已存在时不覆盖；模板为空或写入失败时返回错误。
func ensureDatabaseFile(path string, databaseTemplate []byte) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect sqlite database: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create sqlite database directory: %w", err)
	}

	if len(databaseTemplate) == 0 {
		return fmt.Errorf("embedded sqlite database is empty")
	}

	if err := os.WriteFile(path, databaseTemplate, 0644); err != nil {
		return fmt.Errorf("write sqlite database: %w", err)
	}

	return nil
}
