package apply

// 文件职责：封装更新压缩包内的平台命令名称。
// 使用：PrepareRelease 解压 Release 资产时调用。
// 边界：不重命名当前程序；替换与重启由退出后的助手负责。

import "runtime"

// archiveExecutableName 职责：补齐压缩包中 Windows 命令的扩展名；使用：解压器查找目标文件；边界：不修改实际程序路径。
func archiveExecutableName(commandName string) string {
	if runtime.GOOS == "windows" {
		return commandName + ".exe"
	}
	return commandName
}
