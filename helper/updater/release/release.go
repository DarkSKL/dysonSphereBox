package release

// 文件职责：提供应用自更新 Release 元数据的仓库构造、版本比较和时间格式化工具。
// 使用：updater 根包在检查更新和应用更新前调用，用于定位仓库、判断新旧版本和生成前端展示时间。
// 边界：只处理 Release 元数据，不创建更新器、不下载资产、不校验文件、不写入本地可执行文件。

import (
	selfupdate "github.com/creativeprojects/go-selfupdate"
	"strings"
	"time"
)

// Repository 构造应用自更新仓库标识。
// 职责：把 owner/repo 转换为 go-selfupdate 可识别的 Repository。
// 使用：updater.CheckAppUpdate 和 updater.UpdateAppSelf 检测远端 Release 时调用。
// 边界：只描述仓库位置，不访问网络，不做版本判断。
func Repository(owner string, repo string) selfupdate.Repository {
	return selfupdate.NewRepositorySlug(owner, repo)
}

// IsNewer 判断远端 Release 是否新于当前版本。
// 职责：封装 dev/空版本和缺失 `v` 前缀时的比较规则。
// 使用：updater.CheckAppUpdate 计算 HasUpdate 字段时调用。
// 边界：nil Release 返回 false；dev 或空当前版本按可更新处理，便于开发构建测试更新链路。
func IsNewer(release *selfupdate.Release, current string) bool {
	if release == nil {
		return false
	}

	normalizedCurrent := strings.TrimSpace(current)
	if normalizedCurrent == "" || strings.Contains(normalizedCurrent, "dev") {
		return true
	}
	if !strings.HasPrefix(strings.ToLower(normalizedCurrent), "v") {
		normalizedCurrent = "v" + normalizedCurrent
	}

	return release.GreaterThan(normalizedCurrent)
}

// FormatTime 将发布时间转换为前端展示字符串。
// 职责：统一更新检查结果中的时间格式。
// 使用：updater.CheckAppUpdate 写入 PublishedAt 时调用。
// 边界：零值时间返回空字符串，不向前端暴露无意义时间。
func FormatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}

	return value.Local().Format("2006-01-02 15:04:05")
}
