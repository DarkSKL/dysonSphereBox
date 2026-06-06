package model

import (
	"DysonSphereBox/model/modDependence"
	"DysonSphereBox/model/system"
)

var ModelGroupApp = new(ModelGroup)

type ModelGroup struct {
	SystemModelGroup        system.ModelGroup
	ModDependenceModelGroup modDependence.ModelGroup
}

func RegisterModels() []interface{} {
	models := make([]interface{}, 0)
	models = append(models, ModelGroupApp.SystemModelGroup.RegisterModels()...)
	models = append(models, ModelGroupApp.ModDependenceModelGroup.RegisterModels()...)

	return models
}
