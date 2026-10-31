package transport

// 文件职责：提供版本锁定的零科技分拣器额定能力；使用：端口容量下界检查；边界：只覆盖原版 2011..2014、整数 1..3 跨距、非堆叠连续供料和满电，不伪造实际游戏吞吐保证。
// 逻辑流：1. 核对数值目录来源 => 2. 取得原型 STT 和零科技抓取批量 => 3. 计入逐件抓取/放置 tick => 4. 换算每分钟额定上限。

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"
)

// inserterFactsData 职责：嵌入本机固定构建提取的纯数值事实；使用：离线容量核验；边界：不包含模型、游戏程序或反编译源码。
//
//go:embed data/inserters-25955035.json
var inserterFactsData []byte

// inserterFactsOnce 职责：串行初始化只读事实；使用：并发生成；边界：初始化错误不会回退无限容量。
var inserterFactsOnce sync.Once

// inserterFacts 职责：缓存版本验证后的事实；使用：容量查询；边界：不向外暴露可写映射。
var inserterFacts inserterCatalog

// inserterFactsError 职责：保存事实加载错误；使用：失败关闭；边界：不依赖运行中游戏目录。
var inserterFactsError error

// InserterProfile 职责：说明额定容量所覆盖的版本和运行假设；使用：生成诊断；边界：不是对用户安装版本或科技状态的自动探测。
type InserterProfile struct {
	// ID 职责：保存零科技非堆叠假设版本；使用：诊断和 fixture；边界：不同科技不能沿用此额定值冒充已核对。
	ID string `json:"id"`
	// GameVersion 职责：保存提取事实的游戏版本；使用：覆盖声明；边界：不是量化配方数据集版本。
	GameVersion string `json:"gameVersion"`
	// SteamBuildID 职责：保存 Steam 构建；使用：版本锁定；边界：固定为目录来源。
	SteamBuildID string `json:"steamBuildId"`
	// AssemblySHA256 职责：保存核对计时公式的 DLL 指纹；使用：来源审计；边界：不分发 DLL。
	AssemblySHA256 string `json:"assemblySha256"`
	// ResourcesSHA256 职责：保存原始 STT 数值来源；使用：目录校验；边界：不分发资源。
	ResourcesSHA256 string `json:"resourcesSha256"`
}

// inserterCatalog 职责：承载提取目录；使用：一次加载；边界：内部字段不是前端配置入口。
type inserterCatalog struct {
	// Schema 职责：标记事实格式；使用：拒绝未知格式；边界：只接受 v1。
	Schema string `json:"schema"`
	// Source 职责：保存来源版本；使用：指纹核对；边界：与碰撞目录同一构建。
	Source struct {
		// GameVersion 职责：保存实际资源版本；使用：诊断；边界：不能为空。
		GameVersion string `json:"gameVersion"`
		// SteamBuildID 职责：锁定构建；使用：初始化核验；边界：未知构建失败。
		SteamBuildID string `json:"steamBuildId"`
		// AssemblySHA256 职责：保存 DLL 哈希；使用：公式来源；边界：必须与已审核值一致。
		AssemblySHA256 string `json:"assemblySha256"`
		// ResourcesSHA256 职责：保存资源哈希；使用：数值来源；边界：必须与已审核值一致。
		ResourcesSHA256 string `json:"resourcesSha256"`
	} `json:"source"`
	// Profile 职责：保存零科技计时参数；使用：额定上限；边界：不接受调用方隐式覆盖。
	Profile struct {
		// ID 职责：记录假设版本；使用：诊断；边界：固定非堆叠满电。
		ID string `json:"id"`
		// TicksPerSecond 职责：记录游戏固定 tick 频率；使用：单位换算；边界：60。
		TicksPerSecond int `json:"ticksPerSecond"`
		// TimeUnitsPerTick 职责：记录满电每 tick 计时增量；使用：STT 周期；边界：10000。
		TimeUnitsPerTick int `json:"timeUnitsPerTick"`
		// ObsoleteStackInput 职责：记录 MkIII 零科技每周期件数；使用：等级 3；边界：未堆叠为 1。
		ObsoleteStackInput int `json:"obsoleteStackInput"`
		// PileStackInput 职责：记录集装臂零科技批量；使用：等级 4；边界：为 2，逐件取放各消耗 tick。
		PileStackInput int `json:"pileStackInput"`
		// StackOutput 职责：记录零科技输出堆叠；使用：额定公式适用范围；边界：为 1。
		StackOutput int `json:"stackOutput"`
		// InputCargoStack 职责：记录输入货物堆叠数；使用：额定公式适用范围；边界：只覆盖 1。
		InputCargoStack int `json:"inputCargoStack"`
		// Power 职责：记录满电倍率；使用：适用范围；边界：固定为 1。
		Power float64 `json:"power"`
		// Bidirectional 职责：记录是否双向工作；使用：适用范围；边界：零科技为 false。
		Bidirectional bool `json:"bidirectional"`
	} `json:"profile"`
	// Inserters 职责：列举四个等级的原始 STT；使用：容量公式；边界：必须完整且互不重复。
	Inserters []struct {
		// ItemID 职责：标识实际分拣器物品；使用：等级匹配；边界：2011..2014。
		ItemID int `json:"itemId"`
		// Grade 职责：记录原型等级；使用：零科技堆叠分支；边界：1..4。
		Grade int `json:"grade"`
		// STT 职责：记录 PrefabDesc 转换后的单程计时；使用：跨度乘数；边界：必须是当前计时增量的整数倍。
		STT int `json:"stt"`
	} `json:"inserters"`
}

