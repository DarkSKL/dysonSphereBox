package copyfiles

// 文件职责：处理 V2 文件复制前的冲突检查和用户决策。
// 使用：copyPlannedFile 在准备目标文件前调用，决定当前文件应复制、跳过、失败或返回待前端处理的冲突。
// 边界：只处理冲突和覆盖记录；不备份文件、不复制文件、不释放旧 installed 记录。

import (
	"DysonSphereBox/helper/modDependence/installer/contract"
	"DysonSphereBox/helper/modDependence/installer/v2/copyconflict"
	v2InstallError "DysonSphereBox/helper/modDependence/installer/v2/installerror"
	"DysonSphereBox/helper/modDependence/installer/v2/installplan"
	"fmt"
	"os"
)

// prepareConflict 处理单文件冲突决策。
// 职责：检查目标文件冲突，并根据用户选项决定覆盖、跳过、失败或返回待处理冲突。
// 使用：copyPlannedFile 在复制目标文件前调用。
// 边界：只处理冲突和覆盖记录；不备份文件、不释放旧记录。
func (session *copySession) prepareConflict(file installplan.File) (bool, error) {
	conflict, err := copyconflict.InspectFile(session.db, file, session.artifact)
	if err != nil {
		return false, err
	}
	if conflict == nil {
		return true, nil
	}

	decision := copyconflict.ResolveDecision(session.options, file)
	switch decision {
	case contract.InstallConflictActionOverwrite:
		if info, err := os.Stat(file.TargetPath); err == nil && info.IsDir() {
			return false, fmt.Errorf("目标路径是目录，无法用文件覆盖：%s", file.TargetPath)
		} else if err != nil && !os.IsNotExist(err) {
			return false, err
		}
		session.overwrittenFiles = append(session.overwrittenFiles, file)
		return true, nil
	case contract.InstallConflictActionSkip:
		return false, nil
	case contract.InstallConflictActionFail:
		return false, fmt.Errorf("用户选择终止安装：%s", conflict.Message)
	default:
		return false, &v2InstallError.Conflict{Conflict: *conflict}
	}
}
