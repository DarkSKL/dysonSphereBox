package headless

// 文件职责：汇总检查当前游戏目录的无头服务器准备状态。
// 使用：HeadlessService 在状态刷新、保存设置、准备环境、启动和停止后调用 InspectHeadlessEnvironment。
// 边界：只读取文件、进程和端口状态，不写入磁盘；路径无效时返回错误。

import (
	bepinexStatus "DysonSphereBox/helper/modDependence/bepinex/status"
	"DysonSphereBox/helper/modDependence/gamepath"
	headlessCommand "DysonSphereBox/helper/modDependence/headless/command"
	"DysonSphereBox/helper/modDependence/headless/contract"
	headlessDependency "DysonSphereBox/helper/modDependence/headless/dependency"
	headlessNebula "DysonSphereBox/helper/modDependence/headless/nebula"
	headlessNetwork "DysonSphereBox/helper/modDependence/headless/network"
	"DysonSphereBox/helper/modDependence/headless/wincommand"
	"DysonSphereBox/helper/modDependence/headless/winprocess"
	"path/filepath"
)

// BuildUnconfiguredHeadlessEnvironmentStatus 构造未配置游戏路径时的只读开服状态。
// 职责：在游戏目录不可用时返回可展示的缺省状态。
// 使用：HeadlessService.GetEnvironmentStatus 捕获配置缺失或路径错误后调用。
// 边界：不把未配置路径当成接口异常；不访问游戏目录，只返回引导用户设置路径的问题列表。
func BuildUnconfiguredHeadlessEnvironmentStatus(reason string) contract.HeadlessEnvironmentStatus {
	status := contract.HeadlessEnvironmentStatus{Dependencies: headlessDependency.Inspect(""), HostPort: defaultNebulaHostPort, ServerArchive: readHeadlessServerArchive(), Network: headlessNetwork.InspectStatus(), Issues: unconfiguredHeadlessIssues(reason), Ready: false}
	status.NebulaReady = headlessDependency.Ready(status.Dependencies)
	return status
}

// InspectHeadlessEnvironment 汇总检查当前游戏目录的无头服务器准备状态。
// 职责：聚合游戏目录中 BepInEx、Nebula 依赖、steam_appid、进程、端口、网络和启动命令状态。
// 使用：状态刷新、保存设置、准备环境、启动和停止后都调用该函数返回最新状态。
// 边界：只读取文件、进程和端口状态，不写入磁盘；路径无效时返回错误。
// 逻辑流：1. 校验游戏目录 => 2. 读取端口和启动存档 => 3. 查询进程和端口监听 => 4. 检测 BepInEx 与依赖 => 5. 生成启动命令、网络状态和问题列表。
func InspectHeadlessEnvironment(gameDir string) (contract.HeadlessEnvironmentStatus, error) {
	if err := gamepath.AssertDysonSphereGameDir(gameDir); err != nil {
		return contract.HeadlessEnvironmentStatus{}, err
	}
	gameDir = filepath.Clean(gameDir)
	pluginRoot := filepath.Join(gameDir, headlessPluginRoot)
	hostPort := headlessNebula.ReadHostPort(gameDir, nebulaConfigRelative, nebulaSettingsSection, defaultNebulaHostPort)
	serverArchive := readHeadlessServerArchive()
	processID, processRunning := winprocess.FindDysonSphereHeadlessProcess(gameDir, dysonSphereProcessName, nebulaServerArgument)
	tcpListening := winprocess.IsNetstatPortListening("tcp", hostPort)
	udpListening := winprocess.IsNetstatPortListening("udp", hostPort)
	status := contract.HeadlessEnvironmentStatus{GameDir: gameDir, PluginRoot: pluginRoot, BepInExStatus: bepinexStatus.CheckBepInExStatus(gameDir), Dependencies: headlessDependency.Inspect(pluginRoot), SteamAppIDPath: filepath.Join(gameDir, headlessSteamAppIDFile), HostPort: hostPort, NebulaConfigPath: filepath.Join(gameDir, filepath.FromSlash(nebulaConfigRelative)), ServerArchive: serverArchive, ProcessRunning: processRunning, ProcessID: processID, TCPListening: tcpListening, UDPListening: udpListening, PortListening: tcpListening || udpListening, BepInExLogPath: filepath.Join(gameDir, filepath.FromSlash(headlessBepInExLogFile)), OutputLogPath: filepath.Join(gameDir, headlessOutputLogFile), Network: headlessNetwork.InspectStatus(), StartCommand: headlessCommand.BuildStartCommand(gameDir, dysonSphereProcessName, nebulaServerArgument, headlessDefaultSaveMode, serverArchive)}
	status.StartCommandText = wincommand.Quote(status.StartCommand)
	status.SteamAppIDExists, status.SteamAppIDValid = inspectSteamAppID(status.SteamAppIDPath)
	status.NebulaReady = headlessDependency.Ready(status.Dependencies)
	status.Issues = collectHeadlessIssues(status)
	status.Ready = len(status.Issues) == 0
	return status, nil
}
