package core

// 文件职责：维护应用基础设施模块 version。
// 使用：由启动流程初始化，service/helper 在运行期读取已初始化资源。
// 边界：不实现具体业务用例，不直接处理 Mod 安装、市场查询或页面交互。

// Version 是当前应用版本。
// 职责：保存当前构建的语义化版本标识。
// 使用：发布构建时通过 -ldflags 注入，例如 -X DysonSphereBox/core.Version=v1.0.0。
// 边界：开发环境未注入时保持 dev 标记，更新检查会把它视为可被正式版本更新。
var Version = "v0.0.0"
