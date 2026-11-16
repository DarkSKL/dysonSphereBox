package build

// 文件职责：以显式类型复制最终证据关系图和参数集合。
// 使用：定稿冻结和每次终检建立独占临时图；边界：不用反射，不复制任务回调，保留循环引用及图内指针身份。

import (
	blueprint "DysonSphereBox/helper/blueprint"
	candidates "DysonSphereBox/helper/blueprint/build/candidates"
	preview "DysonSphereBox/helper/blueprint/build/preview"
	statistics "DysonSphereBox/helper/factory/statistics"
	quant "DysonSphereBox/helper/quant"
)

// finalizationEvidence 职责：拥有冻结的实体、运输及物料证据；使用：独立最终验证；边界：不保存可变builder或任务回调，内部关系只读。
type finalizationEvidence struct {
	// productionBuffers 职责：保存终检所需的独立关系；使用：重建临时视图；边界：不对外提供可变引用。
	productionBuffers []*productionBuffer
	// byproductInputs 职责：保存终检所需的独立关系；使用：重建临时视图；边界：不对外提供可变引用。
	byproductInputs []*byproductPriorityInput
	// mode1Deliveries 职责：保存终检所需的独立关系；使用：重建临时视图；边界：不对外提供可变引用。
	mode1Deliveries *mode1DeliveryPlan
	// denseBoundary 职责：保存终检所需的独立关系；使用：重建临时视图；边界：不对外提供可变引用。
	denseBoundary *denseBoundaryLayout
	// denseProducerSplits 职责：保存终检所需的独立关系；使用：重建临时视图；边界：不对外提供可变引用。
	denseProducerSplits *denseProducerSplitPlan
	// denseStationOutputs 职责：保存终检所需的独立关系；使用：重建临时视图；边界：不对外提供可变引用。
	denseStationOutputs *denseStationOutputPlan
	// denseSpatial 职责：保存终检所需的独立关系；使用：重建临时视图；边界：不对外提供可变引用。
	denseSpatial bool
	// nativeLatitudeScaled 职责：保存终检所需的独立关系；使用：重建临时视图；边界：不对外提供可变引用。
	nativeLatitudeScaled bool
	// nativeLongitudeScale 职责：保存终检所需的独立关系；使用：重建临时视图；边界：不对外提供可变引用。
	nativeLongitudeScale float64
	// polarFrame 职责：保存终检所需的独立关系；使用：重建临时视图；边界：不对外提供可变引用。
	polarFrame *densePolarFrame
	// denseStationSprayNetwork 职责：保存终检所需的独立关系；使用：重建临时视图；边界：不对外提供可变引用。
	denseStationSprayNetwork *denseStationSprayPlan
	// denseMaterialNetworks 职责：保存终检所需的独立关系；使用：重建临时视图；边界：不对外提供可变引用。
	denseMaterialNetworks *denseMaterialNetworkPlan
	// denseFlyBlackbox 职责：保存终检所需的独立关系；使用：重建临时视图；边界：不对外提供可变引用。
	denseFlyBlackbox *denseFlyBlackboxState
	// denseConnectedStreets 职责：保存终检所需的独立关系；使用：重建临时视图；边界：不对外提供可变引用。
	denseConnectedStreets []*denseConnectedStreetWork
	// all 职责：保存终检所需的独立关系；使用：重建临时视图；边界：不对外提供可变引用。
	all []*placed
	// machineLines 职责：保存终检所需的独立关系；使用：重建临时视图；边界：不对外提供可变引用。
	machineLines []*line
	// setting 职责：保存终检所需的独立关系；使用：重建临时视图；边界：不对外提供可变引用。
	setting Setting
	// flowGraph 职责：保存终检所需的独立关系；使用：重建临时视图；边界：不对外提供可变引用。
	flowGraph *quant.FlowGraph
	// transportRoutes 职责：保存终检所需的独立关系；使用：重建临时视图；边界：不对外提供可变引用。
	transportRoutes []*transportRoute
	// machineRecycles 职责：保存终检所需的独立关系；使用：重建临时视图；边界：不对外提供可变引用。
	machineRecycles []*machineRecycle
	// logistics 职责：保存终检所需的独立关系；使用：重建临时视图；边界：不对外提供可变引用。
	logistics *LogisticsSummary
	// stationLinks 职责：保存终检所需的独立关系；使用：重建临时视图；边界：不对外提供可变引用。
	stationLinks []stationTransportLink
	// stationLeftovers 职责：保存终检所需的独立关系；使用：重建临时视图；边界：不对外提供可变引用。
	stationLeftovers []stationLeftoverReturn
	// flyEndpoints 职责：保存终检所需的独立关系；使用：重建临时视图；边界：不对外提供可变引用。
	flyEndpoints []*flyTransportEndpoint
	// busLinks 职责：保存终检所需的独立关系；使用：重建临时视图；边界：不对外提供可变引用。
	busLinks []busTransportLink
	// busLayers 职责：保存终检所需的独立关系；使用：重建临时视图；边界：不对外提供可变引用。
	busLayers map[int]int
	// busLayerBelts 职责：保存终检所需的独立关系；使用：重建临时视图；边界：不对外提供可变引用。
	busLayerBelts map[int][]*placed
	// busFabric 职责：保存终检所需的独立关系；使用：重建临时视图；边界：不对外提供可变引用。
	busFabric []*busFabricRoute
	// directBoundary 职责：保存是否使用真实设施边界；使用：选择既有验证语义；边界：不保存或执行装配闭包。
	directBoundary bool
	// policy 职责：保存纯校验策略；使用：物流终检；边界：不保存生成钩子。
	policy *assemblyPolicy
}

// captureFinalizationEvidence 职责：切断装配器的全部可变证据别名；使用：定稿提交；边界：只复制校验所需数据域。
func captureFinalizationEvidence(source *builder, c *finalizationCloner) *finalizationEvidence {
	c.seedPlacedGraph(source.all)
	result := &finalizationEvidence{
		productionBuffers: copyFinalizationSlice(c, source.productionBuffers, func(v *productionBuffer) *productionBuffer {
			return copyFinalizationPointer(c, v, func(v productionBuffer) productionBuffer { return c.buildProductionBuffer(v) })
		}),
		byproductInputs: copyFinalizationSlice(c, source.byproductInputs, func(v *byproductPriorityInput) *byproductPriorityInput {
			return copyFinalizationPointer(c, v, func(v byproductPriorityInput) byproductPriorityInput { return c.buildByproductPriorityInput(v) })
		}),
		mode1Deliveries:          copyFinalizationPointer(c, source.mode1Deliveries, func(v mode1DeliveryPlan) mode1DeliveryPlan { return c.buildMode1DeliveryPlan(v) }),
		denseBoundary:            copyFinalizationPointer(c, source.denseBoundary, func(v denseBoundaryLayout) denseBoundaryLayout { return c.buildDenseBoundaryLayout(v) }),
		denseProducerSplits:      copyFinalizationPointer(c, source.denseProducerSplits, func(v denseProducerSplitPlan) denseProducerSplitPlan { return c.buildDenseProducerSplitPlan(v) }),
		denseStationOutputs:      copyFinalizationPointer(c, source.denseStationOutputs, func(v denseStationOutputPlan) denseStationOutputPlan { return c.buildDenseStationOutputPlan(v) }),
		denseSpatial:             source.denseSpatial,
		nativeLatitudeScaled:     source.nativeLatitudeScaled,
		nativeLongitudeScale:     source.nativeLongitudeScale,
		polarFrame:               copyFinalizationPointer(c, source.polarFrame, func(v densePolarFrame) densePolarFrame { return c.buildDensePolarFrame(v) }),
		denseStationSprayNetwork: copyFinalizationPointer(c, source.denseStationSprayNetwork, func(v denseStationSprayPlan) denseStationSprayPlan { return c.buildDenseStationSprayPlan(v) }),
		denseMaterialNetworks:    copyFinalizationPointer(c, source.denseMaterialNetworks, func(v denseMaterialNetworkPlan) denseMaterialNetworkPlan { return c.buildDenseMaterialNetworkPlan(v) }),
		denseFlyBlackbox:         copyFinalizationPointer(c, source.denseFlyBlackbox, func(v denseFlyBlackboxState) denseFlyBlackboxState { return c.buildDenseFlyBlackboxState(v) }),
		denseConnectedStreets: copyFinalizationSlice(c, source.denseConnectedStreets, func(v *denseConnectedStreetWork) *denseConnectedStreetWork {
			return copyFinalizationPointer(c, v, func(v denseConnectedStreetWork) denseConnectedStreetWork { return c.buildDenseConnectedStreetWork(v) })
		}),
		all: copyFinalizationSlice(c, source.all, func(v *placed) *placed {
			return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
		}),
		machineLines: copyFinalizationSlice(c, source.machineLines, func(v *line) *line { return copyFinalizationPointer(c, v, func(v line) line { return c.buildLine(v) }) }),
		setting:      c.buildSetting(source.setting),
		flowGraph:    copyFinalizationPointer(c, source.flowGraph, func(v quant.FlowGraph) quant.FlowGraph { return c.quantFlowGraph(v) }),
		transportRoutes: copyFinalizationSlice(c, source.transportRoutes, func(v *transportRoute) *transportRoute {
			return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
		}),
		machineRecycles: copyFinalizationSlice(c, source.machineRecycles, func(v *machineRecycle) *machineRecycle {
			return copyFinalizationPointer(c, v, func(v machineRecycle) machineRecycle { return c.buildMachineRecycle(v) })
		}),
		logistics:        copyFinalizationPointer(c, source.logistics, func(v LogisticsSummary) LogisticsSummary { return c.buildLogisticsSummary(v) }),
		stationLinks:     copyFinalizationSlice(c, source.stationLinks, func(v stationTransportLink) stationTransportLink { return c.buildStationTransportLink(v) }),
		stationLeftovers: copyFinalizationSlice(c, source.stationLeftovers, func(v stationLeftoverReturn) stationLeftoverReturn { return c.buildStationLeftoverReturn(v) }),
		flyEndpoints: copyFinalizationSlice(c, source.flyEndpoints, func(v *flyTransportEndpoint) *flyTransportEndpoint {
			return copyFinalizationPointer(c, v, func(v flyTransportEndpoint) flyTransportEndpoint { return c.buildFlyTransportEndpoint(v) })
		}),
		busLinks:  copyFinalizationSlice(c, source.busLinks, func(v busTransportLink) busTransportLink { return c.buildBusTransportLink(v) }),
		busLayers: copyFinalizationMap(c, source.busLayers, func(v int) int { return v }, func(v int) int { return v }),
		busLayerBelts: copyFinalizationMap(c, source.busLayerBelts, func(v int) int { return v }, func(v []*placed) []*placed {
			return copyFinalizationSlice(c, v, func(v *placed) *placed {
				return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
			})
		}),
		busFabric: copyFinalizationSlice(c, source.busFabric, func(v *busFabricRoute) *busFabricRoute {
			return copyFinalizationPointer(c, v, func(v busFabricRoute) busFabricRoute { return c.buildBusFabricRoute(v) })
		}),
		directBoundary: source.directBoundaryUnits != nil,
	}
	result.policy = cloneAssemblyPolicy(source.policy)
	return result
}

