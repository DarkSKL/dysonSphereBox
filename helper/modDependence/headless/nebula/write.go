package nebula

// 文件职责：写入 Nebula 无头服务器配置文件中的 HostPort。
// 使用：headless.SaveHeadlessSettings 保存端口时调用 WriteHostPort。
// 边界：只维护 HostPort，不改动其它 Nebula 配置项，也不判断端口是否被占用。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteHostPort 写入 nebula.cfg 的 HostPort 配置。
// 职责：创建配置目录并在 Nebula 设置节中更新或插入端口。
// 使用：headless.SaveHeadlessSettings 保存端口时调用。
// 边界：只维护 HostPort，不改动其它配置项；文件不存在时创建最小配置。
// 逻辑流：1. 创建配置目录 => 2. 读取现有配置 => 3. 新文件写入设置节 => 4. 旧文件优先替换设置节端口 => 5. 不存在时插入设置节并保存。
func WriteHostPort(gameDir string, configRelative string, sectionName string, hostPort int) error {
	configPath := filepath.Join(gameDir, filepath.FromSlash(configRelative))
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return err
	}

	content, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	newLine := fmt.Sprintf("HostPort = %d", hostPort)
	if os.IsNotExist(err) || len(content) == 0 {
		return os.WriteFile(configPath, []byte("["+sectionName+"]\r\n\r\n"+newLine+"\r\n"), 0o644)
	}

	lines := normalizeConfigLines(content)
	replaced := replaceHostPortInSection(lines, sectionName, newLine)
	for index, line := range lines {
		if isInConfigSection(line) {
			break
		}
		if _, ok := parseHostPortLine(line); !ok {
			continue
		}

		lines[index] = ""
	}
	if !replaced {
		lines = insertHostPortInSection(lines, sectionName, newLine)
	}

	return os.WriteFile(configPath, []byte(strings.Join(lines, "\r\n")), 0o644)
}
