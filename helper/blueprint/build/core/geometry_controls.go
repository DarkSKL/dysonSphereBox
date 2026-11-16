package build

// 文件职责：按值保存几何阶段需要的装配标量控制，不保留指针或回调。
// 使用：不可变装配快照的捕获与恢复；边界：字段清单由builder标量定义产生，业务算法不在此复制。

// assemblyControls 职责：保存标量状态；使用：几何步骤克隆；边界：只含值语义。
type assemblyControls struct {
	previewActive              bool
	denseSpatial               bool
	nativeLatitudeScaled       bool
	nativeLongitudeScale       float64
	nativeCapacityReserve      bool
	denseSpatialOrder          int
	denseMaxSpan               float64
	denseLatitudeSpan          float64
	denseRefine                bool
	denseStreetCandidate       bool
	denseStreetRotation        int
	denseStreetGroupLimit      int
	densePairedOutputs         bool
	denseCapacityFill          bool
	directBoundaryLaneLimit    float64
	directBoundaryExtentScale  float64
	owner                      int
	flyLayoutHeight            float64
	connectedCompactBelts      bool
	connectedLegacyInputs      bool
	connectedHorizontalPacking bool
	connectedGlobalPower       bool
	flyCompact                 bool
	flySurfacePairs            bool
}

// captureAssemblyControls 职责：读取当前标量；使用：步骤发布；边界：不保存源对象引用。
func captureAssemblyControls(b *builder) assemblyControls {
	return assemblyControls{
		previewActive:              b.previewActive,
		denseSpatial:               b.denseSpatial,
		nativeLatitudeScaled:       b.nativeLatitudeScaled,
		nativeLongitudeScale:       b.nativeLongitudeScale,
		nativeCapacityReserve:      b.nativeCapacityReserve,
		denseSpatialOrder:          b.denseSpatialOrder,
		denseMaxSpan:               b.denseMaxSpan,
		denseLatitudeSpan:          b.denseLatitudeSpan,
		denseRefine:                b.denseRefine,
		denseStreetCandidate:       b.denseStreetCandidate,
		denseStreetRotation:        b.denseStreetRotation,
		denseStreetGroupLimit:      b.denseStreetGroupLimit,
		densePairedOutputs:         b.densePairedOutputs,
		denseCapacityFill:          b.denseCapacityFill,
		directBoundaryLaneLimit:    b.directBoundaryLaneLimit,
		directBoundaryExtentScale:  b.directBoundaryExtentScale,
		owner:                      b.owner,
		flyLayoutHeight:            b.flyLayoutHeight,
		connectedCompactBelts:      b.connectedCompactBelts,
		connectedLegacyInputs:      b.connectedLegacyInputs,
		connectedHorizontalPacking: b.connectedHorizontalPacking,
		connectedGlobalPower:       b.connectedGlobalPower,
		flyCompact:                 b.flyCompact,
		flySurfacePairs:            b.flySurfacePairs,
	}
}

// apply 职责：恢复当前步骤标量；使用：独占临时图；边界：不改变原快照。
func (c assemblyControls) apply(b *builder) {
	b.previewActive = c.previewActive
	b.denseSpatial = c.denseSpatial
	b.nativeLatitudeScaled = c.nativeLatitudeScaled
	b.nativeLongitudeScale = c.nativeLongitudeScale
	b.nativeCapacityReserve = c.nativeCapacityReserve
	b.denseSpatialOrder = c.denseSpatialOrder
	b.denseMaxSpan = c.denseMaxSpan
	b.denseLatitudeSpan = c.denseLatitudeSpan
	b.denseRefine = c.denseRefine
	b.denseStreetCandidate = c.denseStreetCandidate
	b.denseStreetRotation = c.denseStreetRotation
	b.denseStreetGroupLimit = c.denseStreetGroupLimit
	b.densePairedOutputs = c.densePairedOutputs
	b.denseCapacityFill = c.denseCapacityFill
	b.directBoundaryLaneLimit = c.directBoundaryLaneLimit
	b.directBoundaryExtentScale = c.directBoundaryExtentScale
	b.owner = c.owner
	b.flyLayoutHeight = c.flyLayoutHeight
	b.connectedCompactBelts = c.connectedCompactBelts
	b.connectedLegacyInputs = c.connectedLegacyInputs
	b.connectedHorizontalPacking = c.connectedHorizontalPacking
	b.connectedGlobalPower = c.connectedGlobalPower
	b.flyCompact = c.flyCompact
	b.flySurfacePairs = c.flySurfacePairs
}
