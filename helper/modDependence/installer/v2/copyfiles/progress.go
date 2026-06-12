package copyfiles

// 文件职责：发送 V2 文件复制阶段的进度事件。
// 使用：copyPlannedFile 在处理每个规划文件前调用，让前端看到复制阶段的线性进度。
// 边界：只发送进度事件，不修改复制结果、不写数据库；totalFiles 为 0 时按 1 兜底避免除零。

import (
	"DysonSphereBox/helper/modDependence/installer/progresscalc"
	"DysonSphereBox/helper/modDependence/installer/v2/installplan"
	progressEvent "DysonSphereBox/helper/modDependence/installer/v2/progress/event"
	"fmt"
)

// emitProgress 发送单文件复制进度事件。
// 职责：把当前文件序号映射到复制阶段整体进度区间。
// 使用：copyPlannedFile 开始处理每个文件时调用。
// 边界：只发送进度事件，不修改复制结果；totalFiles 为 0 时按 1 兜底避免除零。
func (session *copySession) emitProgress(index int, totalFiles int, file installplan.File) {
	progressEvent.Package(
		file.PackageKey,
		"copy",
		fmt.Sprintf("正在复制文件 %d/%d", index+1, totalFiles),
		progresscalc.Map(session.progressStart, session.progressEnd, progresscalc.Percent(int64(index+1), int64(max(totalFiles, 1)))),
	)
}
