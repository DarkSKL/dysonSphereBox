package utils

// 文件职责：读取 Windows 实际文档目录。
// 使用：本地资源目录解析；边界：尊重重定向，不创建目录。

import "golang.org/x/sys/windows"

// DocumentsDirectory 职责：查询当前用户的文档已知文件夹；使用：默认路径探测；边界：失败返回错误，不拼接猜测路径。
func DocumentsDirectory() (string, error) {
	return windows.KnownFolderPath(windows.FOLDERID_Documents, 0)
}
