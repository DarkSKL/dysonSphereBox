package generator

// 文件职责：显式装配生产工具白名单、参数默认和稳定属性。
// 使用：生成器初始化；边界：不按方法名自动暴露能力，不保存任务状态，配置不能覆盖设备物理事实。

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	core "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/workflow"
)

// parameterValue 职责：按真实参数标签生成无损默认描述；使用：注册时一次转换；边界：只处理已知配置值，整数不经float64。
func parameterValue(v reflect.Value) any {
	switch v.Kind() {
	case reflect.Struct:
		out := map[string]any{}
		for i := 0; i < v.NumField(); i++ {
			f := v.Type().Field(i)
			if !f.IsExported() {
				continue
			}
			name := f.Name
			for _, tag := range []string{"json", "yaml"} {
				if n := strings.Split(f.Tag.Get(tag), ",")[0]; n != "" {
					name = n
					break
				}
			}
			if name != "-" {
				out[name] = parameterValue(v.Field(i))
			}
		}
		return out
	case reflect.Slice, reflect.Array:
		out := make([]any, v.Len())
		for i := range out {
			out[i] = parameterValue(v.Index(i))
		}
		return out
	case reflect.Bool:
		return v.Bool()
	case reflect.String:
		return v.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint()
	case reflect.Float32, reflect.Float64:
		return v.Float()
	default:
		panic("unsupported internal default type: " + v.Type().String())
	}
}

// defaults 职责：转换已知强类型参数默认值；使用：注册装配；边界：任何新字段类型须先由参数编译器支持。
func defaults(v any) map[string]any { return parameterValue(reflect.ValueOf(v)).(map[string]any) }

// candidateIndex 职责：把当前规格表下标交给引擎；使用：有界items；边界：具体可解释策略仍保存在强类型产物中。
type candidateIndex struct {
	Index int `json:"index"`
}

// indexProvider 职责：为强类型规格表生成稳定下标；使用：登记属性目录；边界：只能读取注册的当前作用域产物。
func indexProvider[T any](name string, key workflow.ArtifactKey[[]T]) workflow.StateProvider {
	return workflow.StateProvider{Name: name, Type: reflect.TypeFor[[]candidateIndex](), Requires: []workflow.Artifact{key.Ref()}, Resolve: func(_ context.Context, s *workflow.StepContext) (any, error) {
		values, err := workflow.Read(s, key)
		if err != nil {
			return nil, err
		}
		indices := make([]candidateIndex, len(values))
		for index := range indices {
			indices[index].Index = index
		}
		return indices, nil
	}}
}

// inputProvider 职责：公开明确的只读领域属性；使用：流程选择；边界：不能通过任意Go字段路径读取实例。
func inputProvider[T any](name string, key workflow.ArtifactKey[*core.PreparedInput], get func(*core.PreparedInput) (T, error)) workflow.StateProvider {
	return workflow.StateProvider{Name: name, Type: reflect.TypeFor[T](), Requires: []workflow.Artifact{key.Ref()}, Resolve: func(_ context.Context, s *workflow.StepContext) (any, error) {
		v, err := workflow.Read(s, key)
		if err != nil {
			return nil, err
		}
		return get(v)
	}}
}

