package saveanalysis

// 文件职责：按受支持存档布局顺序消费区段，仅保留查询字段。
// 使用：只读解码器内部调用；边界：版本不符立即失败，不推测区段位置。

// layoutDysonSphere 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDysonSphere struct {
	layerCount    int64
	layersIdBased []*layoutExportStruct_int32__DysonSphereLayer_
}

// readDysonSphere 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDysonSphere(props ...int64) *layoutDysonSphere {
	r.section = "DysonSphere"
	v := &layoutDysonSphere{}
	r.expect(r.number(4, true), 3, "DysonSphere.version")
	r.skip(4)
	r.readDysonSwarm()
	r.expect(r.number(4, true), 1212, "DysonSphere.reserved")
	var layerCount int64
	layerCount = r.number(4, true)
	v.layerCount = layerCount
	var numLayersIdBased int64
	numLayersIdBased = r.number(4, true)
	var layersIdBased []*layoutExportStruct_int32__DysonSphereLayer_
	nlayersIdBased := r.length(numLayersIdBased - 1)
	layersIdBased = make([]*layoutExportStruct_int32__DysonSphereLayer_, nlayersIdBased)
	for i := int64(0); i < nlayersIdBased; i++ {
		layersIdBased[i] = r.readExportStruct_int32__DysonSphereLayer_()
	}
	v.layersIdBased = layersIdBased
	r.skip(4)
	var rocketCursor int64
	rocketCursor = r.number(4, true)
	var rocketRecycleCursor int64
	rocketRecycleCursor = r.number(4, true)
	nrocketPool := r.length(rocketCursor - 1)
	for i := int64(0); i < nrocketPool; i++ {
		r.readDysonRocket()
	}
	nrocketRecycle := r.length(rocketRecycleCursor)
	r.skip(nrocketRecycle * 4)
	r.skip(4)
	var numAutoNodes int64
	numAutoNodes = r.number(4, true)
	nautoNodes := r.length(numAutoNodes)
	for i := int64(0); i < nautoNodes; i++ {
		r.readDysonAutoNode()
	}
	r.skip(4)
	var nrdCursor int64
	nrdCursor = r.number(4, true)
	var nrdRecycleCursor int64
	nrdRecycleCursor = r.number(4, true)
	nnrdPool := r.length(nrdCursor - 1)
	for i := int64(0); i < nnrdPool; i++ {
		r.readDysonNodeRData()
	}
	nnrdRecycle := r.length(nrdRecycleCursor)
	r.skip(nnrdRecycle * 4)
	r.skip(16)
	return v
}

// layoutDysonSwarm 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDysonSwarm struct {
}

// readDysonSwarm 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDysonSwarm(props ...int64) *layoutDysonSwarm {
	r.section = "DysonSwarm"
	v := &layoutDysonSwarm{}
	r.expect(r.number(4, true), 8, "DysonSwarm.version")
	r.skip(4)
	var sailCapacity int64
	sailCapacity = r.number(4, true)
	var sailCursor int64
	sailCursor = r.number(4, true)
	var sailRecycleCursor int64
	sailRecycleCursor = r.number(4, true)
	var sailPoolForSave []*layoutDysonSail
	nsailPoolForSave := r.length(sailCursor)
	sailPoolForSave = make([]*layoutDysonSail, nsailPoolForSave)
	for i := int64(0); i < nsailPoolForSave; i++ {
		sailPoolForSave[i] = r.readDysonSail()
	}
	nsailInfos := r.length(sailCursor)
	for i := int64(0); i < nsailInfos; i++ {
		r.readDysonSailInfo_ExportStruct(sailPoolForSave[i].st)
	}
	nsailRecycle := r.length(sailRecycleCursor)
	r.skip(nsailRecycle * 4)
	r.skip(4)
	var orbitCursor int64
	orbitCursor = r.number(4, true)
	norbits := r.length(orbitCursor - 1)
	for i := int64(0); i < norbits; i++ {
		r.readSailOrbit()
	}
	var numExpiryOrder int64
	numExpiryOrder = r.number(4, true)
	r.expect(numExpiryOrder, sailCapacity, "DysonSwarm.numExpiryOrder")
	r.skip(8)
	nexpiryOrder := r.length(numExpiryOrder)
	for i := int64(0); i < nexpiryOrder; i++ {
		r.readExpiryOrder()
	}
	r.skip(1)
	var numAbsorbOrder int64
	numAbsorbOrder = r.number(4, true)
	r.expect(numAbsorbOrder, sailCapacity, "DysonSwarm.numAbsorbOrder")
	r.skip(8)
	nabsorbOrder := r.length(numAbsorbOrder)
	for i := int64(0); i < nabsorbOrder; i++ {
		r.readAbsorbOrder()
	}
	var numOrbitColors int64
	numOrbitColors = r.number(4, true)
	norbitColorsHSVA := r.length(numOrbitColors)
	for i := int64(0); i < norbitColorsHSVA; i++ {
		r.readQuaternion()
	}
	r.skip(4)
	var bulletCursor int64
	bulletCursor = r.number(4, true)
	var bulletRecycleCursor int64
	bulletRecycleCursor = r.number(4, true)
	nbulletPool := r.length(bulletCursor - 1)
	for i := int64(0); i < nbulletPool; i++ {
		r.readSailBullet()
	}
	nbulletRecycle := r.length(bulletRecycleCursor)
	r.skip(nbulletRecycle * 4)
	return v
}

// layoutDysonSail 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDysonSail struct {
	st int64
}

// readDysonSail 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDysonSail(props ...int64) *layoutDysonSail {
	r.section = "DysonSail"
	v := &layoutDysonSail{}
	var st int64
	st = r.number(4, false)
	v.st = st
	r.skip(28)
	return v
}

// layoutDysonSailInfo_ExportStruct 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDysonSailInfo_ExportStruct struct {
}

// readDysonSailInfo_ExportStruct 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDysonSailInfo_ExportStruct(props ...int64) *layoutDysonSailInfo_ExportStruct {
	r.section = "DysonSailInfo_ExportStruct"
	v := &layoutDysonSailInfo_ExportStruct{}
	st := props[0]
	_ = st
	if floatValue(st) >= 0.001 {
		r.readDysonSailInfo()
	}
	return v
}

// layoutDysonSailInfo 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDysonSailInfo struct {
}

// readDysonSailInfo 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDysonSailInfo(props ...int64) *layoutDysonSailInfo {
	r.section = "DysonSailInfo"
	v := &layoutDysonSailInfo{}
	r.skip(12)
	r.readVector3()
	return v
}

// layoutSailOrbit 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutSailOrbit struct {
}

// readSailOrbit 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readSailOrbit(props ...int64) *layoutSailOrbit {
	r.section = "SailOrbit"
	v := &layoutSailOrbit{}
	r.expect(r.number(4, true), 0, "SailOrbit.version")
	r.skip(8)
	r.readQuaternion()
	r.readVector3()
	r.skip(5)
	return v
}

// layoutExpiryOrder 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutExpiryOrder struct {
}

// readExpiryOrder 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readExpiryOrder(props ...int64) *layoutExpiryOrder {
	r.section = "ExpiryOrder"
	v := &layoutExpiryOrder{}
	r.skip(12)
	return v
}

// layoutAbsorbOrder 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutAbsorbOrder struct {
}

// readAbsorbOrder 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readAbsorbOrder(props ...int64) *layoutAbsorbOrder {
	r.section = "AbsorbOrder"
	v := &layoutAbsorbOrder{}
	r.skip(20)
	npadding := r.length(4)
	r.skip(npadding * 1)
	return v
}

// layoutSailBullet 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutSailBullet struct {
}

// readSailBullet 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readSailBullet(props ...int64) *layoutSailBullet {
	r.section = "SailBullet"
	v := &layoutSailBullet{}
	r.expect(r.number(4, true), 0, "SailBullet.version")
	r.skip(16)
	r.readVector3()
	r.readVector3()
	r.readVector3()
	r.readVector3()
	r.readVectorLF3()
	r.readVectorLF3()
	return v
}

// layoutExportStruct_int32__DysonSphereLayer_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutExportStruct_int32__DysonSphereLayer_ struct {
	index int64
	data  *layoutDysonSphereLayer
}

