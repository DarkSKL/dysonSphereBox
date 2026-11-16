package workflowjournal

// 文件职责：约束日志保留量和边界错误；使用：Store 初始化及服务输入校验；边界：不读取磁盘或配置。

import (
	"fmt"
	"time"
)

// Limits 职责：指定日志保留预算；使用：New 或零值默认；边界：仅允许收紧默认上限，字节统计为保留 JSON，固定元数据由数量限制。
type Limits struct {
	// MaxTasks 职责：限制任务数量；使用：Start；边界：默认八，活动任务不被挤走。
	MaxTasks int
	// MaxEventsPerTask 职责：限制事件数；使用：Append；边界：默认四千零九十六，超限淘汰前缀。
	MaxEventsPerTask int
	// MaxTaskBytes 职责：限制图及事件字节；使用：Start/Append；边界：默认八 MiB。
	MaxTaskBytes int
	// MaxTotalBytes 职责：限制全部任务载荷；使用：全局淘汰；边界：默认三十二 MiB。
	MaxTotalBytes int
	// MaxGraphBytes 职责：限制完整图；使用：Start；边界：默认四 MiB，超限拒绝，不能截断关联。
	MaxGraphBytes int
	// MaxEventBytes 职责：限制单条事件；使用：Append；边界：默认六十四 KiB。
	MaxEventBytes int
	// MaxPageSize 职责：限制分页数量；使用：Read；边界：默认一百二十八。
	MaxPageSize int
	// TTL 职责：规定完成后保留时间；使用：惰性过期；边界：默认十分钟，读操作不延期。
	TTL time.Duration
}

// Error 职责：提供稳定写入失败码；使用：生命周期接线；边界：不作为生成算法错误。
type Error struct {
	// Code 职责：标记 conflict/closed/capacity/finished/expired/invalid_argument/event_too_large；使用：errors.As；边界：不依赖错误文本解析。
	Code string
	// Message 职责：解释失败；使用：调用方诊断；边界：固定简短文本。
	Message string
}

// Error 职责：实现 error；使用：标准错误链；边界：不泄漏领域实体。
func (e *Error) Error() string { return e.Code + ": " + e.Message }

// fault 职责：统一构造边界错误；使用：Store；边界：文本来自固定描述。
func fault(code, message string) error { return &Error{Code: code, Message: message} }

// normalized 职责：补齐并校验预算；使用：New 及零值初始化；边界：拒绝负数和扩大默认限制。
func normalized(l Limits) (Limits, error) {
	values := []*int{&l.MaxTasks, &l.MaxEventsPerTask, &l.MaxTaskBytes, &l.MaxTotalBytes, &l.MaxGraphBytes, &l.MaxEventBytes, &l.MaxPageSize}
	defaults := []int{8, 4096, 8 << 20, 32 << 20, 4 << 20, 64 << 10, 128}
	for index, value := range values {
		if *value < 0 || *value > defaults[index] {
			return l, fmt.Errorf("日志预算超出允许范围")
		}
		if *value == 0 {
			*value = defaults[index]
		}
	}
	if l.TTL < 0 || l.TTL > 10*time.Minute {
		return l, fmt.Errorf("日志保留期超出允许范围")
	}
	if l.TTL == 0 {
		l.TTL = 10 * time.Minute
	}
	if l.MaxGraphBytes > l.MaxTaskBytes || l.MaxTaskBytes > l.MaxTotalBytes || l.MaxEventBytes > l.MaxTaskBytes {
		return l, fmt.Errorf("日志字节预算关系无效")
	}
	return l, nil
}
