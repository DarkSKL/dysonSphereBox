package blueprint

// 文件职责：定义无损蓝图契约；使用：纯 Go 编解码及 factory 服务；边界：不依赖数据库、Wails 或展示解析器。

// Point 职责：承载三维 float32 坐标；使用：编解码边界传递；边界：不执行文件或游戏操作。
type Point struct {
	// X 职责：保存横向坐标；使用：格式读写；边界：保持原始单位与顺序。
	X float32 `json:"x"`
	// Y 职责：保存纵向坐标；使用：格式读写；边界：保持原始单位与顺序。
	Y float32 `json:"y"`
	// Z 职责：保存高度；使用：格式读写；边界：保持原始单位与顺序。
	Z float32 `json:"z"`
}

// PointInt 职责：承载头部二维整数坐标；使用：编解码边界传递；边界：不执行文件或游戏操作。
type PointInt struct {
	// X 职责：保存横向偏移；使用：格式读写；边界：保持原始单位与顺序。
	X int32 `json:"x"`
	// Y 职责：保存纵向偏移；使用：格式读写；边界：保持原始单位与顺序。
	Y int32 `json:"y"`
}

// Area 职责：承载蓝图区域记录；使用：编解码边界传递；边界：不执行文件或游戏操作。
type Area struct {
	// Index 职责：保存记录索引；使用：格式读写；边界：保持原始单位与顺序。
	Index int8 `json:"index"`
	// ParentIndex 职责：保存父区域；使用：格式读写；边界：保持原始单位与顺序。
	ParentIndex int8 `json:"parentIndex"`
	// TropicAnchor 职责：保存纬线锚点；使用：格式读写；边界：保持原始单位与顺序。
	TropicAnchor int16 `json:"tropicAnchor"`
	// AreaSegments 职责：保存区域分段；使用：格式读写；边界：保持原始单位与顺序。
	AreaSegments int16 `json:"areaSegments"`
	// AnchorLocalOffsetX 职责：保存本地横向锚点；使用：格式读写；边界：保持原始单位与顺序。
	AnchorLocalOffsetX int16 `json:"anchorLocalOffsetX"`
	// AnchorLocalOffsetY 职责：保存本地纵向锚点；使用：格式读写；边界：保持原始单位与顺序。
	AnchorLocalOffsetY int16 `json:"anchorLocalOffsetY"`
	// Width 职责：保存宽度；使用：格式读写；边界：保持原始单位与顺序。
	Width int16 `json:"width"`
	// Height 职责：保存高度；使用：格式读写；边界：保持原始单位与顺序。
	Height int16 `json:"height"`
}

