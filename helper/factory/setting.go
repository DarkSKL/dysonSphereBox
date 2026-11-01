package factory

// 文件职责：规范蓝图生成仍支持的布局设置；使用：公共生成和离线方案导出；边界：不改量化方案、不执行布局。

import "DysonSphereBox/helper/blueprint/build"

// NormalizeCurrentLayoutSetting 清除已移除的混带输入开关。
// 使用：服务层生成请求及方案导出前调用，避免旧客户端状态重新启用混带布局。
// 边界：只固定已移除字段，其他布局选项仍由生成器验证。
func NormalizeCurrentLayoutSetting(setting build.Setting) build.Setting {
	setting.UseMixed = false
	disabled := false
	setting.Mix2 = &disabled
	return setting
}
