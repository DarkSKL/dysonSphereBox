package winprocess

// 文件职责：封装 Windows 进程查询、无头进程筛选以及游戏进程匹配的公共能力。
// 使用：headless 根包查找 DSPGAME 无头进程；gameport 探测戴森球计划进程与其端口。
// 边界：只读取系统进程输出，不启动、停止或修改任何进程；查询失败时按未运行处理。

import (
	"DysonSphereBox/utils/wincommand"
	"encoding/json"
	"strings"
	"time"
)

// Process 表示 Windows 命令查询到的目标进程信息。
// 职责：承载 PID、命令行、可执行路径与创建时间，供无头进程筛选和游戏端口探测共用。
// 使用：ListProcesses 解析 PowerShell 输出后返回给 headless 与 gameport。
// 边界：只在 Windows 命令输出解析中使用；ExecutablePath 可能因权限不足为空。
type Process struct {
	// CreatedAt 职责：记录进程创建时间；使用：计算真实运行时长；边界：读取不到时为零。
	CreatedAt int64 `json:"CreatedAt"`
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
	id, running, _ := InspectHeadlessProcess(gameDir, processName, serverArgument)
	return id, running
}

// InspectHeadlessProcess 职责：读取无头进程 PID 与运行秒数；使用：开服状态采样；边界：查询失败返回未运行，创建时间未知时秒数为零。
func InspectHeadlessProcess(gameDir string, processName string, serverArgument string) (int, bool, int64) {
	processes, err := ListProcesses(processName)
	if err != nil {
		return 0, false, 0
	}

	for _, process := range processes {
		if !isHeadlessProcessForGameDir(process, gameDir, processName, serverArgument) {
			continue
		}
		if process.ProcessID > 0 {
			seconds := int64(0)
			if process.CreatedAt > 0 {
				seconds = max(int64(0), time.Now().Unix()-process.CreatedAt)
			}
			return process.ProcessID, true, seconds
		}
	}

	return 0, false, 0
}

// ListProcesses 查询系统中的目标进程。
// 职责：通过 PowerShell CIM 获取 PID、命令行、可执行路径和创建时间。
// 使用：headless 筛选无头服务器进程；gameport 探测游戏进程及其安装目录。
// 边界：没有进程时返回空切片；输出可能是数组或单对象；命令失败时返回错误。
func ListProcesses(processName string) ([]Process, error) {
	command := `$items = Get-CimInstance Win32_Process -Filter "Name = '` + processName + `'" | Select-Object ProcessId,CommandLine,ExecutablePath,@{Name='CreatedAt';Expression={([DateTimeOffset]$_.CreationDate).ToUnixTimeSeconds()}}; if ($null -eq $items) { '[]' } else { $items | ConvertTo-Json -Compress }`
	output, err := wincommand.Output("powershell", "-NoProfile", "-Command", command)
	if err != nil {
		return nil, err
	}

	trimmedOutput := strings.TrimSpace(string(output))
	if trimmedOutput == "" || trimmedOutput == "null" {
		return []Process{}, nil
	}

	var processes []Process
	if strings.HasPrefix(trimmedOutput, "[") {
		if err := json.Unmarshal([]byte(trimmedOutput), &processes); err != nil {
			return nil, err
		}
		return processes, nil
	}

	var process Process
	if err := json.Unmarshal([]byte(trimmedOutput), &process); err != nil {
		return nil, err
	}

	return []Process{process}, nil
}
