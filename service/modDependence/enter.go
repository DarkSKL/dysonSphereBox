package modDependence

type ServiceGroup struct {
	BepInExService
	MarketplaceService
}

func (serviceGroup *ServiceGroup) RegisterServices() []interface{} {
	return []interface{}{
		&serviceGroup.BepInExService,
		&serviceGroup.MarketplaceService,
	}
}
