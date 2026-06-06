package utils

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"unicode/utf16"
)

const lnkHeaderSize = 0x4C

func ResolveWindowsShortcutTarget(shortcutPath string) (string, error) {
	content, err := os.ReadFile(shortcutPath)
	if err != nil {
		return "", err
	}

	return ParseWindowsShortcutTarget(content)
}

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
