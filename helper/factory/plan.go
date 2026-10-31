package factory

// 文件职责：隔离可信量化快照并兼容旧版缺少派生物流图的请求。
// 使用：服务缓存、生成复核及方案导出共同调用；边界：不接受客户端重写量化字段，客户端流图永不作为生成依据。

import (
	"encoding/json"
	"reflect"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// CloneQuantPlan 职责：复制请求级的所有切片、映射和流图，隔离调用方与服务缓存。
// 使用：Solve 保存快照及 Generate 读取快照；边界：非有限数据返回错误，不回退为共享可写引用。
func CloneQuantPlan(plan quant.Result) (quant.Result, error) {
	data, err := json.Marshal(plan)
	if err != nil {
		return quant.Result{}, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "量化快照包含无效数值，请重新量化"}
	}
	var copied quant.Result
	if err := json.Unmarshal(data, &copied); err != nil {
		return quant.Result{}, err
	}
	return copied, nil
}

// MatchQuantSnapshot 职责：仅比较可信量化字段，不将新版本的派生物流图当作旧工作区的必填内容。
// 使用：服务生成和导出前复核；边界：无论客户端传入、遗漏还是篡改流图，都必须由调用方恢复后端快照，不能直接使用候选图。
func MatchQuantSnapshot(trusted, candidate quant.Result) bool {
	trusted.FlowGraph, candidate.FlowGraph = nil, nil
	// 外供来源标记同样由服务保存的求解请求派生；旧工作区缺字段不改变真实原料数量。
	trusted.ExternalInputItemIDs, candidate.ExternalInputItemIDs = nil, nil
	return reflect.DeepEqual(trusted, candidate)
}
