package overlay

// 文件职责：集中维护悬浮窗原型监听的目标进程名称。
// 使用：由 Overlay 窗口查找 helper 和前端状态展示共同引用，第一版以浏览器进程作为可验证目标。
// 边界：这里只保存稳定常量，不负责枚举窗口、启动附进程或决定业务状态。

// TargetBrowserProcesses 定义第一版悬浮窗原型允许监听的浏览器进程。
// 使用：窗口查找流程按这些进程名匹配可见顶层窗口，后续切换为 DSPGAME.exe 时替换该常量或改为配置项。
// 边界：进程名只用于原型验证，不代表最终游戏识别规则。
var TargetBrowserProcesses = []string{
	"chrome.exe",
	"msedge.exe",
	"firefox.exe",
	"brave.exe",
	"opera.exe",
}
