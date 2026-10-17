package announcement

// 文件职责：严格验证公告HTTP协议字段与时间窗口。
// 使用：有界请求拿到完整JSON后调用纯校验函数。
// 边界：不访问会话、数据库或网络；非法协议不等同于无公告。

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"

	systemAPI "DysonSphereBox/apiDefinition/system"
)

// requireFields 职责：区分字段缺失与显式null；使用：对象解码前；边界：允许未知扩展字段。
func requireFields(raw []byte, names ...string) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if !utf8.Valid(raw) || json.Unmarshal(raw, &fields) != nil || fields == nil {
		return nil, errors.New("公告响应不是有效JSON对象")
	}
	for _, name := range names {
		if _, ok := fields[name]; !ok {
			return nil, fmt.Errorf("公告响应缺少字段：%s", name)
		}
	}
	return fields, nil
}

// requireValues 职责：拒绝必需标量的显式null；使用：非可空分页和已读字段；边界：可空startsAt、endsAt、readAt仍由业务校验。
func requireValues(raw []byte, names ...string) error {
	fields, err := requireFields(raw, names...)
	if err != nil {
		return err
	}
	for _, name := range names {
		if bytes.Equal(bytes.TrimSpace(fields[name]), []byte("null")) {
			return fmt.Errorf("公告响应字段不可为空：%s", name)
		}
	}
	return nil
}

// positiveID 职责：验证正整数十进制稳定ID；使用：主体及公告字段；边界：不转换成浮点数，不接受符号或前导零。
func positiveID(value string) bool {
	n, ok := new(big.Int).SetString(value, 10)
	return ok && n.Sign() > 0 && n.String() == value
}

// parseUTC 职责：解析协议UTC RFC3339时间；使用：所有业务时间；边界：拒绝非UTC偏移或非法时间。
func parseUTC(value string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, errors.New("公告时间格式无效")
	}
	_, offset := t.Zone()
	if offset != 0 {
		return time.Time{}, errors.New("公告时间必须为UTC")
	}
	return t, nil
}

// validateLatest 职责：验证完整最新查询data；使用：GET成功后；边界：仅明确null表示无公告，任何非法字段均失败。
func validateLatest(raw []byte, subject string) (*NoticeLatestDTO, error) {
	if _, err := requireFields(raw, "subjectId", "serverTime", "item"); err != nil {
		return nil, err
	}
	var data systemAPI.ClientNoticeLatestData
	if json.Unmarshal(raw, &data) != nil || !positiveID(data.SubjectID) || data.SubjectID != subject {
		return nil, errors.New("公告主体身份不匹配")
	}
	now, err := parseUTC(data.ServerTime)
	if err != nil {
		return nil, err
	}
	result := &NoticeLatestDTO{SubjectID: data.SubjectID, ServerTime: data.ServerTime}
	if bytes.Equal(bytes.TrimSpace(data.Item), []byte("null")) {
		return result, nil
	}
	if _, err := requireFields(data.Item, "id", "revision", "kind", "title", "contentFormat", "content", "publishedAt", "startsAt", "endsAt"); err != nil {
		return nil, err
	}
	var item systemAPI.ClientNoticeItem
	if json.Unmarshal(data.Item, &item) != nil {
		return nil, errors.New("公告字段类型无效")
	}
	if !positiveID(item.ID) || item.Revision < 1 || item.Revision > 2147483647 || (item.Kind != "notice" && item.Kind != "announcement" && item.Kind != "promotion") || (item.ContentFormat != "html" && item.ContentFormat != "markdown") {
		return nil, errors.New("公告ID、版本或类型无效")
	}
	if strings.TrimSpace(item.Title) == "" || utf8.RuneCountInString(item.Title) > 200 || strings.TrimSpace(item.Content) == "" || len(item.Content) > 256<<10 {
		return nil, errors.New("公告标题或正文超出协议限制")
	}
	if _, err := parseUTC(item.PublishedAt); err != nil {
		return nil, err
	}
	var start, end time.Time
	if item.StartsAt != nil {
		if start, err = parseUTC(*item.StartsAt); err != nil {
			return nil, err
		}
		if now.Before(start) {
			return nil, errors.New("公告尚未生效")
		}
	}
	if item.EndsAt != nil {
		if end, err = parseUTC(*item.EndsAt); err != nil {
			return nil, err
		}
		if !now.Before(end) {
			return nil, errors.New("公告已到期")
		}
	}
	if item.StartsAt != nil && item.EndsAt != nil && !start.Before(end) {
		return nil, errors.New("公告有效时间窗口无效")
	}
	result.Item = &item
	return result, nil
}

