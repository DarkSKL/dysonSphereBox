package saveanalysis

// 文件职责：按受支持存档布局顺序消费区段，仅保留查询字段。
// 使用：只读解码器内部调用；边界：版本不符立即失败，不推测区段位置。

// layoutPlanetFactory 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutPlanetFactory struct {
	planetId int64
}

// readPlanetFactory 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readPlanetFactory(props ...int64) *layoutPlanetFactory {
	r.section = "PlanetFactory"
	v := &layoutPlanetFactory{}
	r.expect(r.number(4, true), 10, "PlanetFactory.version")
	var planetId int64
	planetId = r.number(4, true)
	v.planetId = planetId
	r.planets = append(r.planets, planetId)
	r.line = newFactoryLine()
	r.pathBuffers = map[int64][]byte{}
	r.beltPivots = map[int64][2]int64{}
	r.line.drops = map[int64]DropObservation{}
	if r.lines == nil {
		r.lines = make(map[int64]*factoryLine)
	}
	r.skip(12)
	r.readPlanetData()
	r.skip(1)
	r.readHashSystem()
	r.readHashSystem()
	r.skip(4)
	var entityCursor int64
	entityCursor = r.number(4, true)
	var entityRecycleCursor int64
	entityRecycleCursor = r.number(4, true)
	var entityPool []*layoutEntityData
	nentityPool := r.length(entityCursor - 1)
	entityPool = make([]*layoutEntityData, nentityPool)
	for i := int64(0); i < nentityPool; i++ {
		entityPool[i] = r.readEntityData()
		if id := entityPool[i].id; id != 0 && id != i+1 {
			r.fail("厂区实体编号与池位置不符")
		}
	}
	nentityAnimPool := r.length(entityCursor)
	for i := int64(0); i < nentityAnimPool; i++ {
		r.skip(16)
		r.line.power = append(r.line.power, floatValue(r.number(4, false)))
	}
	nentitySignPool := r.length(entityCursor - 1)
	for i := int64(0); i < nentitySignPool; i++ {
		r.line.sign = append(r.line.sign, r.readSignData(entityPool[i].id).kind)
	}
	nentityConn := r.length(entityCursor*16 - 16)
	for i := int64(0); i < nentityConn; i++ {
		r.readEntityConnPool(entityPool[i/16].id)
	}
	nentityRecycle := r.length(entityRecycleCursor)
	r.skip(nentityRecycle * 4)
	r.skip(4)
	var prebuildCursor int64
	prebuildCursor = r.number(4, true)
	var prebuildRecycleCursor int64
	prebuildRecycleCursor = r.number(4, true)
	nprebuildPool := r.length(prebuildCursor - 1)
	for i := int64(0); i < nprebuildPool; i++ {
		r.readPrebuildExportStruct()
	}
	nprebuildRecycle := r.length(prebuildRecycleCursor)
	r.skip(nprebuildRecycle * 4)
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
	var vegeCursor int64
	vegeCursor = r.number(4, true)
	var vegeRecycleCursor int64
	vegeRecycleCursor = r.number(4, true)
	nvegePool := r.length(vegeCursor - 1)
	for i := int64(0); i < nvegePool; i++ {
		r.readVegeData()
	}
	nvegeRecycle := r.length(vegeRecycleCursor)
	r.skip(nvegeRecycle * 4)
	r.skip(4)
	var veinCursor int64
	veinCursor = r.number(4, true)
	var veinRecycleCursor int64
	veinRecycleCursor = r.number(4, true)
	nveinPool := r.length(veinCursor - 1)
	for i := int64(0); i < nveinPool; i++ {
		r.readVeinData()
	}
	nveinRecycle := r.length(veinRecycleCursor)
	r.skip(nveinRecycle * 4)
	nveinAnimPool := r.length(veinCursor - 1)
	for i := int64(0); i < nveinAnimPool; i++ {
		r.readAnimData()
	}
	r.skip(4)
	var ruinCursor int64
	ruinCursor = r.number(4, true)
	var ruinRecycleCursor int64
	ruinRecycleCursor = r.number(4, true)
	nruinPool := r.length(ruinCursor - 1)
	for i := int64(0); i < nruinPool; i++ {
		r.readRuinData()
	}
	nruinRecycle := r.length(ruinRecycleCursor)
	r.skip(nruinRecycle * 4)
	r.readCargoContainer()
	r.readCargoTraffic()
	r.readFactoryStorage()
	r.readPowerSystem()
	r.readFactorySystem()
	r.pathBuffers = nil
	r.beltPivots = nil
	r.readEnemyDFGroundSystem()
	r.readCombatGroundSystem()
	r.readDefenseSystem()
	r.readPlanetATField()
	r.readConstructionSystem()
	r.readPlanetTransport()
	r.readPlatformSystem()
	r.readDigitalSystem()
	if len(r.line.machines) > 0 || len(r.line.devices) > 0 {
		r.line.currentTech = r.currentTech
		r.line.researchPending = r.pendingTechs[r.currentTech]
		r.line.veins = nil
		r.lines[planetId] = r.line
	}
	r.line = nil
	return v
}

// layoutPlanetData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutPlanetData struct {
}

// readPlanetData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readPlanetData(props ...int64) *layoutPlanetData {
	r.section = "PlanetData"
	v := &layoutPlanetData{}
	var numModData int64
	numModData = r.number(4, true)
	nmodData := r.length(numModData)
	r.skip(nmodData * 1)
	var numVeinAmounts int64
	numVeinAmounts = r.number(4, true)
	nveinAmounts := r.length(numVeinAmounts)
	r.skip(nveinAmounts * 8)
	var numVeinGroups int64
	numVeinGroups = r.number(4, true)
	nveinGroups := r.length(numVeinGroups)
	for i := int64(0); i < nveinGroups; i++ {
		r.readVeinGroup()
	}
	return v
}

// layoutHashSystem 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutHashSystem struct {
}

// readHashSystem 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readHashSystem(props ...int64) *layoutHashSystem {
	r.section = "HashSystem"
	v := &layoutHashSystem{}
	var numHashPool int64
	numHashPool = r.number(4, true)
	nhashPoolRecycle := r.length(numHashPool)
	for i := int64(0); i < nhashPoolRecycle; i++ {
		r.readKeyValuePair_int32__int32_()
	}
	nbucketData := r.length(363)
	for i := int64(0); i < nbucketData; i++ {
		r.readBucketData_ExportStruct(i)
	}
	return v
}

// layoutBucketData_ExportStruct 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutBucketData_ExportStruct struct {
}

// readBucketData_ExportStruct 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readBucketData_ExportStruct(props ...int64) *layoutBucketData_ExportStruct {
	r.section = "BucketData_ExportStruct"
	v := &layoutBucketData_ExportStruct{}
	i := props[0]
	_ = i
	r.skip(4)
	if i < 362 {
		r.skip(4)
	}
	if i < 362 {
		r.skip(4)
	}
	return v
}

// layoutEntityData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutEntityData struct {
	id int64
}

