package artifactfetch

// 文件职责：在 V2 官方包安装前记录远端下载计数。
// 使用：主安装流程在 artifact 安装前调用 RecordOfficialPackageDownloadBeforeInstall。
// 边界：只调用官方下载计数接口，不下载 artifact、不写安装模型、不处理非官方来源。

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	sourceConstant "DysonSphereBox/constant/modDependence/source"
	"DysonSphereBox/core"
	"DysonSphereBox/helper/modDependence/installer/contract"
	progressEvent "DysonSphereBox/helper/modDependence/installer/v2/progress/event"
)

// RecordOfficialPackageDownloadBeforeInstall 记录官方包安装下载次数。
// 职责：在首次安装官方来源包前调用后端下载计数接口。
// 使用：主安装流程在 artifact 安装前调用。
// 边界：非官方来源、无 legacyID 或继续冲突安装时跳过；接口失败会终止安装。
// 逻辑流：1. 判断是否官方首次安装 => 2. 构造下载记录请求 => 3. 调用后端计数接口 => 4. 校验业务响应 => 5. 发送准备进度事件。
func RecordOfficialPackageDownloadBeforeInstall(ctx context.Context, pack modDependenceResponse.RemoteModPackageDTO, options contract.InstallRemotePackageOptions) error {
	if pack.Source != sourceConstant.ModSourceOfficial || pack.LegacyID == 0 {
		return nil
	}
	if !options.ResetSession {
		return nil
	}
	if core.GLOBAL.HTTPClient == nil {
		return fmt.Errorf("http client is not initialized")
	}

	body, err := json.Marshal(modDependenceResponse.DownloadRecordRequest{
		ResourceType: "mod_dependency_package",
		ResourceID:   pack.LegacyID,
		Channel:      "windows",
		Extra: map[string]interface{}{
			"packageKey":      pack.Key,
			"packageName":     pack.DisplayName,
			"packageVersion":  pack.Version,
			"rawDownloadURL":  pack.DownloadURL,
			"installScenario": "desktop_install",
		},
	})
	if err != nil {
		return err
	}

	request, err := core.GLOBAL.HTTPClient.NewRequest(ctx, http.MethodPost, modDependenceResponse.APIURLDownloadRecord, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := core.GLOBAL.HTTPClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("record official package download failed: %s", response.Status)
	}

	result := modDependenceResponse.DownloadRecordResponse{}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return err
	}
	if result.Code != 0 {
		return fmt.Errorf("record official package download failed: %s", result.Msg)
	}
	progressEvent.Package(pack.Key, "prepare", fmt.Sprintf("%s 下载计数已记录，当前累计 %d 次", pack.DisplayName, result.Data.DownloadCount), 12)
	return nil
}
