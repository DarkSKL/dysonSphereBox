package system

import (
	"fmt"

	"DysonSphereBox/core"
	systemHelper "DysonSphereBox/helper/system"
	"DysonSphereBox/utils"
)

// EnvironmentService 提供前端可调用的游戏运行环境能力。
type EnvironmentService struct{}

// LoadOrInitDysonSphereExe 加载或初始化戴森球计划游戏目录。
func (model *EnvironmentService) LoadOrInitDysonSphereExe() (dirPath string, err error) {
	if core.GLOBAL.Configure == nil || core.GLOBAL.Configure.GameStartFilePath == nil {
		err := fmt.Errorf("configure is not initialized")
		systemHelper.RecordGamePathFailure("loadOrInitGamePath", "游戏目录加载或初始化失败", err, "")
		return "", err
	}

	configuredPath, err := core.GLOBAL.Configure.GameStartFilePath.Get()
	if err != nil {
		systemHelper.RecordGamePathFailure("loadOrInitGamePath", "游戏目录加载或初始化失败", err, "")
		return "", err
	}

	if dirPath := utils.NormalizeDysonSphereDir(configuredPath); dirPath != "" {
		systemHelper.RecordGamePathSuccess("loadOrInitGamePath", "已加载已保存的游戏目录", dirPath, "configured")
		return dirPath, nil
	}

	dirPath, err = utils.FindDysonSphereDirFromSteam()
	if err != nil || dirPath == "" {
		systemHelper.RecordGamePathFailure("loadOrInitGamePath", "游戏目录加载或初始化失败", err, dirPath)
		return dirPath, err
	}

	if err := core.GLOBAL.Configure.GameStartFilePath.Set(dirPath); err != nil {
		systemHelper.RecordGamePathFailure("loadOrInitGamePath", "游戏目录加载或初始化失败", err, dirPath)
		return "", err
	}

	systemHelper.RecordGamePathSuccess("loadOrInitGamePath", "已从 Steam 初始化游戏目录", dirPath, "steam")
	return dirPath, nil
}

// CheckDysonSphereExe 检查目录是否能归一化为戴森球计划安装目录。
func (model *EnvironmentService) CheckDysonSphereExe(dirPath string) bool {
	return utils.NormalizeDysonSphereDir(dirPath) != ""
}
