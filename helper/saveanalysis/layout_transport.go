package saveanalysis

// 文件职责：按受支持存档布局顺序消费区段，仅保留查询字段。
// 使用：只读解码器内部调用；边界：版本不符立即失败，不推测区段位置。

// layoutPlanetTransport 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutPlanetTransport struct {
}

// readPlanetTransport 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readPlanetTransport(props ...int64) *layoutPlanetTransport {
	r.section = "PlanetTransport"
	v := &layoutPlanetTransport{}
	r.expect(r.number(4, true), 1, "PlanetTransport.version")
	var stationCursor int64
	stationCursor = r.number(4, true)
	r.skip(4)
	var stationRecycleCursor int64
	stationRecycleCursor = r.number(4, true)
	nstationPool := r.length(stationCursor - 1)
	for i := int64(0); i < nstationPool; i++ {
		r.readExportStruct_int32__StationComponent_()
	}
	nstationRecycle := r.length(stationRecycleCursor)
	r.skip(nstationRecycle * 4)
	r.skip(4)
	var dispenserCursor int64
	dispenserCursor = r.number(4, true)
	var dispenserRecycleCursor int64
	dispenserRecycleCursor = r.number(4, true)
	ndispenserPool := r.length(dispenserCursor - 1)
	for i := int64(0); i < ndispenserPool; i++ {
		r.readExportStruct_int32__DispenserComponent_()
	}
	ndispenserRecycle := r.length(dispenserRecycleCursor)
	r.skip(ndispenserRecycle * 4)
	return v
}

// layoutExportStruct_int32__StationComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutExportStruct_int32__StationComponent_ struct {
}

// readExportStruct_int32__StationComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readExportStruct_int32__StationComponent_(props ...int64) *layoutExportStruct_int32__StationComponent_ {
	r.section = "ExportStruct<int32, StationComponent>"
	v := &layoutExportStruct_int32__StationComponent_{}
	var index int64
	index = r.number(4, true)
	if index != 0 {
		r.readStationComponent()
	}
	return v
}

// layoutStationComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutStationComponent struct {
	work            []freightRecord
	ports           []stationBelt
	id              int64
	gid             int64
	entityId        int64
	planetId        int64
	isStellar       int64
	storage         []*layoutStationStore
	isCollector     int64
	isVeinCollector int64
	// remoteGroupMask 职责：保留原版塔的全部已用组位；使用：隔离组选择；边界：按位解释，不把负值视作无组。
	remoteGroupMask uint64
}

// readStationComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readStationComponent(props ...int64) *layoutStationComponent {
	r.section = "StationComponent"
	v := &layoutStationComponent{}
	r.expect(r.number(4, true), 8, "StationComponent.version")
	var id int64
	id = r.number(4, true)
	v.id = id
	var gid int64
	gid = r.number(4, true)
	v.gid = gid
	var entityId int64
	entityId = r.number(4, true)
	v.entityId = entityId
	var planetId int64
	planetId = r.number(4, true)
	v.planetId = planetId
	r.skip(12)
	r.readVector3()
	r.readVector3()
	r.readQuaternion()
	var isStellar int64
	isStellar = r.number(1, false)
	v.isStellar = isStellar
	r.skip(36)
	var workDroneCount int64
	workDroneCount = r.number(4, true)
	r.skip(4)
	nworkDroneDatas := r.length(workDroneCount)
	for i := int64(0); i < nworkDroneDatas; i++ {
		v.work = append(v.work, r.readDroneData())
	}
	nworkDroneOrders := r.length(workDroneCount)
	for i := int64(0); i < nworkDroneOrders; i++ {
		v.work[i].order = r.readFreightOrder("LocalLogisticOrder")
	}
	r.skip(4)
	var workShipCount int64
	workShipCount = r.number(4, true)
	r.skip(20)
	nworkShipDatas := r.length(workShipCount)
	for i := int64(0); i < nworkShipDatas; i++ {
		v.work = append(v.work, r.readShipData())
	}
	nworkShipOrders := r.length(workShipCount)
	for i := int64(0); i < nworkShipOrders; i++ {
		v.work[workDroneCount+i].order = r.readFreightOrder("RemoteLogisticOrder")
	}
	var numStorage int64
	numStorage = r.number(4, true)
	var storage []*layoutStationStore
	nstorage := r.length(numStorage)
	storage = make([]*layoutStationStore, nstorage)
	for i := int64(0); i < nstorage; i++ {
		storage[i] = r.readStationStore()
	}
	v.storage = storage
	var numPriorityLocks int64
	numPriorityLocks = r.number(4, true)
	npriorityLocks := r.length(numPriorityLocks)
	for i := int64(0); i < npriorityLocks; i++ {
		r.readStationPriorityLock()
	}
	var numSlots int64
	numSlots = r.number(4, true)
	nslots := r.length(numSlots)
	for i := int64(0); i < nslots; i++ {
		dir := r.number(4, true)
		belt := r.number(4, true)
		slot := r.number(4, true)
		r.skip(4)
		if belt > 0 {
			v.ports = append(v.ports, stationBelt{dir, belt, int(slot) - 1})
			var item int64
			if dir == 1 && slot > 0 && slot <= int64(len(storage)) {
				item = storage[slot-1].itemId
			}
			r.line.ports[belt] = append(r.line.ports[belt], linePort{entityId, dir, item})
		}
	}
	r.skip(4)
	var numRemotePairProcess int64
	numRemotePairProcess = r.number(4, true)
	nremotePairProcess := r.length(numRemotePairProcess)
	r.skip(nremotePairProcess * 4)
	r.skip(4)
	var isCollector int64
	isCollector = r.number(1, false)
	v.isCollector = isCollector
	var isVeinCollector int64
	isVeinCollector = r.number(1, false)
	v.isVeinCollector = isVeinCollector
	if len(r.planets) == 0 || r.planets[len(r.planets)-1] != v.planetId {
		r.fail("物流塔与所属厂区星球不一致")
	}
	if v.isVeinCollector != 0 {
		if r.line.veinStations == nil {
			r.line.veinStations = map[int64]bool{}
		}
		r.line.veinStations[v.entityId] = true
	}
	r.stations = append(r.stations, v)
	var numCollectionIds int64
	numCollectionIds = r.number(4, true)
	ncollectionIds := r.length(numCollectionIds)
	r.skip(ncollectionIds * 4)
	var numCollectionPerTick int64
	numCollectionPerTick = r.number(4, true)
	ncollectionPerTick := r.length(numCollectionPerTick)
	r.skip(ncollectionPerTick * 4)
	var numCurrentCollections int64
	numCurrentCollections = r.number(4, true)
	ncurrentCollections := r.length(numCurrentCollections)
	r.skip(ncurrentCollections * 4)
	r.skip(4)
	var numDroneDispatchStatus int64
	numDroneDispatchStatus = r.number(1, false)
	ndroneDispatchStatus := r.length(numDroneDispatchStatus)
	r.skip(ndroneDispatchStatus * 1)
	r.skip(42)
	// 原版 StationComponent v8 最后先写 Int64 分组掩码，再写 Int32 路线优先模式。
	v.remoteGroupMask = uint64(r.number(8, false))
	r.skip(4)
	return v
}

// readDroneData 保留飞行方向、对端星球内站号和实际载货，用同位置的活动订单核对。
func (r *reader) readDroneData() freightRecord {
	r.section = "DroneData"
	r.expect(r.number(4, true), 1, "DroneData.version")
	r.skip(24)
	d := freightRecord{other: r.number(4, true), direction: floatValue(r.number(4, false))}
	r.skip(8)
	d.item = r.number(4, true)
	d.count = r.number(4, true)
	r.skip(8)
	return d
}