// readExportStruct_int32__DysonSphereLayer_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readExportStruct_int32__DysonSphereLayer_(props ...int64) *layoutExportStruct_int32__DysonSphereLayer_ {
	r.section = "ExportStruct<int32, DysonSphereLayer>"
	v := &layoutExportStruct_int32__DysonSphereLayer_{}
	var index int64
	index = r.number(4, true)
	v.index = index
	var data *layoutDysonSphereLayer
	if index != 0 {
		data = r.readDysonSphereLayer()
	}
	v.data = data
	return v
}

// layoutDysonSphereLayer 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDysonSphereLayer struct {
	id              int64
	orbitRadius     int64
	orbitRotation   *layoutQuaternion
	currentRotation *layoutQuaternion
	nodePool        []*layoutExportStruct_int32__DysonNode_
	framePool       []*layoutExportStruct_int32__DysonFrame_
	shellPool       []*layoutExportStruct_int32__DysonShell_
}

// readDysonSphereLayer 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDysonSphereLayer(props ...int64) *layoutDysonSphereLayer {
	r.section = "DysonSphereLayer"
	v := &layoutDysonSphereLayer{}
	r.expect(r.number(4, true), 2, "DysonSphereLayer.version")
	var id int64
	id = r.number(4, true)
	v.id = id
	var orbitRadius int64
	orbitRadius = r.number(4, false)
	v.orbitRadius = orbitRadius
	var orbitRotation *layoutQuaternion
	orbitRotation = r.readQuaternion()
	v.orbitRotation = orbitRotation
	r.readQuaternion()
	r.skip(8)
	var currentRotation *layoutQuaternion
	currentRotation = r.readQuaternion()
	v.currentRotation = currentRotation
	r.readQuaternion()
	r.skip(8)
	r.readColor_ExportStruct()
	r.skip(4)
	var nodeCursor int64
	nodeCursor = r.number(4, true)
	var nodeRecycleCursor int64
	nodeRecycleCursor = r.number(4, true)
	var nodePool []*layoutExportStruct_int32__DysonNode_
	nnodePool := r.length(nodeCursor - 1)
	nodePool = make([]*layoutExportStruct_int32__DysonNode_, nnodePool)
	for i := int64(0); i < nnodePool; i++ {
		nodePool[i] = r.readExportStruct_int32__DysonNode_()
	}
	v.nodePool = nodePool
	nnodeRecycle := r.length(nodeRecycleCursor)
	r.skip(nnodeRecycle * 4)
	r.skip(4)
	var frameCursor int64
	frameCursor = r.number(4, true)
	var frameRecycleCursor int64
	frameRecycleCursor = r.number(4, true)
	var framePool []*layoutExportStruct_int32__DysonFrame_
	nframePool := r.length(frameCursor - 1)
	framePool = make([]*layoutExportStruct_int32__DysonFrame_, nframePool)
	for i := int64(0); i < nframePool; i++ {
		framePool[i] = r.readExportStruct_int32__DysonFrame_()
	}
	v.framePool = framePool
	nframeRecycle := r.length(frameRecycleCursor)
	r.skip(nframeRecycle * 4)
	r.skip(4)
	var shellCursor int64
	shellCursor = r.number(4, true)
	var shellRecycleCursor int64
	shellRecycleCursor = r.number(4, true)
	var shellPool []*layoutExportStruct_int32__DysonShell_
	nshellPool := r.length(shellCursor - 1)
	shellPool = make([]*layoutExportStruct_int32__DysonShell_, nshellPool)
	for i := int64(0); i < nshellPool; i++ {
		shellPool[i] = r.readExportStruct_int32__DysonShell_()
	}
	v.shellPool = shellPool
	nshellRecycle := r.length(shellRecycleCursor)
	r.skip(nshellRecycle * 4)
	return v
}

// layoutColor_ExportStruct 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutColor_ExportStruct struct {
}

// readColor_ExportStruct 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readColor_ExportStruct(props ...int64) *layoutColor_ExportStruct {
	r.section = "Color_ExportStruct"
	v := &layoutColor_ExportStruct{}
	var available int64
	available = r.number(1, false)
	var numCellColors int64
	if available != 0 {
		numCellColors = r.number(4, true)
	}
	if available != 0 {
		ncellColors := r.length(numCellColors)
		for i := int64(0); i < ncellColors; i++ {
			r.readRGBA()
		}
	}
	return v
}

// layoutExportStruct_int32__DysonNode_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutExportStruct_int32__DysonNode_ struct {
	index int64
	data  *layoutDysonNode
}

// readExportStruct_int32__DysonNode_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readExportStruct_int32__DysonNode_(props ...int64) *layoutExportStruct_int32__DysonNode_ {
	r.section = "ExportStruct<int32, DysonNode>"
	v := &layoutExportStruct_int32__DysonNode_{}
	var index int64
	index = r.number(4, true)
	v.index = index
	var data *layoutDysonNode
	if index != 0 {
		data = r.readDysonNode()
	}
	v.data = data
	return v
}

// layoutDysonNode 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDysonNode struct {
	id      int64
	protoId int64
	layerId int64
	pos     *layoutVector3
	sp      int64
	spMax   int64
	color   *layoutRGBA
}

// readDysonNode 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDysonNode(props ...int64) *layoutDysonNode {
	r.section = "DysonNode"
	v := &layoutDysonNode{}
	r.expect(r.number(4, true), 5, "DysonNode.version")
	var id int64
	id = r.number(4, true)
	v.id = id
	var protoId int64
	protoId = r.number(4, true)
	v.protoId = protoId
	var layerId int64
	layerId = r.number(4, true)
	v.layerId = layerId
	r.skip(2)
	var pos *layoutVector3
	pos = r.readVector3()
	v.pos = pos
	var sp int64
	sp = r.number(4, true)
	v.sp = sp
	var spMax int64
	spMax = r.number(4, true)
	v.spMax = spMax
	r.skip(28)
	var color *layoutRGBA
	color = r.readRGBA()
	v.color = color
	return v
}

// layoutExportStruct_int32__DysonFrame_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutExportStruct_int32__DysonFrame_ struct {
	index int64
	data  *layoutDysonFrame
}

// readExportStruct_int32__DysonFrame_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readExportStruct_int32__DysonFrame_(props ...int64) *layoutExportStruct_int32__DysonFrame_ {
	r.section = "ExportStruct<int32, DysonFrame>"
	v := &layoutExportStruct_int32__DysonFrame_{}
	var index int64
	index = r.number(4, true)
	v.index = index
	var data *layoutDysonFrame
	if index != 0 {
		data = r.readDysonFrame()
	}
	v.data = data
	return v
}

// layoutDysonFrame 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDysonFrame struct {
	id      int64
	protoId int64
	layerId int64
	nodeAId int64
	nodeBId int64
	euler   int64
	spA     int64
	spB     int64
	spMax   int64
	color   *layoutRGBA
}

// readDysonFrame 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDysonFrame(props ...int64) *layoutDysonFrame {
	r.section = "DysonFrame"
	v := &layoutDysonFrame{}
	r.expect(r.number(4, true), 1, "DysonFrame.version")
	var id int64
	id = r.number(4, true)
	v.id = id
	var protoId int64
	protoId = r.number(4, true)
	v.protoId = protoId
	var layerId int64
	layerId = r.number(4, true)
	v.layerId = layerId
	r.skip(1)
	var nodeAId int64
	nodeAId = r.number(4, true)
	v.nodeAId = nodeAId
	var nodeBId int64
	nodeBId = r.number(4, true)
	v.nodeBId = nodeBId
	var euler int64
	euler = r.number(1, false)
	v.euler = euler
	var spA int64
	spA = r.number(4, true)
	v.spA = spA
	var spB int64
	spB = r.number(4, true)
	v.spB = spB
	var spMax int64
	spMax = r.number(4, true)
	v.spMax = spMax
	var color *layoutRGBA
	color = r.readRGBA()
	v.color = color
	return v
}

// layoutExportStruct_int32__DysonShell_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutExportStruct_int32__DysonShell_ struct {
	index int64
	data  *layoutDysonShell
}

