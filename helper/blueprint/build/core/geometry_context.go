package build

// 文件职责：冻结装配到几何步骤之间的候选策略值。
// 使用：几何快照交接后恢复到当前取消ctx；边界：不保存旧context、函数或注册器，外部ctx不能替换已冻结的候选规则。

import "context"

// geometryContext 职责：明确保存当前候选的几何策略；使用：每个独立变换；边界：只包含具体值，不作为任意共享属性map。
type geometryContext struct {
	Rules                                                                                                    finalizationRuleContext
	Latitude                                                                                                 string
	NativeBand                                                                                               surfacePackedBand
	HasNativeBand                                                                                            bool
	PolarRoot, PolarMirror, PrototypeFootprint, Adjacent                                                     bool
	Workload, NativeWorkload                                                                                 int
	UnitPowerReserve, UnitPowerRequested, BalancedFrame, GroundTracks                                        bool
	BentRows, ShortPairedOutputs, ProducerSplitters, SpatialCompact                                          bool
	MaterialNetworks, SharedStationInputs, SharedStationOutputs, IndependentStations, StationInputAssignment bool
	ChemicalFour, RefineryCollinear, RefinerySameSide, HasRefinerySameSide                                   bool
	SpatialHoles, StreetClearance                                                                            float64
	HasStreetClearance                                                                                       bool
	FlyPruneDisabled                                                                                         bool
	FlyBound                                                                                                 denseFlyFacilityBound
	HasFlyBound                                                                                              bool
}

// captureGeometryContext 职责：捕获实际装配使用的完整几何选择；使用：装配快照；边界：无外部字段名映射，非法分流器规则直接失败。
func captureGeometryContext(ctx context.Context) (geometryContext, error) {
	rules, err := captureFinalizationRuleContext(ctx)
	if err != nil {
		return geometryContext{}, err
	}
	flag := func(key any) bool { value, _ := ctx.Value(key).(bool); return value }
	values := geometryContext{Rules: rules,
		PolarRoot: flag(densityPolarRootKey{}), PolarMirror: flag(densityPolarMirrorKey{}), PrototypeFootprint: flag(densityPrototypeFootprintKey{}), Adjacent: flag(denseAdjacentLatitudeKey{}),
		UnitPowerReserve: flag(denseUnitPowerReserveKey{}), UnitPowerRequested: flag(denseUnitPowerRequestedKey{}), BalancedFrame: flag(denseBalancedFrameKey{}), GroundTracks: flag(denseGroundTracksKey{}),
		BentRows: flag(denseBentRowsKey{}), ShortPairedOutputs: flag(denseShortPairedOutputsKey{}), ProducerSplitters: flag(denseProducerSplittersKey{}), SpatialCompact: flag(denseSpatialCompactKey{}),
		MaterialNetworks: flag(denseMaterialNetworksKey{}), SharedStationInputs: flag(denseStationSharedInputsKey{}), SharedStationOutputs: flag(denseStationSharedOutputsKey{}), IndependentStations: flag(independentStationPlacementKey{}), StationInputAssignment: flag(stationInputAssignmentKey{}),
		ChemicalFour: flag(chemicalFourKey{}), RefineryCollinear: flag(refineryCollinearKey{}), FlyPruneDisabled: flag(denseFlyFacilityPruneDisabledKey{}),
	}
	values.Latitude, _ = ctx.Value(densityLatitudeKey{}).(string)
	values.NativeBand, values.HasNativeBand = ctx.Value(densityNativeBandKey{}).(surfacePackedBand)
	values.Workload, _ = ctx.Value(generationWorkloadKey{}).(int)
	values.NativeWorkload, _ = ctx.Value(denseNativeWorkloadKey{}).(int)
	values.RefinerySameSide, values.HasRefinerySameSide = ctx.Value(refinerySameSideKey{}).(bool)
	values.SpatialHoles, _ = ctx.Value(denseSpatialHolesKey{}).(float64)
	values.StreetClearance, values.HasStreetClearance = ctx.Value(denseStreetFoldClearanceKey{}).(float64)
	values.FlyBound, values.HasFlyBound = ctx.Value(denseFlyFacilityBoundKey{}).(denseFlyFacilityBound)
	return values, nil
}

// absentGeometryValue 职责：明确遮蔽调用者新增的可选值；使用：冻结时原值不存在；边界：类型断言失败等同原始缺省，不使用nil传给context.WithValue。
type absentGeometryValue struct{}

// apply 职责：在本步骤ctx恢复值而保留本次取消及观察身份；使用：独占图恢复；边界：不会继承旧步骤截止或外部覆盖值。
func (values geometryContext) apply(ctx context.Context) context.Context {
	ctx = values.Rules.validationContext(ctx)
	for _, entry := range []struct{ key, value any }{
		{densityLatitudeKey{}, values.Latitude}, {densityPolarRootKey{}, values.PolarRoot}, {densityPolarMirrorKey{}, values.PolarMirror}, {densityPrototypeFootprintKey{}, values.PrototypeFootprint}, {denseAdjacentLatitudeKey{}, values.Adjacent},
		{generationWorkloadKey{}, values.Workload}, {denseNativeWorkloadKey{}, values.NativeWorkload}, {denseUnitPowerReserveKey{}, values.UnitPowerReserve}, {denseUnitPowerRequestedKey{}, values.UnitPowerRequested}, {denseBalancedFrameKey{}, values.BalancedFrame}, {denseGroundTracksKey{}, values.GroundTracks},
		{denseBentRowsKey{}, values.BentRows}, {denseShortPairedOutputsKey{}, values.ShortPairedOutputs}, {denseProducerSplittersKey{}, values.ProducerSplitters}, {denseSpatialCompactKey{}, values.SpatialCompact}, {denseMaterialNetworksKey{}, values.MaterialNetworks},
		{denseStationSharedInputsKey{}, values.SharedStationInputs}, {denseStationSharedOutputsKey{}, values.SharedStationOutputs}, {independentStationPlacementKey{}, values.IndependentStations}, {stationInputAssignmentKey{}, values.StationInputAssignment},
		{chemicalFourKey{}, values.ChemicalFour}, {refineryCollinearKey{}, values.RefineryCollinear}, {denseSpatialHolesKey{}, values.SpatialHoles}, {denseFlyFacilityPruneDisabledKey{}, values.FlyPruneDisabled},
	} {
		ctx = context.WithValue(ctx, entry.key, entry.value)
	}
	var band, refinery, clearance, bound any = absentGeometryValue{}, absentGeometryValue{}, absentGeometryValue{}, absentGeometryValue{}
	if values.HasNativeBand {
		band = values.NativeBand
	}
	if values.HasRefinerySameSide {
		refinery = values.RefinerySameSide
	}
	if values.HasStreetClearance {
		clearance = values.StreetClearance
	}
	if values.HasFlyBound {
		bound = values.FlyBound
	}
	ctx = context.WithValue(ctx, densityNativeBandKey{}, band)
	ctx = context.WithValue(ctx, refinerySameSideKey{}, refinery)
	ctx = context.WithValue(ctx, denseStreetFoldClearanceKey{}, clearance)
	return context.WithValue(ctx, denseFlyFacilityBoundKey{}, bound)
}
