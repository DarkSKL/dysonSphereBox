package modDependence

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	headlessSteamAppID      = "1366540"
	defaultNebulaHostPort   = 8469
	dysonSphereProcessName  = "DSPGAME.exe"
	nebulaServerArgument    = "-nebula-server"
	nebulaConfigRelative    = "BepInEx/config/nebula.cfg"
	headlessSteamAppIDFile  = "steam_appid.txt"
	headlessOutputLogFile   = "output_log.txt"
	headlessBepInExLogFile  = "BepInEx/LogOutput.log"
	headlessPluginRoot      = "BepInEx/plugins"
	headlessDefaultSaveMode = "-load-latest"
)

// HeadlessDependencyStatus 表示无头服务器依赖 Mod 在本地插件目录中的状态。
// 用途：开服页面展示 Nebula 依赖是否齐全；边界：版本来自本地 manifest.json，缺失时不猜测远端版本。
type HeadlessDependencyStatus struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Folder           string   `json:"folder"`
	InstalledVersion string   `json:"installedVersion"`
	Description      string   `json:"description"`
	Function         string   `json:"function"`
	Source           string   `json:"source"`
	PluginDir        string   `json:"pluginDir"`
	ManifestPath     string   `json:"manifestPath"`
	Installed        bool     `json:"installed"`
	MissingFiles     []string `json:"missingFiles"`
}

// HeadlessEnvironmentStatus 表示开服所需本地环境的聚合检测结果。
// 用途：前端展示一键开服准备状态、端口、进程和缺失项；边界：该状态只读，不会修改文件。
type HeadlessEnvironmentStatus struct {
	GameDir          string                     `json:"gameDir"`
	PluginRoot       string                     `json:"pluginRoot"`
	BepInExStatus    BepInExStatus              `json:"bepInExStatus"`
	Dependencies     []HeadlessDependencyStatus `json:"dependencies"`
	NebulaReady      bool                       `json:"nebulaReady"`
	SteamAppIDPath   string                     `json:"steamAppIdPath"`
	SteamAppIDExists bool                       `json:"steamAppIdExists"`
	SteamAppIDValid  bool                       `json:"steamAppIdValid"`
	HostPort         int                        `json:"hostPort"`
	NebulaConfigPath string                     `json:"nebulaConfigPath"`
	ProcessRunning   bool                       `json:"processRunning"`
	ProcessID        int                        `json:"processId"`
	TCPListening     bool                       `json:"tcpListening"`
	UDPListening     bool                       `json:"udpListening"`
	PortListening    bool                       `json:"portListening"`
	StartCommand     []string                   `json:"startCommand"`
	StartCommandText string                     `json:"startCommandText"`
	BepInExLogPath   string                     `json:"bepInExLogPath"`
	OutputLogPath    string                     `json:"outputLogPath"`
	Network          HeadlessNetworkStatus      `json:"network"`
	Issues           []string                   `json:"issues"`
	Ready            bool                       `json:"ready"`
}

// HeadlessPrepareResult 表示无头服务器环境准备动作的执行结果。
// 用途：前端点击准备环境后展示写入项和最新状态；边界：只准备基础环境，不自动安装 Nebula 整合包。
type HeadlessPrepareResult struct {
	GameDir     string                    `json:"gameDir"`
	Changed     bool                      `json:"changed"`
	Message     string                    `json:"message"`
	WrittenPath []string                  `json:"writtenPath"`
	Status      HeadlessEnvironmentStatus `json:"status"`
}