// readEntityData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readEntityData(props ...int64) *layoutEntityData {
	r.section = "EntityData"
	v := &layoutEntityData{}
	r.expect(r.number(1, false), 12, "EntityData.version")
	var id int64
	id = r.number(4, true)
	v.id = id
	if id > 0 {
		r.line.protos[id] = r.number(2, true)
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
	var idSum int64
	if id > 0 {
		idSum = r.number(4, true)
	}
	var beltId *layoutcid
	if id > 0 {
		beltId = r.readcid()
	}
	var powerConId *layoutcid
	if id > 0 && beltId != nil && idSum > beltId.value {
		powerConId = r.readcid()
	}
	var inserterId *layoutcid
	if id > 0 && powerConId != nil && idSum > beltId.value+powerConId.value {
		inserterId = r.readcid()
	}
	var assemblerId *layoutcid
	if id > 0 && inserterId != nil && idSum > beltId.value+powerConId.value+inserterId.value {
		assemblerId = r.readcid()
	}
	var labId *layoutcid
	if id > 0 && assemblerId != nil && idSum > beltId.value+powerConId.value+inserterId.value+assemblerId.value {
		labId = r.readcid()
	}
	var powerNodeId *layoutcid
	if id > 0 && labId != nil && idSum > beltId.value+powerConId.value+inserterId.value+assemblerId.value+labId.value {
		powerNodeId = r.readcid()
	}
	var powerGenId *layoutcid
	if id > 0 && powerNodeId != nil && idSum > beltId.value+powerConId.value+inserterId.value+assemblerId.value+labId.value+powerNodeId.value {
		powerGenId = r.readcid()
	}
	var combatStatId *layoutcid
	if id > 0 && powerGenId != nil && idSum > beltId.value+powerConId.value+inserterId.value+assemblerId.value+labId.value+powerNodeId.value+powerGenId.value {
		combatStatId = r.readcid()
	}
	var constructionStatId *layoutcid
	if id > 0 && combatStatId != nil && idSum > beltId.value+powerConId.value+inserterId.value+assemblerId.value+labId.value+powerNodeId.value+powerGenId.value+combatStatId.value {
		constructionStatId = r.readcid()
	}
	var fractionateId *layoutcid
	if id > 0 && constructionStatId != nil && idSum > beltId.value+powerConId.value+inserterId.value+assemblerId.value+labId.value+powerNodeId.value+powerGenId.value+combatStatId.value+constructionStatId.value {
		fractionateId = r.readcid()
	}
	var storageId *layoutcid
	if id > 0 && fractionateId != nil && idSum > beltId.value+powerConId.value+inserterId.value+assemblerId.value+labId.value+powerNodeId.value+powerGenId.value+combatStatId.value+constructionStatId.value+fractionateId.value {
		storageId = r.readcid()
	}
	var tankId *layoutcid
	if id > 0 && storageId != nil && idSum > beltId.value+powerConId.value+inserterId.value+assemblerId.value+labId.value+powerNodeId.value+powerGenId.value+combatStatId.value+constructionStatId.value+fractionateId.value+storageId.value {
		tankId = r.readcid()
	}
	var splitterId *layoutcid
	if id > 0 && tankId != nil && idSum > beltId.value+powerConId.value+inserterId.value+assemblerId.value+labId.value+powerNodeId.value+powerGenId.value+combatStatId.value+constructionStatId.value+fractionateId.value+storageId.value+tankId.value {
		splitterId = r.readcid()
	}
	var ejectorId *layoutcid
	if id > 0 && splitterId != nil && idSum > beltId.value+powerConId.value+inserterId.value+assemblerId.value+labId.value+powerNodeId.value+powerGenId.value+combatStatId.value+constructionStatId.value+fractionateId.value+storageId.value+tankId.value+splitterId.value {
		ejectorId = r.readcid()
	}
	var minerId *layoutcid
	if id > 0 && ejectorId != nil && idSum > beltId.value+powerConId.value+inserterId.value+assemblerId.value+labId.value+powerNodeId.value+powerGenId.value+combatStatId.value+constructionStatId.value+fractionateId.value+storageId.value+tankId.value+splitterId.value+ejectorId.value {
		minerId = r.readcid()
	}
	var siloId *layoutcid
	if id > 0 && minerId != nil && idSum > beltId.value+powerConId.value+inserterId.value+assemblerId.value+labId.value+powerNodeId.value+powerGenId.value+combatStatId.value+constructionStatId.value+fractionateId.value+storageId.value+tankId.value+splitterId.value+ejectorId.value+minerId.value {
		siloId = r.readcid()
	}
	var stationId *layoutcid
	if id > 0 && siloId != nil && idSum > beltId.value+powerConId.value+inserterId.value+assemblerId.value+labId.value+powerNodeId.value+powerGenId.value+combatStatId.value+constructionStatId.value+fractionateId.value+storageId.value+tankId.value+splitterId.value+ejectorId.value+minerId.value+siloId.value {
		stationId = r.readcid()
	}
	var dispenserId *layoutcid
	if id > 0 && stationId != nil && idSum > beltId.value+powerConId.value+inserterId.value+assemblerId.value+labId.value+powerNodeId.value+powerGenId.value+combatStatId.value+constructionStatId.value+fractionateId.value+storageId.value+tankId.value+splitterId.value+ejectorId.value+minerId.value+siloId.value+stationId.value {
		dispenserId = r.readcid()
	}
	var turretId *layoutcid
	if id > 0 && dispenserId != nil && idSum > beltId.value+powerConId.value+inserterId.value+assemblerId.value+labId.value+powerNodeId.value+powerGenId.value+combatStatId.value+constructionStatId.value+fractionateId.value+storageId.value+tankId.value+splitterId.value+ejectorId.value+minerId.value+siloId.value+stationId.value+dispenserId.value {
		turretId = r.readcid()
	}
	var beaconId *layoutcid
	if id > 0 && turretId != nil && idSum > beltId.value+powerConId.value+inserterId.value+assemblerId.value+labId.value+powerNodeId.value+powerGenId.value+combatStatId.value+constructionStatId.value+fractionateId.value+storageId.value+tankId.value+splitterId.value+ejectorId.value+minerId.value+siloId.value+stationId.value+dispenserId.value+turretId.value {
		beaconId = r.readcid()
	}
	var fieldGenId *layoutcid
	if id > 0 && beaconId != nil && idSum > beltId.value+powerConId.value+inserterId.value+assemblerId.value+labId.value+powerNodeId.value+powerGenId.value+combatStatId.value+constructionStatId.value+fractionateId.value+storageId.value+tankId.value+splitterId.value+ejectorId.value+minerId.value+siloId.value+stationId.value+dispenserId.value+turretId.value+beaconId.value {
		fieldGenId = r.readcid()
	}
	var battleBaseId *layoutcid
	if id > 0 && fieldGenId != nil && idSum > beltId.value+powerConId.value+inserterId.value+assemblerId.value+labId.value+powerNodeId.value+powerGenId.value+combatStatId.value+constructionStatId.value+fractionateId.value+storageId.value+tankId.value+splitterId.value+ejectorId.value+minerId.value+siloId.value+stationId.value+dispenserId.value+turretId.value+beaconId.value+fieldGenId.value {
		battleBaseId = r.readcid()
	}
	var constructionModuleId *layoutcid
	if id > 0 && battleBaseId != nil && idSum > beltId.value+powerConId.value+inserterId.value+assemblerId.value+labId.value+powerNodeId.value+powerGenId.value+combatStatId.value+constructionStatId.value+fractionateId.value+storageId.value+tankId.value+splitterId.value+ejectorId.value+minerId.value+siloId.value+stationId.value+dispenserId.value+turretId.value+beaconId.value+fieldGenId.value+battleBaseId.value {
		constructionModuleId = r.readcid()
	}
	var combatModuleId *layoutcid
	if id > 0 && constructionModuleId != nil && idSum > beltId.value+powerConId.value+inserterId.value+assemblerId.value+labId.value+powerNodeId.value+powerGenId.value+combatStatId.value+constructionStatId.value+fractionateId.value+storageId.value+tankId.value+splitterId.value+ejectorId.value+minerId.value+siloId.value+stationId.value+dispenserId.value+turretId.value+beaconId.value+fieldGenId.value+battleBaseId.value+constructionModuleId.value {
		combatModuleId = r.readcid()
	}
	var powerAccId *layoutcid
	if id > 0 && combatModuleId != nil && idSum > beltId.value+powerConId.value+inserterId.value+assemblerId.value+labId.value+powerNodeId.value+powerGenId.value+combatStatId.value+constructionStatId.value+fractionateId.value+storageId.value+tankId.value+splitterId.value+ejectorId.value+minerId.value+siloId.value+stationId.value+dispenserId.value+turretId.value+beaconId.value+fieldGenId.value+battleBaseId.value+constructionModuleId.value+combatModuleId.value {
		powerAccId = r.readcid()
	}
	var powerExcId *layoutcid
	if id > 0 && powerAccId != nil && idSum > beltId.value+powerConId.value+inserterId.value+assemblerId.value+labId.value+powerNodeId.value+powerGenId.value+combatStatId.value+constructionStatId.value+fractionateId.value+storageId.value+tankId.value+splitterId.value+ejectorId.value+minerId.value+siloId.value+stationId.value+dispenserId.value+turretId.value+beaconId.value+fieldGenId.value+battleBaseId.value+constructionModuleId.value+combatModuleId.value+powerAccId.value {
		powerExcId = r.readcid()
	}
	var warningId *layoutcid
	if id > 0 && powerExcId != nil && idSum > beltId.value+powerConId.value+inserterId.value+assemblerId.value+labId.value+powerNodeId.value+powerGenId.value+combatStatId.value+constructionStatId.value+fractionateId.value+storageId.value+tankId.value+splitterId.value+ejectorId.value+minerId.value+siloId.value+stationId.value+dispenserId.value+turretId.value+beaconId.value+fieldGenId.value+battleBaseId.value+constructionModuleId.value+combatModuleId.value+powerAccId.value+powerExcId.value {
		warningId = r.readcid()
	}
	var monitorId *layoutcid
	if id > 0 && warningId != nil && idSum > beltId.value+powerConId.value+inserterId.value+assemblerId.value+labId.value+powerNodeId.value+powerGenId.value+combatStatId.value+constructionStatId.value+fractionateId.value+storageId.value+tankId.value+splitterId.value+ejectorId.value+minerId.value+siloId.value+stationId.value+dispenserId.value+turretId.value+beaconId.value+fieldGenId.value+battleBaseId.value+constructionModuleId.value+combatModuleId.value+powerAccId.value+powerExcId.value+warningId.value {
		monitorId = r.readcid()
	}
	var speakerId *layoutcid
	if id > 0 && monitorId != nil && idSum > beltId.value+powerConId.value+inserterId.value+assemblerId.value+labId.value+powerNodeId.value+powerGenId.value+combatStatId.value+constructionStatId.value+fractionateId.value+storageId.value+tankId.value+splitterId.value+ejectorId.value+minerId.value+siloId.value+stationId.value+dispenserId.value+turretId.value+beaconId.value+fieldGenId.value+battleBaseId.value+constructionModuleId.value+combatModuleId.value+powerAccId.value+powerExcId.value+warningId.value+monitorId.value {
		speakerId = r.readcid()
	}
	var spraycoaterId *layoutcid
	if id > 0 && speakerId != nil && idSum > beltId.value+powerConId.value+inserterId.value+assemblerId.value+labId.value+powerNodeId.value+powerGenId.value+combatStatId.value+constructionStatId.value+fractionateId.value+storageId.value+tankId.value+splitterId.value+ejectorId.value+minerId.value+siloId.value+stationId.value+dispenserId.value+turretId.value+beaconId.value+fieldGenId.value+battleBaseId.value+constructionModuleId.value+combatModuleId.value+powerAccId.value+powerExcId.value+warningId.value+monitorId.value+speakerId.value {
		spraycoaterId = r.readcid()
	}
	var pilerId *layoutcid
	if id > 0 && spraycoaterId != nil && idSum > beltId.value+powerConId.value+inserterId.value+assemblerId.value+labId.value+powerNodeId.value+powerGenId.value+combatStatId.value+constructionStatId.value+fractionateId.value+storageId.value+tankId.value+splitterId.value+ejectorId.value+minerId.value+siloId.value+stationId.value+dispenserId.value+turretId.value+beaconId.value+fieldGenId.value+battleBaseId.value+constructionModuleId.value+combatModuleId.value+powerAccId.value+powerExcId.value+warningId.value+monitorId.value+speakerId.value+spraycoaterId.value {
		pilerId = r.readcid()
	}
	var extraInfoId *layoutcid
	if id > 0 && pilerId != nil && idSum > beltId.value+powerConId.value+inserterId.value+assemblerId.value+labId.value+powerNodeId.value+powerGenId.value+combatStatId.value+constructionStatId.value+fractionateId.value+storageId.value+tankId.value+splitterId.value+ejectorId.value+minerId.value+siloId.value+stationId.value+dispenserId.value+turretId.value+beaconId.value+fieldGenId.value+battleBaseId.value+constructionModuleId.value+combatModuleId.value+powerAccId.value+powerExcId.value+warningId.value+monitorId.value+speakerId.value+spraycoaterId.value+pilerId.value {
		extraInfoId = r.readcid()
	}
	if id > 0 && extraInfoId != nil && idSum > beltId.value+powerConId.value+inserterId.value+assemblerId.value+labId.value+powerNodeId.value+powerGenId.value+combatStatId.value+constructionStatId.value+fractionateId.value+storageId.value+tankId.value+splitterId.value+ejectorId.value+minerId.value+siloId.value+stationId.value+dispenserId.value+turretId.value+beaconId.value+fieldGenId.value+battleBaseId.value+constructionModuleId.value+combatModuleId.value+powerAccId.value+powerExcId.value+warningId.value+monitorId.value+speakerId.value+spraycoaterId.value+pilerId.value+extraInfoId.value {
		r.readcid()
	}
	return v
}

// layoutcid 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutcid struct {
	value int64
}

// readcid 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readcid(props ...int64) *layoutcid {
	r.section = "cid"
	v := &layoutcid{}
	var available int64
	available = r.number(1, false)
	var value int64
	if available > 0 {
		value = r.number(3, true)
	}
	v.value = value
	return v
}

// layoutAnimData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutAnimData struct {
}

// readAnimData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readAnimData(props ...int64) *layoutAnimData {
	r.section = "AnimData"
	v := &layoutAnimData{}
	r.skip(20)
	return v
}