// Building 职责：承载建筑与连接的无损记录；使用：编解码边界传递；边界：不执行文件或游戏操作。
type Building struct {
	// Num 职责：保存原布局判别字；使用：格式读写；边界：保持原始单位与顺序。
	Num int32 `json:"num"`
	// RecordLayout 职责：保存v2/v1/v1-old/legacy 布局；使用：格式读写；边界：保持原始单位与顺序。
	RecordLayout string `json:"recordLayout"`
	// Index 职责：保存连接引用索引，区别数组下标；使用：格式读写；边界：保持原始单位与顺序。
	Index int32 `json:"index"`
	// ItemID 职责：保存建筑原型；使用：格式读写；边界：保持原始单位与顺序。
	ItemID int16 `json:"itemId"`
	// ModelIndex 职责：保存模型原型；使用：格式读写；边界：保持原始单位与顺序。
	ModelIndex int16 `json:"modelIndex"`
	// AreaIndex 职责：保存所属区域；使用：格式读写；边界：保持原始单位与顺序。
	AreaIndex int8 `json:"areaIndex"`
	// LocalOffset 职责：保存首端坐标；使用：格式读写；边界：保持原始单位与顺序。
	LocalOffset Point `json:"localOffset"`
	// Yaw 职责：保存首端角度；使用：格式读写；边界：保持原始单位与顺序。
	Yaw float32 `json:"yaw"`
	// Tilt 职责：保存首端倾角；使用：格式读写；边界：保持原始单位与顺序。
	Tilt float32 `json:"tilt"`
	// Pitch 职责：保存首端俯仰；使用：格式读写；边界：保持原始单位与顺序。
	Pitch float32 `json:"pitch"`
	// LocalOffset2 职责：保存次端坐标；使用：格式读写；边界：保持原始单位与顺序。
	LocalOffset2 Point `json:"localOffset2"`
	// Yaw2 职责：保存次端角度；使用：格式读写；边界：保持原始单位与顺序。
	Yaw2 float32 `json:"yaw2"`
	// Tilt2 职责：保存次端倾角；使用：格式读写；边界：保持原始单位与顺序。
	Tilt2 float32 `json:"tilt2"`
	// Pitch2 职责：保存次端俯仰；使用：格式读写；边界：保持原始单位与顺序。
	Pitch2 float32 `json:"pitch2"`
	// OutputObjIdx 职责：保存输出建筑索引，-1 表示无连接；使用：格式读写；边界：保持原始单位与顺序。
	OutputObjIdx int32 `json:"outputObjIdx"`
	// InputObjIdx 职责：保存输入建筑索引；使用：格式读写；边界：保持原始单位与顺序。
	InputObjIdx int32 `json:"inputObjIdx"`
	// OutputToSlot 职责：保存输出目标槽位；使用：格式读写；边界：保持原始单位与顺序。
	OutputToSlot int8 `json:"outputToSlot"`
	// InputFromSlot 职责：保存输入来源槽位；使用：格式读写；边界：保持原始单位与顺序。
	InputFromSlot int8 `json:"inputFromSlot"`
	// OutputFromSlot 职责：保存输出来源槽位；使用：格式读写；边界：保持原始单位与顺序。
	OutputFromSlot int8 `json:"outputFromSlot"`
	// InputToSlot 职责：保存输入目标槽位；使用：格式读写；边界：保持原始单位与顺序。
	InputToSlot int8 `json:"inputToSlot"`
	// OutputOffset 职责：保存输出偏移；使用：格式读写；边界：保持原始单位与顺序。
	OutputOffset int8 `json:"outputOffset"`
	// InputOffset 职责：保存输入偏移；使用：格式读写；边界：保持原始单位与顺序。
	InputOffset int8 `json:"inputOffset"`
	// RecipeID 职责：保存制造配方；使用：格式读写；边界：保持原始单位与顺序。
	RecipeID int16 `json:"recipeId"`
	// FilterID 职责：保存物品过滤器；使用：格式读写；边界：保持原始单位与顺序。
	FilterID int16 `json:"filterId"`
	// Parameters 职责：保存参数原始整数，保留未知位；使用：格式读写；边界：保持原始单位与顺序。
	Parameters []int32 `json:"parameters"`
	// Content 职责：保存UTF-8 内容；使用：格式读写；边界：保持原始单位与顺序。
	Content string `json:"content"`
	// RawTail 职责：保存扩展记录的原始尾部；使用：格式读写；边界：保持原始单位与顺序。
	RawTail []byte `json:"rawTail,omitempty"`
}

// ReformRect 职责：承载地基矩形记录；使用：编解码边界传递；边界：不执行文件或游戏操作。
type ReformRect struct {
	// Reserved 职责：保存保留字节；使用：格式读写；边界：保持原始单位与顺序。
	Reserved uint8 `json:"reserved"`
	// X 职责：保存横向位置；使用：格式读写；边界：保持原始单位与顺序。
	X int16 `json:"x"`
	// Y 职责：保存纵向位置；使用：格式读写；边界：保持原始单位与顺序。
	Y int16 `json:"y"`
	// W 职责：保存宽度；使用：格式读写；边界：保持原始单位与顺序。
	W uint8 `json:"w"`
	// H 职责：保存高度；使用：格式读写；边界：保持原始单位与顺序。
	H uint8 `json:"h"`
	// Data 职责：保存装饰和颜色位域；使用：格式读写；边界：保持原始单位与顺序。
	Data uint8 `json:"data"`
	// AreaIndex 职责：保存所属区域；使用：格式读写；边界：保持原始单位与顺序。
	AreaIndex uint8 `json:"areaIndex"`
}

// ReformData 职责：承载地基与自定义颜色；使用：编解码边界传递；边界：不执行文件或游戏操作。
type ReformData struct {
	// Reserved 职责：保存保留字节；使用：格式读写；边界：保持原始单位与顺序。
	Reserved uint8 `json:"reserved"`
	// Rects 职责：保存矩形序列；使用：格式读写；边界：保持原始单位与顺序。
	Rects []ReformRect `json:"rects"`
	// CustomReformColorMask 职责：保存颜色掩码；使用：格式读写；边界：保持原始单位与顺序。
	CustomReformColorMask uint32 `json:"customReformColorMask"`
	// CustomReformColors 职责：保存颜色原始值；使用：格式读写；边界：保持原始单位与顺序。
	CustomReformColors []uint32 `json:"customReformColors"`
}

// Summary 职责：承载派生建筑摘要；使用：编解码边界传递；边界：不执行文件或游戏操作。
type Summary struct {
	// BuildingCount 职责：保存建筑总数；使用：格式读写；边界：保持原始单位与顺序。
	BuildingCount int `json:"buildingCount"`
	// AreaCount 职责：保存区域总数；使用：格式读写；边界：保持原始单位与顺序。
	AreaCount int `json:"areaCount"`
	// ItemCounts 职责：保存按原型计数；使用：格式读写；边界：保持原始单位与顺序。
	ItemCounts map[int16]int `json:"itemCounts"`
}