// HeadlessStartResult 表示无头服务器启动动作的执行结果。
// 用途：前端四宫格“启动”按钮展示启动反馈；边界：环境未就绪时不会启动游戏进程。
type HeadlessStartResult struct {
	// GameDir 是本次启动使用的戴森球计划安装目录。
	GameDir string `json:"gameDir"`
	// Started 表示本次调用是否成功发起了新的无头进程。
	Started bool `json:"started"`
	// Skipped 表示由于已运行或环境未就绪而跳过启动。
	Skipped bool `json:"skipped"`
	// ProcessID 是本次启动或检测到的 DSPGAME.exe 进程号。
	ProcessID int `json:"processId"`
	// Message 是面向前端展示的启动结果摘要。
	Message string `json:"message"`
	// StartCommandText 是实际执行的启动命令文本，用于日志和排错。
	StartCommandText string `json:"startCommandText"`
	// Status 是启动动作完成后重新检测得到的最新环境状态。
	Status HeadlessEnvironmentStatus `json:"status"`
}

// HeadlessStopResult 表示无头服务器停止动作的执行结果。
// 用途：前端四宫格“停止”按钮展示停止反馈；边界：只停止确认带有 -nebula-server 参数的 DSPGAME.exe 进程。
type HeadlessStopResult struct {
	// GameDir 是本次停止使用的戴森球计划安装目录。
	GameDir string `json:"gameDir"`
	// Stopped 表示本次调用是否成功终止了无头服务器进程。
	Stopped bool `json:"stopped"`
	// Skipped 表示没有找到可安全停止的无头服务器进程。
	Skipped bool `json:"skipped"`
	// ProcessID 是本次停止前识别到的无头服务器进程号。
	ProcessID int `json:"processId"`
	// Message 是面向前端展示的停止结果摘要。
	Message string `json:"message"`
	// Status 是停止动作完成后重新检测得到的最新环境状态。
	Status HeadlessEnvironmentStatus `json:"status"`
}

// HeadlessNetworkSample 表示一次真实网卡速率采样点。
// 用途：前端网络折线图展示最近若干次检测的上下行速率；边界：首次检测没有上一帧时速率为 0。
type HeadlessNetworkSample struct {
	// Label 是采样发生时的本地时间，格式为 HH:mm:ss。
	Label string `json:"label"`
	// CapturedAtUnix 是采样发生时的 Unix 秒级时间戳，用于计算真实采样窗口。
	CapturedAtUnix int64 `json:"capturedAtUnix"`
	// UploadMBps 是本次采样计算得到的上行速率，单位 MB/s。
	UploadMBps float64 `json:"uploadMBps"`
	// DownloadMBps 是本次采样计算得到的下行速率，单位 MB/s。
	DownloadMBps float64 `json:"downloadMBps"`
}

// HeadlessNetworkStatus 表示基于系统网卡计数器计算出的网络速率。
// 用途：开服页展示真实上下行速率、峰值和折线图；边界：读取失败时保持 Valid=false，不使用模拟数据。
type HeadlessNetworkStatus struct {
	// Valid 表示本次是否成功读取到 Windows 网卡累计字节计数。
	Valid bool `json:"valid"`
	// UploadBytesPerSecond 是最近两次采样之间计算出的上行字节速率。
	UploadBytesPerSecond float64 `json:"uploadBytesPerSecond"`
	// DownloadBytesPerSecond 是最近两次采样之间计算出的下行字节速率。
	DownloadBytesPerSecond float64 `json:"downloadBytesPerSecond"`
	// UploadMBps 是最近一次上行速率，单位 MB/s。
	UploadMBps float64 `json:"uploadMBps"`
	// DownloadMBps 是最近一次下行速率，单位 MB/s。
	DownloadMBps float64 `json:"downloadMBps"`
	// UploadPeakMBps 是当前保留采样窗口中的上行峰值，单位 MB/s。
	UploadPeakMBps float64 `json:"uploadPeakMBps"`
	// DownloadPeakMBps 是当前保留采样窗口中的下行峰值，单位 MB/s。
	DownloadPeakMBps float64 `json:"downloadPeakMBps"`
	// SampleWindowSeconds 是采样窗口覆盖的秒数。
	SampleWindowSeconds int `json:"sampleWindowSeconds"`
	// Samples 是最近的真实速率采样点，最多保留 12 个点。
	Samples []HeadlessNetworkSample `json:"samples"`
	// Error 是读取网卡统计失败时的诊断信息。
	Error string `json:"error"`
}

