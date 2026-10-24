package headless

// 文件职责：保存无头服务器开服页设置并协调 Nebula 配置同步。
// 使用：HeadlessService 在用户保存端口和存档设置时调用 SaveHeadlessSettings。
// 边界：不负责 Mod 市场查询、依赖安装、进程启动或前端页面状态管理。

import (
	"fmt"
	"path/filepath"
	"strings"

	"DysonSphereBox/core"
	"DysonSphereBox/helper/modDependence/gamepath"
	"DysonSphereBox/helper/modDependence/headless/contract"
	nebulaConfig "DysonSphereBox/helper/modDependence/nebula"
	"DysonSphereBox/helper/modDependence/winprocess"
)

// SaveHeadlessSettings 保存开服页设置并同步 Nebula 配置文件。
// 职责：保存启动存档配置并把主机端口写入 nebula.cfg。
// 使用：前端设置弹窗提交端口和存档名时由服务层调用。
// 边界：存档名暂不验证真实存在，只作为文本参与启动命令；端口被占用时返回错误。
// 逻辑流：1. 校验游戏目录和端口范围 => 2. 检查端口占用 => 3. 保存存档配置 => 4. 写入 Nebula HostPort => 5. 重新检测环境并返回设置结果。
func SaveHeadlessSettings(gameDir string, hostPort int, serverArchive string) (*contract.HeadlessSettings, error) {
	if err := gamepath.AssertDysonSphereGameDir(gameDir); err != nil {
		return nil, err
	}
	if hostPort <= 0 || hostPort > 65535 {
		return nil, fmt.Errorf("host port must be between 1 and 65535")
	}
	if processID, running := winprocess.FindDysonSphereHeadlessProcess(gameDir, dysonSphereProcessName, nebulaServerArgument); running && processID > 0 {
		return nil, fmt.Errorf("服务器运行中，请先中止服务器再保存配置")
	}
	currentPort := nebulaConfig.ReadHostPort(gameDir, nebulaConfigRelative, nebulaSettingsSection, defaultNebulaHostPort)
	if hostPort != currentPort && winprocess.IsPortOccupied(hostPort) {
		return nil, fmt.Errorf("开服端口 %d 已被占用，请更换端口后再保存", hostPort)
	}

	gameDir = filepath.Clean(gameDir)
	normalizedArchive, err := normalizeHeadlessArchive(serverArchive)
	if err != nil {
		return nil, err
	}
	if core.GLOBAL.Configure == nil || core.GLOBAL.Configure.GameServerArchive == nil {
		return nil, fmt.Errorf("GameServerArchive config is not initialized")
	}
	if err := nebulaConfig.WriteHostPort(gameDir, nebulaConfigRelative, nebulaSettingsSection, hostPort); err != nil {
		return nil, err
	}
	if err := core.GLOBAL.Configure.GameServerArchive.Set(normalizedArchive); err != nil {
		rollbackErr := nebulaConfig.WriteHostPort(gameDir, nebulaConfigRelative, nebulaSettingsSection, currentPort)
		if rollbackErr != nil {
			return nil, fmt.Errorf("保存存档配置失败，端口回滚也失败（%v）: %w", rollbackErr, err)
		}
		return nil, fmt.Errorf("保存存档配置失败，端口已恢复: %w", err)
	}

	status, err := InspectHeadlessEnvironment(gameDir)
	if err != nil {
		return nil, err
	}

	return &contract.HeadlessSettings{
		HostPort:         status.HostPort,
		ServerArchive:    status.ServerArchive,
		NebulaConfigPath: status.NebulaConfigPath,
		Status:           status,
	}, nil
}

// readHeadlessServerArchive 读取已保存的开服存档设置。
// 职责：从全局配置读取启动存档文本。
// 使用：构造环境状态和启动命令前调用。
// 边界：配置未初始化或读取失败时返回空字符串，表示使用默认最近存档。
func readHeadlessServerArchive() string {
	if core.GLOBAL.Configure == nil || core.GLOBAL.Configure.GameServerArchive == nil {
		return ""
	}

	serverArchive, err := core.GLOBAL.Configure.GameServerArchive.Get()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(serverArchive)
}
