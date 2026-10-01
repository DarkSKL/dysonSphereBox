package artifactplan

// 文件职责：校验 artifact 解压目录是否可以作为普通 Mod 安装。
// 使用：PlanFiles 扫描文件前调用，用于阻止 BepInEx 框架包进入普通 Mod 复制流程。
// 边界：与 r2modman 一致，不再按 DLL、配置等特征做启发式拦截，任意文件都按安装规则落位；不修改文件、不写 manifest。
// 逻辑流：1. 遍历解压目录 => 2. 识别 BepInExPack 根目录 => 3. 命中则返回阻断错误。

import (
	"DysonSphereBox/helper/modDependence/installer/artifactlayout"
	v2Artifact "DysonSphereBox/helper/modDependence/installer/v2/artifact"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// errBepInExPackFound 标记遍历中发现框架包目录，用于提前结束遍历。
var errBepInExPackFound = errors.New("bepinex pack found")

// validateInstallLayout 校验 artifact 不是 BepInEx 框架包。
// 职责：阻止 BepInExPack 被当作普通 Mod 展平复制到 plugins。
// 使用：PlanFiles 扫描文件前调用。
// 边界：框架包由 BepInEx 安装器处理（对应 R2 的 mod loader 包规则）；其余布局一律放行。
func validateInstallLayout(extractPath string, artifact v2Artifact.Artifact) error {
	err := filepath.WalkDir(extractPath, func(sourcePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relativePath, err := filepath.Rel(extractPath, sourcePath)
		if err != nil {
			return err
		}
		root, _ := artifactlayout.SplitFirstPathSegment(relativePath)
		if strings.EqualFold(root, "BepInExPack") || (entry.IsDir() && strings.EqualFold(entry.Name(), "BepInExPack")) {
			return errBepInExPackFound
		}
		return nil
	})
	if errors.Is(err, errBepInExPackFound) {
		return fmt.Errorf("%s 是 BepInEx 框架包，应通过 BepInEx 安装器处理，不能作为普通 Mod 安装", artifact.DisplayName)
	}
	return err
}
