package utils

// 文件职责：提供 Windows .lnk 快捷方式目标路径读取和 Shell Link 二进制解析工具。
// 使用：戴森球计划自动路径探测在 Steam 探测失败后解析桌面或开始菜单快捷方式时调用。
// 边界：只解析本地快捷方式目标路径，不执行快捷方式、不访问数据库、不判断目标是否为游戏目录。

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"unicode/utf16"
)

// lnkHeaderSize 是 Windows Shell Link 文件头长度。
// 职责：限定 .lnk 二进制解析的最小可读范围。
// 使用：ParseWindowsShortcutTarget 检查内容长度和定位后续结构时读取。
// 边界：只适用于 Windows .lnk 格式，不代表其它快捷方式或 URL 文件。
const lnkHeaderSize = 0x4C

// ResolveWindowsShortcutTarget 读取 Windows 快捷方式并解析目标路径。
// 职责：从磁盘读取 .lnk 文件并解析其本地目标路径。
// 使用：自动搜索戴森球计划快捷方式时作为 ShortcutResolver 注入。
// 边界：读取失败或快捷方式结构异常时返回错误，不猜测目标路径。
func ResolveWindowsShortcutTarget(shortcutPath string) (string, error) {
	content, err := os.ReadFile(shortcutPath)
	if err != nil {
		return "", err
	}

	return ParseWindowsShortcutTarget(content)
}

// ParseWindowsShortcutTarget 从 .lnk 二进制内容解析本地目标路径。
// 职责：解析 Shell Link Header、可选 IDList 和 LinkInfo 中的本地目标路径。
// 使用：ResolveWindowsShortcutTarget 读取文件后调用，也可由测试直接传入二进制内容。
// 边界：仅解析本地路径信息；缺少目标路径时返回可读错误，不支持网络路径或参数解析。
// 逻辑流：1. 校验 .lnk 头长度 => 2. 根据 flags 跳过 IDList => 3. 读取 LinkInfo 区块 => 4. 优先解析 Unicode 路径 => 5. 兜底解析 ANSI 路径。
func ParseWindowsShortcutTarget(content []byte) (string, error) {
	if len(content) < lnkHeaderSize {
		return "", errors.New("shortcut content is too small")
	}

	flags := binary.LittleEndian.Uint32(content[0x14:0x18])
	offset := lnkHeaderSize

	if flags&0x01 != 0 {
		if len(content) < offset+2 {
			return "", errors.New("shortcut link target id list is truncated")
		}

		idListSize := int(binary.LittleEndian.Uint16(content[offset : offset+2]))
		offset += 2 + idListSize
		if len(content) < offset {
			return "", errors.New("shortcut link target id list exceeds content")
		}
	}

	if flags&0x02 != 0 {
		if len(content) < offset+0x1C {
			return "", errors.New("shortcut link info is truncated")
		}

		linkInfoSize := int(binary.LittleEndian.Uint32(content[offset : offset+4]))
		if len(content) < offset+linkInfoSize {
			return "", errors.New("shortcut link info exceeds content")
		}

		linkInfo := content[offset : offset+linkInfoSize]
		targetPath := parseLinkInfoTargetPath(linkInfo)
		if targetPath != "" {
			return targetPath, nil
		}
	}

	return "", errors.New("shortcut target path not found")
}

// parseLinkInfoTargetPath 从 LinkInfo 区块提取目标路径。
// 职责：封装 Unicode 和 ANSI 本地路径偏移的读取顺序。
// 使用：ParseWindowsShortcutTarget 在确认 LinkInfo 区块完整后调用。
// 边界：LinkInfo 过短或偏移无效时返回空字符串，不返回错误给外层统一处理。
func parseLinkInfoTargetPath(linkInfo []byte) string {
	if len(linkInfo) < 0x1C {
		return ""
	}

	linkInfoHeaderSize := int(binary.LittleEndian.Uint32(linkInfo[4:8]))
	localBasePathOffset := int(binary.LittleEndian.Uint32(linkInfo[16:20]))

	if linkInfoHeaderSize >= 0x24 && len(linkInfo) >= 0x24 {
		localBasePathOffsetUnicode := int(binary.LittleEndian.Uint32(linkInfo[28:32]))
		if targetPath := readUTF16StringAt(linkInfo, localBasePathOffsetUnicode); targetPath != "" {
			return targetPath
		}
	}

	return readCStringAt(linkInfo, localBasePathOffset)
}

// readCStringAt 从指定偏移读取以 NUL 结尾的单字节字符串。
// 职责：解析旧版 .lnk LinkInfo 中的 ANSI 本地路径。
// 使用：parseLinkInfoTargetPath 在 Unicode 路径不存在时兜底调用。
// 边界：偏移无效、未找到结束符或内容为空时返回空字符串。
func readCStringAt(content []byte, offset int) string {
	if offset <= 0 || offset >= len(content) {
		return ""
	}

	end := bytes.IndexByte(content[offset:], 0)
	if end < 0 {
		return ""
	}

	return string(content[offset : offset+end])
}

// readUTF16StringAt 从指定偏移读取以 NUL 结尾的 UTF-16 字符串。
// 职责：解析新版 .lnk LinkInfo 中的 Unicode 本地路径。
// 使用：parseLinkInfoTargetPath 在 LinkInfo 头支持 Unicode 偏移时优先调用。
// 边界：偏移无效或没有有效字符时返回空字符串；不处理非 UTF-16 编码内容。
func readUTF16StringAt(content []byte, offset int) string {
	if offset <= 0 || offset+1 >= len(content) {
		return ""
	}

	chars := make([]uint16, 0, (len(content)-offset)/2)
	for index := offset; index+1 < len(content); index += 2 {
		char := binary.LittleEndian.Uint16(content[index : index+2])
		if char == 0 {
			break
		}

		chars = append(chars, char)
	}
	if len(chars) == 0 {
		return ""
	}

	return string(utf16.Decode(chars))
}
