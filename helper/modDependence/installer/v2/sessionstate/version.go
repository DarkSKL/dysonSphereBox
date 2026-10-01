package sessionstate

// 文件职责：定义组件版本替换的安装会话证据。
// 使用：旧文件准备、成功提交和失败清理通过 Manifest 交换版本信息。
// 边界：只定义内部 JSON 与内存查询，不读取数据库或执行文件操作。

// VersionReplacement 保存同一 Mod 的组件版本替换关系。
// 职责：记录安装成功时需要切换的旧组件引用。
// 使用：旧文件移除前持久化，主包完成事务中读取。
// 边界：旧组件关系在提交前保持不变，失败清理无需逆向迁移。
type VersionReplacement struct {
	// OldComponentKey 职责：记录旧版本组件 key；使用：定位旧文件和引用；边界：必须不同于新 key。
	OldComponentKey string `json:"oldComponentKey"`
	// NewComponentKey 职责：记录新版本组件 key；使用：接管共享引用；边界：成功提交前必须保存组件。
	NewComponentKey string `json:"newComponentKey"`
}

// IsReplacedComponent 判断组件是否属于本次被替换的旧版本。
// 职责：区分原有文件记录与本次新增记录，避免失败清理误删旧清单。
// 使用：manifest 补全和会话记录兜底查询过滤时调用。
// 边界：只匹配精确 key；不判断文件是否已复制或移除。
func (manifest Manifest) IsReplacedComponent(componentKey string) bool {
	for _, replacement := range manifest.VersionReplacements {
		if replacement.OldComponentKey == componentKey {
			return true
		}
	}
	return false
}
