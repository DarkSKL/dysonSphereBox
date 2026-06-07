package modDependence

import modDependenceHelper "DysonSphereBox/helper/modDependence"

type BepInExService struct {
}

func (service *BepInExService) Install() (*modDependenceHelper.BepInExInstallResult, error) {
	return modDependenceHelper.InstallBepInEx()
}

func (service *BepInExService) Uninstall() (*modDependenceHelper.BepInExUninstallResult, error) {
	return modDependenceHelper.UninstallBepInEx()
}

func (service *BepInExService) Update() (*modDependenceHelper.BepInExInstallResult, error) {
	status, err := service.GetStatus()
	if err != nil {
		return nil, err
	}
	if status.Installed {
		return &modDependenceHelper.BepInExInstallResult{
			GameDir:   status.GameDir,
			Installed: false,
			Skipped:   true,
			Message:   "BepInEx 已安装，跳过重复安装",
			Status:    *status,
		}, nil
	}

	return modDependenceHelper.InstallBepInEx()
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

	gameDir, err := modDependenceHelper.GetConfiguredGameDir()
	if err != nil {
		return nil, err
	}

	status := modDependenceHelper.CheckBepInExStatus(gameDir)
	return &status, nil
}
