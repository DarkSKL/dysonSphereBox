package generator

// 文件职责：把固定参数覆盖层绑定为无业务搜索的配置工具。
// 使用：模式YAML按四层顺序提交；边界：不调用其他工具、不调度候选、不读取或修改实体。

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"DysonSphereBox/helper/blueprint/build/workflow"
)

// modeTools 职责：固定一个层的前后产物权限；使用：只读注册receiver；边界：零input仅供公共初始层，不保存任务状态。
type modeTools struct {
	// input 职责：限定前层只读键；使用：固定层receiver；边界：公共层不读取零键。
	input workflow.ArtifactKey[modeValues]
	// output 职责：限定本层产物写入；使用：固定层receiver；边界：不写入实体。
	output workflow.ArtifactKey[modeValues]
	// layer 职责：固定来源标记；使用：注册时选择；边界：配置不能任意改变。
	layer string
}

// Configure 职责：合并一层显式参数并输出可追踪事实；使用：四个稳定工具ID；边界：类型或语义错误致命，不作为候选无解回退。
func (t *modeTools) Configure(_ context.Context, s *workflow.StepContext, patch modePatch) error {
	values := initialModeValues()
	if t.layer != "common" {
		previous, err := workflow.Read(s, t.input)
		if err != nil {
			return err
		}
		values = previous
	}
	values, err := mergeModeValues(values, patch, t.layer)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return err
	}
	if err := s.Diagnostic("mode-parameters:" + string(raw)); err != nil {
		return err
	}
	return workflow.Stage(s, t.output, values)
}

// modeToolSpecs 职责：注册唯一类型合并实现及固定四层；使用：主registry追加；边界：每个receiver仅授权对应前后键，静态参数也经过相同语义验证。
func modeToolSpecs(a artifacts, implementation string) []workflow.ToolSpec {
	stages := a.modeStages
	specs := []workflow.ToolSpec{}
	for _, layer := range []struct {
		id, name string
		input,   // output 职责：限定本层产物写入；使用：固定层receiver；边界：不写入实体。
		output workflow.ArtifactKey[modeValues]
	}{
		{"mode.defaults@1", "common", workflow.ArtifactKey[modeValues]{}, stages.common},
		{"mode.configure@1", "mode", stages.common, stages.mode},
		{"mode.submode@1", "submode", stages.mode, stages.submode},
		{"mode.geography@1", "geography", stages.submode, stages.effective},
	} {
		reads := []workflow.Artifact{}
		if layer.name != "common" {
			reads = append(reads, layer.input.Ref())
		}
		writes := []workflow.Artifact{layer.output.Ref()}
		specs = append(specs, workflow.ToolSpec{ID: layer.id, Receiver: &modeTools{input: layer.input, output: layer.output, layer: layer.name}, Method: "Configure", Implementation: implementation, Reads: reads, Requires: reads, Writes: writes, Provides: writes, Role: workflow.Diagnostic,
			Validate: workflow.ArgumentValidator(func(patch modePatch) error {
				_, err := mergeModeValues(initialModeValues(), patch, "validation")
				return err
			}),
		})
	}
	return specs
}

// modeReadRequirements 职责：给共享枚举器追加受限策略读取能力；使用：主registry对白名单四个工具调用；边界：不授予实体写权限，其他工具无须读取此键。
func modeReadRequirements(a artifacts, spec *workflow.ToolSpec) {
	switch spec.ID {
	case "surface.enumerate@1", "connected.enumerate@1", "station.enumerate@1", "bus.enumerate@1":
		spec.Reads = append(spec.Reads, a.modeStages.effective.Ref())
		spec.Requires = append(spec.Requires, a.modeStages.effective.Ref())
	}
}

// modeProvider 职责：从有效配置键读取一个明确注册的字段；使用：枚举工具参数引用；边界：不接受任意字段路径，错误不降级为默认。
func modeProvider[T any](name string, key workflow.ArtifactKey[modeValues], get func(modePatch) T) workflow.StateProvider {
	return workflow.StateProvider{Name: name, Type: reflect.TypeFor[T](), Requires: []workflow.Artifact{key.Ref()}, Resolve: func(_ context.Context, s *workflow.StepContext) (any, error) {
		values, err := workflow.Read(s, key)
		if err != nil {
			return nil, err
		}
		if values.Patch.StationFamily == nil || values.Patch.BusTighten == nil {
			return nil, fmt.Errorf("模式参数未完整初始化")
		}
		return get(values.Patch), nil
	}}
}