// readShipData 保留星际站全局编号、两端星球与实载，避免把星球内站号当作全局编号。
func (r *reader) readShipData() freightRecord {
	r.section = "ShipData"
	r.expect(r.number(4, true), 1, "ShipData.version")
	r.skip(4)
	d := freightRecord{ship: true, planetA: r.number(4, true), planetB: r.number(4, true)}
	r.skip(24 + 12 + 8 + 16 + 12 + 4 + 24 + 16)
	d.other = r.number(4, true)
	d.direction = float32(r.number(4, true))
	r.skip(4)
	d.item = r.number(4, true)
	d.count = r.number(4, true)
	r.skip(16)
	return d
}

// readFreightOrder 读取活动运输订单。两类订单使用同一布局，对端编号分别属于本地或全局。
func (r *reader) readFreightOrder(section string) freightOrder {
	r.section = section
	r.expect(r.number(4, true), 0, section+".version")
	return freightOrder{r.number(4, true), int(r.number(4, true)), int(r.number(4, true)), r.number(4, true), r.number(4, true), r.number(4, true)}
}

// layoutStationStore 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutStationStore struct {
	localOrder, remoteOrder int64
	itemId                  int64
	count                   int64
	maxWithKeepMode         int64
	localLogic              int64
	remoteLogic             int64
}

// readStationStore 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readStationStore(props ...int64) *layoutStationStore {
	r.section = "StationStore"
	v := &layoutStationStore{}
	r.expect(r.number(4, true), 2, "StationStore.version")
	var itemId int64
	itemId = r.number(4, true)
	v.itemId = itemId
	var count int64
	count = r.number(4, true)
	v.count = count
	r.skip(4)
	v.localOrder = r.number(4, true)
	v.remoteOrder = r.number(4, true)
	var maxWithKeepMode int64
	maxWithKeepMode = r.number(4, true)
	v.maxWithKeepMode = maxWithKeepMode
	r.skip(4)
	var localLogic int64
	localLogic = r.number(4, true)
	v.localLogic = localLogic
	var remoteLogic int64
	remoteLogic = r.number(4, true)
	v.remoteLogic = remoteLogic
	return v
}

// layoutStationPriorityLock 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutStationPriorityLock struct {
}

// readStationPriorityLock 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readStationPriorityLock(props ...int64) *layoutStationPriorityLock {
	r.section = "StationPriorityLock"
	v := &layoutStationPriorityLock{}
	r.expect(r.number(4, true), 0, "StationPriorityLock.version")
	r.skip(2)
	return v
}

// layoutExportStruct_int32__DispenserComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutExportStruct_int32__DispenserComponent_ struct {
}

// readExportStruct_int32__DispenserComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readExportStruct_int32__DispenserComponent_(props ...int64) *layoutExportStruct_int32__DispenserComponent_ {
	r.section = "ExportStruct<int32, DispenserComponent>"
	v := &layoutExportStruct_int32__DispenserComponent_{}
	var index int64
	index = r.number(4, true)
	if index != 0 {
		r.readDispenserComponent()
	}
	return v
}

// layoutDispenserComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDispenserComponent struct {
}

// readDispenserComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDispenserComponent(props ...int64) *layoutDispenserComponent {
	r.section = "DispenserComponent"
	v := &layoutDispenserComponent{}
	r.expect(r.number(4, true), 0, "DispenserComponent.version")
	r.skip(60)
	var workCourierCount int64
	workCourierCount = r.number(4, true)
	r.skip(5)
	nworkCourierDatas := r.length(workCourierCount)
	for i := int64(0); i < nworkCourierDatas; i++ {
		r.readCourierData()
	}
	norders := r.length(workCourierCount)
	for i := int64(0); i < norders; i++ {
		r.readDeliveryLogisticOrder()
	}
	var holdupItemCount int64
	holdupItemCount = r.number(4, true)
	r.skip(4)
	nholdupPackage := r.length(holdupItemCount)
	for i := int64(0); i < nholdupPackage; i++ {
		r.readDispenserStore()
	}
	r.skip(16)
	return v
}

