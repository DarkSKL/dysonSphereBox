package install

// 文件职责：维护 Mod 安装流程对外发送的事件名常量。
// 使用：installer 进度推送通过该常量向 Wails 前端发送安装进度事件。
// 边界：只定义事件名，不发送事件、不保存进度、不决定前端如何渲染。

// ProgressEventName 是安装进度 Wails 事件名。
// 职责：统一后端安装器和前端监听的事件通道。
// 使用：emitInstallProgress 和 emitRuntimeOnlyProgress 发送进度时引用。
// 边界：修改该值会影响前端监听协议，必须同步更新前端事件订阅。
const ProgressEventName = "modDependence:installProgress"
