package repair

// 文件职责：提供历史迁移修复过程使用的路径比较与临时路径生成。
// 使用：RepairScopedFile 和阻塞目录迁移流程判断路径等价、生成临时文件名时调用。
// 边界：只按 Windows 语义做字符串级路径清理和大小写无关比较，不解析符号链接。

import (
	"path/filepath"
	"strings"
)

// scopedRepairTempPath 返回迁移过程使用的临时文件路径。
// 职责：为阻塞目录场景生成同目录临时名称。
// 使用：repairScopedFileWithBlockingDirectory 调用。
// 边界：只拼接后缀，不创建文件，也不检查是否存在。
func scopedRepairTempPath(path string) string {
	return path + ".dsb-migrate"
}

// sameFilesystemPath 判断两个文件系统路径是否等价。
// 职责：统一清理路径并进行大小写无关比较。
// 使用：执行迁移时避免重复移动。
// 边界：按 Windows 语义比较大小写，不解析符号链接。
func sameFilesystemPath(left string, right string) bool {
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}
