package store

// 文件职责：提供 legacy 模型 JSON 字段的序列化规则。
// 使用：DTO 转模型时处理 gameVersions、tags 等可空 JSON 字段。
// 边界：nil 写入 JSON null；不可序列化值返回错误，不吞掉数据异常。

import (
	"encoding/json"

	"gorm.io/datatypes"
)

// jsonValue 将任意 DTO 字段序列化为 datatypes.JSON。
// 职责：统一 legacy 模型 JSON 字段的空值与序列化行为。
// 使用：DTO 转模型时处理 gameVersions、tags 等字段。
// 边界：nil 写入 JSON null；不可序列化值返回错误。
func jsonValue(value interface{}) (datatypes.JSON, error) {
	if value == nil {
		return datatypes.JSON([]byte("null")), nil
	}

	content, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	return datatypes.JSON(content), nil
}
