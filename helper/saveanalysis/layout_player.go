package saveanalysis

// 文件职责：按受支持存档布局顺序消费区段，仅保留查询字段。
// 使用：只读解码器内部调用；边界：版本不符立即失败，不推测区段位置。

// layoutPlayer 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutPlayer struct {
}

// readPlayer 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readPlayer(props ...int64) *layoutPlayer {
	r.section = "Player"
	v := &layoutPlayer{}
	r.expect(r.number(4, true), 7, "Player.version")
	r.skip(17)
	r.readVector3()
	r.readVectorLF3()
	r.readQuaternion()
	r.skip(9)
	r.readVectorLF3()
	r.skip(12)
	r.readMecha()
	r.readStorageComponent()
	r.readDeliveryPackage()
	r.readPlayerNavigation()
	r.readVegetableCollection()
	r.skip(8)
	return v
}

// layoutMecha 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutMecha struct {
}

// readMecha 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readMecha(props ...int64) *layoutMecha {
	r.section = "Mecha"
	v := &layoutMecha{}
	r.expect(r.number(4, true), 11, "Mecha.version")
	r.skip(50)
	r.readStorageComponent()
	r.readStorageComponent()
	r.skip(132)
	r.readMechaForge()
	r.readMechaLab()
	r.readConstructionModuleComponent()
	r.readVector3()
	r.skip(4)
	r.readVector3()
	r.skip(4)
	r.readVector3()
	r.skip(135)
	r.readStorageComponent()
	r.readStorageComponent()
	r.readEnemyHatredTarget()
	r.readEnemyHatredTarget()
	r.skip(161)
	r.readStorageComponent()
	r.readCombatModuleComponent()
	r.readCombatModuleComponent()
	var numEnergyShieldResistHistory int64
	numEnergyShieldResistHistory = r.number(4, true)
	nenergyShieldResistHistory := r.length(numEnergyShieldResistHistory)
	r.skip(nenergyShieldResistHistory * 8)
	r.readMechaAppearance()
	r.readMechaAppearance()
	var numDiyItems int64
	numDiyItems = r.number(4, true)
	ndiyItems := r.length(numDiyItems)
	for i := int64(0); i < ndiyItems; i++ {
		r.readKeyValuePair_int32__int32_()
	}
	r.expect(r.number(4, true), 2119973658, "Mecha.constantNumber")
	return v
}

// layoutStorageComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutStorageComponent struct {
	id       int64
	entityId int64
	v_type   int64
	size     int64
	bans     int64
	grids    []*layoutStorageGrid
}

// readStorageComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readStorageComponent(props ...int64) *layoutStorageComponent {
	r.section = "StorageComponent"
	v := &layoutStorageComponent{}
	r.expect(r.number(4, true), 2, "StorageComponent.version")
	var id int64
	id = r.number(4, true)
	v.id = id
	var entityId int64
	entityId = r.number(4, true)
	v.entityId = entityId
	r.skip(16)
	var v_type int64
	v_type = r.number(4, true)
	v.v_type = v_type
	var size int64
	size = r.number(4, true)
	v.size = size
	var bans int64
	bans = r.number(4, true)
	v.bans = bans
	var grids []*layoutStorageGrid
	ngrids := r.length(size)
	grids = make([]*layoutStorageGrid, ngrids)
	for i := int64(0); i < ngrids; i++ {
		grids[i] = r.readStorageGrid()
	}
	v.grids = grids
	return v
}

// layoutStorageGrid 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutStorageGrid struct {
	itemId    int64
	filter    int64
	count     int64
	stackSize int64
}

// readStorageGrid 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readStorageGrid(props ...int64) *layoutStorageGrid {
	r.section = "StorageGrid"
	v := &layoutStorageGrid{}
	var itemId int64
	itemId = r.number(4, true)
	v.itemId = itemId
	var filter int64
	filter = r.number(4, true)
	v.filter = filter
	var count int64
	count = r.number(4, true)
	v.count = count
	var stackSize int64
	stackSize = r.number(4, true)
	v.stackSize = stackSize
	r.skip(4)
	return v
}

