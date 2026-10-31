package transport

// 文件职责：提供固定构建的电力节点和物流塔基础能力；使用：蓝图覆盖和端口/槽位检查；边界：仅覆盖已提取原型，不包含科技或 Mod，不推断实际供电与运输是否运行。

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"sync"
)

// infrastructureFactsData 职责：嵌入独立纯数值事实；使用：离线检查；边界：不分发游戏程序、模型或反编译源码。
//
//go:embed data/infrastructure-25955035.json
var infrastructureFactsData []byte

// infrastructureFactsOnce 职责：串行加载已锁定来源；使用：并发蓝图任务；边界：不会在出错后回退无穷能力。
var infrastructureFactsOnce sync.Once

// infrastructureFacts 职责：缓存只读基础设施事实；使用：覆盖与能力查询；边界：不导出可变切片。
var infrastructureFacts infrastructureCatalog

// infrastructureFactsError 职责：缓存目录校验错误；使用：失败关闭；边界：不依赖用户游戏安装路径。
var infrastructureFactsError error

// infrastructureStationFact 职责：保存塔的基础槽位与带端口数；使用：按原型查询；边界：不包括科技扩容。
type infrastructureStationFact struct {
	// ItemID 职责：关联实际物品原型；使用：查找；边界：仅 2103、2104。
	ItemID int `json:"itemId"`
	// MaxItemKinds 职责：记录物流物品槽位；使用：分组校验；边界：端口数与槽位数并非同一概念。
	MaxItemKinds int `json:"maxItemKinds"`
	// MaxItemCount 职责：记录单槽基础货物容量；使用：蓝图塔参数上限检查；边界：不包含科技扩容。
	MaxItemCount int `json:"maxItemCount"`
	// PortCount 职责：记录 SlotConfig 的物流带端口数；使用：端口占用检查；边界：不能用于分拣器槽位。
	PortCount int `json:"portCount"`
}

// infrastructureCatalog 职责：承载已提取基础设施目录；使用：初始化校验；边界：不是用户输入或自动配置。
type infrastructureCatalog struct {
	// Schema 职责：标识提取格式；使用：拒绝未知目录；边界：仅 v1。
	Schema string `json:"schema"`
	// Source 职责：保存与分拣器目录一致的版本来源；使用：锁定已核对构建；边界：不接受未知来源。
	Source struct {
		// SteamBuildID 职责：记录 Steam 构建；使用：来源比对；边界：不是运行时探测。
		SteamBuildID string `json:"steamBuildId"`
		// GameVersion 职责：记录资源游戏版本；使用：覆盖声明；边界：不是量化配方版本。
		GameVersion string `json:"gameVersion"`
		// AssemblySHA256 职责：记录读取语义的 DLL 指纹；使用：来源比对；边界：不分发 DLL。
		AssemblySHA256 string `json:"assemblySha256"`
		// ResourcesSHA256 职责：记录数值资源指纹；使用：来源比对；边界：不分发资源。
		ResourcesSHA256 string `json:"resourcesSha256"`
	} `json:"source"`
	// PowerNodes 职责：保存已核对节点几何；使用：覆盖与连接检查；边界：仅普通电力感应塔。
	PowerNodes []struct {
		// ItemID 职责：关联节点物品原型；使用：完整性校验；边界：2201。
		ItemID int `json:"itemId"`
		// Node 职责：确认原型为可连接电力节点；使用：来源验证；边界：不能为 false。
		Node bool `json:"node"`
		// ConnectDistance 职责：记录节点连接半径；使用：电网连通检查；边界：与覆盖半径不同。
		ConnectDistance float64 `json:"connectDistance"`
		// CoverRadius 职责：记录消费者覆盖半径；使用：NeedPower 检查；边界：不证明电网具有电能。
		CoverRadius float64 `json:"coverRadius"`
	} `json:"powerNodes"`
	// Stations 职责：保存两类物流塔；使用：端口和槽位查询；边界：数量和物品 ID 必须完整。
	Stations []infrastructureStationFact `json:"stations"`
}

