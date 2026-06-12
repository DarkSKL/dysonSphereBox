package headless

// 文件职责：准备无头服务器运行所需的基础本地文件和目录。
// 使用：HeadlessService.PrepareEnvironment 在确认 BepInEx 可用后调用 EnsureHeadlessBaseEnvironment。
// 边界：只写 steam_appid.txt 和插件根目录，不自动安装 Nebula 依赖包、不启动进程。

import (
	"os"
	"path/filepath"

	"DysonSphereBox/helper/modDependence/gamepath"
	"DysonSphereBox/helper/modDependence/headless/contract"
)

// EnsureHeadlessBaseEnvironment 准备开服所需的基础运行文件。
// 职责：创建插件根目录并确保 steam_appid.txt 内容正确。
// 使用：HeadlessService.PrepareEnvironment 在确认 BepInEx 可用后调用。
// 边界：只写 steam_appid.txt 和插件根目录，不自动安装 Nebula 依赖包。
// 逻辑流：1. 校验游戏目录 => 2. 创建 BepInEx/plugins => 3. 检查并修复 steam_appid.txt => 4. 重新检测环境 => 5. 返回写入路径和最新状态。
func EnsureHeadlessBaseEnvironment(gameDir string) (*contract.HeadlessPrepareResult, error) {
	if err := gamepath.AssertDysonSphereGameDir(gameDir); err != nil {
		return nil, err
	}
	gameDir = filepath.Clean(gameDir)
	writtenPaths := make([]string, 0)
	pluginRoot := filepath.Join(gameDir, headlessPluginRoot)
	if err := os.MkdirAll(pluginRoot, 0o755); err != nil {
		return nil, err
	}
	steamAppIDPath := filepath.Join(gameDir, headlessSteamAppIDFile)
	_, steamAppIDValid := inspectSteamAppID(steamAppIDPath)
	if !steamAppIDValid {
		if err := os.WriteFile(steamAppIDPath, []byte(headlessSteamAppID+"\r\n"), 0o644); err != nil {
			return nil, err
		}
		writtenPaths = append(writtenPaths, steamAppIDPath)
	}
	status, err := InspectHeadlessEnvironment(gameDir)
	if err != nil {
		return nil, err
	}
	message := "无头服务器基础环境已准备"
	if !status.Ready {
		message = "基础环境已准备，仍需补齐开服依赖"
	}
	return &contract.HeadlessPrepareResult{GameDir: gameDir, Changed: len(writtenPaths) > 0, Message: message, WrittenPath: writtenPaths, Status: status}, nil
}
