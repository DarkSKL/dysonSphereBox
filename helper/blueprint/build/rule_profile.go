package build

// 文件职责：定义最终数值规则审计的版本、来源及覆盖契约。
// 使用：所有生成模式在编码前返回相同的规则证据；边界：来源是已核对的数值事实，不分发游戏实现或宣称 Unity 验收。

import (
	"fmt"

	"DysonSphereBox/helper/blueprint/build/placement"
)

// RuleProfile 职责：标识本次规则所依据的原版构建；使用：结果与回归溯源；边界：量化数据版本另行记录，不伪装成同一版本。
type RuleProfile struct {
	// Version 职责：标识独立规则修订；使用：回归追踪；边界：规则变化必须更新版本。
	Version string `json:"version"`
	// GameVersion 职责：标识几何来源版本；使用：覆盖提示；边界：不是当前运行游戏探测值。
	GameVersion string `json:"gameVersion"`
	// SteamBuildID 职责：标识提取资源构建；使用：精确复验；边界：不读取用户安装状态。
	SteamBuildID string `json:"steamBuildId"`
	// AssemblySHA256 职责：保存核对规则的程序集指纹；使用：来源复验；边界：不携带程序集内容。
	AssemblySHA256 string `json:"assemblySha256"`
	// ResourcesSHA256 职责：保存几何资源指纹；使用：来源复验；边界：不携带美术资源。
	ResourcesSHA256 string `json:"resourcesSha256"`
}

// RuleAudit 职责：汇报最终 float32 记录的数值预检；使用：生成诊断与验收报告；边界：partial 永不等同于游戏内可放置或满产证明。
type RuleAudit struct {
	// Profile 职责：保存数值规则来源；使用：复验；边界：不推断运行时 Mod。
	Profile RuleProfile `json:"profile"`
	// QuantDataVersion 职责：单独保存量化版本；使用：避免混淆量化与几何来源；边界：不暗改方案版本。
	QuantDataVersion string `json:"quantDataVersion"`
	// CoordinateSpace 职责：区分 planet-surface 与 equatorial-tangent；使用：解释预检作用域；边界：切平面不验证纬圈曲率。
	CoordinateSpace string `json:"coordinateSpace"`
	// Coverage 职责：明确数值预检的部分覆盖；使用：用户提示；边界：始终为 partial-numeric。
	Coverage string `json:"coverage"`
	// Checks 职责：列出实际执行的规则和未验证项；使用：前端诊断；边界：硬失败不返回成功结果。
	Checks []LogisticsCheck `json:"checks"`
	// PowerNetworkCount 职责：统计已生成感应塔网络分量；使用：提示需要外接电力的网络数；边界：不是发电量证明。
	PowerNetworkCount int `json:"powerNetworkCount"`
	// Unverified 职责：列出离线规则无法证明的环境条件；使用：游戏验收清单；边界：不能隐藏为通过。
	Unverified []string `json:"unverified"`
}

// verifiedRuleProfile 职责：将规则声明与已校验碰撞目录绑定；使用：最终规则入口；边界：未知版本立即失败而非复用旧数值。
func verifiedRuleProfile() (RuleProfile, error) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return RuleProfile{}, err
	}
	if catalog.Version() != "0.10.35.29151" {
		return RuleProfile{}, fmt.Errorf("游戏规则版本与碰撞目录不匹配：%s", catalog.Version())
	}
	return RuleProfile{Version: "vanilla-25955035-rules-v1", GameVersion: catalog.Version(), SteamBuildID: "25955035", AssemblySHA256: "ef314425dade51f529f184f07be136cd3c2660a6a81515603c7226fbe7ab1d8c", ResourcesSHA256: "8343173562b15b06a400d70cb1510a106d7654f96342857e5925ded5fd1900d1"}, nil
}
