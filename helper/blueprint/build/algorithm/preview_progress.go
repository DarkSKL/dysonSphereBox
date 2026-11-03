package build

// 文件职责：把最终编码的蓝图按量化产线归属拆成稳定预览事件；使用：显式启用动画预览的生成调用；边界：不参与布局决策，普通生成不解码或发送分块。

import (
	"context"
	"errors"
	"fmt"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// 预览协议常量描述事件版本、类型和蓝图局部坐标语义；调用方应忽略未知版本和未知事件类型。
const (
	// ProgressProtocolVersion 职责：标记生成事件契约版本；使用：前端按版本解读载荷；边界：协议变化时递增。
	ProgressProtocolVersion = 1
	// ProgressKindPhase 职责：标记普通阶段事件；使用：生成状态提示；边界：不含建筑记录。
	ProgressKindPhase = "phase"
	// ProgressKindBuildingBatch 职责：标记模块建筑分块；使用：增量渲染预览；边界：只发送已严格编码蓝图的记录。
	ProgressKindBuildingBatch = "building-batch"
	// ProgressKindPreviewComplete 职责：标记预览分块流完整结束；使用：前端提交暂存预览；边界：缺少该事件时不得视为完整。
	ProgressKindPreviewComplete = "preview-complete"
	// PreviewCoordinateSystem 职责：标记 Area 局部坐标约定；使用：解释每条建筑记录的 AreaIndex 与 LocalOffset；边界：不是行星世界坐标。
	PreviewCoordinateSystem = "blueprint-area-local-v1"
	// previewBuildingBatchSize 职责：限制单个事件的建筑数；使用：分块发布和桥接负载控制；边界：不改变蓝图建筑索引。
	previewBuildingBatchSize = 128
)

// PreviewModule 职责：标识本次分块所属量化产线或共享设施；使用：预览动画分组；边界：ID 在同一请求内稳定，不作为持久化主键。
type PreviewModule struct {
	// ID 职责：提供请求内稳定模块键；使用：合并同一产线分块；边界：production:<index> 或 shared。
	ID string `json:"id"`
	// Name 职责：提供模块显示名；使用：预览标题；边界：不参与匹配和物料计算。
	Name string `json:"name"`
	// ProductionIndex 职责：关联量化产线数组下标；使用：前端联动工作区节点；边界：共享设施为 -1。
	ProductionIndex int `json:"productionIndex"`
	// MainItemID 职责：记录主产物原型；使用：预览图标和分组；边界：共享设施为 0。
	MainItemID int `json:"mainItemId,omitempty"`
	// RecipeID 职责：记录量化配方；使用：模块说明；边界：共享设施为 0。
	RecipeID int `json:"recipeId,omitempty"`
	// MachineID 职责：记录量化设备原型；使用：模块说明；边界：共享设施为 0。
	MachineID int `json:"machineId,omitempty"`
	// MachineCount 职责：记录量化机器数；使用：模块统计；边界：不等于该模块所有建筑数。
	MachineCount int `json:"machineCount,omitempty"`
}

// BuildingBatch 职责：承载单个模块的最终建筑记录分块和 Area 元数据；使用：请求级动画预览事件；边界：建筑索引与连接字段直接采用严格编码记录。
type BuildingBatch struct {
	// Module 职责：标识当前建筑组；使用：预览渲染分组；边界：名称只展示，不作为身份键。
	Module PreviewModule `json:"module"`
	// BatchIndex 职责：记录模块内零基分块序号；使用：检测模块块序；边界：不替代请求级 Sequence。
	BatchIndex int `json:"batchIndex"`
	// BatchCount 职责：记录当前模块总分块数；使用：模块完成检测；边界：全局总数由外层事件提供。
	BatchCount int `json:"batchCount"`
	// Buildings 职责：承载最终建筑记录；使用：增量渲染与连接；边界：Index、AreaIndex 和连接下标保持编码值。
	Buildings []blueprint.Building `json:"buildings"`
	// Areas 职责：提供局部坐标的 Area 元数据；使用：解释建筑坐标；边界：不会把局部偏移换算成行星世界坐标。
	Areas []blueprint.Area `json:"areas"`
}

// previewGroup 职责：保存一个预览模块及其最终建筑数组下标；使用：按模块拆分事件；边界：下标来自编码前的稳定记录表。
type previewGroup struct {
	// module 职责：保存模块描述；使用：构建批次元数据；边界：不修改量化计划。
	module PreviewModule
	// buildingIndices 职责：保存模块建筑在最终文档中的数组下标；使用：分块读取记录；边界：每条建筑只能归属一个组。
	buildingIndices []int
}

// emitBlueprintProgress 职责：为最终成功结果发布版本化模块建筑分块；使用：Generate 返回前的显式预览选项；边界：仅严格编码后解码最终蓝图，缺少完整事件流不得当作预览成功。
func emitBlueprintProgress(ctx context.Context, request Request, result Result, dataset *quant.Dataset) error {
	if !request.EmitBlueprintProgress || !hasProgressObserver(ctx) {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return previewContextError(err)
	}
	document, err := blueprint.Decode(result.Blueprint, true)
	if err != nil {
		return &blueprint.Error{Code: "GEN_E_PROGRESS_PREVIEW", Message: "无法解析最终预览记录: " + err.Error()}
	}
	if result.BuildingCount != len(document.Buildings) {
		return &blueprint.Error{Code: "GEN_E_PROGRESS_PREVIEW", Message: "最终预览建筑数与生成结果不一致"}
	}
	groups, err := previewGroups(request.Plan, result, dataset, len(document.Buildings))
	if err != nil {
		return err
	}
	totalBatches := 0
	for _, group := range groups {
		totalBatches += (len(group.buildingIndices) + previewBuildingBatchSize - 1) / previewBuildingBatchSize
	}
	currentBatch := 0
	for _, group := range groups {
		batchCount := (len(group.buildingIndices) + previewBuildingBatchSize - 1) / previewBuildingBatchSize
		for batchIndex, start := 0, 0; start < len(group.buildingIndices); batchIndex, start = batchIndex+1, start+previewBuildingBatchSize {
			if err := ctx.Err(); err != nil {
				return previewContextError(err)
			}
			end := min(start+previewBuildingBatchSize, len(group.buildingIndices))
			buildings := make([]blueprint.Building, 0, end-start)
			for _, index := range group.buildingIndices[start:end] {
				building := document.Buildings[index]
				building.Parameters = append([]int32(nil), building.Parameters...)
				building.RawTail = append([]byte(nil), building.RawTail...)
				buildings = append(buildings, building)
			}
			currentBatch++
			publishProgress(ctx, Progress{
				Kind:             ProgressKindBuildingBatch,
				Stage:            "preview",
				Message:          "正在输出蓝图模块预览",
				Current:          currentBatch,
				Total:            totalBatches,
				CoordinateSystem: PreviewCoordinateSystem,
				PreviewBatch: &BuildingBatch{
					Module:     group.module,
					BatchIndex: batchIndex,
					BatchCount: batchCount,
					Buildings:  buildings,
					Areas:      append([]blueprint.Area(nil), document.Areas...),
				},
			})
		}
	}
	if err := ctx.Err(); err != nil {
		return previewContextError(err)
	}
	publishProgress(ctx, Progress{
		Kind:             ProgressKindPreviewComplete,
		Stage:            "preview",
		Message:          "蓝图预览分块已完整输出",
		Current:          totalBatches,
		Total:            totalBatches,
		CoordinateSystem: PreviewCoordinateSystem,
		BuildingCount:    len(document.Buildings),
		BatchCount:       totalBatches,
	})
	return ctx.Err()
}

// previewGroups 职责：校验并组装量化产线与共享设施归属；使用：预览事件生成；边界：任何缺失、重复或越界建筑归属均失败关闭。
func previewGroups(plan quant.Result, result Result, dataset *quant.Dataset, buildingCount int) ([]previewGroup, error) {
	if dataset == nil {
		return nil, &blueprint.Error{Code: "GEN_E_PROGRESS_PREVIEW", Message: "预览缺少原版数据集"}
	}
	groups := make([]previewGroup, 0, len(result.Bindings)+1)
	seen := make([]bool, buildingCount)
	seenProductions := make([]bool, len(plan.Productions))
	claim := func(indices []int, group *previewGroup) error {
		for _, index := range indices {
			if index < 0 || index >= buildingCount || seen[index] {
				return &blueprint.Error{Code: "GEN_E_PROGRESS_PREVIEW", Message: "预览建筑归属索引重复或越界"}
			}
			seen[index] = true
			group.buildingIndices = append(group.buildingIndices, index)
		}
		return nil
	}
	for _, binding := range result.Bindings {
		if binding.ProductionIndex < 0 || binding.ProductionIndex >= len(plan.Productions) {
			return nil, &blueprint.Error{Code: "GEN_E_PROGRESS_PREVIEW", Message: "预览产线索引超出量化方案"}
		}
		if seenProductions[binding.ProductionIndex] {
			return nil, &blueprint.Error{Code: "GEN_E_PROGRESS_PREVIEW", Message: "预览产线绑定重复"}
		}
		seenProductions[binding.ProductionIndex] = true
		production := plan.Productions[binding.ProductionIndex]
		item, ok := dataset.ByID[production.MainItemID]
		name := fmt.Sprintf("物品 %d", production.MainItemID)
		if ok {
			name = item.Name
		}
		group := previewGroup{module: PreviewModule{
			ID:              fmt.Sprintf("production:%d", binding.ProductionIndex),
			Name:            name,
			ProductionIndex: binding.ProductionIndex,
			MainItemID:      production.MainItemID,
			RecipeID:        production.RecipeID,
			MachineID:       production.MachineID,
			MachineCount:    production.MachineCount,
		}}
		if err := claim(binding.BuildingIndices, &group); err != nil {
			return nil, err
		}
		if len(group.buildingIndices) == 0 {
			return nil, &blueprint.Error{Code: "GEN_E_PROGRESS_PREVIEW", Message: "预览产线没有建筑记录"}
		}
		groups = append(groups, group)
	}
	for _, bound := range seenProductions {
		if !bound {
			return nil, &blueprint.Error{Code: "GEN_E_PROGRESS_PREVIEW", Message: "预览存在未绑定产线"}
		}
	}
	if len(result.SharedBuildingIndices) > 0 {
		group := previewGroup{module: PreviewModule{ID: "shared", Name: "共享设施", ProductionIndex: -1}}
		if err := claim(result.SharedBuildingIndices, &group); err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	for _, claimed := range seen {
		if !claimed {
			return nil, &blueprint.Error{Code: "GEN_E_PROGRESS_PREVIEW", Message: "预览存在未归属建筑记录"}
		}
	}
	return groups, nil
}

// previewContextError 职责：把预览发送阶段的取消和截止转换为稳定蓝图错误；使用：批次边界检查；边界：不将部分事件流标记为完整。
func previewContextError(err error) error {
	if errors.Is(err, context.Canceled) {
		return &blueprint.Error{Code: "GEN_E_CANCELED", Message: "蓝图预览事件输出已取消"}
	}
	return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "蓝图预览事件输出超时"}
}
