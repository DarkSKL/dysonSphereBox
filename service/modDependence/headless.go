package modDependence

// 文件职责：提供 Nebula 无头服务器环境检测、设置保存、准备、启动和停止的 Wails 服务。
// 使用：开服页通过生成绑定读取状态、保存端口和启动存档，并控制无头服务器进程。
// 边界：服务层只决定流程顺序和审计记录；文件写入、端口检测和进程控制由 helper 子包负责。

import (
	headlessAudit "DysonSphereBox/helper/modDependence/audit/headless"
	bepinexInstall "DysonSphereBox/helper/modDependence/bepinex/install"
	"DysonSphereBox/helper/modDependence/gamepath"
	"DysonSphereBox/helper/modDependence/headless"
	headlessContract "DysonSphereBox/helper/modDependence/headless/contract"
	"sync"
)

// HeadlessService 提供前端可调用的无头服务器环境准备能力。
// 职责：编排无头服务器状态检测、设置保存、环境准备、启动和停止。
// 使用：开服页检测并准备启动 Nebula 无头服务器所需的本地文件、依赖和端口状态。
// 边界：服务层只决定调用顺序和日志记录，具体文件写入、进程检测和端口检测由 helper 完成。
type HeadlessService struct {
	// operationMutex 职责：串行化设置与进程变更；使用：公开操作方法加锁；边界：不锁定 Mod 安装器。
	operationMutex sync.Mutex
}

// GetEnvironmentStatus 读取当前无头服务器环境状态。
// 职责：返回当前游戏目录、BepInEx、Nebula 依赖、端口和进程状态。
// 使用：开服页进入或定时刷新状态时调用。
// 边界：该方法只读，不会写入 steam_appid.txt 或安装任何依赖。
func (service *HeadlessService) GetEnvironmentStatus() (*headlessContract.HeadlessEnvironmentStatus, error) {
	service.operationMutex.Lock()
	defer service.operationMutex.Unlock()
	gameDir, err := gamepath.GetConfiguredGameDir()
	if err != nil {
		status := headless.BuildUnconfiguredHeadlessEnvironmentStatus(err.Error())
		return &status, nil
	}

	status, err := headless.InspectHeadlessEnvironment(gameDir)
	if err != nil {
		headlessAudit.RecordHeadlessFailure("status", "无头服务器环境检测失败", err)
		return nil, err
	}

	headlessAudit.RecordHeadlessStatusObservation(status, "status")
	return &status, nil
}

// SaveSettings 保存开服页设置。
// 职责：保存无头服务器端口和启动存档设置。
// 使用：前端设置弹窗提交端口和存档名；边界：端口同步写入 nebula.cfg，存档名只记录为文本并参与下一次启动。
// 边界：不验证存档实际存在；配置写入失败会记录系统日志并返回错误。
func (service *HeadlessService) SaveSettings(hostPort int, serverArchive string) (*headlessContract.HeadlessSettings, error) {
	service.operationMutex.Lock()
	defer service.operationMutex.Unlock()
	gameDir, err := gamepath.GetConfiguredGameDir()
	if err != nil {
		headlessAudit.RecordHeadlessFailure("settings", "无头服务器设置保存失败", err)
		return nil, err
	}

	result, err := headless.SaveHeadlessSettings(gameDir, hostPort, serverArchive)
	if err != nil {
		headlessAudit.RecordHeadlessFailure("settings", "无头服务器设置保存失败", err)
		return nil, err
	}

	headlessAudit.RecordHeadlessSettingsResult(result)
	headlessAudit.RecordHeadlessStatusObservation(result.Status, "settings")
	return result, nil
}

// PrepareEnvironment 准备无头服务器基础运行环境。
// 职责：确保 BepInEx 和无头服务器基础文件可用，并返回缺失依赖状态。
// 使用：前端点击“准备环境”时调用；会先确保 BepInEx 可用，再写入 steam_appid.txt 并返回缺失的 Nebula 依赖。
// 边界：该方法不会自动安装 Nebula 整合包，也不会启动游戏进程。
// 逻辑流：1. 安装或确认 BepInEx => 2. BepInEx 未就绪则返回阻塞状态 => 3. 写入基础环境文件 => 4. 记录准备结果和状态观察。
func (service *HeadlessService) PrepareEnvironment() (*headlessContract.HeadlessPrepareResult, error) {
	service.operationMutex.Lock()
	defer service.operationMutex.Unlock()
	return service.prepareEnvironment()
}

