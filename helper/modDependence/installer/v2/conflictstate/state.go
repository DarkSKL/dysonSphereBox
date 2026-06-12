package conflictstate

// 文件职责：维护 V2 安装冲突与安装会话 manifest 的持久化状态。
// 使用：由安装主流程、继续冲突流程、会话追加流程和失败清理流程写入 running/conflict/failed 状态。
// 边界：只更新 installed package 的安装状态和 InstallManifest，不复制文件、不清理缓存、不决定业务流程下一步。
// 逻辑流：1. 从冲突错误读取 DTO => 2. 合并当前 manifest 与用户决策 => 3. 写入数据库状态 => 4. 发送运行时进度或失败快照。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	installConstant "DysonSphereBox/constant/modDependence/install"
	"DysonSphereBox/helper/modDependence/installer/contract"
	"DysonSphereBox/helper/modDependence/installer/progresscalc"
	"DysonSphereBox/helper/modDependence/installer/shared"
	v2InstallError "DysonSphereBox/helper/modDependence/installer/v2/installerror"
	progressEvent "DysonSphereBox/helper/modDependence/installer/v2/progress/event"
	progressPayload "DysonSphereBox/helper/modDependence/installer/v2/progress/payload"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionsnapshot"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionstate"
	modDependenceModel "DysonSphereBox/model/modDependence/installed"
	"encoding/json"
	"gorm.io/datatypes"
	"time"
)

// PersistConflict 保存可恢复的安装冲突状态。
// 职责：把冲突错误中的 DTO 写入 manifest，并把主包状态标记为 conflict。
// 使用：安装主流程遇到 v2 installerror.Conflict 时调用，供前端展示冲突并继续处理。
// 边界：非冲突错误直接返回 nil；不会清理文件或回滚复制结果。
func PersistConflict(pack modDependenceResponse.RemoteModPackageDTO, err error, options contract.InstallRemotePackageOptions) error {
	conflictErr := v2InstallError.AsConflict(err)
	if conflictErr == nil {
		return nil
	}
	manifest := sessionsnapshot.LoadManifest(pack.Key)
	manifest.Conflict = &conflictErr.Conflict
	if len(options.Decisions) > 0 {
		manifest.Decisions = options.Decisions
	}
	errorMessage := conflictErr.Error()
	if saveErr := SaveManifest(pack.Key, string(installConstant.StateConflict), "conflict", "等待处理文件冲突", 100, errorMessage, &manifest); saveErr != nil {
		return saveErr
	}
	progressEvent.Emit(progressPayload.Payload{
		PackageKey:   pack.Key,
		Stage:        "conflict",
		Status:       "等待处理文件冲突",
		Percent:      100,
		Message:      errorMessage,
		State:        string(installConstant.StateConflict),
		ErrorMessage: errorMessage,
		Conflict:     &conflictErr.Conflict,
	})
	return nil
}

// SaveManifest 保存安装状态和会话 manifest。
// 职责：统一写入 install_state、stage、status、progress、error 和 manifest JSON。
// 使用：安装会话初始化、计划追加、冲突继续和失败清理状态落库时调用。
// 边界：不会创建安装包基础记录，调用前必须确保对应 package_key 已存在。
func SaveManifest(packageKey string, state string, stage string, status string, percent int, errorMessage string, manifest *sessionstate.Manifest) error {
	db, err := shared.RequireDatabase()
	if err != nil {
		return err
	}
	content, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	return db.Model(&modDependenceModel.ModInstalledPackage{}).
		Where("package_key = ?", packageKey).
		Updates(map[string]interface{}{
			"install_state":    state,
			"install_stage":    stage,
			"install_status":   status,
			"install_progress": progresscalc.Clamp(percent),
			"error_message":    errorMessage,
			"install_manifest": datatypes.JSON(content),
			"updated_at":       time.Now(),
		}).Error
}

// MarkFailed 将冲突安装标记为失败并发送失败快照。
// 职责：在用户选择终止冲突安装后清空 manifest，并同步运行时进度为 failed。
// 使用：ContinueRemoteModPackageInstall 处理 fail 动作时调用。
// 边界：保存失败会被忽略以保留原错误返回；不执行清理流程。
func MarkFailed(packageKey string, err error) {
	errorMessage := ""
	if err != nil {
		errorMessage = err.Error()
	}
	_ = SaveManifest(packageKey, string(installConstant.StateFailed), "failed", "安装失败", 100, errorMessage, &sessionstate.Manifest{})
	progressEvent.MarkFailure(packageKey, "安装失败", err)
}
