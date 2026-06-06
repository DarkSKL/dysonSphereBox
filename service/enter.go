package service

import (
	"DysonSphereBox/service/modDependence"
	"DysonSphereBox/service/system"
)

var ServiceGroupApp = new(ServiceGroup)

type ServiceGroup struct {
	SystemServiceGroup        system.ServiceGroup
	ModDependenceServiceGroup modDependence.ServiceGroup
}

func RegisterServices() []interface{} {
	services := make([]interface{}, 0)
	services = append(services,
		ServiceGroupApp.SystemServiceGroup.RegisterServices()...)
	services = append(services, ServiceGroupApp.ModDependenceServiceGroup.RegisterServices()...)

	return services
}
