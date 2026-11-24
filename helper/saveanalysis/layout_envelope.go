package saveanalysis

// 文件职责：按受支持存档布局顺序消费区段，仅保留查询字段。
// 使用：只读解码器内部调用；边界：版本不符立即失败，不推测区段位置。

// layoutGameSave 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutGameSave struct {
	fileLength         int64
	majorGameVersion   int64
	minorGameVersion   int64
	releaseGameVersion int64
	buildGameVersion   int64
	gameTick           int64
}

// readGameSave 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readGameSave(props ...int64) *layoutGameSave {
	r.section = "GameSave"
	v := &layoutGameSave{}
	r.readVFSaveHeader()
	var fileLength int64
	fileLength = r.number(8, true)
	v.fileLength = fileLength
	r.expect(r.number(4, true), 7, "GameSave.version")
	r.skip(2)
	var majorGameVersion int64
	majorGameVersion = r.number(4, true)
	v.majorGameVersion = majorGameVersion
	var minorGameVersion int64
	minorGameVersion = r.number(4, true)
	v.minorGameVersion = minorGameVersion
	var releaseGameVersion int64
	releaseGameVersion = r.number(4, true)
	v.releaseGameVersion = releaseGameVersion
	var buildGameVersion int64
	buildGameVersion = r.number(4, true)
	v.buildGameVersion = buildGameVersion
	r.gameVersion(v)
	var gameTick int64
	gameTick = r.number(8, true)
	v.gameTick = gameTick
	r.skip(8)
	var sizeOfPngFile int64
	sizeOfPngFile = r.number(4, true)
	nscreenShotPngFile := r.length(sizeOfPngFile)
	r.skip(nscreenShotPngFile * 1)
	r.readAccountData()
	r.skip(8)
	r.readGameData()
	return v
}

// layoutVFSaveHeader 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutVFSaveHeader struct {
}

// readVFSaveHeader 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readVFSaveHeader(props ...int64) *layoutVFSaveHeader {
	r.section = "VFSaveHeader"
	v := &layoutVFSaveHeader{}
	r.expect(r.number(1, false), 86, "VFSaveHeader.magic0")
	r.expect(r.number(1, false), 70, "VFSaveHeader.magic1")
	r.expect(r.number(1, false), 83, "VFSaveHeader.magic2")
	r.expect(r.number(1, false), 65, "VFSaveHeader.magic3")
	r.expect(r.number(1, false), 86, "VFSaveHeader.magic4")
	r.expect(r.number(1, false), 69, "VFSaveHeader.magic5")
	return v
}

// layoutAccountData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutAccountData struct {
}

// readAccountData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readAccountData(props ...int64) *layoutAccountData {
	r.section = "AccountData"
	v := &layoutAccountData{}
	r.expect(r.number(4, true), 0, "AccountData.version")
	r.skip(12)
	r.string()
	return v
}

// layoutGameData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutGameData struct {
	gameDesc     *layoutGameDesc
	factoryCount int64
	dysonSpheres []*layoutExportStruct_int32__DysonSphere_
}

