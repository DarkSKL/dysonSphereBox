package build

// 文件职责：把外部生成请求转换为独立、只读的领域输入快照。
// 使用：工作流输入适配工具与独立算法回归；边界：不执行布局候选、重试、反射或工作流调度，不持有持久可变装配器。

import (
	"context"
	"encoding/json"
	"fmt"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// PreparedInput 职责：保存规范请求、独立设备数据和守恒物料图；使用：单候选核心按只读数据创建短时装配状态；边界：所有字段私有，外部只能读取值或独立副本。
type PreparedInput struct {
	// request 职责：保存深复制并归一化的领域请求；使用：核心求解；边界：创建后不写入。
	request Request
	// requestedSetting 职责：保留用户设置经最早归一阶段补默认后的独立值；使用：结果步骤比较自动升档；边界：不取最低档位计划的已升档设置，不再次校验原请求。
	requestedSetting Setting
	// dataset 职责：保存本任务独立的数据集；使用：设备/配方查询；边界：不引用可由量化调用者修改的全局缓存。
	dataset *quant.Dataset
	// graph 职责：保存重新核算的规范流图；使用：候选只读访问；边界：不信任请求自带流边。
	graph *quant.FlowGraph
	// policy 职责：保存可信模式的纯装配能力值；使用：核心创建装配器；边界：无生成、终检或重试回调。
	policy *assemblyPolicy
	// warnings/checks 职责：保留准备阶段诊断；使用：候选复制；边界：不当作最终实体校验证明。
	warnings []string
	checks   []LogisticsCheck
	// fingerprint 职责：标识规范输入内容；使用：实例可追踪身份；边界：方案与工具版本由调用层另行组合。
	fingerprint string
	// bytes 职责：保守估计快照保留空间；使用：任务内存预算；边界：不代表进程 RSS。
	bytes int64
	// nativeWorkload 职责：记录原生坐标选择使用的实际工作量估计；使用：显式领域事实查询；边界：不修改制造设备数量。
	nativeWorkload int
	// geometry 职责：保存当前显式球面规格；使用：跨步骤重建算法上下文；边界：不持有context或调度器。
	geometry SurfaceCandidateSpec
}

// cloneInputRequest 职责：复制请求内全部映射、切片与选项指针；使用：任务入口和快照导出；边界：内部原生纬带字段按值补回，不接受非有限 JSON 数值。
func cloneInputRequest(request Request) (Request, error) {
	raw, err := json.Marshal(request)
	if err != nil {
		return Request{}, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "生成输入不能形成有限数值快照: " + err.Error()}
	}
	var clone Request
	if err := json.Unmarshal(raw, &clone); err != nil {
		return Request{}, err
	}
	clone.shape, clone.search = request.shape, request.search
	clone.warperStock = request.warperStock
	if request.SurfaceLayout != nil {
		clone.SurfaceLayout.polarRoot = request.SurfaceLayout.polarRoot
		clone.SurfaceLayout.adjacentDepth = request.SurfaceLayout.adjacentDepth
		clone.SurfaceLayout.offsetWindow = request.SurfaceLayout.offsetWindow
		if request.SurfaceLayout.nativeBand != nil {
			band := *request.SurfaceLayout.nativeBand
			clone.SurfaceLayout.nativeBand = &band
		}
	}
	return clone, nil
}

// RequestSnapshot 职责：导出独立规范请求；使用：诊断或外层服务结果映射；边界：返回容器的修改不影响本实例输入。
func (input *PreparedInput) RequestSnapshot() (Request, error) {
	if input == nil {
		return Request{}, fmt.Errorf("生成输入快照为空")
	}
	return cloneInputRequest(input.request)
}

// Fingerprint 职责：返回规范输入指纹；使用：追踪与配置绑定；边界：不替代服务层可信 Plan 验证。
func (input *PreparedInput) Fingerprint() string { return input.fingerprint }

// EstimatedBytes 职责：提供领域快照保守大小；使用：工作流预算；边界：不可作为实际峰值内存测量。
func (input *PreparedInput) EstimatedBytes() int64 { return input.bytes }

// Mode 职责：返回可信拓扑编号；使用：方案族选择；边界：不运行模式分支。
func (input *PreparedInput) Mode() int { return input.request.Mode }

// Density 职责：返回已验证密度；使用：显式流程条件；边界：不授予参数修改设备事实的权限。
func (input *PreparedInput) Density() string { return input.request.Density }

// SurfaceClass 职责：把真实选区归一为稳定流程分支值；使用：YAML 的登记属性提供器；边界：精确纬圈和南北边缘仍由输入完整保留。
func (input *PreparedInput) SurfaceClass() string {
	layout := input.request.SurfaceLayout
	if layout == nil || layout.Mode == "planar" {
		return "planar"
	}
	if selected, ok := densityLatitudeSelectionFor(layout.LatitudeZone); ok {
		if selected.tier >= 4 {
			return "polar"
		}
		if selected.tier >= 2 {
			return "temperate"
		}
		return "equatorial"
	}
	return "automatic"
}

// RequiresNativeSurface 职责：明确提供原生球面坐标能力需求；使用：流程决定原生候选链；边界：不在核心入口偷偷切换算法。
func (input *PreparedInput) RequiresNativeSurface() bool {
	layout := input.request.SurfaceLayout
	if layout == nil || layout.LatitudeZone == "" {
		return false
	}
	selection, ok := densityLatitudeSelectionFor(layout.LatitudeZone)
	return ok && (selection.tier >= 4 || input.nativeWorkload >= input.request.search.LargeLayoutThreshold)
}

// newAssembly 职责：根据只读领域输入创建当前步骤的短时装配状态；使用：单候选核心；边界：不注册工作区、不创建候选调度或重试。
func (input *PreparedInput) newAssembly(ctx context.Context, preview *generationPreview) (*builder, error) {
	assembly, err := newBuilder(input.request.Setting)
	if err != nil {
		return nil, err
	}
	assembly.policy, assembly.generationCtx = input.policy, ctx
	assembly.flowGraph, assembly.preview = input.graph, preview
	assembly.warnings = append([]string(nil), input.warnings...)
	assembly.logistics = newLogisticsSummary(*input.graph, input.request.Setting)
	assembly.logistics.Checks = append(assembly.logistics.Checks, input.checks...)
	return assembly, nil
}
