package generator

// 文件职责：把获选几何的真实Area交给引擎逐区域执行并严格汇合。
// 使用：编码前的共享区域流程；边界：工具不遍历工作流，区域报告不代替最终全图实体和连接验证。

import (
	"context"
	"fmt"
	"reflect"

	core "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/workflow"
)

// DescribeRegions 职责：发布获选几何的实际区域及纬带事实；使用：regions节点之前；边界：不根据最初请求选区推测所有Area。
func (t *toolset) DescribeRegions(ctx context.Context, s *workflow.StepContext, _ emptyArgs) error {
	selected, err := workflow.Read(s, t.keys.selected)
	if err != nil {
		return err
	}
	regions, err := core.GeometryRegionsContext(ctx, selected.Verified.Geometry())
	if err != nil {
		return err
	}
	return workflow.Stage(s, t.keys.regions, regions)
}

// CheckRegion 职责：只检查固定作用域中的真实区域；使用：引擎regions节点；边界：不能写另一区域报告，未知区域或事实不能跳过。
func (t *toolset) CheckRegion(ctx context.Context, s *workflow.StepContext, args core.RegionCheckOptions) error {
	selected, err := workflow.Read(s, t.keys.selected)
	if err != nil {
		return err
	}
	report, err := core.CheckGeometryRegion(ctx, selected.Verified.Geometry(), s.Region(), args)
	if err != nil {
		return err
	}
	return workflow.Stage(s, t.keys.regionReport, report)
}

// ValidateRegions 职责：汇合同内容的完整全图与逐区域证明；使用：唯一编码前的最终门槛；边界：不隐藏运行任何遗漏规则。
func (t *toolset) ValidateRegions(ctx context.Context, s *workflow.StepContext, _ emptyArgs) error {
	selected, err := workflow.Read(s, t.keys.selected)
	if err != nil {
		return err
	}
	reports := make([]core.RegionValidation, 0, len(s.Regions()))
	for _, region := range s.Regions() {
		report, err := workflow.ReadRegion(s, t.keys.regionReport, region)
		if err != nil {
			return err
		}
		reports = append(reports, report)
	}
	if err = core.RequireGeometryRegions(ctx, selected.Verified.Geometry(), reports); err != nil {
		return err
	}
	selected.Verified, err = core.FreezeValidatedGeometry(ctx, selected.Verified.Geometry(), selected.Verified.Validation())
	if err != nil {
		return err
	}
	return workflow.Stage(s, t.keys.approved, selected)
}

// regionProviders 职责：公开有限区域ID及当前真实纬带类别；使用：regions与switch；边界：只读取登记产物，不反射任意实例字段。
func regionProviders(a artifacts) []workflow.StateProvider {
	return []workflow.StateProvider{
		{Name: "factory.region-ids", Type: reflect.TypeFor[[]string](), Requires: []workflow.Artifact{a.regions.Ref()}, Resolve: func(_ context.Context, s *workflow.StepContext) (any, error) {
			regions, err := workflow.Read(s, a.regions)
			if err != nil {
				return nil, err
			}
			ids := make([]string, len(regions))
			for i, region := range regions {
				ids[i] = region.ID
			}
			return ids, nil
		}},
		{Name: "factory.region-class", Type: reflect.TypeFor[string](), Requires: []workflow.Artifact{a.regions.Ref()}, Resolve: func(_ context.Context, s *workflow.StepContext) (any, error) {
			regions, err := workflow.Read(s, a.regions)
			if err != nil {
				return nil, err
			}
			for _, region := range regions {
				if region.ID == s.Region() {
					return region.Class, nil
				}
			}
			return nil, fmt.Errorf("current region has no geometry descriptor")
		}},
	}
}
