package system

type ModelGroup struct {
	Config
}

func (modelGroup *ModelGroup) RegisterModels() []interface{} {
	return []interface{}{
		&modelGroup.Config,
	}
}
