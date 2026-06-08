package modDependence

type ServiceGroup struct {
	BepInExService
	HeadlessService
	InstallerService
	MarketplaceService
}

func (serviceGroup *ServiceGroup) RegisterServices() []interface{} {
	return []interface{}{
		&serviceGroup.BepInExService,
		&serviceGroup.HeadlessService,
		&serviceGroup.InstallerService,
		&serviceGroup.MarketplaceService,
	}
}
