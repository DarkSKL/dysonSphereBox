package build

// 文件职责：定义输入准备的封闭不可变阶段产物与版本身份。
// 使用：输入工作流分别提交事实、校验证据、档位计划和工作量；边界：不暴露缓存、可写请求或装配器，不以完成布尔值代替实际结果。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// inputStageIdentity 职责：保存真实内容及依赖版本的身份与保守大小；使用：所有阶段产物共享只读查询；边界：不是可伪造的公开完成标志。
type inputStageIdentity struct {
	// fingerprint 职责：绑定阶段内容及父产物；使用：组合核对；边界：零值无效。
	fingerprint string
	// bytes 职责：估计阶段及其引用内容保留大小；使用：任务内存预算；边界：不等于进程RSS。
	bytes int64
}

// Fingerprint 职责：返回不可变阶段内容身份；使用：依赖与审计；边界：空字符串不表示完成。
func (identity inputStageIdentity) Fingerprint() string { return identity.fingerprint }

// EstimatedBytes 职责：返回阶段保守大小；使用：工作流预算；边界：只读且不遍历可变缓存。
func (identity inputStageIdentity) EstimatedBytes() int64 { return identity.bytes }

// inputStageError 职责：拒绝缺失、错序或跨来源产物；使用：所有阶段和组合边界；边界：保持陈旧计划的公开错误类别。
func inputStageError(message string) error {
	return &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: message}
}

// makeInputStageIdentity 职责：按实际结果及依赖摘要建立稳定身份；使用：每个阶段成功出口；边界：取消不签发产物，非有限内容不能形成成功版本。
func makeInputStageIdentity(ctx context.Context, kind string, value any, parents ...string) (inputStageIdentity, error) {
	if err := ctx.Err(); err != nil {
		return inputStageIdentity{}, err
	}
	raw, err := json.Marshal(struct {
		Kind    string
		Parents []string
		Value   any
	}{kind, parents, value})
	if err != nil {
		return inputStageIdentity{}, inputStageError("输入阶段不能形成有限内容快照：" + err.Error())
	}
	if err := ctx.Err(); err != nil {
		return inputStageIdentity{}, err
	}
	digest := sha256.Sum256(raw)
	return inputStageIdentity{fingerprint: hex.EncodeToString(digest[:]), bytes: int64(len(raw))*4 + 256}, nil
}