// validationView 职责：重建单项规则的独占临时对象；使用：独立规则入口；边界：所有报告写入及派生缓存仅影响临时图。
func (source *finalizationEvidence) validationView(c *finalizationCloner) *builder {
	c.seedPlacedGraph(source.all)
	result := &builder{
		productionBuffers: copyFinalizationSlice(c, source.productionBuffers, func(v *productionBuffer) *productionBuffer {
			return copyFinalizationPointer(c, v, func(v productionBuffer) productionBuffer { return c.buildProductionBuffer(v) })
		}),
		byproductInputs: copyFinalizationSlice(c, source.byproductInputs, func(v *byproductPriorityInput) *byproductPriorityInput {
			return copyFinalizationPointer(c, v, func(v byproductPriorityInput) byproductPriorityInput { return c.buildByproductPriorityInput(v) })
		}),
		mode1Deliveries:          copyFinalizationPointer(c, source.mode1Deliveries, func(v mode1DeliveryPlan) mode1DeliveryPlan { return c.buildMode1DeliveryPlan(v) }),
		denseBoundary:            copyFinalizationPointer(c, source.denseBoundary, func(v denseBoundaryLayout) denseBoundaryLayout { return c.buildDenseBoundaryLayout(v) }),
		denseProducerSplits:      copyFinalizationPointer(c, source.denseProducerSplits, func(v denseProducerSplitPlan) denseProducerSplitPlan { return c.buildDenseProducerSplitPlan(v) }),
		denseStationOutputs:      copyFinalizationPointer(c, source.denseStationOutputs, func(v denseStationOutputPlan) denseStationOutputPlan { return c.buildDenseStationOutputPlan(v) }),
		denseSpatial:             source.denseSpatial,
		nativeLatitudeScaled:     source.nativeLatitudeScaled,
		nativeLongitudeScale:     source.nativeLongitudeScale,
		polarFrame:               copyFinalizationPointer(c, source.polarFrame, func(v densePolarFrame) densePolarFrame { return c.buildDensePolarFrame(v) }),
		denseStationSprayNetwork: copyFinalizationPointer(c, source.denseStationSprayNetwork, func(v denseStationSprayPlan) denseStationSprayPlan { return c.buildDenseStationSprayPlan(v) }),
		denseMaterialNetworks:    copyFinalizationPointer(c, source.denseMaterialNetworks, func(v denseMaterialNetworkPlan) denseMaterialNetworkPlan { return c.buildDenseMaterialNetworkPlan(v) }),
		denseFlyBlackbox:         copyFinalizationPointer(c, source.denseFlyBlackbox, func(v denseFlyBlackboxState) denseFlyBlackboxState { return c.buildDenseFlyBlackboxState(v) }),
		denseConnectedStreets: copyFinalizationSlice(c, source.denseConnectedStreets, func(v *denseConnectedStreetWork) *denseConnectedStreetWork {
			return copyFinalizationPointer(c, v, func(v denseConnectedStreetWork) denseConnectedStreetWork { return c.buildDenseConnectedStreetWork(v) })
		}),
		all: copyFinalizationSlice(c, source.all, func(v *placed) *placed {
			return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
		}),
		machineLines: copyFinalizationSlice(c, source.machineLines, func(v *line) *line { return copyFinalizationPointer(c, v, func(v line) line { return c.buildLine(v) }) }),
		setting:      c.buildSetting(source.setting),
		flowGraph:    copyFinalizationPointer(c, source.flowGraph, func(v quant.FlowGraph) quant.FlowGraph { return c.quantFlowGraph(v) }),
		transportRoutes: copyFinalizationSlice(c, source.transportRoutes, func(v *transportRoute) *transportRoute {
			return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
		}),
		machineRecycles: copyFinalizationSlice(c, source.machineRecycles, func(v *machineRecycle) *machineRecycle {
			return copyFinalizationPointer(c, v, func(v machineRecycle) machineRecycle { return c.buildMachineRecycle(v) })
		}),
		logistics:        copyFinalizationPointer(c, source.logistics, func(v LogisticsSummary) LogisticsSummary { return c.buildLogisticsSummary(v) }),
		stationLinks:     copyFinalizationSlice(c, source.stationLinks, func(v stationTransportLink) stationTransportLink { return c.buildStationTransportLink(v) }),
		stationLeftovers: copyFinalizationSlice(c, source.stationLeftovers, func(v stationLeftoverReturn) stationLeftoverReturn { return c.buildStationLeftoverReturn(v) }),
		flyEndpoints: copyFinalizationSlice(c, source.flyEndpoints, func(v *flyTransportEndpoint) *flyTransportEndpoint {
			return copyFinalizationPointer(c, v, func(v flyTransportEndpoint) flyTransportEndpoint { return c.buildFlyTransportEndpoint(v) })
		}),
		busLinks:  copyFinalizationSlice(c, source.busLinks, func(v busTransportLink) busTransportLink { return c.buildBusTransportLink(v) }),
		busLayers: copyFinalizationMap(c, source.busLayers, func(v int) int { return v }, func(v int) int { return v }),
		busLayerBelts: copyFinalizationMap(c, source.busLayerBelts, func(v int) int { return v }, func(v []*placed) []*placed {
			return copyFinalizationSlice(c, v, func(v *placed) *placed {
				return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
			})
		}),
		busFabric: copyFinalizationSlice(c, source.busFabric, func(v *busFabricRoute) *busFabricRoute {
			return copyFinalizationPointer(c, v, func(v busFabricRoute) busFabricRoute { return c.buildBusFabricRoute(v) })
		}),
	}
	result.policy = cloneAssemblyPolicy(source.policy)
	if source.directBoundary {
		result.directBoundaryUnits = finalizationBoundaryMarker
	}
	return result
}

// quantItemRate 职责：复制ItemRate的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) quantItemRate(v quant.ItemRate) quant.ItemRate {
	out := v
	out.Name = c.copyString(v.Name)
	return out
}

// quantProductionCapacity 职责：复制ProductionCapacity的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) quantProductionCapacity(v quant.ProductionCapacity) quant.ProductionCapacity {
	out := v
	out.Basis = c.copyString(v.Basis)
	out.Inputs = copyFinalizationSlice(c, v.Inputs, func(v quant.ItemRate) quant.ItemRate { return c.quantItemRate(v) })
	out.Outputs = copyFinalizationSlice(c, v.Outputs, func(v quant.ItemRate) quant.ItemRate { return c.quantItemRate(v) })
	out.GrossInputs = copyFinalizationSlice(c, v.GrossInputs, func(v quant.ItemRate) quant.ItemRate { return c.quantItemRate(v) })
	out.GrossOutputs = copyFinalizationSlice(c, v.GrossOutputs, func(v quant.ItemRate) quant.ItemRate { return c.quantItemRate(v) })
	return out
}

// quantProduction 职责：复制Production的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) quantProduction(v quant.Production) quant.Production {
	out := v
	out.Inputs = copyFinalizationSlice(c, v.Inputs, func(v quant.ItemRate) quant.ItemRate { return c.quantItemRate(v) })
	out.Outputs = copyFinalizationSlice(c, v.Outputs, func(v quant.ItemRate) quant.ItemRate { return c.quantItemRate(v) })
	out.Capacity = copyFinalizationPointer(c, v.Capacity, func(v quant.ProductionCapacity) quant.ProductionCapacity { return c.quantProductionCapacity(v) })
	out.TargetItemIDs = copyFinalizationSlice(c, v.TargetItemIDs, func(v int) int { return v })
	return out
}

// quantRecipe 职责：复制Recipe的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) quantRecipe(v quant.Recipe) quant.Recipe {
	out := v
	out.Name = c.copyString(v.Name)
	out.Factories = copyFinalizationSlice(c, v.Factories, func(v int) int { return v })
	out.Items = copyFinalizationSlice(c, v.Items, func(v int) int { return v })
	out.ItemCounts = copyFinalizationSlice(c, v.ItemCounts, func(v float64) float64 { return v })
	out.Results = copyFinalizationSlice(c, v.Results, func(v int) int { return v })
	out.ResultCounts = copyFinalizationSlice(c, v.ResultCounts, func(v float64) float64 { return v })
	return out
}

// buildVector 职责：复制vector的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildVector(v vector) vector {
	out := v
	return out
}

// quantFlowEdge 职责：复制FlowEdge的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) quantFlowEdge(v quant.FlowEdge) quant.FlowEdge {
	out := v
	out.ID = c.copyString(v.ID)
	out.SourceNodeID = c.copyString(v.SourceNodeID)
	out.TargetNodeID = c.copyString(v.TargetNodeID)
	out.FlowKind = c.copyString(v.FlowKind)
	return out
}

// buildTransportRoute 职责：复制transportRoute的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildTransportRoute(v transportRoute) transportRoute {
	out := v
	out.entryDirection = c.buildVector(v.entryDirection)
	out.edge = c.quantFlowEdge(v.edge)
	out.from = copyFinalizationPointer(c, v.from, func(v transportRow) transportRow { return c.buildTransportRow(v) })
	out.to = copyFinalizationPointer(c, v.to, func(v transportRow) transportRow { return c.buildTransportRow(v) })
	out.filters = copyFinalizationSlice(c, v.filters, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.path = copyFinalizationSlice(c, v.path, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.sharedPath = copyFinalizationSlice(c, v.sharedPath, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.start = copyFinalizationPointer(c, v.start, func(v placed) placed { return c.buildPlaced(v) })
	out.end = copyFinalizationPointer(c, v.end, func(v placed) placed { return c.buildPlaced(v) })
	out.routingEntry = copyFinalizationPointer(c, v.routingEntry, func(v placed) placed { return c.buildPlaced(v) })
	out.priorityInput = copyFinalizationPointer(c, v.priorityInput, func(v placed) placed { return c.buildPlaced(v) })
	out.boundary = copyFinalizationPointer(c, v.boundary, func(v placed) placed { return c.buildPlaced(v) })
	return out
}

// buildMode1DeliveryGroup 职责：复制mode1DeliveryGroup的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildMode1DeliveryGroup(v mode1DeliveryGroup) mode1DeliveryGroup {
	out := v
	out.members = copyFinalizationSlice(c, v.members, func(v *transportRoute) *transportRoute {
		return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	})
	out.sources = copyFinalizationMap(c, v.sources, func(v *transportRow) *transportRow {
		return copyFinalizationPointer(c, v, func(v transportRow) transportRow { return c.buildTransportRow(v) })
	}, func(v float64) float64 { return v })
	out.head = copyFinalizationPointer(c, v.head, func(v placed) placed { return c.buildPlaced(v) })
	out.end = copyFinalizationPointer(c, v.end, func(v placed) placed { return c.buildPlaced(v) })
	out.entry = copyFinalizationPointer(c, v.entry, func(v placed) placed { return c.buildPlaced(v) })
	out.monitor = copyFinalizationPointer(c, v.monitor, func(v placed) placed { return c.buildPlaced(v) })
	return out
}

// buildSplitterGateAssembly 职责：复制splitterGateAssembly的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildSplitterGateAssembly(v splitterGateAssembly) splitterGateAssembly {
	out := v
	out.gate = copyFinalizationPointer(c, v.gate, func(v placed) placed { return c.buildPlaced(v) })
	out.entities = copyFinalizationSlice(c, v.entities, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.input = copyFinalizationSlice(c, v.input, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.outputs = copyFinalizationMap(c, v.outputs, func(v int8) int8 { return v }, func(v []*placed) []*placed {
		return copyFinalizationSlice(c, v, func(v *placed) *placed {
			return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
		})
	})
	out.outputSlots = copyFinalizationSlice(c, v.outputSlots, func(v int8) int8 { return v })
	out.links = copyFinalizationMap(c, v.links, func(v int8) int8 { return v }, func(v *splitterGatePortLink) *splitterGatePortLink {
		return copyFinalizationPointer(c, v, func(v splitterGatePortLink) splitterGatePortLink { return c.buildSplitterGatePortLink(v) })
	})
	return out
}

// buildSplitterGatePortLink 职责：复制splitterGatePortLink的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildSplitterGatePortLink(v splitterGatePortLink) splitterGatePortLink {
	out := v
	out.assembly = copyFinalizationPointer(c, v.assembly, func(v splitterGateAssembly) splitterGateAssembly { return c.buildSplitterGateAssembly(v) })
	out.nodes = copyFinalizationSlice(c, v.nodes, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	return out
}

// buildDenseCargoTarget 职责：复制denseCargoTarget的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseCargoTarget(v denseCargoTarget) denseCargoTarget {
	out := v
	out.relative = c.buildVector(v.relative)
	out.track = copyFinalizationSlice(c, v.track, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.coater = copyFinalizationPointer(c, v.coater, func(v placed) placed { return c.buildPlaced(v) })
	return out
}

// blueprintPoint 职责：复制Point的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) blueprintPoint(v blueprint.Point) blueprint.Point {
	out := v
	return out
}

// blueprintBuilding 职责：复制Building的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) blueprintBuilding(v blueprint.Building) blueprint.Building {
	out := v
	out.RecordLayout = c.copyString(v.RecordLayout)
	out.LocalOffset = c.blueprintPoint(v.LocalOffset)
	out.LocalOffset2 = c.blueprintPoint(v.LocalOffset2)
	out.Parameters = copyFinalizationSlice(c, v.Parameters, func(v int32) int32 { return v })
	out.Content = c.copyString(v.Content)
	out.RawTail = copyFinalizationSlice(c, v.RawTail, func(v byte) byte { return v })
	return out
}