// prepareEnvironment 职责：在操作锁内准备基础环境；使用：Prepare/Start 共用；边界：调用方持锁，运行时禁止覆盖环境。
func (service *HeadlessService) prepareEnvironment() (*headlessContract.HeadlessPrepareResult, error) {
	status, err := service.inspectEnvironment()
	if err != nil {
		return nil, err
	}
	if status.ProcessRunning {
		return &headlessContract.HeadlessPrepareResult{GameDir: status.GameDir, Message: "服务器已在运行", Status: *status}, nil
	}
	bepInExResult, err := bepinexInstall.InstallBepInEx()
	if err != nil {
		headlessAudit.RecordHeadlessFailure("prepare", "无头服务器环境准备失败", err)
		return nil, err
	}
	if !bepInExResult.Status.Installed {
		result := &headlessContract.HeadlessPrepareResult{
			GameDir: bepInExResult.Status.GameDir,
			Message: "BepInEx 前置框架未就绪",
		}
		headlessAudit.RecordHeadlessPrepareResult(result)
		return result, nil
	}

	result, err := headless.EnsureHeadlessBaseEnvironment(bepInExResult.Status.GameDir)
	if err != nil {
		headlessAudit.RecordHeadlessFailure("prepare", "无头服务器环境准备失败", err)
		return nil, err
	}

	headlessAudit.RecordHeadlessPrepareResult(result)
	headlessAudit.RecordHeadlessStatusObservation(result.Status, "prepare")
	return result, nil
}

// StartEnvironment 一键启动无头服务器。
// 职责：准备环境并在就绪时启动 DSPGAME.exe 无头服务器进程。
// 使用：开服页四宫格“启动”按钮调用；会先准备基础环境，再在完整就绪时启动 DSPGAME.exe 无头进程。
// 边界：该方法不会自动下载 Nebula 依赖；依赖缺失时返回阻塞状态而不是强行启动。
// 逻辑流：1. 调用 PrepareEnvironment => 2. 未就绪则返回 skipped 结果 => 3. 就绪后启动进程 => 4. 记录启动结果和状态观察。
func (service *HeadlessService) StartEnvironment() (*headlessContract.HeadlessStartResult, error) {
	service.operationMutex.Lock()
	defer service.operationMutex.Unlock()
	prepareResult, err := service.prepareEnvironment()
	if err != nil {
		headlessAudit.RecordHeadlessFailure("start", "无头服务器启动准备失败", err)
		return nil, err
	}
	if !prepareResult.Status.Ready && !prepareResult.Status.ProcessRunning {
		result := &headlessContract.HeadlessStartResult{
			GameDir:          prepareResult.GameDir,
			Started:          false,
			Skipped:          true,
			Message:          "开服环境未就绪，暂不启动",
			StartCommandText: prepareResult.Status.StartCommandText,
			Status:           prepareResult.Status,
		}
		headlessAudit.RecordHeadlessStartResult(result)
		headlessAudit.RecordHeadlessStatusObservation(result.Status, "start")
		return result, nil
	}

	result, err := headless.StartHeadlessEnvironment(prepareResult.GameDir)
	if err != nil {
		headlessAudit.RecordHeadlessFailure("start", "无头服务器启动失败", err)
		return nil, err
	}

	headlessAudit.RecordHeadlessStartResult(result)
	headlessAudit.RecordHeadlessStatusObservation(result.Status, "start")
	return result, nil
}

// StopEnvironment 停止正在运行的无头服务器。
// 职责：停止当前游戏目录下的 Nebula 无头服务器进程。
// 使用：开服页四宫格“停止”按钮调用；只停止当前游戏目录下确认带有 -nebula-server 参数的无头服务器进程。
// 边界：未找到可安全停止的进程时返回跳过结果，不会误关普通游戏进程。
func (service *HeadlessService) StopEnvironment() (*headlessContract.HeadlessStopResult, error) {
	service.operationMutex.Lock()
	defer service.operationMutex.Unlock()
	gameDir, err := gamepath.GetConfiguredGameDir()
	if err != nil {
		headlessAudit.RecordHeadlessFailure("stop", "无头服务器停止失败", err)
		return nil, err
	}

	result, err := headless.StopHeadlessEnvironment(gameDir)
	if err != nil {
		headlessAudit.RecordHeadlessFailure("stop", "无头服务器停止失败", err)
		return nil, err
	}

	headlessAudit.RecordHeadlessStopResult(result)
	headlessAudit.RecordHeadlessStatusObservation(result.Status, "stop")
	return result, nil
}

// inspectEnvironment 职责：提供锁内只读状态；使用：准备流程先检查运行状态；边界：不再次获取操作锁。
func (service *HeadlessService) inspectEnvironment() (*headlessContract.HeadlessEnvironmentStatus, error) {
	gameDir, err := gamepath.GetConfiguredGameDir()
	if err != nil {
		status := headless.BuildUnconfiguredHeadlessEnvironmentStatus(err.Error())
		return &status, nil
	}
	status, err := headless.InspectHeadlessEnvironment(gameDir)
	return &status, err
}
