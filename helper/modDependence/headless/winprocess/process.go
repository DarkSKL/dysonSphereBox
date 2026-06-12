package winprocess

// 文件职责：封装无头服务器依赖的 Windows 进程查询和无头进程筛选。
// 使用：由 headless 根包在环境状态刷新时调用，用于查找 DSPGAME 无头进程。
// 边界：只读取系统进程输出，不启动、停止或修改任何进程；查询失败时按未运行处理。

import (
	"DysonSphereBox/helper/modDependence/headless/wincommand"
	"encoding/json"
	"strings"
)

// windowsProcess 表示 Windows 命令查询到的游戏进程信息。
// 职责：承载 PID、命令行和可执行路径，用于识别无头服务器进程。
// 使用：listDysonSphereProcesses 解析 PowerShell 输出后交给 FindDysonSphereHeadlessProcess 筛选。
// 边界：只在 Windows 命令输出解析中使用，不暴露给服务层或前端 DTO。
type windowsProcess struct {
	// ProcessID 职责：记录进程 ID；使用：返回给上层用于展示或停止流程；边界：0 表示无效进程。
	ProcessID int `json:"ProcessId"`
	// CommandLine 职责：记录进程命令行；使用：判断是否包含无头服务器启动参数；边界：权限不足时可能为空。
	CommandLine string `json:"CommandLine"`
	// ExecutablePath 职责：记录进程可执行文件路径；使用：校验是否属于当前游戏目录；边界：路径可能为空或未规范化。
	ExecutablePath string `json:"ExecutablePath"`
}

// FindDysonSphereHeadlessProcess 查找当前游戏目录下的无头服务器进程。
// 职责：在目标游戏进程列表中筛选带无头参数且路径匹配的进程。
// 使用：headless.InspectHeadlessEnvironment 生成进程状态时传入游戏目录、进程名和服务器参数。
// 边界：查询失败时返回未运行，不向上抛出错误影响状态刷新。
// 逻辑流：1. 查询 Windows 进程列表 => 2. 过滤无头启动参数 => 3. 比较可执行路径或命令行路径 => 4. 返回首个有效 PID。
func FindDysonSphereHeadlessProcess(gameDir string, processName string, serverArgument string) (int, bool) {
	processes, err := listDysonSphereProcesses(processName)
	if err != nil {
		return 0, false
	}

	for _, process := range processes {
		if !isHeadlessProcessForGameDir(process, gameDir, processName, serverArgument) {
			continue
		}
		if process.ProcessID > 0 {
			return process.ProcessID, true
		}
	}

	return 0, false
}

// listDysonSphereProcesses 模块说明。
// 职责：封装 PowerShell CIM 查询和 JSON 输出兼容处理。
// 使用：FindDysonSphereHeadlessProcess 获取候选进程时调用。
// 边界：不判断无头参数或路径归属；没有进程时返回空切片。

// listDysonSphereProcesses 查询系统中的目标游戏进程。
// 职责：通过 PowerShell CIM 获取 PID、命令行和可执行路径。
// 使用：FindDysonSphereHeadlessProcess 过滤无头服务器进程前调用。
// 边界：没有进程时返回空切片；输出可能是数组或单对象。
func listDysonSphereProcesses(processName string) ([]windowsProcess, error) {
	command := `$items = Get-CimInstance Win32_Process -Filter "Name = '` + processName + `'" | Select-Object ProcessId,CommandLine,ExecutablePath; if ($null -eq $items) { '[]' } else { $items | ConvertTo-Json -Compress }`
	output, err := wincommand.Output("powershell", "-NoProfile", "-Command", command)
	if err != nil {
		return nil, err
	}

	trimmedOutput := strings.TrimSpace(string(output))
	if trimmedOutput == "" || trimmedOutput == "null" {
		return []windowsProcess{}, nil
	}

	var processes []windowsProcess
	if strings.HasPrefix(trimmedOutput, "[") {
		if err := json.Unmarshal([]byte(trimmedOutput), &processes); err != nil {
			return nil, err
		}
		return processes, nil
	}

	var process windowsProcess
	if err := json.Unmarshal([]byte(trimmedOutput), &process); err != nil {
		return nil, err
	}

	return []windowsProcess{process}, nil
}