// buildProductionRateAnnotation 职责：复制ProductionRateAnnotation的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildProductionRateAnnotation(v ProductionRateAnnotation) ProductionRateAnnotation {
	out := v
	out.Direction = c.copyString(v.Direction)
	out.Basis = c.copyString(v.Basis)
	return out
}

// buildPlaced 职责：复制placed的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildPlaced(v placed) placed {
	out := v
	out.deliveryMonitor = copyFinalizationPointer(c, v.deliveryMonitor, func(v mode1DeliveryGroup) mode1DeliveryGroup { return c.buildMode1DeliveryGroup(v) })
	out.splitterPortLink = copyFinalizationPointer(c, v.splitterPortLink, func(v splitterGatePortLink) splitterGatePortLink { return c.buildSplitterGatePortLink(v) })
	out.splitterGate = copyFinalizationPointer(c, v.splitterGate, func(v splitterGateAssembly) splitterGateAssembly { return c.buildSplitterGateAssembly(v) })
	out.flowMonitorHost = copyFinalizationPointer(c, v.flowMonitorHost, func(v placed) placed { return c.buildPlaced(v) })
	out.densityCargo = c.buildDenseCargoTarget(v.densityCargo)
	out.layoutFootprint = copyFinalizationPointer(c, v.layoutFootprint, func(v [2]vector) [2]vector {
		return func(v [2]vector) [2]vector {
			for i := range v {
				v[i] = c.buildVector(v[i])
			}
			return v
		}(v)
	})
	out.densityPattern = c.copyString(v.densityPattern)
	out.previewID = c.copyString(v.previewID)
	out.record = c.blueprintBuilding(v.record)
	out.a = c.buildVector(v.a)
	out.b = c.buildVector(v.b)
	out.input = copyFinalizationPointer(c, v.input, func(v placed) placed { return c.buildPlaced(v) })
	out.output = copyFinalizationPointer(c, v.output, func(v placed) placed { return c.buildPlaced(v) })
	out.productionRate = copyFinalizationPointer(c, v.productionRate, func(v ProductionRateAnnotation) ProductionRateAnnotation { return c.buildProductionRateAnnotation(v) })
	return out
}

