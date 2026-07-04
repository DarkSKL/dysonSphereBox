package system

// 文件职责：定义首次使用引导教程的稳定状态常量。
// 使用：由系统教程服务写入数据库状态、前端通过 Wails DTO 判断是否已完成或跳过教程。
// 边界：只保存跨层共享的枚举值，不执行数据库读写、不描述具体页面教程步骤。

// TutorialStatusCompleted 表示用户已经完整看完某页某版本教程。
// 职责：作为教程完成状态的唯一稳定枚举值。
// 使用：TutorialService.MarkTutorialCompleted 写入数据库，前端查询 DTO 后据此停止自动弹出。
// 边界：不表示用户跳过；新版教程通过 tutorial_version 另建记录。
const TutorialStatusCompleted = "completed"

// TutorialStatusSkipped 表示用户主动关闭并跳过某页某版本教程。
// 职责：作为教程跳过状态的唯一稳定枚举值。
// 使用：TutorialService.SkipTutorial 写入数据库，前端后续进入同版本页面时不再自动弹出。
// 边界：不表示教程内容已阅读完成；用户可通过手动入口重新查看并覆盖状态。
const TutorialStatusSkipped = "skipped"