// layoutSignData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutSignData struct {
	kind byte
}

// readSignData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readSignData(props ...int64) *layoutSignData {
	r.section = "SignData"
	v := &layoutSignData{}
	id := props[0]
	_ = id
	if id != 0 {
		v.kind = byte(r.number(1, false))
	}
	var iconType int64
	if id != 0 {
		iconType = r.number(1, false)
	}
	if id != 0 {
		r.skip(2)
	}
	if id != 0 && iconType&0x80 != 0 {
		r.skip(4)
	}
	if id != 0 {
		r.skip(4)
	}
	if id != 0 {
		r.skip(4)
	}
	if id != 0 {
		r.skip(4)
	}
	if id != 0 {
		r.skip(4)
	}
	return v
}

// layoutEntityConnPool 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutEntityConnPool struct {
}

// readEntityConnPool 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readEntityConnPool(props ...int64) *layoutEntityConnPool {
	r.section = "EntityConnPool"
	v := &layoutEntityConnPool{}
	id := props[0]
	_ = id
	var available int64
	if id != 0 {
		available = r.number(1, false)
	}
	if id != 0 && available != 0 {
		r.skip(4)
	}
	return v
}

// layoutPrebuildExportStruct 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutPrebuildExportStruct struct {
}

// readPrebuildExportStruct 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readPrebuildExportStruct(props ...int64) *layoutPrebuildExportStruct {
	r.section = "PrebuildExportStruct"
	v := &layoutPrebuildExportStruct{}
	var prebuildData *layoutPrebuildData
	prebuildData = r.readPrebuildData()
	nprebuildConnPool := r.length(16)
	for i := int64(0); i < nprebuildConnPool; i++ {
		r.readEntityConnPool(prebuildData.id)
	}
	return v
}

// layoutPrebuildData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutPrebuildData struct {
	id int64
}

// readPrebuildData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readPrebuildData(props ...int64) *layoutPrebuildData {
	r.section = "PrebuildData"
	v := &layoutPrebuildData{}
	r.expect(r.number(1, false), 7, "PrebuildData.version")
	var id int64
	id = r.number(4, true)
	v.id = id
	if id != 0 {
		r.skip(2)
	}
	if id != 0 {
		r.skip(2)
	}
	if id != 0 {
		r.skip(4)
	}
	if id != 0 {
		r.readVector3()
	}
	if id != 0 {
		r.readQuaternion()
	}
	if id != 0 {
		r.readVector3()
	}
	if id != 0 {
		r.readQuaternion()
	}
	if id != 0 {
		r.skip(4)
	}
	if id != 0 {
		r.skip(4)
	}
	if id != 0 {
		r.skip(4)
	}
	if id != 0 {
		r.skip(1)
	}
	if id != 0 {
		r.skip(4)
	}
	if id != 0 {
		r.skip(4)
	}
	if id != 0 {
		r.skip(4)
	}
	if id != 0 {
		r.skip(1)
	}
	if id != 0 {
		r.skip(2)
	}
	if id != 0 {
		r.skip(2)
	}
	if id != 0 {
		r.skip(4)
	}
	if id != 0 {
		r.skip(4)
	}
	if id != 0 {
		r.skip(4)
	}
	if id != 0 {
		r.skip(4)
	}
	if id != 0 {
		r.skip(1)
	}
	var paramCount int64
	if id != 0 {
		paramCount = r.number(4, true)
	}
	if id != 0 {
		nparameters := r.length(paramCount)
		r.skip(nparameters * 4)
	}
	var contentLength int64
	if id != 0 {
		contentLength = r.number(4, true)
	}
	if id != 0 && contentLength > 0 {
		r.string()
	}
	return v
}

// layoutCraftData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutCraftData struct {
}

// readCraftData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readCraftData(props ...int64) *layoutCraftData {
	r.section = "CraftData"
	v := &layoutCraftData{}
	r.expect(r.number(1, false), 3, "CraftData.version")
	var id int64
	id = r.number(4, true)
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
		r.skip(2)
	}
	if id > 0 {
		r.skip(1)
	}
	var dynamic int64
	if id > 0 {
		dynamic = r.number(1, false)
	}
	var isSpace int64
	if id > 0 {
		isSpace = r.number(1, false)
	}
	if id > 0 {
		r.skip(1)
	}
	if id > 0 && isSpace != 0 {
		r.readVectorLF3()
	}
	if id > 0 && isSpace == 0 {
		r.readVector3()
	}
	if id > 0 {
		r.readQuaternion()
	}
	if id > 0 && dynamic != 0 {
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
		r.flexible()
	}
	if id > 0 {
		r.flexible()
	}
	if id > 0 {
		r.flexible()
	}
	if id > 0 {
		r.flexible()
	}
	if id > 0 {
		r.flexible()
	}
	if id > 0 {
		r.flexible()
	}
	if id > 0 {
		r.flexible()
	}
	if id > 0 {
		r.flexible()
	}
	if id > 0 {
		r.flexible()
	}
	if id > 0 {
		r.flexible()
	}
	if id > 0 {
		r.flexible()
	}
	if id > 0 {
		r.flexible()
	}
	if id > 0 {
		r.flexible()
	}
	if id > 0 {
		r.flexible()
	}
	if id > 0 {
		r.flexible()
	}
	if id > 0 {
		r.flexible()
	}
	if id > 0 {
		r.flexible()
	}
	if id > 0 {
		r.flexible()
	}
	if id > 0 {
		r.flexible()
	}
	if id > 0 {
		r.flexible()
	}
	if id > 0 {
		r.flexible()
	}
	if id > 0 {
		r.flexible()
	}
	if id > 0 {
		r.flexible()
	}
	if id > 0 {
		r.flexible()
	}
	if id > 0 {
		r.flexible()
	}
	return v
}

// layoutEnemyData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutEnemyData struct {
}

// readEnemyData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readEnemyData(props ...int64) *layoutEnemyData {
	r.section = "EnemyData"
	v := &layoutEnemyData{}
	r.expect(r.number(1, false), 0, "EnemyData.version")
	var id int64
	id = r.number(4, true)
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
		r.skip(2)
	}
	if id > 0 {
		r.skip(2)
	}
	var dynamic int64
	if id > 0 {
		dynamic = r.number(1, false)
	}
	var isSpace int64
	if id > 0 {
		isSpace = r.number(1, false)
	}
	if id > 0 {
		r.skip(1)
	}
	if id > 0 {
		r.skip(1)
	}
	if id > 0 && isSpace != 0 {
		r.readVectorLF3()
	}
	if id > 0 && isSpace == 0 {
		r.readVector3()
	}
	if id > 0 {
		r.readQuaternion()
	}
	if id > 0 && dynamic != 0 {
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
	if id > 0 && isSpace == 0 && dynamic == 0 {
		r.skip(2)
	}
	if id > 0 && isSpace == 0 && dynamic == 0 {
		r.skip(2)
	}
	if id > 0 && isSpace == 0 && dynamic == 0 {
		r.skip(2)
	}
	if id > 0 && isSpace == 0 && dynamic == 0 {
		r.skip(2)
	}
	if id > 0 && isSpace == 0 && dynamic == 0 {
		r.skip(2)
	}
	if id > 0 && isSpace != 0 && dynamic == 0 {
		r.skip(2)
	}
	if id > 0 && isSpace != 0 && dynamic == 0 {
		r.skip(2)
	}
	if id > 0 && isSpace != 0 && dynamic == 0 {
		r.skip(2)
	}
	if id > 0 && isSpace != 0 && dynamic == 0 {
		r.skip(2)
	}
	if id > 0 && isSpace != 0 && dynamic == 0 {
		r.skip(2)
	}
	if id > 0 && isSpace != 0 && dynamic == 0 {
		r.skip(2)
	}
	if id > 0 && isSpace != 0 && dynamic != 0 {
		r.skip(2)
	}
	if id > 0 && isSpace != 0 && dynamic != 0 {
		r.skip(2)
	}
	if id > 0 {
		r.flexible()
	}
	return v
}

// layoutVegeData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutVegeData struct {
}

// readVegeData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readVegeData(props ...int64) *layoutVegeData {
	r.section = "VegeData"
	v := &layoutVegeData{}
	r.expect(r.number(1, false), 2, "VegeData.version")
	r.skip(20)
	r.readVector3()
	r.readQuaternion()
	r.readVector3()
	r.skip(1)
	return v
}

// layoutVeinData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutVeinData struct {
}

// readVeinData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readVeinData(props ...int64) *layoutVeinData {
	r.section = "VeinData"
	v := &layoutVeinData{}
	r.expect(r.number(1, false), 1, "VeinData.version")
	id := r.number(4, true)
	r.skip(6)
	amount, item := r.number(4, true), r.number(4, true)
	if id > 0 {
		r.line.veins[id] = veinSource{item, amount}
	}
	r.readVector3()
	r.skip(28)
	return v
}

// layoutRuinData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutRuinData struct {
}

// readRuinData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readRuinData(props ...int64) *layoutRuinData {
	r.section = "RuinData"
	v := &layoutRuinData{}
	r.expect(r.number(1, false), 1, "RuinData.version")
	var id int64
	id = r.number(4, true)
	if id > 0 {
		r.skip(2)
	}
	if id > 0 {
		r.skip(4)
	}
	if id > 0 {
		r.readVector3()
	}
	if id > 0 {
		r.readQuaternion()
	}
	if id > 0 {
		r.skip(4)
	}
	return v
}

// layoutCargoContainer 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutCargoContainer struct {
}

// readCargoContainer 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readCargoContainer(props ...int64) *layoutCargoContainer {
	r.section = "CargoContainer"
	v := &layoutCargoContainer{}
	r.expect(r.number(4, true), 2, "CargoContainer.version")
	var poolCapacity int64
	poolCapacity = r.number(4, true)
	var cursor int64
	cursor = r.number(4, true)
	r.skip(8)
	ncargoPool := r.length(cursor)
	for i := int64(0); i < ncargoPool; i++ {
		r.readCargo()
	}
	nrecycleIds := r.length(poolCapacity)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutCargo 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutCargo struct {
}

// readCargo 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readCargo(props ...int64) *layoutCargo {
	r.section = "Cargo"
	v := &layoutCargo{}
	r.skip(4)
	return v
}

// layoutCargoTraffic 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutCargoTraffic struct {
}