// layoutCourierData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutCourierData struct {
}

// readCourierData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readCourierData(props ...int64) *layoutCourierData {
	r.section = "CourierData"
	v := &layoutCourierData{}
	r.expect(r.number(4, true), 0, "CourierData.version")
	r.readVector3()
	r.readVector3()
	r.skip(32)
	return v
}

// layoutDeliveryLogisticOrder 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDeliveryLogisticOrder struct {
}

// readDeliveryLogisticOrder 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDeliveryLogisticOrder(props ...int64) *layoutDeliveryLogisticOrder {
	r.section = "DeliveryLogisticOrder"
	v := &layoutDeliveryLogisticOrder{}
	r.expect(r.number(4, true), 0, "DeliveryLogisticOrder.version")
	r.skip(16)
	return v
}

// layoutDispenserStore 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDispenserStore struct {
}

// readDispenserStore 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDispenserStore(props ...int64) *layoutDispenserStore {
	r.section = "DispenserStore"
	v := &layoutDispenserStore{}
	r.expect(r.number(4, true), 0, "DispenserStore.version")
	r.skip(12)
	return v
}

// layoutPlatformSystem 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutPlatformSystem struct {
}

// readPlatformSystem 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readPlatformSystem(props ...int64) *layoutPlatformSystem {
	r.section = "PlatformSystem"
	v := &layoutPlatformSystem{}
	r.expect(r.number(4, true), 1, "PlatformSystem.version")
	var numReformData int64
	numReformData = r.number(4, true)
	nreformData := r.length(numReformData)
	data := r.readSurfaceBytes(nreformData)
	var latitudeCount int64
	latitudeCount = r.number(4, true)
	nreformOffsets := r.length(latitudeCount)
	if r.surface != nil {
		r.surface.terrain.ReformData = data
		if nreformOffsets > (int64(len(r.data))-r.offset)/4 {
			r.fail("地基纬带偏移数组截断")
		}
		r.surface.terrain.ReformOffsets = make([]int64, nreformOffsets)
		for i := range r.surface.terrain.ReformOffsets {
			r.surface.terrain.ReformOffsets[i] = r.number(4, true)
		}
	} else {
		r.skip(nreformOffsets * 4)
	}
	var numReformCustomColors int64
	numReformCustomColors = r.number(4, true)
	nreformCustomColors := r.length(numReformCustomColors)
	for i := int64(0); i < nreformCustomColors; i++ {
		r.readRGBAF()
	}
	return v
}

// layoutRGBAF 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutRGBAF struct {
}

// readRGBAF 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readRGBAF(props ...int64) *layoutRGBAF {
	r.section = "RGBAF"
	v := &layoutRGBAF{}
	r.skip(16)
	return v
}

// layoutDigitalSystem 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDigitalSystem struct {
}

// readDigitalSystem 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDigitalSystem(props ...int64) *layoutDigitalSystem {
	r.section = "DigitalSystem"
	v := &layoutDigitalSystem{}
	r.expect(r.number(4, true), 3, "DigitalSystem.version")
	r.skip(4)
	var speakerCursor int64
	speakerCursor = r.number(4, true)
	var speakerRecycleCursor int64
	speakerRecycleCursor = r.number(4, true)
	nspeakerPool := r.length(speakerCursor - 1)
	for i := int64(0); i < nspeakerPool; i++ {
		r.readSpeakerComponent()
	}
	nspeakerRecycle := r.length(speakerRecycleCursor)
	r.skip(nspeakerRecycle * 4)
	r.readObjectPool_ExtraInfoComponent_()
	r.readObjectPool_MarkerComponent_()
	r.skip(4)
	return v
}

// layoutSpeakerComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutSpeakerComponent struct {
}

// readSpeakerComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readSpeakerComponent(props ...int64) *layoutSpeakerComponent {
	r.section = "SpeakerComponent"
	v := &layoutSpeakerComponent{}
	r.expect(r.number(4, true), 1, "SpeakerComponent.version")
	r.skip(62)
	return v
}

