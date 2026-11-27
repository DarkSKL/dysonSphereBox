package translation

// 文件职责：经固定账号快照发送有界批量翻译请求并校验结果。
// 使用：市场服务在用户主动开启中文时调用；凭据只在 Go 内存使用。
// 边界：不访问供应商、不记录正文或凭据、不自动重试；切换会话后拒绝旧响应。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	translationAPI "DysonSphereBox/apiDefinition/modTranslation"
	"DysonSphereBox/core/request"
	accountHelper "DysonSphereBox/helper/system/account"
)

// EncodeRequest 职责：校验批次并编码原文；使用：服务入口发请求前调用；边界：非法单项交给服务器返回条目错误，不改变原文。
func EncodeRequest(texts []string) ([]byte, error) {
	if len(texts) == 0 || len(texts) > translationAPI.MaxBatchSize {
		return nil, errors.New("翻译批次需要 1 至 20 项")
	}
	for _, text := range texts {
		if !utf8.ValidString(text) {
			return nil, errors.New("翻译原文编码无效")
		}
	}
	raw, err := json.Marshal(translationAPI.BatchRequest{Texts: texts})
	if err != nil || len(raw) > translationAPI.MaxRequestBytes {
		return nil, errors.New("翻译请求超过 128 KiB")
	}
	return raw, nil
}

// TranslateBatch 职责：发送固定会话的翻译批次；使用：MarketplaceService 传入原文及无凭据作用域；边界：未验证、切号、切服或鉴权失败均不返回译文。
// 逻辑流：1. 校验批次和会话 => 2. 固定目标/token 请求 => 3. 有界解码并处理鉴权 => 4. 再核对会话并校验同序结果。
func TranslateBatch(caller context.Context, texts []string, sessionKey string) (translationAPI.BatchData, error) {
	var empty translationAPI.BatchData
	raw, err := EncodeRequest(texts)
	if err != nil {
		return empty, err
	}
	if sessionKey == "" {
		return empty, errors.New("请先登录并验证账号后重试翻译")
	}
	data, err := request.Do[json.RawMessage](caller, accountHelper.ServerRequests(), translationAPI.TranslateBatchAPI, request.Input{
		SessionKey: sessionKey, Payload: json.RawMessage(raw),
	})
	if err != nil {
		return empty, translationFailure(err)
	}
	return DecodeBatch(data, texts)
}

// translationFailure 将传输、认证和协议异常映射为既有市场提示。
func translationFailure(err error) error {
	if detail := request.Details(err); detail != nil {
		switch detail.Kind {
		case request.AuthRequired, request.PasswordRequired:
			return errors.New("请先登录并验证账号后重试翻译")
		case request.Stale:
			return errors.New("账号或服务器已切换，已丢弃翻译响应")
		case request.Unauthorized:
			return errors.New("登录已失效，请登录后重试翻译")
		case request.ReadFailed:
			return errors.New("翻译响应读取失败或超出限制")
		case request.InvalidEnvelope, request.InvalidData:
			return errors.New("翻译响应格式无效")
		case request.HTTPFailed, request.BusinessFailed:
			return errors.New("翻译暂不可用，请检查登录状态后手动重试")
		}
	}
	return errors.New("翻译请求失败，请手动重试")
}

// DecodeBatch 职责：校验等长结果并恢复失败原文；使用：HTTP 成功后及契约测试；边界：畸形批次整体失败，单项错误不污染其它项。
func DecodeBatch(raw json.RawMessage, texts []string) (translationAPI.BatchData, error) {
	var payload struct {
		Items []json.RawMessage `json:"items"`
	}
	if json.Unmarshal(raw, &payload) != nil || len(payload.Items) != len(texts) {
		return translationAPI.BatchData{}, errors.New("翻译结果数量或格式不匹配")
	}
	result := translationAPI.BatchData{Items: make([]translationAPI.Item, len(payload.Items))}
	for i, rawItem := range payload.Items {
		trimmed := bytes.TrimSpace(rawItem)
		if len(trimmed) == 0 || trimmed[0] != '{' || json.Unmarshal(trimmed, &result.Items[i]) != nil {
			return translationAPI.BatchData{}, errors.New("翻译结果数量或格式不匹配")
		}
		item := &result.Items[i]
		if item.Status == "ready" && item.Reason == "" && strings.TrimSpace(item.Data) != "" {
			continue
		}
		item.Status, item.Data = "error", texts[i]
		switch item.Reason {
		case "invalidItem", "translationDisabled", "providerFailed", "storageUnavailable":
		default:
			item.Reason = "providerFailed"
		}
	}
	return result, nil
}