type headlessManifest struct {
	Name          string   `json:"name"`
	VersionNumber string   `json:"version_number"`
	Description   string   `json:"description"`
	WebsiteURL    string   `json:"website_url"`
	Dependencies  []string `json:"dependencies"`
}

type headlessNetworkCounter struct {
	receivedBytes uint64
	sentBytes     uint64
	capturedAt    time.Time
}

type headlessPowerShellNetworkCounter struct {
	ReceivedBytes uint64 `json:"ReceivedBytes"`
	SentBytes     uint64 `json:"SentBytes"`
}

type headlessWindowsProcess struct {
	ProcessID      int    `json:"ProcessId"`
	CommandLine    string `json:"CommandLine"`
	ExecutablePath string `json:"ExecutablePath"`
}

var (
	headlessNetworkMutex   sync.Mutex
	headlessNetworkLast    *headlessNetworkCounter
	headlessNetworkSamples []HeadlessNetworkSample
)

type headlessDependencySpec struct {
	ID            string
	Name          string
	Folder        string
	Function      string
	Description   string
	Source        string
	RequiredFiles []string
}

var headlessDependencySpecs = []headlessDependencySpec{
	{
		ID:          "nebula",
		Name:        "NebulaMultiplayerMod",
		Folder:      "nebula-NebulaMultiplayerMod",
		Function:    "联机 Mod 本体",
		Description: "让戴森球计划支持多人联机，并提供无头服务器入口参数。",
		Source:      "https://thunderstore.io/c/dyson-sphere-program/p/nebula/NebulaMultiplayerMod/",
		RequiredFiles: []string{
			"NebulaModel.dll",
			"NebulaNetwork.dll",
			"NebulaPatcher.dll",
			"NebulaWorld.dll",
		},
	},
	{
		ID:          "api",
		Name:        "NebulaMultiplayerModApi",
		Folder:      "nebula-NebulaMultiplayerModApi",
		Function:    "Nebula API",
		Description: "给其它 Mod 接入 Nebula 联机能力的接口层。",
		Source:      "https://thunderstore.io/c/dyson-sphere-program/p/nebula/NebulaMultiplayerModApi/",
	},
	{
		ID:          "compat",
		Name:        "NebulaCompatibilityAssist",
		Folder:      "starfi5h-NebulaCompatibilityAssist",
		Function:    "联机兼容补丁",
		Description: "修复 Nebula 与其它 Mod 的兼容性和同步问题。",
		Source:      "https://thunderstore.io/c/dyson-sphere-program/p/starfi5h/NebulaCompatibilityAssist/",
	},
	{
		ID:          "illine",
		Name:        "IlLine",
		Folder:      "PhantomGamers-IlLine",
		Function:    "堆栈行号辅助",
		Description: "改善错误堆栈中的 IL 行号，便于定位报错。",
		Source:      "https://thunderstore.io/c/dyson-sphere-program/p/PhantomGamers/IlLine/",
	},
	{
		ID:          "error-analyzer",
		Name:        "ErrorAnalyzer",
		Folder:      "starfi5h-ErrorAnalyzer",
		Function:    "错误分析工具",
		Description: "增强错误弹窗，辅助定位可能有问题的 Mod 或建筑。",
		Source:      "https://thunderstore.io/c/dyson-sphere-program/p/starfi5h/ErrorAnalyzer/",
	},
	{
		ID:          "bullet-time",
		Name:        "BulletTime",
		Folder:      "starfi5h-BulletTime",
		Function:    "速度控制与后台保存",
		Description: "提供暂停、加速、子弹时间和后台自动保存能力。",
		Source:      "https://thunderstore.io/c/dyson-sphere-program/p/starfi5h/BulletTime/",
	},
	{
		ID:          "mod-save",
		Name:        "DSPModSave",
		Folder:      "CommonAPI-DSPModSave",
		Function:    "Mod 独立存档库",
		Description: "让 Mod 把额外数据保存到独立文件，避免污染原版存档。",
		Source:      "https://thunderstore.io/c/dyson-sphere-program/p/CommonAPI/DSPModSave/",
	},
}

