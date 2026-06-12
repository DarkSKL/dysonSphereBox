package installerror

// 文件职责：提供 V2 安装器的结构化错误包装、冲突错误类型和特殊错误识别。
// 使用：artifact 安装阶段用 WrapArtifact 包装上下文，复制阶段用 Conflict 暂停安装，服务层用 IsConflict 和 IsCancelled 区分展示状态。
// 边界：不执行回滚、清理、进度发送或审计记录；错误被识别后由上层安装流程决定恢复策略。

import (
	"context"
	"errors"
	"strings"

	"DysonSphereBox/helper/modDependence/fileops"
	"DysonSphereBox/helper/modDependence/installer/contract"
	v2Artifact "DysonSphereBox/helper/modDependence/installer/v2/artifact"
)

// Conflict 表示安装流程因文件冲突暂停。
// 职责：把冲突 DTO 包装成 error，交由上层持久化 conflict 状态。
// 使用：复制计划文件时遇到需要用户决策的冲突后返回。
// 边界：不代表不可恢复失败，用户继续选择后可恢复安装。
type Conflict struct {
	// Conflict 职责：保存需要前端处理的冲突详情；使用：持久化安装会话；边界：必须包含 packageKey 和 targetPath。
	Conflict contract.InstallConflictDTO
}

// Error 返回安装冲突的用户可读提示。
// 职责：实现 error 接口并返回冲突说明。
// 使用：错误传播、日志和前端提示读取该文本。
// 边界：该错误用于暂停安装并等待前端选择覆盖、跳过或终止。
func (err *Conflict) Error() string {
	return err.Conflict.Message
}

// WrapArtifact 将 artifact 阶段错误包装为可审计的契约错误。
// 职责：补齐阶段、包、组件、版本和下载地址上下文。
// 使用：下载、解压、规划、复制和记录 artifact 失败时调用。
// 边界：nil 错误直接返回 nil；已包装错误不重复包装，避免错误链膨胀。
func WrapArtifact(stage string, artifact v2Artifact.Artifact, err error) error {
	if err == nil {
		return nil
	}

	var artifactErr *contract.ArtifactInstallError
	if errors.As(err, &artifactErr) {
		return err
	}

	return &contract.ArtifactInstallError{
		Stage:            stage,
		PackageKey:       artifact.PackageKey,
		ComponentKey:     artifact.ComponentKey,
		ComponentName:    artifact.DisplayName,
		ComponentVersion: artifact.Version,
		DownloadURL:      artifact.DownloadURL,
		Err:              err,
	}
}

// IsConflict 判断错误是否为需要用户处理的安装冲突。
// 职责：识别直接冲突错误或被 artifact 错误包裹的冲突错误。
// 使用：服务层遇到该错误时保留 conflict 状态，不把它改写成普通 failed。
// 边界：只做错误分类，不修改安装会话或进度。
func IsConflict(err error) bool {
	var conflictErr *Conflict
	if errors.As(err, &conflictErr) {
		return true
	}
	var artifactErr *contract.ArtifactInstallError
	if errors.As(err, &artifactErr) {
		return IsConflict(artifactErr.Err)
	}
	return false
}

// AsConflict 尝试从错误链中提取安装冲突错误。
// 职责：统一解析直接冲突错误或 artifact 包装后的冲突错误。
// 使用：冲突持久化流程需要读取冲突 DTO 时调用。
// 边界：无法识别冲突时返回 nil，不修改原始错误。
func AsConflict(err error) *Conflict {
	var conflictErr *Conflict
	if errors.As(err, &conflictErr) {
		return conflictErr
	}
	var artifactErr *contract.ArtifactInstallError
	if errors.As(err, &artifactErr) {
		return AsConflict(artifactErr.Err)
	}
	return nil
}

// IsCancelled 判断错误是否来自用户主动取消安装。
// 职责：识别文件操作取消、context 取消和被 artifact 错误包裹的取消。
// 使用：服务层遇到取消时写入“已取消”状态，而不是展示成网络或文件失败。
// 边界：只做错误分类；不会主动调用 cancel 或清理文件。
func IsCancelled(err error) bool {
	if errors.Is(err, fileops.ErrInstallCancelled) {
		return true
	}
	if errors.Is(err, context.Canceled) {
		return true
	}
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "context canceled") {
		return true
	}
	var artifactErr *contract.ArtifactInstallError
	if errors.As(err, &artifactErr) {
		return IsCancelled(artifactErr.Err)
	}
	return false
}