// readExportStruct_int32__DysonShell_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readExportStruct_int32__DysonShell_(props ...int64) *layoutExportStruct_int32__DysonShell_ {
	r.section = "ExportStruct<int32, DysonShell>"
	v := &layoutExportStruct_int32__DysonShell_{}
	var index int64
	index = r.number(4, true)
	v.index = index
	var data *layoutDysonShell
	if index != 0 {
		data = r.readDysonShell()
	}
	v.data = data
	return v
}

// layoutDysonShell 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDysonShell struct {
	id          int64
	protoId     int64
	layerId     int64
	color       *layoutRGBA
	nodesId     []int64
	radius      int64
	gridScale   int64
	vertexCount int64
	verts       []*layoutVector3
	nodecps     []int64
}

// readDysonShell 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDysonShell(props ...int64) *layoutDysonShell {
	r.section = "DysonShell"
	v := &layoutDysonShell{}
	r.expect(r.number(4, true), 4, "DysonShell.version")
	var id int64
	id = r.number(4, true)
	v.id = id
	var protoId int64
	protoId = r.number(4, true)
	v.protoId = protoId
	var layerId int64
	layerId = r.number(4, true)
	v.layerId = layerId
	r.skip(4)
	var color *layoutRGBA
	color = r.readRGBA()
	v.color = color
	var numNodes int64
	numNodes = r.number(4, true)
	var nodesId []int64
	nnodesId := r.length(numNodes)
	nodesId = make([]int64, nnodesId)
	for i := int64(0); i < nnodesId; i++ {
		nodesId[i] = r.number(4, true)
	}
	v.nodesId = nodesId
	var radius int64
	radius = r.number(8, false)
	v.radius = radius
	var gridScale int64
	gridScale = r.number(4, true)
	v.gridScale = gridScale
	var numPolygon int64
	numPolygon = r.number(4, true)
	npolygon := r.length(numPolygon)
	for i := int64(0); i < npolygon; i++ {
		r.readVectorLF3()
	}
	var numPolyn int64
	numPolyn = r.number(4, true)
	npolyn := r.length(numPolyn)
	for i := int64(0); i < npolyn; i++ {
		r.readVectorLF3()
	}
	var numPolynu int64
	numPolynu = r.number(4, true)
	npolynu := r.length(numPolynu)
	r.skip(npolynu * 8)
	r.skip(1)
	r.readVectorLF3()
	r.readVectorLF3()
	r.readVectorLF3()
	r.readVectorLF3()
	r.readVectorLF3()
	r.readVectorLF3()
	r.readVectorLF3()
	r.readVectorLF3()
	r.readVectorLF3()
	r.readVectorLF3()
	var vertexCount int64
	vertexCount = r.number(4, true)
	v.vertexCount = vertexCount
	var verts []*layoutVector3
	nverts := r.length(vertexCount)
	verts = make([]*layoutVector3, nverts)
	for i := int64(0); i < nverts; i++ {
		verts[i] = r.readVector3()
	}
	v.verts = verts
	nuvs := r.length(vertexCount)
	for i := int64(0); i < nuvs; i++ {
		r.readVector2()
	}
	nuv2s := r.length(vertexCount)
	for i := int64(0); i < nuv2s; i++ {
		r.readVector2()
	}
	nvkeys := r.length(vertexCount)
	r.skip(nvkeys * 4)
	nvAdjs := r.length(vertexCount * 6)
	r.skip(nvAdjs * 2)
	nvertAttr := r.length(vertexCount)
	r.skip(nvertAttr * 4)
	var numVertsq int64
	numVertsq = r.number(4, true)
	nvertsq := r.length(numVertsq)
	r.skip(nvertsq * 2)
	var numVertsqOffset int64
	numVertsqOffset = r.number(4, true)
	nvertsqOffset := r.length(numVertsqOffset)
	r.skip(nvertsqOffset * 2)
	var numNodecps int64
	numNodecps = r.number(4, true)
	var nodecps []int64
	nnodecps := r.length(numNodecps)
	nodecps = make([]int64, nnodecps)
	for i := int64(0); i < nnodecps; i++ {
		nodecps[i] = r.number(4, true)
	}
	v.nodecps = nodecps
	return v
}

// layoutDysonRocket 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDysonRocket struct {
}

// readDysonRocket 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDysonRocket(props ...int64) *layoutDysonRocket {
	r.section = "DysonRocket"
	v := &layoutDysonRocket{}
	r.expect(r.number(4, true), 0, "DysonRocket.version")
	r.skip(24)
	r.readVectorLF3()
	r.readQuaternion()
	r.readVector3()
	r.readVector3()
	return v
}

// layoutDysonAutoNode 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDysonAutoNode struct {
}

// readDysonAutoNode 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDysonAutoNode(props ...int64) *layoutDysonAutoNode {
	r.section = "DysonAutoNode"
	v := &layoutDysonAutoNode{}
	var available int64
	available = r.number(4, true)
	if available != 0 {
		r.skip(4)
	}
	if available != 0 {
		r.skip(4)
	}
	return v
}

// layoutDysonNodeRData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDysonNodeRData struct {
}

// readDysonNodeRData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDysonNodeRData(props ...int64) *layoutDysonNodeRData {
	r.section = "DysonNodeRData"
	v := &layoutDysonNodeRData{}
	r.expect(r.number(4, true), 0, "DysonNodeRData.version")
	r.skip(8)
	r.readVector3()
	r.skip(4)
	r.readQuaternion()
	return v
}

// layoutSpaceSector 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutSpaceSector struct {
}

// readSpaceSector 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readSpaceSector(props ...int64) *layoutSpaceSector {
	r.section = "SpaceSector"
	v := &layoutSpaceSector{}
	r.expect(r.number(4, true), 1, "SpaceSector.version")
	r.skip(4)
	var astroCursor int64
	astroCursor = r.number(4, true)
	nastros := r.length(astroCursor - 1)
	for i := int64(0); i < nastros; i++ {
		r.readAstroData()
	}
	r.skip(4)
	var enemyCursor int64
	enemyCursor = r.number(4, true)
	var enemyRecycleCursor int64
	enemyRecycleCursor = r.number(4, true)
	nenemyPool := r.length(enemyCursor - 1)
	for i := int64(0); i < nenemyPool; i++ {
		r.readEnemyData()
	}
	nenemyRecycle := r.length(enemyRecycleCursor)
	r.skip(nenemyRecycle * 4)
	nenemyAnimPool := r.length(enemyCursor - 1)
	for i := int64(0); i < nenemyAnimPool; i++ {
		r.readAnimData()
	}
	r.skip(4)
	var craftCursor int64
	craftCursor = r.number(4, true)
	var craftRecycleCursor int64
	craftRecycleCursor = r.number(4, true)
	ncraftPool := r.length(craftCursor - 1)
	for i := int64(0); i < ncraftPool; i++ {
		r.readCraftData()
	}
	ncraftRecycle := r.length(craftRecycleCursor)
	r.skip(ncraftRecycle * 4)
	ncraftAnimPool := r.length(craftCursor - 1)
	for i := int64(0); i < ncraftAnimPool; i++ {
		r.readAnimData()
	}
	r.readDataPool_SpaceRuinData_()
	r.readSkillSystem()
	var numDfHives int64
	numDfHives = r.number(4, true)
	ndfHives := r.length(numDfHives)
	for i := int64(0); i < ndfHives; i++ {
		r.readEnemyDFHiveSystem_ExportStruct()
	}
	r.readCombatSpaceSystem()
	return v
}

// layoutAstroData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutAstroData struct {
}

// readAstroData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readAstroData(props ...int64) *layoutAstroData {
	r.section = "AstroData"
	v := &layoutAstroData{}
	var id int64
	id = r.number(4, true)
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.readVectorLF3()
	}
	if id > 0 {
		r.readVectorLF3()
	}
	if id > 0 {
		r.readQuaternion()
	}
	if id > 0 {
		r.readQuaternion()
	}
	r.skip(4)
	return v
}

// layoutDataPool_SpaceRuinData_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPool_SpaceRuinData_ struct {
}

// readDataPool_SpaceRuinData_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPool_SpaceRuinData_(props ...int64) *layoutDataPool_SpaceRuinData_ {
	r.section = "DataPool<SpaceRuinData>"
	v := &layoutDataPool_SpaceRuinData_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readSpaceRuinData()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutSpaceRuinData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutSpaceRuinData struct {
}

