package build

// 文件职责：定义几何定稿、最终验证和可编码冻结产物的所有权边界。
// 使用：联合求解器提交定稿产物，独立校验与编码入口依次消费。
// 边界：字段全部私有；公开读取只返回深拷贝，未通过同内容验证的快照不能编码。

import (
	"context"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/utils/digest"
)

// finalizationVersion 职责：标识冻结契约版本；使用：验证及编码匹配；边界：不代替真实规则版本。
const finalizationVersion = "final-entities-v1"

// GeometrySnapshot 职责：拥有全部修复及参数写入完成后的不可变实体和证据；使用：Validate 的唯一领域输入；边界：零值无效，不持有原始可变 builder。
type GeometrySnapshot struct {
	// data 职责：保存封闭的定稿内容；使用：内部只读共享；边界：任何公开访问都复制切片和参数。
	data *finalizedGeometry
}

// finalizedGeometry 职责：保存定稿记录、只读指针证据和展示元数据；使用：内部校验；边界：证据已与装配对象深拷贝隔离，每次校验再创建独立临时视图。
type finalizedGeometry struct {
	// proofIdentity 职责：为本次定稿提供不可伪造的独立候选身份；使用：单项证明只引用轻量token；边界：同内容重新装配也必须创建不同token，不保留整图反向引用。
	proofIdentity *ruleProofIdentity
	// document 职责：保存唯一待编码内容；使用：终检与编码；边界：创建后不再写入。
	document blueprint.Document
	// request 职责：保存规范需求；使用：独立终检；边界：与请求方切片和配置指针隔离。
	request Request
	// rules 职责：冻结候选实际使用的规则上下文；使用：独立校验重放；边界：不依赖调用方后来更换的context值或保留回调。
	rules finalizationRuleContext
	// dataset 职责：保存独立只读原型；使用：逐机能力核验；边界：与量化缓存及调用方映射隔离。
	dataset *quant.Dataset
	// evidence 职责：拥有只读实体关系图；使用：重建临时终检视图；边界：不保存装配器本身或可执行回调。
	evidence *finalizationEvidence
	// surface 职责：保存独立球面几何；使用：规则检查；边界：旧坐标缓存不带入终检。
	surface *surfaceEncoding
	// bindings 职责：保存产线归属；使用：结果投影；边界：不向外暴露底层切片。
	bindings []ProductionBinding
	// shared 职责：保存公共实体下标；使用：结果投影；边界：不参与重新编号。
	shared []int
	// zones 职责：保存最终展示区域；使用：结果投影；边界：不移动实体。
	zones []Zone
	// warnings 职责：保留装配告警；使用：成功投影；边界：不能降低任何硬规则。
	warnings []string
	// fingerprint 职责：绑定文档与需求；使用：阶段交接；边界：报告写入不改变该指纹。
	fingerprint [32]byte
	// previewScene 职责：保存独立最终场景与不对外暴露的旁路会话身份；使用：获选后原协议提交；边界：不参与领域校验，也不发布成功事件。
	previewScene *generationPreviewSnapshot
	// bytes 职责：记录封闭内容的保守保留空间；使用：产物预算；边界：不是RSS，也不计任务级旁路会话。
	bytes int64
}

// FinalValidation 职责：保存绑定同一内容和规则版本的最终验证产物；使用：FreezeValidatedGeometry；边界：报告与原内容分离，零值不能充当证据。
type FinalValidation struct {
	// data 职责：保存封闭验证结果；使用：只读共享；边界：不暴露可变报告。
	data *finalValidationData
}