// buildLine 职责：复制line的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildLine(v line) line {
	out := v
	out.all = copyFinalizationSlice(c, v.all, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.inputs = copyFinalizationSlice(c, v.inputs, func(v []*placed) []*placed {
		return copyFinalizationSlice(c, v, func(v *placed) *placed {
			return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
		})
	})
	out.outputs = copyFinalizationSlice(c, v.outputs, func(v []*placed) []*placed {
		return copyFinalizationSlice(c, v, func(v *placed) *placed {
			return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
		})
	})
	out.output = copyFinalizationSlice(c, v.output, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.machines = copyFinalizationSlice(c, v.machines, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.coaters = copyFinalizationSlice(c, v.coaters, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.coaterInput = copyFinalizationPointer(c, v.coaterInput, func(v placed) placed { return c.buildPlaced(v) })
	out.recipe = c.quantRecipe(v.recipe)
	out.info = c.quantProduction(v.info)
	return out
}

// buildMachineRecycle 职责：复制machineRecycle的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildMachineRecycle(v machineRecycle) machineRecycle {
	out := v
	out.row = copyFinalizationPointer(c, v.row, func(v transportRow) transportRow { return c.buildTransportRow(v) })
	out.source = copyFinalizationSlice(c, v.source, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.surplus = copyFinalizationSlice(c, v.surplus, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.intake = copyFinalizationSlice(c, v.intake, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.outputGate = copyFinalizationPointer(c, v.outputGate, func(v placed) placed { return c.buildPlaced(v) })
	out.inputGate = copyFinalizationPointer(c, v.inputGate, func(v placed) placed { return c.buildPlaced(v) })
	out.fresh = copyFinalizationPointer(c, v.fresh, func(v placed) placed { return c.buildPlaced(v) })
	out.returned = copyFinalizationPointer(c, v.returned, func(v placed) placed { return c.buildPlaced(v) })
	out.freshPath = copyFinalizationSlice(c, v.freshPath, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.returnedPath = copyFinalizationSlice(c, v.returnedPath, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.route = copyFinalizationPointer(c, v.route, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	return out
}

// buildConnectedSelfSpray 职责：复制connectedSelfSpray的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildConnectedSelfSpray(v connectedSelfSpray) connectedSelfSpray {
	out := v
	out.row = copyFinalizationPointer(c, v.row, func(v transportRow) transportRow { return c.buildTransportRow(v) })
	out.source = copyFinalizationPointer(c, v.source, func(v placed) placed { return c.buildPlaced(v) })
	out.arm = copyFinalizationPointer(c, v.arm, func(v placed) placed { return c.buildPlaced(v) })
	out.path = copyFinalizationSlice(c, v.path, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	return out
}

// buildTransportRow 职责：复制transportRow的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildTransportRow(v transportRow) transportRow {
	out := v
	out.info = c.quantProduction(v.info)
	out.recipe = c.quantRecipe(v.recipe)
	out.line = copyFinalizationPointer(c, v.line, func(v line) line { return c.buildLine(v) })
	out.inputs = copyFinalizationMap(c, v.inputs, func(v int) int { return v }, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.outputs = copyFinalizationSlice(c, v.outputs, func(v *transportRoute) *transportRoute {
		return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	})
	out.inputStubs = copyFinalizationMap(c, v.inputStubs, func(v int) int { return v }, func(v []*placed) []*placed {
		return copyFinalizationSlice(c, v, func(v *placed) *placed {
			return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
		})
	})
	out.sprayInput = copyFinalizationPointer(c, v.sprayInput, func(v placed) placed { return c.buildPlaced(v) })
	out.sprayStub = copyFinalizationSlice(c, v.sprayStub, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.recycles = copyFinalizationSlice(c, v.recycles, func(v *machineRecycle) *machineRecycle {
		return copyFinalizationPointer(c, v, func(v machineRecycle) machineRecycle { return c.buildMachineRecycle(v) })
	})
	out.buffers = copyFinalizationMap(c, v.buffers, func(v int) int { return v }, func(v *productionBuffer) *productionBuffer {
		return copyFinalizationPointer(c, v, func(v productionBuffer) productionBuffer { return c.buildProductionBuffer(v) })
	})
	out.selfSpray = copyFinalizationPointer(c, v.selfSpray, func(v connectedSelfSpray) connectedSelfSpray { return c.buildConnectedSelfSpray(v) })
	return out
}

// buildProductionBuffer 职责：复制productionBuffer的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildProductionBuffer(v productionBuffer) productionBuffer {
	out := v
	out.row = copyFinalizationPointer(c, v.row, func(v transportRow) transportRow { return c.buildTransportRow(v) })
	out.source = copyFinalizationPointer(c, v.source, func(v placed) placed { return c.buildPlaced(v) })
	out.store = copyFinalizationPointer(c, v.store, func(v placed) placed { return c.buildPlaced(v) })
	out.feed = copyFinalizationPointer(c, v.feed, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	out.main = copyFinalizationSlice(c, v.main, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.intake = copyFinalizationSlice(c, v.intake, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.outlet = copyFinalizationSlice(c, v.outlet, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.storeIntake = copyFinalizationSlice(c, v.storeIntake, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.storeOutlet = copyFinalizationSlice(c, v.storeOutlet, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.gates = copyFinalizationSlice(c, v.gates, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.inArms = copyFinalizationSlice(c, v.inArms, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.outArms = copyFinalizationSlice(c, v.outArms, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.entities = copyFinalizationSlice(c, v.entities, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	return out
}

// buildByproductPriorityInput 职责：复制byproductPriorityInput的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildByproductPriorityInput(v byproductPriorityInput) byproductPriorityInput {
	out := v
	out.row = copyFinalizationPointer(c, v.row, func(v transportRow) transportRow { return c.buildTransportRow(v) })
	out.routes = copyFinalizationSlice(c, v.routes, func(v *transportRoute) *transportRoute {
		return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	})
	out.gate = copyFinalizationPointer(c, v.gate, func(v placed) placed { return c.buildPlaced(v) })
	out.local = copyFinalizationSlice(c, v.local, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.fresh = copyFinalizationSlice(c, v.fresh, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.outlet = copyFinalizationSlice(c, v.outlet, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.entities = copyFinalizationSlice(c, v.entities, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.feed = copyFinalizationPointer(c, v.feed, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	return out
}

// buildMode1DeliveryIdentity 职责：复制mode1DeliveryIdentity的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildMode1DeliveryIdentity(v mode1DeliveryIdentity) mode1DeliveryIdentity {
	out := v
	out.edge = c.quantFlowEdge(v.edge)
	out.from = copyFinalizationPointer(c, v.from, func(v transportRow) transportRow { return c.buildTransportRow(v) })
	out.to = copyFinalizationPointer(c, v.to, func(v transportRow) transportRow { return c.buildTransportRow(v) })
	out.first = copyFinalizationPointer(c, v.first, func(v placed) placed { return c.buildPlaced(v) })
	out.start = copyFinalizationPointer(c, v.start, func(v placed) placed { return c.buildPlaced(v) })
	return out
}

// buildMode1DeliveryPlan 职责：复制mode1DeliveryPlan的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildMode1DeliveryPlan(v mode1DeliveryPlan) mode1DeliveryPlan {
	out := v
	out.groups = copyFinalizationSlice(c, v.groups, func(v *mode1DeliveryGroup) *mode1DeliveryGroup {
		return copyFinalizationPointer(c, v, func(v mode1DeliveryGroup) mode1DeliveryGroup { return c.buildMode1DeliveryGroup(v) })
	})
	out.byRoute = copyFinalizationMap(c, v.byRoute, func(v *transportRoute) *transportRoute {
		return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	}, func(v *mode1DeliveryGroup) *mode1DeliveryGroup {
		return copyFinalizationPointer(c, v, func(v mode1DeliveryGroup) mode1DeliveryGroup { return c.buildMode1DeliveryGroup(v) })
	})
	out.identities = copyFinalizationMap(c, v.identities, func(v *transportRoute) *transportRoute {
		return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	}, func(v mode1DeliveryIdentity) mode1DeliveryIdentity { return c.buildMode1DeliveryIdentity(v) })
	out.inputs = copyFinalizationSlice(c, v.inputs, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.edges = copyFinalizationMap(c, v.edges, func(v [2]*placed) [2]*placed {
		return func(v [2]*placed) [2]*placed {
			for i := range v {
				v[i] = copyFinalizationPointer(c, v[i], func(v placed) placed { return c.buildPlaced(v) })
			}
			return v
		}(v)
	}, func(v bool) bool { return v })
	return out
}

// buildDenseBoundaryLayout 职责：复制denseBoundaryLayout的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseBoundaryLayout(v denseBoundaryLayout) denseBoundaryLayout {
	out := v
	out.outputs = copyFinalizationMap(c, v.outputs, func(v *transportRoute) *transportRoute {
		return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	}, func(v vector) vector { return c.buildVector(v) })
	return out
}

// buildDenseProducerSplitLeaf 职责：复制denseProducerSplitLeaf的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseProducerSplitLeaf(v denseProducerSplitLeaf) denseProducerSplitLeaf {
	out := v
	out.group = copyFinalizationPointer(c, v.group, func(v denseProducerSplitGroup) denseProducerSplitGroup { return c.buildDenseProducerSplitGroup(v) })
	out.source = copyFinalizationPointer(c, v.source, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	out.edge = c.quantFlowEdge(v.edge)
	out.from = copyFinalizationPointer(c, v.from, func(v transportRow) transportRow { return c.buildTransportRow(v) })
	out.to = copyFinalizationPointer(c, v.to, func(v transportRow) transportRow { return c.buildTransportRow(v) })
	out.node = copyFinalizationPointer(c, v.node, func(v denseProducerSplitNode) denseProducerSplitNode { return c.buildDenseProducerSplitNode(v) })
	out.prefix = copyFinalizationSlice(c, v.prefix, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	return out
}

// buildDenseProducerSplitNode 职责：复制denseProducerSplitNode的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseProducerSplitNode(v denseProducerSplitNode) denseProducerSplitNode {
	out := v
	out.gate = copyFinalizationPointer(c, v.gate, func(v placed) placed { return c.buildPlaced(v) })
	out.assembly = copyFinalizationPointer(c, v.assembly, func(v splitterGateAssembly) splitterGateAssembly { return c.buildSplitterGateAssembly(v) })
	out.outputSlots = copyFinalizationSlice(c, v.outputSlots, func(v int8) int8 { return v })
	out.entities = copyFinalizationSlice(c, v.entities, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.intake = copyFinalizationSlice(c, v.intake, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.outlets = copyFinalizationSlice(c, v.outlets, func(v []*placed) []*placed {
		return copyFinalizationSlice(c, v, func(v *placed) *placed {
			return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
		})
	})
	out.children = copyFinalizationSlice(c, v.children, func(v *denseProducerSplitNode) *denseProducerSplitNode {
		return copyFinalizationPointer(c, v, func(v denseProducerSplitNode) denseProducerSplitNode { return c.buildDenseProducerSplitNode(v) })
	})
	out.links = copyFinalizationSlice(c, v.links, func(v *transportRoute) *transportRoute {
		return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	})
	out.leaves = copyFinalizationSlice(c, v.leaves, func(v *denseProducerSplitLeaf) *denseProducerSplitLeaf {
		return copyFinalizationPointer(c, v, func(v denseProducerSplitLeaf) denseProducerSplitLeaf { return c.buildDenseProducerSplitLeaf(v) })
	})
	return out
}

// buildDenseProducerSplitGroup 职责：复制denseProducerSplitGroup的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseProducerSplitGroup(v denseProducerSplitGroup) denseProducerSplitGroup {
	out := v
	out.row = copyFinalizationPointer(c, v.row, func(v transportRow) transportRow { return c.buildTransportRow(v) })
	out.head = copyFinalizationPointer(c, v.head, func(v placed) placed { return c.buildPlaced(v) })
	out.root = copyFinalizationPointer(c, v.root, func(v denseProducerSplitNode) denseProducerSplitNode { return c.buildDenseProducerSplitNode(v) })
	out.feed = copyFinalizationPointer(c, v.feed, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	out.leaves = copyFinalizationSlice(c, v.leaves, func(v *denseProducerSplitLeaf) *denseProducerSplitLeaf {
		return copyFinalizationPointer(c, v, func(v denseProducerSplitLeaf) denseProducerSplitLeaf { return c.buildDenseProducerSplitLeaf(v) })
	})
	out.feeds = copyFinalizationSlice(c, v.feeds, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	return out
}

// buildDenseProducerRouteIdentity 职责：复制denseProducerRouteIdentity的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseProducerRouteIdentity(v denseProducerRouteIdentity) denseProducerRouteIdentity {
	out := v
	out.leaf = copyFinalizationPointer(c, v.leaf, func(v denseProducerSplitLeaf) denseProducerSplitLeaf { return c.buildDenseProducerSplitLeaf(v) })
	out.edge = c.quantFlowEdge(v.edge)
	out.from = copyFinalizationPointer(c, v.from, func(v transportRow) transportRow { return c.buildTransportRow(v) })
	out.to = copyFinalizationPointer(c, v.to, func(v transportRow) transportRow { return c.buildTransportRow(v) })
	return out
}

// buildDenseProducerSplitPlan 职责：复制denseProducerSplitPlan的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseProducerSplitPlan(v denseProducerSplitPlan) denseProducerSplitPlan {
	out := v
	out.groups = copyFinalizationSlice(c, v.groups, func(v *denseProducerSplitGroup) *denseProducerSplitGroup {
		return copyFinalizationPointer(c, v, func(v denseProducerSplitGroup) denseProducerSplitGroup { return c.buildDenseProducerSplitGroup(v) })
	})
	out.bySource = copyFinalizationMap(c, v.bySource, func(v *transportRoute) *transportRoute {
		return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	}, func(v *denseProducerSplitLeaf) *denseProducerSplitLeaf {
		return copyFinalizationPointer(c, v, func(v denseProducerSplitLeaf) denseProducerSplitLeaf { return c.buildDenseProducerSplitLeaf(v) })
	})
	out.wires = copyFinalizationSlice(c, v.wires, func(v *transportRoute) *transportRoute {
		return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	})
	out.identities = copyFinalizationMap(c, v.identities, func(v *transportRoute) *transportRoute {
		return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	}, func(v denseProducerRouteIdentity) denseProducerRouteIdentity {
		return c.buildDenseProducerRouteIdentity(v)
	})
	return out
}

// buildDenseStationOutputGroup 职责：复制denseStationOutputGroup的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseStationOutputGroup(v denseStationOutputGroup) denseStationOutputGroup {
	out := v
	out.members = copyFinalizationSlice(c, v.members, func(v *transportRoute) *transportRoute {
		return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	})
	out.representative = copyFinalizationPointer(c, v.representative, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	out.sources = copyFinalizationMap(c, v.sources, func(v *transportRow) *transportRow {
		return copyFinalizationPointer(c, v, func(v transportRow) transportRow { return c.buildTransportRow(v) })
	}, func(v bool) bool { return v })
	out.head = copyFinalizationPointer(c, v.head, func(v placed) placed { return c.buildPlaced(v) })
	out.tower = copyFinalizationPointer(c, v.tower, func(v placed) placed { return c.buildPlaced(v) })
	return out
}

// buildDenseStationOutputPlan 职责：复制denseStationOutputPlan的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseStationOutputPlan(v denseStationOutputPlan) denseStationOutputPlan {
	out := v
	out.byRoute = copyFinalizationMap(c, v.byRoute, func(v *transportRoute) *transportRoute {
		return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	}, func(v *denseStationOutputGroup) *denseStationOutputGroup {
		return copyFinalizationPointer(c, v, func(v denseStationOutputGroup) denseStationOutputGroup { return c.buildDenseStationOutputGroup(v) })
	})
	out.byRepresentative = copyFinalizationMap(c, v.byRepresentative, func(v *transportRoute) *transportRoute {
		return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	}, func(v *denseStationOutputGroup) *denseStationOutputGroup {
		return copyFinalizationPointer(c, v, func(v denseStationOutputGroup) denseStationOutputGroup { return c.buildDenseStationOutputGroup(v) })
	})
	out.groups = copyFinalizationSlice(c, v.groups, func(v *denseStationOutputGroup) *denseStationOutputGroup {
		return copyFinalizationPointer(c, v, func(v denseStationOutputGroup) denseStationOutputGroup { return c.buildDenseStationOutputGroup(v) })
	})
	return out
}

// buildDensePolarFrame 职责：复制densePolarFrame的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDensePolarFrame(v densePolarFrame) densePolarFrame {
	out := v
	out.position = c.copyString(v.position)
	return out
}

// buildDenseStreetAccessory 职责：复制denseStreetAccessory的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseStreetAccessory(v denseStreetAccessory) denseStreetAccessory {
	out := v
	out.entities = copyFinalizationSlice(c, v.entities, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.head = copyFinalizationPointer(c, v.head, func(v placed) placed { return c.buildPlaced(v) })
	out.inner = copyFinalizationPointer(c, v.inner, func(v placed) placed { return c.buildPlaced(v) })
	out.join = copyFinalizationPointer(c, v.join, func(v placed) placed { return c.buildPlaced(v) })
	out.joinInner = copyFinalizationPointer(c, v.joinInner, func(v placed) placed { return c.buildPlaced(v) })
	out.coater = copyFinalizationPointer(c, v.coater, func(v placed) placed { return c.buildPlaced(v) })
	out.supplyHead = copyFinalizationPointer(c, v.supplyHead, func(v placed) placed { return c.buildPlaced(v) })
	out.supplyTail = copyFinalizationPointer(c, v.supplyTail, func(v placed) placed { return c.buildPlaced(v) })
	out.supplyInner = copyFinalizationPointer(c, v.supplyInner, func(v placed) placed { return c.buildPlaced(v) })
	out.material = copyFinalizationSlice(c, v.material, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.core = copyFinalizationSlice(c, v.core, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	return out
}

// buildDenseStreetPort 职责：复制denseStreetPort的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseStreetPort(v denseStreetPort) denseStreetPort {
	out := v
	out.head = copyFinalizationPointer(c, v.head, func(v placed) placed { return c.buildPlaced(v) })
	out.inner = copyFinalizationPointer(c, v.inner, func(v placed) placed { return c.buildPlaced(v) })
	return out
}

// buildDenseStreetSeam 职责：复制denseStreetSeam的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseStreetSeam(v denseStreetSeam) denseStreetSeam {
	out := v
	return out
}

// buildLogisticsCheck 职责：复制LogisticsCheck的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildLogisticsCheck(v LogisticsCheck) LogisticsCheck {
	out := v
	out.RuleID = c.copyString(v.RuleID)
	out.Status = c.copyString(v.Status)
	out.Message = c.copyString(v.Message)
	return out
}

// buildDenseStreet 职责：复制denseStreet的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseStreet(v denseStreet) denseStreet {
	out := v
	out.groups = copyFinalizationSlice(c, v.groups, func(v *line) *line { return copyFinalizationPointer(c, v, func(v line) line { return c.buildLine(v) }) })
	out.all = copyFinalizationSlice(c, v.all, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.ports = copyFinalizationSlice(c, v.ports, func(v denseStreetPort) denseStreetPort { return c.buildDenseStreetPort(v) })
	out.seams = copyFinalizationSlice(c, v.seams, func(v denseStreetSeam) denseStreetSeam { return c.buildDenseStreetSeam(v) })
	out.checks = copyFinalizationSlice(c, v.checks, func(v LogisticsCheck) LogisticsCheck { return c.buildLogisticsCheck(v) })
	out.accessories = copyFinalizationSlice(c, v.accessories, func(v *denseStreetAccessory) *denseStreetAccessory {
		return copyFinalizationPointer(c, v, func(v denseStreetAccessory) denseStreetAccessory { return c.buildDenseStreetAccessory(v) })
	})
	return out
}

// buildDenseStreetAccessoryLink 职责：复制denseStreetAccessoryLink的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseStreetAccessoryLink(v denseStreetAccessoryLink) denseStreetAccessoryLink {
	out := v
	out.purpose = c.copyString(v.purpose)
	out.start = copyFinalizationPointer(c, v.start, func(v placed) placed { return c.buildPlaced(v) })
	out.end = copyFinalizationPointer(c, v.end, func(v placed) placed { return c.buildPlaced(v) })
	out.path = copyFinalizationSlice(c, v.path, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.accessory = copyFinalizationPointer(c, v.accessory, func(v denseStreetAccessory) denseStreetAccessory { return c.buildDenseStreetAccessory(v) })
	return out
}

// buildDenseStreetAssembly 职责：复制denseStreetAssembly的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseStreetAssembly(v denseStreetAssembly) denseStreetAssembly {
	out := v
	out.street = copyFinalizationPointer(c, v.street, func(v denseStreet) denseStreet { return c.buildDenseStreet(v) })
	out.all = copyFinalizationSlice(c, v.all, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.accessories = copyFinalizationSlice(c, v.accessories, func(v *denseStreetAccessory) *denseStreetAccessory {
		return copyFinalizationPointer(c, v, func(v denseStreetAccessory) denseStreetAccessory { return c.buildDenseStreetAccessory(v) })
	})
	out.ports = copyFinalizationSlice(c, v.ports, func(v denseStreetPort) denseStreetPort { return c.buildDenseStreetPort(v) })
	out.sprayPorts = copyFinalizationSlice(c, v.sprayPorts, func(v denseStreetPort) denseStreetPort { return c.buildDenseStreetPort(v) })
	out.links = copyFinalizationSlice(c, v.links, func(v *denseStreetAccessoryLink) *denseStreetAccessoryLink {
		return copyFinalizationPointer(c, v, func(v denseStreetAccessoryLink) denseStreetAccessoryLink { return c.buildDenseStreetAccessoryLink(v) })
	})
	return out
}

// buildDenseStationSprayMember 职责：复制denseStationSprayMember的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseStationSprayMember(v denseStationSprayMember) denseStationSprayMember {
	out := v
	out.route = copyFinalizationPointer(c, v.route, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	out.entry = copyFinalizationPointer(c, v.entry, func(v placed) placed { return c.buildPlaced(v) })
	out.tail = copyFinalizationPointer(c, v.tail, func(v placed) placed { return c.buildPlaced(v) })
	out.exit = copyFinalizationPointer(c, v.exit, func(v placed) placed { return c.buildPlaced(v) })
	out.module = copyFinalizationPointer(c, v.module, func(v denseStreetAccessory) denseStreetAccessory { return c.buildDenseStreetAccessory(v) })
	out.assembly = copyFinalizationPointer(c, v.assembly, func(v denseStreetAssembly) denseStreetAssembly { return c.buildDenseStreetAssembly(v) })
	return out
}

// buildDenseStationSprayGroup 职责：复制denseStationSprayGroup的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseStationSprayGroup(v denseStationSprayGroup) denseStationSprayGroup {
	out := v
	out.members = copyFinalizationSlice(c, v.members, func(v *denseStationSprayMember) *denseStationSprayMember {
		return copyFinalizationPointer(c, v, func(v denseStationSprayMember) denseStationSprayMember { return c.buildDenseStationSprayMember(v) })
	})
	out.physical = copyFinalizationPointer(c, v.physical, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	out.chain = copyFinalizationSlice(c, v.chain, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	return out
}

// buildDenseStationSprayPlan 职责：复制denseStationSprayPlan的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseStationSprayPlan(v denseStationSprayPlan) denseStationSprayPlan {
	out := v
	out.groups = copyFinalizationSlice(c, v.groups, func(v *denseStationSprayGroup) *denseStationSprayGroup {
		return copyFinalizationPointer(c, v, func(v denseStationSprayGroup) denseStationSprayGroup { return c.buildDenseStationSprayGroup(v) })
	})
	out.pending = copyFinalizationMap(c, v.pending, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	}, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.byRoute = copyFinalizationMap(c, v.byRoute, func(v *transportRoute) *transportRoute {
		return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	}, func(v *denseStationSprayGroup) *denseStationSprayGroup {
		return copyFinalizationPointer(c, v, func(v denseStationSprayGroup) denseStationSprayGroup { return c.buildDenseStationSprayGroup(v) })
	})
	return out
}

// buildDenseMaterialTap 职责：复制denseMaterialTap的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseMaterialTap(v denseMaterialTap) denseMaterialTap {
	out := v
	out.route = copyFinalizationPointer(c, v.route, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	out.filters = copyFinalizationSlice(c, v.filters, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.mainPrefix = copyFinalizationSlice(c, v.mainPrefix, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	return out
}

// buildDenseInputSplitBranch 职责：复制denseInputSplitBranch的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseInputSplitBranch(v denseInputSplitBranch) denseInputSplitBranch {
	out := v
	out.source = copyFinalizationPointer(c, v.source, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	out.route = copyFinalizationPointer(c, v.route, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	out.prefix = copyFinalizationSlice(c, v.prefix, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.contributions = copyFinalizationMap(c, v.contributions, func(v *transportRoute) *transportRoute {
		return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	}, func(v *transportRoute) *transportRoute {
		return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	})
	return out
}

// buildDenseInputSlice 职责：复制denseInputSlice的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseInputSlice(v denseInputSlice) denseInputSlice {
	out := v
	out.source = copyFinalizationPointer(c, v.source, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	out.branches = copyFinalizationSlice(c, v.branches, func(v *denseInputSplitBranch) *denseInputSplitBranch {
		return copyFinalizationPointer(c, v, func(v denseInputSplitBranch) denseInputSplitBranch { return c.buildDenseInputSplitBranch(v) })
	})
	out.head = copyFinalizationPointer(c, v.head, func(v placed) placed { return c.buildPlaced(v) })
	out.modules = copyFinalizationSlice(c, v.modules, func(v *splitterGateAssembly) *splitterGateAssembly {
		return copyFinalizationPointer(c, v, func(v splitterGateAssembly) splitterGateAssembly { return c.buildSplitterGateAssembly(v) })
	})
	out.joins = copyFinalizationSlice(c, v.joins, func(v *transportRoute) *transportRoute {
		return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	})
	return out
}

// buildDenseInputConsumer 职责：复制denseInputConsumer的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseInputConsumer(v denseInputConsumer) denseInputConsumer {
	out := v
	out.head = copyFinalizationPointer(c, v.head, func(v placed) placed { return c.buildPlaced(v) })
	out.buffer = copyFinalizationSlice(c, v.buffer, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.route = copyFinalizationPointer(c, v.route, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	return out
}

// buildDenseInputSplitNetwork 职责：复制denseInputSplitNetwork的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseInputSplitNetwork(v denseInputSplitNetwork) denseInputSplitNetwork {
	out := v
	out.gates = copyFinalizationSlice(c, v.gates, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.modules = copyFinalizationSlice(c, v.modules, func(v *splitterGateAssembly) *splitterGateAssembly {
		return copyFinalizationPointer(c, v, func(v splitterGateAssembly) splitterGateAssembly { return c.buildSplitterGateAssembly(v) })
	})
	out.branches = copyFinalizationSlice(c, v.branches, func(v *denseInputSplitBranch) *denseInputSplitBranch {
		return copyFinalizationPointer(c, v, func(v denseInputSplitBranch) denseInputSplitBranch { return c.buildDenseInputSplitBranch(v) })
	})
	out.slices = copyFinalizationSlice(c, v.slices, func(v *denseInputSlice) *denseInputSlice {
		return copyFinalizationPointer(c, v, func(v denseInputSlice) denseInputSlice { return c.buildDenseInputSlice(v) })
	})
	out.bySource = copyFinalizationMap(c, v.bySource, func(v *transportRoute) *transportRoute {
		return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	}, func(v *denseInputSlice) *denseInputSlice {
		return copyFinalizationPointer(c, v, func(v denseInputSlice) denseInputSlice { return c.buildDenseInputSlice(v) })
	})
	out.consumers = copyFinalizationSlice(c, v.consumers, func(v *denseInputConsumer) *denseInputConsumer {
		return copyFinalizationPointer(c, v, func(v denseInputConsumer) denseInputConsumer { return c.buildDenseInputConsumer(v) })
	})
	return out
}

// buildDenseMaterialNetwork 职责：复制denseMaterialNetwork的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseMaterialNetwork(v denseMaterialNetwork) denseMaterialNetwork {
	out := v
	out.rows = copyFinalizationSlice(c, v.rows, func(v *transportRow) *transportRow {
		return copyFinalizationPointer(c, v, func(v transportRow) transportRow { return c.buildTransportRow(v) })
	})
	out.planned = copyFinalizationSlice(c, v.planned, func(v float64) float64 { return v })
	out.configured = copyFinalizationSlice(c, v.configured, func(v float64) float64 { return v })
	out.entries = copyFinalizationSlice(c, v.entries, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.exits = copyFinalizationSlice(c, v.exits, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.predecessors = copyFinalizationSlice(c, v.predecessors, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.tracks = copyFinalizationSlice(c, v.tracks, func(v []*placed) []*placed {
		return copyFinalizationSlice(c, v, func(v *placed) *placed {
			return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
		})
	})
	out.joins = copyFinalizationSlice(c, v.joins, func(v *transportRoute) *transportRoute {
		return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	})
	out.intake = copyFinalizationSlice(c, v.intake, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.intakeRoute = copyFinalizationPointer(c, v.intakeRoute, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	out.supplyHead = copyFinalizationPointer(c, v.supplyHead, func(v placed) placed { return c.buildPlaced(v) })
	out.taps = copyFinalizationSlice(c, v.taps, func(v *denseMaterialTap) *denseMaterialTap {
		return copyFinalizationPointer(c, v, func(v denseMaterialTap) denseMaterialTap { return c.buildDenseMaterialTap(v) })
	})
	out.splitInput = copyFinalizationPointer(c, v.splitInput, func(v denseInputSplitNetwork) denseInputSplitNetwork { return c.buildDenseInputSplitNetwork(v) })
	return out
}

// buildDenseMaterialNetworkBinding 职责：复制denseMaterialNetworkBinding的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseMaterialNetworkBinding(v denseMaterialNetworkBinding) denseMaterialNetworkBinding {
	out := v
	out.group = copyFinalizationPointer(c, v.group, func(v denseMaterialNetwork) denseMaterialNetwork { return c.buildDenseMaterialNetwork(v) })
	out.source = copyFinalizationPointer(c, v.source, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	return out
}

// buildDenseMaterialNetworkPlan 职责：复制denseMaterialNetworkPlan的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseMaterialNetworkPlan(v denseMaterialNetworkPlan) denseMaterialNetworkPlan {
	out := v
	out.canonical = copyFinalizationSlice(c, v.canonical, func(v *transportRoute) *transportRoute {
		return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	})
	out.physical = copyFinalizationSlice(c, v.physical, func(v *transportRoute) *transportRoute {
		return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	})
	out.groups = copyFinalizationSlice(c, v.groups, func(v *denseMaterialNetwork) *denseMaterialNetwork {
		return copyFinalizationPointer(c, v, func(v denseMaterialNetwork) denseMaterialNetwork { return c.buildDenseMaterialNetwork(v) })
	})
	out.bindings = copyFinalizationMap(c, v.bindings, func(v *transportRoute) *transportRoute {
		return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	}, func(v denseMaterialNetworkBinding) denseMaterialNetworkBinding {
		return c.buildDenseMaterialNetworkBinding(v)
	})
	out.members = copyFinalizationMap(c, v.members, func(v *transportRoute) *transportRoute {
		return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	}, func(v []*transportRoute) []*transportRoute {
		return copyFinalizationSlice(c, v, func(v *transportRoute) *transportRoute {
			return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
		})
	})
	out.originalOutputs = copyFinalizationMap(c, v.originalOutputs, func(v *transportRow) *transportRow {
		return copyFinalizationPointer(c, v, func(v transportRow) transportRow { return c.buildTransportRow(v) })
	}, func(v []*transportRoute) []*transportRoute {
		return copyFinalizationSlice(c, v, func(v *transportRoute) *transportRoute {
			return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
		})
	})
	out.physicalOutputs = copyFinalizationMap(c, v.physicalOutputs, func(v *transportRow) *transportRow {
		return copyFinalizationPointer(c, v, func(v transportRow) transportRow { return c.buildTransportRow(v) })
	}, func(v []*transportRoute) []*transportRoute {
		return copyFinalizationSlice(c, v, func(v *transportRoute) *transportRoute {
			return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
		})
	})
	out.joins = copyFinalizationSlice(c, v.joins, func(v *transportRoute) *transportRoute {
		return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	})
	return out
}

// buildDenseFlyBoundaryStore 职责：复制denseFlyBoundaryStore的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseFlyBoundaryStore(v denseFlyBoundaryStore) denseFlyBoundaryStore {
	out := v
	out.box = copyFinalizationPointer(c, v.box, func(v placed) placed { return c.buildPlaced(v) })
	out.delivery = copyFinalizationPointer(c, v.delivery, func(v placed) placed { return c.buildPlaced(v) })
	out.ports = copyFinalizationSlice(c, v.ports, func(v *denseFlyBoundaryFacility) *denseFlyBoundaryFacility {
		return copyFinalizationPointer(c, v, func(v denseFlyBoundaryFacility) denseFlyBoundaryFacility { return c.buildDenseFlyBoundaryFacility(v) })
	})
	out.entities = copyFinalizationSlice(c, v.entities, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.spatialPorts = copyFinalizationSlice(c, v.spatialPorts, func(v denseStreetPort) denseStreetPort { return c.buildDenseStreetPort(v) })
	return out
}

// buildFlySelfSpray 职责：复制flySelfSpray的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildFlySelfSpray(v flySelfSpray) flySelfSpray {
	out := v
	out.line = copyFinalizationPointer(c, v.line, func(v line) line { return c.buildLine(v) })
	out.arm = copyFinalizationPointer(c, v.arm, func(v placed) placed { return c.buildPlaced(v) })
	out.path = copyFinalizationSlice(c, v.path, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	return out
}

// buildFlyTransportEndpoint 职责：复制flyTransportEndpoint的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildFlyTransportEndpoint(v flyTransportEndpoint) flyTransportEndpoint {
	out := v
	out.flowEdgeID = c.copyString(v.flowEdgeID)
	out.leftoverAnchor = copyFinalizationPointer(c, v.leftoverAnchor, func(v placed) placed { return c.buildPlaced(v) })
	out.link = copyFinalizationPointer(c, v.link, func(v placed) placed { return c.buildPlaced(v) })
	out.belts = copyFinalizationSlice(c, v.belts, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.box = copyFinalizationPointer(c, v.box, func(v placed) placed { return c.buildPlaced(v) })
	out.delivery = copyFinalizationPointer(c, v.delivery, func(v placed) placed { return c.buildPlaced(v) })
	out.arm = copyFinalizationPointer(c, v.arm, func(v placed) placed { return c.buildPlaced(v) })
	out.arms = copyFinalizationSlice(c, v.arms, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.filters = copyFinalizationSlice(c, v.filters, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.splitter = copyFinalizationPointer(c, v.splitter, func(v placed) placed { return c.buildPlaced(v) })
	out.selfSpray = copyFinalizationPointer(c, v.selfSpray, func(v flySelfSpray) flySelfSpray { return c.buildFlySelfSpray(v) })
	return out
}

// buildDenseFlyBoundaryFacility 职责：复制denseFlyBoundaryFacility的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseFlyBoundaryFacility(v denseFlyBoundaryFacility) denseFlyBoundaryFacility {
	out := v
	out.store = copyFinalizationPointer(c, v.store, func(v denseFlyBoundaryStore) denseFlyBoundaryStore { return c.buildDenseFlyBoundaryStore(v) })
	out.endpoint = copyFinalizationPointer(c, v.endpoint, func(v flyTransportEndpoint) flyTransportEndpoint { return c.buildFlyTransportEndpoint(v) })
	out.routes = copyFinalizationSlice(c, v.routes, func(v *transportRoute) *transportRoute {
		return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	})
	out.entities = copyFinalizationSlice(c, v.entities, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.branches = copyFinalizationMap(c, v.branches, func(v *transportRoute) *transportRoute {
		return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	}, func(v []*placed) []*placed {
		return copyFinalizationSlice(c, v, func(v *placed) *placed {
			return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
		})
	})
	out.monitorHeads = copyFinalizationMap(c, v.monitorHeads, func(v *transportRoute) *transportRoute {
		return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	}, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	return out
}

// buildDenseFlyBlackboxState 职责：复制denseFlyBlackboxState的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseFlyBlackboxState(v denseFlyBlackboxState) denseFlyBlackboxState {
	out := v
	out.facilities = copyFinalizationSlice(c, v.facilities, func(v *denseFlyBoundaryFacility) *denseFlyBoundaryFacility {
		return copyFinalizationPointer(c, v, func(v denseFlyBoundaryFacility) denseFlyBoundaryFacility { return c.buildDenseFlyBoundaryFacility(v) })
	})
	out.stores = copyFinalizationSlice(c, v.stores, func(v *denseFlyBoundaryStore) *denseFlyBoundaryStore {
		return copyFinalizationPointer(c, v, func(v denseFlyBoundaryStore) denseFlyBoundaryStore { return c.buildDenseFlyBoundaryStore(v) })
	})
	return out
}

// buildDenseConnectedStreetFeedback 职责：复制denseConnectedStreetFeedback的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseConnectedStreetFeedback(v denseConnectedStreetFeedback) denseConnectedStreetFeedback {
	out := v
	out.row = copyFinalizationPointer(c, v.row, func(v transportRow) transportRow { return c.buildTransportRow(v) })
	out.source = copyFinalizationPointer(c, v.source, func(v placed) placed { return c.buildPlaced(v) })
	out.arm = copyFinalizationPointer(c, v.arm, func(v placed) placed { return c.buildPlaced(v) })
	out.prefix = copyFinalizationSlice(c, v.prefix, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.wire = copyFinalizationPointer(c, v.wire, func(v denseStreetAccessoryLink) denseStreetAccessoryLink { return c.buildDenseStreetAccessoryLink(v) })
	return out
}

// buildDenseConnectedStreetWork 职责：复制denseConnectedStreetWork的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseConnectedStreetWork(v denseConnectedStreetWork) denseConnectedStreetWork {
	out := v
	out.assembly = copyFinalizationPointer(c, v.assembly, func(v denseStreetAssembly) denseStreetAssembly { return c.buildDenseStreetAssembly(v) })
	out.rows = copyFinalizationSlice(c, v.rows, func(v *transportRow) *transportRow {
		return copyFinalizationPointer(c, v, func(v transportRow) transportRow { return c.buildTransportRow(v) })
	})
	out.routes = copyFinalizationSlice(c, v.routes, func(v *transportRoute) *transportRoute {
		return copyFinalizationPointer(c, v, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	})
	out.feedbacks = copyFinalizationSlice(c, v.feedbacks, func(v *denseConnectedStreetFeedback) *denseConnectedStreetFeedback {
		return copyFinalizationPointer(c, v, func(v denseConnectedStreetFeedback) denseConnectedStreetFeedback {
			return c.buildDenseConnectedStreetFeedback(v)
		})
	})
	return out
}

// buildSetting 职责：复制Setting的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildSetting(v Setting) Setting {
	out := v
	out.NeedPower = copyFinalizationPointer(c, v.NeedPower, func(v bool) bool { return v })
	out.Mix2 = copyFinalizationPointer(c, v.Mix2, func(v bool) bool { return v })
	return out
}

// quantFlowNode 职责：复制FlowNode的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) quantFlowNode(v quant.FlowNode) quant.FlowNode {
	out := v
	out.ID = c.copyString(v.ID)
	out.Kind = c.copyString(v.Kind)
	out.ComponentID = c.copyString(v.ComponentID)
	return out
}

// quantFlowComponent 职责：复制FlowComponent的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) quantFlowComponent(v quant.FlowComponent) quant.FlowComponent {
	out := v
	out.ID = c.copyString(v.ID)
	out.NodeIDs = copyFinalizationSlice(c, v.NodeIDs, func(v string) string { return c.copyString(v) })
	return out
}

// quantFlowBalance 职责：复制FlowBalance的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) quantFlowBalance(v quant.FlowBalance) quant.FlowBalance {
	out := v
	return out
}

// quantFlowGraph 职责：复制FlowGraph的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) quantFlowGraph(v quant.FlowGraph) quant.FlowGraph {
	out := v
	out.Nodes = copyFinalizationSlice(c, v.Nodes, func(v quant.FlowNode) quant.FlowNode { return c.quantFlowNode(v) })
	out.Edges = copyFinalizationSlice(c, v.Edges, func(v quant.FlowEdge) quant.FlowEdge { return c.quantFlowEdge(v) })
	out.Components = copyFinalizationSlice(c, v.Components, func(v quant.FlowComponent) quant.FlowComponent { return c.quantFlowComponent(v) })
	out.ProductionOrder = copyFinalizationSlice(c, v.ProductionOrder, func(v int) int { return v })
	out.Balances = copyFinalizationSlice(c, v.Balances, func(v quant.FlowBalance) quant.FlowBalance { return c.quantFlowBalance(v) })
	out.Digest = c.copyString(v.Digest)
	return out
}

// buildTransportLane 职责：复制TransportLane的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildTransportLane(v TransportLane) TransportLane {
	out := v
	out.EdgeID = c.copyString(v.EdgeID)
	out.SourceNodeID = c.copyString(v.SourceNodeID)
	out.TargetNodeID = c.copyString(v.TargetNodeID)
	out.FlowKind = c.copyString(v.FlowKind)
	out.BuildingIndices = copyFinalizationSlice(c, v.BuildingIndices, func(v int) int { return v })
	return out
}

// buildBoundaryPort 职责：复制BoundaryPort的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildBoundaryPort(v BoundaryPort) BoundaryPort {
	out := v
	out.EdgeID = c.copyString(v.EdgeID)
	out.Kind = c.copyString(v.Kind)
	return out
}

// buildLayoutMetrics 职责：复制LayoutMetrics的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildLayoutMetrics(v LayoutMetrics) LayoutMetrics {
	out := v
	return out
}

// buildSearchCandidateDiagnostic 职责：复制SearchCandidateDiagnostic的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildSearchCandidateDiagnostic(v SearchCandidateDiagnostic) SearchCandidateDiagnostic {
	out := v
	out.ParentKey = c.copyString(v.ParentKey)
	out.Key = c.copyString(v.Key)
	out.Parameters = c.copyString(v.Parameters)
	out.Scope = c.copyString(v.Scope)
	out.Stage = c.copyString(v.Stage)
	out.Status = c.copyString(v.Status)
	out.ErrorCode = c.copyString(v.ErrorCode)
	out.Reason = c.copyString(v.Reason)
	return out
}

// buildLogisticsSummary 职责：复制LogisticsSummary的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildLogisticsSummary(v LogisticsSummary) LogisticsSummary {
	out := v
	out.FlowGraphDigest = c.copyString(v.FlowGraphDigest)
	out.Lanes = copyFinalizationSlice(c, v.Lanes, func(v TransportLane) TransportLane { return c.buildTransportLane(v) })
	out.ExternalPorts = copyFinalizationSlice(c, v.ExternalPorts, func(v BoundaryPort) BoundaryPort { return c.buildBoundaryPort(v) })
	out.RateAnnotations = copyFinalizationSlice(c, v.RateAnnotations, func(v ProductionRateAnnotation) ProductionRateAnnotation { return c.buildProductionRateAnnotation(v) })
	out.Checks = copyFinalizationSlice(c, v.Checks, func(v LogisticsCheck) LogisticsCheck { return c.buildLogisticsCheck(v) })
	out.Metrics = c.buildLayoutMetrics(v.Metrics)
	out.SearchCandidates = copyFinalizationSlice(c, v.SearchCandidates, func(v SearchCandidateDiagnostic) SearchCandidateDiagnostic {
		return c.buildSearchCandidateDiagnostic(v)
	})
	return out
}

// buildStationTransportLink 职责：复制stationTransportLink的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildStationTransportLink(v stationTransportLink) stationTransportLink {
	out := v
	out.tower = copyFinalizationPointer(c, v.tower, func(v placed) placed { return c.buildPlaced(v) })
	out.belt = copyFinalizationPointer(c, v.belt, func(v placed) placed { return c.buildPlaced(v) })
	out.head = copyFinalizationPointer(c, v.head, func(v placed) placed { return c.buildPlaced(v) })
	out.path = copyFinalizationSlice(c, v.path, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.sprayRecycle = copyFinalizationPointer(c, v.sprayRecycle, func(v line) line { return c.buildLine(v) })
	return out
}

// buildStationLeftoverReturn 职责：复制stationLeftoverReturn的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildStationLeftoverReturn(v stationLeftoverReturn) stationLeftoverReturn {
	out := v
	out.edge = c.quantFlowEdge(v.edge)
	out.tower = copyFinalizationPointer(c, v.tower, func(v placed) placed { return c.buildPlaced(v) })
	out.sourceBelt = copyFinalizationPointer(c, v.sourceBelt, func(v placed) placed { return c.buildPlaced(v) })
	out.rawHead = copyFinalizationPointer(c, v.rawHead, func(v placed) placed { return c.buildPlaced(v) })
	out.path = copyFinalizationSlice(c, v.path, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.offset = c.buildVector(v.offset)
	return out
}

// buildBusTransportLink 职责：复制busTransportLink的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildBusTransportLink(v busTransportLink) busTransportLink {
	out := v
	out.kind = c.copyString(v.kind)
	out.endpoint = copyFinalizationPointer(c, v.endpoint, func(v placed) placed { return c.buildPlaced(v) })
	out.ring = copyFinalizationPointer(c, v.ring, func(v placed) placed { return c.buildPlaced(v) })
	out.ringPath = copyFinalizationSlice(c, v.ringPath, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.parts = copyFinalizationSlice(c, v.parts, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.limiterParts = copyFinalizationSlice(c, v.limiterParts, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.gateFeed = copyFinalizationSlice(c, v.gateFeed, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.gateReturn = copyFinalizationSlice(c, v.gateReturn, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.gateEndpoint = copyFinalizationPointer(c, v.gateEndpoint, func(v placed) placed { return c.buildPlaced(v) })
	out.itemRates = copyFinalizationSlice(c, v.itemRates, func(v quant.ItemRate) quant.ItemRate { return c.quantItemRate(v) })
	out.row = copyFinalizationSlice(c, v.row, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	return out
}

// buildBusFabricRoute 职责：复制busFabricRoute的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildBusFabricRoute(v busFabricRoute) busFabricRoute {
	out := v
	out.route = copyFinalizationPointer(c, v.route, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	out.source = copyFinalizationPointer(c, v.source, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	out.target = copyFinalizationPointer(c, v.target, func(v transportRoute) transportRoute { return c.buildTransportRoute(v) })
	out.feed = copyFinalizationSlice(c, v.feed, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.take = copyFinalizationSlice(c, v.take, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	out.recycle = copyFinalizationSlice(c, v.recycle, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	})
	return out
}

// candidatesStage 职责：复制Stage的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) candidatesStage(v candidates.Stage) candidates.Stage {
	out := v
	out.Scope = c.copyString(v.Scope)
	return out
}

// candidatesCounts 职责：复制Counts的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) candidatesCounts(v candidates.Counts) candidates.Counts {
	out := v
	return out
}

// candidatesReport 职责：复制Report的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) candidatesReport(v candidates.Report) candidates.Report {
	out := v
	out.Stages = copyFinalizationSlice(c, v.Stages, func(v candidates.Stage) candidates.Stage { return c.candidatesStage(v) })
	out.FinalRules = c.candidatesCounts(v.FinalRules)
	return out
}

// buildPerformanceStage 职责：复制PerformanceStage的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildPerformanceStage(v PerformanceStage) PerformanceStage {
	out := v
	out.Stage = c.copyString(v.Stage)
	return out
}

// buildPerformanceCandidate 职责：复制PerformanceCandidate的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildPerformanceCandidate(v PerformanceCandidate) PerformanceCandidate {
	out := v
	out.ID = c.copyString(v.ID)
	out.Stages = copyFinalizationSlice(c, v.Stages, func(v PerformanceStage) PerformanceStage { return c.buildPerformanceStage(v) })
	return out
}

// buildPerformanceFlow 职责：复制PerformanceFlow的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildPerformanceFlow(v PerformanceFlow) PerformanceFlow {
	out := v
	return out
}

// buildPerformanceCandidates 职责：复制PerformanceCandidates的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildPerformanceCandidates(v PerformanceCandidates) PerformanceCandidates {
	out := v
	out.Stage = c.copyString(v.Stage)
	return out
}

// buildPerformanceRouting 职责：复制PerformanceRouting的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildPerformanceRouting(v PerformanceRouting) PerformanceRouting {
	out := v
	return out
}

// buildPerformanceCollision 职责：复制PerformanceCollision的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildPerformanceCollision(v PerformanceCollision) PerformanceCollision {
	out := v
	out.Rule = c.copyString(v.Rule)
	return out
}

// buildGenerationPerformance 职责：复制GenerationPerformance的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildGenerationPerformance(v GenerationPerformance) GenerationPerformance {
	out := v
	out.CandidateDeduplication = copyFinalizationPointer(c, v.CandidateDeduplication, func(v candidates.Report) candidates.Report { return c.candidatesReport(v) })
	out.CandidateDetails = copyFinalizationSlice(c, v.CandidateDetails, func(v PerformanceCandidate) PerformanceCandidate { return c.buildPerformanceCandidate(v) })
	out.Scope = c.copyString(v.Scope)
	out.Outcome = c.copyString(v.Outcome)
	out.Stages = copyFinalizationSlice(c, v.Stages, func(v PerformanceStage) PerformanceStage { return c.buildPerformanceStage(v) })
	out.Flow = c.buildPerformanceFlow(v.Flow)
	out.CandidateStages = copyFinalizationSlice(c, v.CandidateStages, func(v PerformanceCandidates) PerformanceCandidates { return c.buildPerformanceCandidates(v) })
	out.Routing = c.buildPerformanceRouting(v.Routing)
	out.Collisions = copyFinalizationSlice(c, v.Collisions, func(v PerformanceCollision) PerformanceCollision { return c.buildPerformanceCollision(v) })
	out.Unmeasured = copyFinalizationSlice(c, v.Unmeasured, func(v string) string { return c.copyString(v) })
	return out
}

// statisticsRate 职责：复制Rate的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) statisticsRate(v statistics.Rate) statistics.Rate {
	out := v
	out.Name = c.copyString(v.Name)
	return out
}

// statisticsPower 职责：复制Power的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) statisticsPower(v statistics.Power) statistics.Power {
	out := v
	out.ExpectedMW = copyFinalizationPointer(c, v.ExpectedMW, func(v float64) float64 { return v })
	return out
}

// statisticsThermal 职责：复制Thermal的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) statisticsThermal(v statistics.Thermal) statistics.Thermal {
	out := v
	out.DissipationMW = copyFinalizationPointer(c, v.DissipationMW, func(v float64) float64 { return v })
	return out
}

// statisticsBuildingGroup 职责：复制BuildingGroup的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) statisticsBuildingGroup(v statistics.BuildingGroup) statistics.BuildingGroup {
	out := v
	out.Name = c.copyString(v.Name)
	out.Category = c.copyString(v.Category)
	out.Inputs = copyFinalizationSlice(c, v.Inputs, func(v statistics.Rate) statistics.Rate { return c.statisticsRate(v) })
	out.Outputs = copyFinalizationSlice(c, v.Outputs, func(v statistics.Rate) statistics.Rate { return c.statisticsRate(v) })
	out.ConfiguredInputs = copyFinalizationSlice(c, v.ConfiguredInputs, func(v statistics.Rate) statistics.Rate { return c.statisticsRate(v) })
	out.ConfiguredOutputs = copyFinalizationSlice(c, v.ConfiguredOutputs, func(v statistics.Rate) statistics.Rate { return c.statisticsRate(v) })
	out.Power = c.statisticsPower(v.Power)
	out.Thermal = c.statisticsThermal(v.Thermal)
	out.Utilization = copyFinalizationPointer(c, v.Utilization, func(v float64) float64 { return v })
	return out
}

// statisticsLine 职责：复制Line的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) statisticsLine(v statistics.Line) statistics.Line {
	out := v
	out.MachineName = c.copyString(v.MachineName)
	out.Inputs = copyFinalizationSlice(c, v.Inputs, func(v statistics.Rate) statistics.Rate { return c.statisticsRate(v) })
	out.Outputs = copyFinalizationSlice(c, v.Outputs, func(v statistics.Rate) statistics.Rate { return c.statisticsRate(v) })
	out.NetInputs = copyFinalizationSlice(c, v.NetInputs, func(v statistics.Rate) statistics.Rate { return c.statisticsRate(v) })
	out.NetOutputs = copyFinalizationSlice(c, v.NetOutputs, func(v statistics.Rate) statistics.Rate { return c.statisticsRate(v) })
	out.ConfiguredInputs = copyFinalizationSlice(c, v.ConfiguredInputs, func(v statistics.Rate) statistics.Rate { return c.statisticsRate(v) })
	out.ConfiguredOutputs = copyFinalizationSlice(c, v.ConfiguredOutputs, func(v statistics.Rate) statistics.Rate { return c.statisticsRate(v) })
	out.JointProducts = copyFinalizationSlice(c, v.JointProducts, func(v int) int { return v })
	out.Power = c.statisticsPower(v.Power)
	out.Thermal = c.statisticsThermal(v.Thermal)
	return out
}

// statisticsMaterial 职责：复制Material的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) statisticsMaterial(v statistics.Material) statistics.Material {
	out := v
	out.Name = c.copyString(v.Name)
	out.HeatValueMJ = copyFinalizationPointer(c, v.HeatValueMJ, func(v float64) float64 { return v })
	return out
}

// statisticsSpace 职责：复制Space的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) statisticsSpace(v statistics.Space) statistics.Space {
	out := v
	return out
}

// statisticsHeatmapEntry 职责：复制HeatmapEntry的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) statisticsHeatmapEntry(v statistics.HeatmapEntry) statistics.HeatmapEntry {
	out := v
	out.Utilization = copyFinalizationPointer(c, v.Utilization, func(v float64) float64 { return v })
	out.ExpectedMW = copyFinalizationPointer(c, v.ExpectedMW, func(v float64) float64 { return v })
	out.Thermal = c.statisticsThermal(v.Thermal)
	return out
}

// statisticsReport 职责：复制Report的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) statisticsReport(v statistics.Report) statistics.Report {
	out := v
	out.SchemaVersion = c.copyString(v.SchemaVersion)
	out.Scope = c.copyString(v.Scope)
	out.PlanID = c.copyString(v.PlanID)
	out.DataVersion = c.copyString(v.DataVersion)
	out.GameVersion = c.copyString(v.GameVersion)
	out.FactsVersion = c.copyString(v.FactsVersion)
	out.Buildings = copyFinalizationSlice(c, v.Buildings, func(v statistics.BuildingGroup) statistics.BuildingGroup { return c.statisticsBuildingGroup(v) })
	out.Lines = copyFinalizationSlice(c, v.Lines, func(v statistics.Line) statistics.Line { return c.statisticsLine(v) })
	out.Materials = copyFinalizationSlice(c, v.Materials, func(v statistics.Material) statistics.Material { return c.statisticsMaterial(v) })
	out.Power = c.statisticsPower(v.Power)
	out.Thermal = c.statisticsThermal(v.Thermal)
	out.Space = copyFinalizationPointer(c, v.Space, func(v statistics.Space) statistics.Space { return c.statisticsSpace(v) })
	out.Heatmap = copyFinalizationSlice(c, v.Heatmap, func(v statistics.HeatmapEntry) statistics.HeatmapEntry { return c.statisticsHeatmapEntry(v) })
	out.Notes = copyFinalizationSlice(c, v.Notes, func(v string) string { return c.copyString(v) })
	return out
}

// quantTarget 职责：复制Target的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) quantTarget(v quant.Target) quant.Target {
	out := v
	return out
}

// quantByproductBalance 职责：复制ByproductBalance的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) quantByproductBalance(v quant.ByproductBalance) quant.ByproductBalance {
	out := v
	out.Name = c.copyString(v.Name)
	return out
}

// quantResult 职责：复制Result的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) quantResult(v quant.Result) quant.Result {
	out := v
	out.Statistics = copyFinalizationPointer(c, v.Statistics, func(v statistics.Report) statistics.Report { return c.statisticsReport(v) })
	out.Targets = copyFinalizationSlice(c, v.Targets, func(v quant.Target) quant.Target { return c.quantTarget(v) })
	out.Dataset = c.copyString(v.Dataset)
	out.DataVersion = c.copyString(v.DataVersion)
	out.GameVersion = c.copyString(v.GameVersion)
	out.PlanID = c.copyString(v.PlanID)
	out.Productions = copyFinalizationSlice(c, v.Productions, func(v quant.Production) quant.Production { return c.quantProduction(v) })
	out.RawInputs = copyFinalizationSlice(c, v.RawInputs, func(v quant.ItemRate) quant.ItemRate { return c.quantItemRate(v) })
	out.ExternalInputItemIDs = copyFinalizationSlice(c, v.ExternalInputItemIDs, func(v int) int { return v })
	out.FlowGraph = copyFinalizationPointer(c, v.FlowGraph, func(v quant.FlowGraph) quant.FlowGraph { return c.quantFlowGraph(v) })
	out.Leftovers = copyFinalizationSlice(c, v.Leftovers, func(v quant.ItemRate) quant.ItemRate { return c.quantItemRate(v) })
	out.Byproducts = copyFinalizationSlice(c, v.Byproducts, func(v quant.ByproductBalance) quant.ByproductBalance { return c.quantByproductBalance(v) })
	out.KeyItems = copyFinalizationSlice(c, v.KeyItems, func(v int) int { return v })
	out.Warnings = copyFinalizationSlice(c, v.Warnings, func(v string) string { return c.copyString(v) })
	out.Amounts = copyFinalizationMap(c, v.Amounts, func(v string) string { return c.copyString(v) }, func(v float64) float64 { return v })
	return out
}

// buildSurfacePackedBand 职责：复制surfacePackedBand的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildSurfacePackedBand(v surfacePackedBand) surfacePackedBand {
	out := v
	return out
}

// buildSurfaceLayout 职责：复制SurfaceLayout的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildSurfaceLayout(v SurfaceLayout) SurfaceLayout {
	out := v
	out.nativeBand = copyFinalizationPointer(c, v.nativeBand, func(v surfacePackedBand) surfacePackedBand { return c.buildSurfacePackedBand(v) })
	out.LatitudeZone = c.copyString(v.LatitudeZone)
	out.Mode = c.copyString(v.Mode)
	return out
}

// buildRequest 职责：复制Request的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildRequest(v Request) Request {
	out := v
	out.TaskID = c.copyString(v.TaskID)
	out.Plan = c.quantResult(v.Plan)
	out.Density = c.copyString(v.Density)
	out.Setting = c.buildSetting(v.Setting)
	out.Name = c.copyString(v.Name)
	out.SurfaceLayout = copyFinalizationPointer(c, v.SurfaceLayout, func(v SurfaceLayout) SurfaceLayout { return c.buildSurfaceLayout(v) })
	return out
}

