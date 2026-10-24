package contract

// 文件职责：定义游戏端口自动识别的提供者契约与跨包投影结构。
// 使用：gameport 注册表遍历 Provider；dsp 等具体游戏实现返回 Preset 供服务层投影给前端。
// 边界：只描述数据形状，不读取进程或端口、不包含具体游戏规则、不依赖 core/model/service。

// Declaration 表示一个可回填到创建房间表单的端口声明。
// 职责：描述同一端口支持的协议；使用：前端按它生成端口行；边界：Proto 只能是 tcp、udp 或 both。
type Declaration struct {
	// Proto 职责：端口协议；使用：创建房间表单的协议选择；边界：tcp、udp 或 both，小写。
	Proto string
	// Port 职责：游戏实际绑定的端口；使用：创建房间表单的端口输入；边界：1–65535。
	Port int
}

// Preset 表示一个游戏的自动识别结果。
// 职责：向服务层同时提供展示信息和可回填端口，可选项即使不可用也会返回。
// 使用：MultiplayerService.GamePortPresets 投影为前端选项。
// 边界：Available 为 false 时 Ports 必须为空，Reason 给出简短不可用原因。
type Preset struct {
	// ID 职责：稳定的游戏标识；使用：前端选项值与已选状态匹配；边界：同一注册表内唯一，小写。
	ID string
	// Name 职责：面向用户显示的游戏名；使用：创建房间协议下拉项；边界：由各游戏 provider 维护。
	Name string
	// Available 职责：当前是否可选用；使用：控制下拉项 disabled；边界：需要进程运行且至少识别到一个端口。
	Available bool
	// Reason 职责：不可用时的简短原因；使用：拼接在禁用选项文本中；边界：可用时为空，文案面向用户。
	Reason string
	// Ports 职责：识别到的端口声明；使用：选中后回填端口行；边界：可用时至少一项，不可用为空。
	Ports []Declaration
}

// Provider 是一个游戏端口自动识别实现。
// 职责：通过 Detect 返回该游戏的进程、端口与可用状态。
// 使用：由 gameport 注册表统一遍历；新增游戏时实现该接口并注册。
// 边界：必须只读，不启动/停止游戏、不写配置；探测失败应返回不可用 Preset 而不是 panic。
type Provider interface {
	// Detect 职责：执行一次只读探测；使用：注册表检测与前端刷新；边界：不能阻塞超过数秒，失败返回不可用结果。
	Detect() Preset
}
