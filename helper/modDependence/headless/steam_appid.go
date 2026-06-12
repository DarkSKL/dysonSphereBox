package headless

// 文件职责：检查无头服务器基础文件 steam_appid.txt 的存在性和内容。
// 使用：InspectHeadlessEnvironment 展示状态，EnsureHeadlessBaseEnvironment 判断是否需要写入。
// 边界：只读取指定文件并比较 AppID，不创建文件、不区分读取失败原因。

import (
	"os"
	"strings"
)

// inspectSteamAppID 检查 steam_appid.txt 是否存在且内容正确。
// 职责：返回文件存在性和内容是否等于戴森球计划 AppID。
// 使用：InspectHeadlessEnvironment 和 EnsureHeadlessBaseEnvironment 判断是否需要写入。
// 边界：读取失败统一返回 false,false，不区分权限和缺失。
func inspectSteamAppID(path string) (bool, bool) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, false
	}

	return true, strings.TrimSpace(string(content)) == headlessSteamAppID
}
