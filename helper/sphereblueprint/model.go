package sphereblueprint

// 文件职责：定义原版球壳的预览数据；使用：服务返回给主页场景；边界：只保留设计数据，不携带施工状态。

// Blueprint 职责：描述完整预览；使用：按类型选择球壳或帆群；边界：空槽由缺失的原始 ID 表示。
type Blueprint struct {
	// Type 对应游戏类型，1 单层、2 多层、3 帆群、4 整球。
	Type int `json:"type"`
	// GameVersion 保留头部版本，供预览显示。
	GameVersion string `json:"gameVersion"`
	// Layers 按原始层 ID 排序，单层使用展示 ID 1。
	Layers []Layer `json:"layers"`
	// Orbits 保存实际存在的帆轨道，包含停用轨道。
	Orbits []Orbit `json:"orbits"`
}

// Orbit 职责：描述帆轨道；使用：保留蓝图姿态及颜色；边界：蓝图没有实时帆数。
type Orbit struct {
	// ID 是从 1 开始的游戏轨道 ID。
	ID int `json:"id"`
	// Radius 使用游戏长度单位。
	Radius float64 `json:"radius"`
	// Rotation 是 Unity 坐标系 xyzw 四元数。
	Rotation []float64 `json:"rotation"`
	// Enabled 是轨道的启用状态。
	Enabled bool `json:"enabled"`
	// Visible 来自游戏视图掩码。
	Visible bool `json:"visible"`
	// HSVA 保存原始轨道颜色，旧版采用游戏默认值。
	HSVA []float64 `json:"hsva"`
}

// Layer 职责：关联一层轨道、结构和涂装；使用：按原始 ID 构建几何；边界：单层无轨道数据，默认正立。
type Layer struct {
	// ID 保留层槽位。
	ID int `json:"id"`
	// Radius 保存轨道半径，单层取第一个节点的长度，空单层取 1。
	Radius float64 `json:"radius"`
	// Rotation 是 Unity 坐标系目标旋转 xyzw。
	Rotation []float64 `json:"rotation"`
	// Visible 表示游戏视图是否显示此层。
	Visible bool `json:"visible"`
	// Nodes 仅保存实际存在的节点。
	Nodes []Node `json:"nodes"`
	// Frames 保留显式框架。
	Frames []Frame `json:"frames"`
	// Shells 保留显式壳面。
	Shells []Shell `json:"shells"`
	// GridMode 是逐格画网格模式。
	GridMode int `json:"gridMode"`
	// CellColors 是连续 RGBA 字节的 Base64，空字符串表示未涂装。
	CellColors string `json:"cellColors"`
}

// Node 职责：描述骨架顶点；使用：按层内 ID 引用；边界：坐标保留原始半径和手性。
type Node struct {
	// ID 是本层节点池索引。
	ID int `json:"id"`
	// Pos 是有限的非零 Unity xyz 坐标。
	Pos []float64 `json:"pos"`
	// Color 按无符号 RRGGBBAA 打包。
	Color uint32 `json:"color"`
	// Proto 保存原型 ID。
	Proto int `json:"proto"`
}

// Frame 职责：描述节点间的梁；使用：按弧线类型采样；边界：端点均在本层。
type Frame struct {
	// ID 是本层框架池索引。
	ID int `json:"id"`
	// A 是起点 ID。
	A int `json:"a"`
	// B 是终点 ID。
	B int `json:"b"`
	// Euler 表示使用游戏欧拉插值。
	Euler bool `json:"euler"`
	// Color 按无符号 RRGGBBAA 打包。
	Color uint32 `json:"color"`
	// Proto 保存原型 ID。
	Proto int `json:"proto"`
}

// Shell 职责：描述显式壳面；使用：按原绕序构建；边界：不推导未记录的面。
type Shell struct {
	// ID 是本层壳面池索引。
	ID int `json:"id"`
	// Nodes 是按边界顺序记录的节点 ID。
	Nodes []int `json:"nodes"`
	// Seed 保留游戏随机种子。
	Seed int `json:"seed"`
	// Color 按无符号 RRGGBBAA 打包。
	Color uint32 `json:"color"`
	// Proto 保存原型 ID。
	Proto int `json:"proto"`
}