// readCargoTraffic 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readCargoTraffic(props ...int64) *layoutCargoTraffic {
	r.section = "CargoTraffic"
	v := &layoutCargoTraffic{}
	r.expect(r.number(4, true), 4, "CargoTraffic.version")
	var beltCursor int64
	beltCursor = r.number(4, true)
	r.skip(4)
	var beltRecycleCursor int64
	beltRecycleCursor = r.number(4, true)
	var splitterCursor int64
	splitterCursor = r.number(4, true)
	r.skip(4)
	var splitterRecycleCursor int64
	splitterRecycleCursor = r.number(4, true)
	var pathCursor int64
	pathCursor = r.number(4, true)
	r.skip(4)
	var pathRecycleCursor int64
	pathRecycleCursor = r.number(4, true)
	nbeltPool := r.length(beltCursor - 1)
	for i := int64(0); i < nbeltPool; i++ {
		r.readBeltComponent()
	}
	nbeltRecycle := r.length(beltRecycleCursor)
	r.skip(nbeltRecycle * 4)
	nsplitterPool := r.length(splitterCursor - 1)
	for i := int64(0); i < nsplitterPool; i++ {
		r.readSplitterComponent()
	}
	nsplitterRecycle := r.length(splitterRecycleCursor)
	r.skip(nsplitterRecycle * 4)
	npathPool := r.length(pathCursor - 1)
	for i := int64(0); i < npathPool; i++ {
		r.readCargoPathExportStruct()
	}
	npathRecycle := r.length(pathRecycleCursor)
	r.skip(npathRecycle * 4)
	var monitorCursor int64
	monitorCursor = r.number(4, true)
	r.skip(4)
	var monitorRecycleCursor int64
	monitorRecycleCursor = r.number(4, true)
	nmonitorPool := r.length(monitorCursor - 1)
	for i := int64(0); i < nmonitorPool; i++ {
		r.readMonitorComponent()
	}
	nmonitorRecycle := r.length(monitorRecycleCursor)
	r.skip(nmonitorRecycle * 4)
	var spraycoaterCursor int64
	spraycoaterCursor = r.number(4, true)
	r.skip(4)
	var spraycoaterRecycleCursor int64
	spraycoaterRecycleCursor = r.number(4, true)
	nspraycoaterPool := r.length(spraycoaterCursor - 1)
	for i := int64(0); i < nspraycoaterPool; i++ {
		r.readSpraycoaterComponent()
	}
	nspraycoaterRecycle := r.length(spraycoaterRecycleCursor)
	r.skip(nspraycoaterRecycle * 4)
	var pilerCursor int64
	pilerCursor = r.number(4, true)
	r.skip(4)
	var pilerRecycleCursor int64
	pilerRecycleCursor = r.number(4, true)
	npilerPool := r.length(pilerCursor - 1)
	for i := int64(0); i < npilerPool; i++ {
		r.readPilerComponent()
	}
	npilerRecycle := r.length(pilerRecycleCursor)
	r.skip(npilerRecycle * 4)
	return v
}

// layoutBeltComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutBeltComponent struct {
}

// readBeltComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readBeltComponent(props ...int64) *layoutBeltComponent {
	r.section = "BeltComponent"
	v := &layoutBeltComponent{}
	id, entity := r.number(4, true), r.number(4, true)
	r.skip(4)
	path, start, pivot := r.number(4, true), r.number(4, true), r.number(4, true)
	if id > 0 {
		r.beltPivots[entity] = [2]int64{path, start + pivot}
	}
	r.skip(4)
	out := r.number(4, true)
	r.skip(12)
	if id > 0 {
		r.line.addBelt(id, entity, out)
	}
	return v
}

// layoutSplitterComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutSplitterComponent struct {
}

// readSplitterComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readSplitterComponent(props ...int64) *layoutSplitterComponent {
	r.section = "SplitterComponent"
	v := &layoutSplitterComponent{}
	r.expect(r.number(4, true), 2, "SplitterComponent.version")
	r.skip(73)
	return v
}

// layoutCargoPathExportStruct 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutCargoPathExportStruct struct {
}

// readCargoPathExportStruct 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readCargoPathExportStruct(props ...int64) *layoutCargoPathExportStruct {
	r.section = "CargoPathExportStruct"
	v := &layoutCargoPathExportStruct{}
	var available int64
	available = r.number(4, true)
	if available != 0 {
		r.readCargoPath()
	}
	return v
}

// layoutCargoPath 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutCargoPath struct {
}

// readCargoPath 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readCargoPath(props ...int64) *layoutCargoPath {
	r.section = "CargoPath"
	v := &layoutCargoPath{}
	r.expect(r.number(4, true), 1, "CargoPath.version")
	path := r.number(4, true)
	r.skip(4)
	var bufferLength int64
	bufferLength = r.number(4, true)
	r.skip(4)
	var chunkCount int64
	chunkCount = r.number(4, true)
	r.skip(13)
	var numBelts int64
	numBelts = r.number(4, true)
	var numInputPaths int64
	numInputPaths = r.number(4, true)
	nbuffer := r.length(bufferLength)
	start := r.offset
	r.skip(nbuffer)
	r.pathBuffers[path] = r.data[start:r.offset]
	nchunks := r.length(chunkCount * 3)
	r.skip(nchunks * 4)
	npointPos := r.length(bufferLength)
	for i := int64(0); i < npointPos; i++ {
		r.readVector3()
	}
	npointRot := r.length(bufferLength)
	for i := int64(0); i < npointRot; i++ {
		r.readQuaternion()
	}
	nbelts := r.length(numBelts)
	r.skip(nbelts * 4)
	ninputPaths := r.length(numInputPaths)
	r.skip(ninputPaths * 4)
	return v
}

// layoutMonitorComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutMonitorComponent struct {
}

// readMonitorComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readMonitorComponent(props ...int64) *layoutMonitorComponent {
	r.section = "MonitorComponent"
	v := &layoutMonitorComponent{}
	r.expect(r.number(4, true), 3, "MonitorComponent.version")
	r.skip(25)
	var periodTickCount int64
	periodTickCount = r.number(2, false)
	r.skip(2)
	r.flexible()
	r.flexible()
	r.flexible()
	r.skip(1)
	ncargoBytesArray := r.length(periodTickCount)
	r.skip(ncargoBytesArray * 1)
	nperiodCargoBytesArray := r.length(periodTickCount)
	for i := int64(0); i < nperiodCargoBytesArray; i++ {
		r.flexible()
	}
	r.flexible()
	r.skip(8)
	r.flexible()
	r.flexible()
	r.skip(9)
	return v
}

// layoutSpraycoaterComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutSpraycoaterComponent struct {
}

// readSpraycoaterComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readSpraycoaterComponent(props ...int64) *layoutSpraycoaterComponent {
	r.section = "SpraycoaterComponent"
	v := &layoutSpraycoaterComponent{}
	r.expect(r.number(4, true), 2, "SpraycoaterComponent.version")
	r.skip(12)
	r.flexible()
	r.flexible()
	r.flexible()
	r.flexible()
	r.flexible()
	r.flexible()
	r.flexible()
	r.flexible()
	r.skip(1)
	r.flexible()
	return v
}

// layoutPilerComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutPilerComponent struct {
}

// readPilerComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readPilerComponent(props ...int64) *layoutPilerComponent {
	r.section = "PilerComponent"
	v := &layoutPilerComponent{}
	r.expect(r.number(4, true), 1, "PilerComponent.version")
	r.skip(13)
	r.flexible()
	r.flexible()
	r.skip(13)
	return v
}

// layoutFactoryStorage 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutFactoryStorage struct {
	storagePool []*layoutStorageExportStruct
	tankPool    []*layoutTankComponent
}

// readFactoryStorage 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readFactoryStorage(props ...int64) *layoutFactoryStorage {
	r.section = "FactoryStorage"
	v := &layoutFactoryStorage{}
	r.expect(r.number(4, true), 0, "FactoryStorage.version")
	var storageCursor int64
	storageCursor = r.number(4, true)
	r.skip(4)
	var storageRecycleCursor int64
	storageRecycleCursor = r.number(4, true)
	var storagePool []*layoutStorageExportStruct
	nstoragePool := r.length(storageCursor - 1)
	storagePool = make([]*layoutStorageExportStruct, nstoragePool)
	for i := int64(0); i < nstoragePool; i++ {
		storagePool[i] = r.readStorageExportStruct()
	}
	v.storagePool = storagePool
	nstorageRecycle := r.length(storageRecycleCursor)
	r.skip(nstorageRecycle * 4)
	r.skip(4)
	var tankCursor int64
	tankCursor = r.number(4, true)
	var tankRecycleCursor int64
	tankRecycleCursor = r.number(4, true)
	var tankPool []*layoutTankComponent
	ntankPool := r.length(tankCursor - 1)
	tankPool = make([]*layoutTankComponent, ntankPool)
	for i := int64(0); i < ntankPool; i++ {
		tankPool[i] = r.readTankComponent()
	}
	v.tankPool = tankPool
	ntankRecycle := r.length(tankRecycleCursor)
	r.skip(ntankRecycle * 4)
	r.storages = append(r.storages, v)
	return v
}

// layoutStorageExportStruct 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutStorageExportStruct struct {
	storage *layoutStorageComponent
}

// readStorageExportStruct 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readStorageExportStruct(props ...int64) *layoutStorageExportStruct {
	r.section = "StorageExportStruct"
	v := &layoutStorageExportStruct{}
	var available int64
	available = r.number(4, true)
	if available != 0 {
		r.skip(4)
	}
	var storage *layoutStorageComponent
	if available != 0 {
		storage = r.readStorageComponent()
	}
	v.storage = storage
	return v
}

// layoutTankComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutTankComponent struct {
	id            int64
	entityId      int64
	fluidCapacity int64
	fluidCount    int64
	fluidId       int64
}

// readTankComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readTankComponent(props ...int64) *layoutTankComponent {
	r.section = "TankComponent"
	v := &layoutTankComponent{}
	r.expect(r.number(4, true), 1, "TankComponent.version")
	var id int64
	id = r.number(4, true)
	v.id = id
	var entityId int64
	entityId = r.number(4, true)
	v.entityId = entityId
	r.skip(28)
	var fluidCapacity int64
	fluidCapacity = r.number(4, true)
	v.fluidCapacity = fluidCapacity
	var fluidCount int64
	fluidCount = r.number(4, true)
	v.fluidCount = fluidCount
	r.skip(4)
	var fluidId int64
	fluidId = r.number(4, true)
	v.fluidId = fluidId
	r.skip(3)
	return v
}

// layoutPowerSystem 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutPowerSystem struct {
}

