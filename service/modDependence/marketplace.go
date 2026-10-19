package modDependence

// 文件职责：提供 Mod 市场查询、刷新与可选描述翻译服务。
// 使用：Mod 市场页通过生成绑定加载远端包，用户开启中文后翻译可见描述。
// 边界：来源分派、缓存和 DTO 转换由 marketplace helper 负责，翻译鉴权由 Go helper 处理。

import (
	ModDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	translationAPI "DysonSphereBox/apiDefinition/modTranslation"
	sourceConstant "DysonSphereBox/constant/modDependence/source"
	marketplaceHelper "DysonSphereBox/helper/modDependence/marketplace"
	marketplaceContract "DysonSphereBox/helper/modDependence/marketplace/contract"
	translationHelper "DysonSphereBox/helper/modDependence/translation"
	"context"
)

// MarketplaceService 提供前端可调用的 Mod 市场查询能力。
// 职责：编排 Thunderstore 和手动来源的统一市场查询。
// 使用：Mod 市场页面加载、刷新、搜索分页时通过 Wails 绑定调用。
// 边界：市场查询直连 Thunderstore；可选翻译由 translation helper 请求应用服务器。
type MarketplaceService struct {
}

// TranslateDescriptions 职责：暴露可选的描述批量翻译；使用：市场页为可见描述传入最多二十项及会话标识；边界：token 仅由 Go 持有，错误由页面保留原文并手动重试。
func (service *MarketplaceService) TranslateDescriptions(texts []string, sessionKey string) (translationAPI.BatchData, error) {
	return translationHelper.TranslateBatch(context.Background(), texts, sessionKey)
}

// GetRemotePackageList 获取指定来源的统一 Mod 包分页列表。
// 职责：按来源、分页和关键字读取统一远端包列表。
// 使用：source 支持 thunderstore、manual；keyword 为空时返回默认分页。
// 边界：Thunderstore 使用本地缓存与分页，manual 当前返回空列表等待导入流程填充。
func (service *MarketplaceService) GetRemotePackageList(source string, page int, pageSize int, keyword string) (ModDependenceResponse.RemotePackageListData, error) {
	return marketplaceHelper.ListRemotePackages(context.Background(), marketplaceContract.RemotePackageQuery{
		Source:   sourceConstant.ModSource(source),
		Page:     page,
		PageSize: pageSize,
		Keyword:  keyword,
		UseCache: true,
	})
}

// RefreshRemotePackageList 强制刷新指定来源的统一 Mod 包分页列表。
// 职责：绕过可复用缓存并重新读取指定来源的远端包列表。
// 使用：前端点击刷新时调用，Thunderstore 会绕过未过期缓存重新拉取来源数据。
// 边界：刷新只影响市场缓存，不修改安装状态或本地文件。
func (service *MarketplaceService) RefreshRemotePackageList(source string, page int, pageSize int, keyword string) (ModDependenceResponse.RemotePackageListData, error) {
	return marketplaceHelper.ListRemotePackages(context.Background(), marketplaceContract.RemotePackageQuery{
		Source:   sourceConstant.ModSource(source),
		Page:     page,
		PageSize: pageSize,
		Keyword:  keyword,
		UseCache: false,
	})
}