// InspectHeadlessEnvironment 汇总检查当前游戏目录的无头服务器准备状态。
// 使用：HeadlessService.GetEnvironmentStatus 调用；边界：只读取文件、进程和端口状态，不写入磁盘。
func InspectHeadlessEnvironment(gameDir string) (HeadlessEnvironmentStatus, error) {
	if err := assertDysonSphereGameDir(gameDir); err != nil {
		return HeadlessEnvironmentStatus{}, err
	}

	gameDir = filepath.Clean(gameDir)
	pluginRoot := filepath.Join(gameDir, headlessPluginRoot)
	hostPort := readNebulaHostPort(gameDir)
	processID, processRunning := findHeadlessDysonSphereProcess(gameDir)
	tcpListening := isNetstatPortListening("tcp", hostPort)
	udpListening := isNetstatPortListening("udp", hostPort)

	status := HeadlessEnvironmentStatus{
		GameDir:          gameDir,
		PluginRoot:       pluginRoot,
		BepInExStatus:    CheckBepInExStatus(gameDir),
		Dependencies:     inspectHeadlessDependencies(pluginRoot),
		SteamAppIDPath:   filepath.Join(gameDir, headlessSteamAppIDFile),
		HostPort:         hostPort,
		NebulaConfigPath: filepath.Join(gameDir, filepath.FromSlash(nebulaConfigRelative)),
		ProcessRunning:   processRunning,
		ProcessID:        processID,
		TCPListening:     tcpListening,
		UDPListening:     udpListening,
		PortListening:    tcpListening || udpListening,
		BepInExLogPath:   filepath.Join(gameDir, filepath.FromSlash(headlessBepInExLogFile)),
		OutputLogPath:    filepath.Join(gameDir, headlessOutputLogFile),
		Network:          inspectHeadlessNetworkStatus(),
		StartCommand: []string{
			filepath.Join(gameDir, dysonSphereProcessName),
			"-batchmode",
			"-nographics",
			nebulaServerArgument,
			headlessDefaultSaveMode,
		},
	}
	status.StartCommandText = quoteCommand(status.StartCommand)

	status.SteamAppIDExists, status.SteamAppIDValid = inspectSteamAppID(status.SteamAppIDPath)
	status.NebulaReady = dependenciesReady(status.Dependencies)
	status.Issues = collectHeadlessIssues(status)
	status.Ready = len(status.Issues) == 0

	return status, nil
}

