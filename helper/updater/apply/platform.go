package apply

// 文件职责：封装应用自更新资产应用阶段的平台路径处理。
// 使用：ApplyRelease 解压 Release 资产和修正 Windows 可执行文件扩展名时调用。
// 边界：只处理命令名和目标路径，不访问远端版本、不决定是否需要更新。

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// archiveExecutableName 返回当前平台在更新压缩包内的可执行文件名。
// 职责：根据运行平台补齐 Windows `.exe` 后缀。
// 使用：ApplyRelease 解压 Release 资产时传给 go-selfupdate.DecompressCommand。
// 边界：只描述压缩包内命令名，不读取当前进程路径。
func archiveExecutableName(commandName string) string {
	if runtime.GOOS == "windows" {
		return commandName + ".exe"
	}

	return commandName
}

// ensureWindowsExecutableExtension 修正 Windows 自更新后的可执行文件扩展名。
// 职责：处理 go-selfupdate 在 Windows 下可能写出无 `.exe` 后缀目标文件的情况。
// 使用：ApplyRelease 成功应用更新后调用。
// 边界：非 Windows 或目标已带 `.exe` 时直接返回；重命名失败会阻止本轮更新视为成功。
// 逻辑流：1. 判断平台和后缀 => 2. 删除可能残留的目标 exe => 3. 将无后缀文件重命名为 exe。
func ensureWindowsExecutableExtension(executablePath string) error {
	if runtime.GOOS != "windows" || strings.EqualFold(filepath.Ext(executablePath), ".exe") {
		return nil
	}

	executablePathWithExt := executablePath + ".exe"
	_ = os.Remove(executablePathWithExt)
	if err := os.Rename(executablePath, executablePathWithExt); err != nil {
		return fmt.Errorf("rename updated executable to %q: %w", executablePathWithExt, err)
	}

	return nil
}