// buildSurfaceBandPlan 职责：复制SurfaceBandPlan的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildSurfaceBandPlan(v SurfaceBandPlan) SurfaceBandPlan {
	out := v
	return out
}

// buildSurfacePlan 职责：复制SurfacePlan的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildSurfacePlan(v SurfacePlan) SurfacePlan {
	out := v
	out.LatitudeZone = c.copyString(v.LatitudeZone)
	out.LatitudeZoneLabel = c.copyString(v.LatitudeZoneLabel)
	out.PlacementHint = c.copyString(v.PlacementHint)
	out.SafeLatitudeMinDegrees = copyFinalizationPointer(c, v.SafeLatitudeMinDegrees, func(v float64) float64 { return v })
	out.SafeLatitudeMaxDegrees = copyFinalizationPointer(c, v.SafeLatitudeMaxDegrees, func(v float64) float64 { return v })
	out.Bands = copyFinalizationSlice(c, v.Bands, func(v SurfaceBandPlan) SurfaceBandPlan { return c.buildSurfaceBandPlan(v) })
	out.Mode = c.copyString(v.Mode)
	out.ColliderGameVersion = c.copyString(v.ColliderGameVersion)
	out.CollisionCoverage = c.copyString(v.CollisionCoverage)
	out.PlacementStatus = c.copyString(v.PlacementStatus)
	return out
}

