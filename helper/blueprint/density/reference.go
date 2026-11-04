// Package density 职责：提供来源可追踪的密铺参考单元；使用：生成器局部候选；边界：不导入 builder 或服务，不访问游戏文件。
package density

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"

	"DysonSphereBox/helper/blueprint"
)

// ReferenceSourceSHA256 职责：锁定当前用户参考图；使用：校准与结果溯源；边界：不是生成产物的指纹。
const ReferenceSourceSHA256 = "8f6ef87ef8eeef70e998b4e1f23d4b5259759a4b5e7b684717ef032bf1d94492"

// referenceData 职责：携带从用户原图提取的原始记录；使用：随应用加载；边界：由提取脚本生成，不手改样例数字。
//
//go:embed data/reference-units.json
var referenceData []byte

// ReferenceUnit 职责：保存两台机器及其局部真实运输记录；使用：构建来源型候选；边界：跨界节点需要装配器重新接驳。
type ReferenceUnit struct {
	// BoundaryReferences 职责：保存裁切时的双向外部引用；使用：带采样邻域完整性核对；边界：需要按原引用字段方向转换，不能把全部引用都当带边。
	BoundaryReferences []ReferenceBoundary `json:"boundaryReferences"`
	// MachineIndices 职责：定位两排原型机器；使用：从 Records 取原始姿态；边界：不是生成结果编号。
	MachineIndices []int32 `json:"machineIndices"`
	// MachineItemID 职责：限定适用原型；使用：候选选择；边界：不按机器类型猜测其它型号。
	MachineItemID int `json:"machineItemId"`
	// ModelIndex 职责：限定原型模型；使用：版本匹配；边界：不同模型必须单独校准。
	ModelIndex int16 `json:"modelIndex"`
	// Records 职责：保留原始双端姿态、槽位和偏移；使用：局部装配；边界：调用方不得修改缓存记录。
	Records []blueprint.Building `json:"records"`
}

// referenceCatalog 职责：绑定源指纹与参考单元；使用：内嵌载入；边界：不是用户配置。
type referenceCatalog struct {
	// SourceSHA256 职责：保存原图指纹；使用：加载来源核对；边界：必须与锁定值相同。
	SourceSHA256 string `json:"sourceSha256"`
	// GameVersion 职责：保存原图版本；使用：限定几何解释；边界：不默认为其它游戏版本兼容。
	GameVersion string `json:"gameVersion"`
	// Units 职责：保存已提取的真实局部单元；使用：按机器原型检索；边界：原始引用编号并非输出编号。
	Units []ReferenceUnit `json:"units"`
}

// referenceOnce 职责：控制一次性加载；使用：并发生成；边界：失败不重试覆盖来源。
var referenceOnce sync.Once

// referenceSource 职责：保存只读参考目录；使用：多次生成共用；边界：调用方不可改写其记录。
var referenceSource referenceCatalog

// referenceError 职责：保留初始化失败；使用：向调用方返回；边界：不静默采用默认间距。
var referenceError error

// Reference 职责：按确切原型返回只读参考单元与源指纹；使用：密铺候选生成；边界：未知原型返回 false，损坏数据返回错误。
func Reference(item int) (ReferenceUnit, string, bool, error) {
	referenceOnce.Do(func() {
		referenceError = json.Unmarshal(referenceData, &referenceSource)
		if referenceError == nil && (referenceSource.SourceSHA256 != ReferenceSourceSHA256 || referenceSource.GameVersion != "0.10.35.29151") {
			referenceError = fmt.Errorf("密铺参考数据来源不匹配")
		}
	})
	if referenceError != nil {
		return ReferenceUnit{}, "", false, referenceError
	}
	for _, unit := range referenceSource.Units {
		if unit.MachineItemID == item {
			return unit, referenceSource.SourceSHA256, true, nil
		}
	}
	return ReferenceUnit{}, referenceSource.SourceSHA256, false, nil
}
