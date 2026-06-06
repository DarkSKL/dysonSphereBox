package system

type ServiceGroup struct {
	EnvironmentService
}

func (serviceGroup *ServiceGroup) RegisterServices() []interface{} {
	return []interface{}{
		&serviceGroup.EnvironmentService,
	}
}