// layoutMechaForge 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutMechaForge struct {
}

// readMechaForge 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readMechaForge(props ...int64) *layoutMechaForge {
	r.section = "MechaForge"
	v := &layoutMechaForge{}
	r.expect(r.number(4, true), 0, "MechaForge.version")
	var numTasks int64
	numTasks = r.number(4, true)
	ntasks := r.length(numTasks)
	for i := int64(0); i < ntasks; i++ {
		r.readForgeTask()
	}
	return v
}

// layoutForgeTask 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutForgeTask struct {
}

// readForgeTask 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readForgeTask(props ...int64) *layoutForgeTask {
	r.section = "ForgeTask"
	v := &layoutForgeTask{}
	r.expect(r.number(4, true), 0, "ForgeTask.version")
	r.skip(16)
	var numItemsIds int64
	numItemsIds = r.number(4, true)
	var numProductIds int64
	numProductIds = r.number(4, true)
	nitemsIds := r.length(numItemsIds)
	for i := int64(0); i < nitemsIds; i++ {
		r.readItemId()
	}
	nproductIds := r.length(numProductIds)
	for i := int64(0); i < nproductIds; i++ {
		r.readProductId()
	}
	r.skip(4)
	return v
}

// layoutItemId 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutItemId struct {
}

// readItemId 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readItemId(props ...int64) *layoutItemId {
	r.section = "ItemId"
	v := &layoutItemId{}
	r.skip(12)
	return v
}

// layoutProductId 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutProductId struct {
}

// readProductId 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readProductId(props ...int64) *layoutProductId {
	r.section = "ProductId"
	v := &layoutProductId{}
	r.skip(12)
	return v
}

// layoutMechaLab 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutMechaLab struct {
}

// readMechaLab 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readMechaLab(props ...int64) *layoutMechaLab {
	r.section = "MechaLab"
	v := &layoutMechaLab{}
	r.expect(r.number(4, true), 0, "MechaLab.version")
	var numItemPoints int64
	numItemPoints = r.number(4, true)
	nitemPoints := r.length(numItemPoints)
	for i := int64(0); i < nitemPoints; i++ {
		r.readKeyValuePair_int32__int32_()
	}
	return v
}

// layoutConstructionModuleComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutConstructionModuleComponent struct {
}

// readConstructionModuleComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readConstructionModuleComponent(props ...int64) *layoutConstructionModuleComponent {
	r.section = "ConstructionModuleComponent"
	v := &layoutConstructionModuleComponent{}
	r.expect(r.number(4, true), 1, "ConstructionModuleComponent.version")
	r.skip(44)
	r.readVector3()
	var numAutoReconstructTargets int64
	numAutoReconstructTargets = r.number(4, true)
	nautoReconstructTargets := r.length(numAutoReconstructTargets)
	for i := int64(0); i < nautoReconstructTargets; i++ {
		r.readKeyValuePair_int32__float32_()
	}
	if numAutoReconstructTargets > 0 {
		r.skip(4)
	}
	var numBuildTargets int64
	numBuildTargets = r.number(4, true)
	nbuildTargets := r.length(numBuildTargets)
	for i := int64(0); i < nbuildTargets; i++ {
		r.readKeyValuePair_int32__float32_()
	}
	if numBuildTargets > 0 {
		r.skip(4)
	}
	var numRepairTargets int64
	numRepairTargets = r.number(4, true)
	nrepairTargets := r.length(numRepairTargets)
	for i := int64(0); i < nrepairTargets; i++ {
		r.readConstructTargetExtra()
	}
	if numRepairTargets > 0 {
		r.skip(4)
	}
	return v
}

// layoutKeyValuePair_int32__float32_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutKeyValuePair_int32__float32_ struct {
}

// readKeyValuePair_int32__float32_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readKeyValuePair_int32__float32_(props ...int64) *layoutKeyValuePair_int32__float32_ {
	r.section = "KeyValuePair<int32, float32>"
	v := &layoutKeyValuePair_int32__float32_{}
	r.skip(8)
	return v
}

