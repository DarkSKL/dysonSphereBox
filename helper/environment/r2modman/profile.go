package r2modman

// 文件职责：读取和校验 r2modman 的 Profile 清单，保留原始 YAML 节点供局部更新。
// 使用：检索、资源列表和操作计划阶段调用。
// 边界：不执行下载，不从显示名猜测文件归属，不改写未知清单字段。

import (
	environmentConstant "DysonSphereBox/constant/environment"
	"DysonSphereBox/helper/modDependence/fileops"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// 清单文档保存原文及可编辑节点；职责：连接展示与局部更新；使用：操作计划；边界：只在当前调用中有效。
type manifestDocument struct {
	// path 职责：记录已校验的 Profile 根；使用：包文件定位；边界：不是前端直接输入。
	path string
	// raw 职责：保留原始清单字节；使用：外部变更复核与备份；边界：不覆盖后续外部修改。
	raw []byte
	// node 职责：保留 YAML 未知字段及顺序；使用：局部更新清单；边界：只在当前操作内编辑。
	node yaml.Node
	// mods 职责：保存当前包状态；使用：依赖规划；边界：不作为永久缓存。
	mods []Mod
}

// 清单解析结构仅描述已有字段；职责：接收 R2 YAML 数据；使用：节点解码；边界：未知字段保留在原节点，不依赖此结构重写文档。
type manifestRecord struct {
	// Name 职责：读取完整包身份；使用：定位归属目录；边界：必须通过路径校验。
	Name string `yaml:"name"`
	// DisplayName 职责：读取显示名；使用：卡片；边界：为空时回退完整身份。
	DisplayName string `yaml:"displayName"`
	// Author 职责：读取作者名；使用：详情；边界：不参与路径拼接。
	Author string `yaml:"authorName"`
	// Description 职责：读取说明；使用：纯文本展示；边界：不执行富文本。
	Description string `yaml:"description"`
	// WebsiteURL 职责：读取网站地址；使用：详情；边界：不发起请求。
	WebsiteURL string `yaml:"websiteUrl"`
	// Dependencies 职责：读取依赖标识；使用：启停约束；边界：缺失依赖不自动下载。
	Dependencies []string `yaml:"dependencies"`
	// Enabled 职责：区分已声明的启用状态和无效缺失；使用：后缀规则校验；边界：缺失时拒绝修改。
	Enabled *bool `yaml:"enabled"`
	// Version 职责：读取 R2 三段版本对象；使用：展示和最低依赖版本检查；边界：不把它序列化回未知字段。
	Version struct {
		// Major 职责：读取主版本；使用：依赖比较；边界：不参与包目录名。
		Major int `yaml:"major"`
		// Minor 职责：读取次版本；使用：依赖比较；边界：遵循三段版本格式。
		Minor int `yaml:"minor"`
		// Patch 职责：读取补丁版本；使用：依赖比较；边界：不解析预发布标签。
		Patch int `yaml:"patch"`
	} `yaml:"versionNumber"`
}

// ValidateProfile 职责：规范化并限定戴森球计划 Profile 路径；使用：所有公开文件操作前；边界：只接受游戏 profiles 直接子目录及普通 mods.yml。
func ValidateProfile(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("未选择 r2modman Profile")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("Profile 目录不可用: %w", err)
	}
	if !strings.EqualFold(filepath.Base(filepath.Dir(abs)), "profiles") || !strings.EqualFold(filepath.Base(filepath.Dir(filepath.Dir(abs))), environmentConstant.R2GameDirectory) {
		return "", fmt.Errorf("请选择 DysonSphereProgram/profiles 下的 Profile 目录")
	}
	if _, err := safeProfilePath(abs, "mods.yml"); err != nil {
		return "", err
	}
	return abs, nil
}

// 职责：拒绝越界和链接路径；使用：读取清单或计划包文件时逐层检查；边界：允许末级尚不存在，用于重命名目标，父级链接一律拒绝。
func safeProfilePath(root string, relative string) (string, error) {
	target := filepath.Join(root, filepath.FromSlash(relative))
	if !fileops.IsSubPath(root, target) || target == root {
		return "", fmt.Errorf("Profile 路径越界: %s", relative)
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return "", err
	}
	current := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		resolved, err := filepath.EvalSymlinks(current)
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 || !strings.EqualFold(filepath.Clean(resolved), filepath.Clean(current)) {
			return "", fmt.Errorf("Profile 包含链接路径，不能直接管理: %s", relative)
		}
	}
	return target, nil
}

// 职责：解析清单并验证唯一包身份；使用：查询与写操作的共同入口；边界：错误 YAML 或重复记录会整体报错，防止部分重写。
func readManifest(path string) (*manifestDocument, error) {
	root, err := ValidateProfile(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(root, "mods.yml"))
	if err != nil {
		return nil, fmt.Errorf("读取 mods.yml 失败: %w", err)
	}
	doc := &manifestDocument{path: root, raw: data, mods: []Mod{}}
	if err := yaml.Unmarshal(data, &doc.node); err != nil {
		return nil, fmt.Errorf("解析 mods.yml 失败: %w", err)
	}
	if len(doc.node.Content) != 1 || doc.node.Content[0].Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("mods.yml 必须是 R2 模组列表")
	}
	seen := map[string]bool{}
	for _, node := range doc.node.Content[0].Content {
		var record manifestRecord
		if node.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("mods.yml 包记录格式无效")
		}
		if err := node.Decode(&record); err != nil {
			return nil, fmt.Errorf("解析模组记录失败: %w", err)
		}
		if !validPackageName(record.Name) || seen[strings.ToLower(record.Name)] || record.Enabled == nil {
			return nil, fmt.Errorf("mods.yml 含无效或重复模组: %s", record.Name)
		}
		seen[strings.ToLower(record.Name)] = true
		name := record.DisplayName
		if name == "" {
			name = record.Name
		}
		dependencies := record.Dependencies
		if dependencies == nil {
			dependencies = []string{}
		}
		doc.mods = append(doc.mods, Mod{Name: record.Name, DisplayName: name, Author: record.Author, Version: fmt.Sprintf("%d.%d.%d", record.Version.Major, record.Version.Minor, record.Version.Patch), Description: record.Description, WebsiteURL: record.WebsiteURL, Dependencies: dependencies, Enabled: *record.Enabled})
	}
	return doc, nil
}

// 职责：校验清单标识可作为单个目录名；使用：清单读取和操作定位；边界：不接受 Windows 特殊路径、分隔符或目录穿越。
func validPackageName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, `/\:<>"|?*`) && !strings.HasSuffix(name, ".") && strings.TrimSpace(name) == name
}

// ReadProfile 职责：读取 Profile 及文件可管理状态；使用：资源页与检索列表；边界：只读，不修复异常或自动切换状态。
func ReadProfile(path string) (ProfileState, error) {
	doc, err := readManifest(path)
	if err != nil {
		return ProfileState{}, err
	}
	profile := &Profile{Name: filepath.Base(doc.path), Path: doc.path, ModCount: len(doc.mods)}
	for index := range doc.mods {
		mod := &doc.mods[index]
		if mod.Enabled {
			profile.EnabledCount++
		}
		_, err := packageFiles(doc.path, *mod)
		mod.CanManage = err == nil
		if err != nil {
			mod.Warning = err.Error()
		}
	}
	updateModPermissions(doc.mods)
	return ProfileState{Profile: profile, Mods: doc.mods}, nil
}
