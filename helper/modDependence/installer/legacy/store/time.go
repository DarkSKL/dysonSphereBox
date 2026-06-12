package store

// 文件职责：解析旧官方接口返回的可空时间字段。
// 使用：packageDTOToModel 和 dependencyDTOToModel 转换更新时间时调用 parseOptionalRemoteTime。
// 边界：空值或无法识别的格式返回 nil 且不报错，避免远端格式漂移阻断安装。

import (
	"strings"
	"time"
)

// parseOptionalRemoteTime 解析旧官方接口返回的可空时间字符串。
// 职责：兼容多种远端时间格式并返回可空 time.Time。
// 使用：DTO 转模型时处理更新时间。
// 边界：空值或无法识别的格式返回 nil 且不报错，避免远端格式漂移阻断安装。
func parseOptionalRemoteTime(value *string) (*time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}

	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	trimmedValue := strings.TrimSpace(*value)
	for _, format := range formats {
		parsed, err := time.Parse(format, trimmedValue)
		if err == nil {
			return &parsed, nil
		}
	}

	return nil, nil
}
