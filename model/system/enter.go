package system

type ModelGroup struct {
	Config
	Log
}

func (modelGroup *ModelGroup) RegisterModels() []interface{} {
	return []interface{}{
		&modelGroup.Config,
		&modelGroup.Log,
	}
}
