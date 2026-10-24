package nebula

// 文件职责：提供 nebula.cfg 行级解析与节定位工具。
// 使用：ReadHostPort 和 WriteHostPort 共享这些函数保持读写规则一致。
// 边界：只处理内存中的配置行，不读写文件、不校验端口占用、不解释其它 Nebula 配置。

import (
	"strconv"
	"strings"
)

// normalizeConfigLines 统一配置文件换行格式。
// 职责：把 CRLF/LF 内容转换成行切片供解析和回写。
// 使用：读取 nebula.cfg 后由解析和写入函数调用。
// 边界：不去除注释或空行，保留原有配置结构。
func normalizeConfigLines(content []byte) []string {
	return strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
}

// findHostPortInSection 在目标设置节中查找 HostPort。
// 职责：只在目标 section 内读取端口，避免误读其它模块配置。
// 使用：ReadHostPort 的优先解析路径。
// 边界：遇到下一节或端口非法时返回 false。
func findHostPortInSection(lines []string, sectionName string) (int, bool) {
	inSection := false
	for _, line := range lines {
		if isSettingsSection(line, sectionName) {
			inSection = true
			continue
		}
		if inSection && isInConfigSection(line) {
			return 0, false
		}
		if !inSection {
			continue
		}
		if port, ok := parseHostPortLine(line); ok {
			return port, true
		}
	}

	return 0, false
}

// replaceHostPortInSection 替换目标设置节内已有 HostPort 行。
// 职责：在不打乱其它配置的情况下更新端口。
// 使用：WriteHostPort 写回旧配置时调用。
// 边界：没有目标节或没有 HostPort 时返回 false，由调用方插入新行。
func replaceHostPortInSection(lines []string, sectionName string, newLine string) bool {
	inSection := false
	for index, line := range lines {
		if isSettingsSection(line, sectionName) {
			inSection = true
			continue
		}
		if inSection && isInConfigSection(line) {
			return false
		}
		if !inSection {
			continue
		}
		if _, ok := parseHostPortLine(line); ok {
			lines[index] = newLine
			return true
		}
	}

	return false
}

// insertHostPortInSection 向配置中插入 HostPort 行。
// 职责：优先插入已有目标设置节，否则在文件末尾创建新节。
// 使用：WriteHostPort 未能替换已有行时调用。
// 边界：保留原始其它行，不清理重复的跨节 HostPort。
func insertHostPortInSection(lines []string, sectionName string, newLine string) []string {
	for index, line := range lines {
		if !isSettingsSection(line, sectionName) {
			continue
		}

		insertIndex := index + 1
		for insertIndex < len(lines) && strings.TrimSpace(lines[insertIndex]) == "" {
			insertIndex++
		}
		lines = append(lines[:insertIndex], append([]string{newLine}, lines[insertIndex:]...)...)
		return lines
	}

	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
		lines = append(lines, "")
	}

	return append(lines, "["+sectionName+"]", "", newLine)
}

// parseHostPortLine 解析单行 HostPort 配置。
// 职责：识别 HostPort = number 并校验端口范围。
// 使用：端口读取、替换和清理旧配置时调用。
// 边界：键名不匹配、格式错误或端口越界时返回 false。
func parseHostPortLine(line string) (int, bool) {
	parts := strings.SplitN(strings.TrimSpace(line), "=", 2)
	if len(parts) != 2 || !strings.EqualFold(strings.TrimSpace(parts[0]), "HostPort") {
		return 0, false
	}

	port, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || port <= 0 || port > 65535 {
		return 0, false
	}

	return port, true
}

// isSettingsSection 判断配置行是否是目标设置节。
// 职责：集中维护 section 名称比较规则。
// 使用：端口查找、替换和插入函数调用。
// 边界：比较大小写不敏感，但不解析嵌套或注释。
func isSettingsSection(line string, sectionName string) bool {
	return strings.EqualFold(strings.TrimSpace(line), "["+sectionName+"]")
}

// isInConfigSection 判断配置行是否是任意 section 标题。
// 职责：帮助端口解析识别节边界。
// 使用：findHostPortInSection 和 replaceHostPortInSection 调用。
// 边界：只按方括号格式判断，不校验 section 名是否合法。
func isInConfigSection(line string) bool {
	trimmedLine := strings.TrimSpace(line)
	return strings.HasPrefix(trimmedLine, "[") && strings.HasSuffix(trimmedLine, "]")
}