// readSpaceRuinData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readSpaceRuinData(props ...int64) *layoutSpaceRuinData {
	r.section = "SpaceRuinData"
	v := &layoutSpaceRuinData{}
	return v
}

// layoutSkillSystem 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutSkillSystem struct {
}

// readSkillSystem 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readSkillSystem(props ...int64) *layoutSkillSystem {
	r.section = "SkillSystem"
	v := &layoutSkillSystem{}
	r.expect(r.number(4, true), 3, "SkillSystem.version")
	r.readDataPool_CombatStat_()
	var numRemovedSkillTargets int64
	numRemovedSkillTargets = r.number(4, true)
	nremovedSkillTargets := r.length(numRemovedSkillTargets)
	for i := int64(0); i < nremovedSkillTargets; i++ {
		r.readSkillTarget()
	}
	r.readDataPoolRenderer_LocalGeneralProjectile_()
	r.readDataPoolRenderer_LocalLaserContinuous_()
	r.readDataPoolRenderer_LocalLaserOneShot_()
	r.readDataPool_LocalCannonade_()
	r.readDataPoolRenderer_LocalDisturbingWave_()
	r.readDataPoolRenderer_GeneralProjectile_()
	r.readDataPool_SpaceLaserOneShot_()
	r.readDataPool_SpaceLaserSweep_()
	r.readDataPool_Bomb_Explosive_()
	r.readDataPool_Bomb_EMCapsule_()
	r.readDataPool_Bomb_Liquid_()
	r.readDataPoolRenderer_LocalLaserOneShot_()
	r.readDataPoolRenderer_LocalGeneralProjectile_()
	r.readDataPoolRenderer_LocalGeneralProjectile_()
	r.readDataPoolRenderer_LocalLaserOneShot_()
	r.readDataPoolRenderer_LocalGeneralProjectile_()
	r.readDataPoolRenderer_LocalLaserOneShot_()
	r.readDataPoolRenderer_LocalGeneralProjectile_()
	r.readDataPoolRenderer_LocalGeneralProjectile_()
	r.readDataPoolRenderer_LocalGeneralProjectile_()
	r.readDataPoolRenderer_LocalLaserContinuous_()
	r.readDataPool_LocalCannonade_()
	r.readDataPoolRenderer_GeneralProjectile_()
	r.readDataPoolRenderer_GeneralProjectile_()
	r.readDataPool_GeneralMissile_()
	r.readVFTrailRenderer()
	r.readDataPoolRenderer_LocalDisturbingWave_()
	r.readDataPoolRenderer_GeneralProjectile_()
	r.readDataPool_SpaceLaserOneShot_()
	r.readDataPoolRenderer_GeneralProjectile_()
	r.readDataPool_SpaceLaserOneShot_()
	r.readDataPool_SpaceLaserSweep_()
	r.readDataPoolRenderer_GeneralExpImpProjectile_()
	r.readDataPool_SpaceLaserOneShot_()
	r.readDataPoolRenderer_GeneralProjectile_()
	r.readDataPoolRenderer_GeneralProjectile_()
	r.readDataPoolRenderer_LocalGeneralProjectile_()
	r.readDataPoolRenderer_GeneralProjectile_()
	r.readDataPoolRenderer_LocalLaserOneShot_()
	r.readDataPool_SpaceLaserOneShot_()
	r.readDataPool_LocalCannonade_()
	r.readDataPool_GeneralCannonade_()
	r.readDataPoolRenderer_GeneralProjectile_()
	r.readDataPool_GeneralMissile_()
	r.readVFTrailRenderer()
	r.readDataPoolRenderer_GeneralShieldBurst_()
	var numHitEffects int64
	numHitEffects = r.number(2, true)
	nhitEffects := r.length(numHitEffects)
	for i := int64(0); i < nhitEffects; i++ {
		r.readHitEffect_ExportStruct()
	}
	return v
}

// layoutDataPool_CombatStat_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPool_CombatStat_ struct {
}

// readDataPool_CombatStat_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPool_CombatStat_(props ...int64) *layoutDataPool_CombatStat_ {
	r.section = "DataPool<CombatStat>"
	v := &layoutDataPool_CombatStat_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readCombatStat()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutCombatStat 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutCombatStat struct {
}

// readCombatStat 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readCombatStat(props ...int64) *layoutCombatStat {
	r.section = "CombatStat"
	v := &layoutCombatStat{}
	r.expect(r.number(1, false), 1, "CombatStat.version")
	r.skip(41)
	r.readVector3()
	r.skip(13)
	r.readImpactData()
	return v
}

// layoutImpactData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutImpactData struct {
}

// readImpactData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readImpactData(props ...int64) *layoutImpactData {
	r.section = "ImpactData"
	v := &layoutImpactData{}
	r.expect(r.number(1, false), 0, "ImpactData.version")
	var hasMass int64
	hasMass = r.number(1, false)
	if hasMass > 0 {
		r.skip(4)
	}
	if hasMass > 0 {
		r.readVector3()
	}
	if hasMass > 0 {
		r.readVector3()
	}
	return v
}

// layoutSkillTarget 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutSkillTarget struct {
}

// readSkillTarget 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readSkillTarget(props ...int64) *layoutSkillTarget {
	r.section = "SkillTarget"
	v := &layoutSkillTarget{}
	r.skip(12)
	return v
}

// layoutDataPoolRenderer_LocalGeneralProjectile_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPoolRenderer_LocalGeneralProjectile_ struct {
}

// readDataPoolRenderer_LocalGeneralProjectile_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPoolRenderer_LocalGeneralProjectile_(props ...int64) *layoutDataPoolRenderer_LocalGeneralProjectile_ {
	r.section = "DataPoolRenderer<LocalGeneralProjectile>"
	v := &layoutDataPoolRenderer_LocalGeneralProjectile_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readLocalGeneralProjectile()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutLocalGeneralProjectile 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutLocalGeneralProjectile struct {
}

// readLocalGeneralProjectile 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readLocalGeneralProjectile(props ...int64) *layoutLocalGeneralProjectile {
	r.section = "LocalGeneralProjectile"
	v := &layoutLocalGeneralProjectile{}
	r.expect(r.number(1, false), 1, "LocalGeneralProjectile.version")
	var id int64
	id = r.number(4, true)
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(1)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(1)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	return v
}

// layoutDataPoolRenderer_LocalLaserContinuous_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPoolRenderer_LocalLaserContinuous_ struct {
}

// readDataPoolRenderer_LocalLaserContinuous_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPoolRenderer_LocalLaserContinuous_(props ...int64) *layoutDataPoolRenderer_LocalLaserContinuous_ {
	r.section = "DataPoolRenderer<LocalLaserContinuous>"
	v := &layoutDataPoolRenderer_LocalLaserContinuous_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readLocalLaserContinuous()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutLocalLaserContinuous 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutLocalLaserContinuous struct {
}

// readLocalLaserContinuous 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readLocalLaserContinuous(props ...int64) *layoutLocalLaserContinuous {
	r.section = "LocalLaserContinuous"
	v := &layoutLocalLaserContinuous{}
	r.expect(r.number(1, false), 0, "LocalLaserContinuous.version")
	var id int64
	id = r.number(4, true)
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(1)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(1)
	}
	if id > 0 {
		r.skip(4)
	}
	return v
}

// layoutDataPoolRenderer_LocalLaserOneShot_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPoolRenderer_LocalLaserOneShot_ struct {
}

// readDataPoolRenderer_LocalLaserOneShot_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPoolRenderer_LocalLaserOneShot_(props ...int64) *layoutDataPoolRenderer_LocalLaserOneShot_ {
	r.section = "DataPoolRenderer<LocalLaserOneShot>"
	v := &layoutDataPoolRenderer_LocalLaserOneShot_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readLocalLaserOneShot()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutLocalLaserOneShot 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutLocalLaserOneShot struct {
}

// readLocalLaserOneShot 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readLocalLaserOneShot(props ...int64) *layoutLocalLaserOneShot {
	r.section = "LocalLaserOneShot"
	v := &layoutLocalLaserOneShot{}
	r.expect(r.number(1, false), 0, "LocalLaserOneShot.version")
	var id int64
	id = r.number(4, true)
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(1)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(1)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(2)
	}
	return v
}