// EnsureHeadlessBaseEnvironment 准备开服所需的基础运行文件。
// 使用：HeadlessService.PrepareEnvironment 调用；边界：只写 steam_appid.txt 和插件根目录，不自动安装 Nebula 依赖包。
func EnsureHeadlessBaseEnvironment(gameDir string) (*HeadlessPrepareResult, error) {
	if err := assertDysonSphereGameDir(gameDir); err != nil {
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

	return &HeadlessPrepareResult{
		GameDir:     gameDir,
		Changed:     len(writtenPaths) > 0,
		Message:     message,
		WrittenPath: writtenPaths,
		Status:      status,
	}, nil
}

// StartHeadlessEnvironment 启动 Nebula 无头服务器进程。
// 使用：开服页四宫格“启动”按钮调用；边界：只在检测 ready 后启动，不负责下载 Nebula Mod。
func StartHeadlessEnvironment(gameDir string) (*HeadlessStartResult, error) {
	status, err := InspectHeadlessEnvironment(gameDir)
	if err != nil {
		return nil, err
	}

	if status.ProcessRunning {
		return &HeadlessStartResult{
			GameDir:          status.GameDir,
			Started:          false,
			Skipped:          true,
			ProcessID:        status.ProcessID,
			Message:          "无头服务器已在运行",
			StartCommandText: status.StartCommandText,
			Status:           status,
		}, nil
	}

	if !status.Ready {
		return &HeadlessStartResult{
			GameDir:          status.GameDir,
			Started:          false,
			Skipped:          true,
			Message:          "开服环境未就绪，暂不启动",
			StartCommandText: status.StartCommandText,
			Status:           status,
		}, nil
	}

	if len(status.StartCommand) == 0 {
		return nil, fmt.Errorf("headless start command is empty")
	}

	command := exec.Command(status.StartCommand[0], status.StartCommand[1:]...)
	command.Dir = status.GameDir
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := command.Start(); err != nil {
		return nil, err
	}

	processID := command.Process.Pid
	if err := command.Process.Release(); err != nil {
		return nil, err
	}

	time.Sleep(1200 * time.Millisecond)
	nextStatus, err := InspectHeadlessEnvironment(status.GameDir)
	if err != nil {
		return nil, err
	}
	if nextStatus.ProcessID == 0 {
		nextStatus.ProcessID = processID
	}

	return &HeadlessStartResult{
		GameDir:          status.GameDir,
		Started:          true,
		Skipped:          false,
		ProcessID:        processID,
		Message:          "无头服务器启动命令已执行",
		StartCommandText: status.StartCommandText,
		Status:           nextStatus,
	}, nil
}

// StopHeadlessEnvironment 停止 Nebula 无头服务器进程。
// 使用：开服页四宫格“停止”按钮调用；边界：只终止当前游戏目录下带有 -nebula-server 参数的进程，避免误关普通游戏。
func StopHeadlessEnvironment(gameDir string) (*HeadlessStopResult, error) {
	status, err := InspectHeadlessEnvironment(gameDir)
	if err != nil {
		return nil, err
	}

	if !status.ProcessRunning || status.ProcessID == 0 {
		return &HeadlessStopResult{
			GameDir:   status.GameDir,
			Stopped:   false,
			Skipped:   true,
			ProcessID: 0,
			Message:   "未找到正在运行的无头服务器",
			Status:    status,
		}, nil
	}

	output, err := exec.Command("taskkill", "/PID", strconv.Itoa(status.ProcessID), "/T", "/F").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("stop headless process %d: %w: %s", status.ProcessID, err, strings.TrimSpace(string(output)))
	}

	time.Sleep(1000 * time.Millisecond)
	nextStatus, err := InspectHeadlessEnvironment(status.GameDir)
	if err != nil {
		return nil, err
	}

	stopped := !nextStatus.ProcessRunning
	message := "无头服务器已停止"
	if !stopped {
		message = "停止命令已执行，但进程仍在运行"
	}

	return &HeadlessStopResult{
		GameDir:   status.GameDir,
		Stopped:   stopped,
		Skipped:   false,
		ProcessID: status.ProcessID,
		Message:   message,
		Status:    nextStatus,
	}, nil
}

func inspectHeadlessDependencies(pluginRoot string) []HeadlessDependencyStatus {
	results := make([]HeadlessDependencyStatus, 0, len(headlessDependencySpecs))
	for _, spec := range headlessDependencySpecs {
		pluginDir := filepath.Join(pluginRoot, spec.Folder)
		result := HeadlessDependencyStatus{
			ID:           spec.ID,
			Name:         spec.Name,
			Folder:       spec.Folder,
			Function:     spec.Function,
			Description:  spec.Description,
			Source:       spec.Source,
			PluginDir:    pluginDir,
			ManifestPath: filepath.Join(pluginDir, "manifest.json"),
			MissingFiles: make([]string, 0),
		}

		if info, err := os.Stat(pluginDir); err != nil || !info.IsDir() {
			result.MissingFiles = append(result.MissingFiles, spec.Folder)
			results = append(results, result)
			continue
		}

		if manifest, err := readHeadlessManifest(result.ManifestPath); err == nil {
			if strings.TrimSpace(manifest.Name) != "" {
				result.Name = manifest.Name
			}
			result.InstalledVersion = strings.TrimSpace(manifest.VersionNumber)
			if strings.TrimSpace(manifest.Description) != "" {
				result.Description = manifest.Description
			}
			if strings.TrimSpace(manifest.WebsiteURL) != "" {
				result.Source = manifest.WebsiteURL
			}
		}

		for _, requiredFile := range spec.RequiredFiles {
			if _, err := os.Stat(filepath.Join(pluginDir, requiredFile)); err != nil {
				if os.IsNotExist(err) {
					result.MissingFiles = append(result.MissingFiles, requiredFile)
					continue
				}
				result.MissingFiles = append(result.MissingFiles, requiredFile)
			}
		}
		result.Installed = len(result.MissingFiles) == 0
		results = append(results, result)
	}

	return results
}

