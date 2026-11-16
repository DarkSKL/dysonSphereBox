package build

// 文件职责：以typed复制与同一复制器记账隔离原始请求。
// 使用：工作流Request产物的克隆与预算边界；边界：不执行配置/模式业务规则，保留NaN等无效值到正式输入归一节点。

import "unsafe"

// CloneRequest 职责：完整复制请求可变容器和内部球面字段；使用：任务实例播种和快照读取；边界：不归一设置、不校验业务，不通过JSON丢弃非有限值。
func CloneRequest(request Request) (Request, error) {
	return newFinalizationCloner().buildRequest(request), nil
}

// RequestEstimatedBytes 职责：按同一typed复制器保守计量原始请求保留空间；使用：Request产物预算；边界：不以JSON失败伪造巨大预算，也不执行任何数值或模式校验。
func RequestEstimatedBytes(request Request) int64 {
	cloner := newFinalizationCloner()
	_ = cloner.buildRequest(request)
	return cloner.bytes + int64(unsafe.Sizeof(request)) + 256
}