// layoutDataPool_LocalCannonade_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPool_LocalCannonade_ struct {
}

// readDataPool_LocalCannonade_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPool_LocalCannonade_(props ...int64) *layoutDataPool_LocalCannonade_ {
	r.section = "DataPool<LocalCannonade>"
	v := &layoutDataPool_LocalCannonade_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readLocalCannonade()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutLocalCannonade 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutLocalCannonade struct {
}

// readLocalCannonade 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readLocalCannonade(props ...int64) *layoutLocalCannonade {
	r.section = "LocalCannonade"
	v := &layoutLocalCannonade{}
	r.expect(r.number(1, false), 0, "LocalCannonade.version")
	var id int64
	id = r.number(4, true)
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(1)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(1)
	}
	if id > 0 {
		r.skip(4)
	}
	return v
}

// layoutDataPoolRenderer_LocalDisturbingWave_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPoolRenderer_LocalDisturbingWave_ struct {
}

// readDataPoolRenderer_LocalDisturbingWave_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPoolRenderer_LocalDisturbingWave_(props ...int64) *layoutDataPoolRenderer_LocalDisturbingWave_ {
	r.section = "DataPoolRenderer<LocalDisturbingWave>"
	v := &layoutDataPoolRenderer_LocalDisturbingWave_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readLocalDisturbingWave()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutLocalDisturbingWave 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutLocalDisturbingWave struct {
}

// readLocalDisturbingWave 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readLocalDisturbingWave(props ...int64) *layoutLocalDisturbingWave {
	r.section = "LocalDisturbingWave"
	v := &layoutLocalDisturbingWave{}
	r.expect(r.number(4, true), 0, "LocalDisturbingWave.version")
	var id int64
	id = r.number(4, true)
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.readQuaternion()
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(1)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(4)
	}
	return v
}

// layoutDataPoolRenderer_GeneralProjectile_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPoolRenderer_GeneralProjectile_ struct {
}

// readDataPoolRenderer_GeneralProjectile_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPoolRenderer_GeneralProjectile_(props ...int64) *layoutDataPoolRenderer_GeneralProjectile_ {
	r.section = "DataPoolRenderer<GeneralProjectile>"
	v := &layoutDataPoolRenderer_GeneralProjectile_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readGeneralProjectile()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutGeneralProjectile 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutGeneralProjectile struct {
}

// readGeneralProjectile 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readGeneralProjectile(props ...int64) *layoutGeneralProjectile {
	r.section = "GeneralProjectile"
	v := &layoutGeneralProjectile{}
	r.expect(r.number(1, false), 2, "GeneralProjectile.version")
	var id int64
	id = r.number(4, true)
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.readVectorLF3()
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.expect(r.number(4, true), 0, "GeneralProjectile.zero")
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(1)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	return v
}

// layoutDataPool_SpaceLaserOneShot_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPool_SpaceLaserOneShot_ struct {
}

// readDataPool_SpaceLaserOneShot_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPool_SpaceLaserOneShot_(props ...int64) *layoutDataPool_SpaceLaserOneShot_ {
	r.section = "DataPool<SpaceLaserOneShot>"
	v := &layoutDataPool_SpaceLaserOneShot_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readSpaceLaserOneShot()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutSpaceLaserOneShot 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutSpaceLaserOneShot struct {
}

// readSpaceLaserOneShot 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readSpaceLaserOneShot(props ...int64) *layoutSpaceLaserOneShot {
	r.section = "SpaceLaserOneShot"
	v := &layoutSpaceLaserOneShot{}
	r.expect(r.number(1, false), 0, "SpaceLaserOneShot.version")
	var id int64
	id = r.number(4, true)
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.readVectorLF3()
	}
	if id > 0 {
		r.readVectorLF3()
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(1)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(1)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(4)
	}
	return v
}

// layoutDataPool_SpaceLaserSweep_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPool_SpaceLaserSweep_ struct {
}

// readDataPool_SpaceLaserSweep_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPool_SpaceLaserSweep_(props ...int64) *layoutDataPool_SpaceLaserSweep_ {
	r.section = "DataPool<SpaceLaserSweep>"
	v := &layoutDataPool_SpaceLaserSweep_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readSpaceLaserSweep()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutSpaceLaserSweep 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutSpaceLaserSweep struct {
}

// readSpaceLaserSweep 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readSpaceLaserSweep(props ...int64) *layoutSpaceLaserSweep {
	r.section = "SpaceLaserSweep"
	v := &layoutSpaceLaserSweep{}
	r.expect(r.number(1, false), 0, "SpaceLaserSweep.version")
	var id int64
	id = r.number(4, true)
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(1)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	return v
}

// layoutDataPool_Bomb_Explosive_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPool_Bomb_Explosive_ struct {
}

// readDataPool_Bomb_Explosive_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPool_Bomb_Explosive_(props ...int64) *layoutDataPool_Bomb_Explosive_ {
	r.section = "DataPool<Bomb_Explosive>"
	v := &layoutDataPool_Bomb_Explosive_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readBomb_Explosive()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutBomb_Explosive 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutBomb_Explosive struct {
}

// readBomb_Explosive 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readBomb_Explosive(props ...int64) *layoutBomb_Explosive {
	r.section = "Bomb_Explosive"
	v := &layoutBomb_Explosive{}
	r.expect(r.number(4, true), 0, "Bomb_Explosive.version")
	var id int64
	id = r.number(4, true)
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.readVectorLF3()
	}
	if id > 0 {
		r.readQuaternion()
	}
	if id > 0 {
		r.readVectorLF3()
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	return v
}

// layoutDataPool_Bomb_EMCapsule_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPool_Bomb_EMCapsule_ struct {
}

// readDataPool_Bomb_EMCapsule_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPool_Bomb_EMCapsule_(props ...int64) *layoutDataPool_Bomb_EMCapsule_ {
	r.section = "DataPool<Bomb_EMCapsule>"
	v := &layoutDataPool_Bomb_EMCapsule_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readBomb_EMCapsule()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutBomb_EMCapsule 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutBomb_EMCapsule struct {
}

// readBomb_EMCapsule 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readBomb_EMCapsule(props ...int64) *layoutBomb_EMCapsule {
	r.section = "Bomb_EMCapsule"
	v := &layoutBomb_EMCapsule{}
	r.expect(r.number(4, true), 0, "Bomb_EMCapsule.version")
	var id int64
	id = r.number(4, true)
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.readVectorLF3()
	}
	if id > 0 {
		r.readQuaternion()
	}
	if id > 0 {
		r.readVectorLF3()
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	return v
}

// layoutDataPool_Bomb_Liquid_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPool_Bomb_Liquid_ struct {
}

// readDataPool_Bomb_Liquid_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPool_Bomb_Liquid_(props ...int64) *layoutDataPool_Bomb_Liquid_ {
	r.section = "DataPool<Bomb_Liquid>"
	v := &layoutDataPool_Bomb_Liquid_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readBomb_Liquid()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutBomb_Liquid 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutBomb_Liquid struct {
}

// readBomb_Liquid 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readBomb_Liquid(props ...int64) *layoutBomb_Liquid {
	r.section = "Bomb_Liquid"
	v := &layoutBomb_Liquid{}
	r.expect(r.number(4, true), 0, "Bomb_Liquid.version")
	var id int64
	id = r.number(4, true)
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.readVectorLF3()
	}
	if id > 0 {
		r.readQuaternion()
	}
	if id > 0 {
		r.readVectorLF3()
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	return v
}

// layoutDataPool_GeneralMissile_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPool_GeneralMissile_ struct {
}

// readDataPool_GeneralMissile_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPool_GeneralMissile_(props ...int64) *layoutDataPool_GeneralMissile_ {
	r.section = "DataPool<GeneralMissile>"
	v := &layoutDataPool_GeneralMissile_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readGeneralMissile()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutGeneralMissile 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutGeneralMissile struct {
}

