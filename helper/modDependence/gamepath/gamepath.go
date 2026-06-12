package gamepath

// 文件职责：封装戴森球计划游戏目录的配置读取、归一化与结构校验规则。
// 使用：BepInEx 安装器、无头服务器准备流程和 Mod 安装器在接触游戏目录前调用。
// 边界：不负责写入配置、安装文件或修复目录内容；路径无效时返回错误而不是猜测目录。

import (
	"DysonSphereBox/core"
	"DysonSphereBox/utils/dysonsphere"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GetConfiguredGameDir 从全局配置读取并校验戴森球计划游戏目录。
// 职责：读取 GameStartFilePath 配置、归一化为游戏目录并校验关键结构。
// 使用：安装、卸载、开服准备等后端流程需要当前游戏目录时调用。
// 边界：配置缺失、路径无法归一化或目录结构不完整时返回错误，不创建任何文件。
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
	if err := AssertDysonSphereGameDir(gameDir); err != nil {
		return "", err
	}
	return gameDir, nil
}

// AssertDysonSphereGameDir 校验目录是否具备戴森球计划本体的关键结构。
// 职责：确认目录中存在 DSPGAME.exe、DSPGAME_Data 和 MonoBleedingEdge。
// 使用：调用方在写入 BepInEx、无头服务器配置或 Mod 文件前调用。
// 边界：只检查 DSPGAME.exe、DSPGAME_Data 和 MonoBleedingEdge，不校验 Steam、版本或权限。
func AssertDysonSphereGameDir(gameDir string) error {
	requiredPaths := []string{
		dysonsphere.ExeName,
		"DSPGAME_Data",
		"MonoBleedingEdge",
	}

	missingPaths := make([]string, 0)
	for _, requiredPath := range requiredPaths {
		if _, err := os.Stat(filepath.Join(gameDir, requiredPath)); err != nil {
			if os.IsNotExist(err) {
				missingPaths = append(missingPaths, requiredPath)
				continue
			}
			return err
		}
	}

	if len(missingPaths) > 0 {
		return fmt.Errorf("game directory is incomplete, missing: %s", strings.Join(missingPaths, ", "))
	}

	return nil
}
