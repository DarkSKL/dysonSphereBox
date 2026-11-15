package facts

// 文件职责：加载本地原版资源提取的只读能量事实；使用：统计算法；边界：未知原型不可默认零功率，运行期不访问游戏目录。

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"
)

// data/once/catalog/failure 职责：缓存版本化只读目录；使用：并发统计；边界：加载失败始终返回错误。
//
//go:embed game-25955035.json
var data []byte
var once sync.Once
var catalog *Catalog
var failure error

// Power 职责：保存PowerDesc原型数值；使用：额定能力；边界：每tick为1/60秒，运行负载和科技另行解释。
type Power struct {
	Consumer   bool  `json:"consumer"`          // 职责：识别用电设备；使用：分类；边界：节点不一定耗电。
	Generator  bool  `json:"generator"`         // 职责：识别发电设施；使用：热能分析；边界：不保证发电。
	Work       int64 `json:"workEnergyPerTick"` // 职责：额定每tick焦耳；使用：MW换算；边界：未喷涂。
	Idle       int64 `json:"idleEnergyPerTick"` // 职责：待机每tick焦耳；使用：平均功耗；边界：不乘增产倍率。
	Generation int64 `json:"genEnergyPerTick"`  // 职责：额定发电；使用：供能对比；边界：不含环境系数。
	FuelUse    int64 `json:"useFuelPerTick"`    // 职责：额定燃料热消耗；使用：热效率；边界：不含燃料喷涂。
}

// Item 职责：保存物品及建筑事实；使用：按ID统计；边界：指针Power为空且PowerKnown为true才表示无电力组件。
type Item struct {
	ID         int    `json:"ID"`         // 职责：原型主键；使用：查表；边界：不使用名称匹配。
	Name       string `json:"Name"`       // 职责：名称键；使用：显示；边界：不保证本地化。
	ModelIndex int    `json:"ModelIndex"` // 职责：关联建筑模型；使用：版本核对；边界：不包含美术资源。
	HeatValue  int64  `json:"HeatValue"`  // 职责：未喷涂热值J；使用：化学能通量；边界：非燃料通常为零。
	PowerKnown bool   `json:"powerKnown"` // 职责：区分无组件和未提取；使用：覆盖率；边界：未知不是零。
	Power      *Power `json:"power"`      // 职责：额定电力；使用：能耗计算；边界：可无组件。
}

// Catalog 职责：封装只读索引；使用：统计初始化；边界：不暴露可写map。
type Catalog struct {
	items   map[int]Item
	Version string
}

// Load 职责：校验目录版本并构建索引；使用：量化和蓝图统计；边界：不自动接受其他游戏构建的数据。
func Load() (*Catalog, error) {
	once.Do(func() {
		var input struct {
			Schema string `json:"schema"`
			Source struct {
				GameVersion string `json:"gameVersion"`
				Build       string `json:"steamBuildId"`
				Assembly    string `json:"assemblySha256"`
				Resources   string `json:"resourcesSha256"`
			} `json:"source"`
			Items []Item `json:"items"`
		}
		if err := json.Unmarshal(data, &input); err != nil {
			failure = err
			return
		}
		if input.Schema != "factory-statistics-facts/v1" || input.Source.Build != "25955035" || input.Source.Assembly != "ef314425dade51f529f184f07be136cd3c2660a6a81515603c7226fbe7ab1d8c" || input.Source.Resources != "8343173562b15b06a400d70cb1510a106d7654f96342857e5925ded5fd1900d1" {
			failure = fmt.Errorf("统计原型来源未核对")
			return
		}
		catalog = &Catalog{items: map[int]Item{}, Version: input.Schema + "/" + input.Source.GameVersion + "/" + input.Source.Build}
		for _, item := range input.Items {
			if item.ID <= 0 || item.HeatValue < 0 || item.Power != nil && (item.Power.Work < 0 || item.Power.Idle < 0 || item.Power.Generation < 0 || item.Power.FuelUse < 0) {
				failure = fmt.Errorf("统计原型数值无效")
				return
			}
			if _, ok := catalog.items[item.ID]; ok {
				failure = fmt.Errorf("统计原型重复")
				return
			}
			catalog.items[item.ID] = item
		}
	})
	return catalog, failure
}

// Lookup 职责：返回隔离副本；使用：按物品核对；边界：未知返回false，不能修改缓存的Power。
func (c *Catalog) Lookup(id int) (Item, bool) {
	item, ok := c.items[id]
	if item.Power != nil {
		copy := *item.Power
		item.Power = &copy
	}
	return item, ok
}
