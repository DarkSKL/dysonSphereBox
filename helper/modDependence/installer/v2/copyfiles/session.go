package copyfiles

// 文件职责：维护 V2 文件复制过程中的临时会话状态。
// 使用：CopyPlannedFiles 创建会话后在循环中追加回滚计划、覆盖记录和已安装文件列表。
// 边界：只保存本次复制过程的内存状态，不直接复制文件；回滚动作只撤销当前会话已写入的文件。

import (
	"DysonSphereBox/helper/modDependence/installer/contract"
	"DysonSphereBox/helper/modDependence/installer/shared"
	v2Artifact "DysonSphereBox/helper/modDependence/installer/v2/artifact"
	"DysonSphereBox/helper/modDependence/installer/v2/copyconflict"
	"DysonSphereBox/helper/modDependence/installer/v2/copyrollback"
	"DysonSphereBox/helper/modDependence/installer/v2/installplan"
	cleanupFile "DysonSphereBox/helper/modDependence/installer/v2/sessioncleanup/file"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionsnapshot"
	"context"

	"gorm.io/gorm"
)

// copySession 表示一次安装计划复制的运行期状态。
// 职责：集中保存数据库句柄、artifact 信息、冲突选项、回滚清单和已安装文件。
// 使用：copyPlannedFile 逐个文件更新该状态，CopyPlannedFiles 在失败时调用 rollback。
// 边界：不跨安装流程复用，不持久化自身字段。
type copySession struct {
	// ctx 职责：保存取消信号上下文；使用：复制前后和文件复制工具中检查；边界：不创建新的生命周期。
	ctx context.Context
	// db 职责：保存已校验的数据库连接；使用：冲突检查、旧记录读取和释放旧记录；边界：不负责数据库初始化。
	db *gorm.DB
	// artifact 职责：保存当前复制的 artifact 元数据；使用：冲突检查和 manifest 追加时读取；边界：不修改 artifact。
	artifact v2Artifact.Artifact
	// options 职责：保存用户对文件冲突的处理选项；使用：冲突发生时决策覆盖、跳过或失败；边界：不在会话中修改选项。
	options contract.InstallRemotePackageOptions
	// packageKey 职责：保存当前安装记录主包 key；使用：回滚 manifest、读取旧记录和过滤旧记录；边界：必须来自 artifact。
	packageKey string
	// progressStart 职责：保存复制阶段进度起点；使用：发送复制进度事件时映射百分比；边界：不校验调用方传入范围。
	progressStart int
	// progressEnd 职责：保存复制阶段进度终点；使用：发送复制进度事件时映射百分比；边界：不校验调用方传入范围。
	progressEnd int
	// hasPreviousInstalledPackage 职责：标记 manifest 是否存在旧主包快照；使用：过滤被覆盖文件旧归属时判断保留范围；边界：只反映当前会话快照。
	hasPreviousInstalledPackage bool
	// rollbackPlan 职责：记录已经覆盖或写入的目标文件回滚信息；使用：失败时恢复或删除已写入文件；边界：只覆盖当前复制会话。
	rollbackPlan []installplan.CopyRollback
	// overwrittenFiles 职责：记录用户允许覆盖的文件；使用：复制完成后释放旧文件归属记录；边界：跳过和失败的文件不加入。
	overwrittenFiles []installplan.File
	// installableFiles 职责：记录已成功写入并追加 manifest 的文件；使用：返回给上层保存 installed 文件记录；边界：跳过的文件不返回。
	installableFiles []installplan.File
}

// newCopySession 创建文件复制会话。
// 职责：校验数据库连接、加载安装 manifest，并初始化回滚与结果切片。
// 使用：CopyPlannedFiles 在进入文件循环前调用。
// 边界：不检查文件冲突、不创建目录、不复制文件。
func newCopySession(ctx context.Context, files []installplan.File, artifact v2Artifact.Artifact, options contract.InstallRemotePackageOptions, progressStart int, progressEnd int) (*copySession, error) {
	db, err := shared.RequireDatabase()
	if err != nil {
		return nil, err
	}

	packageKey := artifact.InstallRecordPackageKey()
	manifest := sessionsnapshot.LoadManifest(packageKey)
	return &copySession{
		ctx:                         ctx,
		db:                          db,
		artifact:                    artifact,
		options:                     options,
		packageKey:                  packageKey,
		progressStart:               progressStart,
		progressEnd:                 progressEnd,
		hasPreviousInstalledPackage: manifest.PreviousPackage != nil,
		rollbackPlan:                make([]installplan.CopyRollback, 0, len(files)),
		overwrittenFiles:            make([]installplan.File, 0, len(files)),
		installableFiles:            make([]installplan.File, 0, len(files)),
	}, nil
}

// rollback 回滚当前复制会话已写入的文件和 manifest 记录。
// 职责：恢复被覆盖文件、删除新复制文件，并从 manifest 移除已追加条目。
// 使用：CopyPlannedFiles 在任意复制步骤失败或取消后调用。
// 边界：忽略 manifest 移除错误，避免覆盖原始失败原因；只处理当前会话已记录项目。
func (session *copySession) rollback() {
	copyrollback.RollbackCopiedFiles(session.rollbackPlan)
	_ = cleanupFile.RemoveFromManifest(session.packageKey, session.installableFiles)
}

// releaseOverwrittenRecords 释放被覆盖文件的旧归属记录。
// 职责：在全部文件复制成功后删除或调整允许覆盖文件的旧 installed file 记录。
// 使用：CopyPlannedFiles 循环完成后调用。
// 边界：必须在复制成功后执行；失败时由调用方触发回滚。
func (session *copySession) releaseOverwrittenRecords() error {
	for _, file := range session.overwrittenFiles {
		if err := copyconflict.ReleaseRecords(session.db, file); err != nil {
			return err
		}
	}
	return nil
}
