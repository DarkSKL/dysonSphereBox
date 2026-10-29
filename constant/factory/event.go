package factory

// 文件职责：定义蓝图生成事件名；使用：桌面进度桥接；边界：不承载任务状态或执行逻辑。

// GenerationProgressEvent 职责：统一蓝图阶段通知通道；使用：生成服务向前端发送；边界：订阅方必须按 requestId 隔离任务。
const GenerationProgressEvent = "factory:generationProgress"
