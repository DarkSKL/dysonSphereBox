package system

// 文件职责：提供戴森球计划游戏目录加载、自动定位和有效性检测的 Wails 服务。
// 使用：设置页或启动流程需要读取、初始化或校验游戏目录时调用。
// 边界：只处理游戏目录配置，不安装 Mod、不启动游戏、不管理远端市场。

import (
	"fmt"

	"DysonSphereBox/core"
	systemHelper "DysonSphereBox/helper/system"
	"DysonSphereBox/utils/dysonsphere"
)

// EnvironmentService 提供前端可调用的游戏运行环境能力。
// 职责：定位、加载和校验戴森球计划游戏安装目录。
// 使用：设置页自动识别游戏目录或用户手动选择目录后调用。
// 边界：不安装 Mod、不启动游戏、不直接修改除 GameStartFilePath 之外的配置。
type EnvironmentService struct{}

// LoadOrInitDysonSphereExe 加载或初始化戴森球计划游戏目录。
// 职责：优先读取已保存路径，缺失时从 Steam 自动定位并保存。
// 使用：设置页点击自动获取或应用启动后需要初始化游戏目录时调用。
// 边界：无法定位时返回空路径和错误；不会猜测非 DSPGAME.exe 所在目录。
// 逻辑流：1. 校验 Configure 可用 => 2. 读取已保存路径并归一化 => 3. 已保存路径有效则返回 => 4. 从 Steam 查找 => 5. 保存并返回新路径。
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

	if dirPath := dysonsphere.NormalizeDir(configuredPath); dirPath != "" {
		return dirPath, nil
	}

	dirPath, err = dysonsphere.FindDirFromSteam()
	if err != nil || dirPath == "" {
		systemHelper.RecordGamePathFailure("loadOrInitGamePath", "游戏目录加载或初始化失败", err, dirPath)
		return dirPath, err
	}

	if err := core.GLOBAL.Configure.GameStartFilePath.Set(dirPath); err != nil {
		systemHelper.RecordGamePathFailure("loadOrInitGamePath", "游戏目录加载或初始化失败", err, dirPath)
		return "", err
	}

	return dirPath, nil
}

// CheckDysonSphereExe 检查目录是否能归一化为戴森球计划安装目录。
// 职责：判断用户输入或选择的路径是否指向有效游戏目录。
// 使用：设置页保存前或选择目录后即时校验时调用。
// 边界：只返回布尔结果，不写入配置，不记录日志。
func (model *EnvironmentService) CheckDysonSphereExe(dirPath string) bool {
	return dysonsphere.NormalizeDir(dirPath) != ""
}
