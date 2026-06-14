package environment

// 文件职责：解析环境管理涉及的客户端资源库、游戏 Mod、蓝图和存档目录。
// 使用：环境管理服务在概览、列表扫描和首次同步预览时调用。
// 边界：只解析和探测目录，不创建资源文件、不复制资源、不写入数据库配置。

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"DysonSphereBox/core"
	"DysonSphereBox/utils"
	"DysonSphereBox/utils/dysonsphere"
)

// 目录解析常量集中描述环境管理固定路径片段。
// 职责：统一资源库和配置键名，避免服务层重复硬编码。
// 使用：Resolver 和 scanner 在拼接 AppData 与配置读取时引用。
// 边界：常量只表达本地路径约定，不代表目录一定存在。
const (
	// ConfigGroupGame 职责：保存游戏相关配置分组名；使用：读取蓝图、存档和启动路径配置；边界：不直接读写配置。
	ConfigGroupGame = "Game"
	// ConfigKeyBlueprintDirPath 职责：保存用户确认的蓝图根目录配置键；使用：蓝图根目录解析优先读取；边界：值不存在时回退自动探测。
	ConfigKeyBlueprintDirPath = "BlueprintDirPath"
	// ConfigKeySaveDirPath 职责：保存用户确认的存档根目录配置键；使用：存档根目录解析优先读取；边界：值不存在时回退自动探测。
	ConfigKeySaveDirPath = "SaveDirPath"
	// EnvironmentStateDefault 职责：表示资源库项目尚未明确启用或禁用；使用：只读扫描和新导入项展示；边界：临时 syncing 不写入该值。
	EnvironmentStateDefault = "default"
	// EnvironmentStateEnabled 职责：表示资源已同步到游戏目录；使用：首次纳管和后续启用成功后展示；边界：只读资源库扫描不会猜测启用状态。
	EnvironmentStateEnabled = "enabled"
	// EnvironmentStateDisabled 职责：表示资源存在但未同步到游戏目录；使用：后续禁用成功后展示；边界：只读扫描默认不写入该值。
	EnvironmentStateDisabled = "disabled"
	// EnvironmentStateMissing 职责：表示记录存在但源文件丢失；使用：后续状态复核扩展；边界：第一阶段只读扫描不主动写入记录。
	EnvironmentStateMissing = "missing"
	// EnvironmentStateConflict 职责：表示同步记录与真实目标文件不一致；使用：状态复核或禁用冲突时写入，提醒用户手动处理；边界：不代表文件已被修改或删除。
	EnvironmentStateConflict = "conflict"
)

// RootResolution 描述某类环境目录解析结果。
// 职责：统一向服务层返回路径、来源、置信度和是否需要用户确认。
// 使用：GetOverview、列表扫描和前端遮罩判断读取。
// 边界：Path 为空表示未定位；NeedConfirm 为 true 时服务层不得执行写入类操作。
type RootResolution struct {
	// Path 职责：保存解析出的绝对路径；使用：扫描和前端展示；边界：为空表示不可用。
	Path string `json:"path"`
	// Source 职责：保存路径来源；使用：前端提示 configured、detected、default、missing；边界：只用于展示和决策提示。
	Source string `json:"source"`
	// Confidence 职责：保存路径置信度；使用：区分 confirmed、detected、guessed、missing；边界：不替代实际目录存在性校验。
	Confidence string `json:"confidence"`
	// NeedConfirm 职责：标记写入前是否需要用户确认；使用：蓝图默认目录尚未存在时阻止写入；边界：只读扫描仍可返回该路径。
	NeedConfirm bool `json:"needConfirm"`
	// Exists 职责：标记路径当前是否存在；使用：前端显示目录状态；边界：存在不代表有读写权限。
	Exists bool `json:"exists"`
	// Message 职责：保存用户可读解析说明；使用：前端状态条展示；边界：不用于程序分支判断。
	Message string `json:"message"`
}

// Resolver 提供环境管理目录解析能力。
// 职责：从 exe 同级目录、配置项和 Windows 文档目录解析资源库与游戏真实目录。
// 使用：EnvironmentManagerService 的查询和扫描方法创建后调用。
// 边界：不创建业务资源、不修改配置；必须允许目录不存在并返回明确状态。
type Resolver struct{}

