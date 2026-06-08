package system

type ServiceGroup struct {
	EnvironmentService
	ConfigurationService
	UpdateService
}

func (serviceGroup *ServiceGroup) RegisterServices() []interface{} {
	return []interface{}{
		&serviceGroup.EnvironmentService,
		&serviceGroup.ConfigurationService,
		&serviceGroup.UpdateService,
	}
}
