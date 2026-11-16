package build

// 文件职责：一次解析各模式共同的固定装配事实；使用：候选创建和机器行装配；边界：仅缓存内嵌原型，不缓存请求、实体或可变候选状态。

import (
	_ "embed"
	"encoding/json"
	"sync"
)

//go:embed models.json
var modelData []byte

//go:embed geometry.json
var geometryData []byte

// assemblyFacts 职责：持有随应用版本冻结的模型和槽位表；使用：所有模式只读共享；边界：加载成功后禁止修改 map，几何槽位通过值类型交付。
type assemblyFacts struct {
	models   map[int]int16
	geometry map[int]geometry
}

// loadAssemblyFacts 职责：合并模型和机器几何加载并缓存首次结果；使用：候选和行装配入口；边界：内嵌数据不可热更新，错误也缓存而不回退默认值。
var loadAssemblyFacts = sync.OnceValues(func() (*assemblyFacts, error) {
	facts := &assemblyFacts{}
	if err := json.Unmarshal(modelData, &facts.models); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(geometryData, &facts.geometry); err != nil {
		return nil, err
	}
	return facts, nil
})
