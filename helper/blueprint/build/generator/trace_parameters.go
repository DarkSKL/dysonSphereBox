package generator

// 文件职责：显式将生产工具的策略参数纳入旁路追踪白名单。
// 使用：注册目录组装；边界：仅用于已审查的配置参数结构，不从实例或请求反射取值。

import (
	"fmt"
	"reflect"
	"strings"

	"DysonSphereBox/helper/blueprint/build/workflow"
)

// exposeStrategyParameters 职责：为已知无请求实体的工具参数登记顶层字段；使用：当前生产工具目录；边界：未来加入含敏感对象的参数必须先重新审查调用点。
func exposeStrategyParameters(specs []workflow.ToolSpec) error {
	for index := range specs {
		spec := &specs[index]
		method, ok := reflect.TypeOf(spec.Receiver).MethodByName(spec.Method)
		if !ok || method.Type.NumIn() != 4 {
			return fmt.Errorf("invalid strategy tool signature: %s", spec.ID)
		}
		args := method.Type.In(3)
		if args.Kind() != reflect.Struct {
			return fmt.Errorf("strategy arguments must be a struct: %s", spec.ID)
		}
		names := []string{}
		for field := 0; field < args.NumField(); field++ {
			definition := args.Field(field)
			if !definition.IsExported() {
				continue
			}
			name := strings.Split(definition.Tag.Get("json"), ",")[0]
			if name == "-" {
				continue
			}
			if name == "" {
				name = definition.Name
			}
			names = append(names, name)
		}
		spec.TraceFields = names
	}
	return nil
}
