package source

// 文件职责：规范化 Gitee Release API 返回的缺省字段。
// 使用：Source.ListReleases 在解码每个 Release 后调用，确保后续接口适配拿到稳定展示字段。
// 边界：只补齐元数据默认值，不筛选版本、不下载附件、不修改远端原始资产内容。

// normalize 规范化 Gitee Release 缺失字段。
// 职责：补齐稳定 ID、展示名称和页面地址，提升不同 Gitee 响应形状的兼容性。
// 使用：Source.ListReleases 解码每个 Release 后立即调用。
// 边界：只填充元数据默认值，不筛选草稿、不比较版本、不修改附件下载内容。
func (release *giteeRelease) normalize() {
	if release.ID == 0 {
		release.ID = stableID(release.TagName)
	}
	if release.Name == "" {
		release.Name = release.TagName
	}
	if release.HTMLURL == "" && release.URL != "" {
		release.HTMLURL = release.URL
	}
}