// Document 职责：承载独立严格编解码文档；使用：编解码边界传递；边界：不执行文件或游戏操作。
type Document struct {
	// EnvelopeVersion 职责：保存信封版本；使用：格式读写；边界：保持原始单位与顺序。
	EnvelopeVersion int `json:"envelopeVersion"`
	// BinaryVersion 职责：保存二进制版本；使用：格式读写；边界：保持原始单位与顺序。
	BinaryVersion int32 `json:"binaryVersion"`
	// Name 职责：保存短标题；使用：格式读写；边界：保持原始单位与顺序。
	Name string `json:"name"`
	// Description 职责：保存长描述；使用：格式读写；边界：保持原始单位与顺序。
	Description string `json:"description"`
	// Author 职责：保存作者；使用：格式读写；边界：保持原始单位与顺序。
	Author string `json:"author"`
	// GameVersion 职责：保存游戏版本；使用：格式读写；边界：保持原始单位与顺序。
	GameVersion string `json:"gameVersion"`
	// CreatedAt 职责：保存ISO 时间；使用：格式读写；边界：保持原始单位与顺序。
	CreatedAt string `json:"createdAt"`
	// Checksum 职责：保存校验文本；使用：格式读写；边界：保持原始单位与顺序。
	Checksum string `json:"checksum"`
	// ChecksumValid 职责：保存MD5F 比对结果；使用：格式读写；边界：保持原始单位与顺序。
	ChecksumValid bool `json:"checksumValid"`
	// CursorOffset 职责：保存光标偏移；使用：格式读写；边界：保持原始单位与顺序。
	CursorOffset PointInt `json:"cursorOffset"`
	// CursorTargetArea 职责：保存光标区域；使用：格式读写；边界：保持原始单位与顺序。
	CursorTargetArea int32 `json:"cursorTargetArea"`
	// DragBoxSize 职责：保存拖拽框尺寸；使用：格式读写；边界：保持原始单位与顺序。
	DragBoxSize PointInt `json:"dragBoxSize"`
	// PrimaryAreaIndex 职责：保存主区域；使用：格式读写；边界：保持原始单位与顺序。
	PrimaryAreaIndex int32 `json:"primaryAreaIndex"`
	// Areas 职责：保存区域序列；使用：格式读写；边界：保持原始单位与顺序。
	Areas []Area `json:"areas"`
	// Buildings 职责：保存建筑序列，保序；使用：格式读写；边界：保持原始单位与顺序。
	Buildings []Building `json:"buildings"`
	// Patch 职责：保存版本补丁；使用：格式读写；边界：保持原始单位与顺序。
	Patch int32 `json:"patch"`
	// Reform 职责：保存地基数据；使用：格式读写；边界：保持原始单位与顺序。
	Reform *ReformData `json:"reform,omitempty"`
	// TrailingBytes 职责：保存额外尾部长度；使用：格式读写；边界：保持原始单位与顺序。
	TrailingBytes int `json:"trailingBytes"`
	// RawTail 职责：保存原始尾部保留；使用：格式读写；边界：保持原始单位与顺序。
	RawTail []byte `json:"rawTail,omitempty"`
	// EnvelopeFields 职责：保存原始信封字段，保留未知字段与编码；使用：格式读写；边界：保持原始单位与顺序。
	EnvelopeFields []string `json:"envelopeFields,omitempty"`
	// OriginalGzip 职责：保存原压缩数据，仅无损写回时复用；使用：格式读写；边界：保持原始单位与顺序。
	OriginalGzip []byte `json:"originalGzip,omitempty"`
	// Summary 职责：保存派生摘要；使用：格式读写；边界：保持原始单位与顺序。
	Summary Summary `json:"summary"`
}

// EncodeResult 职责：承载自检后的编码结果；使用：编解码边界传递；边界：不执行文件或游戏操作。
type EncodeResult struct {
	// BlueprintRef 职责：大对象临时令牌；使用：分片读取；边界：不是任意文件路径。
	BlueprintRef string `json:"blueprintRef,omitempty"`
	// BlueprintLength 职责：生成文本长度；使用：分片进度；边界：单位为字节。
	BlueprintLength int `json:"blueprintLength,omitempty"`
	// Blueprint 职责：保存完整蓝图串；使用：格式读写；边界：保持原始单位与顺序。
	Blueprint string `json:"blueprint"`
	// Length 职责：保存文本字节数；使用：格式读写；边界：保持原始单位与顺序。
	Length int `json:"length"`
	// Checksum 职责：保存重算校验；使用：格式读写；边界：保持原始单位与顺序。
	Checksum string `json:"checksum"`
	// Verified 职责：保存严格自解码通过；使用：格式读写；边界：保持原始单位与顺序。
	Verified bool `json:"verified"`
}