// buildRuleProfile 职责：复制RuleProfile的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildRuleProfile(v RuleProfile) RuleProfile {
	out := v
	out.Version = c.copyString(v.Version)
	out.GameVersion = c.copyString(v.GameVersion)
	out.SteamBuildID = c.copyString(v.SteamBuildID)
	out.AssemblySHA256 = c.copyString(v.AssemblySHA256)
	out.ResourcesSHA256 = c.copyString(v.ResourcesSHA256)
	return out
}

// buildRuleAudit 职责：复制RuleAudit的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildRuleAudit(v RuleAudit) RuleAudit {
	out := v
	out.Profile = c.buildRuleProfile(v.Profile)
	out.QuantDataVersion = c.copyString(v.QuantDataVersion)
	out.CoordinateSpace = c.copyString(v.CoordinateSpace)
	out.Coverage = c.copyString(v.Coverage)
	out.Checks = copyFinalizationSlice(c, v.Checks, func(v LogisticsCheck) LogisticsCheck { return c.buildLogisticsCheck(v) })
	out.Unverified = copyFinalizationSlice(c, v.Unverified, func(v string) string { return c.copyString(v) })
	return out
}

// buildDensityProductionReport 职责：复制DensityProductionReport的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDensityProductionReport(v DensityProductionReport) DensityProductionReport {
	out := v
	return out
}