// readGameData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readGameData(props ...int64) *layoutGameData {
	r.section = "GameData"
	v := &layoutGameData{}
	r.expect(r.number(4, true), 13, "GameData.version")
	r.expect(r.number(4, true), 23, "GameData.patch")
	r.readAccountData()
	r.string()
	var gameDesc *layoutGameDesc
	gameDesc = r.readGameDesc()
	v.gameDesc = gameDesc
	r.skip(8)
	r.readGameAchievementData()
	r.readGamePrefsData()
	r.readGameHistoryData()
	r.readGalaxyData()
	r.skip(2)
	r.readGameStatData()
	r.skip(4)
	r.readPlayer()
	var factoryCount int64
	factoryCount = r.number(4, true)
	v.factoryCount = factoryCount
	r.readGalacticTransport()
	r.readGalacticDigitalSystem()
	r.readTrashSystem()
	nfactories := r.length(factoryCount)
	for i := int64(0); i < nfactories; i++ {
		r.readPlanetFactory()
	}
	var dysonSpheresCount int64
	dysonSpheresCount = r.number(4, true)
	var dysonSpheres []*layoutExportStruct_int32__DysonSphere_
	ndysonSpheres := r.length(dysonSpheresCount)
	dysonSpheres = make([]*layoutExportStruct_int32__DysonSphere_, ndysonSpheres)
	for i := int64(0); i < ndysonSpheres; i++ {
		dysonSpheres[i] = r.readExportStruct_int32__DysonSphere_()
	}
	v.dysonSpheres = dysonSpheres
	r.readSpaceSector()
	r.expect(r.number(4, true), 0, "GameData.zero1")
	r.expect(r.number(4, true), 0, "GameData.zero2")
	r.readGameAbnormalityData_0925()
	r.readMilestoneSystem()
	r.readGoalSystem()
	r.readWarningSystem()
	r.world = v
	return v
}

// layoutGameDesc 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutGameDesc struct {
	galaxyAlgo         int64
	galaxySeed         int64
	starCount          int64
	starNameLCID       int64
	savedThemeIDs      []int64
	resourceMultiplier float64
}

// readGameDesc 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readGameDesc(props ...int64) *layoutGameDesc {
	r.section = "GameDesc"
	v := &layoutGameDesc{}
	r.expect(r.number(4, true), 10, "GameDesc.version")
	r.skip(24)
	var galaxyAlgo int64
	galaxyAlgo = r.number(4, true)
	v.galaxyAlgo = galaxyAlgo
	var galaxySeed int64
	galaxySeed = r.number(4, true)
	v.galaxySeed = galaxySeed
	var starCount int64
	starCount = r.number(4, true)
	v.starCount = starCount
	var starNameLCID int64
	starNameLCID = r.number(4, true)
	v.starNameLCID = starNameLCID
	r.skip(4)
	v.resourceMultiplier = float64(floatValue(r.number(4, false)))
	var numSavedThemeIds int64
	numSavedThemeIds = r.number(4, true)
	nsavedThemeIds := r.length(numSavedThemeIds)
	for i := int64(0); i < nsavedThemeIds; i++ {
		v.savedThemeIDs = append(v.savedThemeIDs, r.number(4, true))
	}
	r.skip(3)
	r.readCombatSettings()
	r.skip(4)
	return v
}

// layoutCombatSettings 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutCombatSettings struct {
}

// readCombatSettings 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readCombatSettings(props ...int64) *layoutCombatSettings {
	r.section = "CombatSettings"
	v := &layoutCombatSettings{}
	r.expect(r.number(4, true), 0, "CombatSettings.version")
	r.skip(36)
	return v
}

// layoutGameAchievementData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutGameAchievementData struct {
}

// readGameAchievementData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readGameAchievementData(props ...int64) *layoutGameAchievementData {
	r.section = "GameAchievementData"
	v := &layoutGameAchievementData{}
	r.expect(r.number(4, true), 0, "GameAchievementData.version")
	var num int64
	num = r.number(4, true)
	ndata := r.length(num)
	for i := int64(0); i < ndata; i++ {
		r.readGameAchievementRuntimeData()
	}
	return v
}

// layoutGameAchievementRuntimeData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutGameAchievementRuntimeData struct {
}

// readGameAchievementRuntimeData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readGameAchievementRuntimeData(props ...int64) *layoutGameAchievementRuntimeData {
	r.section = "GameAchievementRuntimeData"
	v := &layoutGameAchievementRuntimeData{}
	var id int64
	id = r.number(4, true)
	if id == 73 {
		r.readAchievementRuntimeData_Fractinator()
	}
	if id == 116 {
		r.readAchievementRuntimeData_MechaShieldResist()
	}
	return v
}

