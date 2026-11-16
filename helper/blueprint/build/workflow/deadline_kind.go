package workflow

// 文件职责：公开局部候选截止的只读错误分类；使用：服务错误投影；边界：不把正常优化宽限结束或业务无解分类成超时。
import "errors"

// IsCandidateDeadline 职责：识别引擎局部候选预算耗尽；使用：领域适配输出明确超时；边界：仅检查原因链，不创建或更改错误。
func IsCandidateDeadline(err error) bool { return errors.Is(err, localDeadline) }
