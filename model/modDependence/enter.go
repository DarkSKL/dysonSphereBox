package modDependence

type ModelGroup struct {
	ModDependency
	ModDependencyPackage
	ModInstalledPackage
	ModInstalledComponent
	ModInstalledPackageComponent
	ModInstalledFile
	ModRemotePackageCache
}

func (modelGroup *ModelGroup) RegisterModels() []interface{} {
	return []interface{}{
		&modelGroup.ModDependency,
		&modelGroup.ModDependencyPackage,
		&modelGroup.ModInstalledPackage,
		&modelGroup.ModInstalledComponent,
		&modelGroup.ModInstalledPackageComponent,
		&modelGroup.ModInstalledFile,
		&modelGroup.ModRemotePackageCache,
	}
}
