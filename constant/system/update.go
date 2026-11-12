package system

// 文件职责：定义应用更新进度事件；使用：更新服务与前端订阅共同遵循该契约；边界：不承载模组安装进度。

// AppUpdateProgressEvent 职责：标识应用更新包下载进度；使用：Wails 服务广播；边界：不表示可执行文件已替换成功。
const AppUpdateProgressEvent = "system:appUpdateProgress"
