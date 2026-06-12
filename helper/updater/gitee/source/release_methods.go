package source

// 文件职责：实现 Gitee Release 到 go-selfupdate.SourceRelease 的接口适配。
// 使用：go-selfupdate 检测版本、读取发布说明和选择附件时通过这些方法访问 Release 元数据。
// 边界：只暴露规范化后的只读视图，不执行版本比较、不访问网络、不写入本地文件。

import (
	"time"

	selfupdate "github.com/creativeprojects/go-selfupdate"
)

// GetID 返回稳定的发布 ID，用于适配 go-selfupdate.SourceRelease。
// 职责：实现 SourceRelease 发布 ID 接口。
// 使用：go-selfupdate 在检测和关联资产时调用。
// 边界：返回 normalize 后的值，未规范化时可能为 0。
func (release *giteeRelease) GetID() int64 {
	return release.ID
}

// GetTagName 返回 Gitee 发布标签名。
// 职责：实现 SourceRelease 标签读取接口。
// 使用：go-selfupdate 根据标签判断 Release 版本。
// 边界：不修正标签格式，格式兼容交给更新器处理。
func (release *giteeRelease) GetTagName() string {
	return release.TagName
}

// GetDraft 返回发布是否仍为草稿状态。
// 职责：实现 SourceRelease 草稿状态读取接口。
// 使用：go-selfupdate 筛选可用 Release 时调用。
// 边界：只返回远端状态，不修改 Release 列表。
func (release *giteeRelease) GetDraft() bool {
	return release.Draft
}

// GetPrerelease 返回发布是否为预发布版本。
// 职责：实现 SourceRelease 预发布状态读取接口。
// 使用：go-selfupdate 筛选可用 Release 时调用。
// 边界：只返回远端状态，不在此处决定是否允许预发布。
func (release *giteeRelease) GetPrerelease() bool {
	return release.Prerelease
}

// GetPublishedAt 返回发布时间，缺失时回退到创建时间。
// 职责：实现 SourceRelease 发布时间读取接口。
// 使用：go-selfupdate 和 CheckResult 展示发布时间时调用。
// 边界：PublishedAt 为零值时回退 CreatedAt，二者都缺失则返回零值。
func (release *giteeRelease) GetPublishedAt() time.Time {
	if !time.Time(release.PublishedAt).IsZero() {
		return time.Time(release.PublishedAt)
	}

	return time.Time(release.CreatedAt)
}

// GetReleaseNotes 返回 Gitee 发布说明正文。
// 职责：实现 SourceRelease 发布说明读取接口。
// 使用：CheckAppUpdate 写入前端可展示的 ReleaseNotes。
// 边界：不截断、不解析、不清洗内容。
func (release *giteeRelease) GetReleaseNotes() string {
	return release.Body
}

// GetName 返回发布名称，缺失时 normalize 会回退到标签名。
// 职责：实现 SourceRelease 名称读取接口。
// 使用：CheckAppUpdate 写入 ReleaseName。
// 边界：未调用 normalize 时可能为空。
func (release *giteeRelease) GetName() string {
	return release.Name
}

// GetURL 返回发布页面地址，用于更新提示跳转。
// 职责：实现 SourceRelease URL 读取接口。
// 使用：CheckAppUpdate 写入 ReleaseURL。
// 边界：HTMLURL 缺失时 normalize 用 URL 兜底，仍可能不是可浏览页面。
func (release *giteeRelease) GetURL() string {
	return release.HTMLURL
}

// GetAssets 返回 go-selfupdate 可识别的发布附件列表。
// 职责：把内部 giteeAsset 切片转换为 SourceAsset 接口切片。
// 使用：go-selfupdate 根据当前平台和校验链选择合适附件。
// 边界：只转换视图，不过滤附件；下载可用性由后续请求校验。
func (release *giteeRelease) GetAssets() []selfupdate.SourceAsset {
	assets := make([]selfupdate.SourceAsset, 0, len(release.Assets))
	for index := range release.Assets {
		assets = append(assets, &release.Assets[index])
	}

	return assets
}
