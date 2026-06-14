package environment

// 文件职责：扫描环境管理客户端资源库和游戏真实目录的只读信息。
// 使用：EnvironmentManagerService 列表接口与首次同步预览接口调用。
// 边界：只读取文件系统并生成 DTO 数据，不复制、不删除、不移动、不写入数据库。

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ModResource 描述客户端资源库中的 Mod 项。
// 职责：保存扫描得到的 Mod 目录摘要和 manifest 元数据。
// 使用：服务层转换为前端 Wails DTO。
// 边界：只读资源库信息；启用状态需由同步记录或后续服务复核决定。
type ModResource struct {
	// ID 职责：保存基于资源库相对路径生成的稳定 ID；使用：前端列表 key；边界：不作为数据库持久 ID 的最终来源。
	ID string `json:"id"`
	// Name 职责：保存显示名；使用：前端卡片展示；边界：manifest 缺失时回退目录名。
	Name string `json:"name"`
	// Version 职责：保存 manifest 版本；使用：前端展示；边界：缺失时为空。
	Version string `json:"version"`
	// Author 职责：保存 manifest 作者；使用：前端详情扩展；边界：缺失时为空。
	Author string `json:"author"`
	// Description 职责：保存 manifest 描述；使用：后续详情页展示；边界：不做 Markdown 清洗。
	Description string `json:"description"`
	// Manifest 职责：保存 manifest.json 原始解析结果；使用：Mod 详情展示依赖、官网和原始字段。
	// 边界：只保存 JSON 对象内容；缺失或解析失败时为空 map。
	Manifest map[string]interface{} `json:"manifest"`
	// State 职责：保存只读扫描状态；使用：前端状态徽标；边界：第一阶段资源库扫描默认 default。
	State string `json:"state"`
	// SizeBytes 职责：保存目录递归大小；使用：前端格式化展示；边界：无法访问的文件跳过。
	SizeBytes int64 `json:"sizeBytes"`
	// UpdatedAt 职责：保存目录内最新文件修改时间；使用：前端排序和展示；边界：无法读取时为零值。
	UpdatedAt time.Time `json:"updatedAt"`
	// LibraryPath 职责：保存资源库相对路径；使用：后续导入和同步计划定位；边界：不包含 AppData 绝对路径。
	LibraryPath string `json:"libraryPath"`
	// Supported 职责：标记是否符合第一版目录型 Mod 管理范围；使用：前端提示裸 DLL 不支持；边界：只基于扫描规则判断。
	Supported bool `json:"supported"`
	// Warning 职责：保存只读扫描警告；使用：前端卡片或详情提示；边界：不阻断其它资源展示。
	Warning string `json:"warning"`
}

// BlueprintResource 描述客户端资源库中的蓝图文件或文件夹。
// 职责：保存蓝图镜像目录树节点的只读摘要。
// 使用：服务层按 parentId 查询当前目录内容。
// 边界：只读扫描不会创建环境目录树数据库记录。
type BlueprintResource struct {
	// ID 职责：保存节点稳定 ID；使用：前端卡片 key 和面包屑导航；边界：由相对路径派生。
	ID string `json:"id"`
	// ParentID 职责：保存父文件夹节点 ID；使用：前端目录过滤；边界：根目录为空。
	ParentID string `json:"parentId"`
	// Type 职责：保存 folder 或 blueprint；使用：前端选择图标和交互；边界：只基于文件系统类型。
	Type string `json:"type"`
	// Name 职责：保存节点显示名；使用：前端卡片展示；边界：蓝图文件去扩展名显示。
	Name string `json:"name"`
	// State 职责：保存只读扫描状态；使用：前端状态徽标；边界：第一阶段默认 default。
	State string `json:"state"`
	// SizeBytes 职责：保存文件或目录大小；使用：前端展示；边界：目录大小为递归估算。
	SizeBytes int64 `json:"sizeBytes"`
	// UpdatedAt 职责：保存文件或目录最新修改时间；使用：前端排序和展示；边界：无法读取时为零值。
	UpdatedAt time.Time `json:"updatedAt"`
	// RelativePath 职责：保存相对蓝图资源库根的路径；使用：后续移动和同步计划定位；边界：必须保持相对路径。
	RelativePath string `json:"relativePath"`
	// Extension 职责：保存蓝图文件扩展名；使用：前端提示 txt/bp/json 类型；边界：文件夹为空。
	Extension string `json:"extension"`
}