func readHeadlessManifest(path string) (headlessManifest, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return headlessManifest{}, err
	}

	var manifest headlessManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		return headlessManifest{}, err
	}

	return manifest, nil
}

func dependenciesReady(dependencies []HeadlessDependencyStatus) bool {
	for _, dependency := range dependencies {
		if !dependency.Installed {
			return false
		}
	}

	return true
}

func inspectSteamAppID(path string) (bool, bool) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, false
	}

	return true, strings.TrimSpace(string(content)) == headlessSteamAppID
}

func readNebulaHostPort(gameDir string) int {
	content, err := os.ReadFile(filepath.Join(gameDir, filepath.FromSlash(nebulaConfigRelative)))
	if err != nil {
		return defaultNebulaHostPort
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		parts := strings.SplitN(strings.TrimSpace(line), "=", 2)
		if len(parts) != 2 || !strings.EqualFold(strings.TrimSpace(parts[0]), "HostPort") {
			continue
		}

		port, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err == nil && port > 0 && port <= 65535 {
			return port
		}
	}

	return defaultNebulaHostPort
}

func collectHeadlessIssues(status HeadlessEnvironmentStatus) []string {
	issues := make([]string, 0)
	if !status.BepInExStatus.Installed {
		issues = append(issues, "BepInEx 前置框架未就绪")
	}
	if !status.NebulaReady {
		issues = append(issues, "Nebula 无头服务器依赖未安装完整")
	}
	if !status.SteamAppIDValid {
		issues = append(issues, "steam_appid.txt 缺失或内容不是 1366540")
	}
	if status.ProcessRunning && !status.PortListening {
		issues = append(issues, fmt.Sprintf("DSPGAME 进程运行中，但端口 %d 未监听", status.HostPort))
	}

	return issues
}

func findHeadlessDysonSphereProcess(gameDir string) (int, bool) {
	processes, err := listDysonSphereProcesses()
	if err != nil {
		return 0, false
	}

	for _, process := range processes {
		if !isHeadlessProcessForGameDir(process, gameDir) {
			continue
		}
		if process.ProcessID > 0 {
			return process.ProcessID, true
		}
	}

	return 0, false
}

func listDysonSphereProcesses() ([]headlessWindowsProcess, error) {
	command := `$items = Get-CimInstance Win32_Process -Filter "Name = 'DSPGAME.exe'" | Select-Object ProcessId,CommandLine,ExecutablePath; if ($null -eq $items) { '[]' } else { $items | ConvertTo-Json -Compress }`
	output, err := exec.Command("powershell", "-NoProfile", "-Command", command).Output()
	if err != nil {
		return nil, err
	}

	trimmedOutput := strings.TrimSpace(string(output))
	if trimmedOutput == "" || trimmedOutput == "null" {
		return []headlessWindowsProcess{}, nil
	}

	var processes []headlessWindowsProcess
	if strings.HasPrefix(trimmedOutput, "[") {
		if err := json.Unmarshal([]byte(trimmedOutput), &processes); err != nil {
			return nil, err
		}
		return processes, nil
	}

	var process headlessWindowsProcess
	if err := json.Unmarshal([]byte(trimmedOutput), &process); err != nil {
		return nil, err
	}

	return []headlessWindowsProcess{process}, nil
}