// layoutConstructTargetExtra 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutConstructTargetExtra struct {
}

// readConstructTargetExtra 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readConstructTargetExtra(props ...int64) *layoutConstructTargetExtra {
	r.section = "ConstructTargetExtra"
	v := &layoutConstructTargetExtra{}
	r.skip(12)
	return v
}

// layoutEnemyHatredTarget 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutEnemyHatredTarget struct {
}

// readEnemyHatredTarget 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readEnemyHatredTarget(props ...int64) *layoutEnemyHatredTarget {
	r.section = "EnemyHatredTarget"
	v := &layoutEnemyHatredTarget{}
	r.expect(r.number(1, false), 0, "EnemyHatredTarget.version")
	r.skip(12)
	return v
}

// layoutCombatModuleComponent 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutCombatModuleComponent struct {
}

// readCombatModuleComponent 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readCombatModuleComponent(props ...int64) *layoutCombatModuleComponent {
	r.section = "CombatModuleComponent"
	v := &layoutCombatModuleComponent{}
	r.expect(r.number(4, true), 3, "CombatModuleComponent.version")
	r.skip(29)
	var numModuleFleets int64
	numModuleFleets = r.number(4, true)
	nmoduleFleets := r.length(numModuleFleets)
	for i := int64(0); i < nmoduleFleets; i++ {
		r.readModuleFleet()
	}
	var numModuleFleetPoses int64
	numModuleFleetPoses = r.number(4, true)
	nmoduleFleetPoses := r.length(numModuleFleetPoses)
	for i := int64(0); i < nmoduleFleetPoses; i++ {
		r.readPose()
	}
	return v
}

// layoutModuleFleet 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutModuleFleet struct {
}

// readModuleFleet 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readModuleFleet(props ...int64) *layoutModuleFleet {
	r.section = "ModuleFleet"
	v := &layoutModuleFleet{}
	r.expect(r.number(4, true), 1, "ModuleFleet.version")
	r.skip(14)
	var numFighters int64
	numFighters = r.number(4, true)
	nfighters := r.length(numFighters)
	for i := int64(0); i < nfighters; i++ {
		r.readModuleFighter()
	}
	return v
}

// layoutModuleFighter 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutModuleFighter struct {
}

// readModuleFighter 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readModuleFighter(props ...int64) *layoutModuleFighter {
	r.section = "ModuleFighter"
	v := &layoutModuleFighter{}
	r.skip(24)
	r.readVector3()
	r.readQuaternion()
	r.skip(4)
	return v
}

// layoutPose 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutPose struct {
}

// readPose 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readPose(props ...int64) *layoutPose {
	r.section = "Pose"
	v := &layoutPose{}
	r.readVector3()
	r.readQuaternion()
	return v
}

// layoutMechaAppearance 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutMechaAppearance struct {
}

// readMechaAppearance 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readMechaAppearance(props ...int64) *layoutMechaAppearance {
	r.section = "MechaAppearance"
	v := &layoutMechaAppearance{}
	r.expect(r.number(4, true), 1, "MechaAppearance.version")
	r.string()
	r.string()
	var thumbnailLength int64
	thumbnailLength = r.number(4, true)
	nthumbnail := r.length(thumbnailLength)
	r.skip(nthumbnail * 1)
	var numMainColors int64
	numMainColors = r.number(4, true)
	nmainColors := r.length(numMainColors)
	for i := int64(0); i < nmainColors; i++ {
		r.readRGBA()
	}
	npartColors := r.length(numMainColors * 64)
	for i := int64(0); i < npartColors; i++ {
		r.readRGBA()
	}
	r.skip(16)
	r.readMechaArmor()
	return v
}

// layoutRGBA 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutRGBA struct {
	v_r int64
	g   int64
	b   int64
	a   int64
}

