package contract

// 文件职责：定义 artifact 安装失败时跨包可识别的错误契约。
// 使用：installer 实现层包装下载、解压、规划、复制、记录阶段错误，audit 层通过 errors.As 展开结构化元数据。
// 边界：只承载错误上下文，不执行重试、清理、回滚或用户提示渲染。

import "fmt"

// ArtifactInstallError 表示 Mod 包中某个组件的安装失败。
// 职责：携带失败阶段、组件信息、下载地址和原始错误，供上层审计和提示使用。
// 使用：安装 artifact 的下载、解压、规划、复制和记录阶段通过该错误包装底层失败。
// 边界：该错误不执行回滚，回滚由调用链捕获错误后统一处理。
type ArtifactInstallError struct {
	// Stage 职责：记录失败阶段；使用：前端和审计日志展示 download/extract/plan/copy/record；边界：只作诊断，不驱动回滚分支。
	Stage string
	// PackageKey 职责：记录父安装包 key；使用：日志定位本次安装会话；边界：必须来自当前安装上下文。
	PackageKey string
	// ComponentKey 职责：记录失败组件 key；使用：日志定位依赖组件；边界：可能与 PackageKey 不同。
	ComponentKey string
	// ComponentName 职责：记录失败组件显示名；使用：错误文本和审计展示；边界：为空时 Error 回退 ComponentKey。
	ComponentName string
	// ComponentVersion 职责：记录失败组件版本；使用：日志排查远端包版本；边界：不做版本比较。
	ComponentVersion string
	// DownloadURL 职责：记录失败组件下载地址；使用：排查远端资源问题；边界：本地 artifact 可能为空。
	DownloadURL string
	// Err 职责：保存底层错误；使用：errors.Unwrap/errors.As 保留原始失败原因；边界：可为空，此时只返回阶段失败说明。
	Err error
}

// Error 返回 artifact 安装失败的用户可读说明。
// 职责：实现 error 接口并拼接阶段、组件和底层错误。
// 使用：service、audit 和进度失败文本读取该说明。
// 边界：nil 接收者返回空字符串，避免错误处理链二次 panic。
func (err *ArtifactInstallError) Error() string {
	if err == nil {
		return ""
	}
	componentName := err.ComponentName
	if componentName == "" {
		componentName = err.ComponentKey
	}
	if err.Err == nil {
		return fmt.Sprintf("%s 阶段失败：%s", err.Stage, componentName)
	}
	return fmt.Sprintf("%s 阶段失败：%s：%v", err.Stage, componentName, err.Err)
}

// Unwrap 返回 artifact 安装失败的底层错误。
// 职责：支持 errors.Is 与 errors.As 继续识别原始文件、网络或取消错误。
// 使用：调用方判断取消、冲突或底层 IO 错误时使用。
// 边界：nil 接收者返回 nil，不改变错误链语义。
func (err *ArtifactInstallError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Err
}
