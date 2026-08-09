package gamepath

// 文件职责：封装戴森球计划游戏目录的配置读取、归一化与可执行文件校验规则。
// 使用：BepInEx 安装器、无头服务器准备流程和 Mod 安装器在接触游戏目录前调用。
// 边界：只以 DSPGAME.exe 识别游戏目录，不负责写入配置、安装文件或修复目录内容。

import (
	"DysonSphereBox/core"
	"DysonSphereBox/utils"
	"DysonSphereBox/utils/dysonsphere"
	"fmt"
	"path/filepath"
)

// GetConfiguredGameDir 从全局配置读取并校验戴森球计划游戏目录。
// 职责：读取 GameStartFilePath 配置，并归一化为真实包含 DSPGAME.exe 的游戏目录。
// 使用：安装、卸载、开服准备等后端流程需要当前游戏目录时调用。
// 边界：配置缺失或 DSPGAME.exe 不存在时返回错误；不要求目录名或其它游戏子目录匹配。
func GetConfiguredGameDir() (string, error) {
	if core.GLOBAL.Configure == nil || core.GLOBAL.Configure.GameStartFilePath == nil {
		return "", fmt.Errorf("configure is not initialized")
	}
	configuredPath, err := core.GLOBAL.Configure.GameStartFilePath.Get()
	if err != nil {
		return "", err
	}
	gameDir := dysonsphere.NormalizeDir(configuredPath)
	if gameDir == "" {
		return "", fmt.Errorf("GameStartFilePath is not configured or invalid")
	}
	return gameDir, nil
}

// AssertDysonSphereGameDir 校验目录是否包含戴森球计划可执行文件。
// 职责：确认目标目录中存在普通文件 DSPGAME.exe。
// 使用：调用方在写入 BepInEx、无头服务器配置或 Mod 文件前调用。
// 边界：只以 DSPGAME.exe 作为路径身份目标，不校验目录名、Steam、版本、其它子目录或权限。
func AssertDysonSphereGameDir(gameDir string) error {
	gameDir = utils.CleanPath(gameDir)
	if gameDir == "" || !utils.FileExists(filepath.Join(gameDir, dysonsphere.ExeName)) {
		return fmt.Errorf("game directory does not contain %s", dysonsphere.ExeName)
	}

	return nil
}
