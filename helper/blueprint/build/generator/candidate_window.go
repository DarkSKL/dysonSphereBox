package generator

// 文件职责：为相邻纬带公平尝试提供显式候选窗口工具，主布局与剩余列表不重复执行。
// 使用：YAML先设置本轮窗口，再对已规范去重的各族列表应用；边界：不运行算法、不增加预算、不改候选内容或物理规则。
import (
	"context"
	"fmt"
	"reflect"

	core "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/workflow"
)

// candidateWindowArgs 职责：描述稳定候选列表的半开窗口；使用：显式工作流轮次；边界：limit0表示余下全部，非法负值拒绝。
type candidateWindowArgs struct {
	Start int `json:"start"`
	Limit int `json:"limit"`
}

// candidateWindowConfig 职责：发布本轮范围；使用：factory默认和expanded主布局/剩余两轮；边界：作用域由工作流隔离。
type candidateWindowConfig struct {
	key workflow.ArtifactKey[candidateWindowArgs]
}

// candidatePassArgs 职责：给出第一轮每表面要比较的主布局数；使用：YAML调节探索宽度；边界：0明确关闭分轮，不放宽任何计算预算。
type candidatePassArgs struct {
	Primary int `json:"primary"`
}

// validateCandidatePass 职责：在配置编译时拒绝非法主布局数；使用：反射目录和运行入口共用；边界：不运行任何候选。
func validateCandidatePass(args candidatePassArgs) error {
	return validateCandidateWindow(candidateWindowArgs{Limit: args.Primary})
}

// candidatePassPlanner 职责：将列表能力与显式主布局数转换为有限轮次；使用：扩带搜索；边界：有条件依赖的配送组保持一次完整调用。
type candidatePassPlanner struct {
	input  workflow.ArtifactKey[*core.PreparedInput]
	output workflow.ArtifactKey[[]candidateWindowArgs]
}

// Plan 职责：仅构造轮次数据，实际调度仍由YAML和引擎拥有；使用：表面枚举后；边界：两个窗口不相交且并集为原列表。
func (tool *candidatePassPlanner) Plan(ctx context.Context, step *workflow.StepContext, args candidatePassArgs) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateCandidatePass(args); err != nil {
		return err
	}
	input, err := workflow.Read(step, tool.input)
	if err != nil {
		return err
	}
	passes := []candidateWindowArgs{{}}
	if args.Primary > 0 && input.Mode() != 2 && input.Mode() != 5 {
		passes = []candidateWindowArgs{{Limit: args.Primary}, {Start: args.Primary}}
	}
	return workflow.Stage(step, tool.output, passes)
}

// candidatePassProvider 职责：公开已规划的强类型有限轮次；使用：candidates.items；边界：只读当前实例，不枚举或执行布局。
func candidatePassProvider(key workflow.ArtifactKey[[]candidateWindowArgs]) workflow.StateProvider {
	return workflow.StateProvider{Name: "search.surface-passes", Type: reflect.TypeFor[[]candidateWindowArgs](), Requires: []workflow.Artifact{key.Ref()}, Resolve: func(_ context.Context, step *workflow.StepContext) (any, error) { return workflow.Read(step, key) }}
}

// Configure 职责：固定当前候选轮次；使用：反射工具入口；边界：不隐式控制执行顺序。
func (tool *candidateWindowConfig) Configure(ctx context.Context, step *workflow.StepContext, args candidateWindowArgs) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateCandidateWindow(args); err != nil {
		return err
	}
	return workflow.Stage(step, tool.key, args)
}

// validateCandidateWindow 职责：校验范围参数；使用：编译与运行共用；边界：上界与工作流最大候选表一致，不把0当缺失。
func validateCandidateWindow(args candidateWindowArgs) error {
	if args.Start < 0 || args.Limit < 0 || args.Start > 4096 || args.Limit > 4096 {
		return fmt.Errorf("candidate window outside 0..4096")
	}
	return nil
}

// candidateWindowTool 职责：对固定类型列表复用唯一切片逻辑；使用：直连/塔/总线；边界：有条件依赖的配送组保持原完整搜索。
type candidateWindowTool[T any] struct {
	list   workflow.ArtifactKey[[]T]
	window workflow.ArtifactKey[candidateWindowArgs]
}

// Window 职责：保留本轮明确范围并继承原列表依赖；使用：枚举后、派发前；边界：每次读取独立副本，Stage再次隔离，未选项由后续轮次覆盖。
func (tool *candidateWindowTool[T]) Window(ctx context.Context, step *workflow.StepContext, _ emptyArgs) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	args, err := workflow.Read(step, tool.window)
	if err != nil {
		return err
	}
	if err := validateCandidateWindow(args); err != nil {
		return err
	}
	values, err := workflow.Read(step, tool.list)
	if err != nil {
		return err
	}
	start := min(args.Start, len(values))
	end := len(values)
	if args.Limit > 0 {
		end = min(end, start+args.Limit)
	}
	if err := step.Diagnostic(fmt.Sprintf("candidate-window:start=%d limit=%d total=%d selected=%d", args.Start, args.Limit, len(values), end-start)); err != nil {
		return err
	}
	return workflow.Stage(step, tool.list, values[start:end])
}

// candidateWindowSpec 职责：为每个列表绑定最小读写权限；使用：目录初始化；边界：同键变换由引擎保留来源依赖。
func candidateWindowSpec[T any](id string, key workflow.ArtifactKey[[]T], window workflow.ArtifactKey[candidateWindowArgs], implementation string) workflow.ToolSpec {
	return workflow.ToolSpec{ID: id, Receiver: &candidateWindowTool[T]{key, window}, Method: "Window", Implementation: implementation, Role: workflow.Transform,
		Reads: []workflow.Artifact{key.Ref(), window.Ref()}, Requires: []workflow.Artifact{key.Ref(), window.Ref()}, Writes: []workflow.Artifact{key.Ref()}, Provides: []workflow.Artifact{key.Ref()}}
}

// candidateWindowSpecs 职责：注册窗口配置及共享泛型工具；使用：newRegistry；边界：配送组不裁掉失败依赖，默认窗口保持原候选集合。
func candidateWindowSpecs(a artifacts, implementation string) []workflow.ToolSpec {
	return []workflow.ToolSpec{
		{ID: "search.plan-passes@1", Receiver: &candidatePassPlanner{a.transport, a.searchPasses}, Method: "Plan", Implementation: implementation, Role: workflow.Transform,
			Reads: []workflow.Artifact{a.transport.Ref()}, Requires: []workflow.Artifact{a.transport.Ref()}, Writes: []workflow.Artifact{a.searchPasses.Ref()}, Provides: []workflow.Artifact{a.searchPasses.Ref()}, Defaults: defaults(candidatePassArgs{Primary: 1}), Validate: workflow.ArgumentValidator(validateCandidatePass)},
		{ID: "search.window@1", Receiver: &candidateWindowConfig{a.searchWindow}, Method: "Configure", Implementation: implementation, Role: workflow.Transform,
			Writes: []workflow.Artifact{a.searchWindow.Ref()}, Provides: []workflow.Artifact{a.searchWindow.Ref()}, Defaults: defaults(candidateWindowArgs{}), Validate: workflow.ArgumentValidator(validateCandidateWindow)},
		candidateWindowSpec("connected.window@1", a.connected, a.searchWindow, implementation),
		candidateWindowSpec("station.window@1", a.station, a.searchWindow, implementation),
		candidateWindowSpec("bus.window@1", a.bus, a.searchWindow, implementation),
	}
}
