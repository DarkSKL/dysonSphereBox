package modDependence

import (
	"errors"
	"fmt"
)

// ArtifactInstallError 表示统一来源安装包中某个组件的安装失败。
// 用途：Thunderstore 依赖链安装失败时，把具体组件、阶段和下载地址写入日志。
type ArtifactInstallError struct {
	Stage            string
	PackageKey       string
	ComponentKey     string
	ComponentName    string
	ComponentVersion string
	DownloadURL      string
	Err              error
}

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

// Unwrap 返回原始错误，便于 errors.Is/errors.As 继续识别网络或文件系统错误。
func (err *ArtifactInstallError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Err
}

func wrapArtifactInstallError(stage string, artifact installArtifact, err error) error {
	if err == nil {
		return nil
	}

	var artifactErr *ArtifactInstallError
	if errors.As(err, &artifactErr) {
		return err
	}

	return &ArtifactInstallError{
		Stage:            stage,
		PackageKey:       artifact.PackageKey,
		ComponentKey:     artifact.ComponentKey,
		ComponentName:    artifact.DisplayName,
		ComponentVersion: artifact.Version,
		DownloadURL:      artifact.DownloadURL,
		Err:              err,
	}
}

// IsInstallConflictError 判断错误是否为需要用户处理的安装冲突。
// 用途：服务层遇到该错误时保留 conflict 状态，不把它改写成普通 failed。
func IsInstallConflictError(err error) bool {
	var conflictErr *installFileConflictError
	if errors.As(err, &conflictErr) {
		return true
	}
	var artifactErr *ArtifactInstallError
	if errors.As(err, &artifactErr) {
		return IsInstallConflictError(artifactErr.Err)
	}
	return false
}

// IsInstallCancelledError 判断错误是否来自用户主动取消安装。
// 用途：服务层遇到取消时写入“已取消”状态，而不是展示成网络或文件失败。
func IsInstallCancelledError(err error) bool {
	if errors.Is(err, errInstallCancelled) {
		return true
	}
	var artifactErr *ArtifactInstallError
	if errors.As(err, &artifactErr) {
		return IsInstallCancelledError(artifactErr.Err)
	}
	return false
}