// SaveResource 描述游戏真实存档目录中的存档文件。
// 职责：保存实时扫描得到的存档文件摘要。
// 使用：服务层返回给存档标签页，并可写入最近扫描索引。
// 边界：存档身份以真实目录为准；该结构不代表资源库项目。
type SaveResource struct {
	// ID 职责：保存基于 SaveRoot 相对路径生成的稳定 ID；使用：前端列表 key 和后续操作定位；边界：操作前仍需重新扫描。
	ID string `json:"id"`
	// Name 职责：保存去扩展名后的显示名；使用：前端卡片展示；边界：不解析存档内部元数据。
	Name string `json:"name"`
	// FileName 职责：保存真实文件名；使用：前端详情和导出默认名；边界：不包含目录。
	FileName string `json:"fileName"`
	// RelativePath 职责：保存相对 SaveRoot 的路径；使用：操作前边界校验；边界：不得用于跨根目录操作。
	RelativePath string `json:"relativePath"`
	// SizeBytes 职责：保存文件大小；使用：前端展示；边界：不表示可写性。
	SizeBytes int64 `json:"sizeBytes"`
	// LastModifiedAt 职责：保存文件最后修改时间；使用：前端显示最近保存；边界：不解析游戏内部时间。
	LastModifiedAt time.Time `json:"lastModifiedAt"`
	// Status 职责：保存扫描状态；使用：前端展示 normal/invalid；边界：locked 第一阶段不做占用检测。
	Status string `json:"status"`
}

// ScanMods 扫描客户端 Mod 资源库。
// 职责：读取 AppData/plugins 下的一级 Mod 目录并生成摘要。
// 使用：ListMods 和概览计数调用。
// 边界：裸 DLL 只作为不支持项提示；不会写入数据库或复制到游戏目录。
func ScanMods(libraryRoot string) ([]ModResource, error) {
	entries, err := os.ReadDir(libraryRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return []ModResource{}, nil
		}
		return nil, err
	}

	resources := make([]ModResource, 0, len(entries))
	for _, entry := range entries {
		fullPath := filepath.Join(libraryRoot, entry.Name())
		if !entry.IsDir() {
			if strings.EqualFold(filepath.Ext(entry.Name()), ".dll") {
				info, _ := entry.Info()
				resources = append(resources, ModResource{
					ID:          stableIDFromPath("mod", entry.Name()),
					Name:        strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())),
					State:       EnvironmentStateMissing,
					SizeBytes:   fileSize(info),
					UpdatedAt:   fileModTime(info),
					LibraryPath: filepath.ToSlash(filepath.Join("plugins", entry.Name())),
					Supported:   false,
					Warning:     "第一版暂不支持裸 DLL Mod，请导入目录或压缩包",
				})
			}
			continue
		}

		sizeBytes, updatedAt := scanPathSummary(fullPath)
		manifest := readManifestJSON(filepath.Join(fullPath, "manifest.json"))
		name := stringFromManifest(manifest, "name")
		if name == "" {
			name = entry.Name()
		}
		resources = append(resources, ModResource{
			ID:          stableIDFromPath("mod", entry.Name()),
			Name:        name,
			Version:     stringFromManifest(manifest, "version_number"),
			Author:      stringFromManifest(manifest, "author"),
			Description: stringFromManifest(manifest, "description"),
			Manifest:    manifest,
			State:       EnvironmentStateDefault,
			SizeBytes:   sizeBytes,
			UpdatedAt:   updatedAt,
			LibraryPath: filepath.ToSlash(filepath.Join("plugins", entry.Name())),
			Supported:   true,
		})
	}

	sort.Slice(resources, func(i int, j int) bool {
		return strings.ToLower(resources[i].Name) < strings.ToLower(resources[j].Name)
	})
	return resources, nil
}