// layoutAchievementRuntimeData_Fractinator 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutAchievementRuntimeData_Fractinator struct {
}

// readAchievementRuntimeData_Fractinator 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readAchievementRuntimeData_Fractinator(props ...int64) *layoutAchievementRuntimeData_Fractinator {
	r.section = "AchievementRuntimeData_Fractinator"
	v := &layoutAchievementRuntimeData_Fractinator{}
	r.skip(2)
	r.expect(r.number(2, true), 0, "AchievementRuntimeData_Fractinator.version")
	r.skip(12)
	var factoryCount int64
	factoryCount = r.number(4, true)
	nheavyHydSamp := r.length(factoryCount * 600)
	r.skip(nheavyHydSamp * 8)
	nproductFix := r.length(factoryCount)
	r.skip(nproductFix * 8)
	return v
}

// layoutAchievementRuntimeData_MechaShieldResist 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutAchievementRuntimeData_MechaShieldResist struct {
}

// readAchievementRuntimeData_MechaShieldResist 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readAchievementRuntimeData_MechaShieldResist(props ...int64) *layoutAchievementRuntimeData_MechaShieldResist {
	r.section = "AchievementRuntimeData_MechaShieldResist"
	v := &layoutAchievementRuntimeData_MechaShieldResist{}
	r.expect(r.number(4, true), 0, "AchievementRuntimeData_MechaShieldResist.version")
	r.skip(8)
	var historyResistLength int64
	historyResistLength = r.number(4, true)
	nhistoryResist := r.length(historyResistLength)
	r.skip(nhistoryResist * 8)
	return v
}

// layoutGamePrefsData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutGamePrefsData struct {
}

// readGamePrefsData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readGamePrefsData(props ...int64) *layoutGamePrefsData {
	r.section = "GamePrefsData"
	v := &layoutGamePrefsData{}
	r.expect(r.number(4, true), 21, "GamePrefsData.version")
	r.readVectorLF3()
	r.readQuaternion()
	r.skip(17)
	var numReplicatorMultipliers int64
	numReplicatorMultipliers = r.number(4, true)
	nreplicatorMultipliers := r.length(numReplicatorMultipliers)
	for i := int64(0); i < nreplicatorMultipliers; i++ {
		r.readKeyValuePair_int32__int32_()
	}
	r.skip(7)
	var numTutorialsShowing int64
	numTutorialsShowing = r.number(4, true)
	ntutorialsShowing := r.length(numTutorialsShowing)
	r.skip(ntutorialsShowing * 4)
	var numAstroNames int64
	numAstroNames = r.number(4, true)
	nastroNames := r.length(numAstroNames)
	for i := int64(0); i < nastroNames; i++ {
		r.readAstroName()
	}
	var numPickFilters int64
	numPickFilters = r.number(4, true)
	npickFilters := r.length(numPickFilters)
	for i := int64(0); i < npickFilters; i++ {
		r.readKeyValuePair_int32__int32_()
	}
	var numEnemyDropBans int64
	numEnemyDropBans = r.number(4, true)
	nenemyDropBans := r.length(numEnemyDropBans)
	r.skip(nenemyDropBans * 4)
	var numTrackedEnemyClusters int64
	numTrackedEnemyClusters = r.number(4, true)
	ntrackedEnemyClusters := r.length(numTrackedEnemyClusters)
	r.skip(ntrackedEnemyClusters * 4)
	r.skip(45)
	r.readControlPanelFilter()
	var numUiControlPanelAstroExpands int64
	numUiControlPanelAstroExpands = r.number(4, true)
	nuiControlPanelAstroExpands := r.length(numUiControlPanelAstroExpands)
	r.skip(nuiControlPanelAstroExpands * 4)
	r.skip(40)
	var numMarkerSelectedColor int64
	numMarkerSelectedColor = r.number(4, true)
	nmarkerSelectedColor := r.length(numMarkerSelectedColor)
	r.skip(nmarkerSelectedColor * 4)
	r.skip(7)
	var numVeinHighlightMasks int64
	numVeinHighlightMasks = r.number(4, true)
	nveinHighlightMasks := r.length(numVeinHighlightMasks)
	for i := int64(0); i < nveinHighlightMasks; i++ {
		r.readVeinHighlightMask()
	}
	var numGreenPrebuildWarningPlanetIds int64
	numGreenPrebuildWarningPlanetIds = r.number(4, true)
	ngreenPrebuildWarningPlanetIds := r.length(numGreenPrebuildWarningPlanetIds)
	r.skip(ngreenPrebuildWarningPlanetIds * 4)
	return v
}

