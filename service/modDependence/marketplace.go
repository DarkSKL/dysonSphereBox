package modDependence

import (
	modDependenceHelper "DysonSphereBox/helper/modDependence"
	ModDependenceResponse "DysonSphereBox/model/modDependence/response"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"DysonSphereBox/apiDefinition"
	"DysonSphereBox/core"
)

type MarketplaceService struct {
}

// GetRemotePackageList 获取指定来源的统一 Mod 包分页列表。
// 使用：source 支持 official、thunderstore、manual；keyword 为空时返回默认分页。
// 边界：国内源会转成统一 DTO，Thunderstore 使用本地缓存与分页，manual 当前返回空列表等待导入流程填充。
func (service *MarketplaceService) GetRemotePackageList(source string, page int, pageSize int, keyword string) (ModDependenceResponse.RemotePackageListData, error) {
	return modDependenceHelper.ListRemotePackages(context.Background(), modDependenceHelper.RemotePackageQuery{
		Source:   ModDependenceResponse.ModSource(source),
		Page:     page,
		PageSize: pageSize,
		Keyword:  keyword,
		UseCache: true,
	})
}

// RefreshRemotePackageList 强制刷新指定来源的统一 Mod 包分页列表。
// 用途：前端点击刷新时调用，Thunderstore 会绕过未过期缓存重新拉取来源数据。
func (service *MarketplaceService) RefreshRemotePackageList(source string, page int, pageSize int, keyword string) (ModDependenceResponse.RemotePackageListData, error) {
	return modDependenceHelper.ListRemotePackages(context.Background(), modDependenceHelper.RemotePackageQuery{
		Source:   ModDependenceResponse.ModSource(source),
		Page:     page,
		PageSize: pageSize,
		Keyword:  keyword,
		UseCache: false,
	})
}

// GetPackageList 获取远端 Mod 整合包分页列表。
// 边界：返回 response DTO，避免把 GORM 模型中的 time.Time 暴露给 Wails 模块生成器。
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
	requestPath := apiDefinition.API_URL_MOD_DEPENDENCE_LIST + "?" + query.Encode()

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
