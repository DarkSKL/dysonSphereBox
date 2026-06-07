package modDependence

type ServiceGroup struct {
	BepInExService
	InstallerService
	MarketplaceService
}

func (serviceGroup *ServiceGroup) RegisterServices() []interface{} {
	return []interface{}{
		&serviceGroup.BepInExService,
		&serviceGroup.InstallerService,
		&serviceGroup.MarketplaceService,
	}
}