// loadInserterFacts 职责：锁定来源及计时假设；使用：容量与诊断查询；边界：事实损坏或未知版本返回错误，不能默认为无限。
func loadInserterFacts() error {
	inserterFactsOnce.Do(func() {
		if err := json.Unmarshal(inserterFactsData, &inserterFacts); err != nil {
			inserterFactsError = err
			return
		}
		source, profile := inserterFacts.Source, inserterFacts.Profile
		if inserterFacts.Schema != "dsp-inserter-capacity/v1" || source.SteamBuildID != "25955035" || source.GameVersion != "0.10.35.29151" || source.AssemblySHA256 != "ef314425dade51f529f184f07be136cd3c2660a6a81515603c7226fbe7ab1d8c" || source.ResourcesSHA256 != "8343173562b15b06a400d70cb1510a106d7654f96342857e5925ded5fd1900d1" {
			inserterFactsError = fmt.Errorf("分拣器数值目录来源或版本未核对")
			return
		}
		if profile.ID != "vanilla-no-tech-unstacked-full-power/v1" || profile.TicksPerSecond != 60 || profile.TimeUnitsPerTick != 10000 || profile.ObsoleteStackInput != 1 || profile.PileStackInput != 2 || profile.StackOutput != 1 || profile.InputCargoStack != 1 || profile.Power != 1 || profile.Bidirectional {
			inserterFactsError = fmt.Errorf("分拣器科技、堆叠或计时假设未覆盖")
			return
		}
		seen := map[int]bool{}
		for _, fact := range inserterFacts.Inserters {
			if fact.ItemID != 2010+fact.Grade || fact.Grade < 1 || fact.Grade > 4 || seen[fact.Grade] || fact.STT < profile.TimeUnitsPerTick || fact.STT%profile.TimeUnitsPerTick != 0 {
				inserterFactsError = fmt.Errorf("分拣器原型或计时值未覆盖")
				return
			}
			seen[fact.Grade] = true
		}
		if len(seen) != 4 {
			inserterFactsError = fmt.Errorf("分拣器额定事实缺少等级")
		}
	})
	return inserterFactsError
}

// InserterCapacityProfile 职责：返回只读的额定值覆盖声明；使用：结果诊断；边界：不判断玩家实际科技、供电或堆叠。
func InserterCapacityProfile() (InserterProfile, error) {
	if err := loadInserterFacts(); err != nil {
		return InserterProfile{}, err
	}
	source := inserterFacts.Source
	return InserterProfile{ID: inserterFacts.Profile.ID, GameVersion: source.GameVersion, SteamBuildID: source.SteamBuildID, AssemblySHA256: source.AssemblySHA256, ResourcesSHA256: source.ResourcesSHA256}, nil
}

// InserterCapacityPerMinute 职责：计算单臂零科技非堆叠额定吞吐上限；使用：按实际蓝图 Parameters[0] 跨距核对端口流量；边界：仅等级 1..4、跨度 1..3，未知数据失败，不等于带速或实际连续供料保证。
func InserterCapacityPerMinute(level, distance int) (float64, error) {
	if err := loadInserterFacts(); err != nil {
		return 0, err
	}
	if distance < 1 || distance > 3 {
		return 0, fmt.Errorf("分拣器跨度 %d 未覆盖，已核对整数跨度 1 到 3", distance)
	}
	for _, fact := range inserterFacts.Inserters {
		if fact.Grade != level {
			continue
		}
		batch := 1
		if level == 3 {
			batch = inserterFacts.Profile.ObsoleteStackInput
		} else if level == 4 {
			batch = inserterFacts.Profile.PileStackInput
		}
		// 已核对目录的单程时间都是整 tick；多件批量取和放各多花 batch-1 tick，不能只乘堆叠数。
		cycleTicks := 2 * (fact.STT*distance/inserterFacts.Profile.TimeUnitsPerTick + batch - 1)
		return float64(batch*inserterFacts.Profile.TicksPerSecond*60) / float64(cycleTicks), nil
	}
	return 0, fmt.Errorf("分拣器等级 %d 未覆盖，支持 1 到 4", level)
}