// layoutVectorLF3 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutVectorLF3 struct {
}

// readVectorLF3 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readVectorLF3(props ...int64) *layoutVectorLF3 {
	r.section = "VectorLF3"
	v := &layoutVectorLF3{}
	r.skip(24)
	return v
}

// layoutQuaternion 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutQuaternion struct {
	x int64
	y int64
	z int64
	w int64
}

// readQuaternion 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readQuaternion(props ...int64) *layoutQuaternion {
	r.section = "Quaternion"
	v := &layoutQuaternion{}
	var x int64
	x = r.number(4, false)
	v.x = x
	var y int64
	y = r.number(4, false)
	v.y = y
	var z int64
	z = r.number(4, false)
	v.z = z
	var w int64
	w = r.number(4, false)
	v.w = w
	return v
}

// layoutKeyValuePair_int32__int32_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutKeyValuePair_int32__int32_ struct {
}

// readKeyValuePair_int32__int32_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readKeyValuePair_int32__int32_(props ...int64) *layoutKeyValuePair_int32__int32_ {
	r.section = "KeyValuePair<int32, int32>"
	v := &layoutKeyValuePair_int32__int32_{}
	r.skip(8)
	return v
}

// layoutAstroName 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutAstroName struct {
}

// readAstroName 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readAstroName(props ...int64) *layoutAstroName {
	r.section = "AstroName"
	v := &layoutAstroName{}
	id := r.number(4, true)
	name := r.string()
	if r.astroNames == nil {
		r.astroNames = make(map[int64]string)
	}
	if _, exists := r.astroNames[id]; exists || id <= 0 {
		r.fail("天体自定义名称标识无效或重复")
	}
	r.astroNames[id] = name
	return v
}

// layoutControlPanelFilter 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutControlPanelFilter struct {
}

// readControlPanelFilter 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readControlPanelFilter(props ...int64) *layoutControlPanelFilter {
	r.section = "ControlPanelFilter"
	v := &layoutControlPanelFilter{}
	r.expect(r.number(4, true), 1, "ControlPanelFilter.version")
	r.skip(8)
	r.string()
	var numItemsFilter int64
	numItemsFilter = r.number(4, true)
	nitemsFilter := r.length(numItemsFilter)
	r.skip(nitemsFilter * 4)
	r.skip(4)
	r.string()
	r.skip(4)
	return v
}

// layoutVeinHighlightMask 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutVeinHighlightMask struct {
}

// readVeinHighlightMask 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readVeinHighlightMask(props ...int64) *layoutVeinHighlightMask {
	r.section = "VeinHighlightMask"
	v := &layoutVeinHighlightMask{}
	r.skip(8)
	return v
}

// layoutGameHistoryData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutGameHistoryData struct {
}

