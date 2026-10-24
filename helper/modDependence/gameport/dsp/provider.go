package dsp

// 文件职责：探测戴森球计划进程及其 Nebula 主机端口，供创建房间表单自动回填。
// 使用：gameport.DetectAll 在创建房间弹窗打开时调用；只读进程、端口和 nebula.cfg。
// 边界：不启动/停止游戏、不写配置；无法确认主机端口时返回不可用 Preset 与简短原因。

import (
	"path/filepath"
	"strings"

	mpConstant "DysonSphereBox/constant/multiplayer"
	"DysonSphereBox/helper/modDependence/gamepath"
	"DysonSphereBox/helper/modDependence/gameport/contract"
	"DysonSphereBox/helper/modDependence/nebula"
	"DysonSphereBox/helper/modDependence/winprocess"
	"DysonSphereBox/utils/dysonsphere"
)

const (
	// providerID 职责：标识戴森球计划 provider；使用：前端选项值与已选状态匹配；边界：注册表内唯一。
	providerID = "dsp"
	// providerName 职责：面向用户显示的游戏名；使用：创建房间端口下拉项；边界：与游戏正式名称保持一致。
	providerName = "戴森球计划"
	// reasonNotRunning 职责：进程未启动时的禁用原因；使用：拼接在下拉项文本中；边界：不用于其它失败场景。
	reasonNotRunning = "未启动"
	// reasonNoPort 职责：进程运行但主机端口未监听时的禁用原因；使用：提示用户先在游戏内开启多人主机；边界：不表示检测命令失败。
	reasonNoPort = "未检测到端口"
	// reasonDetectFailed 职责：进程或端口查询失败时的禁用原因；使用：系统命令不可用时提示；边界：不启动或修复任何系统服务。
	reasonDetectFailed = "检测失败"
)

// provider 是戴森球计划端口识别实现。
// 职责：实现 contract.Provider，无状态地完成一次进程与端口探测。
// 使用：由 gameport 注册表持有并调用 Detect。
// 边界：不缓存结果，不修改游戏配置或进程。
type provider struct{}

// New 创建戴森球计划端口识别 provider。
// 职责：向 gameport 注册表提供无状态探测实现。
// 使用：gameport 注册表初始化时调用。
// 边界：不持有配置或缓存，每次 Detect 重新读取当前进程与端口。
func New() contract.Provider { return provider{} }

// Detect 探测当前戴森球计划进程与 Nebula 主机端口。
// 职责：优先匹配已配置游戏目录的进程，读取其 nebula.cfg HostPort，并确认进程确实绑定了该端口及协议。
// 使用：创建房间弹窗打开时由 gameport.DetectAll 调用。
// 边界：进程未运行、已运行但未监听主机端口或命令失败时返回不可用；不修改任何游戏文件。
// 逻辑流：1. 查询 DSPGAME 进程 => 2. 读取各进程游戏目录的 HostPort => 3. 按 PID 查询绑定并匹配 HostPort => 4. 选择游戏目录最匹配、创建时间最新的进程 => 5. 返回端口与协议。
func (provider) Detect() contract.Preset {
	processes, err := winprocess.ListProcesses(dysonsphere.ExeName)
	if err != nil {
		return unavailable(reasonDetectFailed)
	}
	if len(processes) == 0 {
		return unavailable(reasonNotRunning)
	}

	configuredDir, _ := gamepath.GetConfiguredGameDir()
	best := candidate{}
	readAny := false
	for _, process := range processes {
		bindings, err := winprocess.ListBindings(process.ProcessID)
		if err != nil {
			continue
		}
		readAny = true
		dir := processDirectory(process, configuredDir)
		hostPort := nebula.DefaultHostPort
		if dir != "" {
			hostPort = nebula.ReadHostPort(dir, nebula.ConfigRelative, nebula.SettingsSection, nebula.DefaultHostPort)
		}
		declarations := resolveDeclarations(bindings, hostPort)
		if len(declarations) == 0 {
			continue
		}
		score := 0
		if configuredDir != "" && sameDirectory(dir, configuredDir) {
			score = 2
		}
		current := candidate{declarations: declarations, score: score, createdAt: process.CreatedAt}
		if best.declarations == nil || current.score > best.score || (current.score == best.score && current.createdAt > best.createdAt) {
			best = current
		}
	}

	if best.declarations == nil {
		if readAny {
			return unavailable(reasonNoPort)
		}
		return unavailable(reasonDetectFailed)
	}

	return contract.Preset{ID: providerID, Name: providerName, Available: true, Ports: best.declarations}
}

