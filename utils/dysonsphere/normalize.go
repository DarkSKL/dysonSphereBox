package dysonsphere

// 文件职责：提供戴森球计划游戏目录归一化规则。
// 使用：配置保存、自动搜索和快捷方式解析后调用 NormalizeDir，确保结果真实包含 DSPGAME.exe。
// 边界：只校验本地路径，不写入配置、不访问数据库、不执行游戏文件。

import (
	"path/filepath"
	"strings"

	"DysonSphereBox/utils"
)

// NormalizeDir 将用户选择的路径归一化为包含 DSPGAME.exe 的游戏目录。
// 职责：把用户选择的 exe 路径或目录路径统一转换为游戏根目录。
// 使用：配置保存、自动搜索和快捷方式解析后调用，确保返回目录真实包含 DSPGAME.exe。
// 边界：可接受 exe 文件路径或目录路径；校验失败返回空字符串，不猜测不存在的目录。
func NormalizeDir(path string) string {
	path = utils.CleanPath(path)
	if path == "" {
		return ""
	}

	if strings.EqualFold(filepath.Base(path), ExeName) {
		if utils.FileExists(path) {
			return filepath.Dir(path)
		}
		return ""
	}

	if utils.FileExists(filepath.Join(path, ExeName)) {
		return path
	}

	return ""
}
