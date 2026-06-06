package modDependence

type ServiceGroup struct {
	BepInExService
}

func (serviceGroup *ServiceGroup) RegisterServices() []interface{} {
	return []interface{}{
		&serviceGroup.BepInExService,
	}
}
