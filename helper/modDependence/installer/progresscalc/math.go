package progresscalc

// 文件职责：提供安装进度百分比的纯计算能力。
// 使用：V2 安装、卸载、下载 reader 和恢复快照在计算对外进度前调用。
// 边界：只处理整数百分比计算，不发送事件、不写数据库、不读取安装状态。

// Percent 根据当前值和总量计算百分比。
// 职责：把字节数或条目数转换为 0 到 100 的整数进度。
// 使用：下载、解压、复制、卸载等阶段根据 current/total 生成局部百分比时调用。
// 边界：total 小于等于 0 时返回 0，结果始终经过 Clamp。
func Percent(current int64, total int64) int {
	if total <= 0 {
		return 0
	}
	return Clamp(int((current * 100) / total))
}

// Map 将局部阶段百分比映射到整体安装百分比区间。
// 职责：把下载、解压、复制等阶段内进度映射到全局进度条。
// 使用：调用方传入阶段起止百分比和阶段内百分比。
// 边界：start/end 会被限制到 0 到 100；end 小于 start 时按 start 处理。
func Map(start int, end int, percent int) int {
	start = Clamp(start)
	end = Clamp(end)
	if end < start {
		end = start
	}
	return Clamp(start + (end-start)*Clamp(percent)/100)
}

// Clamp 限制进度百分比范围。
// 职责：保证所有对外进度位于 0 到 100。
// 使用：事件发送、数据库状态更新和恢复快照写入前调用。
// 边界：小于 0 返回 0，大于 100 返回 100。
func Clamp(percent int) int {
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
}
