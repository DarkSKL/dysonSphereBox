package shared

// 文件职责：维护安装器内部可复用的路径片段清洗规则。
// 使用：旧官方整合包缓存目录、V2 artifact 解压目录、备份目录和插件目录命名时调用。
// 边界：只处理单个路径片段，不拼接完整路径，不判断父子目录安全关系；目录越界校验仍由 fileops 负责。

import (
	"regexp"
	"strings"
)

// invalidPathSegmentPattern 职责：匹配 Windows 与通用文件系统不安全的路径片段字符。
// 使用：SanitizePathSegment 在替换非法字符时复用该正则。
// 边界：只覆盖路径片段级别，不检查保留设备名或完整路径权限。
var invalidPathSegmentPattern = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]+`)

// SanitizePathSegment 将任意展示文本清洗成可用于单级目录或文件名的片段。
// 职责：去除空白、替换非法字符、裁剪危险结尾并限制长度。
// 使用：根据包名、版本号、组件 key 构造缓存目录、zip 文件名或插件目录时调用。
// 边界：空值或清洗后为空时返回 fallback；不保证生成值全局唯一，调用方需要自行加入 ID 或 key。
func SanitizePathSegment(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}

	value = invalidPathSegmentPattern.ReplaceAllString(value, "-")
	value = strings.Trim(value, ". ")
	if value == "" {
		return fallback
	}
	if len(value) > 80 {
		value = value[:80]
	}

	return value
}
