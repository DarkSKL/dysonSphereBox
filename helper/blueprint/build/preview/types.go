package preview

// 文件职责：定义与生成算法解耦的增量渲染契约。
// 使用：生成阶段将可见场景交给通知器，前端只解释新增、撤销与替换。
// 边界：不承载蓝图串、配方流量、候选失败原因或算法搜索节点。

import "DysonSphereBox/helper/blueprint"

// 预览协议常量限定渲染操作与坐标语义；使用：后端事件及前端分派；边界：不表示生成任务已成功。
const (
	// ProtocolVersion 职责：标识对象增量协议；使用：与旧最终建筑分块区分；边界：不改变旧阶段协议。
	ProtocolVersion = 2
	// Kind 职责：标识场景差量事件；使用：进度通道分派；边界：不是完整蓝图替换事件。
	Kind = "preview-delta"
	// Add 职责：标识创建可见对象；使用：渲染器入场；边界：对象 ID 在同一任务中稳定。
	Add = "add"
	// Remove 职责：标识撤销可见对象；使用：回退及候选切换；边界：不要求前端理解撤销原因。
	Remove = "remove"
	// Replace 职责：标识同一对象姿态或连接变化；使用：平移和坐标整理；边界：不改变对象身份。
	Replace = "replace"
	// LayoutCoordinates 职责：标识生成中的平面装配格坐标；使用：临时场景；边界：不冒充最终球面 Area 坐标。
	LayoutCoordinates = "layout-grid-v1"
	// AreaCoordinates 职责：标识正式蓝图 Area 局部坐标；使用：成功结果提交；边界：必须同时解释 Areas。
	AreaCoordinates = "blueprint-area-local-v1"
	// BatchSize 职责：限制每个事件的场景操作数；使用：桥接和渲染批处理；边界：不丢弃中间操作或改变算法。
	BatchSize = 128
)

// Building 职责：提供渲染所需的值类型建筑姿态；使用：差量比较与动画；边界：不含参数大数组或未通过校验的导出数据。
type Building struct {
	// Index 职责：附带最终记录索引；使用：成功后建立选择映射；边界：临时对象为 -1，不能用作 objectId。
	Index int32 `json:"index"`
	// ItemID 职责：标识建筑原型；使用：模型选择；边界：不参与任务或对象身份。
	ItemID int16 `json:"itemId"`
	// ModelIndex 职责：标识原型模型；使用：建筑外观选择；边界：不加载游戏资源。
	ModelIndex int16 `json:"modelIndex"`
	// AreaIndex 职责：指明最终局部坐标所属区域；使用：Area 坐标解释；边界：临时平面场景固定为零。
	AreaIndex int8 `json:"areaIndex"`
	// LocalOffset 职责：保存首端姿态位置；使用：渲染与移动动画；边界：按事件坐标系解释。
	LocalOffset blueprint.Point `json:"localOffset"`
	// LocalOffset2 职责：保存分拣器等对象次端位置；使用：双端几何；边界：不是另一座建筑。
	LocalOffset2 blueprint.Point `json:"localOffset2"`
	// Yaw 职责：保存首端朝向；使用：模型姿态；边界：单位沿用建筑记录。
	Yaw float32 `json:"yaw"`
	// Yaw2 职责：保存次端朝向；使用：双端姿态；边界：不从路线失败原因推算。
	Yaw2 float32 `json:"yaw2"`
	// Tilt 职责：保存首端俯仰；使用：高架带姿态；边界：不改变后端几何。
	Tilt float32 `json:"tilt"`
	// Tilt2 职责：保存次端俯仰；使用：双端几何；边界：与正式记录同义。
	Tilt2 float32 `json:"tilt2"`
	// Pitch 职责：保存首端倾角；使用：模型姿态；边界：与正式记录同义。
	Pitch float32 `json:"pitch"`
	// Pitch2 职责：保存次端倾角；使用：双端几何；边界：与正式记录同义。
	Pitch2 float32 `json:"pitch2"`
	// InputObjectID 职责：关联可见输入对象；使用：连接绘制；边界：不使用最终记录编号，空值表示无可见连接。
	InputObjectID string `json:"inputObjectId,omitempty"`
	// OutputObjectID 职责：关联可见输出对象；使用：连接绘制；边界：不要求接收对象先于引用对象到达。
	OutputObjectID string `json:"outputObjectId,omitempty"`
	// InputFromSlot 职责：保存输入设施槽位；使用：局部连接绘制；边界：不是算法决策。
	InputFromSlot int8 `json:"inputFromSlot"`
	// OutputToSlot 职责：保存输出设施槽位；使用：局部连接绘制；边界：不是算法决策。
	OutputToSlot int8 `json:"outputToSlot"`
	// ProductionIndex 职责：保存原量化产线归属；使用：模块着色和选择；边界：共享设施为 -1。
	ProductionIndex int `json:"productionIndex"`
}

// Object 职责：把请求内稳定身份与建筑姿态绑定；使用：后端场景交接；边界：只在本任务中有效，不是持久化主键。
type Object struct {
	// ID 职责：保存稳定对象键；使用：场景差量匹配；边界：同一场景不能重复或为空。
	ID string
	// Building 职责：保存值类型渲染数据；使用：差量比较；边界：不与算法共享可变参数数组。
	Building Building
}

// Operation 职责：描述一项可直接渲染的对象变化；使用：preview-delta 载荷；边界：remove 不携带建筑，add 与 replace 必须携带建筑。
type Operation struct {
	// Type 职责：选择 add、remove、replace；使用：前端应用变化；边界：未知值不得当作新增。
	Type string `json:"type"`
	// ObjectID 职责：指定被修改的稳定对象；使用：查找渲染实例；边界：不依赖蓝图最终索引。
	ObjectID string `json:"objectId"`
	// Building 职责：承载新姿态；使用：新增或替换；边界：删除操作为空。
	Building *Building `json:"building,omitempty"`
}

// Frame 职责：定义一组场景操作的坐标解释；使用：临时布局与最终记录切换；边界：只包含最多数十个区域元数据，不包含完整建筑。
type Frame struct {
	// CoordinateSystem 职责：标识坐标语义；使用：选择平面或 Area 投影；边界：不能静默混用。
	CoordinateSystem string
	// Areas 职责：保存最终区域定义；使用：Area 坐标解释；边界：临时平面坐标不需要区域数组。
	Areas []blueprint.Area
	// PrimaryAreaIndex 职责：保存主区域编号；使用：最终场景投影；边界：不是对象 ID。
	PrimaryAreaIndex int32
}

// Scene 职责：保存一个候选可被提交的渲染快照；使用：选择最佳候选后恢复正确场景；边界：仅内部持有，不作为事件整体推送。
type Scene struct {
	// Objects 职责：保存快照内对象；使用：通知器比较；边界：不会作为完整列表直接发送前端。
	Objects []Object
	// Frame 职责：保存该快照的坐标语义；使用：最终候选提交；边界：必须与姿态来源一致。
	Frame Frame
}