// readRGBA 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readRGBA(props ...int64) *layoutRGBA {
	r.section = "RGBA"
	v := &layoutRGBA{}
	var v_r int64
	v_r = r.number(1, false)
	v.v_r = v_r
	var g int64
	g = r.number(1, false)
	v.g = g
	var b int64
	b = r.number(1, false)
	v.b = b
	var a int64
	a = r.number(1, false)
	v.a = a
	return v
}

// layoutMechaArmor 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutMechaArmor struct {
}

// readMechaArmor 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readMechaArmor(props ...int64) *layoutMechaArmor {
	r.section = "MechaArmor"
	v := &layoutMechaArmor{}
	r.expect(r.number(4, true), 1, "MechaArmor.version")
	var numBoneArmors int64
	numBoneArmors = r.number(4, true)
	nboneArmors := r.length(numBoneArmors - 1)
	for i := int64(0); i < nboneArmors; i++ {
		r.readExportStruct_int32__BoneArmor_()
	}
	var numMaterials int64
	numMaterials = r.number(4, true)
	nmaterials := r.length(numMaterials)
	for i := int64(0); i < nmaterials; i++ {
		r.readMechaMaterial()
	}
	return v
}

// layoutExportStruct_int32__BoneArmor_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutExportStruct_int32__BoneArmor_ struct {
}

// readExportStruct_int32__BoneArmor_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readExportStruct_int32__BoneArmor_(props ...int64) *layoutExportStruct_int32__BoneArmor_ {
	r.section = "ExportStruct<int32, BoneArmor>"
	v := &layoutExportStruct_int32__BoneArmor_{}
	var index int64
	index = r.number(4, true)
	if index != 0 {
		r.readBoneArmor()
	}
	return v
}

// layoutBoneArmor 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutBoneArmor struct {
}

// readBoneArmor 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readBoneArmor(props ...int64) *layoutBoneArmor {
	r.section = "BoneArmor"
	v := &layoutBoneArmor{}
	r.expect(r.number(1, false), 1, "BoneArmor.version")
	r.skip(5)
	r.readVector3()
	r.readQuaternion()
	r.readVector3()
	var voxelsLength int64
	voxelsLength = r.number(4, true)
	nvoxels := r.length(voxelsLength)
	r.skip(nvoxels * 1)
	var numFilters int64
	numFilters = r.number(4, true)
	nfilters := r.length(numFilters)
	for i := int64(0); i < nfilters; i++ {
		r.readVoxelFilterWithClassId()
	}
	nmatIds := r.length(8)
	r.skip(nmatIds * 4)
	r.skip(4)
	return v
}

// layoutVoxelFilterWithClassId 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutVoxelFilterWithClassId struct {
}

// readVoxelFilterWithClassId 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readVoxelFilterWithClassId(props ...int64) *layoutVoxelFilterWithClassId {
	r.section = "VoxelFilterWithClassId"
	v := &layoutVoxelFilterWithClassId{}
	var classId int64
	classId = r.number(4, true)
	if classId == 1 {
		r.readVoxelFilter_Curve()
	}
	if classId == 2 {
		r.readVoxelFilter_Fan()
	}
	if classId == 3 {
		r.readVoxelFilter_Extrude()
	}
	return v
}

// layoutVoxelFilter_Curve 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutVoxelFilter_Curve struct {
}

// readVoxelFilter_Curve 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readVoxelFilter_Curve(props ...int64) *layoutVoxelFilter_Curve {
	r.section = "VoxelFilter_Curve"
	v := &layoutVoxelFilter_Curve{}
	r.expect(r.number(1, false), 0, "VoxelFilter_Curve.version")
	r.skip(1)
	var size int64
	size = r.number(1, false)
	nstatesAndPointSets := r.length(size + 1)
	for i := int64(0); i < nstatesAndPointSets; i++ {
		r.readVoxelFilterStateAndPointSet()
	}
	return v
}

// layoutVoxelFilterStateAndPointSet 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutVoxelFilterStateAndPointSet struct {
}