// candidate 表示一次可用于比较的进程探测结果。
// 职责：暂存端口声明、目录匹配分数和创建时间；使用：Detect 在多个游戏进程间择优。
// 边界：只在单次 Detect 内使用，不暴露给服务层。
type candidate struct {
	// declarations 职责：该进程匹配到的端口声明；使用：最终 Preset.Ports；边界：不为空才参与比较。
	declarations []contract.Declaration
	// score 职责：游戏目录匹配优先级；使用：配置目录一致时优先；边界：0 或 2。
	score int
	// createdAt 职责：进程创建时间；使用：同分时选择更新进程；边界：读取不到时为零。
	createdAt int64
}

// processDirectory 返回进程对应的游戏目录。
// 职责：优先使用可执行文件所在目录，路径不可读时回退到已配置游戏目录。
// 使用：Detect 读取 nebula.cfg 前调用。
// 边界：两者都为空时返回空字符串，调用方不得拼接相对配置路径。
func processDirectory(process winprocess.Process, fallback string) string {
	if strings.TrimSpace(process.ExecutablePath) == "" {
		return fallback
	}

	return filepath.Dir(process.ExecutablePath)
}

// sameDirectory 判断两个目录是否指向同一游戏目录。
// 职责：清理路径后按 Windows 大小写不敏感规则比较。
// 使用：Detect 为进程目录匹配打分。
// 边界：任一为空时返回 false。
func sameDirectory(left, right string) bool {
	if left == "" || right == "" {
		return false
	}

	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

// resolveDeclarations 把进程绑定转换为创建房间端口声明。
// 职责：只接受 Nebula HostPort，并据 TCP/UDP 实际绑定合并为 both。
// 使用：Detect 对每个候选进程调用。
// 边界：HostPort 未被该进程绑定（未开多人主机或配置不一致）时返回空。
func resolveDeclarations(bindings []winprocess.Binding, hostPort int) []contract.Declaration {
	tcp := hasBinding(bindings, mpConstant.ProtoTCP, hostPort)
	udp := hasBinding(bindings, mpConstant.ProtoUDP, hostPort)
	if !tcp && !udp {
		return nil
	}

	return declarationsFor(hostPort, tcp, udp)
}

// hasBinding 判断绑定列表是否包含指定协议和端口。
// 职责：精确匹配协议与端口；使用：resolveDeclarations 推断协议；边界：忽略其它端口与协议。
func hasBinding(bindings []winprocess.Binding, proto string, port int) bool {
	for _, binding := range bindings {
		if binding.Proto == proto && binding.Port == port {
			return true
		}
	}

	return false
}

// declarationsFor 生成端口声明。
// 职责：TCP 与 UDP 同时监听时合并为 both，否则返回单一协议。
// 使用：resolveDeclarations 生成结果。
// 边界：tcp 与 udp 都为 false 时返回 nil。
func declarationsFor(port int, tcp, udp bool) []contract.Declaration {
	switch {
	case tcp && udp:
		return []contract.Declaration{{Proto: mpConstant.ProtoBoth, Port: port}}
	case tcp:
		return []contract.Declaration{{Proto: mpConstant.ProtoTCP, Port: port}}
	case udp:
		return []contract.Declaration{{Proto: mpConstant.ProtoUDP, Port: port}}
	default:
		return nil
	}
}

// unavailable 生成不可用 Preset。
// 职责：统一填充游戏标识、名称与禁用原因。
// 使用：Detect 的失败分支。
// 边界：Ports 保持空，前端只展示禁用项。
func unavailable(reason string) contract.Preset {
	return contract.Preset{ID: providerID, Name: providerName, Available: false, Reason: reason}
}
