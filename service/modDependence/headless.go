package modDependence

import modDependenceHelper "DysonSphereBox/helper/modDependence"

// HeadlessService 提供前端可调用的无头服务器环境准备能力。
// 用途：开服页检测并准备启动 Nebula 无头服务器所需的本地文件、依赖和端口状态。
type HeadlessService struct {
}

// GetEnvironmentStatus 读取当前无头服务器环境状态。
// 边界：该方法只读，不会写入 steam_appid.txt 或安装任何依赖。
func (service *HeadlessService) GetEnvironmentStatus() (*modDependenceHelper.HeadlessEnvironmentStatus, error) {
	gameDir, err := modDependenceHelper.GetConfiguredGameDir()
	if err != nil {
		modDependenceHelper.RecordHeadlessFailure("status", "无头服务器环境检测失败", err)
		return nil, err
	}

	status, err := modDependenceHelper.InspectHeadlessEnvironment(gameDir)
	if err != nil {
		modDependenceHelper.RecordHeadlessFailure("status", "无头服务器环境检测失败", err)
		return nil, err
	}

	modDependenceHelper.RecordHeadlessStatusObservation(status, "status")
	return &status, nil
}

// SaveSettings 保存开服页设置。
// 使用：前端设置弹窗提交端口和存档名；边界：端口同步写入 nebula.cfg，存档名只记录为文本并参与下一次启动。
func (service *HeadlessService) SaveSettings(hostPort int, serverArchive string) (*modDependenceHelper.HeadlessSettings, error) {
	gameDir, err := modDependenceHelper.GetConfiguredGameDir()
	if err != nil {
		modDependenceHelper.RecordHeadlessFailure("settings", "无头服务器设置保存失败", err)
		return nil, err
	}

	result, err := modDependenceHelper.SaveHeadlessSettings(gameDir, hostPort, serverArchive)
	if err != nil {
		modDependenceHelper.RecordHeadlessFailure("settings", "无头服务器设置保存失败", err)
		return nil, err
	}

	modDependenceHelper.RecordHeadlessSettingsResult(result)
	modDependenceHelper.RecordHeadlessStatusObservation(result.Status, "settings")
	return result, nil
}

// PrepareEnvironment 准备无头服务器基础运行环境。
// 使用：前端点击“准备环境”时调用；会先确保 BepInEx 可用，再写入 steam_appid.txt 并返回缺失的 Nebula 依赖。
// 边界：该方法不会自动安装 Nebula 整合包，也不会启动游戏进程。
func (service *HeadlessService) PrepareEnvironment() (*modDependenceHelper.HeadlessPrepareResult, error) {
	bepInExResult, err := modDependenceHelper.InstallBepInEx()
	if err != nil {
		modDependenceHelper.RecordHeadlessFailure("prepare", "无头服务器环境准备失败", err)
		return nil, err
	}
	if !bepInExResult.Status.Installed {
		result := &modDependenceHelper.HeadlessPrepareResult{
			GameDir: bepInExResult.Status.GameDir,
			Message: "BepInEx 前置框架未就绪",
		}
		modDependenceHelper.RecordHeadlessPrepareResult(result)
		return result, nil
	}

	result, err := modDependenceHelper.EnsureHeadlessBaseEnvironment(bepInExResult.Status.GameDir)
	if err != nil {
		modDependenceHelper.RecordHeadlessFailure("prepare", "无头服务器环境准备失败", err)
		return nil, err
	}

	modDependenceHelper.RecordHeadlessPrepareResult(result)
	modDependenceHelper.RecordHeadlessStatusObservation(result.Status, "prepare")
	return result, nil
}

// StartEnvironment 一键启动无头服务器。
// 使用：开服页四宫格“启动”按钮调用；会先准备基础环境，再在完整就绪时启动 DSPGAME.exe 无头进程。
// 边界：该方法不会自动下载 Nebula 依赖；依赖缺失时返回阻塞状态而不是强行启动。
func (service *HeadlessService) StartEnvironment() (*modDependenceHelper.HeadlessStartResult, error) {
	prepareResult, err := service.PrepareEnvironment()
	if err != nil {
		modDependenceHelper.RecordHeadlessFailure("start", "无头服务器启动准备失败", err)
		return nil, err
	}
	if !prepareResult.Status.Ready {
		result := &modDependenceHelper.HeadlessStartResult{
			GameDir:          prepareResult.GameDir,
			Started:          false,
			Skipped:          true,
			Message:          "开服环境未就绪，暂不启动",
			StartCommandText: prepareResult.Status.StartCommandText,
			Status:           prepareResult.Status,
		}
		modDependenceHelper.RecordHeadlessStartResult(result)
		modDependenceHelper.RecordHeadlessStatusObservation(result.Status, "start")
		return result, nil
	}

	result, err := modDependenceHelper.StartHeadlessEnvironment(prepareResult.GameDir)
	if err != nil {
		modDependenceHelper.RecordHeadlessFailure("start", "无头服务器启动失败", err)
		return nil, err
	}

	modDependenceHelper.RecordHeadlessStartResult(result)
	modDependenceHelper.RecordHeadlessStatusObservation(result.Status, "start")
	return result, nil
}

// StopEnvironment 停止正在运行的无头服务器。
// 使用：开服页四宫格“停止”按钮调用；只停止当前游戏目录下确认带有 -nebula-server 参数的无头服务器进程。
// 边界：未找到可安全停止的进程时返回跳过结果，不会误关普通游戏进程。
func (service *HeadlessService) StopEnvironment() (*modDependenceHelper.HeadlessStopResult, error) {
	gameDir, err := modDependenceHelper.GetConfiguredGameDir()
	if err != nil {
		modDependenceHelper.RecordHeadlessFailure("stop", "无头服务器停止失败", err)
		return nil, err
	}

	result, err := modDependenceHelper.StopHeadlessEnvironment(gameDir)
	if err != nil {
		modDependenceHelper.RecordHeadlessFailure("stop", "无头服务器停止失败", err)
		return nil, err
	}

	modDependenceHelper.RecordHeadlessStopResult(result)
	modDependenceHelper.RecordHeadlessStatusObservation(result.Status, "stop")
	return result, nil
}
