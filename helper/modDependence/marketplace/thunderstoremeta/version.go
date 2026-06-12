package thunderstoremeta

// 文件职责：提供 Thunderstore 包版本选择和时间比较的纯工具。
// 使用：marketplace Thunderstore provider 和其它需要选择最新 active 版本的辅助流程调用。
// 边界：只处理 Thunderstore API 契约数据和时间文本，不发 HTTP 请求、不读缓存、不转换前端 DTO。

import (
	thunderstoreAPI "DysonSphereBox/apiDefinition/modDependence/thunderstore"
	"errors"
	"strings"
	"time"
)

// LatestActiveVersion 选择版本列表中创建时间最新的 active 版本。
// 职责：按创建时间从 active 版本中挑选市场展示和依赖解析使用的版本。
// 使用：Thunderstore 市场列表转换和包解析流程调用。
// 边界：没有 active 版本时返回 false；时间解析失败时回退字符串比较。
func LatestActiveVersion(versions []thunderstoreAPI.Version) (thunderstoreAPI.Version, bool) {
	var latest thunderstoreAPI.Version
	hasLatest := false
	for _, version := range versions {
		if !version.IsActive {
			continue
		}
		if !hasLatest || CompareDate(version.DateCreated, latest.DateCreated) > 0 {
			latest = version
			hasLatest = true
		}
	}

	return latest, hasLatest
}

// CompareDate 比较 Thunderstore 时间文本。
// 职责：优先按时间语义比较版本创建时间。
// 使用：LatestActiveVersion 选择最新版本时调用。
// 边界：任一时间解析失败时回退字符串比较，避免来源格式差异阻断市场展示。
func CompareDate(left string, right string) int {
	leftTime, leftErr := ParseTime(left)
	rightTime, rightErr := ParseTime(right)
	if leftErr == nil && rightErr == nil {
		if leftTime.After(rightTime) {
			return 1
		}
		if leftTime.Before(rightTime) {
			return -1
		}
		return 0
	}

	return strings.Compare(left, right)
}

// ParseTime 按 Thunderstore 常见格式解析时间。
// 职责：兼容 RFC3339 和 Thunderstore 无时区小数秒时间。
// 使用：CompareDate 比较创建时间时调用。
// 边界：所有格式失败时返回最后一次解析错误；空错误会兜底为 empty time。
func ParseTime(value string) (time.Time, error) {
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999",
		"2006-01-02T15:04:05",
	}

	var parseErr error
	for _, format := range formats {
		parsed, err := time.Parse(format, value)
		if err == nil {
			return parsed, nil
		}
		parseErr = err
	}

	if parseErr == nil {
		parseErr = errors.New("empty time")
	}

	return time.Time{}, parseErr
}
