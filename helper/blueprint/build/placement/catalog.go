package placement

// 文件职责：加载经本机游戏资源核对的数值碰撞盒目录。
// 使用：球面布局验证按物品与模型双重匹配查表；开发提取脚本维护版本和来源。
// 边界：不读取用户游戏目录，不分发美术资源；未知版本或映射不能默认为零占位。

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
)

// colliderData 职责：嵌入纯数值事实；使用：离线验证；边界：不包含模型或反编译实现。
//
//go:embed game-colliders-25955035.json
var colliderData []byte

// catalogOnce 职责：保证目录只校验一次；使用：并发生成；边界：初始化失败不改为默认成功。
var catalogOnce sync.Once

// loadedCatalog 职责：缓存只读目录；使用：LoadCatalog；边界：不向调用方暴露可写 map。
var loadedCatalog *Catalog

// catalogError 职责：缓存目录初始化失败；使用：LoadCatalog；边界：损坏数据始终失败关闭。
var catalogError error

// Collider 职责：保存原型局部坐标下的建造定向盒；使用：世界变换；边界：数组数字按原始 float32 读取，不使用模型渲染边界。
type Collider struct {
	// Position 职责：保存局部中心；使用：世界变换；边界：Unity 局部 x/y/z。
	Position [3]float32 `json:"pos"`
	// Extents 职责：保存局部半尺寸；使用：窄相位；边界：严格正数。
	Extents [3]float32 `json:"ext"`
	// Rotation 职责：保存局部单位四元数；使用：旋转局部盒；边界：顺序 x/y/z/w。
	Rotation [4]float32 `json:"q"`
}

// Pose 职责：保存原版 PrefabDesc 的局部端口姿态与 Transform 缩放；使用：分拣器端点、建筑端口和附加区域适配；边界：只描述资源数值，不直接等价于 Unity Physics 查询。
type Pose struct {
	// Position 职责：保存端口相对建筑根节点的位置；使用：球面切线坐标变换；边界：单位为游戏局部长度。
	Position [3]float32 `json:"position"`
	// Rotation 职责：保存端口相对建筑根节点的四元数；使用：端口方向和动态建造盒；边界：顺序 x/y/z/w。
	Rotation [4]float32 `json:"rotation"`
	// LocalScale 职责：保存附加碰撞 Transform 的局部缩放；使用：还原 addonAreaSize；边界：普通端口缩放通常为单位值。
	LocalScale [3]float32 `json:"localScale"`
}

// PrefabGeometry 职责：保存 PrefabDesc 从 SlotConfig 解析出的端口与附加区域；使用：动态建筑几何预检；边界：缺少该目录或字段时必须按未验证处理。
type PrefabGeometry struct {
	// PortPoses 职责：保存建筑物流端口；使用：端口连接和极点姿态核对；边界：顺序对应游戏 slot index。
	PortPoses []Pose `json:"portPoses"`
	// SlotPoses 职责：保存分拣器插入端口；使用：分拣器端点修正；边界：顺序对应游戏 slot index。
	SlotPoses []Pose `json:"slotPoses"`
	// AddonAreaColPoses 职责：保存带、喷涂等附加区域的碰撞姿态；使用：动态扫掠预检；边界：不包含 Unity 场景中的实际 Collider 实例。
	AddonAreaColPoses []Pose `json:"addonAreaColPoses"`
	// AddonAreaPoses 职责：保存附加区域逻辑中心；使用：附加对象匹配；边界：中心不含独立 Transform 旋转。
	AddonAreaPoses []struct {
		Position [3]float32 `json:"position"`
	} `json:"addonAreaPoses"`
	// AddonAreaSize 职责：保存附加区域半尺寸；使用：线段和区域碰撞预检；边界：来源为 Transform.localScale 的一半。
	AddonAreaSize [][3]float32 `json:"addonAreaSize"`
}

