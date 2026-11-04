package build

// 文件职责：从获选候选的实际机器生成密度覆盖报告。
// 使用：所有最终规则和编码成功后；边界：不推测普通对照收益、不承诺游戏放置。

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/contract"
	"DysonSphereBox/helper/blueprint/density"
)

// DensityProductionReport 职责：记录单条产线实际应用的布局；使用：结果诊断；边界：数量来自最终候选实体。
type DensityProductionReport struct {
	// ReferenceMachines 职责：统计采用原图低层轨道变体的机器；使用：区分参考密铺和普通密集单排；边界：它是双排/单机计数的子集，不额外相加。
	ReferenceMachines int `json:"referenceMachines"`
	// ProductionIndex 职责：保存可信计划产线下标；使用：关联配方；边界：不是建筑编号。
	ProductionIndex int `json:"productionIndex"`
	// MachineID 职责：保存机器原型；使用：支持范围展示；边界：不推断其它等级。
	MachineID int `json:"machineId"`
	// MachineCount 职责：保存实际生产机器数；使用：覆盖统计；边界：不含带和分拣器。
	MachineCount int `json:"machineCount"`
	// PairedMachines 职责：保存相向双排中的机器数；使用：布局说明；边界：不代表面积收益。
	PairedMachines int `json:"pairedMachines"`
	// RowMachines 职责：保存密集单排中的机器数；使用：区分多端口候选；边界：不伪报双排覆盖。
	RowMachines int `json:"rowMachines"`
	// SingleMachines 职责：保存独立单机或单列堆叠中的机器数量；使用：说明无需缩短相邻水平间距的部分；边界：不算作密度改善。
	SingleMachines int `json:"singleMachines"`
}

// DensityReport 职责：返回真实密度应用范围与结果指纹；使用：前端和导出审阅；边界：数值预检与游戏实测严格分开。
type DensityReport struct {
	// ReferenceSourceSHA256 职责：保存实际使用的参考来源；使用：复验局部规则；边界：只在应用参考单元时填写。
	ReferenceSourceSHA256 string `json:"referenceSourceSha256,omitempty"`
	// Requested 职责：保存规范请求密度；使用：结果来源校验；边界：只有 standard/dense。
	Requested string `json:"requested"`
	// Applied 职责：标识是否实际创建多机密集单元；使用：成功说明；边界：单机不伪报应用。
	Applied bool `json:"applied"`
	// ProfileVersion 职责：保存生成规则版本；使用：旧结果失效；边界：不是游戏版本。
	ProfileVersion string `json:"profileVersion"`
	// BlueprintSHA256 职责：绑定最终编码产物；使用：验收记录关联；边界：摘要不替代格式校验。
	BlueprintSHA256 string `json:"blueprintSha256"`
	// LayoutEnvelopeArea 职责：保存现有布局包络格面积；使用：指标诊断；边界：不是测地面积或普通方案节省率。
	LayoutEnvelopeArea float64 `json:"layoutEnvelopeArea"`
	// Productions 职责：保存逐产线布局覆盖；使用：支持范围展示；边界：只含最终实际机器。
	Productions []DensityProductionReport `json:"productions"`
	// Verification 职责：说明当前数值检查覆盖；使用：结果提示；边界：不标记为游戏运行已验证。
	Verification string `json:"verification"`
}

// densityReport 职责：统计最终候选保留的机器布局来源；使用：finish 编码后；边界：dense 存在未登记机器时拒绝，禁止普通布局冒充密集结果。
func (b *builder) densityReport(req Request, code string, area float64) (*DensityReport, error) {
	digest := sha256.Sum256([]byte(code))
	report := &DensityReport{Requested: normalizeDensity(req.Density), ProfileVersion: contract.DensityProfileVersion, BlueprintSHA256: hex.EncodeToString(digest[:]), LayoutEnvelopeArea: area, Productions: []DensityProductionReport{}, Verification: "通过当前数值预检；游戏放置与连续运行尚未实测"}
	if report.Requested == densityDense {
		report.Verification = "按整图单次粘贴规则预检；已有场景接入、拖拽复制和实际运行需游戏验证"
	}
	byOwner := map[int]*DensityProductionReport{}
	for _, entity := range b.all {
		if entity.record.RecipeID <= 0 {
			continue
		}
		production := byOwner[entity.owner]
		if production == nil {
			production = &DensityProductionReport{ProductionIndex: entity.owner, MachineID: int(entity.record.ItemID)}
			byOwner[entity.owner] = production
		}
		production.MachineCount++
		if report.Requested == densityStandard {
			continue
		}
		switch entity.densityPattern {
		case "reference-pair":
			production.PairedMachines++
			production.ReferenceMachines++
			report.Applied = true
			report.ReferenceSourceSHA256 = density.ReferenceSourceSHA256
		case "reference-single":
			production.SingleMachines++
			production.ReferenceMachines++
			report.ReferenceSourceSHA256 = density.ReferenceSourceSHA256
		case "paired-rows":
			production.PairedMachines++
			report.Applied = true
		case "single-row":
			production.RowMachines++
			report.Applied = true
		case "single-machine":
			production.SingleMachines++
		default:
			return nil, &blueprint.Error{Code: "GEN_E_DENSITY_PROFILE", Message: "密集结果存在未登记布局来源的机器"}
		}
	}
	owners := make([]int, 0, len(byOwner))
	for owner := range byOwner {
		owners = append(owners, owner)
	}
	sort.Ints(owners)
	for _, owner := range owners {
		report.Productions = append(report.Productions, *byOwner[owner])
	}
	return report, nil
}
