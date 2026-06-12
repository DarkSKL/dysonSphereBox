package modDependence

// 文件职责：提供统一 Mod 市场列表查询、刷新和旧官方分页接口兼容的 Wails 服务。
// 使用：Mod 市场页通过生成绑定按来源、分页和关键字加载远端包。
// 边界：服务层只编排查询入口；来源分派、缓存和 DTO 转换由 marketplace helper 负责。

import (
	ModDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	sourceConstant "DysonSphereBox/constant/modDependence/source"
	"DysonSphereBox/core"
	marketplaceHelper "DysonSphereBox/helper/modDependence/marketplace"
	marketplaceContract "DysonSphereBox/helper/modDependence/marketplace/contract"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// MarketplaceService 提供前端可调用的 Mod 市场查询能力。
// 职责：编排统一来源市场查询和旧官方整合包接口兼容查询。
// 使用：Mod 市场页面加载、刷新、搜索分页时通过 Wails 绑定调用。
// 边界：统一来源列表交给 helper provider 处理，历史官方整合包接口保留在服务方法中兼容旧前端。
type MarketplaceService struct {
}

// GetRemotePackageList 获取指定来源的统一 Mod 包分页列表。
// 职责：按来源、分页和关键字读取统一远端包列表。
// 使用：source 支持 official、thunderstore、manual；keyword 为空时返回默认分页。
// 边界：国内源会转成统一 DTO，Thunderstore 使用本地缓存与分页，manual 当前返回空列表等待导入流程填充。
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

// GetPackageList 获取远端 Mod 整合包分页列表。
// 职责：兼容旧官方整合包分页接口。
// 使用：旧市场页面或仍依赖 PackageListData 的前端逻辑调用。
// 边界：返回 response DTO，避免把 GORM 模型中的 time.Time 暴露给 Wails 模块生成器。
// 逻辑流：1. 规范化分页参数 => 2. 通过 GLOBAL.HTTPClient 发起官方接口请求 => 3. 校验 HTTP 和业务状态码 => 4. 解码并返回分页数据。
func (service *MarketplaceService) GetPackageList(page int, pageSize int) (ModDependenceResponse.PackageListData, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	query := url.Values{}
	query.Set("page", strconv.Itoa(page))
	query.Set("pageSize", strconv.Itoa(pageSize))
	requestPath := ModDependenceResponse.APIURLPackageList + "?" + query.Encode()

	request, err := core.GLOBAL.HTTPClient.NewRequest(context.Background(), http.MethodGet, requestPath, nil)
	if err != nil {
		return ModDependenceResponse.PackageListData{}, err
	}

	response, err := core.GLOBAL.HTTPClient.Do(request)
	if err != nil {
		return ModDependenceResponse.PackageListData{}, err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return ModDependenceResponse.PackageListData{}, fmt.Errorf("get mod dependency package list failed: %s", response.Status)
	}

	var result ModDependenceResponse.PackageListResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return ModDependenceResponse.PackageListData{}, err
	}
	if result.Code != 0 {
		return ModDependenceResponse.PackageListData{}, fmt.Errorf("get mod dependency package list failed: %s", result.Msg)
	}

	return result.Data, nil
}
