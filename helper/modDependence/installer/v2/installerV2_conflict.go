package v2

// 文件职责：处理用户对 V2 安装文件冲突的继续决策，并重新进入安装流程。
// 使用：由 InstallerService 在前端选择覆盖、跳过或失败后调用，恢复 install_state=conflict 的包。
// 边界：只校验冲突状态并更新 manifest 决策；实际复制、清理和状态持久化由安装主流程及 conflictstate 子包负责。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	installConstant "DysonSphereBox/constant/modDependence/install"
	"DysonSphereBox/helper/modDependence/installer/contract"
	"DysonSphereBox/helper/modDependence/installer/shared"
	"DysonSphereBox/helper/modDependence/installer/v2/conflictstate"
	"DysonSphereBox/helper/modDependence/installer/v2/copyconflict"
	"DysonSphereBox/helper/modDependence/installer/v2/operationguard"
	cleanupEntry "DysonSphereBox/helper/modDependence/installer/v2/sessioncleanup/entry"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionstate"
	modDependenceModel "DysonSphereBox/model/modDependence/installed"
	"encoding/json"
	"fmt"
	"strings"
)

// ContinueRemoteModPackageInstall 按用户选择继续待处理的冲突安装。
// 职责：读取待处理冲突 manifest，合并用户决策，并重新调用 V2 安装主流程。
// 使用：前端冲突弹窗提交覆盖、跳过或失败动作后由 InstallerService 调用。
// 边界：只恢复 install_state=conflict 的安装包；选择失败会把当前包标记为 failed。
func ContinueRemoteModPackageInstall(packageKey string, action contract.InstallConflictAction) (*contract.PackageInstallResult, error) {
	release, err := operationguard.Acquire()
	if err != nil {
		return nil, err
	}
	defer release()
	db, err := shared.RequireDatabase()
	if err != nil {
		return nil, err
	}
	record := modDependenceModel.ModInstalledPackage{}
	if err := db.Where("package_key = ?", strings.TrimSpace(packageKey)).Take(&record).Error; err != nil {
		return nil, err
	}
	if record.InstallState != string(installConstant.StateConflict) {
		return nil, fmt.Errorf("当前安装包没有待处理的安装冲突")
	}
	manifest := sessionstate.Manifest{}
	if len(record.InstallManifest) > 0 {
		if err := json.Unmarshal(record.InstallManifest, &manifest); err != nil {
			return nil, err
		}
	}
	if manifest.Conflict == nil {
		return nil, fmt.Errorf("没有可继续处理的安装冲突")
	}
	if action == contract.InstallConflictActionFail {
		err := fmt.Errorf("用户选择终止安装：%s", manifest.Conflict.Message)
		err = cleanupEntry.CleanupFailed(record.PackageKey, err)
		return nil, err
	}
	if action != contract.InstallConflictActionOverwrite && action != contract.InstallConflictActionSkip {
		return nil, fmt.Errorf("unsupported conflict action: %s", action)
	}
	manifest.Decisions = copyconflict.AppendOrReplaceDecision(manifest.Decisions, contract.InstallConflictDecisionDTO{TargetPath: manifest.Conflict.TargetPath, ComponentKey: manifest.Conflict.ComponentKey, Action: action})
	manifest.Conflict = nil
	pack := modDependenceResponse.RemoteModPackageDTO{}
	if err := json.Unmarshal(record.Raw, &pack); err != nil {
		return nil, err
	}
	if err := conflictstate.SaveManifest(record.PackageKey, string(installConstant.StateRunning), "prepare", "继续安装", 1, "", &manifest); err != nil {
		return nil, err
	}
	return installRemoteModPackageWithOptions(pack, contract.InstallRemotePackageOptions{Decisions: manifest.Decisions})
}