// readVoxelFilterStateAndPointSet 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readVoxelFilterStateAndPointSet(props ...int64) *layoutVoxelFilterStateAndPointSet {
	r.section = "VoxelFilterStateAndPointSet"
	v := &layoutVoxelFilterStateAndPointSet{}
	r.skip(4)
	r.readVector2()
	return v
}

// layoutVector2 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutVector2 struct {
}

// readVector2 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readVector2(props ...int64) *layoutVector2 {
	r.section = "Vector2"
	v := &layoutVector2{}
	r.skip(8)
	return v
}

// layoutVoxelFilter_Fan 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutVoxelFilter_Fan struct {
}

// readVoxelFilter_Fan 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readVoxelFilter_Fan(props ...int64) *layoutVoxelFilter_Fan {
	r.section = "VoxelFilter_Fan"
	v := &layoutVoxelFilter_Fan{}
	r.expect(r.number(1, false), 0, "VoxelFilter_Fan.version")
	r.skip(7)
	return v
}

// layoutVoxelFilter_Extrude 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutVoxelFilter_Extrude struct {
}

// readVoxelFilter_Extrude 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readVoxelFilter_Extrude(props ...int64) *layoutVoxelFilter_Extrude {
	r.section = "VoxelFilter_Extrude"
	v := &layoutVoxelFilter_Extrude{}
	r.expect(r.number(1, false), 0, "VoxelFilter_Extrude.version")
	r.skip(1)
	var size int64
	size = r.number(1, false)
	r.skip(1)
	nstatesAndPointSets := r.length(size + 1)
	for i := int64(0); i < nstatesAndPointSets; i++ {
		r.readVoxelFilterStateAndPointSet()
	}
	return v
}

// layoutMechaMaterial 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutMechaMaterial struct {
}

// readMechaMaterial 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readMechaMaterial(props ...int64) *layoutMechaMaterial {
	r.section = "MechaMaterial"
	v := &layoutMechaMaterial{}
	r.expect(r.number(4, true), 0, "MechaMaterial.version")
	r.skip(4)
	var numColors int64
	numColors = r.number(4, true)
	ncolors := r.length(numColors)
	for i := int64(0); i < ncolors; i++ {
		r.readRGBA()
	}
	return v
}

// layoutDeliveryPackage 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDeliveryPackage struct {
}

// readDeliveryPackage 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDeliveryPackage(props ...int64) *layoutDeliveryPackage {
	r.section = "DeliveryPackage"
	v := &layoutDeliveryPackage{}
	r.expect(r.number(4, true), 0, "DeliveryPackage.version")
	var gridCount int64
	gridCount = r.number(4, true)
	r.skip(4)
	ngrid := r.length(gridCount)
	for i := int64(0); i < ngrid; i++ {
		r.readDeliveryPackageGrid()
	}
	r.skip(10)
	return v
}

// layoutDeliveryPackageGrid 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutDeliveryPackageGrid struct {
}

// readDeliveryPackageGrid 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readDeliveryPackageGrid(props ...int64) *layoutDeliveryPackageGrid {
	r.section = "DeliveryPackageGrid"
	v := &layoutDeliveryPackageGrid{}
	var itemId int64
	itemId = r.number(4, true)
	if itemId > 0 {
		r.skip(4)
	}
	if itemId > 0 {
		r.skip(4)
	}
	if itemId > 0 {
		r.skip(4)
	}
	if itemId > 0 {
		r.skip(4)
	}
	if itemId > 0 {
		r.skip(4)
	}
	if itemId > 0 {
		r.skip(4)
	}
	return v
}

// layoutPlayerNavigation 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutPlayerNavigation struct {
}

// readPlayerNavigation 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readPlayerNavigation(props ...int64) *layoutPlayerNavigation {
	r.section = "PlayerNavigation"
	v := &layoutPlayerNavigation{}
	r.expect(r.number(4, true), 2, "PlayerNavigation.version")
	r.skip(5)
	r.readVectorLF3()
	r.skip(47)
	return v
}

// layoutVegetableCollection 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutVegetableCollection struct {
}