// inputStageCause 职责：保留阶段取消的标准原因；使用：下层业务检查返回后；边界：不把超时或取消伪装为陈旧计划。
func inputStageCause(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// inputRequestIdentity 职责：补齐请求JSON以外的私有选区内容；使用：阶段指纹；边界：不计算几何、不改变内部选区。
func inputRequestIdentity(request Request) any {
	var band [3]int
	depth := 0
	set, root := false, false
	offset := false
	if request.SurfaceLayout != nil {
		root = request.SurfaceLayout.polarRoot
		depth = request.SurfaceLayout.adjacentDepth
		offset = request.SurfaceLayout.offsetWindow
		if request.SurfaceLayout.nativeBand != nil {
			set = true
			b := request.SurfaceLayout.nativeBand
			band = [3]int{b.low, b.high, b.segments}
		}
	}
	return struct {
		Request            Request
		NativeBand         [3]int
		BandSet, PolarRoot bool
		AdjacentDepth      int                 `json:"adjacentDepth,omitempty"`
		OffsetWindow       bool                `json:"offsetWindow,omitempty"`
		ShapePolicy        *LayoutShapePolicy  `json:"shapePolicy"`
		SearchPolicy       *LayoutSearchPolicy `json:"searchPolicy"`
	}{request, band, set, root, depth, offset, request.shape, request.search}
}

// NormalizedInputRequest 职责：保存独立且已规范的请求/设置/能力选择；使用：数据事实和来源核对；边界：尚未信任生产计划。
type NormalizedInputRequest struct {
	inputStageIdentity
	// request/policy 职责：保存本阶段规范结果；使用：后续只读阶段；边界：不引用调用者容器。
	request Request
	policy  *assemblyPolicy
}

// InputDatasetFacts 职责：保存从数据源独立复制的设备配方事实；使用：版本和产能规则；边界：不暴露quant事实缓存。
type InputDatasetFacts struct {
	inputStageIdentity
	// source/dataset 职责：绑定规范请求与独立事实；使用：后续内容校验；边界：创建后只读。
	source  *NormalizedInputRequest
	dataset *quant.Dataset
}

// PlanSourceValidation 职责：保存Plan来源、版本及非空制造集合的封闭证据；使用：生产速率复核前置；边界：不代替逐机能力或物料守恒。
type PlanSourceValidation struct {
	inputStageIdentity
	// requestID/datasetID 职责：绑定被校验的两份内容；使用：拒绝跨来源证据；边界：不只比较PlanID字符串。
	requestID, datasetID string
	// planID/dataVersion/productions 职责：记录实际核验事实；使用：稳定证据摘要；边界：不能由外部设置。
	planID, dataVersion string
	productions         int
}

// ProductionInputFacts 职责：保存以设备原型重算的配置能力与精确台数；使用：流图及容量阶段；边界：不接受省略来源核对。
type ProductionInputFacts struct {
	inputStageIdentity
	// request/facts/source 职责：绑定本次重算结果及可信来源证据；使用：后续纯阶段；边界：不修改原请求。
	request Request
	facts   *InputDatasetFacts
	source  *PlanSourceValidation
	policy  *assemblyPolicy
}

// InputFlowFacts 职责：保存按规范生产账重新建立的守恒流图；使用：最终输入组合；边界：不信任请求自带FlowGraph，不更改生产设备。
type InputFlowFacts struct {
	inputStageIdentity
	// production/graph 职责：绑定生产事实和真实新流图；使用：版本交接；边界：不暴露可写图。
	production *ProductionInputFacts
	graph      *quant.FlowGraph
}

// productionCapacityObservation 职责：记录逐生产项实际核验的设备与物料口径；使用：生产容量证据摘要；边界：不替代执行校验本身。
type productionCapacityObservation struct {
	// RecipeID/MachineID/MachineCount/InputKinds/OutputKinds 职责：记录实际核验对象和端点口径；使用：证据摘要；边界：不从理论端口估算实体。
	RecipeID, MachineID, MachineCount, InputKinds, OutputKinds int
	// ExactMachineCount 职责：记录原型重算精确需求；使用：取整能力证据；边界：不改变整数机器数。
	ExactMachineCount float64
	// LocalRecycle 职责：记录本地毛流回收需求；使用：能力证据；边界：不以净流远程端点替代。
	LocalRecycle bool
}

// ProductionCapacityValidation 职责：保存逐机能力、物料合法性和本地回流能力证据；使用：最低档位和最终组合；边界：绑定重算生产事实，不表示选定档位已够用。
type ProductionCapacityValidation struct {
	inputStageIdentity
	// productionID/observations 职责：保存验证对象版本及实际逐项结果；使用：拒绝缺失或跨计划组合；边界：字段私有。
	productionID string
	observations []productionCapacityObservation
}

// InputTransportPlan 职责：保存最低档位结果或一个明确升档请求；使用：档位容量规则与工作量估计；边界：应用档位不隐式证明容量通过。
type InputTransportPlan struct {
	inputStageIdentity
	// request/dataset/policy 职责：保存明确档位后的只读输入；使用：容量及工作量；边界：不重载数据。
	request Request
	dataset *quant.Dataset
	policy  *assemblyPolicy
	// production/capacity 职责：绑定初次最低档位的生产证明；使用：初次组合；边界：运输重试使用base而不重建生产事实。
	production *ProductionInputFacts
	capacity   *ProductionCapacityValidation
	// base 职责：绑定运输候选原始已验证输入；使用：复用其流图及能力；边界：不持有可变builder。
	base *PreparedInput
}

// Levels 职责：返回明确档位值；使用：诊断或任务预算；边界：不授予输入修改权。
func (plan *InputTransportPlan) Levels() TransportCandidateSpec {
	if plan == nil {
		return TransportCandidateSpec{}
	}
	return TransportCandidateSpec{BeltLevel: plan.request.Setting.BeltLv, InserterLevel: plan.request.Setting.InserterLv}
}

// transportCapacityObservation 职责：保存一个确定档位下的真实拆行结果；使用：容量证明摘要；边界：不执行候选布局或升档重试。
type transportCapacityObservation struct {
	// RecipeID/MachineID 职责：绑定拆行核验对象；使用：运输容量证据；边界：不创建实际机器。
	RecipeID, MachineID int
	// Counts 职责：保存确定档位下实际容量拆行；使用：证据摘要；边界：创建后私有只读。
	Counts []int
}

// TransportCapacityValidation 职责：证明一个明确档位满足逐机分拣器和带毛流限制；使用：输入组合；边界：不适用于其它档位，取消不签发。
type TransportCapacityValidation struct {
	inputStageIdentity
	// planID/observations 职责：绑定运输计划及实际容量拆行；使用：最终组合；边界：不暴露可写列表。
	planID       string
	observations []transportCapacityObservation
}

// InputWorkloadFacts 职责：保存指定档位下机器与缓存模块的真实工作量估计；使用：原生坐标预算和最终组合；边界：不是空完成标记，不改变机器数。
type InputWorkloadFacts struct {
	inputStageIdentity
	// planID/workload 职责：绑定档位及实际估计值；使用：组合及原生选择；边界：不接受其它档位的估计。
	planID   string
	workload int
}

// Clone 职责：复制NormalizedInputRequest封闭句柄；使用：任务产物所有权边界；边界：只共享不可变私有内容，零值不能充当合法阶段。
func (value *NormalizedInputRequest) Clone() (*NormalizedInputRequest, error) {
	if value == nil || value.fingerprint == "" {
		return nil, inputStageError("缺少NormalizedInputRequest阶段产物")
	}
	copy := *value
	return &copy, nil
}

// Clone 职责：复制InputDatasetFacts封闭句柄；使用：任务产物所有权边界；边界：只共享不可变私有内容，零值不能充当合法阶段。
func (value *InputDatasetFacts) Clone() (*InputDatasetFacts, error) {
	if value == nil || value.fingerprint == "" {
		return nil, inputStageError("缺少InputDatasetFacts阶段产物")
	}
	copy := *value
	return &copy, nil
}

// Clone 职责：复制PlanSourceValidation封闭句柄；使用：任务产物所有权边界；边界：只共享不可变私有内容，零值不能充当合法阶段。
func (value *PlanSourceValidation) Clone() (*PlanSourceValidation, error) {
	if value == nil || value.fingerprint == "" {
		return nil, inputStageError("缺少PlanSourceValidation阶段产物")
	}
	copy := *value
	return &copy, nil
}

// Clone 职责：复制ProductionInputFacts封闭句柄；使用：任务产物所有权边界；边界：只共享不可变私有内容，零值不能充当合法阶段。
func (value *ProductionInputFacts) Clone() (*ProductionInputFacts, error) {
	if value == nil || value.fingerprint == "" {
		return nil, inputStageError("缺少ProductionInputFacts阶段产物")
	}
	copy := *value
	return &copy, nil
}

// Clone 职责：复制InputFlowFacts封闭句柄；使用：任务产物所有权边界；边界：只共享不可变私有内容，零值不能充当合法阶段。
func (value *InputFlowFacts) Clone() (*InputFlowFacts, error) {
	if value == nil || value.fingerprint == "" {
		return nil, inputStageError("缺少InputFlowFacts阶段产物")
	}
	copy := *value
	return &copy, nil
}

// Clone 职责：复制ProductionCapacityValidation封闭句柄；使用：任务产物所有权边界；边界：只共享不可变私有内容，零值不能充当合法阶段。
func (value *ProductionCapacityValidation) Clone() (*ProductionCapacityValidation, error) {
	if value == nil || value.fingerprint == "" {
		return nil, inputStageError("缺少ProductionCapacityValidation阶段产物")
	}
	copy := *value
	return &copy, nil
}

// Clone 职责：复制InputTransportPlan封闭句柄；使用：任务产物所有权边界；边界：只共享不可变私有内容，零值不能充当合法阶段。
func (value *InputTransportPlan) Clone() (*InputTransportPlan, error) {
	if value == nil || value.fingerprint == "" {
		return nil, inputStageError("缺少InputTransportPlan阶段产物")
	}
	copy := *value
	return &copy, nil
}

// Clone 职责：复制TransportCapacityValidation封闭句柄；使用：任务产物所有权边界；边界：只共享不可变私有内容，零值不能充当合法阶段。
func (value *TransportCapacityValidation) Clone() (*TransportCapacityValidation, error) {
	if value == nil || value.fingerprint == "" {
		return nil, inputStageError("缺少TransportCapacityValidation阶段产物")
	}
	copy := *value
	return &copy, nil
}

// Clone 职责：复制InputWorkloadFacts封闭句柄；使用：任务产物所有权边界；边界：只共享不可变私有内容，零值不能充当合法阶段。
func (value *InputWorkloadFacts) Clone() (*InputWorkloadFacts, error) {
	if value == nil || value.fingerprint == "" {
		return nil, inputStageError("缺少InputWorkloadFacts阶段产物")
	}
	copy := *value
	return &copy, nil
}

// Fingerprint 职责：返回NormalizedInputRequest真实内容及依赖身份；使用：工作流版本关联；边界：nil句柄返回空摘要而非有效证明。
func (value *NormalizedInputRequest) Fingerprint() string {
	if value == nil {
		return ""
	}
	return value.fingerprint
}

// EstimatedBytes 职责：返回NormalizedInputRequest保守保留大小；使用：任务预算；边界：nil句柄返回零，不能代替Clone的有效性检查。
func (value *NormalizedInputRequest) EstimatedBytes() int64 {
	if value == nil {
		return 0
	}
	return value.bytes
}

// Fingerprint 职责：返回InputDatasetFacts真实内容及依赖身份；使用：工作流版本关联；边界：nil句柄返回空摘要而非有效证明。
func (value *InputDatasetFacts) Fingerprint() string {
	if value == nil {
		return ""
	}
	return value.fingerprint
}

// EstimatedBytes 职责：返回InputDatasetFacts保守保留大小；使用：任务预算；边界：nil句柄返回零，不能代替Clone的有效性检查。
func (value *InputDatasetFacts) EstimatedBytes() int64 {
	if value == nil {
		return 0
	}
	return value.bytes
}

// Fingerprint 职责：返回PlanSourceValidation真实内容及依赖身份；使用：工作流版本关联；边界：nil句柄返回空摘要而非有效证明。
func (value *PlanSourceValidation) Fingerprint() string {
	if value == nil {
		return ""
	}
	return value.fingerprint
}

// EstimatedBytes 职责：返回PlanSourceValidation保守保留大小；使用：任务预算；边界：nil句柄返回零，不能代替Clone的有效性检查。
func (value *PlanSourceValidation) EstimatedBytes() int64 {
	if value == nil {
		return 0
	}
	return value.bytes
}

// Fingerprint 职责：返回ProductionInputFacts真实内容及依赖身份；使用：工作流版本关联；边界：nil句柄返回空摘要而非有效证明。
func (value *ProductionInputFacts) Fingerprint() string {
	if value == nil {
		return ""
	}
	return value.fingerprint
}

// EstimatedBytes 职责：返回ProductionInputFacts保守保留大小；使用：任务预算；边界：nil句柄返回零，不能代替Clone的有效性检查。
func (value *ProductionInputFacts) EstimatedBytes() int64 {
	if value == nil {
		return 0
	}
	return value.bytes
}

// Fingerprint 职责：返回InputFlowFacts真实内容及依赖身份；使用：工作流版本关联；边界：nil句柄返回空摘要而非有效证明。
func (value *InputFlowFacts) Fingerprint() string {
	if value == nil {
		return ""
	}
	return value.fingerprint
}

// EstimatedBytes 职责：返回InputFlowFacts保守保留大小；使用：任务预算；边界：nil句柄返回零，不能代替Clone的有效性检查。
func (value *InputFlowFacts) EstimatedBytes() int64 {
	if value == nil {
		return 0
	}
	return value.bytes
}

// Fingerprint 职责：返回ProductionCapacityValidation真实内容及依赖身份；使用：工作流版本关联；边界：nil句柄返回空摘要而非有效证明。
func (value *ProductionCapacityValidation) Fingerprint() string {
	if value == nil {
		return ""
	}
	return value.fingerprint
}

// EstimatedBytes 职责：返回ProductionCapacityValidation保守保留大小；使用：任务预算；边界：nil句柄返回零，不能代替Clone的有效性检查。
func (value *ProductionCapacityValidation) EstimatedBytes() int64 {
	if value == nil {
		return 0
	}
	return value.bytes
}

// Fingerprint 职责：返回InputTransportPlan真实内容及依赖身份；使用：工作流版本关联；边界：nil句柄返回空摘要而非有效证明。
func (value *InputTransportPlan) Fingerprint() string {
	if value == nil {
		return ""
	}
	return value.fingerprint
}

// EstimatedBytes 职责：返回InputTransportPlan保守保留大小；使用：任务预算；边界：nil句柄返回零，不能代替Clone的有效性检查。
func (value *InputTransportPlan) EstimatedBytes() int64 {
	if value == nil {
		return 0
	}
	return value.bytes
}

// Fingerprint 职责：返回TransportCapacityValidation真实内容及依赖身份；使用：工作流版本关联；边界：nil句柄返回空摘要而非有效证明。
func (value *TransportCapacityValidation) Fingerprint() string {
	if value == nil {
		return ""
	}
	return value.fingerprint
}

// EstimatedBytes 职责：返回TransportCapacityValidation保守保留大小；使用：任务预算；边界：nil句柄返回零，不能代替Clone的有效性检查。
func (value *TransportCapacityValidation) EstimatedBytes() int64 {
	if value == nil {
		return 0
	}
	return value.bytes
}

// Fingerprint 职责：返回InputWorkloadFacts真实内容及依赖身份；使用：工作流版本关联；边界：nil句柄返回空摘要而非有效证明。
func (value *InputWorkloadFacts) Fingerprint() string {
	if value == nil {
		return ""
	}
	return value.fingerprint
}

// EstimatedBytes 职责：返回InputWorkloadFacts保守保留大小；使用：任务预算；边界：nil句柄返回零，不能代替Clone的有效性检查。
func (value *InputWorkloadFacts) EstimatedBytes() int64 {
	if value == nil {
		return 0
	}
	return value.bytes
}
