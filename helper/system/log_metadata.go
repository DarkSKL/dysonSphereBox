package system

// 文件职责：序列化系统日志扩展 metadata。
// 使用：RecordLog 创建持久化模型前调用 marshalMetadata。
// 边界：空 metadata 写入空 JSON 对象；不可序列化内容返回错误，不创建日志。

import (
	"encoding/json"
	"fmt"

	"gorm.io/datatypes"
)

// marshalMetadata 序列化日志扩展信息。
// 职责：把 map 类型 metadata 转换为 SQLite JSON 字段。
// 使用：RecordLog 创建持久化模型前调用。
// 边界：空 metadata 写入空 JSON 对象；不可序列化内容返回错误。
func marshalMetadata(metadata map[string]interface{}) (datatypes.JSON, error) {
	if len(metadata) == 0 {
		return datatypes.JSON([]byte("{}")), nil
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal system log metadata: %w", err)
	}

	return datatypes.JSON(data), nil
}
