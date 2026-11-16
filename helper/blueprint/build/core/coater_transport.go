package build

// 文件职责：从可信喷涂配置确定机器行附加供剂端口的物品。
// 使用：容量校验、直连、配送与物流塔共用；边界：供剂流量仍来自量化 Inputs，不把喷涂剂当成配方机器原料。

import "DysonSphereBox/helper/quant"

// productionSprayItem 职责：识别当前产线实际使用的增产剂；使用：附加输入校验及供剂端口；边界：未启用喷涂或等级无效返回0，不凭空增加物流需求。
func productionSprayItem(production quant.Production) int {
	if production.ProductionMode < 0 || production.IncreasePoints <= 0 || production.IncreaseLv < 1 || production.IncreaseLv > 3 {
		return 0
	}
	return 1140 + production.IncreaseLv
}
