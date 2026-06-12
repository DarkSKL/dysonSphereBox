package copyfiles

// 文件职责：编排 V2 安装计划文件复制流程。
// 使用：由 artifact 文件规划流程在拿到 installplan.File 列表后调用，把文件写入游戏目录。
// 边界：不规划 artifact 布局、不保存 installed 模型、不决定服务层日志；会话状态和单文件复制步骤由同包独立文件维护。

import (
	"DysonSphereBox/helper/modDependence/installer/contract"
	v2Artifact "DysonSphereBox/helper/modDependence/installer/v2/artifact"
	"DysonSphereBox/helper/modDependence/installer/v2/installplan"
	"context"
)

// CopyPlannedFiles 复制规划文件并处理安装冲突。
// 职责：按安装计划写入目标文件、记录回滚信息、追加安装会话并释放可覆盖的旧文件归属。
// 使用：artifact 安装流程在 plan 阶段完成后调用，传入冲突决策和进度区间。
// 边界：Windows 文件系统默认不区分大小写，历史记录可能混用斜杠；路径比较统一交给 v2/installpath 子包。
// 逻辑流：1. 校验取消状态 => 2. 检查文件冲突并应用决策 => 3. 备份已存在目标文件 => 4. 复制文件并追加会话 => 5. 释放被覆盖旧记录。
func CopyPlannedFiles(ctx context.Context, files []installplan.File, artifact v2Artifact.Artifact, options contract.InstallRemotePackageOptions, progressStart int, progressEnd int) ([]installplan.File, error) {
	session, err := newCopySession(ctx, files, artifact, options, progressStart, progressEnd)
	if err != nil {
		return nil, err
	}

	totalFiles := len(files)
	for index, file := range files {
		if err := session.copyPlannedFile(index, totalFiles, file); err != nil {
			session.rollback()
			return nil, err
		}
	}

	if err := session.releaseOverwrittenRecords(); err != nil {
		session.rollback()
		return nil, err
	}

	return session.installableFiles, nil
}
