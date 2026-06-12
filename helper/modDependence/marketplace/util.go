package marketplace

// 文件职责：提供市场包关键字匹配与旧官方 DTO 空值归一化辅助能力。
// 使用：由官方源和 Thunderstore 市场查询转换流程调用，统一搜索过滤和 key 片段处理。
// 边界：不访问远端 API、不读取缓存、不写入安装记录；只处理内存中的 DTO 字段。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	"strings"
)

// matchesPackageKeyword 判断统一包 DTO 是否匹配搜索关键字。
// 职责：在名称、描述、作者和命名空间中执行大小写不敏感包含匹配。
// 使用：官方源和 Thunderstore 列表转换后过滤包列表。
// 边界：空关键字始终返回 true；不做分词或拼音匹配。
func matchesPackageKeyword(pack modDependenceResponse.RemoteModPackageDTO, keyword string) bool {
	keyword = strings.TrimSpace(strings.ToLower(keyword))
	if keyword == "" {
		return true
	}

	candidates := []string{
		pack.DisplayName,
		pack.Name,
		pack.Description,
		pack.Author,
		pack.Namespace,
	}
	for _, candidate := range candidates {
		if strings.Contains(strings.ToLower(candidate), keyword) {
			return true
		}
	}
	return false
}

// int64Value 解引用可选 int64。
// 职责：统一旧官方 DTO 中指针数值的空值处理。
// 使用：officialPackageToRemote 转换下载数、大小和可信度时调用。
// 边界：nil 返回 0，不推断远端默认值。
func int64Value(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

// stringValue 解引用可选字符串。
// 职责：统一旧官方 DTO 中指针字符串的空值处理。
// 使用：officialPackageToRemote 和 officialDependencyToRemote 转换展示字段、URL 和版本号时调用。
// 边界：nil 返回空字符串，不推断默认业务值。
func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// sanitizeKeySegment 清理统一 key 片段。
// 职责：避免版本号等片段中的冒号破坏统一 key 分隔规则。
// 使用：官方源包和依赖 DTO 生成 key 时调用。
// 边界：空值回退到 fallback；只替换冒号，不做文件名安全处理。
func sanitizeKeySegment(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return strings.ReplaceAll(value, ":", "-")
}
