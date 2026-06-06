package system

type ServiceGroup struct {
	EnvironmentService
	ConfigurationService
}

func (serviceGroup *ServiceGroup) RegisterServices() []interface{} {
	return []interface{}{
		&serviceGroup.EnvironmentService,
		&serviceGroup.ConfigurationService,
	}
}
