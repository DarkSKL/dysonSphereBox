package winver

// 文件职责：读取 Windows PE 文件版本资源。
// 使用：BepInEx 状态检测需要读取 BepInEx.dll 文件版本时调用。
// 边界：只封装 Windows version.dll 读取，不判断 BepInEx 是否安装、不读取 Doorstop 配置。

import (
	"DysonSphereBox/utils"
	"fmt"
	"golang.org/x/sys/windows"
	"unsafe"
)

// ReadWindowsFileVersion 读取 Windows PE 文件版本号。
// 职责：通过 version.dll 读取 DLL 的 VS_FIXEDFILEINFO 版本字段。
// 使用：readBepInExVersion 优先读取 BepInEx.dll 文件版本时调用。
// 边界：仅在 Windows API 可用且文件存在时返回版本；任何读取失败都返回空字符串。
// 逻辑流：1. 校验文件存在 => 2. 调用 GetFileVersionInfoSizeW 获取缓冲区大小 => 3. 读取版本资源 => 4. 查询根版本结构 => 5. 格式化四段版本号。
func ReadWindowsFileVersion(path string) string {
	if !utils.FileExists(path) {
		return ""
	}

	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return ""
	}

	versionDLL := windows.NewLazySystemDLL("version.dll")
	getFileVersionInfoSizeW := versionDLL.NewProc("GetFileVersionInfoSizeW")
	getFileVersionInfoW := versionDLL.NewProc("GetFileVersionInfoW")
	verQueryValueW := versionDLL.NewProc("VerQueryValueW")

	size, _, _ := getFileVersionInfoSizeW.Call(uintptr(unsafe.Pointer(pathPtr)), 0)
	if size == 0 {
		return ""
	}

	buffer := make([]byte, size)
	ok, _, _ := getFileVersionInfoW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		size,
		uintptr(unsafe.Pointer(&buffer[0])),
	)
	if ok == 0 {
		return ""
	}

	rootPtr, err := windows.UTF16PtrFromString(`\`)
	if err != nil {
		return ""
	}

	var fixedInfoPtr uintptr
	var fixedInfoLength uint32
	ok, _, _ = verQueryValueW.Call(
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(rootPtr)),
		uintptr(unsafe.Pointer(&fixedInfoPtr)),
		uintptr(unsafe.Pointer(&fixedInfoLength)),
	)
	if ok == 0 || fixedInfoPtr == 0 || fixedInfoLength < uint32(unsafe.Sizeof(vsFixedFileInfo{})) {
		return ""
	}

	fixedInfo := (*vsFixedFileInfo)(unsafe.Pointer(fixedInfoPtr))
	if fixedInfo.Signature != 0xFEEF04BD {
		return ""
	}

	return fmt.Sprintf(
		"%d.%d.%d.%d",
		fixedInfo.FileVersionMS>>16,
		fixedInfo.FileVersionMS&0xffff,
		fixedInfo.FileVersionLS>>16,
		fixedInfo.FileVersionLS&0xffff,
	)
}

// vsFixedFileInfo 映射 Windows VS_FIXEDFILEINFO 二进制结构。
// 职责：承载 version.dll 返回的固定版本资源字段。
// 使用：ReadWindowsFileVersion 将 unsafe 指针转换为该结构后读取 FileVersionMS/LS。
// 边界：仅用于 Windows 文件版本读取，不应作为业务 DTO 或持久化模型。
type vsFixedFileInfo struct {
	// Signature 职责：标识版本结构签名；使用：读取前校验必须为 0xFEEF04BD；边界：不匹配时视为无效版本资源。
	Signature uint32
	// StrucVersion 职责：记录结构版本；使用：保留 Windows API 原始字段；边界：当前业务不基于该字段分支。
	StrucVersion uint32
	// FileVersionMS 职责：保存文件版本高 32 位；使用：格式化主版本和次版本；边界：仅在 Signature 有效后读取。
	FileVersionMS uint32
	// FileVersionLS 职责：保存文件版本低 32 位；使用：格式化修订号和构建号；边界：仅在 Signature 有效后读取。
	FileVersionLS uint32
	// ProductVersionMS 职责：保存产品版本高 32 位；使用：保留系统结构字段；边界：BepInEx 状态不展示该字段。
	ProductVersionMS uint32
	// ProductVersionLS 职责：保存产品版本低 32 位；使用：保留系统结构字段；边界：BepInEx 状态不展示该字段。
	ProductVersionLS uint32
	// FileFlagsMask 职责：保存文件标志掩码；使用：保留 Windows API 原始字段；边界：当前业务不解释标志位。
	FileFlagsMask uint32
	// FileFlags 职责：保存文件标志；使用：保留 Windows API 原始字段；边界：当前业务不依据调试/预发布标志分支。
	FileFlags uint32
	// FileOS 职责：保存目标操作系统标识；使用：保留 Windows API 原始字段；边界：不用于判断当前系统兼容性。
	FileOS uint32
	// FileType 职责：保存文件类型；使用：保留 Windows API 原始字段；边界：不用于校验 DLL 类型。
	FileType uint32
	// FileSubtype 职责：保存文件子类型；使用：保留 Windows API 原始字段；边界：当前业务不读取。
	FileSubtype uint32
	// FileDateMS 职责：保存文件日期高 32 位；使用：保留 Windows API 原始字段；边界：不用于版本排序。
	FileDateMS uint32
	// FileDateLS 职责：保存文件日期低 32 位；使用：保留 Windows API 原始字段；边界：不用于版本排序。
	FileDateLS uint32
}
