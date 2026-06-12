package contract

// 文件职责：定义无头服务器依赖 Mod 检测结果契约。
// 使用：dependency.Inspect 构造 HeadlessDependencyStatus，环境状态 DTO 聚合后返回前端。
// 边界：只描述本地检测结果，不包含市场搜索结果、不触发依赖安装、不读取 manifest。

// HeadlessDependencyStatus 表示无头服务器依赖 Mod 在本地插件目录中的状态。
// 职责：描述单个无头服务器依赖 Mod 的本地安装与缺失文件情况。
// 使用：InspectHeadlessEnvironment 汇总依赖状态后返回给开服页。
// 边界：版本来自本地 manifest.json，缺失时不猜测远端版本。
type HeadlessDependencyStatus struct {
	// ID 职责：提供依赖的稳定内部标识；使用：前端按依赖类别渲染状态；边界：不等同于 Thunderstore packageKey。
	ID string `json:"id"`
	// Name 职责：记录依赖显示名称；使用：前端列表展示；边界：manifest 可覆盖默认名称。
	Name string `json:"name"`
	// Folder 职责：记录插件目录名；使用：定位本地 BepInEx/plugins 下的依赖目录；边界：目录名来自内置规格。
	Folder string `json:"folder"`
	// InstalledVersion 职责：记录 manifest 中声明的版本；使用：前端展示本地版本；边界：读取不到 manifest 时为空。
	InstalledVersion string `json:"installedVersion"`
	// Description 职责：说明依赖作用；使用：前端帮助用户理解缺失项；边界：manifest 描述可覆盖默认描述。
	Description string `json:"description"`
	// Function 职责：概括该依赖在开服链路中的功能；使用：前端按功能展示；边界：只作说明，不参与安装判断。
	Function string `json:"function"`
	// Source 职责：记录依赖来源页面或 manifest URL；使用：前端引导用户查找依赖；边界：不会在后端自动下载。
	Source string `json:"source"`
	// PluginDir 职责：记录本地插件目录路径；使用：日志和前端状态展示；边界：目录可能不存在。
	PluginDir string `json:"pluginDir"`
	// ManifestPath 职责：记录 manifest.json 路径；使用：排查版本读取失败；边界：文件缺失不直接视为依赖缺失，仍看 RequiredFiles。
	ManifestPath string `json:"manifestPath"`
	// Installed 职责：表示依赖目录和必需文件是否齐全；使用：计算 NebulaReady；边界：不校验 DLL 内容或运行时兼容性。
	Installed bool `json:"installed"`
	// MissingFiles 职责：列出依赖缺失目录或文件；使用：前端展示阻塞原因；边界：只包含内置规格中的必需项。
	MissingFiles []string `json:"missingFiles"`
}