// AppDataRoot 解析便携版应用数据根目录。
// 职责：返回 exe 同级 appData 路径，用作环境资源库根。
// 使用：资源库扫描、导入和后续同步流程调用。
// 边界：解析失败时返回错误；本函数不创建目录。
func (resolver Resolver) AppDataRoot() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}

	return filepath.Join(filepath.Dir(exePath), "appData"), nil
}

// LibraryRoot 解析指定资源库子目录。
// 职责：把 AppDataRoot 和资源库片段组合为绝对路径。
// 使用：扫描 plugins、blueprints 和 environment 内部目录时调用。
// 边界：不创建目录；resourceParts 为空时返回 AppDataRoot。
func (resolver Resolver) LibraryRoot(resourceParts ...string) (string, error) {
	appDataRoot, err := resolver.AppDataRoot()
	if err != nil {
		return "", err
	}

	parts := append([]string{appDataRoot}, resourceParts...)
	return filepath.Join(parts...), nil
}

// ResolveModRoot 解析游戏侧 BepInEx/plugins 目录。
// 职责：基于 Game/StartupFilePath 定位游戏目录并拼接 Mod 同步目标根。
// 使用：概览、Mod 首次同步预览和启用禁用流程调用。
// 边界：游戏路径无效时返回空路径；不会创建 BepInEx/plugins。
func (resolver Resolver) ResolveModRoot() RootResolution {
	gamePath := ""
	if core.GLOBAL.Configure != nil && core.GLOBAL.Configure.GameStartFilePath != nil {
		configuredPath, _ := core.GLOBAL.Configure.GameStartFilePath.Get()
		gamePath = dysonsphere.NormalizeDir(configuredPath)
	}
	if gamePath == "" {
		return RootResolution{
			Source:     "missing",
			Confidence: "missing",
			Message:    "未配置有效的游戏目录",
		}
	}

	modRoot := filepath.Join(gamePath, "BepInEx", "plugins")
	return RootResolution{
		Path:       modRoot,
		Source:     "configured",
		Confidence: "confirmed",
		Exists:     dirExists(modRoot),
		Message:    "已根据游戏目录定位 Mod 目录",
	}
}

// ResolveBlueprintRoot 解析游戏真实蓝图目录。
// 职责：优先使用用户配置，其次从文档目录探测 Blueprint/Blueprints 候选目录。
// 使用：概览、蓝图扫描和后续蓝图同步流程调用。
// 边界：默认候选不存在时返回 needConfirm，不在本函数创建目录。
func (resolver Resolver) ResolveBlueprintRoot() RootResolution {
	if configured := resolver.configuredDir(ConfigKeyBlueprintDirPath); configured != "" {
		return RootResolution{
			Path:       configured,
			Source:     "configured",
			Confidence: "confirmed",
			Exists:     dirExists(configured),
			Message:    "已使用用户确认的蓝图目录",
		}
	}

	documentsDir := documentsRoot()
	if documentsDir == "" {
		return RootResolution{Source: "missing", Confidence: "missing", Message: "未找到用户文档目录"}
	}

	candidates := []string{
		filepath.Join(documentsDir, "Dyson Sphere Program", "Blueprint"),
		filepath.Join(documentsDir, "Dyson Sphere Program", "Blueprints"),
	}
	for _, candidate := range candidates {
		if dirExists(candidate) {
			return RootResolution{
				Path:       candidate,
				Source:     "detected",
				Confidence: "detected",
				Exists:     true,
				Message:    "已从用户文档目录探测到蓝图目录",
			}
		}
	}

	defaultPath := candidates[0]
	return RootResolution{
		Path:        defaultPath,
		Source:      "default",
		Confidence:  "guessed",
		NeedConfirm: true,
		Exists:      false,
		Message:     "仅推断默认蓝图目录，写入前需要确认",
	}
}

