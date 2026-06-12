package artifactinstall

// 文件职责：规划并执行单个 artifact 的安装文件处理流程。
// 使用：V2 主安装流程逐个 artifact 调用，完成下载/解压/布局规划/复制。
// 边界：只处理 artifact 文件阶段，不保存组件模型或主包完成状态。

import (
	artifactConstant "DysonSphereBox/constant/modDependence/artifact"
	"DysonSphereBox/helper/modDependence/installer/contract"
	"DysonSphereBox/helper/modDependence/installer/progresscalc"
	v2Artifact "DysonSphereBox/helper/modDependence/installer/v2/artifact"
	"DysonSphereBox/helper/modDependence/installer/v2/artifactextract"
	"DysonSphereBox/helper/modDependence/installer/v2/artifactfetch"
	"DysonSphereBox/helper/modDependence/installer/v2/artifactplan"
	"DysonSphereBox/helper/modDependence/installer/v2/cancel"
	"DysonSphereBox/helper/modDependence/installer/v2/copyfiles"
	v2InstallError "DysonSphereBox/helper/modDependence/installer/v2/installerror"
	"DysonSphereBox/helper/modDependence/installer/v2/installmanifest"
	"DysonSphereBox/helper/modDependence/installer/v2/installplan"
	progressEvent "DysonSphereBox/helper/modDependence/installer/v2/progress/event"
	"context"
	"fmt"
)

// PlannedFilesHook 在计划文件写入会话后触发测试钩子。
// 职责：让根安装流程把测试阶段钩子注入 artifact 文件安装流程。
// 使用：集成测试需要卡住计划完成边界时传入；生产流程可传 nil。
// 边界：只接收 packageKey 与 componentKey，不读取或修改安装状态。
type PlannedFilesHook func(packageKey string, componentKey string)

// InstallFiles 处理单个 artifact 的文件安装。
// 职责：按 artifact 类型下载或读取本地路径，规划目标文件并复制到游戏目录。
// 使用：InstallRemoteModPackageWithOptions 遍历 artifact 队列时调用。
// 边界：返回 installplan.File 供持久化使用；复制冲突返回可恢复错误，保存数据库由上层完成。
// 逻辑流：1. 检查取消 => 2. 获取 artifact 路径 => 3. zip 解压或本地目录直接规划 => 4. 写入会话计划 => 5. 复制文件并返回结果。
func InstallFiles(ctx context.Context, bufferDir string, gameDir string, artifact v2Artifact.Artifact, progressStart int, progressEnd int, options contract.InstallRemotePackageOptions, afterPlannedFiles PlannedFilesHook) ([]installplan.File, string, error) {
	if err := cancel.Check(ctx); err != nil {
		return nil, "", err
	}
	progressEvent.Package(artifact.PackageKey, "download", fmt.Sprintf("正在获取 %s", artifact.DisplayName), progresscalc.Map(progressStart, progressEnd, 5))
	artifactPath, err := artifactfetch.ResolvePath(ctx, bufferDir, artifact, progresscalc.Map(progressStart, progressEnd, 5), progresscalc.Map(progressStart, progressEnd, 40))
	if err != nil {
		return nil, "", v2InstallError.WrapArtifact("download", artifact, err)
	}

	switch artifact.ArtifactKind {
	case artifactConstant.ArtifactKindRemoteZip, artifactConstant.ArtifactKindLocalZip, "":
		if err := cancel.Check(ctx); err != nil {
			return nil, "", err
		}
		progressEvent.Package(artifact.PackageKey, "extract", fmt.Sprintf("正在解压 %s", artifact.DisplayName), progresscalc.Map(progressStart, progressEnd, 45))
		extractPath, err := artifactextract.ExtractZip(ctx, bufferDir, artifactPath, artifact, progresscalc.Map(progressStart, progressEnd, 45), progresscalc.Map(progressStart, progressEnd, 55))
		if err != nil {
			return nil, "", v2InstallError.WrapArtifact("extract", artifact, err)
		}
		files, err := planAndCopyFiles(ctx, extractPath, gameDir, artifact, options, progresscalc.Map(progressStart, progressEnd, 58), progresscalc.Map(progressStart, progressEnd, 70), progressEnd, afterPlannedFiles)
		if err != nil {
			return nil, "", err
		}
		return files, artifactPath, nil
	case artifactConstant.ArtifactKindLocalDir:
		files, err := planAndCopyFiles(ctx, artifactPath, gameDir, artifact, options, progresscalc.Map(progressStart, progressEnd, 20), progresscalc.Map(progressStart, progressEnd, 55), progressEnd, afterPlannedFiles)
		if err != nil {
			return nil, "", err
		}
		return files, artifactPath, nil
	default:
		return nil, "", fmt.Errorf("unsupported artifact kind: %s", artifact.ArtifactKind)
	}
}

// planAndCopyFiles 规划并复制 artifact 文件的共享流程。
// 职责：把 zip 解压目录和本地目录两条路径统一到计划、manifest、复制阶段。
// 使用：InstallFiles 在不同 artifact 类型完成输入目录准备后调用。
// 边界：不解析 artifactPath，不持久化组件记录；错误会带上当前阶段便于上层清理。
// 逻辑流：1. 检查取消 => 2. 规划目标文件 => 3. 写入计划清单并触发钩子 => 4. 再次检查取消 => 5. 复制文件。
func planAndCopyFiles(ctx context.Context, sourceDir string, gameDir string, artifact v2Artifact.Artifact, options contract.InstallRemotePackageOptions, planPercent int, copyPercent int, progressEnd int, afterPlannedFiles PlannedFilesHook) ([]installplan.File, error) {
	if err := cancel.Check(ctx); err != nil {
		return nil, err
	}
	progressEvent.Package(artifact.PackageKey, "plan", fmt.Sprintf("正在分析 %s", artifact.DisplayName), planPercent)
	files, err := artifactplan.PlanFiles(ctx, sourceDir, gameDir, artifact)
	if err != nil {
		return nil, v2InstallError.WrapArtifact("plan", artifact, err)
	}
	if err := installmanifest.AppendPlannedFiles(artifact.InstallRecordPackageKey(), files); err != nil {
		return nil, v2InstallError.WrapArtifact("plan", artifact, err)
	}
	if afterPlannedFiles != nil {
		afterPlannedFiles(artifact.InstallRecordPackageKey(), artifact.ComponentKey)
	}
	if err := cancel.Check(ctx); err != nil {
		return nil, err
	}
	progressEvent.Package(artifact.PackageKey, "copy", fmt.Sprintf("正在复制 %s", artifact.DisplayName), copyPercent)
	files, err = copyfiles.CopyPlannedFiles(ctx, files, artifact, options, copyPercent, progressEnd)
	if err != nil {
		return nil, v2InstallError.WrapArtifact("copy", artifact, err)
	}
	return files, nil
}