// readGameHistoryData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readGameHistoryData(props ...int64) *layoutGameHistoryData {
	r.section = "GameHistoryData"
	v := &layoutGameHistoryData{}
	r.expect(r.number(4, true), 20, "GameHistoryData.version")
	var numRecipeUnlocked int64
	numRecipeUnlocked = r.number(4, true)
	nrecipeUnlocked := r.length(numRecipeUnlocked)
	r.recipes = make(map[int]bool)
	for i := int64(0); i < nrecipeUnlocked; i++ {
		r.recipes[int(r.number(4, true))] = true
	}
	var numEnemyDropItemUnlocked int64
	numEnemyDropItemUnlocked = r.number(4, true)
	nenemyDropItemUnlocked := r.length(numEnemyDropItemUnlocked)
	r.skip(nenemyDropItemUnlocked * 4)
	var numTutorialUnlocked int64
	numTutorialUnlocked = r.number(4, true)
	ntutorialUnlocked := r.length(numTutorialUnlocked)
	r.skip(ntutorialUnlocked * 4)
	var numFeatureKeys int64
	numFeatureKeys = r.number(4, true)
	nfeatureKeys := r.length(numFeatureKeys)
	for i := int64(0); i < nfeatureKeys; i++ {
		key := r.number(4, true)
		if key == 1000103 || key == 1000111 {
			r.capsuleRemoved = true
		}
	}
	var numFeatureValues int64
	numFeatureValues = r.number(4, true)
	nfeatureValues := r.length(numFeatureValues)
	for i := int64(0); i < nfeatureValues; i++ {
		r.readKeyValuePair_int32__int32_()
	}
	r.readJournalSystem()
	var numTechState int64
	numTechState = r.number(4, true)
	ntechState := r.length(numTechState)
	r.pendingTechs = make(map[int64]bool)
	for i := int64(0); i < ntechState; i++ {
		r.readTechState()
	}
	r.skip(1)
	r.skip(4) // 另存的科技字段可能为空，生产研究站须与科研队首核对。
	var numTechQueue int64
	numTechQueue = r.number(4, true)
	ntechQueue := r.length(numTechQueue)
	if ntechQueue > 0 {
		r.currentTech = r.number(4, true)
		r.skip((ntechQueue - 1) * 4)
	}
	r.readPlanningTechnology()
	r.readCombatSettings()
	var numPropertyDataProduction int64
	numPropertyDataProduction = r.number(4, true)
	npropertyDataProduction := r.length(numPropertyDataProduction)
	for i := int64(0); i < npropertyDataProduction; i++ {
		r.readKeyValuePair_int32__int32_()
	}
	var numPropertyDataConsumption int64
	numPropertyDataConsumption = r.number(4, true)
	npropertyDataConsumption := r.length(numPropertyDataConsumption)
	for i := int64(0); i < npropertyDataConsumption; i++ {
		r.readKeyValuePair_int32__int32_()
	}
	r.skip(2)
	return v
}

// layoutJournalSystem 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutJournalSystem struct {
}

// readJournalSystem 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readJournalSystem(props ...int64) *layoutJournalSystem {
	r.section = "JournalSystem"
	v := &layoutJournalSystem{}
	r.expect(r.number(4, true), 0, "JournalSystem.version")
	var num int64
	num = r.number(4, true)
	ndata := r.length(num)
	for i := int64(0); i < ndata; i++ {
		r.readJournalData()
	}
	return v
}

// layoutJournalData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutJournalData struct {
}

// readJournalData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readJournalData(props ...int64) *layoutJournalData {
	r.section = "JournalData"
	v := &layoutJournalData{}
	r.expect(r.number(4, true), 0, "JournalData.version")
	r.skip(4)
	var num int64
	num = r.number(4, true)
	nparameters := r.length(num)
	r.skip(nparameters * 8)
	return v
}

// layoutTechState 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutTechState struct {
}

// readTechState 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readTechState(props ...int64) *layoutTechState {
	r.section = "TechState"
	v := &layoutTechState{}
	id := r.number(4, true)
	unlocked := r.number(1, false) != 0
	r.skip(8)
	uploaded, needed := r.number(8, true), r.number(8, true)
	r.skip(8)
	r.pendingTechs[id] = !unlocked && uploaded < needed
	return v
}