// finalValidationData 职责：绑定已检查的几何与独立报告；使用：冻结和结果组装；边界：不包含实体写入能力。
type finalValidationData struct {
	// proofs 职责：保存完整同内容单项证明谱系；使用：编码批准守卫；边界：缺任意必需项不能用报告替代。
	proofs []RuleProof
	// geometry 职责：固定证据所属内容；使用：防止跨候选替换；边界：只能引用不可变产物。
	geometry *finalizedGeometry
	// fingerprint 职责：记录被检查内容摘要；使用：编码失效判定；边界：不包含报告本身。
	fingerprint [32]byte
	// version 职责：保存校验契约；使用：拒绝旧证据；边界：还需匹配规则来源。
	version string
	// logistics 职责：保存真实物料及容量报告；使用：结果投影；边界：冻结后只读。
	logistics *LogisticsSummary
	// audit 职责：保存最终规则覆盖；使用：结果投影；边界：不声称游戏运行验证。
	audit *RuleAudit
	// surfacePlan 职责：保留本次规则审计后的球面范围；使用：结果投影；边界：报告变化不修改实体内容。
	surfacePlan *SurfacePlan
	// density 职责：保存实际机器布局统计；使用：编码后补结果串摘要；边界：不重新读取原 builder。
	density *DensityReport
	// warnings 职责：保留终检新增的非阻断提示；使用：编码结果投影；边界：不修改定稿实体或源装配器。
	warnings []string
	// bytes 职责：记录独立验证报告的保守空间；使用：已验证产物计量；边界：允许高估共享不可变内容。
	bytes int64
}

// VerifiedSnapshot 职责：承载与最终验证一致的可编码快照；使用：Encode；边界：只能通过验证或显式匹配冻结入口创建。
type VerifiedSnapshot struct {
	// geometry 职责：引用不可变定稿；使用：编码读；边界：不提供 setter。
	geometry GeometrySnapshot
	// validation 职责：引用同内容证据；使用：编码前复核；边界：零值、异版本、异内容均拒绝。
	validation FinalValidation
}

// Document 职责：返回定稿文档副本；使用：诊断及外部工具查看；边界：修改副本不会修改快照或取得编码权限。
func (snapshot GeometrySnapshot) Document() blueprint.Document {
	if snapshot.data == nil {
		return blueprint.Document{}
	}
	return newFinalizationCloner().blueprintDocument(snapshot.data.document)
}

// Fingerprint 职责：返回不可变内容身份；使用：实例版本和产物依赖关联；边界：零值返回空摘要，不代表验证成功。
func (snapshot GeometrySnapshot) Fingerprint() [32]byte {
	if snapshot.data == nil {
		return [32]byte{}
	}
	return snapshot.data.fingerprint
}

// EstimatedBytes 职责：返回定稿产物的保守内存计量；使用：工作流工作区预算；边界：零值返回零，按复制图的值、集合和字符串空间加余量，不代表峰值RSS。
func (snapshot GeometrySnapshot) EstimatedBytes() int64 {
	if snapshot.data == nil {
		return 0
	}
	return snapshot.data.bytes
}

// Audit 职责：返回最终规则报告副本；使用：工作流发布诊断；边界：编辑报告不改变证据或实体版本。
func (validation FinalValidation) Audit() *RuleAudit {
	if validation.data == nil {
		return nil
	}
	c := newFinalizationCloner()
	return copyFinalizationPointer(c, validation.data.audit, c.buildRuleAudit)
}

// Logistics 职责：返回真实运输及候选比较指标的副本；使用：外部工作流在编码前选优；边界：修改报告不改变内容版本或验证证据。
func (validation FinalValidation) Logistics() *LogisticsSummary {
	if validation.data == nil {
		return nil
	}
	c := newFinalizationCloner()
	return copyFinalizationPointer(c, validation.data.logistics, c.buildLogisticsSummary)
}

// Validation 职责：返回封闭验证句柄；使用：读取报告及内容关联；边界：只共享不可变内部产物。
func (snapshot VerifiedSnapshot) Validation() FinalValidation { return snapshot.validation }

// Geometry 职责：返回只读定稿句柄；使用：获选后导出预览及统计；边界：文档访问仍深拷贝，不授予实体写入权限。
func (snapshot VerifiedSnapshot) Geometry() GeometrySnapshot { return snapshot.geometry }

// Metrics 职责：按值返回已验证候选的比较指标；使用：编码前候选归约；边界：零值返回空指标，编码权限仍需完整验证产物。
func (snapshot VerifiedSnapshot) Metrics() LayoutMetrics {
	if snapshot.validation.data == nil || snapshot.validation.data.logistics == nil {
		return LayoutMetrics{}
	}
	return snapshot.validation.data.logistics.Metrics
}

