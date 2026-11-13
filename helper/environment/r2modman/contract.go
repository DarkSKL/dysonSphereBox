package r2modman

// 文件职责：定义 Profile 检索及直接托管的只读契约。
// 使用：环境服务返回 Wails 类型，扫描器和写入器共享状态。
// 边界：不保存数据库模型，不把前端字段作为文件归属证据。

// Profile 描述已验证的戴森球计划 r2modman 配置档。
// 职责：提供名称、位置与数量；使用：同步选择器展示；边界：检索不自动托管。
type Profile struct {
	// Name 职责：显示配置档名称；使用：资源分区标题；边界：源于目录名称。
	Name string `json:"name"`
	// Path 职责：保存规范化绝对路径；使用：明确选择与防止旧页面误写；边界：写入时重新验证。
	Path string `json:"path"`
	// ModCount 职责：展示已登记的包数量；使用：选择器摘要；边界：包含不可切换的框架包。
	ModCount int `json:"modCount"`
	// EnabledCount 职责：展示清单已启用包数量；使用：摘要；边界：不代表已启动游戏加载。
	EnabledCount int `json:"enabledCount"`
}

// Mod 描述 Profile 中的一条模组清单记录。
// 职责：展示状态及管理边界；使用：卡片和详情；边界：每次操作重新读取 mods.yml 和实际文件。
type Mod struct {
	// Name 职责：保存作者与包名组成的标识；使用：操作参数；边界：禁止路径分隔符和穿越片段。
	Name string `json:"name"`
	// DisplayName 职责：保存展示名；使用：卡片；边界：缺失回退 Name。
	DisplayName string `json:"displayName"`
	// Author 职责：保存作者；使用：详情；边界：缺失为空。
	Author string `json:"author"`
	// Version 职责：保存三段版本号；使用：卡片；边界：格式来自清单。
	Version string `json:"version"`
	// Description 职责：保存包说明；使用：详情；边界：按纯文本展示。
	Description string `json:"description"`
	// WebsiteURL 职责：保存包网站；使用：详情；边界：不主动请求远端。
	WebsiteURL string `json:"websiteUrl"`
	// Dependencies 职责：保存声明的依赖标识；使用：详情与操作校验；边界：不自动下载缺失依赖。
	Dependencies []string `json:"dependencies"`
	// Enabled 职责：保存 R2 清单状态；使用：卡片；边界：实际文件不一致时禁止修改。
	Enabled bool `json:"enabled"`
	// CanManage 职责：标记支持按明确归属修改；使用：前端禁用不支持操作；边界：后端独立校验。
	CanManage bool `json:"canManage"`
	// CanDisable 职责：标记当前包及启用中的依赖者能否安全禁用；使用：前端锁定真正不可管理项；边界：联动禁用仍需明确确认范围。
	CanDisable bool `json:"canDisable"`
	// DisableWith 职责：列出禁用当前包时需联动禁用的其他包标识；使用：前端确认清单；边界：实际提交重新计算并核对范围，不自动扩大用户确认。
	DisableWith []string `json:"disableWith"`
	// DisableReason 职责：解释不能单独禁用的原因；使用：锁定提示和详情；边界：不是操作授权。
	DisableReason string `json:"disableReason"`
	// CanRemove 职责：标记当前包是否可单独移除；使用：右键移除菜单；边界：有已安装依赖者时为 false。
	CanRemove bool `json:"canRemove"`
	// RemoveReason 职责：解释移除限制；使用：详情与菜单提示；边界：不影响查看包详情。
	RemoveReason string `json:"removeReason"`
	// Warning 职责：解释框架、共享文件或异常布局；使用：卡片提示；边界：不隐藏清单记录。
	Warning string `json:"warning"`
}

// ProfileState 聚合选中的 Profile 及当前清单。
// 职责：向资源页返回一致快照；使用：加载和操作后刷新；边界：未托管时 Profile 为空，Mods 为空数组。
type ProfileState struct {
	// Profile 职责：标记当前托管目标；使用：标题和后续操作；边界：未选择时为 nil。
	Profile *Profile `json:"profile"`
	// Mods 职责：保存该配置档的包列表；使用：资源页渲染；边界：不混入盒子本地库或市场安装记录。
	Mods []Mod `json:"mods"`
}
