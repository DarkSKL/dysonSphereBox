package system

type ServiceGroup struct {
	EnvironmentService
	ConfigurationService
	UpdateService
	LogService
}

func (serviceGroup *ServiceGroup) RegisterServices() []interface{} {
	return []interface{}{
		&serviceGroup.EnvironmentService,
		&serviceGroup.ConfigurationService,
		&serviceGroup.UpdateService,
		&serviceGroup.LogService,
	}
}
