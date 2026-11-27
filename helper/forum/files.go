package forum

// 文件职责：提供社区论坛上传文件校验、安全文件名归一化与下载临时目录管理。
// 使用：由 ForumService 文件选择器、multipart 流式上传与托管资源下载校验调用。
// 边界：只做本地文件探测、格式/体积边界检查与临时目录清理，不发起网络请求或写入游戏目录。

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	environmentHelper "DysonSphereBox/helper/environment"
)

// 论坛各类托管文件体积上限，与服务端契约保持一致。
const (
	// MaxImageBytes 限制单张正文图片最大为 10 MiB。
	MaxImageBytes int64 = 10 << 20
	// MaxVideoBytes 限制单个正文 MP4 视频最大为 128 MiB。
	MaxVideoBytes int64 = 128 << 20
	// MaxBlueprintBytes 限制单个托管蓝图 .txt 文件最大为 512 KiB。
	MaxBlueprintBytes int64 = 512 << 10
	// MaxModBytes 限制单个托管模组 .zip 文件最大为 256 MiB。
	MaxModBytes int64 = 256 << 20
	// MaxSaveBytes 限制单个托管存档 .dsv 文件最大为 512 MiB。
	MaxSaveBytes int64 = 512 << 20
)

// InspectedFile 描述通过本地校验的待上传文件属性。
// 职责：向选择器和上传器传递规范路径、文件名、体积与识别到的 MIME 类型。
// 使用：InspectLocalFile 返回。
// 边界：不持有打开的文件句柄。
type InspectedFile struct {
	// Path 职责：清理后的绝对路径；使用：后续流式读取或本地预览；边界：指向已验证存在的普通文件。
	Path string
	// Name 职责：安全文件名；使用：界面展示与 multipart 文件名；边界：非空。
	Name string
	// Size 职责：文件字节数；使用：体积展示与上限检查；边界：正整数。
	Size int64
	// Ext 职责：小写扩展名（含点号）；使用：类型展示；边界：与 kind 匹配。
	Ext string
	// ContentType 职责：识别到的真实 MIME 类型；使用：媒体预览与上传校验；边界：非空。
	ContentType string
}

// InspectLocalFile 校验待上传的本地文件类型、魔数与大小限制。
// 职责：在提交前拦截超限或格式不符的文件，给出清晰错误提示。
// 使用：文件选择对话框回调及 PublishPost/PublishReply 上传前复核调用。
// 边界：仅读取文件头（蓝图文件读取全文检查 UTF-8），不修改原文件。
// 逻辑流：1. 校验路径与普通文件状态 => 2. 校验扩展名与上限 => 3. 读取文件头识别 MIME/编码 => 4. 返回 InspectedFile。
func InspectLocalFile(kind, rawPath string) (InspectedFile, error) {
	cleanPath := strings.TrimSpace(rawPath)
	if cleanPath == "" {
		return InspectedFile{}, InvalidInput("请选择有效文件")
	}
	info, err := os.Stat(cleanPath)
	if err != nil {
		return InspectedFile{}, InvalidInput("无法读取所选文件，请确认文件仍然存在")
	}
	if info.IsDir() {
		return InspectedFile{}, InvalidInput("不支持选择文件夹，请选择具体文件")
	}
	if info.Size() <= 0 {
		return InspectedFile{}, InvalidInput("文件内容为空，无法上传")
	}

	ext := strings.ToLower(filepath.Ext(info.Name()))
	switch kind {
	case "image":
		if info.Size() > MaxImageBytes {
			return InspectedFile{}, InvalidInput("单张图片不能超过 10 MiB")
		}
		mimeType, err := detectImageMIME(cleanPath, ext)
		if err != nil {
			return InspectedFile{}, err
		}
		return InspectedFile{Path: cleanPath, Name: info.Name(), Size: info.Size(), Ext: ext, ContentType: mimeType}, nil

	case "video":
		if info.Size() > MaxVideoBytes {
			return InspectedFile{}, InvalidInput("视频不能超过 128 MiB")
		}
		if ext != ".mp4" {
			return InspectedFile{}, InvalidInput("正文视频仅支持 MP4 格式（.mp4）")
		}
		if err := detectMP4Header(cleanPath); err != nil {
			return InspectedFile{}, err
		}
		return InspectedFile{Path: cleanPath, Name: info.Name(), Size: info.Size(), Ext: ext, ContentType: "video/mp4"}, nil

	case "blueprint":
		if info.Size() > MaxBlueprintBytes {
			return InspectedFile{}, InvalidInput("蓝图文件不能超过 512 KiB")
		}
		if ext != ".txt" {
			return InspectedFile{}, InvalidInput("托管蓝图文件仅支持 UTF-8 编码的 .txt 文件")
		}
		raw, err := os.ReadFile(cleanPath)
		if err != nil {
			return InspectedFile{}, InvalidInput("读取蓝图文件失败")
		}
		if !utf8.Valid(raw) || strings.TrimSpace(string(raw)) == "" {
			return InspectedFile{}, InvalidInput("蓝图文件必须为非空的 UTF-8 文本")
		}
		return InspectedFile{Path: cleanPath, Name: info.Name(), Size: info.Size(), Ext: ext, ContentType: "text/plain; charset=utf-8"}, nil

	case "mod":
		if info.Size() > MaxModBytes {
			return InspectedFile{}, InvalidInput("模组压缩包不能超过 256 MiB")
		}
		if ext != ".zip" {
			return InspectedFile{}, InvalidInput("托管模组文件仅支持 .zip 压缩包")
		}
		if err := detectZIPHeader(cleanPath); err != nil {
			return InspectedFile{}, err
		}
		return InspectedFile{Path: cleanPath, Name: info.Name(), Size: info.Size(), Ext: ext, ContentType: "application/zip"}, nil

	case "save":
		if info.Size() > MaxSaveBytes {
			return InspectedFile{}, InvalidInput("存档文件不能超过 512 MiB")
		}
		if ext != ".dsv" {
			return InspectedFile{}, InvalidInput("托管存档仅支持戴森球主存档（.dsv）")
		}
		return InspectedFile{Path: cleanPath, Name: info.Name(), Size: info.Size(), Ext: ext, ContentType: "application/octet-stream"}, nil

	default:
		return InspectedFile{}, InvalidInput("不支持的文件类型")
	}
}