// readVegetableCollection 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readVegetableCollection(props ...int64) *layoutVegetableCollection {
	r.section = "VegetableCollection"
	v := &layoutVegetableCollection{}
	r.expect(r.number(4, true), 0, "VegetableCollection.version")
	var numPlayerVege int64
	numPlayerVege = r.number(4, true)
	nplayerVege := r.length(numPlayerVege)
	for i := int64(0); i < nplayerVege; i++ {
		r.readKeyValuePair_int32__int32_()
	}
	var ownerCursor int64
	ownerCursor = r.number(4, true)
	nowners := r.length(ownerCursor)
	for i := int64(0); i < nowners; i++ {
		r.readVegeOwner()
	}
	return v
}

// layoutVegeOwner 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutVegeOwner struct {
}

// readVegeOwner 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readVegeOwner(props ...int64) *layoutVegeOwner {
	r.section = "VegeOwner"
	v := &layoutVegeOwner{}
	r.skip(8)
	var nodeCount int64
	nodeCount = r.number(4, true)
	nnodes := r.length(nodeCount)
	for i := int64(0); i < nnodes; i++ {
		r.readKeyValuePair_int32__int32_()
	}
	return v
}

// layoutGalacticTransport 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutGalacticTransport struct {
}

// readGalacticTransport 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readGalacticTransport(props ...int64) *layoutGalacticTransport {
	r.section = "GalacticTransport"
	v := &layoutGalacticTransport{}
	r.expect(r.number(4, true), 1, "GalacticTransport.version")
	var numStation2stationRoutes int64
	numStation2stationRoutes = r.number(4, true)
	nstation2stationRoutes := r.length(numStation2stationRoutes)
	r.skip(nstation2stationRoutes * 8)
	var numAstro2astroRoutes int64
	numAstro2astroRoutes = r.number(4, true)
	nastro2astroRoutes := r.length(numAstro2astroRoutes)
	r.astroRoutes = make(map[uint64]bool)
	for i := int64(0); i < nastro2astroRoutes; i++ {
		r.readKeyValuePair_int64__LogisticShipRoute_()
	}
	var numAstro2astroBans int64
	numAstro2astroBans = r.number(4, true)
	nastro2astroBans := r.length(numAstro2astroBans)
	r.astroBans = make(map[uint64]bool)
	for i := int64(0); i < nastro2astroBans; i++ {
		r.astroBans[uint64(r.number(8, false))] = true
	}
	return v
}

// layoutKeyValuePair_int64__LogisticShipRoute_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutKeyValuePair_int64__LogisticShipRoute_ struct {
}

// readKeyValuePair_int64__LogisticShipRoute_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readKeyValuePair_int64__LogisticShipRoute_(props ...int64) *layoutKeyValuePair_int64__LogisticShipRoute_ {
	r.section = "KeyValuePair<int64, LogisticShipRoute>"
	v := &layoutKeyValuePair_int64__LogisticShipRoute_{}
	key := uint64(r.number(8, false))
	if r.readLogisticShipRoute().enabled {
		r.astroRoutes[key] = true
	}
	return v
}

// layoutLogisticShipRoute 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutLogisticShipRoute struct {
	enabled bool
}

// readLogisticShipRoute 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readLogisticShipRoute(props ...int64) *layoutLogisticShipRoute {
	r.section = "LogisticShipRoute"
	v := &layoutLogisticShipRoute{}
	r.expect(r.number(4, true), 0, "LogisticShipRoute.version")
	v.enabled = r.number(1, false) != 0
	r.string()
	return v
}

// layoutGalacticDigitalSystem 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutGalacticDigitalSystem struct {
}

// readGalacticDigitalSystem 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readGalacticDigitalSystem(props ...int64) *layoutGalacticDigitalSystem {
	r.section = "GalacticDigitalSystem"
	v := &layoutGalacticDigitalSystem{}
	r.expect(r.number(4, true), 1, "GalacticDigitalSystem.version")
	r.readObjectPool_TodoModule_()
	return v
}

// layoutObjectPool_TodoModule_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutObjectPool_TodoModule_ struct {
}

