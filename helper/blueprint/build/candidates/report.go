package candidates

// 文件职责：定义候选去重的独立统计契约；使用：生成结果及性能观察；边界：规划计数与真实候选执行、规则检查分开。

// Stage 职责：记录一次或多次同类规划的候选收缩；使用：按模式定位浪费；边界：同名阶段可跨不同纬度重复累计，不将它们当唯一全厂方案。
type Stage struct {
	Scope             string `json:"scope"`             // 职责：规划阶段键；使用：归类；边界：不是错误码。
	Proposed          int    `json:"proposed"`          // 职责：规范化前候选数；使用：原预算；边界：还未装配。
	Unique            int    `json:"unique"`            // 职责：保留的有效键数；使用：实际派发上限；边界：不保证不同键一定产生不同实体。
	Skipped           int    `json:"skipped"`           // 职责：被精确归一后去掉的计划数；使用：节省证据；边界：不含真实几何失败。
	InactiveModifiers int    `json:"inactiveModifiers"` // 职责：删除的无效策略修饰数；使用：解释规范化；边界：一项候选可能含多个修饰。
}

// Report 职责：解释单请求内的全局去重效果；使用：所有模式统一输出；边界：不会把部分阶段的相同错误归为重复方案。
type Report struct {
	Stages            []Stage `json:"stages"`            // 职责：阶段汇总；使用：细分；边界：稳定按scope排序。
	Proposed          int     `json:"proposed"`          // 职责：所有已登记阶段的原候选总数；使用：对比；边界：包括嵌套规划。
	Unique            int     `json:"unique"`            // 职责：规范化保留计划总数；使用：对比；边界：不同阶段不相互抵消。
	Skipped           int     `json:"skipped"`           // 职责：实际省掉的计划数；使用：对比；边界：不包含规则复用。
	InactiveModifiers int     `json:"inactiveModifiers"` // 职责：无效修饰合计；使用：说明；边界：不是建筑数量。
	FinalRules        Counts  `json:"finalRules"`        // 职责：完整相同终检状态的复用事实；使用：跨候选与重试阶段；边界：成功缓存最多128项，失败不缓存。
}
