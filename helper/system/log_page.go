package system

// 文件职责：归一化系统日志分页参数。
// 使用：LogService.ListLogs 构造数据库分页查询前调用 NormalizeLogPage。
// 边界：只修正分页参数，不修改筛选条件、不访问数据库。

const (
	// defaultLogPage 职责：定义日志查询默认页码；使用：NormalizeLogPage 在页码非法时回退；边界：页码从 1 开始。
	defaultLogPage = 1
	// defaultLogPageSize 职责：定义日志查询默认分页大小；使用：NormalizeLogPage 在分页大小非法时回退；边界：不得超过 maxLogPageSize。
	defaultLogPageSize = 50
	// maxLogPageSize 职责：限制单次日志查询最大返回数量；使用：NormalizeLogPage 防止前端一次拉取过多日志；边界：只影响分页参数，不删除日志。
	maxLogPageSize = 200
)

// NormalizeLogPage 归一化日志分页参数。
// 职责：为日志分页提供默认页码、默认分页大小和最大分页限制。
// 使用：LogService.ListLogs 构造数据库分页查询前调用。
// 边界：只修正分页参数，不修改筛选条件。
func NormalizeLogPage(page int, pageSize int) (int, int) {
	if page < 1 {
		page = defaultLogPage
	}
	if pageSize < 1 {
		pageSize = defaultLogPageSize
	}
	if pageSize > maxLogPageSize {
		pageSize = maxLogPageSize
	}

	return page, pageSize
}