// layoutObjectPool_ExtraInfoComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutObjectPool_ExtraInfoComponent_ struct {
}

// readObjectPool_ExtraInfoComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readObjectPool_ExtraInfoComponent_(props ...int64) *layoutObjectPool_ExtraInfoComponent_ {
	r.section = "ObjectPool<ExtraInfoComponent>"
	v := &layoutObjectPool_ExtraInfoComponent_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readExportStruct_int32__ExtraInfoComponent_()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutExportStruct_int32__ExtraInfoComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutExportStruct_int32__ExtraInfoComponent_ struct {
}

// readExportStruct_int32__ExtraInfoComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readExportStruct_int32__ExtraInfoComponent_(props ...int64) *layoutExportStruct_int32__ExtraInfoComponent_ {
	r.section = "ExportStruct<int32, ExtraInfoComponent>"
	v := &layoutExportStruct_int32__ExtraInfoComponent_{}
	var index int64
	index = r.number(4, true)
	if index != 0 {
		r.readExtraInfoComponent()
	}
	return v
}

// layoutExtraInfoComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutExtraInfoComponent struct {
}

// readExtraInfoComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readExtraInfoComponent(props ...int64) *layoutExtraInfoComponent {
	r.section = "ExtraInfoComponent"
	v := &layoutExtraInfoComponent{}
	r.expect(r.number(4, true), 0, "ExtraInfoComponent.version")
	r.skip(9)
	var infoLen int64
	infoLen = r.number(4, true)
	if infoLen > 0 {
		r.string()
	}
	return v
}

// layoutObjectPool_MarkerComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutObjectPool_MarkerComponent_ struct {
}

// readObjectPool_MarkerComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readObjectPool_MarkerComponent_(props ...int64) *layoutObjectPool_MarkerComponent_ {
	r.section = "ObjectPool<MarkerComponent>"
	v := &layoutObjectPool_MarkerComponent_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readExportStruct_int32__MarkerComponent_()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutExportStruct_int32__MarkerComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutExportStruct_int32__MarkerComponent_ struct {
}

// readExportStruct_int32__MarkerComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readExportStruct_int32__MarkerComponent_(props ...int64) *layoutExportStruct_int32__MarkerComponent_ {
	r.section = "ExportStruct<int32, MarkerComponent>"
	v := &layoutExportStruct_int32__MarkerComponent_{}
	var index int64
	index = r.number(4, true)
	if index != 0 {
		r.readMarkerComponent()
	}
	return v
}

// layoutMarkerComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutMarkerComponent struct {
}

// readMarkerComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readMarkerComponent(props ...int64) *layoutMarkerComponent {
	r.section = "MarkerComponent"
	v := &layoutMarkerComponent{}
	r.expect(r.number(4, true), 2, "MarkerComponent.version")
	r.skip(32)
	var lenWord int64
	lenWord = r.number(4, true)
	if lenWord != 0 {
		r.string()
	}
	r.skip(22)
	r.readVector3()
	r.readQuaternion()
	r.skip(4)
	var lenName int64
	lenName = r.number(4, true)
	if lenName != 0 {
		r.string()
	}
	var lenTags int64
	lenTags = r.number(4, true)
	if lenTags != 0 {
		r.string()
	}
	return v
}

// layoutExportStruct_int32__DysonSphere_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutExportStruct_int32__DysonSphere_ struct {
	index int64
	data  *layoutDysonSphere
}

// readExportStruct_int32__DysonSphere_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readExportStruct_int32__DysonSphere_(props ...int64) *layoutExportStruct_int32__DysonSphere_ {
	r.section = "ExportStruct<int32, DysonSphere>"
	v := &layoutExportStruct_int32__DysonSphere_{}
	var index int64
	index = r.number(4, true)
	v.index = index
	var data *layoutDysonSphere
	if index != 0 {
		data = r.readDysonSphere()
	}
	v.data = data
	return v
}