// readGeneralMissile 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readGeneralMissile(props ...int64) *layoutGeneralMissile {
	r.section = "GeneralMissile"
	v := &layoutGeneralMissile{}
	r.expect(r.number(1, false), 2, "GeneralMissile.version")
	var id int64
	id = r.number(4, true)
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.readVectorLF3()
	}
	if id > 0 {
		r.readVectorLF3()
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.readQuaternion()
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(1)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	return v
}

// layoutVFTrailRenderer 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutVFTrailRenderer struct {
}

// readVFTrailRenderer 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readVFTrailRenderer(props ...int64) *layoutVFTrailRenderer {
	r.section = "VFTrailRenderer"
	v := &layoutVFTrailRenderer{}
	r.expect(r.number(1, false), 0, "VFTrailRenderer.version")
	var trailStride int64
	trailStride = r.number(4, true)
	r.skip(4)
	var numSmokePool int64
	numSmokePool = r.number(4, true)
	nsmokePool := r.length(numSmokePool - trailStride)
	for i := int64(0); i < nsmokePool; i++ {
		r.readSmokeData()
	}
	return v
}

// layoutSmokeData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutSmokeData struct {
}

// readSmokeData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readSmokeData(props ...int64) *layoutSmokeData {
	r.section = "SmokeData"
	v := &layoutSmokeData{}
	r.expect(r.number(1, false), 0, "SmokeData.version")
	r.skip(4)
	var createTime int64
	createTime = r.number(4, false)
	if createTime > 0 {
		r.skip(4)
	}
	if createTime > 0 {
		r.readVector3()
	}
	if createTime > 0 {
		r.readVector3()
	}
	return v
}

// layoutDataPoolRenderer_GeneralExpImpProjectile_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPoolRenderer_GeneralExpImpProjectile_ struct {
}

// readDataPoolRenderer_GeneralExpImpProjectile_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPoolRenderer_GeneralExpImpProjectile_(props ...int64) *layoutDataPoolRenderer_GeneralExpImpProjectile_ {
	r.section = "DataPoolRenderer<GeneralExpImpProjectile>"
	v := &layoutDataPoolRenderer_GeneralExpImpProjectile_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readGeneralExpImpProjectile()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutGeneralExpImpProjectile 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutGeneralExpImpProjectile struct {
}

// readGeneralExpImpProjectile 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readGeneralExpImpProjectile(props ...int64) *layoutGeneralExpImpProjectile {
	r.section = "GeneralExpImpProjectile"
	v := &layoutGeneralExpImpProjectile{}
	r.expect(r.number(1, false), 0, "GeneralExpImpProjectile.version")
	var id int64
	if id > 0 {
		id = r.number(4, true)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.readVectorLF3()
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(1)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	return v
}

// layoutDataPool_GeneralCannonade_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPool_GeneralCannonade_ struct {
}

// readDataPool_GeneralCannonade_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPool_GeneralCannonade_(props ...int64) *layoutDataPool_GeneralCannonade_ {
	r.section = "DataPool<GeneralCannonade>"
	v := &layoutDataPool_GeneralCannonade_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readGeneralCannonade()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutGeneralCannonade 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutGeneralCannonade struct {
}

// readGeneralCannonade 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readGeneralCannonade(props ...int64) *layoutGeneralCannonade {
	r.section = "GeneralCannonade"
	v := &layoutGeneralCannonade{}
	r.expect(r.number(1, false), 0, "GeneralCannonade.version")
	var id int64
	if id > 0 {
		id = r.number(4, true)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.readVectorLF3()
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(1)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(1)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.readVector3()
	}
	return v
}

// layoutDataPoolRenderer_GeneralShieldBurst_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPoolRenderer_GeneralShieldBurst_ struct {
}

// readDataPoolRenderer_GeneralShieldBurst_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPoolRenderer_GeneralShieldBurst_(props ...int64) *layoutDataPoolRenderer_GeneralShieldBurst_ {
	r.section = "DataPoolRenderer<GeneralShieldBurst>"
	v := &layoutDataPoolRenderer_GeneralShieldBurst_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readGeneralShieldBurst()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutGeneralShieldBurst 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutGeneralShieldBurst struct {
}

// readGeneralShieldBurst 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readGeneralShieldBurst(props ...int64) *layoutGeneralShieldBurst {
	r.section = "GeneralShieldBurst"
	v := &layoutGeneralShieldBurst{}
	r.expect(r.number(1, false), 1, "GeneralShieldBurst.version")
	r.skip(8)
	r.readVectorLF3()
	r.readVector3()
	r.skip(43)
	return v
}

// layoutHitEffect_ExportStruct 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutHitEffect_ExportStruct struct {
}

// readHitEffect_ExportStruct 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readHitEffect_ExportStruct(props ...int64) *layoutHitEffect_ExportStruct {
	r.section = "HitEffect_ExportStruct"
	v := &layoutHitEffect_ExportStruct{}
	var id int64
	id = r.number(2, true)
	if id != 0 {
		r.readDataPoolRenderer_ParticleData_()
	}
	return v
}

// layoutDataPoolRenderer_ParticleData_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPoolRenderer_ParticleData_ struct {
}

// readDataPoolRenderer_ParticleData_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPoolRenderer_ParticleData_(props ...int64) *layoutDataPoolRenderer_ParticleData_ {
	r.section = "DataPoolRenderer<ParticleData>"
	v := &layoutDataPoolRenderer_ParticleData_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readParticleData()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutParticleData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutParticleData struct {
}

// readParticleData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readParticleData(props ...int64) *layoutParticleData {
	r.section = "ParticleData"
	v := &layoutParticleData{}
	r.expect(r.number(1, false), 0, "ParticleData.version")
	var id int64
	id = r.number(4, true)
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	var astroId int64
	if id > 0 {
		astroId = r.number(4, true)
	}
	if id > 0 && astroId == 0 {
		r.readVectorLF3()
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	return v
}

// layoutEnemyDFHiveSystem_ExportStruct 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutEnemyDFHiveSystem_ExportStruct struct {
}

// readEnemyDFHiveSystem_ExportStruct 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readEnemyDFHiveSystem_ExportStruct(props ...int64) *layoutEnemyDFHiveSystem_ExportStruct {
	r.section = "EnemyDFHiveSystem_ExportStruct"
	v := &layoutEnemyDFHiveSystem_ExportStruct{}
	var magic int64
	magic = r.number(4, true)
	if magic == 19884 {
		r.readEnemyDFHiveSystem()
	}
	if magic == 19884 {
		r.readEnemyDFHiveSystem_ExportStruct()
	}
	return v
}

// layoutEnemyDFHiveSystem 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutEnemyDFHiveSystem struct {
}

// readEnemyDFHiveSystem 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readEnemyDFHiveSystem(props ...int64) *layoutEnemyDFHiveSystem {
	r.section = "EnemyDFHiveSystem"
	v := &layoutEnemyDFHiveSystem{}
	r.expect(r.number(4, true), 3, "EnemyDFHiveSystem.version")
	r.skip(12)
	var numPbuilders int64
	numPbuilders = r.number(4, true)
	npbuilders := r.length(numPbuilders)
	for i := int64(0); i < npbuilders; i++ {
		r.readGrowthPattern_DFSpace_Builder()
	}
	r.skip(48)
	r.readDataPool_EnemyBuilderComponent_()
	r.readDataPool_DFSCoreComponent_()
	r.readDataPool_DFSNodeComponent_()
	r.readDataPool_DFSConnectorComponent_()
	r.readDataPool_DFSReplicatorComponent_()
	r.readDataPool_DFSGammaComponent_()
	r.readDataPool_DFSTurretComponent_()
	r.readObjectPool_DFRelayComponent_()
	r.readDataPool_DFTinderComponent_()
	r.readDataPool_EnemyUnitComponent_()
	r.skip(4)
	var idleRelayCount int64
	idleRelayCount = r.number(4, true)
	nidleRelayIds := r.length(idleRelayCount)
	r.skip(nidleRelayIds * 4)
	r.skip(4)
	var idleTinderCount int64
	idleTinderCount = r.number(4, true)
	nidleTinderIds := r.length(idleTinderCount)
	r.skip(nidleTinderIds * 4)
	nforms := r.length(3)
	for i := int64(0); i < nforms; i++ {
		r.readEnemyFormation()
	}
	r.readEvolveData()
	r.readHatredList()
	r.readHatredList()
	return v
}

