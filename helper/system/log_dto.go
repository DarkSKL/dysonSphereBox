package system

// 文件职责：提供系统日志持久化模型到前端 DTO 的转换。
// 使用：LogService.ListLogs 查询数据库后调用 ToLogDTOs。
// 边界：不直接访问数据库；任意一条 Metadata 解码失败会返回错误，由服务层透传。

import (
	"encoding/json"
	"fmt"
	"time"

	systemModel "DysonSphereBox/model/system"
)

// LogDTO 表示前端可安全消费的系统日志。
// 职责：把持久化日志模型转换成 Wails 友好的 JSON DTO。
// 使用：日志页展示审计记录时读取。
// 边界：DateTime 使用字符串，Metadata 使用普通 map，避免 Wails 生成弱类型或暴露 GORM 持久化细节。
type LogDTO struct {
	// ID 职责：保存日志主键；使用：前端列表 key 或调试定位；边界：不暴露数据库写入能力。
	ID uint `json:"id"`
	// Namespace 职责：保存日志所属业务域；使用：前端筛选和展示；边界：为空值应由写入层归一化。
	Namespace string `json:"namespace"`
	// Module 职责：保存业务域下的模块名；使用：前端筛选和展示；边界：为空值应由写入层归一化。
	Module string `json:"module"`
	// Event 职责：保存稳定事件名；使用：检索具体操作类型；边界：不应保存用户可变文案。
	Event string `json:"event"`
	// Level 职责：保存日志等级；使用：前端按严重程度过滤和着色；边界：未知值由写入层归一化。
	Level systemModel.LogLevel `json:"level"`
	// Type 职责：保存日志业务分类；使用：前端按操作、配置、安装等类别过滤；边界：未知值由写入层归一化。
	Type systemModel.LogType `json:"type"`
	// Content 职责：保存面向用户或维护者阅读的日志摘要；使用：日志列表主文案；边界：结构化信息应放入 Metadata。
	Content string `json:"content"`
	// Metadata 职责：保存结构化扩展信息；使用：详情弹窗或诊断查看；边界：必须是 JSON 可序列化数据。
	Metadata map[string]interface{} `json:"metadata"`
	// Error 职责：保存失败详情；使用：日志详情展示错误原因；边界：成功日志保持为空。
	Error string `json:"error"`
	// DateTime 职责：保存日志发生时间；使用：前端排序和展示；边界：使用 RFC3339Nano 字符串避免暴露 time.Time。
	DateTime string `json:"dateTime"`
}

// ToLogDTOs 将持久化模型转换为前端 DTO。
// 职责：批量转换系统日志持久化模型为 Wails 友好的 DTO。
// 使用：LogService.ListLogs 查询数据库后调用。
// 边界：任意一条 Metadata 解码失败会返回错误，由服务层透传。
// 逻辑流：1. 遍历持久化模型 => 2. 单条转换 Metadata 和时间 => 3. 聚合 DTO 列表返回。
func ToLogDTOs(items []systemModel.Log) ([]LogDTO, error) {
	results := make([]LogDTO, 0, len(items))
	for index := range items {
		item, err := toLogDTO(items[index])
		if err != nil {
			return nil, err
		}
		results = append(results, item)
	}

	return results, nil
}

// toLogDTO 转换单条系统日志模型。
// 职责：把 GORM 模型中的 JSON 和时间字段转换为前端安全类型。
// 使用：ToLogDTOs 遍历日志列表时调用。
// 边界：Metadata 解码失败时返回错误，不丢弃异常日志。
func toLogDTO(item systemModel.Log) (LogDTO, error) {
	metadata := make(map[string]interface{})
	if len(item.Metadata) > 0 {
		if err := json.Unmarshal(item.Metadata, &metadata); err != nil {
			return LogDTO{}, fmt.Errorf("decode system log metadata: %w", err)
		}
	}

	dateTime := ""
	if item.DateTime != nil {
		dateTime = item.DateTime.Format(time.RFC3339Nano)
	}

	return LogDTO{
		ID:        item.ID,
		Namespace: item.Namespace,
		Module:    item.Module,
		Event:     item.Event,
		Level:     item.Level,
		Type:      item.Type,
		Content:   item.Content,
		Metadata:  metadata,
		Error:     item.Error,
		DateTime:  dateTime,
	}, nil
}
