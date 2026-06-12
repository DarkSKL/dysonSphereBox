package dependency

// 文件职责：维护 Nebula 无头服务器所需依赖 Mod 的本地检测规格。
// 使用：Inspect 遍历 specs，按目录、默认展示信息和必需文件生成前端依赖状态。
// 边界：规格只用于本地检测，不负责下载、安装、市场查询或版本解析策略。

// spec 定义开服所需依赖 Mod 的本地检测规则。
// 职责：描述依赖目录名、展示信息和必须存在的文件。
// 使用：Inspect 遍历该规格生成前端状态。
// 边界：只做本地检测规则，不负责下载、安装或版本解析策略。
type spec struct {
	// ID 职责：提供依赖内部标识；使用：前端稳定映射依赖项；边界：不等同于 Thunderstore key。
	ID string
	// Name 职责：提供默认依赖名称；使用：manifest 缺失时展示；边界：可被 manifest 覆盖。
	Name string
	// Folder 职责：指定 BepInEx/plugins 下的依赖目录；使用：本地存在性检测；边界：不递归搜索其它目录。
	Folder string
	// Function 职责：概括依赖功能；使用：前端帮助用户理解依赖作用；边界：不参与 Ready 计算。
	Function string
	// Description 职责：提供默认依赖说明；使用：manifest 缺失时展示；边界：可被 manifest 覆盖。
	Description string
	// Source 职责：提供依赖来源链接；使用：前端引导用户查看；边界：后端不会自动下载。
	Source string
	// RequiredFiles 职责：列出判断依赖完整性的必需文件；使用：缺失时写入 MissingFiles；边界：空列表表示目录存在即可视为安装。
	RequiredFiles []string
}

// 依赖规格集合职责：集中维护 Nebula 及辅助依赖的检测规则。
// 使用：Inspect 遍历该列表生成 contract.HeadlessDependencyStatus。
// 边界：该列表只做检测，不承担市场查询、安装排序或依赖下载。
var specs = []spec{
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