// buildDensityReport 职责：复制DensityReport的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDensityReport(v DensityReport) DensityReport {
	out := v
	out.ReferenceSourceSHA256 = c.copyString(v.ReferenceSourceSHA256)
	out.Requested = c.copyString(v.Requested)
	out.ProfileVersion = c.copyString(v.ProfileVersion)
	out.BlueprintSHA256 = c.copyString(v.BlueprintSHA256)
	out.Productions = copyFinalizationSlice(c, v.Productions, func(v DensityProductionReport) DensityProductionReport { return c.buildDensityProductionReport(v) })
	out.Verification = c.copyString(v.Verification)
	return out
}

// buildProductionBinding 职责：复制ProductionBinding的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildProductionBinding(v ProductionBinding) ProductionBinding {
	out := v
	out.MachineIndices = copyFinalizationSlice(c, v.MachineIndices, func(v int) int { return v })
	out.BuildingIndices = copyFinalizationSlice(c, v.BuildingIndices, func(v int) int { return v })
	return out
}

// buildZone 职责：复制Zone的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildZone(v Zone) Zone {
	out := v
	out.Key = c.copyString(v.Key)
	out.ProductionIndices = copyFinalizationSlice(c, v.ProductionIndices, func(v int) int { return v })
	return out
}