// layoutGrowthPattern_DFSpace_Builder 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutGrowthPattern_DFSpace_Builder struct {
}

// readGrowthPattern_DFSpace_Builder 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readGrowthPattern_DFSpace_Builder(props ...int64) *layoutGrowthPattern_DFSpace_Builder {
	r.section = "GrowthPattern_DFSpace_Builder"
	v := &layoutGrowthPattern_DFSpace_Builder{}
	r.expect(r.number(1, false), 0, "GrowthPattern_DFSpace_Builder.version")
	var protoId int64
	protoId = r.number(2, true)
	var instId int64
	instId = r.flexible()
	if instId > 0 {
		r.skip(2)
	}
	if protoId != 8109 && protoId != 8110 {
		r.flexible()
	}
	r.readVector3()
	r.readQuaternion()
	if protoId == 8109 || protoId == 8110 {
		r.skip(1)
	}
	if protoId == 8109 || protoId == 8110 {
		r.skip(4)
	}
	r.skip(6)
	var numConnections int64
	numConnections = r.number(2, true)
	nconnections := r.length(numConnections)
	r.skip(nconnections * 2)
	r.skip(1)
	return v
}

// layoutDataPool_DFSCoreComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPool_DFSCoreComponent_ struct {
}

// readDataPool_DFSCoreComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPool_DFSCoreComponent_(props ...int64) *layoutDataPool_DFSCoreComponent_ {
	r.section = "DataPool<DFSCoreComponent>"
	v := &layoutDataPool_DFSCoreComponent_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readDFSCoreComponent()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutDFSCoreComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDFSCoreComponent struct {
}

// readDFSCoreComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDFSCoreComponent(props ...int64) *layoutDFSCoreComponent {
	r.section = "DFSCoreComponent"
	v := &layoutDFSCoreComponent{}
	r.expect(r.number(1, false), 1, "DFSCoreComponent.version")
	var id int64
	id = r.number(2, false)
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	return v
}

// layoutDataPool_DFSNodeComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPool_DFSNodeComponent_ struct {
}

// readDataPool_DFSNodeComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPool_DFSNodeComponent_(props ...int64) *layoutDataPool_DFSNodeComponent_ {
	r.section = "DataPool<DFSNodeComponent>"
	v := &layoutDataPool_DFSNodeComponent_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readDFSNodeComponent()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutDFSNodeComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDFSNodeComponent struct {
}

// readDFSNodeComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDFSNodeComponent(props ...int64) *layoutDFSNodeComponent {
	r.section = "DFSNodeComponent"
	v := &layoutDFSNodeComponent{}
	r.expect(r.number(1, false), 0, "DFSNodeComponent.version")
	var id int64
	id = r.number(2, false)
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	return v
}

// layoutDataPool_DFSConnectorComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPool_DFSConnectorComponent_ struct {
}

// readDataPool_DFSConnectorComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPool_DFSConnectorComponent_(props ...int64) *layoutDataPool_DFSConnectorComponent_ {
	r.section = "DataPool<DFSConnectorComponent>"
	v := &layoutDataPool_DFSConnectorComponent_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readDFSConnectorComponent()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutDFSConnectorComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDFSConnectorComponent struct {
}

// readDFSConnectorComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDFSConnectorComponent(props ...int64) *layoutDFSConnectorComponent {
	r.section = "DFSConnectorComponent"
	v := &layoutDFSConnectorComponent{}
	r.expect(r.number(1, false), 0, "DFSConnectorComponent.version")
	var id int64
	id = r.number(2, false)
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(2)
	}
	return v
}

// layoutDataPool_DFSReplicatorComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPool_DFSReplicatorComponent_ struct {
}

// readDataPool_DFSReplicatorComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPool_DFSReplicatorComponent_(props ...int64) *layoutDataPool_DFSReplicatorComponent_ {
	r.section = "DataPool<DFSReplicatorComponent>"
	v := &layoutDataPool_DFSReplicatorComponent_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readDFSReplicatorComponent()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutDFSReplicatorComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDFSReplicatorComponent struct {
}

// readDFSReplicatorComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDFSReplicatorComponent(props ...int64) *layoutDFSReplicatorComponent {
	r.section = "DFSReplicatorComponent"
	v := &layoutDFSReplicatorComponent{}
	r.expect(r.number(1, false), 0, "DFSReplicatorComponent.version")
	var id int64
	id = r.number(2, false)
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(1)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(1)
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.readQuaternion()
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(2)
	}
	return v
}

// layoutDataPool_DFSGammaComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPool_DFSGammaComponent_ struct {
}

// readDataPool_DFSGammaComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPool_DFSGammaComponent_(props ...int64) *layoutDataPool_DFSGammaComponent_ {
	r.section = "DataPool<DFSGammaComponent>"
	v := &layoutDataPool_DFSGammaComponent_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readDFSGammaComponent()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutDFSGammaComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDFSGammaComponent struct {
}

// readDFSGammaComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDFSGammaComponent(props ...int64) *layoutDFSGammaComponent {
	r.section = "DFSGammaComponent"
	v := &layoutDFSGammaComponent{}
	r.expect(r.number(1, false), 0, "DFSGammaComponent.version")
	var id int64
	id = r.number(2, false)
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	return v
}

// layoutDataPool_DFSTurretComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPool_DFSTurretComponent_ struct {
}

// readDataPool_DFSTurretComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPool_DFSTurretComponent_(props ...int64) *layoutDataPool_DFSTurretComponent_ {
	r.section = "DataPool<DFSTurretComponent>"
	v := &layoutDataPool_DFSTurretComponent_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readDFSTurretComponent()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutDFSTurretComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDFSTurretComponent struct {
}

// readDFSTurretComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDFSTurretComponent(props ...int64) *layoutDFSTurretComponent {
	r.section = "DFSTurretComponent"
	v := &layoutDFSTurretComponent{}
	r.expect(r.number(1, false), 0, "DFSTurretComponent.version")
	var id int64
	id = r.number(2, false)
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(1)
	}
	if id > 0 {
		r.skip(1)
	}
	if id > 0 {
		r.skip(1)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	return v
}

// layoutObjectPool_DFRelayComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutObjectPool_DFRelayComponent_ struct {
}

// readObjectPool_DFRelayComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readObjectPool_DFRelayComponent_(props ...int64) *layoutObjectPool_DFRelayComponent_ {
	r.section = "ObjectPool<DFRelayComponent>"
	v := &layoutObjectPool_DFRelayComponent_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readExportStruct_int32__DFRelayComponent_()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutExportStruct_int32__DFRelayComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutExportStruct_int32__DFRelayComponent_ struct {
}

// readExportStruct_int32__DFRelayComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readExportStruct_int32__DFRelayComponent_(props ...int64) *layoutExportStruct_int32__DFRelayComponent_ {
	r.section = "ExportStruct<int32, DFRelayComponent>"
	v := &layoutExportStruct_int32__DFRelayComponent_{}
	var index int64
	index = r.number(4, true)
	if index != 0 {
		r.readDFRelayComponent()
	}
	return v
}

// layoutDFRelayComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDFRelayComponent struct {
}

// readDFRelayComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDFRelayComponent(props ...int64) *layoutDFRelayComponent {
	r.section = "DFRelayComponent"
	v := &layoutDFRelayComponent{}
	r.expect(r.number(1, false), 1, "DFRelayComponent.version")
	var id int64
	id = r.number(2, false)
	if id > 0 {
		r.skip(10) // enemyId、builderId、dockIndex。
		hiveID, planetID := r.number(4, true), r.number(4, true)
		position := r.readVector3()
		yaw := floatValue(r.number(4, false))
		baseState, baseID := r.number(1, true), r.number(2, true)
		r.skip(4) // baseTicks。
		r.readEvolveData()
		r.skip(3) // baseRespawnCD、direction。
		stage := r.number(1, true)
		r.skip(8) // uSpeed、param0。
		if baseState == 1 && baseID == 0 && stage == 2 {
			r.recordVirtualRelay(planetID, hiveID, id, position, yaw)
		}
	}
	var num int64
	num = r.number(1, false)
	ncarrierDatas := r.length(num)
	for i := int64(0); i < ncarrierDatas; i++ {
		r.readDFCarrierData()
	}
	r.skip(12)
	r.readVector3()
	r.skip(12)
	r.readVector3()
	r.skip(4)
	return v
}

