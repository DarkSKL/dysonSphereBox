package modDependence

import modDependenceHelper "DysonSphereBox/helper/modDependence"

type BepInExService struct {
}

func (service *BepInExService) Install() (*modDependenceHelper.BepInExInstallResult, error) {
	result, err := modDependenceHelper.InstallBepInEx()
	if err != nil {
		modDependenceHelper.RecordBepInExFailure("install", "BepInEx 安装失败", err)
		return nil, err
	}

	modDependenceHelper.RecordBepInExInstallResult("install", result)
	return result, nil
}

func (service *BepInExService) Uninstall() (*modDependenceHelper.BepInExUninstallResult, error) {
	result, err := modDependenceHelper.UninstallBepInEx()
	if err != nil {
		modDependenceHelper.RecordBepInExFailure("uninstall", "BepInEx 卸载失败", err)
		return nil, err
	}

	modDependenceHelper.RecordBepInExUninstallSuccess(result)
	return result, nil
}

func (service *BepInExService) Update() (*modDependenceHelper.BepInExInstallResult, error) {
	status, err := service.GetStatus()
	if err != nil {
		return nil, err
	}
	if status.Installed {
		result := &modDependenceHelper.BepInExInstallResult{
			GameDir:   status.GameDir,
			Installed: false,
			Skipped:   true,
			Message:   "BepInEx 已安装，跳过重复安装",
			Status:    *status,
		}
		modDependenceHelper.RecordBepInExInstallResult("update", result)
		return result, nil
	}

	result, err := modDependenceHelper.InstallBepInEx()
	if err != nil {
		modDependenceHelper.RecordBepInExFailure("update", "BepInEx 更新失败", err)
		return nil, err
	}

	modDependenceHelper.RecordBepInExInstallResult("update", result)
	return result, nil
}

func (service *BepInExService) GetLatestVersion() (*modDependenceHelper.BepInExPackage, error) {
	if _, err := modDependenceHelper.GetConfiguredGameDir(); err != nil {
		return nil, err
	}

	return modDependenceHelper.ResolveLatestBepInExPackage()
}

func (service *BepInExService) GetStatus() (*modDependenceHelper.BepInExStatus, error) {
	gameDir, err := modDependenceHelper.GetConfiguredGameDir()
	if err != nil {
		return nil, err
	}

	status := modDependenceHelper.CheckBepInExStatus(gameDir)
	return &status, nil
}

func (service *BepInExService) SearchAndLoadBepInEx(selectedPath string) (*modDependenceHelper.BepInExStatus, error) {
	if selectedPath != "" {
		return modDependenceHelper.SearchAndLoadBepInEx(selectedPath)
	}

	return service.GetStatus()
}