// ScanBlueprints 扫描客户端蓝图镜像资源库。
// 职责：读取 AppData/blueprints 下的目录和蓝图文件并生成树状节点。
// 使用：ListBlueprints 和概览计数调用。
// 边界：只纳入 .txt、.bp、.json 文件，其中 .json 作为需后续确认的可见项。
func ScanBlueprints(libraryRoot string) ([]BlueprintResource, error) {
	if !dirExists(libraryRoot) {
		return []BlueprintResource{}, nil
	}

	resources := []BlueprintResource{}
	err := filepath.WalkDir(libraryRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if path == libraryRoot {
			return nil
		}

		relativePath, err := filepath.Rel(libraryRoot, path)
		if err != nil {
			return nil
		}
		relativePath = filepath.Clean(relativePath)
		parentID := ""
		parentPath := filepath.Dir(relativePath)
		if parentPath != "." {
			parentID = stableIDFromPath("bpf", parentPath)
		}

		if entry.IsDir() {
			sizeBytes, updatedAt := scanPathSummary(path)
			resources = append(resources, BlueprintResource{
				ID:           stableIDFromPath("bpf", relativePath),
				ParentID:     parentID,
				Type:         "folder",
				Name:         entry.Name(),
				SizeBytes:    sizeBytes,
				UpdatedAt:    updatedAt,
				RelativePath: filepath.ToSlash(relativePath),
			})
			return nil
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".txt" && ext != ".bp" && ext != ".json" {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return nil
		}
		resources = append(resources, BlueprintResource{
			ID:           stableIDFromPath("bp", relativePath),
			ParentID:     parentID,
			Type:         "blueprint",
			Name:         strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())),
			State:        EnvironmentStateDefault,
			SizeBytes:    info.Size(),
			UpdatedAt:    info.ModTime(),
			RelativePath: filepath.ToSlash(relativePath),
			Extension:    ext,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(resources, func(i int, j int) bool {
		if resources[i].Type != resources[j].Type {
			return resources[i].Type == "folder"
		}
		return strings.ToLower(resources[i].Name) < strings.ToLower(resources[j].Name)
	})
	return resources, nil
}

// ScanSaves 扫描真实存档目录。
// 职责：读取 SaveRoot 下的存档文件并忽略环境管理临时文件。
// 使用：ListSaves、RefreshSaves 和概览计数调用。
// 边界：不复制存档、不解析游戏内部元数据、不跟随目录树操作。
func ScanSaves(saveRoot string) ([]SaveResource, error) {
	if !dirExists(saveRoot) {
		return []SaveResource{}, nil
	}

	resources := []SaveResource{}
	err := filepath.WalkDir(saveRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if path == saveRoot {
			return nil
		}
		if entry.IsDir() {
			if entry.Name() == ".dsbox-tmp" {
				return filepath.SkipDir
			}
			return nil
		}
		if isTemporarySaveFile(entry.Name()) {
			return nil
		}

		relativePath, err := filepath.Rel(saveRoot, path)
		if err != nil {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		status := "normal"
		if strings.EqualFold(filepath.Ext(entry.Name()), ".tmp") {
			status = "invalid"
		}
		resources = append(resources, SaveResource{
			ID:             stableIDFromPath("save", relativePath),
			Name:           strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())),
			FileName:       entry.Name(),
			RelativePath:   filepath.ToSlash(filepath.Clean(relativePath)),
			SizeBytes:      info.Size(),
			LastModifiedAt: info.ModTime(),
			Status:         status,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(resources, func(i int, j int) bool {
		return resources[i].LastModifiedAt.After(resources[j].LastModifiedAt)
	})
	return resources, nil
}

// CountFirstLevelDirs 统计指定目录下一级目录数量。
// 职责：为首次同步预览统计游戏侧已有 Mod 数量。
// 使用：PreviewInitialSync 调用。
// 边界：目录不存在返回 0；普通文件不计入目录型 Mod。
func CountFirstLevelDirs(root string) int {
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			count++
		}
	}
	return count
}

// CountBlueprintFiles 统计蓝图根目录下可纳管蓝图文件数量。
// 职责：为首次同步预览统计游戏侧已有蓝图文件。
// 使用：PreviewInitialSync 调用。
// 边界：只统计 .txt 和 .bp；.json 等未知扩展需要后续用户确认，不纳入自动数量。
func CountBlueprintFiles(root string) int {
	if !dirExists(root) {
		return 0
	}
	count := 0
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".txt" || ext == ".bp" {
			count++
		}
		return nil
	})
	return count
}

// scanPathSummary 递归统计路径大小和最新修改时间。
// 职责：为资源卡片提供轻量摘要。
// 使用：Mod 和蓝图资源库扫描调用。
// 边界：无法访问的文件跳过，不返回错误以避免单个文件阻断列表。
func scanPathSummary(root string) (int64, time.Time) {
	var sizeBytes int64
	var updatedAt time.Time
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			sizeBytes += info.Size()
		}
		if info.ModTime().After(updatedAt) {
			updatedAt = info.ModTime()
		}
		return nil
	})
	return sizeBytes, updatedAt
}

// stringFromManifest 从 manifest 字段中读取字符串。
// 职责：兼容 Thunderstore manifest 常见字段。
// 使用：Mod 资源库扫描时读取 name、version_number、author、description。
// 边界：非字符串或缺失字段返回空字符串。
func stringFromManifest(manifest map[string]interface{}, key string) string {
	value, ok := manifest[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}

// isTemporarySaveFile 判断存档扫描应忽略的临时文件。
// 职责：过滤环境管理导入过程中的半成品文件。
// 使用：ScanSaves 遍历文件时调用。
// 边界：只按文件名后缀判断，不删除临时文件。
func isTemporarySaveFile(name string) bool {
	lowerName := strings.ToLower(name)
	return strings.HasSuffix(lowerName, ".dsbox.tmp") || strings.HasSuffix(lowerName, ".dsbox-part")
}

// fileSize 安全读取文件大小。
// 职责：从可能为空的 FileInfo 中返回大小。
// 使用：裸 DLL 不支持项扫描时调用。
// 边界：info 为空返回 0。
func fileSize(info os.FileInfo) int64 {
	if info == nil {
		return 0
	}
	return info.Size()
}

// fileModTime 安全读取文件修改时间。
// 职责：从可能为空的 FileInfo 中返回修改时间。
// 使用：裸 DLL 不支持项扫描时调用。
// 边界：info 为空返回零值。
func fileModTime(info os.FileInfo) time.Time {
	if info == nil {
		return time.Time{}
	}
	return info.ModTime()
}