// layoutDFCarrierData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDFCarrierData struct {
}

// readDFCarrierData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDFCarrierData(props ...int64) *layoutDFCarrierData {
	r.section = "DFCarrierData"
	v := &layoutDFCarrierData{}
	r.expect(r.number(1, false), 0, "DFCarrierData.version")
	r.skip(10)
	r.readVectorLF3()
	r.readQuaternion()
	r.skip(17)
	return v
}

// layoutDataPool_DFTinderComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPool_DFTinderComponent_ struct {
}

// readDataPool_DFTinderComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPool_DFTinderComponent_(props ...int64) *layoutDataPool_DFTinderComponent_ {
	r.section = "DataPool<DFTinderComponent>"
	v := &layoutDataPool_DFTinderComponent_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readDFTinderComponent()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutDFTinderComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDFTinderComponent struct {
}

// readDFTinderComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDFTinderComponent(props ...int64) *layoutDFTinderComponent {
	r.section = "DFTinderComponent"
	v := &layoutDFTinderComponent{}
	r.expect(r.number(1, false), 0, "DFTinderComponent.version")
	var id int64
	id = r.number(2, false)
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.skip(4)
	}
	return v
}

// layoutCombatSpaceSystem 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutCombatSpaceSystem struct {
}

// readCombatSpaceSystem 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readCombatSpaceSystem(props ...int64) *layoutCombatSpaceSystem {
	r.section = "CombatSpaceSystem"
	v := &layoutCombatSpaceSystem{}
	r.expect(r.number(4, true), 0, "CombatSpaceSystem.version")
	r.readDataPool_UnitComponent_()
	r.readDataPool_FleetComponent_()
	return v
}

// layoutGameAbnormalityData_0925 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutGameAbnormalityData_0925 struct {
}

// readGameAbnormalityData_0925 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readGameAbnormalityData_0925(props ...int64) *layoutGameAbnormalityData_0925 {
	r.section = "GameAbnormalityData_0925"
	v := &layoutGameAbnormalityData_0925{}
	r.expect(r.number(4, true), 0, "GameAbnormalityData_0925.version")
	r.expect(r.number(4, true), 0, "GameAbnormalityData_0925.reserved2")
	r.skip(4)
	var numRow int64
	numRow = r.number(4, true)
	nrow := r.length(numRow)
	for i := int64(0); i < nrow; i++ {
		r.readGameAbnormalityDataRow()
	}
	return v
}

// layoutGameAbnormalityDataRow 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutGameAbnormalityDataRow struct {
}

// readGameAbnormalityDataRow 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readGameAbnormalityDataRow(props ...int64) *layoutGameAbnormalityDataRow {
	r.section = "GameAbnormalityDataRow"
	v := &layoutGameAbnormalityDataRow{}
	var numCol int64
	numCol = r.number(4, true)
	ndata := r.length(numCol)
	for i := int64(0); i < ndata; i++ {
		r.readAbnormalityRuntimeData()
	}
	return v
}

// layoutAbnormalityRuntimeData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutAbnormalityRuntimeData struct {
}

// readAbnormalityRuntimeData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readAbnormalityRuntimeData(props ...int64) *layoutAbnormalityRuntimeData {
	r.section = "AbnormalityRuntimeData"
	v := &layoutAbnormalityRuntimeData{}
	r.expect(r.number(1, false), 0, "AbnormalityRuntimeData.version")
	var protoId int64
	protoId = r.number(1, false)
	if protoId > 0 {
		r.skip(8)
	}
	var numEvidences int64
	if protoId > 0 {
		numEvidences = r.number(4, true)
	}
	if protoId > 0 {
		nevidences := r.length(numEvidences)
		r.skip(nevidences * 8)
	}
	return v
}

// layoutMilestoneSystem 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutMilestoneSystem struct {
}

// readMilestoneSystem 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readMilestoneSystem(props ...int64) *layoutMilestoneSystem {
	r.section = "MilestoneSystem"
	v := &layoutMilestoneSystem{}
	r.expect(r.number(4, true), 0, "MilestoneSystem.version")
	var numMilestoneDatas int64
	numMilestoneDatas = r.number(4, true)
	nmilestoneDatas := r.length(numMilestoneDatas)
	for i := int64(0); i < nmilestoneDatas; i++ {
		r.readKeyValuePair_int32__MilestoneData_()
	}
	return v
}

// layoutKeyValuePair_int32__MilestoneData_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutKeyValuePair_int32__MilestoneData_ struct {
}

// readKeyValuePair_int32__MilestoneData_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readKeyValuePair_int32__MilestoneData_(props ...int64) *layoutKeyValuePair_int32__MilestoneData_ {
	r.section = "KeyValuePair<int32, MilestoneData>"
	v := &layoutKeyValuePair_int32__MilestoneData_{}
	r.skip(4)
	r.readMilestoneData()
	return v
}

// layoutMilestoneData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutMilestoneData struct {
}

// readMilestoneData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readMilestoneData(props ...int64) *layoutMilestoneData {
	r.section = "MilestoneData"
	v := &layoutMilestoneData{}
	r.expect(r.number(4, true), 0, "MilestoneData.version")
	r.skip(12)
	r.readJournalData()
	return v
}

// layoutGoalSystem 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutGoalSystem struct {
}

// readGoalSystem 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readGoalSystem(props ...int64) *layoutGoalSystem {
	r.section = "GoalSystem"
	v := &layoutGoalSystem{}
	r.expect(r.number(4, true), 0, "GoalSystem.version")
	var numGoalDatas int64
	numGoalDatas = r.number(4, true)
	ngoalDatas := r.length(numGoalDatas)
	for i := int64(0); i < ngoalDatas; i++ {
		r.readKeyValuePair_int32__GoalData_()
	}
	var queueCursor int64
	queueCursor = r.number(4, true)
	ngoalQueue := r.length(queueCursor)
	r.skip(ngoalQueue * 4)
	return v
}

// layoutKeyValuePair_int32__GoalData_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutKeyValuePair_int32__GoalData_ struct {
}

// readKeyValuePair_int32__GoalData_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readKeyValuePair_int32__GoalData_(props ...int64) *layoutKeyValuePair_int32__GoalData_ {
	r.section = "KeyValuePair<int32, GoalData>"
	v := &layoutKeyValuePair_int32__GoalData_{}
	r.skip(4)
	r.readGoalData()
	return v
}

// layoutGoalData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutGoalData struct {
}

// readGoalData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readGoalData(props ...int64) *layoutGoalData {
	r.section = "GoalData"
	v := &layoutGoalData{}
	r.expect(r.number(4, true), 0, "GoalData.version")
	r.skip(25)
	var numDisplayingState int64
	numDisplayingState = r.number(4, true)
	ndisplayingState := r.length(numDisplayingState)
	r.skip(ndisplayingState * 1)
	r.skip(1)
	return v
}

// layoutWarningSystem 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutWarningSystem struct {
}

// readWarningSystem 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readWarningSystem(props ...int64) *layoutWarningSystem {
	r.section = "WarningSystem"
	v := &layoutWarningSystem{}
	r.expect(r.number(4, true), 0, "WarningSystem.version")
	r.skip(4)
	var warningCursor int64
	warningCursor = r.number(4, true)
	var warningRecycleCursor int64
	warningRecycleCursor = r.number(4, true)
	nwarningPool := r.length(warningCursor - 1)
	for i := int64(0); i < nwarningPool; i++ {
		r.readWarningData()
	}
	nwarningRecycle := r.length(warningRecycleCursor)
	r.skip(nwarningRecycle * 4)
	return v
}

// layoutWarningData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutWarningData struct {
}

// readWarningData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readWarningData(props ...int64) *layoutWarningData {
	r.section = "WarningData"
	v := &layoutWarningData{}
	r.expect(r.number(4, true), 2, "WarningData.version")
	r.skip(32)
	r.readVector3()
	return v
}
