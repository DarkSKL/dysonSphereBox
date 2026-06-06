package modDependence

type ModelGroup struct {
	ModDependency
	ModDependencyPackage
}

func (modelGroup *ModelGroup) RegisterModels() []interface{} {
	return []interface{}{
		&modelGroup.ModDependency,
		&modelGroup.ModDependencyPackage,
	}
}
