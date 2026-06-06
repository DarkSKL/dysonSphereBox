package service

import "DysonSphereBox/service/system"

var ServiceGroupApp = new(ServiceGroup)

type ServiceGroup struct {
	SystemServiceGroup system.ServiceGroup
}

func RegisterServices() []interface{} {
	services := make([]interface{}, 0)
	services = append(services, ServiceGroupApp.SystemServiceGroup.RegisterServices()...)

	return services
}