// layoutGalaxyData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutGalaxyData struct {
}

// readGalaxyData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readGalaxyData(props ...int64) *layoutGalaxyData {
	r.section = "GalaxyData"
	v := &layoutGalaxyData{}
	r.expect(r.number(4, true), 1, "GalaxyData.version")
	r.readLinkedListTermM1_int32__PlanetScannedData_()
	r.readLinkedListTermM1_int32__int32_()
	return v
}

// layoutLinkedListTermM1_int32__PlanetScannedData_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutLinkedListTermM1_int32__PlanetScannedData_ struct {
}

// readLinkedListTermM1_int32__PlanetScannedData_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readLinkedListTermM1_int32__PlanetScannedData_(props ...int64) *layoutLinkedListTermM1_int32__PlanetScannedData_ {
	r.section = "LinkedListTermM1<int32, PlanetScannedData>"
	v := &layoutLinkedListTermM1_int32__PlanetScannedData_{}
	var id int64
	id = r.number(4, true)
	if id != -1 {
		scan := r.readPlanetScannedData()
		if r.scanned != nil {
			r.scanned[id] = scan
		}
	}
	if id != -1 {
		r.readLinkedListTermM1_int32__PlanetScannedData_()
	}
	return v
}

// layoutPlanetScannedData 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
// readPlanetScannedData 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readPlanetScannedData(props ...int64) *scannedPlanet {
	r.section = "PlanetScannedData"
	v := &scannedPlanet{}
	r.expect(r.number(4, true), 0, "PlanetScannedData.version")
	v.landPercent = float64(floatValue(r.number(4, false)))
	r.readVector3() // 矿脉偏向
	v.birthPoint = r.readVector3().vector()
	r.readVector3()
	r.readVector3()
	var numVeinGroups int64
	numVeinGroups = r.number(4, true)
	nveinGroups := r.length(numVeinGroups - 1)
	for i := int64(0); i < nveinGroups; i++ {
		group := r.readVeinGroup()
		group.ID = i + 1
		if group.VeinType > 0 && group.Count > 0 {
			v.groups = append(v.groups, group)
		}
	}
	return v
}

// layoutVector3 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutVector3 struct {
	x int64
	y int64
	z int64
}

// readVector3 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readVector3(props ...int64) *layoutVector3 {
	r.section = "Vector3"
	v := &layoutVector3{}
	var x int64
	x = r.number(4, false)
	v.x = x
	var y int64
	y = r.number(4, false)
	v.y = y
	var z int64
	z = r.number(4, false)
	v.z = z
	return v
}

// layoutVeinGroup 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
// readVeinGroup 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readVeinGroup(props ...int64) PlanetVeinGroup {
	r.section = "VeinGroup"
	r.expect(r.number(4, true), 0, "VeinGroup.version")
	return PlanetVeinGroup{
		VeinType: r.number(4, true), Position: r.readVector3().vector(),
		Count: r.number(4, true), Amount: r.number(8, true),
	}
}

// layoutLinkedListTermM1_int32__int32_ 职责：保存该区段所需字段；使用：内部顺序解码；边界：不暴露无关游戏状态。
type layoutLinkedListTermM1_int32__int32_ struct {
}

// readLinkedListTermM1_int32__int32_ 职责：消费该区段；使用：由父区段调用；边界：短读或版本不符时终止。
func (r *reader) readLinkedListTermM1_int32__int32_(props ...int64) *layoutLinkedListTermM1_int32__int32_ {
	r.section = "LinkedListTermM1<int32, int32>"
	v := &layoutLinkedListTermM1_int32__int32_{}
	var id int64
	id = r.number(4, true)
	if id != -1 {
		r.worldPlanetIDs = append(r.worldPlanetIDs, id)
		r.skip(4)
	}
	if id != -1 {
		r.readLinkedListTermM1_int32__int32_()
	}
	return v
}