func isHeadlessProcessForGameDir(process headlessWindowsProcess, gameDir string) bool {
	commandLine := strings.ToLower(process.CommandLine)
	if !strings.Contains(commandLine, nebulaServerArgument) {
		return false
	}

	gameDir = filepath.Clean(gameDir)
	expectedExe := filepath.Clean(filepath.Join(gameDir, dysonSphereProcessName))
	if sameWindowsPath(process.ExecutablePath, expectedExe) {
		return true
	}

	normalizedCommandLine := normalizeWindowsPathForCompare(commandLine)
	return strings.Contains(normalizedCommandLine, normalizeWindowsPathForCompare(expectedExe)) ||
		strings.Contains(normalizedCommandLine, normalizeWindowsPathForCompare(gameDir))
}

func sameWindowsPath(left string, right string) bool {
	return normalizeWindowsPathForCompare(left) == normalizeWindowsPathForCompare(right)
}

func normalizeWindowsPathForCompare(path string) string {
	normalized := strings.Trim(strings.TrimSpace(path), `"`)
	normalized = filepath.Clean(normalized)
	normalized = strings.ReplaceAll(normalized, "/", `\`)
	return strings.ToLower(normalized)
}

func isNetstatPortListening(protocol string, port int) bool {
	output, err := exec.Command("netstat", "-ano", "-p", protocol).Output()
	if err != nil {
		return false
	}

	portToken := ":" + strconv.Itoa(port)
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		normalized := strings.ToUpper(strings.TrimSpace(line))
		if !strings.Contains(normalized, strings.ToUpper(protocol)) || !strings.Contains(normalized, portToken) {
			continue
		}
		if protocol == "tcp" {
			return strings.Contains(normalized, "LISTENING")
		}
		return true
	}

	return false
}

func inspectHeadlessNetworkStatus() HeadlessNetworkStatus {
	counter, err := readHeadlessNetworkCounter()

	headlessNetworkMutex.Lock()
	defer headlessNetworkMutex.Unlock()

	status := HeadlessNetworkStatus{
		Valid:   err == nil,
		Samples: append([]HeadlessNetworkSample(nil), headlessNetworkSamples...),
	}
	if err != nil {
		status.Error = err.Error()
		status.UploadPeakMBps, status.DownloadPeakMBps = networkPeaks(headlessNetworkSamples)
		status.SampleWindowSeconds = networkSampleWindowSeconds(headlessNetworkSamples)
		return status
	}

	uploadMBps := 0.0
	downloadMBps := 0.0
	if headlessNetworkLast != nil {
		elapsedSeconds := counter.capturedAt.Sub(headlessNetworkLast.capturedAt).Seconds()
		if elapsedSeconds > 0 {
			if counter.sentBytes >= headlessNetworkLast.sentBytes {
				status.UploadBytesPerSecond = float64(counter.sentBytes-headlessNetworkLast.sentBytes) / elapsedSeconds
			}
			if counter.receivedBytes >= headlessNetworkLast.receivedBytes {
				status.DownloadBytesPerSecond = float64(counter.receivedBytes-headlessNetworkLast.receivedBytes) / elapsedSeconds
			}
			uploadMBps = bytesPerSecondToMBps(status.UploadBytesPerSecond)
			downloadMBps = bytesPerSecondToMBps(status.DownloadBytesPerSecond)
		}
	}

	status.UploadMBps = uploadMBps
	status.DownloadMBps = downloadMBps
	sample := HeadlessNetworkSample{
		Label:          counter.capturedAt.Format("15:04:05"),
		CapturedAtUnix: counter.capturedAt.Unix(),
		UploadMBps:     uploadMBps,
		DownloadMBps:   downloadMBps,
	}
	headlessNetworkSamples = append(headlessNetworkSamples, sample)
	if len(headlessNetworkSamples) > 12 {
		headlessNetworkSamples = headlessNetworkSamples[len(headlessNetworkSamples)-12:]
	}
	headlessNetworkLast = &counter

	status.Samples = append([]HeadlessNetworkSample(nil), headlessNetworkSamples...)
	status.UploadPeakMBps, status.DownloadPeakMBps = networkPeaks(headlessNetworkSamples)
	status.SampleWindowSeconds = networkSampleWindowSeconds(headlessNetworkSamples)

	return status
}

func readHeadlessNetworkCounter() (headlessNetworkCounter, error) {
	if counter, err := readPowerShellNetworkCounter(); err == nil {
		return counter, nil
	}

	return readNetstatNetworkCounter()
}

func readPowerShellNetworkCounter() (headlessNetworkCounter, error) {
	command := `$items = Get-NetAdapterStatistics | Select-Object ReceivedBytes,SentBytes; $items | ConvertTo-Json -Compress`
	output, err := exec.Command("powershell", "-NoProfile", "-Command", command).Output()
	if err != nil {
		return headlessNetworkCounter{}, err
	}

	trimmedOutput := strings.TrimSpace(string(output))
	if trimmedOutput == "" {
		return headlessNetworkCounter{}, fmt.Errorf("network statistics output is empty")
	}

	var items []headlessPowerShellNetworkCounter
	if strings.HasPrefix(trimmedOutput, "[") {
		if err := json.Unmarshal([]byte(trimmedOutput), &items); err != nil {
			return headlessNetworkCounter{}, err
		}
	} else {
		var item headlessPowerShellNetworkCounter
		if err := json.Unmarshal([]byte(trimmedOutput), &item); err != nil {
			return headlessNetworkCounter{}, err
		}
		items = append(items, item)
	}

	counter := headlessNetworkCounter{capturedAt: time.Now()}
	for _, item := range items {
		counter.receivedBytes += item.ReceivedBytes
		counter.sentBytes += item.SentBytes
	}
	if counter.receivedBytes == 0 && counter.sentBytes == 0 {
		return headlessNetworkCounter{}, fmt.Errorf("network statistics counter is empty")
	}

	return counter, nil
}

func readNetstatNetworkCounter() (headlessNetworkCounter, error) {
	output, err := exec.Command("netstat", "-e").Output()
	if err != nil {
		return headlessNetworkCounter{}, err
	}

	numbers := make([]uint64, 0, 2)
	for _, field := range strings.Fields(string(output)) {
		normalized := strings.ReplaceAll(field, ",", "")
		value, err := strconv.ParseUint(normalized, 10, 64)
		if err == nil {
			numbers = append(numbers, value)
		}
	}
	if len(numbers) < 2 {
		return headlessNetworkCounter{}, fmt.Errorf("network statistics counter is unavailable")
	}

	return headlessNetworkCounter{
		receivedBytes: numbers[0],
		sentBytes:     numbers[1],
		capturedAt:    time.Now(),
	}, nil
}

func bytesPerSecondToMBps(value float64) float64 {
	if value <= 0 {
		return 0
	}

	return value / 1024 / 1024
}

func networkPeaks(samples []HeadlessNetworkSample) (float64, float64) {
	uploadPeak := 0.0
	downloadPeak := 0.0
	for _, sample := range samples {
		if sample.UploadMBps > uploadPeak {
			uploadPeak = sample.UploadMBps
		}
		if sample.DownloadMBps > downloadPeak {
			downloadPeak = sample.DownloadMBps
		}
	}

	return uploadPeak, downloadPeak
}

func networkSampleWindowSeconds(samples []HeadlessNetworkSample) int {
	if len(samples) <= 1 {
		return 0
	}

	first := samples[0].CapturedAtUnix
	last := samples[len(samples)-1].CapturedAtUnix
	if first <= 0 || last <= first {
		return 0
	}

	return int(last - first)
}

func quoteCommand(parts []string) string {
	quotedParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.ContainsAny(part, " \t") {
			quotedParts = append(quotedParts, strconv.Quote(part))
			continue
		}
		quotedParts = append(quotedParts, part)
	}

	return strings.Join(quotedParts, " ")
}
