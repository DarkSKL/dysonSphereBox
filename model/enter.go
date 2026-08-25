package model

// 文件职责：定义数据库模型或模型注册模块 enter。
// 使用：由 core 自动迁移并由 service/helper 通过 GORM 读写。
// 边界：不包含远端接口 DTO、服务编排或文件系统操作。

import (
	"DysonSphereBox/model/account"
	"DysonSphereBox/model/environment"
	"DysonSphereBox/model/modDependence"
	"DysonSphereBox/model/system"
)

// ModelGroupApp 是模型注册聚合入口的全局实例。
// 职责：保存所有领域模型注册组的单例聚合实例。
// 使用：core/database.go 只通过 RegisterModels 获取需要迁移的模型集合。
// 边界：只用于模型注册，不保存业务数据或执行数据库操作。
var ModelGroupApp = new(ModelGroup)

// ModelGroup 聚合各业务域的 GORM 模型注册器。
// 职责：组合系统领域和 Mod 依赖领域模型组。
// 使用：RegisterModels 遍历各领域模型组时读取。
// 边界：这里只组合领域模型组，不直接声明数据库字段或执行迁移逻辑。
type ModelGroup struct {
	// AccountModelGroup 职责：聚合账号会话模型注册器；使用：RegisterModels 收集 SQLite 会话表；边界：不读取或写入账号会话。
	AccountModelGroup account.ModelGroup
	// SystemModelGroup 职责：聚合系统领域配置与日志模型注册器；使用：RegisterModels 收集系统模型时读取；边界：不执行系统配置读写。
	SystemModelGroup system.ModelGroup
	// ModDependenceModelGroup 职责：聚合 Mod 依赖领域模型注册器；使用：RegisterModels 收集安装、依赖和文件记录模型时读取；边界：不执行安装状态迁移逻辑。
	ModDependenceModelGroup modDependence.ModelGroup
	// EnvironmentModelGroup 职责：聚合环境管理领域模型注册器；使用：RegisterModels 收集资源项目、同步文件和存档索引模型；边界：不执行本机目录扫描。
	EnvironmentModelGroup environment.ModelGroup
}

// RegisterModels 返回当前应用需要 AutoMigrate 的全部模型。
// 职责：聚合所有领域需要迁移的 GORM 模型。
// 使用：数据库初始化时调用；新增领域模型组必须在这里显式追加。
// 边界：只返回模型指针列表，不执行 AutoMigrate，也不读写数据。
func RegisterModels() []interface{} {
	models := make([]interface{}, 0)
	models = append(models, ModelGroupApp.AccountModelGroup.RegisterModels()...)
	models = append(models, ModelGroupApp.SystemModelGroup.RegisterModels()...)
	models = append(models, ModelGroupApp.ModDependenceModelGroup.RegisterModels()...)
	models = append(models, ModelGroupApp.EnvironmentModelGroup.RegisterModels()...)

	return models
}