// validateReceipt 职责：核对完整幂等回执；使用：用户确认POST成功后；边界：不接受其他主体、ID或版本的成功。
func validateReceipt(raw []byte, subject, id string, revision int64) (*MarkReadDTO, error) {
	if _, err := requireFields(raw, "subjectId", "id", "revision", "firstReadAt"); err != nil {
		return nil, err
	}
	var receipt MarkReadDTO
	if json.Unmarshal(raw, &receipt) != nil || receipt.SubjectID != subject || receipt.ID != id || receipt.Revision != revision {
		return nil, errors.New("公告确认回执不匹配")
	}
	if _, err := parseUTC(receipt.FirstReadAt); err != nil {
		return nil, err
	}
	return &receipt, nil
}

// validateHistory 职责：严格校验分页历史条目与首次已读时间；使用：历史GET成功后；边界：允许过期条目查看但不替代用户确认时效判断。
func validateHistory(raw []byte, subject string, page, pageSize int64) (*NoticeHistoryDTO, error) {
	if err := requireValues(raw, "subjectId", "serverTime", "page", "pageSize", "total", "list"); err != nil {
		return nil, err
	}
	var data systemAPI.ClientNoticeHistoryData
	if json.Unmarshal(raw, &data) != nil || !positiveID(data.SubjectID) || data.SubjectID != subject {
		return nil, errors.New("公告主体身份不匹配")
	}
	serverTime, err := parseUTC(data.ServerTime)
	if err != nil {
		return nil, err
	}
	if data.Page != page || data.PageSize != pageSize || data.Total < 0 || data.Page < 1 || data.PageSize < 1 || data.PageSize > 100 {
		return nil, errors.New("公告分页元数据无效")
	}
	if data.List == nil || int64(len(data.List)) > data.PageSize {
		return nil, errors.New("公告历史列表格式无效")
	}
	result := &NoticeHistoryDTO{SubjectID: data.SubjectID, ServerTime: data.ServerTime, Total: data.Total, Page: data.Page, PageSize: data.PageSize, List: make([]systemAPI.ClientNoticeHistoryEntry, 0, len(data.List))}
	seen := make(map[string]struct{}, len(data.List))
	for _, rawEntry := range data.List {
		if _, err := requireFields(rawEntry, "id", "revision", "kind", "title", "contentFormat", "content", "publishedAt", "startsAt", "endsAt", "state", "read", "readAt"); err != nil {
			return nil, err
		}
		if err := requireValues(rawEntry, "read"); err != nil {
			return nil, err
		}
		var item systemAPI.ClientNoticeHistoryEntry
		if json.Unmarshal(rawEntry, &item) != nil {
			return nil, errors.New("公告历史条目字段类型无效")
		}
		key := item.ID + ":" + fmt.Sprintf("%d", item.Revision)
		if _, exists := seen[key]; exists {
			return nil, errors.New("公告历史列表含重复版本")
		}
		seen[key] = struct{}{}
		if !positiveID(item.ID) || item.Revision < 1 || item.Revision > 2147483647 || (item.Kind != "notice" && item.Kind != "announcement" && item.Kind != "promotion") || item.State != "published" || (item.ContentFormat != "html" && item.ContentFormat != "markdown") {
			return nil, errors.New("公告ID、版本、类型或状态无效")
		}
		if strings.TrimSpace(item.Title) == "" || utf8.RuneCountInString(item.Title) > 200 || strings.TrimSpace(item.Content) == "" || len(item.Content) > 256<<10 {
			return nil, errors.New("公告标题或正文超出协议限制")
		}
		publishedAt, parseErr := parseUTC(item.PublishedAt)
		if parseErr != nil || publishedAt.After(serverTime) {
			return nil, errors.New("公告发布时间无效")
		}
		var start, end time.Time
		if item.StartsAt != nil {
			if start, err = parseUTC(*item.StartsAt); err != nil || start.After(serverTime) {
				return nil, errors.New("公告开始时间无效")
			}
		}
		if item.EndsAt != nil {
			if end, err = parseUTC(*item.EndsAt); err != nil {
				return nil, errors.New("公告到期时间无效")
			}
		}
		if item.StartsAt != nil && item.EndsAt != nil && !start.Before(end) {
			return nil, errors.New("公告有效时间窗口无效")
		}
		if item.Read != (item.ReadAt != nil) {
			return nil, errors.New("公告已读时间与状态不匹配")
		}
		if item.ReadAt != nil {
			readAt, readErr := parseUTC(*item.ReadAt)
			if readErr != nil || readAt.Before(publishedAt) {
				return nil, errors.New("公告首次已读时间无效")
			}
		}
		result.List = append(result.List, item)
	}
	return result, nil
}