// newRegistry 职责：把稳定业务ID绑定到明确Go入口；使用：生产配置编译；边界：签名、权限、默认值任一非法都拒绝整个目录。
func newRegistry(a artifacts) (*workflow.Registry, error) {
	implementation, err := implementationIdentity()
	if err != nil {
		return nil, err
	}
	t := &toolset{keys: a}
	spec := func(id, method string, reads, writes []workflow.Artifact, args any, role workflow.Role) workflow.ToolSpec {
		values := defaults(args)
		if _, indexed := args.(indexArgs); indexed {
			values = nil
		}
		if _, trial := args.(refinementSpecArgs); trial {
			values = nil
		}
		return workflow.ToolSpec{ID: id, Receiver: t, Method: method, Implementation: implementation, Reads: reads, Requires: reads, Writes: writes, Provides: writes, Defaults: values, FailureCodes: solutionFailures, Role: role}
	}
	one := func(v workflow.Artifact) []workflow.Artifact { return []workflow.Artifact{v} }
	specs := []workflow.ToolSpec{
		spec("transport.enumerate@1", "PlanTransport", one(a.base.Ref()), one(a.transportSpecs.Ref()), core.TransportSearchOptions{}, workflow.Transform),
		spec("surface.enumerate@1", "PlanSurface", one(a.transport.Ref()), one(a.surfaceSpecs.Ref()), core.DefaultSurfaceSearchOptions(), workflow.Transform),
		spec("surface.prepare@1", "ApplySurface", []workflow.Artifact{a.transport.Ref(), a.surfaceSpecs.Ref()}, one(a.input.Ref()), indexArgs{}, workflow.Transform),
		spec("connected.enumerate@1", "PlanConnected", one(a.input.Ref()), one(a.connected.Ref()), core.DefaultConnectedSearchOptions(), workflow.Transform),
		spec("connected.assemble@1", "AssembleConnected", []workflow.Artifact{a.input.Ref(), a.connected.Ref()}, []workflow.Artifact{a.choice.Ref(), a.geometryStages.assembly.Ref()}, indexArgs{}, workflow.Transform),
		spec("station.enumerate@1", "PlanStation", one(a.input.Ref()), one(a.station.Ref()), core.DefaultStationSearchOptions(), workflow.Transform),
		spec("station.assemble@1", "AssembleStation", []workflow.Artifact{a.input.Ref(), a.station.Ref()}, []workflow.Artifact{a.choice.Ref(), a.geometryStages.assembly.Ref()}, indexArgs{}, workflow.Transform),
		spec("distributor.enumerate@1", "PlanDistributor", one(a.input.Ref()), one(a.distributor.Ref()), core.DefaultDistributorSearchOptions(), workflow.Transform),
		spec("distributor.assemble@1", "AssembleDistributor", []workflow.Artifact{a.input.Ref(), a.distributor.Ref()}, []workflow.Artifact{a.choice.Ref(), a.geometryStages.assembly.Ref()}, indexArgs{}, workflow.Transform),
		spec("bus.enumerate@1", "PlanBus", one(a.input.Ref()), one(a.bus.Ref()), busArgs{Tighten: true}, workflow.Transform),
		spec("bus.assemble@1", "AssembleBus", []workflow.Artifact{a.input.Ref(), a.bus.Ref()}, []workflow.Artifact{a.choice.Ref(), a.geometryStages.assembly.Ref()}, indexArgs{}, workflow.Transform),
		spec("station.prepare-refinement@1", "PrepareRefinement", []workflow.Artifact{a.verified.Ref(), a.choice.Ref()}, one(a.refinement.Ref()), refinementArgs{Enabled: true}, workflow.Diagnostic),
		spec("station.assemble-spec@1", "AssembleStationSpec", one(a.input.Ref()), []workflow.Artifact{a.choice.Ref(), a.geometryStages.assembly.Ref()}, refinementSpecArgs{}, workflow.Transform),
		spec("factory.select@1", "SelectValidated", []workflow.Artifact{a.verified.Ref(), a.choice.Ref()}, one(a.selected.Ref()), emptyArgs{}, workflow.Validate),
		spec("factory.check-shape@1", "CheckShape", one(a.selected.Ref()), nil, emptyArgs{}, workflow.Validate),
		spec("factory.check-candidate-shape@1", "CheckCandidateShape", one(a.verified.Ref()), nil, emptyArgs{}, workflow.Validate),
		spec("geometry.describe-regions@1", "DescribeRegions", one(a.selected.Ref()), one(a.regions.Ref()), emptyArgs{}, workflow.Diagnostic),
		spec("geometry.check-region@1", "CheckRegion", one(a.selected.Ref()), one(a.regionReport.Ref()), core.RegionCheckOptions{}, workflow.Diagnostic),
		spec("factory.validate-regions@1", "ValidateRegions", one(a.selected.Ref()), one(a.approved.Ref()), emptyArgs{}, workflow.Validate),
		spec("blueprint.encode@1", "Encode", one(a.approved.Ref()), one(a.encoded.Ref()), emptyArgs{}, workflow.Encode),
	}
	specs = append(specs, geometryToolSpecs(a, implementation)...)
	specs = append(specs, inputToolSpecs(a, implementation)...)
	specs = append(specs, ruleToolSpecs(a, implementation)...)
	specs = append(specs, resultToolSpecs(a, implementation)...)
	specs = append(specs, modeToolSpecs(a, implementation)...)
	specs = append(specs, candidateWindowSpecs(a, implementation)...)
	validators := map[string]func(any) error{
		"connected.enumerate@1":   workflow.ArgumentValidator(core.ValidateConnectedSearchOptions),
		"distributor.enumerate@1": workflow.ArgumentValidator(core.ValidateDistributorSearchOptions),
		"station.enumerate@1":     workflow.ArgumentValidator(core.ValidateStationSearchOptions),
		"surface.enumerate@1":     workflow.ArgumentValidator(core.ValidateSurfaceSearchOptions),
	}
	for index := range specs {
		modeReadRequirements(a, &specs[index])
		if validate, exists := validators[specs[index].ID]; exists {
			specs[index].Validate = validate
		}
		if specs[index].ID == "factory.validate-regions@1" {
			specs[index].AllRegions = []workflow.Artifact{a.regionReport.Ref()}
			specs[index].Requires = append(specs[index].Requires, a.regionReport.Ref())
			specs[index].Consumes = []workflow.Artifact{a.selected.Ref(), a.regionReport.Ref()}
		}
		if specs[index].ID == "factory.select@1" {
			specs[index].Consumes = []workflow.Artifact{a.verified.Ref()}
		}
		if specs[index].ID == "blueprint.encode@1" {
			specs[index].Consumes = []workflow.Artifact{a.approved.Ref()}
		}
	}
	providers := []workflow.StateProvider{
		candidatePassProvider(a.searchPasses),
		distributorGroupProvider(a.distributor),
		refinementProvider(a),
		indexProvider("transport.candidates", a.transportSpecs), indexProvider("surface.candidates", a.surfaceSpecs), indexProvider("connected.candidates", a.connected), indexProvider("station.candidates", a.station), indexProvider("distributor.candidates", a.distributor), indexProvider("bus.candidates", a.bus),
		inputProvider("request.family", a.input, func(v *core.PreparedInput) (string, error) {
			switch v.Mode() {
			case 1:
				return "connected", nil
			case 2, 5:
				return "distributor", nil
			case 3, 4:
				return "station", nil
			case 6:
				return "bus", nil
			}
			return "", fmt.Errorf("unregistered generation mode")
		}),
		inputProvider("surface.class", a.transport, func(v *core.PreparedInput) (string, error) { return v.SurfaceClass(), nil }),
		inputProvider("surface.strategy", a.transport, func(v *core.PreparedInput) (string, error) {
			req, err := v.RequestSnapshot()
			if err != nil {
				return "", err
			}
			if req.SurfaceLayout != nil && req.SurfaceLayout.LatitudeZone != "" {
				if req.SurfaceLayout.ExpandAdjacent {
					return "adjacent", nil
				}
				if v.RequiresNativeSurface() {
					return "native", nil
				}
				return "adaptive", nil
			}
			return "strict", nil
		}),
	}
	providers = append(providers, regionProviders(a)...)
	providers = append(providers, modeProviders(a)...)
	if err := exposeStrategyParameters(specs); err != nil {
		return nil, err
	}
	return workflow.NewRegistry(specs, providers)
}