// ResolveSaveRoot 解析游戏真实存档目录。
// 职责：优先使用用户配置，其次从文档目录探测 Save/save 候选目录。
// 使用：概览、存档实时扫描和后续存档操作调用。
// 边界：目录不存在时返回缺失状态；不会创建存档目录。
func (resolver Resolver) ResolveSaveRoot() RootResolution {
	if configured := resolver.configuredDir(ConfigKeySaveDirPath); configured != "" {
		return RootResolution{
			Path:       configured,
			Source:     "configured",
			Confidence: "confirmed",
			Exists:     dirExists(configured),
			Message:    "已使用用户确认的存档目录",
		}
	}

	documentsDir := documentsRoot()
	if documentsDir == "" {
		return RootResolution{Source: "missing", Confidence: "missing", Message: "未找到用户文档目录"}
	}

	candidates := []string{
		filepath.Join(documentsDir, "Dyson Sphere Program", "Save"),
		filepath.Join(documentsDir, "Dyson Sphere Program", "save"),
	}
	for _, candidate := range candidates {
		if dirExists(candidate) {
			return RootResolution{
				Path:       candidate,
				Source:     "detected",
				Confidence: "detected",
				Exists:     true,
				Message:    "已从用户文档目录探测到存档目录",
			}
		}
	}

	return RootResolution{Source: "missing", Confidence: "missing", Message: "未发现存档目录"}
}

// configuredDir 读取并清理 Game 分组下的目录配置。
// 职责：把配置项文本转换为规范路径。
// 使用：蓝图和存档目录解析优先读取用户确认路径。
// 边界：配置服务不可用或值为空时返回空字符串，不记录错误。
func (resolver Resolver) configuredDir(key string) string {
	if core.GLOBAL.Configure == nil {
		return ""
	}

	value, err := core.InitConfigUnit(ConfigGroupGame, key).Get()
	if err != nil {
		return ""
	}

	return utils.CleanPath(value)
}

// documentsRoot 返回当前用户文档目录候选。
// 职责：在 Windows 便携环境中定位 Documents/My Documents 目录。
// 使用：蓝图和存档目录自动探测调用。
// 边界：只使用环境变量候选，不调用系统对话框，不创建目录。
func documentsRoot() string {
	candidates := []string{}
	userProfile := os.Getenv("USERPROFILE")
	if userProfile != "" {
		candidates = append(candidates,
			filepath.Join(userProfile, "Documents"),
			filepath.Join(userProfile, "My Documents"),
		)
	}
	homeDir, err := os.UserHomeDir()
	if err == nil && homeDir != "" {
		candidates = append(candidates,
			filepath.Join(homeDir, "Documents"),
			filepath.Join(homeDir, "My Documents"),
		)
	}

	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		candidate = utils.CleanPath(candidate)
		if candidate == "" {
			continue
		}
		key := strings.ToLower(candidate)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if dirExists(candidate) {
			return candidate
		}
	}

	return ""
}

// dirExists 判断路径是否存在且为目录。
// 职责：提供环境 helper 内部目录存在性判断。
// 使用：Resolver 和扫描器检测路径状态时调用。
// 边界：访问失败、空路径或普通文件均返回 false。
func dirExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// stableIDFromPath 基于相对路径生成稳定列表 ID。
// 职责：为只读扫描结果提供不会泄露绝对路径细节的本地 ID。
// 使用：资源库扫描和存档扫描生成前端列表项。
// 边界：只保证同根同相对路径稳定，不作为安全校验。
func stableIDFromPath(prefix string, relativePath string) string {
	sum := sha1.Sum([]byte(strings.ToLower(filepath.ToSlash(relativePath))))
	return prefix + "_" + hex.EncodeToString(sum[:])[:16]
}

// readManifestJSON 读取 manifest.json 的部分常用字段。
// 职责：从 Mod 目录中解析显示名、版本、作者和描述。
// 使用：Mod 资源库扫描时增强卡片展示。
// 边界：解析失败返回空 map，不阻断列表扫描。
func readManifestJSON(path string) map[string]interface{} {
	content, err := os.ReadFile(path)
	if err != nil {
		return map[string]interface{}{}
	}

	var data map[string]interface{}
	if err := json.Unmarshal(content, &data); err != nil {
		return map[string]interface{}{}
	}
	return data
}
