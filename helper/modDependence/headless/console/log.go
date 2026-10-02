package console

// 文件职责：读取固定服务器日志尾部并导出原始日志。
// 使用：开服服务调用 ReadLog、ExportLog；不接受任意源路径。
// 边界：读取最多 256 KiB/300 行；不修改日志，不解析在线玩家。

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"DysonSphereBox/helper/modDependence/headless/contract"
)

// logLevelPattern 职责：识别 BepInEx 与 Unity 日志等级；使用：快照行着色；边界：只识别行首等级，正文关键词不改变等级。
var logLevelPattern = regexp.MustCompile(`(?i)^\s*\[?\s*(fatal|error|warning|warn|info|message|debug)\b`)

// LogPath 职责：将来源枚举映射到固定日志；使用：读写导出前校验；边界：拒绝未知来源，不开放路径输入。
func LogPath(gameDir, source string) (string, error) {
	switch source {
	case "bepinex":
		return filepath.Join(gameDir, "BepInEx", "LogOutput.log"), nil
	case "unity":
		return filepath.Join(gameDir, "output_log.txt"), nil
	default:
		return "", fmt.Errorf("不支持的日志来源: %s", source)
	}
}

// ReadLog 职责：读取日志尾部；使用：控制台周期刷新；边界：缺失返回空快照，未完成末行保留，下次读取会更新。
func ReadLog(gameDir, source string) (*contract.HeadlessLogSnapshot, error) {
	path, err := LogPath(gameDir, source)
	if err != nil {
		return nil, err
	}
	result := &contract.HeadlessLogSnapshot{Path: path, Lines: []contract.HeadlessLogLine{}}
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	result.Exists = true
	result.UpdatedAt = info.ModTime().Format(time.RFC3339)
	start := max(int64(0), info.Size()-256*1024)
	result.Truncated = start > 0
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(file, 256*1024))
	if err != nil {
		return nil, err
	}
	text := strings.ToValidUTF8(string(data), "\uFFFD")
	if start > 0 {
		if newline := strings.IndexByte(text, '\n'); newline >= 0 {
			text = text[newline+1:]
		} else {
			text = ""
		}
	}
	lines := strings.Split(strings.TrimRight(text, "\r\n"), "\n")
	if len(lines) > 300 {
		result.Truncated = true
		lines = lines[len(lines)-300:]
	}
	for _, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			continue
		}
		level := "info"
		if match := logLevelPattern.FindStringSubmatch(line); len(match) > 1 {
			switch strings.ToLower(match[1]) {
			case "fatal", "error":
				level = "error"
			case "warning", "warn":
				level = "warning"
			case "debug":
				level = "debug"
			}
		}
		result.Lines = append(result.Lines, contract.HeadlessLogLine{Content: line, Level: level})
	}
	return result, nil
}

// ExportLog 职责：复制完整固定源日志到用户选择文件；使用：服务确认导出路径后调用；边界：拒绝覆盖源文件，文件写入错误透传。
func ExportLog(gameDir, source, target string) error {
	path, err := LogPath(gameDir, source)
	if err != nil {
		return err
	}
	if strings.EqualFold(filepath.Clean(path), filepath.Clean(target)) {
		return fmt.Errorf("不能覆盖原始日志")
	}
	input, err := os.Open(path)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.Create(target)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
