package bepinex

// 文件职责：维护 BepInEx 在 Thunderstore 中的稳定身份常量。
// 使用：由 BepInEx 安装解析、Thunderstore 市场过滤和依赖链判断共同引用。
// 边界：只描述来源身份，不负责下载、安装、版本选择或本地状态检测。

// Owner 是 BepInEx 在 Thunderstore 中的命名空间。
// 职责：集中保存 BepInEx 依赖识别所需的作者/团队标识。
// 使用：市场过滤、依赖递归判断和最新 BepInEx 包解析时比较。
// 边界：不表示本地安装目录或包可信状态。
const Owner = "xiaoye97"

// Name 是 BepInEx 在 Thunderstore 中的包名称。
// 职责：集中保存 BepInEx 依赖识别所需的包名。
// 使用：市场过滤、依赖递归判断和最新 BepInEx 包解析时比较。
// 边界：不表示本地插件目录名，插件目录仍由安装逻辑决定。
const Name = "BepInEx"
