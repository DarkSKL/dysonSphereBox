package marketplace

// 文件职责：提供统一市场包的关键字匹配能力。
// 使用：Thunderstore 列表查询转换后调用，以统一执行本地搜索过滤。
// 边界：不访问远端 API、不读取缓存、不写入安装记录；只读取内存中的 DTO 字段。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	"strings"
)

// matchesPackageKeyword 判断统一包 DTO 是否匹配搜索关键字。
// 职责：在名称、描述、作者和命名空间中执行大小写不敏感包含匹配。
// 使用：Thunderstore 列表转换后过滤包列表。
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