// detectImageMIME 读取文件头并验证是否为服务端支持的 JPEG/PNG/GIF/WebP 图片。
func detectImageMIME(path, ext string) (string, error) {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
	default:
		return "", InvalidInput("正文图片仅支持 JPEG、PNG、GIF、WebP 格式")
	}
	header, err := readHeaderBytes(path, 512)
	if err != nil {
		return "", InvalidInput("读取图片文件失败")
	}
	mimeType := http.DetectContentType(header)
	switch mimeType {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return mimeType, nil
	default:
		return "", InvalidInput("图片文件内容与格式不匹配，仅支持 JPEG、PNG、GIF、WebP")
	}
}

// detectMP4Header 校验文件前部是否包含 ISO BMFF ftyp 盒标识。
func detectMP4Header(path string) error {
	header, err := readHeaderBytes(path, 32)
	if err != nil {
		return InvalidInput("读取视频文件失败")
	}
	if len(header) < 12 || string(header[4:8]) != "ftyp" {
		return InvalidInput("视频文件不是有效的 MP4 容器格式")
	}
	return nil
}

// detectZIPHeader 校验文件前部是否具备标准 ZIP 魔数。
func detectZIPHeader(path string) error {
	header, err := readHeaderBytes(path, 4)
	if err != nil {
		return InvalidInput("读取模组压缩包失败")
	}
	if len(header) < 4 || header[0] != 'P' || header[1] != 'K' {
		return InvalidInput("所选模组文件不是有效的 ZIP 压缩包")
	}
	return nil
}

// readHeaderBytes 读取文件开头的最多 n 个字节用于魔数探测。
func readHeaderBytes(path string, n int) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	buf := make([]byte, n)
	readCount, err := file.Read(buf)
	if err != nil && readCount == 0 {
		return nil, err
	}
	return buf[:readCount], nil
}

// SanitizeDownloadFilename 规范化服务端返回的资源展示名，防止路径穿越和 Windows 非法字符。
// 职责：确保下载落盘文件名安全且带有期望扩展名。
// 使用：DownloadVerifiedFile 落盘前调用。
// 边界：不信任任何路径分隔符，仅保留基名并过滤非法字符。
func SanitizeDownloadFilename(rawName, fallbackBase, requiredExt string) string {
	base := strings.TrimSpace(filepath.Base(strings.ReplaceAll(rawName, "\\", "/")))
	if base == "" || base == "." || base == ".." {
		base = fallbackBase
	}
	replacer := strings.NewReplacer(
		"<", "_", ">", "_", ":", "_", "\"", "_", "/", "_", "\\", "_", "|", "_", "?", "_", "*", "_",
	)
	base = strings.TrimSpace(replacer.Replace(base))
	base = strings.Trim(base, ". ")
	if base == "" {
		base = fallbackBase
	}
	if requiredExt != "" && !strings.EqualFold(filepath.Ext(base), requiredExt) {
		base += requiredExt
	}
	return base
}

// CreateDownloadTempDir 在便携版 appData 下创建独立的论坛下载临时目录。
// 职责：为托管资源下载校验提供与游戏目录隔离的临时暂存区并确保目录已创建。
// 使用：DownloadVerifiedFile 下载前调用，调用方使用完毕后需清理返回目录。
// 边界：优先使用 appData/forum/tmp，避免污染用户游戏目录。
func CreateDownloadTempDir(prefix string) (string, error) {
	resolver := environmentHelper.Resolver{}
	dir, err := resolver.LibraryRoot("forum", "tmp", fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()))
	if err != nil {
		return "", failure("requestFailed", "无法创建临时下载目录")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", failure("requestFailed", "无法创建临时下载目录")
	}
	return dir, nil
}
