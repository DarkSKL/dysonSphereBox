package artifact

// 文件职责：识别 V2 安装流程中应跳过的 BepInEx 基础依赖。
// 使用：依赖解析、会话清理和缓存清理阶段判断组件 key 是否属于 BepInEx 框架包。
// 边界：只匹配 Thunderstore 标准 key 前缀，不访问远端包信息、不安装 BepInEx。

import (
	bepinexConstant "DysonSphereBox/constant/modDependence/bepinex"
	"fmt"
	"strings"
)

// IsBepInExDependencyKey 判断组件 key 是否指向 BepInEx 依赖。
// 职责：识别安装时应跳过的 BepInEx 基础依赖。
// 使用：依赖解析、会话清理和缓存清理阶段调用。
// 边界：只匹配 Thunderstore BepInEx 标准 key 前缀，不做远端包查询。
func IsBepInExDependencyKey(key string) bool {
	return strings.HasPrefix(key, fmt.Sprintf("thunderstore:%s:%s:", bepinexConstant.Owner, bepinexConstant.Name))
}