// EstimatedBytes 职责：返回定稿与验证产物合计保守大小；使用：工作流候选预算；边界：共享只读对象可重复计量，避免少计保留状态。
func (snapshot VerifiedSnapshot) EstimatedBytes() int64 {
	if snapshot.validation.data == nil {
		return 0
	}
	return snapshot.geometry.EstimatedBytes() + snapshot.validation.data.bytes
}

// finalGeometryFingerprint 职责：摘要实际编码文档和可信需求；使用：定稿、验证与编码三处；边界：非有限数据明确失败，不把报告纳入实体版本。
func finalGeometryFingerprint(document blueprint.Document, request Request, rules finalizationRuleContext) ([32]byte, error) {
	var nativeBand [3]int
	var nativeBandSet, polarRoot bool
	offset := false
	depth := 0
	if layout := request.SurfaceLayout; layout != nil {
		polarRoot = layout.polarRoot
		depth = layout.adjacentDepth
		offset = layout.offsetWindow
		if layout.nativeBand != nil {
			nativeBandSet = true
			nativeBand = [3]int{layout.nativeBand.low, layout.nativeBand.high, layout.nativeBand.segments}
		}
	}
	content, err := digest.JSON(struct {
		Document      blueprint.Document
		Request       Request
		Version       string
		NativeBand    [3]int
		NativeBandSet bool
		PolarRoot     bool
		Rules         finalizationRuleContext
		AdjacentDepth int  `json:"adjacentDepth,omitempty"`
		OffsetWindow  bool `json:"offsetWindow,omitempty"`
	}{document, request, finalizationVersion, nativeBand, nativeBandSet, polarRoot, rules, depth, offset})
	if err != nil {
		return [32]byte{}, &blueprint.Error{Code: "GEN_E_FINAL_SNAPSHOT", Message: "定稿内容无法建立稳定指纹：" + err.Error()}
	}
	return content, nil
}

// checkFinalGeometry 职责：拒绝零值和被内部错误改写的产物；使用：每次阶段交接；边界：不修复实体或重新签发证据。
func checkFinalGeometry(ctx context.Context, snapshot GeometrySnapshot) error {
	if err := ctx.Err(); err != nil {
		return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "最终快照处理已取消或超时"}
	}
	if snapshot.data == nil || snapshot.data.evidence == nil || snapshot.data.dataset == nil {
		return &blueprint.Error{Code: "GEN_E_FINAL_SNAPSHOT", Message: "缺少完整几何定稿产物"}
	}
	digest, err := finalGeometryFingerprint(snapshot.data.document, snapshot.data.request, snapshot.data.rules)
	if err != nil {
		return err
	}
	if digest != snapshot.data.fingerprint {
		return &blueprint.Error{Code: "GEN_E_FINAL_SNAPSHOT", Message: "最终实体内容已经变化，必须重新定稿和验证"}
	}
	return nil
}

// FreezeValidatedGeometry 职责：把同一内容的最终证据转换为编码权限；使用：工具分步提交验证后；边界：不接受不同候选、不同内容或不同契约版本的证据。
func FreezeValidatedGeometry(ctx context.Context, geometry GeometrySnapshot, validation FinalValidation) (VerifiedSnapshot, error) {
	if err := checkFinalGeometry(ctx, geometry); err != nil {
		return VerifiedSnapshot{}, err
	}
	if validation.data == nil || validation.data.audit == nil || validation.data.logistics == nil || validation.data.density == nil || validation.data.geometry != geometry.data || validation.data.fingerprint != geometry.data.fingerprint || validation.data.version != finalizationVersion {
		return VerifiedSnapshot{}, &blueprint.Error{Code: "GEN_E_FINAL_SNAPSHOT", Message: "最终验证与待编码实体的内容或版本不一致"}
	}
	if _, err := verifyCompleteRuleSet(geometry, validation.data.proofs); err != nil {
		return VerifiedSnapshot{}, err
	}
	return VerifiedSnapshot{geometry: geometry, validation: validation}, nil
}
