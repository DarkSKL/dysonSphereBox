package density

// 文件职责：提供炼油厂同侧三带的可追踪来源记录。
// 使用：等离子精炼的独立物料带装配；边界：只读内嵌数据，不在线下载，不将来源蓝图当作最终几何证明。

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"sync"

	"DysonSphereBox/helper/blueprint"
)

// RefineryReferenceURL 职责：标识teapot发布的原始单元；使用：来源复核；边界：不依赖网站运行。
const RefineryReferenceURL = "https://www.dsp2b.com/zh-CN/blueprint/69712114457e2a9f4f2d9777"

// RefineryReferenceSHA256 职责：锁定去除首尾空白后的原始蓝图；使用：内嵌数据完整性检查；边界：不是生成结果指纹。
const RefineryReferenceSHA256 = "ab22cf7e5d8cdf44062fa9e555e29e3ec5fab47bc0f5807074600f94be5d7f02"

// refineryReferenceData 职责：保留一个原版炼油厂、三臂和九带的原始姿态；使用：局部候选；边界：所有产能、带等级和最终连接都重新核验。
//
//go:embed data/refinery-same-side.txt
var refineryReferenceData []byte

var (
	// refineryReferenceOnce 职责：只解析一次只读来源；使用：并发生成；边界：失败不退回默认数字。
	refineryReferenceOnce sync.Once
	// refineryReferenceUnit 职责：缓存来源单元；使用：调用方复制记录后装配；边界：不可改写缓存切片。
	refineryReferenceUnit ReferenceUnit
	// refineryReferenceError 职责：保留来源校验错误；使用：失败关闭；边界：不忽略校验失败。
	refineryReferenceError error
)

// RefineryReference 职责：严格解码并返回炼油厂来源单元；使用：同侧三独立轨；边界：只声明原型和来源，不声明任意纬度均可直接平移。
func RefineryReference() (ReferenceUnit, error) {
	refineryReferenceOnce.Do(func() {
		data := bytes.TrimSpace(refineryReferenceData)
		if fmt.Sprintf("%x", sha256.Sum256(data)) != RefineryReferenceSHA256 {
			refineryReferenceError = fmt.Errorf("炼油厂参考数据指纹不匹配")
			return
		}
		document, err := blueprint.Decode(string(data), true)
		if err != nil {
			refineryReferenceError = err
			return
		}
		if document.GameVersion != "0.10.33.27026" || len(document.Buildings) != 13 || len(document.Areas) != 1 || document.Areas[0].AreaSegments != 200 {
			refineryReferenceError = fmt.Errorf("炼油厂参考结构或版本不匹配")
			return
		}
		machine := document.Buildings[0]
		if machine.Index != 0 || machine.ItemID != 2308 || machine.ModelIndex != 63 {
			refineryReferenceError = fmt.Errorf("炼油厂参考原型不匹配")
			return
		}
		refineryReferenceUnit = ReferenceUnit{MachineIndices: []int32{0}, MachineItemID: 2308, ModelIndex: 63, Records: document.Buildings}
	})
	return refineryReferenceUnit, refineryReferenceError
}