// readObjectPool_TodoModule_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readObjectPool_TodoModule_(props ...int64) *layoutObjectPool_TodoModule_ {
	r.section = "ObjectPool<TodoModule>"
	v := &layoutObjectPool_TodoModule_{}
	r.skip(4)
	var cursor int64
	cursor = r.number(4, true)
	var recycleCursor int64
	recycleCursor = r.number(4, true)
	nbuffer := r.length(cursor - 1)
	for i := int64(0); i < nbuffer; i++ {
		r.readExportStruct_int32__TodoModule_()
	}
	nrecycleIds := r.length(recycleCursor)
	r.skip(nrecycleIds * 4)
	return v
}

// layoutExportStruct_int32__TodoModule_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutExportStruct_int32__TodoModule_ struct {
}

// readExportStruct_int32__TodoModule_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readExportStruct_int32__TodoModule_(props ...int64) *layoutExportStruct_int32__TodoModule_ {
	r.section = "ExportStruct<int32, TodoModule>"
	v := &layoutExportStruct_int32__TodoModule_{}
	var index int64
	index = r.number(4, true)
	if index != 0 {
		r.readTodoModule()
	}
	return v
}

// layoutTodoModule 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutTodoModule struct {
}

// readTodoModule 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readTodoModule(props ...int64) *layoutTodoModule {
	r.section = "TodoModule"
	v := &layoutTodoModule{}
	r.expect(r.number(4, true), 0, "TodoModule.version")
	r.skip(13)
	var lenTitle int64
	lenTitle = r.number(4, true)
	if lenTitle != 0 {
		r.string()
	}
	var lenContent int64
	lenContent = r.number(4, true)
	if lenContent != 0 {
		r.string()
	}
	var numContentColorIndex int64
	numContentColorIndex = r.number(4, true)
	ncontentColorIndex := r.length(numContentColorIndex)
	r.skip(ncontentColorIndex * 2)
	return v
}

// layoutTrashSystem 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutTrashSystem struct {
}

// readTrashSystem 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readTrashSystem(props ...int64) *layoutTrashSystem {
	r.section = "TrashSystem"
	v := &layoutTrashSystem{}
	r.expect(r.number(4, true), 0, "TrashSystem.version")
	r.readTrashContainer()
	return v
}

// layoutTrashContainer 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutTrashContainer struct {
}

// readTrashContainer 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readTrashContainer(props ...int64) *layoutTrashContainer {
	r.section = "TrashContainer"
	v := &layoutTrashContainer{}
	r.expect(r.number(4, true), 0, "TrashContainer.version")
	r.skip(4)
	var trashCursor int64
	trashCursor = r.number(4, true)
	var trashRecycleCursor int64
	trashRecycleCursor = r.number(4, true)
	ntrashObjPool := r.length(trashCursor)
	for i := int64(0); i < ntrashObjPool; i++ {
		r.readTrashObject()
	}
	ntrashDataPool := r.length(trashCursor)
	for i := int64(0); i < ntrashDataPool; i++ {
		r.readTrashData()
	}
	ntrashRecycle := r.length(trashRecycleCursor)
	r.skip(ntrashRecycle * 4)
	return v
}

// layoutTrashObject 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutTrashObject struct {
}

// readTrashObject 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readTrashObject(props ...int64) *layoutTrashObject {
	r.section = "TrashObject"
	v := &layoutTrashObject{}
	r.expect(r.number(1, false), 2, "TrashObject.version")
	r.skip(5)
	r.readVector3()
	r.readQuaternion()
	r.skip(4)
	return v
}

// layoutTrashData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutTrashData struct {
}

// readTrashData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readTrashData(props ...int64) *layoutTrashData {
	r.section = "TrashData"
	v := &layoutTrashData{}
	r.expect(r.number(4, true), 2, "TrashData.version")
	r.skip(28)
	r.readVector3()
	r.readQuaternion()
	r.readVectorLF3()
	r.readQuaternion()
	r.readVectorLF3()
	r.readVector3()
	return v
}
