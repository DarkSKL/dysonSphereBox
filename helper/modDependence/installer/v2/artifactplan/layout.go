package artifactplan

// 文件职责：校验 artifact 解压目录是否符合可安装的 BepInEx Mod 布局。
// 使用：PlanFiles 扫描文件前调用，用于阻止框架包、桌面工具或无法识别的包进入自动复制。
// 边界：只做启发式布局判断，不保证 Mod 运行时兼容性、不修改文件、不写 manifest。
// 逻辑流：1. 遍历解压目录 => 2. 识别 BepInExPack、插件根、DLL 和可执行文件 => 3. 根据特征决定允许或返回阻断错误。

import (
	"DysonSphereBox/helper/modDependence/installer/artifactlayout"
	v2Artifact "DysonSphereBox/helper/modDependence/installer/v2/artifact"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// validateInstallLayout 校验 artifact 是否像可安装的 BepInEx Mod。
// 职责：阻止 BepInEx 框架包、桌面工具或没有插件/配置特征的包进入自动安装。
// 使用：PlanFiles 扫描文件前调用。
// 边界：只做启发式布局校验，不保证 Mod 运行时兼容性。
func validateInstallLayout(extractPath string, artifact v2Artifact.Artifact) error {
	hasBepInExPack := false
	hasSpecialBepInExRoot := false
	hasAnyDLL := false
	hasAnyExecutable := false

	err := filepath.WalkDir(extractPath, func(sourcePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}

		relativePath, err := filepath.Rel(extractPath, sourcePath)
		if err != nil {
			return err
		}
		normalizedRelativePath := artifactlayout.NormalizeEntryPath(relativePath)
		lowerRelativePath := strings.ToLower(normalizedRelativePath)
		if strings.HasPrefix(lowerRelativePath, "bepinexpack/") {
			hasBepInExPack = true
		}
		_, _, hasRoot, _ := artifactlayout.SplitInstallRoot(normalizedRelativePath)
		if hasRoot {
			hasSpecialBepInExRoot = true
		}

		extension := strings.ToLower(filepath.Ext(lowerRelativePath))
		if extension == ".dll" {
			hasAnyDLL = true
		}
		if extension == ".exe" || extension == ".msi" {
			hasAnyExecutable = true
		}
		return nil
	})
	if err != nil {
		return err
	}

	if hasBepInExPack {
		return fmt.Errorf("%s 是 BepInEx 框架包，应通过 BepInEx 安装器处理，不能作为普通 Mod 安装", artifact.DisplayName)
	}
	if hasSpecialBepInExRoot || hasAnyDLL {
		return nil
	}
	if hasAnyExecutable {
		return fmt.Errorf("%s 看起来是桌面工具或独立安装器，暂不支持自动安装到游戏 Mod 目录", artifact.DisplayName)
	}

	return fmt.Errorf("%s 未识别到 BepInEx 插件、patcher 或配置文件，已阻止自动安装", artifact.DisplayName)
}