// loadInfrastructureFacts 职责：核对格式、来源和完整性；使用：基础设施查询；边界：未知或损坏目录失败，不返回猜测数值。
func loadInfrastructureFacts() error {
	infrastructureFactsOnce.Do(func() {
		if err := json.Unmarshal(infrastructureFactsData, &infrastructureFacts); err != nil {
			infrastructureFactsError = err
			return
		}
		profile, err := InserterCapacityProfile()
		if err != nil {
			infrastructureFactsError = err
			return
		}
		source := infrastructureFacts.Source
		if infrastructureFacts.Schema != "dsp-infrastructure-facts/v1" || source.SteamBuildID != profile.SteamBuildID || source.GameVersion != profile.GameVersion || source.AssemblySHA256 != profile.AssemblySHA256 || source.ResourcesSHA256 != profile.ResourcesSHA256 {
			infrastructureFactsError = fmt.Errorf("电力/物流塔数值目录来源或版本未核对")
			return
		}
		if len(infrastructureFacts.PowerNodes) != 1 {
			infrastructureFactsError = fmt.Errorf("电力节点事实不完整")
			return
		}
		node := infrastructureFacts.PowerNodes[0]
		if node.ItemID != 2201 || !node.Node || node.CoverRadius <= 0 || node.ConnectDistance <= node.CoverRadius || math.IsNaN(node.CoverRadius) || math.IsInf(node.CoverRadius, 0) || math.IsNaN(node.ConnectDistance) || math.IsInf(node.ConnectDistance, 0) {
			infrastructureFactsError = fmt.Errorf("电力节点半径未核对")
			return
		}
		seen := map[int]bool{}
		for _, station := range infrastructureFacts.Stations {
			if (station.ItemID != 2103 && station.ItemID != 2104) || seen[station.ItemID] || station.MaxItemKinds < 1 || station.MaxItemCount < 1 || station.PortCount < 1 {
				infrastructureFactsError = fmt.Errorf("物流塔槽位或端口事实未核对")
				return
			}
			seen[station.ItemID] = true
		}
		if len(seen) != 2 {
			infrastructureFactsError = fmt.Errorf("物流塔事实不完整")
		}
	})
	return infrastructureFactsError
}

// TeslaPowerCoverRadius 职责：返回普通电力感应塔已核对覆盖半径；使用：消费者中心点覆盖检查；边界：不证明实际供电，目录失效返回错误。
func TeslaPowerCoverRadius() (float64, error) {
	if err := loadInfrastructureFacts(); err != nil {
		return 0, err
	}
	return infrastructureFacts.PowerNodes[0].CoverRadius, nil
}

// TeslaPowerConnectDistance 职责：返回普通电力感应塔节点连接距离；使用：电网连通检查；边界：不等于消费者覆盖半径。
func TeslaPowerConnectDistance() (float64, error) {
	if err := loadInfrastructureFacts(); err != nil {
		return 0, err
	}
	return infrastructureFacts.PowerNodes[0].ConnectDistance, nil
}

// stationFact 职责：按物品取得基础能力；使用：导出查询函数；边界：未知塔或 Mod 原型返回错误。
func stationFact(itemID int) (infrastructureStationFact, error) {
	if err := loadInfrastructureFacts(); err != nil {
		return infrastructureStationFact{}, err
	}
	for _, station := range infrastructureFacts.Stations {
		if station.ItemID == itemID {
			return station, nil
		}
	}
	return infrastructureStationFact{}, fmt.Errorf("物流塔物品 %d 未覆盖，仅支持 2103 和 2104", itemID)
}

// StationItemKindLimit 职责：返回固定构建物流塔基础物品槽位；使用：塔分组容量检查；边界：不包括科技或 Mod，未知原型失败。
func StationItemKindLimit(itemID int) (int, error) {
	fact, err := stationFact(itemID)
	return fact.MaxItemKinds, err
}

// StationItemCapacity 职责：返回物流塔每槽基础物品容量；使用：塔建造参数和最终参数上限校验；边界：不包含科技扩容，未知原型失败。
func StationItemCapacity(itemID int) (int, error) {
	fact, err := stationFact(itemID)
	return fact.MaxItemCount, err
}

// StationPortCount 职责：返回物流塔带端口数；使用：端口占用检查；边界：不等于物品槽位，也不推断端口朝向。
func StationPortCount(itemID int) (int, error) {
	fact, err := stationFact(itemID)
	return fact.PortCount, err
}
