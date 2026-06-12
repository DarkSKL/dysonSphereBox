package marketplace

// 文件职责：对接官方 Mod 依赖包分页 API 并返回统一市场分页结果。
// 使用：由市场 Provider 在 source=official 时调用，供前端列表展示和安装入口复用官方返回数据。
// 边界：不缓存响应、不下载 zip、不写安装状态；DTO 转换和 URL 解析由同包独立文件负责。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	"DysonSphereBox/core"
	"DysonSphereBox/helper/modDependence/marketplace/contract"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// listOfficialRemotePackages 读取官方源整合包并转换为统一市场分页。
// 职责：调用官方分页接口、校验响应并将旧 DTO 转为 RemoteModPackageDTO。
// 使用：ListRemotePackages 在 Source 为 official 时调用。
// 边界：只做市场查询和 DTO 转换，不写安装记录，不下载整合包。
// 逻辑流：1. 构造分页请求 => 2. 调用官方 API => 3. 校验 HTTP 与业务码 => 4. 转换并按关键字过滤 => 5. 返回统一分页。
func listOfficialRemotePackages(ctx context.Context, query contract.RemotePackageQuery) (modDependenceResponse.RemotePackageListData, error) {
	values := url.Values{}
	values.Set("page", strconv.Itoa(query.Page))
	values.Set("pageSize", strconv.Itoa(query.PageSize))
	requestPath := modDependenceResponse.APIURLPackageList + "?" + values.Encode()

	request, err := core.GLOBAL.HTTPClient.NewRequest(ctx, http.MethodGet, requestPath, nil)
	if err != nil {
		return modDependenceResponse.RemotePackageListData{}, err
	}

	response, err := core.GLOBAL.HTTPClient.Do(request)
	if err != nil {
		return modDependenceResponse.RemotePackageListData{}, err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return modDependenceResponse.RemotePackageListData{}, fmt.Errorf("get official mod package list failed: %s", response.Status)
	}

	var result modDependenceResponse.PackageListResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return modDependenceResponse.RemotePackageListData{}, err
	}
	if result.Code != 0 {
		return modDependenceResponse.RemotePackageListData{}, fmt.Errorf("get official mod package list failed: %s", result.Msg)
	}

	list := make([]modDependenceResponse.RemoteModPackageDTO, 0, len(result.Data.List))
	for _, pack := range result.Data.List {
		remotePack := officialPackageToRemote(pack)
		if matchesPackageKeyword(remotePack, query.Keyword) {
			list = append(list, remotePack)
		}
	}

	return modDependenceResponse.RemotePackageListData{
		List:     list,
		Total:    result.Data.Total,
		Page:     result.Data.Page,
		PageSize: result.Data.PageSize,
		Source:   query.Source,
	}, nil
}
