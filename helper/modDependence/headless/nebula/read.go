package nebula

// 文件职责：读取 Nebula 无头服务器配置文件中的 HostPort。
// 使用：headless 环境检测读取当前端口时调用 ReadHostPort。
// 边界：只读取 HostPort，文件缺失、解析失败或端口非法时返回调用方默认端口。

import (
	"os"
	"path/filepath"
)

// ReadHostPort 从 nebula.cfg 读取主机端口。
// 职责：优先读取 Nebula 设置节中的 HostPort，缺失时回退全文件扫描。
// 使用：headless.InspectHeadlessEnvironment 生成当前端口状态时调用。
// 边界：文件缺失、解析失败或端口非法时返回 defaultPort。
func ReadHostPort(gameDir string, configRelative string, sectionName string, defaultPort int) int {
	content, err := os.ReadFile(filepath.Join(gameDir, filepath.FromSlash(configRelative)))
	if err != nil {
		return defaultPort
	}

	lines := normalizeConfigLines(content)
	if port, ok := findHostPortInSection(lines, sectionName); ok {
		return port
	}
	for _, line := range lines {
		if port, ok := parseHostPortLine(line); ok {
			return port
		}
	}

	return defaultPort
}