// blueprintArea 职责：复制Area的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) blueprintArea(v blueprint.Area) blueprint.Area {
	out := v
	return out
}

// buildDenseSurfaceRelocation 职责：复制denseSurfaceRelocation的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildDenseSurfaceRelocation(v denseSurfaceRelocation) denseSurfaceRelocation {
	out := v
	out.sourceRecords = copyFinalizationSlice(c, v.sourceRecords, func(v blueprint.Building) blueprint.Building { return c.blueprintBuilding(v) })
	out.safeAnchors = copyFinalizationSlice(c, v.safeAnchors, func(v int) int { return v })
	return out
}

// buildSurfaceEncoding 职责：复制surfaceEncoding的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) buildSurfaceEncoding(v surfaceEncoding) surfaceEncoding {
	out := v
	out.positionSnapshot = nil
	out.areas = copyFinalizationSlice(c, v.areas, func(v blueprint.Area) blueprint.Area { return c.blueprintArea(v) })
	out.byBuilding = copyFinalizationMap(c, v.byBuilding, func(v *placed) *placed {
		return copyFinalizationPointer(c, v, func(v placed) placed { return c.buildPlaced(v) })
	}, func(v int) int { return v })
	out.latitudeOffsets = copyFinalizationSlice(c, v.latitudeOffsets, func(v int) int { return v })
	out.longitudeScales = copyFinalizationSlice(c, v.longitudeScales, func(v float64) float64 { return v })
	out.plan = copyFinalizationPointer(c, v.plan, func(v SurfacePlan) SurfacePlan { return c.buildSurfacePlan(v) })
	out.anchorCandidates = copyFinalizationSlice(c, v.anchorCandidates, func(v int) int { return v })
	out.relocation = copyFinalizationPointer(c, v.relocation, func(v denseSurfaceRelocation) denseSurfaceRelocation { return c.buildDenseSurfaceRelocation(v) })
	return out
}

// blueprintPointInt 职责：复制PointInt的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) blueprintPointInt(v blueprint.PointInt) blueprint.PointInt {
	out := v
	return out
}

// blueprintReformRect 职责：复制ReformRect的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) blueprintReformRect(v blueprint.ReformRect) blueprint.ReformRect {
	out := v
	return out
}

// blueprintReformData 职责：复制ReformData的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) blueprintReformData(v blueprint.ReformData) blueprint.ReformData {
	out := v
	out.Rects = copyFinalizationSlice(c, v.Rects, func(v blueprint.ReformRect) blueprint.ReformRect { return c.blueprintReformRect(v) })
	out.CustomReformColors = copyFinalizationSlice(c, v.CustomReformColors, func(v uint32) uint32 { return v })
	return out
}

// blueprintSummary 职责：复制Summary的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) blueprintSummary(v blueprint.Summary) blueprint.Summary {
	out := v
	out.ItemCounts = copyFinalizationMap(c, v.ItemCounts, func(v int16) int16 { return v }, func(v int) int { return v })
	return out
}

// blueprintDocument 职责：复制Document的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) blueprintDocument(v blueprint.Document) blueprint.Document {
	out := v
	out.Name = c.copyString(v.Name)
	out.Description = c.copyString(v.Description)
	out.Author = c.copyString(v.Author)
	out.GameVersion = c.copyString(v.GameVersion)
	out.CreatedAt = c.copyString(v.CreatedAt)
	out.Checksum = c.copyString(v.Checksum)
	out.CursorOffset = c.blueprintPointInt(v.CursorOffset)
	out.DragBoxSize = c.blueprintPointInt(v.DragBoxSize)
	out.Areas = copyFinalizationSlice(c, v.Areas, func(v blueprint.Area) blueprint.Area { return c.blueprintArea(v) })
	out.Buildings = copyFinalizationSlice(c, v.Buildings, func(v blueprint.Building) blueprint.Building { return c.blueprintBuilding(v) })
	out.Reform = copyFinalizationPointer(c, v.Reform, func(v blueprint.ReformData) blueprint.ReformData { return c.blueprintReformData(v) })
	out.RawTail = copyFinalizationSlice(c, v.RawTail, func(v byte) byte { return v })
	out.EnvelopeFields = copyFinalizationSlice(c, v.EnvelopeFields, func(v string) string { return c.copyString(v) })
	out.OriginalGzip = copyFinalizationSlice(c, v.OriginalGzip, func(v byte) byte { return v })
	out.Summary = c.blueprintSummary(v.Summary)
	return out
}

// quantDatasetInfo 职责：复制DatasetInfo的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) quantDatasetInfo(v quant.DatasetInfo) quant.DatasetInfo {
	out := v
	out.Key = c.copyString(v.Key)
	out.Name = c.copyString(v.Name)
	out.GameVersion = c.copyString(v.GameVersion)
	out.DataVersion = c.copyString(v.DataVersion)
	return out
}

// quantItem 职责：复制Item的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) quantItem(v quant.Item) quant.Item {
	out := v
	out.Name = c.copyString(v.Name)
	return out
}

// quantDataset 职责：复制Dataset的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) quantDataset(v quant.Dataset) quant.Dataset {
	out := v
	out.Info = c.quantDatasetInfo(v.Info)
	out.Items = copyFinalizationSlice(c, v.Items, func(v quant.Item) quant.Item { return c.quantItem(v) })
	out.Recipes = copyFinalizationSlice(c, v.Recipes, func(v quant.Recipe) quant.Recipe { return c.quantRecipe(v) })
	out.ByID = copyFinalizationMap(c, v.ByID, func(v int) int { return v }, func(v quant.Item) quant.Item { return c.quantItem(v) })
	out.ByName = copyFinalizationMap(c, v.ByName, func(v string) string { return c.copyString(v) }, func(v quant.Item) quant.Item { return c.quantItem(v) })
	out.RecipeByID = copyFinalizationMap(c, v.RecipeByID, func(v int) int { return v }, func(v int) int { return v })
	out.Choices = copyFinalizationMap(c, v.Choices, func(v string) string { return c.copyString(v) }, func(v []int) []int { return copyFinalizationSlice(c, v, func(v int) int { return v }) })
	out.ExternalOnly = copyFinalizationMap(c, v.ExternalOnly, func(v string) string { return c.copyString(v) }, func(v bool) bool { return v })
	out.Order = copyFinalizationSlice(c, v.Order, func(v string) string { return c.copyString(v) })
	return out
}

// previewBuilding 职责：复制Building的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) previewBuilding(v preview.Building) preview.Building {
	out := v
	out.LocalOffset = c.blueprintPoint(v.LocalOffset)
	out.LocalOffset2 = c.blueprintPoint(v.LocalOffset2)
	out.InputObjectID = c.copyString(v.InputObjectID)
	out.OutputObjectID = c.copyString(v.OutputObjectID)
	return out
}

// previewObject 职责：复制Object的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) previewObject(v preview.Object) preview.Object {
	out := v
	out.ID = c.copyString(v.ID)
	out.Building = c.previewBuilding(v.Building)
	return out
}

// previewPoint2 职责：复制Point2的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) previewPoint2(v preview.Point2) preview.Point2 {
	out := v
	return out
}

// previewArea 职责：复制Area的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) previewArea(v preview.Area) preview.Area {
	out := v
	out.AnchorLocalOffset = c.previewPoint2(v.AnchorLocalOffset)
	out.Size = c.previewPoint2(v.Size)
	return out
}

// previewFrame 职责：复制Frame的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) previewFrame(v preview.Frame) preview.Frame {
	out := v
	out.CoordinateSystem = c.copyString(v.CoordinateSystem)
	out.Areas = copyFinalizationSlice(c, v.Areas, func(v preview.Area) preview.Area { return c.previewArea(v) })
	return out
}

// previewScene 职责：复制Scene的可变字段；使用：冻结及临时校验视图；边界：保留图内身份，不与源共享参数或集合。
func (c *finalizationCloner) previewScene(v preview.Scene) preview.Scene {
	out := v
	out.Objects = copyFinalizationSlice(c, v.Objects, func(v preview.Object) preview.Object { return c.previewObject(v) })
	out.Frame = c.previewFrame(v.Frame)
	return out
}
