package build

// 文件职责：定义跨布局族共用的搜索策略；新任务冻结一份，几何探针和候选复核沿用同一份值。

import (
	"fmt"
	"slices"
)

// LayoutSearchPolicy 为原生窗口、局部摆位及供电搜索提供预算与候选尺寸。
type LayoutSearchPolicy struct {
	// LargeLayoutThreshold 按机器数或已估算的几何工作量判定大厂。
	LargeLayoutThreshold int `json:"largeLayoutThreshold" min:"1"`
	// BufferEstimateSpan 是密铺缓存工作量估算的机器行跨度（工作格）。
	BufferEstimateSpan float64 `json:"bufferEstimateSpan" min:"0" max:"10000"`
	// NativeWindowRows 是非极区单带按顺序试用的原生行数。
	NativeWindowRows []int `json:"nativeWindowRows" min:"1" max:"16" itemMin:"6"`
	// StationMaterialThreshold 是追加常规塔物料排序候选的机器数门槛。
	StationMaterialThreshold int `json:"stationMaterialThreshold" min:"0"`
	// SurfaceWidthFraction 控制闭合配送块和塔块在球面上的中间宽度候选。
	SurfaceWidthFraction float64 `json:"surfaceWidthFraction" min:"0" max:"1"`
	// SideWidthFraction 限制中间宽度候选在短侧带的横向展开比例。
	SideWidthFraction float64 `json:"sideWidthFraction" min:"0" max:"1"`
	// PackingAttempts 和 StationPackingAttempts 按单元、朝向独立限制待检查的平移位置数。
	PackingAttempts        int `json:"packingAttempts" min:"1"`
	StationPackingAttempts int `json:"stationPackingAttempts" min:"1"`
	// PowerSearchNodes 和 PowerSearchBranches 限制单个供电候选的总节点与每层分支。
	PowerSearchNodes    int `json:"powerSearchNodes" min:"1"`
	PowerSearchBranches int `json:"powerSearchBranches" min:"1"`
}

// DefaultLayoutSearchPolicy 为旧工作流及独立输入准备提供历史策略，每份默认列表独立。
func DefaultLayoutSearchPolicy() LayoutSearchPolicy {
	return LayoutSearchPolicy{
		LargeLayoutThreshold: 256, BufferEstimateSpan: 32, NativeWindowRows: []int{64, 80, 96, 112},
		StationMaterialThreshold: 80, SurfaceWidthFraction: .95, SideWidthFraction: .6, PackingAttempts: 8192, StationPackingAttempts: 16384,
		PowerSearchNodes: 8192, PowerSearchBranches: 32,
	}
}

// Validate 拒绝无效预算及窗口列表，显式窗口可按任意优先级排列。
func (p LayoutSearchPolicy) Validate() error {
	if p.LargeLayoutThreshold < 1 || p.StationMaterialThreshold < 0 {
		return fmt.Errorf("largeLayoutThreshold 必须为正，stationMaterialThreshold 必须非负")
	}
	if !finitePositive(p.BufferEstimateSpan) || p.BufferEstimateSpan > 10000 {
		return fmt.Errorf("bufferEstimateSpan 必须在 (0,10000] 内")
	}
	if !finitePositive(p.SurfaceWidthFraction) || p.SurfaceWidthFraction > 1 {
		return fmt.Errorf("surfaceWidthFraction 必须在 (0,1] 内")
	}
	if !finitePositive(p.SideWidthFraction) || p.SideWidthFraction > 1 {
		return fmt.Errorf("sideWidthFraction 必须在 (0,1] 内")
	}
	if p.PackingAttempts < 1 || p.StationPackingAttempts < 1 || p.PowerSearchNodes < 1 || p.PowerSearchBranches < 1 {
		return fmt.Errorf("摆位及供电搜索预算必须为正整数")
	}
	if len(p.NativeWindowRows) == 0 || len(p.NativeWindowRows) > 16 {
		return fmt.Errorf("nativeWindowRows 需要 1 至 16 项")
	}
	for i, n := range p.NativeWindowRows {
		if n < 6 || slices.Contains(p.NativeWindowRows[:i], n) {
			return fmt.Errorf("nativeWindowRows 需要互异且至少为 6 的行数")
		}
	}
	return nil
}

// WithLayoutSearch 为当前生成请求冻结配置副本，后续候选共享只读策略。
func (request Request) WithLayoutSearch(policy LayoutSearchPolicy) Request {
	policy.NativeWindowRows = slices.Clone(policy.NativeWindowRows)
	request.search = &policy
	return request
}