// catalogModel 职责：绑定已核对的物品、模型和碰撞盒；使用：目录查表；边界：不把其它模型的盒借给未知物品。
type catalogModel struct {
	Items           []int      `json:"items"`
	MappingVerified bool       `json:"mappingVerified"`
	Colliders       []Collider `json:"buildColliders"`
	// Geometry 职责：绑定 PrefabDesc 端口数据；使用：动态对象验证；边界：空值表示当前原型未提取端口数据。
	Geometry *PrefabGeometry `json:"prefabGeometry,omitempty"`
}

// Catalog 职责：保存已经校验的游戏版本目录；使用：按物品和模型查询建造盒；边界：内部表不可外部修改，未知项返回错误。
type Catalog struct {
	models  map[int]catalogModel
	version string
}

// LoadCatalog 职责：加载已验证映射的内置碰撞目录；使用：生成前或碰撞入口；边界：当前固定为 build 25955035，不推断安装版本或 Mod 补丁。
func LoadCatalog() (*Catalog, error) {
	catalogOnce.Do(func() {
		var data struct {
			MappingVerified bool `json:"mappingVerified"`
			Source          struct {
				Version       string `json:"gameVersion"`
				Build         string `json:"steamBuildId"`
				AssemblyHash  string `json:"assemblySha256"`
				ResourcesHash string `json:"resourcesSha256"`
			} `json:"source"`
			Models map[string]catalogModel `json:"models"`
		}
		if err := json.Unmarshal(colliderData, &data); err != nil {
			catalogError = err
			return
		}
		if !data.MappingVerified || data.Source.Build != "25955035" || data.Source.AssemblyHash != "ef314425dade51f529f184f07be136cd3c2660a6a81515603c7226fbe7ab1d8c" || data.Source.ResourcesHash != "8343173562b15b06a400d70cb1510a106d7654f96342857e5925ded5fd1900d1" || data.Source.Version == "" {
			catalogError = fmt.Errorf("碰撞目录版本、来源或物品映射未核对")
			return
		}
		catalog := &Catalog{models: make(map[int]catalogModel, len(data.Models)), version: data.Source.Version}
		for key, model := range data.Models {
			index, err := strconv.Atoi(key)
			if err != nil || index <= 0 || len(model.Items) == 0 || !model.MappingVerified {
				catalogError = fmt.Errorf("模型 %s 的物品映射无效", key)
				return
			}
			catalog.models[index] = model
		}
		loadedCatalog = catalog
	})
	return loadedCatalog, catalogError
}

// Version 职责：返回目录覆盖的游戏版本；使用：结果诊断；边界：不代表用户当前运行中的游戏版本。
func (catalog *Catalog) Version() string { return catalog.version }

// Shapes 职责：查询并复制已核对的建造盒；使用：生成最终占位；边界：无盒、未知模型或物品映射不匹配返回错误。
func (catalog *Catalog) Shapes(item, model int16) ([]Collider, error) {
	entry, exists := catalog.models[int(model)]
	if exists {
		for _, id := range entry.Items {
			if id == int(item) && len(entry.Colliders) > 0 {
				return append([]Collider(nil), entry.Colliders...), nil
			}
		}
	}
	return nil, fmt.Errorf("物品 %d / 模型 %d 缺少已核对的建造碰撞盒", item, model)
}

// Geometry 职责：查询已核对的 PrefabDesc 端口几何；使用：球面生成器进行分拣器端点和附加区域预检；边界：缺少几何时返回错误，调用方不得用默认端口代替。
func (catalog *Catalog) Geometry(item, model int16) (PrefabGeometry, error) {
	entry, exists := catalog.models[int(model)]
	if exists {
		for _, id := range entry.Items {
			if id == int(item) && entry.Geometry != nil {
				return *entry.Geometry, nil
			}
		}
	}
	return PrefabGeometry{}, fmt.Errorf("物品 %d / 模型 %d 缺少已核对的 PrefabDesc 端口几何", item, model)
}