// readPowerSystem 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readPowerSystem(props ...int64) *layoutPowerSystem {
	r.section = "PowerSystem"
	v := &layoutPowerSystem{}
	r.expect(r.number(4, true), 0, "PowerSystem.version")
	r.skip(4)
	var genCursor int64
	genCursor = r.number(4, true)
	var genRecycleCursor int64
	genRecycleCursor = r.number(4, true)
	ngenPool := r.length(genCursor - 1)
	for i := int64(0); i < ngenPool; i++ {
		r.readPowerGeneratorComponent()
	}
	ngenRecycle := r.length(genRecycleCursor)
	r.skip(ngenRecycle * 4)
	r.skip(4)
	var nodeCursor int64
	nodeCursor = r.number(4, true)
	var nodeRecycleCursor int64
	nodeRecycleCursor = r.number(4, true)
	nnodePool := r.length(nodeCursor - 1)
	for i := int64(0); i < nnodePool; i++ {
		r.readPowerNodeComponent()
	}
	nnodeRecycle := r.length(nodeRecycleCursor)
	r.skip(nnodeRecycle * 4)
	r.skip(4)
	var consumerCursor int64
	consumerCursor = r.number(4, true)
	var consumerRecycleCursor int64
	consumerRecycleCursor = r.number(4, true)
	nconsumerPool := r.length(consumerCursor - 1)
	for i := int64(0); i < nconsumerPool; i++ {
		r.readPowerConsumerComponent()
	}
	nconsumerRecycle := r.length(consumerRecycleCursor)
	r.skip(nconsumerRecycle * 4)
	r.skip(4)
	var accCursor int64
	accCursor = r.number(4, true)
	var accRecycleCursor int64
	accRecycleCursor = r.number(4, true)
	naccPool := r.length(accCursor - 1)
	for i := int64(0); i < naccPool; i++ {
		r.readPowerAccumulatorComponent()
	}
	naccRecycle := r.length(accRecycleCursor)
	r.skip(naccRecycle * 4)
	r.skip(4)
	var excCursor int64
	excCursor = r.number(4, true)
	var excRecycleCursor int64
	excRecycleCursor = r.number(4, true)
	nexcPool := r.length(excCursor - 1)
	for i := int64(0); i < nexcPool; i++ {
		r.readPowerExchangerComponent()
	}
	nexcRecycle := r.length(excRecycleCursor)
	r.skip(nexcRecycle * 4)
	r.skip(4)
	var netCursor int64
	netCursor = r.number(4, true)
	var netRecycleCursor int64
	netRecycleCursor = r.number(4, true)
	nnetPool := r.length(netCursor)
	for i := int64(0); i < nnetPool; i++ {
		r.readExportStruct_int32__PowerNetwork_()
	}
	nnetRecycle := r.length(netRecycleCursor)
	r.skip(nnetRecycle * 4)
	return v
}

// layoutPowerGeneratorComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutPowerGeneratorComponent struct {
}

// readPowerGeneratorComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readPowerGeneratorComponent(props ...int64) *layoutPowerGeneratorComponent {
	r.section = "PowerGeneratorComponent"
	v := &layoutPowerGeneratorComponent{}
	r.expect(r.number(4, true), 8, "PowerGeneratorComponent.version")
	r.skip(134)
	return v
}

// layoutPowerNodeComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutPowerNodeComponent struct {
}

// readPowerNodeComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readPowerNodeComponent(props ...int64) *layoutPowerNodeComponent {
	r.section = "PowerNodeComponent"
	v := &layoutPowerNodeComponent{}
	r.expect(r.number(4, true), 0, "PowerNodeComponent.version")
	r.skip(25)
	r.readVector3()
	r.skip(8)
	return v
}

// layoutPowerConsumerComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutPowerConsumerComponent struct {
}

// readPowerConsumerComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readPowerConsumerComponent(props ...int64) *layoutPowerConsumerComponent {
	r.section = "PowerConsumerComponent"
	v := &layoutPowerConsumerComponent{}
	r.expect(r.number(4, true), 1, "PowerConsumerComponent.version")
	r.skip(12)
	r.readVector3()
	r.skip(36)
	return v
}

// layoutPowerAccumulatorComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutPowerAccumulatorComponent struct {
}

// readPowerAccumulatorComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readPowerAccumulatorComponent(props ...int64) *layoutPowerAccumulatorComponent {
	r.section = "PowerAccumulatorComponent"
	v := &layoutPowerAccumulatorComponent{}
	r.expect(r.number(4, true), 1, "PowerAccumulatorComponent.version")
	r.skip(45)
	return v
}

// layoutPowerExchangerComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutPowerExchangerComponent struct {
}

// readPowerExchangerComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readPowerExchangerComponent(props ...int64) *layoutPowerExchangerComponent {
	r.section = "PowerExchangerComponent"
	v := &layoutPowerExchangerComponent{}
	r.expect(r.number(4, true), 4, "PowerExchangerComponent.version")
	d := deviceRecord{DeviceState: DeviceState{Kind: "power-exchanger", ComponentID: r.number(4, true), EntityID: r.number(4, true)}}
	r.skip(13)
	target, state := floatValue(r.number(4, false)), floatValue(r.number(4, false))
	r.skip(25)
	empty, full := r.number(4, true), r.number(4, true)
	r.skip(36)
	if target < 0 && state < 0 {
		d.ProductID = empty
	} else if target > 0 && state > 0 {
		d.ProductID = full
	}
	if d.ProductID > 0 {
		r.keepDevice(d)
	}
	return v
}

// layoutExportStruct_int32__PowerNetwork_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutExportStruct_int32__PowerNetwork_ struct {
}

// readExportStruct_int32__PowerNetwork_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readExportStruct_int32__PowerNetwork_(props ...int64) *layoutExportStruct_int32__PowerNetwork_ {
	r.section = "ExportStruct<int32, PowerNetwork>"
	v := &layoutExportStruct_int32__PowerNetwork_{}
	var index int64
	index = r.number(4, true)
	if index != 0 {
		r.readPowerNetwork()
	}
	return v
}

// layoutPowerNetwork 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutPowerNetwork struct {
}

// readPowerNetwork 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readPowerNetwork(props ...int64) *layoutPowerNetwork {
	r.section = "PowerNetwork"
	v := &layoutPowerNetwork{}
	r.expect(r.number(4, true), 1, "PowerNetwork.version")
	r.skip(20)
	var numNodes int64
	numNodes = r.number(4, true)
	var numConsumers int64
	numConsumers = r.number(4, true)
	var numGenerator int64
	numGenerator = r.number(4, true)
	var numAccumulator int64
	numAccumulator = r.number(4, true)
	var numExchanger int64
	numExchanger = r.number(4, true)
	nnodes := r.length(numNodes)
	for i := int64(0); i < nnodes; i++ {
		r.readPowerNetworkStructures()
	}
	nconsumers := r.length(numConsumers)
	r.skip(nconsumers * 4)
	ngenerators := r.length(numGenerator)
	r.skip(ngenerators * 4)
	naccumulators := r.length(numAccumulator)
	r.skip(naccumulators * 4)
	nexchangers := r.length(numExchanger)
	r.skip(nexchangers * 4)
	return v
}

// layoutPowerNetworkStructures 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutPowerNetworkStructures struct {
}

// readPowerNetworkStructures 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readPowerNetworkStructures(props ...int64) *layoutPowerNetworkStructures {
	r.section = "PowerNetworkStructures"
	v := &layoutPowerNetworkStructures{}
	r.expect(r.number(4, true), 0, "PowerNetworkStructures.version")
	r.skip(36)
	var numConns int64
	numConns = r.number(4, true)
	var numLines int64
	numLines = r.number(4, true)
	var numConsumers int64
	numConsumers = r.number(4, true)
	nconns := r.length(numConns)
	r.skip(nconns * 4)
	nlines := r.length(numLines)
	r.skip(nlines * 4)
	nconsumers := r.length(numConsumers)
	r.skip(nconsumers * 4)
	return v
}

// layoutFactorySystem 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutFactorySystem struct {
}

// readFactorySystem 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readFactorySystem(props ...int64) *layoutFactorySystem {
	r.section = "FactorySystem"
	v := &layoutFactorySystem{}
	r.expect(r.number(4, true), 1, "FactorySystem.version")
	r.skip(4)
	var minerCursor int64
	minerCursor = r.number(4, true)
	var minerRecycleCursor int64
	minerRecycleCursor = r.number(4, true)
	nminerPool := r.length(minerCursor - 1)
	for i := int64(0); i < nminerPool; i++ {
		r.readMinerComponent()
	}
	nminerRecycle := r.length(minerRecycleCursor)
	r.skip(nminerRecycle * 4)
	r.skip(4)
	var inserterCursor int64
	inserterCursor = r.number(4, true)
	var inserterRecycleCursor int64
	inserterRecycleCursor = r.number(4, true)
	ninserterPool := r.length(inserterCursor - 1)
	for i := int64(0); i < ninserterPool; i++ {
		r.readInserterComponentV2()
	}
	ninserterRecycle := r.length(inserterRecycleCursor)
	r.skip(ninserterRecycle * 4)
	r.skip(4)
	var assemblerCursor int64
	assemblerCursor = r.number(4, true)
	var assemblerRecycleCursor int64
	assemblerRecycleCursor = r.number(4, true)
	nassemblerPool := r.length(assemblerCursor - 1)
	for i := int64(0); i < nassemblerPool; i++ {
		r.readAssemblerComponent()
	}
	nassemblerRecycle := r.length(assemblerRecycleCursor)
	r.skip(nassemblerRecycle * 4)
	r.skip(4)
	var fractionateCursor int64
	fractionateCursor = r.number(4, true)
	var fractionateRecycleCursor int64
	fractionateRecycleCursor = r.number(4, true)
	nfractionatePool := r.length(fractionateCursor - 1)
	for i := int64(0); i < nfractionatePool; i++ {
		r.readFractionatorComponent()
	}
	nfractionateRecycle := r.length(fractionateRecycleCursor)
	r.skip(nfractionateRecycle * 4)
	r.skip(4)
	var ejectorCursor int64
	ejectorCursor = r.number(4, true)
	var ejectorRecycleCursor int64
	ejectorRecycleCursor = r.number(4, true)
	nejectorPool := r.length(ejectorCursor - 1)
	for i := int64(0); i < nejectorPool; i++ {
		r.readEjectorComponent()
	}
	nejectorRecycle := r.length(ejectorRecycleCursor)
	r.skip(nejectorRecycle * 4)
	r.skip(4)
	var siloCursor int64
	siloCursor = r.number(4, true)
	var siloRecycleCursor int64
	siloRecycleCursor = r.number(4, true)
	nsiloPool := r.length(siloCursor - 1)
	for i := int64(0); i < nsiloPool; i++ {
		r.readSiloComponent()
	}
	nsiloRecycle := r.length(siloRecycleCursor)
	r.skip(nsiloRecycle * 4)
	r.skip(4)
	var labCursor int64
	labCursor = r.number(4, true)
	var labRecycleCursor int64
	labRecycleCursor = r.number(4, true)
	nlabPool := r.length(labCursor - 1)
	for i := int64(0); i < nlabPool; i++ {
		r.readLabComponent()
	}
	nlabRecycle := r.length(labRecycleCursor)
	r.skip(nlabRecycle * 4)
	return v
}

// layoutMinerComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutMinerComponent struct {
}

// readMinerComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readMinerComponent(props ...int64) *layoutMinerComponent {
	r.section = "MinerComponent"
	v := &layoutMinerComponent{}
	r.expect(r.number(4, true), 2, "MinerComponent.version")
	d := r.deviceHeader("miner")
	d.minerType = r.number(4, true)
	d.speed = r.number(4, true)
	d.speedDamper = floatValue(r.number(4, false))
	d.Progress = r.number(4, true)
	d.Cycle = r.number(4, true)
	d.target = r.number(4, true)
	r.skip(4)
	d.Veins = r.lineValues(r.length(r.number(4, true)), 4)
	d.currentVein = r.number(4, true)
	r.skip(4)
	d.ProductID = r.number(4, true)
	d.Buffered = r.number(4, true)
	r.skip(8)
	if d.ComponentID > 0 {
		for _, id := range d.Veins {
			if v, ok := r.line.veins[id]; ok {
				d.ResourceRemaining += v.amount
			}
		}
		d.BufferItemID = d.ProductID
		if d.minerType == 2 || d.minerType == 3 {
			d.ProductID = 0
			if d.currentVein >= 0 && d.currentVein < int64(len(d.Veins)) {
				d.ProductID = r.line.veins[d.Veins[d.currentVein]].item
			}
		}
		r.keepDevice(d)
	}
	return v
}

// layoutInserterComponentV2 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutInserterComponentV2 struct {
}

// readInserterComponentV2 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readInserterComponentV2(props ...int64) *layoutInserterComponentV2 {
	r.section = "InserterComponentV2"
	v := &layoutInserterComponentV2{}
	r.readInserterComponent()
	r.readInserterPose()
	return v
}

// layoutInserterComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutInserterComponent struct {
}

// readInserterComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readInserterComponent(props ...int64) *layoutInserterComponent {
	r.section = "InserterComponent"
	v := &layoutInserterComponent{}
	r.expect(r.number(4, true), 4, "InserterComponent.version")
	id, entity := r.number(4, true), r.number(4, true)
	r.skip(4)
	stage := r.number(1, false)
	r.skip(12)
	pick, drop := r.number(4, true), r.number(4, true)
	r.skip(4)
	stackOutput := r.number(1, false)
	bidirectional := r.number(1, false) != 0
	r.skip(2)
	offset := r.number(2, true)
	filter := r.number(2, true)
	item := r.number(2, true)
	r.skip(1)
	count := r.number(2, true)
	r.skip(2)
	if id > 0 && item > 0 && count > 0 {
		v := DropObservation{Source: pick, Sorter: entity, Target: drop, Stage: stage, ItemID: item, Count: count}
		if pivot, ok := r.beltPivots[drop]; ok {
			v.Belt = r.line.beltByEntity[drop]
			pos := pivot[1] + offset
			if stage == 2 && !bidirectional && stackOutput == 1 {
				v.Refusal = dropRefusal(r.pathBuffers[pivot[0]], pos)
			}
		}
		r.line.drops[entity] = v
	}
	if id > 0 {
		r.line.sorters = append(r.line.sorters, lineSorter{entity, pick, drop, filter, bidirectional})
	}
	return v
}

// layoutInserterPose 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutInserterPose struct {
}

// readInserterPose 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readInserterPose(props ...int64) *layoutInserterPose {
	r.section = "InserterPose"
	v := &layoutInserterPose{}
	r.expect(r.number(1, false), 0, "InserterPose.version")
	r.readVector3()
	r.readQuaternion()
	r.skip(2)
	return v
}

// layoutAssemblerComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutAssemblerComponent struct {
}

// readAssemblerComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readAssemblerComponent(props ...int64) *layoutAssemblerComponent {
	r.section = "AssemblerComponent"
	v := &layoutAssemblerComponent{}
	r.expect(r.number(4, true), 6, "AssemblerComponent.version")
	id, entity := r.number(4, true), r.number(4, true)
	r.skip(4)
	active := r.number(1, false) != 0
	r.skip(4)
	progress := r.number(4, true)
	r.skip(26)
	recipeId := r.number(2, true)
	var kind int64
	if recipeId > 0 {
		kind = r.number(1, false)
	}
	var requires, served, products, produced []int64
	var numRequires int64
	if recipeId > 0 {
		numRequires = r.number(1, false)
	}
	if recipeId > 0 {
		nrequires := r.length(numRequires)
		requires = r.lineValues(nrequires, 2)
	}
	var numServed int64
	if recipeId > 0 {
		numServed = r.number(1, false)
	}
	if recipeId > 0 {
		nserved := r.length(numServed)
		served = r.lineValues(nserved, 4)
	}
	var numIncServed int64
	if recipeId > 0 {
		numIncServed = r.number(1, false)
	}
	if recipeId > 0 {
		nincServed := r.length(numIncServed)
		r.skip(nincServed * 4)
	}
	var needs []int64
	var numNeeds int64
	if recipeId > 0 {
		numNeeds = r.number(1, false)
	}
	if recipeId > 0 {
		nneeds := r.length(numNeeds)
		needs = r.lineValues(nneeds, 2)
	}
	var numProducts int64
	if recipeId > 0 {
		numProducts = r.number(1, false)
	}
	if recipeId > 0 {
		nproducts := r.length(numProducts)
		products = r.lineValues(nproducts, 2)
	}
	var numProduced int64
	if recipeId > 0 {
		numProduced = r.number(1, false)
	}
	if recipeId > 0 {
		nproduced := r.length(numProduced)
		produced = r.lineValues(nproduced, 4)
	}
	if id > 0 && recipeId > 0 {
		if entity <= 0 || entity > int64(len(r.line.sign)) || len(products) != len(produced) || len(requires) != len(served) {
			r.fail("生产设备引用或缓冲无效")
		}
		r.line.machines = append(r.line.machines, circuitRecord{
			id: id, entity: entity, recipe: recipeId, kind: kind, active: active, progress: progress,
			requires: requires, served: served, needs: needs, products: products, produced: produced,
		})
	}
	return v
}

// layoutFractionatorComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutFractionatorComponent struct {
}

// readFractionatorComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readFractionatorComponent(props ...int64) *layoutFractionatorComponent {
	r.section = "FractionatorComponent"
	v := &layoutFractionatorComponent{}
	r.expect(r.number(4, true), 5, "FractionatorComponent.version")
	d := r.deviceHeader("fractionator")
	for i := range d.belts {
		d.belts[i] = r.number(4, true)
	}
	for i := range d.out {
		d.out[i] = r.number(1, false) != 0
	}
	d.working = r.number(1, false) != 0
	r.skip(4)
	d.FluidID = r.number(4, true)
	d.ProductID = r.number(4, true)
	d.FluidInput = r.number(4, true)
	r.skip(8)
	d.Buffered = r.number(4, true)
	d.FluidReturn = r.number(4, true)
	r.skip(8)
	d.Progress = r.number(4, true)
	r.expect(r.number(1, false), 0, "FractionatorComponent.reserved")
	r.skip(6)
	d.Capacity = r.number(4, true)
	d.ReturnCapacity = r.number(4, true)
	r.skip(12)
	r.keepDevice(d)
	return v
}

// layoutEjectorComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutEjectorComponent struct {
}

// readEjectorComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readEjectorComponent(props ...int64) *layoutEjectorComponent {
	r.section = "EjectorComponent"
	v := &layoutEjectorComponent{}
	r.expect(r.number(4, true), 4, "EjectorComponent.version")
	r.skip(68)
	r.readVector3()
	r.skip(4)
	r.readQuaternion()
	r.readVector3()
	return v
}

// layoutSiloComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutSiloComponent struct {
}

// readSiloComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readSiloComponent(props ...int64) *layoutSiloComponent {
	r.section = "SiloComponent"
	v := &layoutSiloComponent{}
	r.expect(r.number(4, true), 3, "SiloComponent.version")
	r.skip(52)
	r.readVector3()
	r.readQuaternion()
	return v
}

// layoutLabComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutLabComponent struct {
}

// readLabComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readLabComponent(props ...int64) *layoutLabComponent {
	r.section = "LabComponent"
	v := &layoutLabComponent{}
	r.expect(r.number(4, true), 5, "LabComponent.version")
	d := r.deviceHeader("matrix-lab")
	d.NextLabID = r.number(4, true)
	d.working = r.number(1, false) != 0
	d.Progress = r.number(4, true)
	r.skip(26)
	d.speed = r.number(4, true)
	research := r.number(1, false) != 0
	d.RecipeID = r.number(4, true)
	d.TechID = r.number(4, true)
	if research {
		d.Kind = "research-lab"
		d.MatrixPoints = r.deviceArray(4)
		r.skip(r.length(r.number(4, true)) * 4)
		r.skip(r.length(r.number(4, true)) * 4)
	} else if d.RecipeID > 0 {
		r.skip(8)
		d.requires = r.deviceArray(4)
		d.served = r.deviceArray(4)
		r.skip(r.length(r.number(4, true)) * 4)
		r.skip(r.length(r.number(4, true)) * 4)
		d.products = r.deviceArray(4)
		d.produced = r.deviceArray(4)
		if len(d.requires) != len(d.served) || len(d.products) != len(d.produced) {
			r.fail("矩阵研究站缓冲长度不匹配")
		}
		if len(d.products) == 1 {
			d.ProductID = d.products[0]
			d.Buffered = d.produced[0]
		}
	}
	r.keepDevice(d)
	return v
}

// layoutEnemyDFGroundSystem 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutEnemyDFGroundSystem struct {
}

// readEnemyDFGroundSystem 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readEnemyDFGroundSystem(props ...int64) *layoutEnemyDFGroundSystem {
	r.section = "EnemyDFGroundSystem"
	v := &layoutEnemyDFGroundSystem{}
	r.expect(r.number(4, true), 0, "EnemyDFGroundSystem.version")
	r.skip(12)
	r.readDataPool_EnemyBuilderComponent_()
	r.readObjectPool_DFGBaseComponent_()
	r.readDataPool_DFGConnectorComponent_()
	r.readDataPool_DFGReplicatorComponent_()
	r.readDataPool_DFGTurretComponent_()
	r.readDataPool_DFGShieldComponent_()
	r.readDataPool_EnemyUnitComponent_()
	return v
}

// layoutDataPool_EnemyBuilderComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPool_EnemyBuilderComponent_ struct {
}

// readDataPool_EnemyBuilderComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPool_EnemyBuilderComponent_(props ...int64) *layoutDataPool_EnemyBuilderComponent_ {
	r.section = "DataPool<EnemyBuilderComponent>"
	v := &layoutDataPool_EnemyBuilderComponent_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readEnemyBuilderComponent()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutEnemyBuilderComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutEnemyBuilderComponent struct {
}

// readEnemyBuilderComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readEnemyBuilderComponent(props ...int64) *layoutEnemyBuilderComponent {
	r.section = "EnemyBuilderComponent"
	v := &layoutEnemyBuilderComponent{}
	r.expect(r.number(1, false), 0, "EnemyBuilderComponent.version")
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
		r.skip(2)
	}
	if id > 0 {
		r.skip(2)
	}
	return v
}

// layoutObjectPool_DFGBaseComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutObjectPool_DFGBaseComponent_ struct {
}

// readObjectPool_DFGBaseComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readObjectPool_DFGBaseComponent_(props ...int64) *layoutObjectPool_DFGBaseComponent_ {
	r.section = "ObjectPool<DFGBaseComponent>"
	v := &layoutObjectPool_DFGBaseComponent_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readExportStruct_int32__DFGBaseComponent_()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutExportStruct_int32__DFGBaseComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutExportStruct_int32__DFGBaseComponent_ struct {
}

// readExportStruct_int32__DFGBaseComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readExportStruct_int32__DFGBaseComponent_(props ...int64) *layoutExportStruct_int32__DFGBaseComponent_ {
	r.section = "ExportStruct<int32, DFGBaseComponent>"
	v := &layoutExportStruct_int32__DFGBaseComponent_{}
	var index int64
	index = r.number(4, true)
	if index != 0 {
		r.readDFGBaseComponent()
	}
	return v
}

// layoutDFGBaseComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDFGBaseComponent struct {
}

// readDFGBaseComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDFGBaseComponent(props ...int64) *layoutDFGBaseComponent {
	r.section = "DFGBaseComponent"
	v := &layoutDFGBaseComponent{}
	r.expect(r.number(1, false), 2, "DFGBaseComponent.version")
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
		nforms := r.length(3)
		for i := int64(0); i < nforms; i++ {
			r.readEnemyFormation()
		}
	}
	if id > 0 {
		r.readEvolveData()
	}
	if id > 0 {
		r.readHatredList()
	}
	var incomingSkillsCursor int64
	if id > 0 {
		incomingSkillsCursor = r.number(4, true)
	}
	if id > 0 {
		nincomingSkills := r.length(incomingSkillsCursor)
		for i := int64(0); i < nincomingSkills; i++ {
			r.readKeyValuePair_int32__int32_()
		}
	}
	var numPbuilders int64
	if id > 0 {
		numPbuilders = r.number(4, true)
	}
	if id > 0 {
		npbuilder := r.length(numPbuilders)
		for i := int64(0); i < npbuilder; i++ {
			r.readGrowthPattern_DFGround_Builder()
		}
	}
	if id > 0 {
		r.skip(4)
	}
	return v
}

// layoutEnemyFormation 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutEnemyFormation struct {
}

// readEnemyFormation 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readEnemyFormation(props ...int64) *layoutEnemyFormation {
	r.section = "EnemyFormation"
	v := &layoutEnemyFormation{}
	r.expect(r.number(1, false), 0, "EnemyFormation.version")
	var portCount int64
	portCount = r.number(2, false)
	var vacancyCursor int64
	vacancyCursor = r.number(2, false)
	nunits := r.length(portCount)
	r.skip(nunits * 1)
	if portCount <= 255 {
		nvacancies_uint8 := r.length(vacancyCursor)
		r.skip(nvacancies_uint8 * 1)
	}
	if portCount > 255 {
		nvacancies_uint16 := r.length(vacancyCursor)
		r.skip(nvacancies_uint16 * 2)
	}
	return v
}

// layoutEvolveData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutEvolveData struct {
}

// readEvolveData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readEvolveData(props ...int64) *layoutEvolveData {
	r.section = "EvolveData"
	v := &layoutEvolveData{}
	r.expect(r.number(1, false), 1, "EvolveData.version")
	r.skip(48)
	return v
}

// layoutHatredList 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutHatredList struct {
}

// readHatredList 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readHatredList(props ...int64) *layoutHatredList {
	r.section = "HatredList"
	v := &layoutHatredList{}
	r.expect(r.number(1, false), 0, "HatredList.version")
	r.skip(64)
	return v
}

// layoutGrowthPattern_DFGround_Builder 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutGrowthPattern_DFGround_Builder struct {
}

// readGrowthPattern_DFGround_Builder 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readGrowthPattern_DFGround_Builder(props ...int64) *layoutGrowthPattern_DFGround_Builder {
	r.section = "GrowthPattern_DFGround_Builder"
	v := &layoutGrowthPattern_DFGround_Builder{}
	r.expect(r.number(1, false), 0, "GrowthPattern_DFGround_Builder.version")
	var protoId int64
	protoId = r.number(2, true)
	var instId int64
	instId = r.flexible()
	if instId > 0 {
		r.skip(2)
	}
	r.readVector3()
	r.readQuaternion()
	if protoId == 8127 {
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

// layoutDataPool_DFGConnectorComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPool_DFGConnectorComponent_ struct {
}

// readDataPool_DFGConnectorComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPool_DFGConnectorComponent_(props ...int64) *layoutDataPool_DFGConnectorComponent_ {
	r.section = "DataPool<DFGConnectorComponent>"
	v := &layoutDataPool_DFGConnectorComponent_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readDFGConnectorComponent()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutDFGConnectorComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDFGConnectorComponent struct {
}

// readDFGConnectorComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDFGConnectorComponent(props ...int64) *layoutDFGConnectorComponent {
	r.section = "DFGConnectorComponent"
	v := &layoutDFGConnectorComponent{}
	r.expect(r.number(1, false), 0, "DFGConnectorComponent.version")
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

// layoutDataPool_DFGReplicatorComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPool_DFGReplicatorComponent_ struct {
}

// readDataPool_DFGReplicatorComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPool_DFGReplicatorComponent_(props ...int64) *layoutDataPool_DFGReplicatorComponent_ {
	r.section = "DataPool<DFGReplicatorComponent>"
	v := &layoutDataPool_DFGReplicatorComponent_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readDFGReplicatorComponent()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutDFGReplicatorComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDFGReplicatorComponent struct {
}

// readDFGReplicatorComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDFGReplicatorComponent(props ...int64) *layoutDFGReplicatorComponent {
	r.section = "DFGReplicatorComponent"
	v := &layoutDFGReplicatorComponent{}
	r.expect(r.number(1, false), 0, "DFGReplicatorComponent.version")
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

// layoutDataPool_DFGTurretComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPool_DFGTurretComponent_ struct {
}

// readDataPool_DFGTurretComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPool_DFGTurretComponent_(props ...int64) *layoutDataPool_DFGTurretComponent_ {
	r.section = "DataPool<DFGTurretComponent>"
	v := &layoutDataPool_DFGTurretComponent_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readDFGTurretComponent()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutDFGTurretComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDFGTurretComponent struct {
}

// readDFGTurretComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDFGTurretComponent(props ...int64) *layoutDFGTurretComponent {
	r.section = "DFGTurretComponent"
	v := &layoutDFGTurretComponent{}
	r.expect(r.number(1, false), 0, "DFGTurretComponent.version")
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

// layoutDataPool_DFGShieldComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPool_DFGShieldComponent_ struct {
}

// readDataPool_DFGShieldComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPool_DFGShieldComponent_(props ...int64) *layoutDataPool_DFGShieldComponent_ {
	r.section = "DataPool<DFGShieldComponent>"
	v := &layoutDataPool_DFGShieldComponent_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readDFGShieldComponent()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutDFGShieldComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDFGShieldComponent struct {
}

// readDFGShieldComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDFGShieldComponent(props ...int64) *layoutDFGShieldComponent {
	r.section = "DFGShieldComponent"
	v := &layoutDFGShieldComponent{}
	r.expect(r.number(1, false), 0, "DFGShieldComponent.version")
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
	return v
}

// layoutDataPool_EnemyUnitComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPool_EnemyUnitComponent_ struct {
}

// readDataPool_EnemyUnitComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPool_EnemyUnitComponent_(props ...int64) *layoutDataPool_EnemyUnitComponent_ {
	r.section = "DataPool<EnemyUnitComponent>"
	v := &layoutDataPool_EnemyUnitComponent_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readEnemyUnitComponent()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutEnemyUnitComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutEnemyUnitComponent struct {
}

// readEnemyUnitComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readEnemyUnitComponent(props ...int64) *layoutEnemyUnitComponent {
	r.section = "EnemyUnitComponent"
	v := &layoutEnemyUnitComponent{}
	r.expect(r.number(1, false), 2, "EnemyUnitComponent.version")
	r.flexible()
	r.flexible()
	r.flexible()
	r.skip(9)
	r.flexible()
	r.skip(8)
	r.readVector3()
	r.readTargetQueue()
	r.readHatredList()
	r.skip(12)
	return v
}

// layoutTargetQueue 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutTargetQueue struct {
}

// readTargetQueue 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readTargetQueue(props ...int64) *layoutTargetQueue {
	r.section = "TargetQueue"
	v := &layoutTargetQueue{}
	r.expect(r.number(1, false), 0, "TargetQueue.version")
	var count int64
	count = r.number(1, false)
	if count > 0 {
		r.readVector3()
	}
	if count > 1 {
		r.readVector3()
	}
	r.skip(12)
	return v
}

// layoutCombatGroundSystem 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutCombatGroundSystem struct {
}

// readCombatGroundSystem 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readCombatGroundSystem(props ...int64) *layoutCombatGroundSystem {
	r.section = "CombatGroundSystem"
	v := &layoutCombatGroundSystem{}
	r.expect(r.number(4, true), 0, "CombatGroundSystem.version")
	r.readDataPool_UnitComponent_()
	r.readDataPool_FleetComponent_()
	r.readObjectPool_CombatModuleComponent_()
	return v
}

// layoutDataPool_UnitComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPool_UnitComponent_ struct {
}

// readDataPool_UnitComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPool_UnitComponent_(props ...int64) *layoutDataPool_UnitComponent_ {
	r.section = "DataPool<UnitComponent>"
	v := &layoutDataPool_UnitComponent_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readUnitComponent()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutUnitComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutUnitComponent struct {
}

// readUnitComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readUnitComponent(props ...int64) *layoutUnitComponent {
	r.section = "UnitComponent"
	v := &layoutUnitComponent{}
	r.expect(r.number(4, true), 0, "UnitComponent.version")
	r.skip(34)
	r.readHatredList()
	return v
}

// layoutDataPool_FleetComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPool_FleetComponent_ struct {
}

