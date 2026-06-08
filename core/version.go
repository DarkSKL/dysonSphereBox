package core

// Version 是当前应用版本。
// 用途：发布构建时通过 -ldflags 注入，例如 -X DysonSphereBox/core.Version=v1.0.0。
// 边界：开发环境未注入时保持 dev 标记，更新检查会把它视为可被正式版本更新。
var Version = "0.0.0-dev"
