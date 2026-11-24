package feedback

// 文件职责：构造星系查询参数并校验批量预览的 ID。
// 使用：Wails 查询入口将 DTO 转为 HTTP 查询参数或批量请求的 ID 列表。
// 边界：范围、视区版本和分页的业务语义由服务端校验。

import (
	api "DysonSphereBox/apiDefinition/feedback"
	"math"
	"net/url"
	"strconv"
)

// PreviewIDs 职责：校验十进制正 uint64 字符串并按首次出现顺序去重。
// 使用：批量预览在发送请求前处理 Wails 输入。边界：原始列表须有 1 至 128 项，任一项非法则整批拒绝。
func PreviewIDs(raw []string) ([]string, error) {
	if len(raw) < 1 || len(raw) > 128 {
		return nil, InvalidInput("每批预览需要 1 至 128 个星星 ID")
	}
	seen := make(map[string]struct{}, len(raw))
	ids := make([]string, 0, len(raw))
	for _, id := range raw {
		value, err := strconv.ParseUint(id, 10, 64)
		if err != nil || value == 0 || strconv.FormatUint(value, 10) != id {
			return nil, InvalidInput("星星 ID 必须是规范的正 uint64 十进制字符串")
		}
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// MapQuery 序列化查询坐标和快照版本，确保四个坐标同时出现或一起省略。
func MapQuery(input api.MapRequest) (url.Values, error) {
	query := url.Values{}
	if input.Version != "" {
		query.Set("version", input.Version)
	}

	coordinates := []struct {
		name  string
		value *float64
	}{
		{"minX", input.MinX},
		{"minY", input.MinY},
		{"maxX", input.MaxX},
		{"maxY", input.MaxY},
	}
	present := 0
	for _, coordinate := range coordinates {
		if coordinate.value == nil {
			continue
		}
		value := *coordinate.value
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, InvalidInput("请提供有效的星图坐标")
		}
		query.Set(coordinate.name, strconv.FormatFloat(value, 'f', -1, 64))
		present++
	}
	if present != 0 && present != len(coordinates) {
		return nil, InvalidInput("请同时提供四个星图坐标")
	}
	return query, nil
}

// PageQuery 编码列表分页，Go 零值表示省略并由服务端应用默认值。
func PageQuery(page, size int) url.Values {
	query := url.Values{}
	if page != 0 {
		query.Set("page", strconv.Itoa(page))
	}
	if size != 0 {
		query.Set("pageSize", strconv.Itoa(size))
	}
	return query
}