// readDataPool_FleetComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPool_FleetComponent_(props ...int64) *layoutDataPool_FleetComponent_ {
	r.section = "DataPool<FleetComponent>"
	v := &layoutDataPool_FleetComponent_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readFleetComponent()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutFleetComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutFleetComponent struct {
}

// readFleetComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readFleetComponent(props ...int64) *layoutFleetComponent {
	r.section = "FleetComponent"
	v := &layoutFleetComponent{}
	r.expect(r.number(4, true), 0, "FleetComponent.version")
	r.skip(12)
	r.readHatredList()
	r.skip(12)
	r.readVectorLF3()
	r.skip(5)
	return v
}

// layoutObjectPool_CombatModuleComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutObjectPool_CombatModuleComponent_ struct {
}

// readObjectPool_CombatModuleComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readObjectPool_CombatModuleComponent_(props ...int64) *layoutObjectPool_CombatModuleComponent_ {
	r.section = "ObjectPool<CombatModuleComponent>"
	v := &layoutObjectPool_CombatModuleComponent_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readExportStruct_int32__CombatModuleComponent_()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutExportStruct_int32__CombatModuleComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutExportStruct_int32__CombatModuleComponent_ struct {
}

// readExportStruct_int32__CombatModuleComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readExportStruct_int32__CombatModuleComponent_(props ...int64) *layoutExportStruct_int32__CombatModuleComponent_ {
	r.section = "ExportStruct<int32, CombatModuleComponent>"
	v := &layoutExportStruct_int32__CombatModuleComponent_{}
	var index int64
	index = r.number(4, true)
	if index != 0 {
		r.readCombatModuleComponent()
	}
	return v
}

// layoutDefenseSystem 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDefenseSystem struct {
}

// readDefenseSystem 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDefenseSystem(props ...int64) *layoutDefenseSystem {
	r.section = "DefenseSystem"
	v := &layoutDefenseSystem{}
	r.expect(r.number(4, true), 2, "DefenseSystem.version")
	var turrets *layoutDataPool_TurretComponent_
	turrets = r.readDataPool_TurretComponent_()
	r.readDataPool_BeaconComponent_()
	r.readDataPool_FieldGeneratorComponent_()
	r.readObjectPool_BattleBaseComponent_()
	r.skip(4)
	var otherGlobalTargetCursor int64
	otherGlobalTargetCursor = r.number(4, true)
	nglobalTargets := r.length(otherGlobalTargetCursor)
	for i := int64(0); i < nglobalTargets; i++ {
		r.readTimedSkillTarget()
	}
	if turrets.cursor-turrets.recycleCursor-1 > 0 {
		r.skip(4)
	}
	if turrets.cursor-turrets.recycleCursor-1 > 0 {
		r.skip(4)
	}
	if turrets.cursor-turrets.recycleCursor-1 > 0 {
		r.skip(4)
	}
	if turrets.cursor-turrets.recycleCursor-1 > 0 {
		r.skip(4)
	}
	if turrets.cursor-turrets.recycleCursor-1 > 0 {
		r.skip(4)
	}
	if turrets.cursor-turrets.recycleCursor-1 > 0 {
		r.skip(4)
	}
	if turrets.cursor-turrets.recycleCursor-1 > 0 {
		r.skip(4)
	}
	if turrets.cursor-turrets.recycleCursor-1 > 0 {
		r.skip(1)
	}
	return v
}

// layoutDataPool_TurretComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPool_TurretComponent_ struct {
	cursor        int64
	recycleCursor int64
}

// readDataPool_TurretComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPool_TurretComponent_(props ...int64) *layoutDataPool_TurretComponent_ {
	r.section = "DataPool<TurretComponent>"
	v := &layoutDataPool_TurretComponent_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	v.cursor = cursor
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	v.recycleCursor = recycleCursor
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readTurretComponent()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutTurretComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutTurretComponent struct {
}

// readTurretComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readTurretComponent(props ...int64) *layoutTurretComponent {
	r.section = "TurretComponent"
	v := &layoutTurretComponent{}
	r.expect(r.number(4, true), 4, "TurretComponent.version")
	r.skip(32)
	r.readVector3()
	r.readVector3()
	r.skip(68)
	r.readVector2()
	return v
}

// layoutDataPool_BeaconComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPool_BeaconComponent_ struct {
}

// readDataPool_BeaconComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPool_BeaconComponent_(props ...int64) *layoutDataPool_BeaconComponent_ {
	r.section = "DataPool<BeaconComponent>"
	v := &layoutDataPool_BeaconComponent_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readBeaconComponent()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutBeaconComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutBeaconComponent struct {
}

// readBeaconComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readBeaconComponent(props ...int64) *layoutBeaconComponent {
	r.section = "BeaconComponent"
	v := &layoutBeaconComponent{}
	r.expect(r.number(4, true), 1, "BeaconComponent.version")
	r.skip(16)
	return v
}

// layoutDataPool_FieldGeneratorComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPool_FieldGeneratorComponent_ struct {
}

// readDataPool_FieldGeneratorComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPool_FieldGeneratorComponent_(props ...int64) *layoutDataPool_FieldGeneratorComponent_ {
	r.section = "DataPool<FieldGeneratorComponent>"
	v := &layoutDataPool_FieldGeneratorComponent_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readFieldGeneratorComponent()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutFieldGeneratorComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutFieldGeneratorComponent struct {
}

// readFieldGeneratorComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readFieldGeneratorComponent(props ...int64) *layoutFieldGeneratorComponent {
	r.section = "FieldGeneratorComponent"
	v := &layoutFieldGeneratorComponent{}
	r.expect(r.number(4, true), 0, "FieldGeneratorComponent.version")
	r.skip(12)
	r.readQuaternion()
	r.skip(32)
	return v
}

// layoutObjectPool_BattleBaseComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutObjectPool_BattleBaseComponent_ struct {
}

// readObjectPool_BattleBaseComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readObjectPool_BattleBaseComponent_(props ...int64) *layoutObjectPool_BattleBaseComponent_ {
	r.section = "ObjectPool<BattleBaseComponent>"
	v := &layoutObjectPool_BattleBaseComponent_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readExportStruct_int32__BattleBaseComponent_()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutExportStruct_int32__BattleBaseComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutExportStruct_int32__BattleBaseComponent_ struct {
}

// readExportStruct_int32__BattleBaseComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readExportStruct_int32__BattleBaseComponent_(props ...int64) *layoutExportStruct_int32__BattleBaseComponent_ {
	r.section = "ExportStruct<int32, BattleBaseComponent>"
	v := &layoutExportStruct_int32__BattleBaseComponent_{}
	var index int64
	index = r.number(4, true)
	if index != 0 {
		r.readBattleBaseComponent()
	}
	return v
}

// layoutBattleBaseComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutBattleBaseComponent struct {
}

// readBattleBaseComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readBattleBaseComponent(props ...int64) *layoutBattleBaseComponent {
	r.section = "BattleBaseComponent"
	v := &layoutBattleBaseComponent{}
	r.expect(r.number(4, true), 1, "BattleBaseComponent.version")
	r.skip(57)
	return v
}

// layoutTimedSkillTarget 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutTimedSkillTarget struct {
}

// readTimedSkillTarget 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readTimedSkillTarget(props ...int64) *layoutTimedSkillTarget {
	r.section = "TimedSkillTarget"
	v := &layoutTimedSkillTarget{}
	r.expect(r.number(4, true), 1, "TimedSkillTarget.version")
	r.skip(16)
	return v
}

// layoutPlanetATField 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutPlanetATField struct {
}

// readPlanetATField 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readPlanetATField(props ...int64) *layoutPlanetATField {
	r.section = "PlanetATField"
	v := &layoutPlanetATField{}
	r.expect(r.number(4, true), 0, "PlanetATField.version")
	r.skip(32)
	return v
}

// layoutConstructionSystem 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutConstructionSystem struct {
}

// readConstructionSystem 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readConstructionSystem(props ...int64) *layoutConstructionSystem {
	r.section = "ConstructionSystem"
	v := &layoutConstructionSystem{}
	r.expect(r.number(4, true), 1, "ConstructionSystem.version")
	r.readObjectPool_ConstructionModuleComponent_()
	r.readDataPool_DroneComponent_()
	r.readDataPool_ConstructStat_()
	return v
}

// layoutObjectPool_ConstructionModuleComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutObjectPool_ConstructionModuleComponent_ struct {
}

// readObjectPool_ConstructionModuleComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readObjectPool_ConstructionModuleComponent_(props ...int64) *layoutObjectPool_ConstructionModuleComponent_ {
	r.section = "ObjectPool<ConstructionModuleComponent>"
	v := &layoutObjectPool_ConstructionModuleComponent_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readExportStruct_int32__ConstructionModuleComponent_()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutExportStruct_int32__ConstructionModuleComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutExportStruct_int32__ConstructionModuleComponent_ struct {
}

// readExportStruct_int32__ConstructionModuleComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readExportStruct_int32__ConstructionModuleComponent_(props ...int64) *layoutExportStruct_int32__ConstructionModuleComponent_ {
	r.section = "ExportStruct<int32, ConstructionModuleComponent>"
	v := &layoutExportStruct_int32__ConstructionModuleComponent_{}
	var index int64
	index = r.number(4, true)
	if index != 0 {
		r.readConstructionModuleComponent()
	}
	return v
}

// layoutDataPool_DroneComponent_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPool_DroneComponent_ struct {
}

// readDataPool_DroneComponent_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPool_DroneComponent_(props ...int64) *layoutDataPool_DroneComponent_ {
	r.section = "DataPool<DroneComponent>"
	v := &layoutDataPool_DroneComponent_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readDroneComponent()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutDroneComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDroneComponent struct {
}

// readDroneComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDroneComponent(props ...int64) *layoutDroneComponent {
	r.section = "DroneComponent"
	v := &layoutDroneComponent{}
	r.expect(r.number(4, true), 1, "DroneComponent.version")
	r.skip(44)
	r.readVector3()
	r.readVector3()
	return v
}

// layoutDataPool_ConstructStat_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDataPool_ConstructStat_ struct {
}

// readDataPool_ConstructStat_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDataPool_ConstructStat_(props ...int64) *layoutDataPool_ConstructStat_ {
	r.section = "DataPool<ConstructStat>"
	v := &layoutDataPool_ConstructStat_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readConstructStat()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutConstructStat 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutConstructStat struct {
}

// readConstructStat 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readConstructStat(props ...int64) *layoutConstructStat {
	r.section = "ConstructStat"
	v := &layoutConstructStat{}
	r.expect(r.number(1, false), 0, "ConstructStat.version")
	r.skip(28)
	return v
}
