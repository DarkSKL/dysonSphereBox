package workflow

// 文件职责：区分配置、无解及致命故障；使用：编译和引擎边界；边界：只有工具白名单声明的无解码可回退。

import "fmt"

// Failure 职责：表达可声明的领域无解；使用：工具返回；边界：未在 ToolSpec 声明时仍为致命失败。
type Failure struct {
	// synthetic 职责：区分内核汇总/截止与真实工具无解；使用：条件组触发过滤；边界：工具适配层不能伪造真实失败来源。
	synthetic bool
	// Code 职责：稳定领域失败码；使用：白名单判断；边界：不匹配则终止。
	Code string
	// Message 职责：解释具体约束；使用：追踪；边界：不参与调度判断。
	Message string
	// Cause 职责：保留领域错误及嵌套候选原因；使用：errors.Is/As 和推进过滤；边界：不因包装自动获得可回退权限。
	Cause error
}

// Error 职责：提供领域错误文本；使用：标准 error；边界：不隐式标记为已声明。
func (e *Failure) Error() string { return e.Code + ": " + e.Message }

// Unwrap 职责：暴露原始领域原因链；使用：标准错误遍历；边界：候选类别仍由白名单检查。
func (e *Failure) Unwrap() error { return e.Cause }

// ExecutionError 职责：保存完整失败定位及可回退分类；使用：Run 返回；边界：Fatal panic 不能回退。
type ExecutionError struct {
	// declaredCode 职责：保存调用边界确认的唯一白名单失败类别；使用：条件组与失败过滤；边界：原始 Cause 中未声明的嵌套码不获得调度授权。
	declaredCode string
	// Node 职责：调用路径；使用：定位；边界：包含子流程与区域身份。
	Node string
	// Tool 职责：稳定工具 ID；使用：追踪；边界：调度错误时可为空。
	Tool string
	// Cause 职责：原始错误；使用：errors.Is/As；边界：不会吞取消。
	Cause error
	// Recoverable 职责：内核确认的无解；使用：候选归约；边界：调用方不能用此对象骗过工具错误白名单。
	Recoverable bool
	// Stack 职责：panic 栈；使用：诊断；边界：正常错误为空。
	Stack string
}

// Error 职责：组合错误定位；使用：标准 error；边界：保留原始原因。
func (e *ExecutionError) Error() string {
	return fmt.Sprintf("node %s tool %s: %v", e.Node, e.Tool, e.Cause)
}

// Unwrap 职责：传播原始错误身份；使用：取消检查；边界：不改变回退分类。
func (e *ExecutionError) Unwrap() error { return e.Cause }

// errConfig 职责：统一配置错误前缀；使用：加载与编译；边界：不代表领域无解。
func errConfig(s string) error { return fmt.Errorf("workflow configuration: %s", s) }
