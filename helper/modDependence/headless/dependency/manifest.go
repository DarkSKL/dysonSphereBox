package dependency

// 文件职责：解析 Nebula 无头服务器依赖目录中的 manifest.json 展示信息。
// 使用：Inspect 在依赖目录存在时读取 manifest，并用它覆盖内置依赖名称、版本、描述和来源。
// 边界：只解析展示字段，不递归检查 manifest 依赖、不作为安装清单或数据库模型。

import (
	"encoding/json"
	"os"
)

// manifest 表示依赖 Mod 的 manifest.json 片段。
// 职责：承载依赖目录内 manifest 的名称、版本、描述和来源。
// 使用：readManifest 读取后覆盖默认依赖展示信息。
// 边界：只解析展示所需字段，不作为安装清单或数据库模型。
type manifest struct {
	// Name 职责：记录 manifest 名称；使用：覆盖依赖显示名；边界：为空时保留内置名称。
	Name string `json:"name"`
	// VersionNumber 职责：记录 manifest 版本号；使用：前端显示本地依赖版本；边界：不做语义版本比较。
	VersionNumber string `json:"version_number"`
	// Description 职责：记录 manifest 描述；使用：覆盖依赖说明；边界：为空时保留内置描述。
	Description string `json:"description"`
	// WebsiteURL 职责：记录 manifest 来源链接；使用：覆盖依赖来源；边界：后端不会自动访问该链接。
	WebsiteURL string `json:"website_url"`
	// Dependencies 职责：记录 manifest 依赖声明；使用：保留解析字段供未来扩展；边界：当前开服检测不递归检查该列表。
	Dependencies []string `json:"dependencies"`
}

// readManifest 读取依赖目录中的 manifest.json。
// 职责：解析依赖 Mod 的名称、版本、描述和来源字段。
// 使用：Inspect 在依赖目录存在时调用。
// 边界：读取或解析失败返回错误，调用方保留默认展示信息。
func readManifest(path string) (manifest, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return manifest{}, err
	}

	var result manifest
	if err := json.Unmarshal(content, &result); err != nil {
		return manifest{}, err
	}

	return result, nil
}
