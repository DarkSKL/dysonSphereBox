package environment

// 文件职责：解析《戴森球计划》BLUEPRINT 编码中的元信息与二进制结构。
// 使用：EnvironmentManagerService 在蓝图详情只读查看时调用，前端消费解析后的摘要、区域和建筑列表。
// 边界：只解析并校验结构，不实现游戏专用 MD5F 校验，也不负责重新编码或修改蓝图文件。

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// BlueprintDecodeResult 描述蓝图编码解析后的完整只读结果。
// 职责：聚合外层元信息、二进制内容和面向 UI 的统计摘要。
// 使用：服务层直接返回给前端详情 Drawer 展示。
// 边界：该结构不包含原始蓝图编码，避免把大文本反复传给前端。
type BlueprintDecodeResult struct {
	// Meta 职责：保存 BLUEPRINT 外层逗号字段、标题、版本和校验尾部；使用：详情页基础信息展示；边界：不验证 MD5F 正确性。
	Meta BlueprintMeta `json:"meta"`
	// Binary 职责：保存 gzip 解压后的蓝图头部、区域和建筑记录；使用：详情页和后续诊断读取；边界：只做只读结构解释，不保证覆盖所有未来字段。
	Binary BlueprintBinary `json:"binary"`
	// Summary 职责：保存建筑数量、连接数量和物品聚合；使用：详情页优先展示摘要；边界：由 Binary 派生，不作为持久化数据。
	Summary BlueprintSummary `json:"summary"`
}

// BlueprintMeta 描述 BLUEPRINT 外层可读元信息。
// 职责：把 URL 编码字段和 checksum 归一化为前端易展示字段。
// 使用：详情 Drawer 的基础信息区展示。
// 边界：字段含义来自社区格式观察，缺失字段保持空字符串。
type BlueprintMeta struct {
	// RawFields 职责：保存解码后的原始元信息字段；使用：排查版本差异；边界：不保证每个字段都有固定语义。
	RawFields []string `json:"rawFields"`
	// Title 职责：保存短标题或短描述；使用：详情主标题；边界：字段缺失时为空。
	Title string `json:"title"`
	// Description 职责：保存长标题或长描述；使用：详情说明展示；边界：字段缺失时为空。
	Description string `json:"description"`
	// GameVersion 职责：保存蓝图记录的游戏版本；使用：兼容性提示；边界：仅来自外层字段，不验证实际二进制版本。
	GameVersion string `json:"gameVersion"`
	// CreatedAt 职责：保存 .NET ticks 转换后的 ISO 时间；使用：详情时间展示；边界：ticks 无效时为空。
	CreatedAt string `json:"createdAt"`
	// Checksum 职责：保存尾部 MD5F 文本；使用：诊断展示；边界：当前解析器不校验该值。
	Checksum string `json:"checksum"`
	// Icons 职责：保存外层声明的 5 个图标字段；使用：后续图标映射扩展；边界：只保留原始 ID 文本。
	Icons []string `json:"icons"`
}

// BlueprintBinary 描述蓝图 gzip payload 内的二进制内容。
// 职责：承载头部布局字段、区域数组和建筑数组。
// 使用：详情页高级信息、后续可视化或诊断模块读取。
// 边界：不包含重新编码所需的原始字节偏移或 MD5F 状态。
type BlueprintBinary struct {
	// Version 职责：保存二进制蓝图版本；使用：判断结构兼容性；边界：建筑记录仍会按自身前缀兼容判断。
	Version int32 `json:"version"`
	// CursorOffset 职责：保存光标偏移；使用：详情展示蓝图放置锚点；边界：只读展示，不参与重定位。
	CursorOffset BlueprintPointInt `json:"cursorOffset"`
	// CursorTargetArea 职责：保存光标目标区域索引；使用：详情展示；边界：可能为游戏内部索引。
	CursorTargetArea int32 `json:"cursorTargetArea"`
	// DragBoxSize 职责：保存拖拽框尺寸；使用：详情展示蓝图占用范围；边界：不等同所有建筑包围盒。
	DragBoxSize BlueprintPointInt `json:"dragBoxSize"`
	// PrimaryAreaIdx 职责：保存主区域索引；使用：详情展示；边界：不做区域存在性二次修正。
	PrimaryAreaIdx int32 `json:"primaryAreaIdx"`
	// Areas 职责：保存区域记录；使用：区域列表展示；边界：顺序保持蓝图原始顺序。
	Areas []BlueprintArea `json:"areas"`
	// Buildings 职责：保存建筑记录；使用：建筑列表与统计；边界：大蓝图可能很多，前端需按需展示。
	Buildings []BlueprintBuilding `json:"buildings"`
	// TrailingBytes 职责：保存已知结构读取完成后剩余的尾部字节数；使用：兼容游戏新版附加字段并辅助诊断。
	// 边界：不解释这些字节含义，重新编码前仍需实现完整格式。
	TrailingBytes int `json:"trailingBytes"`
}

// BlueprintPointInt 描述整数二维坐标。
// 职责：统一保存蓝图头部中的 x/y 点位。
// 使用：CursorOffset 和 DragBoxSize 字段复用。
// 边界：不表达三维建筑坐标。
type BlueprintPointInt struct {
	// X 职责：保存 X 轴整数值；使用：布局字段展示；边界：不转换单位。
	X int32 `json:"x"`
	// Y 职责：保存 Y 轴整数值；使用：布局字段展示；边界：不转换单位。
	Y int32 `json:"y"`
}

// BlueprintPointFloat 描述建筑本地三维坐标。
// 职责：统一保存建筑偏移坐标。
// 使用：建筑记录的首端和第二端坐标复用。
// 边界：不转换为行星表面世界坐标。
type BlueprintPointFloat struct {
	// X 职责：保存 X 轴浮点值；使用：建筑坐标展示；边界：保持原始 float32 精度转 float64。
	X float32 `json:"x"`
	// Y 职责：保存 Y 轴浮点值；使用：建筑坐标展示；边界：保持原始 float32 精度转 float64。
	Y float32 `json:"y"`
	// Z 职责：保存 Z 轴浮点值；使用：建筑坐标展示；边界：保持原始 float32 精度转 float64。
	Z float32 `json:"z"`
}

// BlueprintArea 描述蓝图中的一个区域记录。
// 职责：保存区域层级、锚点、分段和宽高。
// 使用：详情页区域表展示。
// 边界：不负责解释 tropicAnchor 与行星纬线的具体游戏语义。
type BlueprintArea struct {
	// Index 职责：保存区域索引；使用：建筑 areaIndex 关联展示；边界：可能与数组下标不同。
	Index int8 `json:"index"`
	// ParentIndex 职责：保存父区域索引；使用：区域层级展示；边界：负值表示无父级或游戏内部状态。
	ParentIndex int8 `json:"parentIndex"`
	// TropicAnchor 职责：保存纬线锚点；使用：高级诊断展示；边界：不转换为角度。
	TropicAnchor int16 `json:"tropicAnchor"`
	// AreaSegments 职责：保存区域分段数；使用：高级诊断展示；边界：不参与前端布局。
	AreaSegments int16 `json:"areaSegments"`
	// AnchorLocalOffsetX 职责：保存锚点本地 X 偏移；使用：区域详情展示；边界：保持原始单位。
	AnchorLocalOffsetX int16 `json:"anchorLocalOffsetX"`
	// AnchorLocalOffsetY 职责：保存锚点本地 Y 偏移；使用：区域详情展示；边界：保持原始单位。
	AnchorLocalOffsetY int16 `json:"anchorLocalOffsetY"`
	// Width 职责：保存区域宽度；使用：区域尺寸展示；边界：不等同建筑数量。
	Width int16 `json:"width"`
	// Height 职责：保存区域高度；使用：区域尺寸展示；边界：不等同建筑数量。
	Height int16 `json:"height"`
}

// BlueprintBuilding 描述新版蓝图中的单个建筑记录。
// 职责：保存建筑索引、物品、坐标、连接、配方、参数和文本内容。
// 使用：建筑统计、详情抽样和后续图形化可视化读取。
// 边界：不解释配方、过滤器或参数的游戏业务含义。
type BlueprintBuilding struct {
	// Num 职责：保存新版建筑记录前缀或旧版首字段；使用：诊断格式分支；边界：旧版没有独立前缀时该值等于建筑索引。
	Num int32 `json:"num"`
	// Index 职责：保存建筑索引；使用：连接字段引用；边界：不保证与数组下标一致。
	Index int32 `json:"index"`
	// ItemID 职责：保存物品 ID；使用：建筑类型判断和名称映射；边界：未知 ID 原样展示。
	ItemID int16 `json:"itemId"`
	// ItemName 职责：保存内置常见物品名；使用：前端友好展示；边界：未内置时为空。
	ItemName string `json:"itemName"`
	// ModelIndex 职责：保存模型索引；使用：高级诊断；边界：不映射具体模型资源。
	ModelIndex int16 `json:"modelIndex"`
	// AreaIndex 职责：保存所属区域索引；使用：区域关联展示；边界：不验证区域是否存在。
	AreaIndex int8 `json:"areaIndex"`
	// LocalOffset 职责：保存建筑首端本地坐标；使用：详情展示和后续可视化；边界：不转换世界坐标。
	LocalOffset BlueprintPointFloat `json:"localOffset"`
	// Yaw 职责：保存首端水平角；使用：高级诊断；边界：不转换角度单位。
	Yaw float32 `json:"yaw"`
	// Tilt 职责：保存传送带或分拣器倾角；使用：高级诊断；边界：非相关建筑为 nil。
	Tilt *float32 `json:"tilt,omitempty"`
	// Pitch 职责：保存分拣器首端俯仰角；使用：高级诊断；边界：非分拣器为 nil。
	Pitch *float32 `json:"pitch,omitempty"`
	// LocalOffset2 职责：保存分拣器第二端坐标；使用：分拣器连接展示；边界：非分拣器为 nil。
	LocalOffset2 *BlueprintPointFloat `json:"localOffset2,omitempty"`
	// Yaw2 职责：保存分拣器第二端水平角；使用：高级诊断；边界：非分拣器为 nil。
	Yaw2 *float32 `json:"yaw2,omitempty"`
	// Tilt2 职责：保存分拣器第二端倾角；使用：高级诊断；边界：非分拣器为 nil。
	Tilt2 *float32 `json:"tilt2,omitempty"`
	// Pitch2 职责：保存分拣器第二端俯仰角；使用：高级诊断；边界：非分拣器为 nil。
	Pitch2 *float32 `json:"pitch2,omitempty"`
	// OutputObjIdx 职责：保存输出连接目标建筑索引；使用：连接统计和可视化；边界：-1 表示无连接。
	OutputObjIdx int32 `json:"outputObjIdx"`
	// InputObjIdx 职责：保存输入连接来源建筑索引；使用：连接统计和可视化；边界：-1 表示无连接。
	InputObjIdx int32 `json:"inputObjIdx"`
	// OutputToSlot 职责：保存输出目标槽位；使用：高级诊断；边界：不解释具体槽位语义。
	OutputToSlot int8 `json:"outputToSlot"`
	// InputFromSlot 职责：保存输入来源槽位；使用：高级诊断；边界：不解释具体槽位语义。
	InputFromSlot int8 `json:"inputFromSlot"`
	// OutputFromSlot 职责：保存输出来源槽位；使用：高级诊断；边界：不解释具体槽位语义。
	OutputFromSlot int8 `json:"outputFromSlot"`
	// InputToSlot 职责：保存输入目标槽位；使用：高级诊断；边界：不解释具体槽位语义。
	InputToSlot int8 `json:"inputToSlot"`
	// OutputOffset 职责：保存输出偏移；使用：高级诊断；边界：不解释游戏内部含义。
	OutputOffset int8 `json:"outputOffset"`
	// InputOffset 职责：保存输入偏移；使用：高级诊断；边界：不解释游戏内部含义。
	InputOffset int8 `json:"inputOffset"`
	// RecipeID 职责：保存配方 ID；使用：后续配方映射扩展；边界：0 通常表示无配方。
	RecipeID int16 `json:"recipeId"`
	// FilterID 职责：保存过滤器物品 ID；使用：后续物品映射扩展；边界：0 通常表示无过滤器。
	FilterID int16 `json:"filterId"`
	// Parameters 职责：保存建筑参数数组；使用：分流器、物流站等高级诊断；边界：不解释参数含义。
	Parameters []int32 `json:"parameters"`
	// Content 职责：保存建筑文本内容；使用：文本类建筑展示；边界：空字符串表示无内容。
	Content string `json:"content"`
}

// BlueprintSummary 描述从蓝图解析结果派生的聚合信息。
// 职责：为详情 Drawer 提供无需遍历建筑列表即可展示的摘要。
// 使用：前端摘要区、建筑统计区读取。
// 边界：统计只基于当前解析出的建筑，不读取外部物品表。
type BlueprintSummary struct {
	// AreaCount 职责：保存区域数量；使用：摘要展示；边界：等于 Binary.Areas 长度。
	AreaCount int `json:"areaCount"`
	// BuildingCount 职责：保存建筑数量；使用：摘要展示；边界：等于 Binary.Buildings 长度。
	BuildingCount int `json:"buildingCount"`
	// ConnectionCount 职责：保存存在输入或输出连接的建筑数量；使用：摘要展示复杂度；边界：不代表连接边去重数量。
	ConnectionCount int `json:"connectionCount"`
	// ParameterizedCount 职责：保存带参数数组的建筑数量；使用：摘要提示特殊建筑；边界：参数含义不在此解析。
	ParameterizedCount int `json:"parameterizedCount"`
	// ContentCount 职责：保存带文本内容的建筑数量；使用：摘要提示文本类信息；边界：空白内容不计入。
	ContentCount int `json:"contentCount"`
	// ItemGroups 职责：保存按 itemId 聚合的建筑数量；使用：详情页建筑组成展示；边界：未知物品名为空。
	ItemGroups []BlueprintItemGroup `json:"itemGroups"`
}

// BlueprintItemGroup 描述同一种物品在蓝图中的数量。
// 职责：把建筑列表聚合成 itemId、名称和数量。
// 使用：详情页建筑组成列表展示。
// 边界：只按 itemId 聚合，不区分模型索引或参数差异。
type BlueprintItemGroup struct {
	// ItemID 职责：保存物品 ID；使用：统计列表 key；边界：未知 ID 仍可展示。
	ItemID int16 `json:"itemId"`
	// ItemName 职责：保存内置物品名；使用：用户可读展示；边界：映射缺失时为空。
	ItemName string `json:"itemName"`
	// Count 职责：保存该物品建筑数量；使用：统计展示；边界：不代表消耗材料数量。
	Count int `json:"count"`
}

// 常见物品名称映射集中保存蓝图详情的轻量中文显示名。
// 职责：覆盖最常见物流建筑，提升详情页可读性。
// 使用：readBlueprintBuilding 和 buildBlueprintSummary 调用。
// 边界：不是完整物品表，未知 ID 保持空名称。
var commonBlueprintItemNames = map[int16]string{
	2001: "传送带 Mk.I",
	2002: "传送带 Mk.II",
	2003: "传送带 Mk.III",
	2011: "分拣器 Mk.I",
	2012: "分拣器 Mk.II",
	2013: "分拣器 Mk.III",
	2020: "四向分流器",
	2101: "小型储物仓",
	2102: "大型储物仓",
}

// ParseBlueprintCode 解析单个 BLUEPRINT 编码。
// 职责：完成外层字段拆分、payload 解压和二进制结构读取。
// 使用：服务层读取蓝图文件文本后调用。
// 边界：不校验 MD5F，不支持非 BLUEPRINT 文本，未知未来建筑分支会返回字段读取错误。
// 逻辑流：1. 拆分外层 meta/payload/checksum => 2. Base64 解码并 gzip 解压 => 3. 读取头部与建筑 => 4. 派生统计摘要。
func ParseBlueprintCode(input string) (BlueprintDecodeResult, error) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "BLUEPRINT:") {
		return BlueprintDecodeResult{}, fmt.Errorf("不是有效的 BLUEPRINT 编码")
	}

	firstQuote := strings.Index(input, "\"")
	lastQuote := strings.LastIndex(input, "\"")
	if firstQuote < 0 || lastQuote <= firstQuote {
		return BlueprintDecodeResult{}, fmt.Errorf("蓝图编码缺少 payload 引号")
	}

	metaPart := input[:firstQuote]
	payloadBase64 := input[firstQuote+1 : lastQuote]
	checksum := input[lastQuote+1:]

	meta := parseBlueprintMeta(metaPart, checksum)
	binaryPayload, err := decodeBlueprintPayload(payloadBase64)
	if err != nil {
		return BlueprintDecodeResult{}, err
	}

	reader := newBlueprintBinaryReader(binaryPayload)
	decodedBinary, err := readBlueprintBinary(reader)
	if err != nil {
		return BlueprintDecodeResult{}, err
	}
	trailingBytes := reader.remaining()
	decodedBinary.TrailingBytes = trailingBytes

	return BlueprintDecodeResult{
		Meta:    meta,
		Binary:  decodedBinary,
		Summary: buildBlueprintSummary(decodedBinary),
	}, nil
}

// parseBlueprintMeta 解析外层逗号字段。
// 职责：把 percent 编码字段转为可读文本，并抽取常用标题、版本、时间和图标。
// 使用：ParseBlueprintCode 调用。
// 边界：字段缺失时保持空值，单个字段解码失败时保留原文。
func parseBlueprintMeta(metaPart string, checksum string) BlueprintMeta {
	rawMeta := strings.TrimPrefix(metaPart, "BLUEPRINT:")
	fields := strings.Split(rawMeta, ",")
	for index, field := range fields {
		decoded, err := url.QueryUnescape(field)
		if err == nil {
			fields[index] = decoded
		}
	}

	icons := make([]string, 0, 5)
	for index := 2; index <= 6 && index < len(fields); index++ {
		icons = append(icons, fields[index])
	}

	return BlueprintMeta{
		RawFields:   fields,
		Title:       fieldAt(fields, 10),
		Description: fieldAt(fields, 14),
		GameVersion: fieldAt(fields, 9),
		CreatedAt:   blueprintTicksToISO(fieldAt(fields, 8)),
		Checksum:    checksum,
		Icons:       icons,
	}
}

// decodeBlueprintPayload 解码并解压蓝图 payload。
// 职责：处理 Base64 与 gzip 两层编码。
// 使用：ParseBlueprintCode 调用。
// 边界：返回解压后的原始二进制，不解释其中字段。
func decodeBlueprintPayload(payloadBase64 string) ([]byte, error) {
	compressed, err := base64.StdEncoding.DecodeString(payloadBase64)
	if err != nil {
		return nil, fmt.Errorf("蓝图 Base64 解码失败: %w", err)
	}

	gzipReader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("蓝图 gzip 打开失败: %w", err)
	}
	defer gzipReader.Close()

	payload, err := io.ReadAll(gzipReader)
	if err != nil {
		return nil, fmt.Errorf("蓝图 gzip 解压失败: %w", err)
	}
	return payload, nil
}

// readBlueprintBinary 读取解压后的蓝图二进制结构。
// 职责：按小端序读取头部、区域和多版本建筑记录。
// 使用：ParseBlueprintCode 调用。
// 边界：越界读取会返回错误，尾部未知字节由上层记录为 TrailingBytes。
func readBlueprintBinary(reader *blueprintBinaryReader) (BlueprintBinary, error) {
	version, err := reader.readInt32()
	if err != nil {
		return BlueprintBinary{}, err
	}
	cursorOffsetX, err := reader.readInt32()
	if err != nil {
		return BlueprintBinary{}, err
	}
	cursorOffsetY, err := reader.readInt32()
	if err != nil {
		return BlueprintBinary{}, err
	}
	cursorTargetArea, err := reader.readInt32()
	if err != nil {
		return BlueprintBinary{}, err
	}
	dragBoxSizeX, err := reader.readInt32()
	if err != nil {
		return BlueprintBinary{}, err
	}
	dragBoxSizeY, err := reader.readInt32()
	if err != nil {
		return BlueprintBinary{}, err
	}
	primaryAreaIdx, err := reader.readInt32()
	if err != nil {
		return BlueprintBinary{}, err
	}
	areaCount, err := reader.readUInt8()
	if err != nil {
		return BlueprintBinary{}, err
	}

	areas := make([]BlueprintArea, 0, int(areaCount))
	for index := 0; index < int(areaCount); index++ {
		area, err := readBlueprintArea(reader)
		if err != nil {
			return BlueprintBinary{}, err
		}
		areas = append(areas, area)
	}

	buildingCount, err := reader.readInt32()
	if err != nil {
		return BlueprintBinary{}, err
	}
	if buildingCount < 0 {
		return BlueprintBinary{}, fmt.Errorf("蓝图建筑数量无效: %d", buildingCount)
	}

	buildings := make([]BlueprintBuilding, 0, int(buildingCount))
	for index := 0; index < int(buildingCount); index++ {
		building, err := readBlueprintBuilding(reader)
		if err != nil {
			return BlueprintBinary{}, fmt.Errorf("解析第 %d 个建筑失败: %w", index+1, err)
		}
		buildings = append(buildings, building)
	}

	return BlueprintBinary{
		Version:          version,
		CursorOffset:     BlueprintPointInt{X: cursorOffsetX, Y: cursorOffsetY},
		CursorTargetArea: cursorTargetArea,
		DragBoxSize:      BlueprintPointInt{X: dragBoxSizeX, Y: dragBoxSizeY},
		PrimaryAreaIdx:   primaryAreaIdx,
		Areas:            areas,
		Buildings:        buildings,
	}, nil
}

// readBlueprintArea 读取单个区域记录。
// 职责：按文档顺序读取区域字段。
// 使用：readBlueprintBinary 循环调用。
// 边界：读取失败立即返回，避免后续字段错位。
func readBlueprintArea(reader *blueprintBinaryReader) (BlueprintArea, error) {
	index, err := reader.readInt8()
	if err != nil {
		return BlueprintArea{}, err
	}
	parentIndex, err := reader.readInt8()
	if err != nil {
		return BlueprintArea{}, err
	}
	tropicAnchor, err := reader.readInt16()
	if err != nil {
		return BlueprintArea{}, err
	}
	areaSegments, err := reader.readInt16()
	if err != nil {
		return BlueprintArea{}, err
	}
	anchorLocalOffsetX, err := reader.readInt16()
	if err != nil {
		return BlueprintArea{}, err
	}
	anchorLocalOffsetY, err := reader.readInt16()
	if err != nil {
		return BlueprintArea{}, err
	}
	width, err := reader.readInt16()
	if err != nil {
		return BlueprintArea{}, err
	}
	height, err := reader.readInt16()
	if err != nil {
		return BlueprintArea{}, err
	}

	return BlueprintArea{
		Index:              index,
		ParentIndex:        parentIndex,
		TropicAnchor:       tropicAnchor,
		AreaSegments:       areaSegments,
		AnchorLocalOffsetX: anchorLocalOffsetX,
		AnchorLocalOffsetY: anchorLocalOffsetY,
		Width:              width,
		Height:             height,
	}, nil
}

// readBlueprintBuilding 读取单个建筑记录并分派到对应格式。
// 职责：根据游戏结构中的新版负前缀识别新格式，否则按旧版无前缀记录解析。
// 使用：readBlueprintBinary 循环调用。
// 边界：旧版记录的首个 int32 已经是 index，不再当作版本前缀拒绝。
func readBlueprintBuilding(reader *blueprintBinaryReader) (BlueprintBuilding, error) {
	firstWord, err := reader.readInt32()
	if err != nil {
		return BlueprintBuilding{}, err
	}
	if firstWord == -100 {
		return readBlueprintBuildingV1WithPrefix(reader, firstWord)
	}
	if firstWord < -100 {
		return readBlueprintBuildingV2(reader, firstWord)
	}
	return readBlueprintBuildingLegacy(reader, firstWord)
}

// readBlueprintBuildingV2 读取带负数前缀的新版建筑记录。
// 职责：根据 itemId 条件读取传送带、分拣器扩展字段和新版通用字段。
// 使用：readBlueprintBuilding 在 firstWord <= -100 时调用。
// 边界：只处理当前已知新版结构，未来新增字段将留在尾部或由越界错误暴露。
func readBlueprintBuildingV2(reader *blueprintBinaryReader, num int32) (BlueprintBuilding, error) {
	index, err := reader.readInt32()
	if err != nil {
		return BlueprintBuilding{}, err
	}
	itemID, err := reader.readInt16()
	if err != nil {
		return BlueprintBuilding{}, err
	}
	modelIndex, err := reader.readInt16()
	if err != nil {
		return BlueprintBuilding{}, err
	}
	areaIndex, err := reader.readInt8()
	if err != nil {
		return BlueprintBuilding{}, err
	}
	localOffset, err := readBlueprintPointFloat(reader)
	if err != nil {
		return BlueprintBuilding{}, err
	}
	yaw, err := reader.readFloat32()
	if err != nil {
		return BlueprintBuilding{}, err
	}

	building := BlueprintBuilding{
		Num:         num,
		Index:       index,
		ItemID:      itemID,
		ItemName:    commonBlueprintItemNames[itemID],
		ModelIndex:  modelIndex,
		AreaIndex:   areaIndex,
		LocalOffset: localOffset,
		Yaw:         yaw,
	}

	if itemID > 2000 && itemID < 2020 {
		tilt, err := reader.readFloat32()
		if err != nil {
			return BlueprintBuilding{}, err
		}
		building.Tilt = &tilt
	}

	if itemID > 2010 && itemID < 2020 {
		pitch, err := reader.readFloat32()
		if err != nil {
			return BlueprintBuilding{}, err
		}
		localOffset2, err := readBlueprintPointFloat(reader)
		if err != nil {
			return BlueprintBuilding{}, err
		}
		yaw2, err := reader.readFloat32()
		if err != nil {
			return BlueprintBuilding{}, err
		}
		tilt2, err := reader.readFloat32()
		if err != nil {
			return BlueprintBuilding{}, err
		}
		pitch2, err := reader.readFloat32()
		if err != nil {
			return BlueprintBuilding{}, err
		}
		building.Pitch = &pitch
		building.LocalOffset2 = &localOffset2
		building.Yaw2 = &yaw2
		building.Tilt2 = &tilt2
		building.Pitch2 = &pitch2
	}

	if err := readBlueprintBuildingCommon(reader, &building, true); err != nil {
		return BlueprintBuilding{}, err
	}
	return building, nil
}

// readBlueprintBuildingV1WithPrefix 读取带 -100 前缀的中间版建筑记录。
// 职责：兼容游戏早期带负前缀但仍使用旧版坐标顺序的建筑结构。
// 使用：readBlueprintBuilding 在 firstWord == -100 时调用。
// 边界：该结构没有新版文本内容字段，忽略旧版未知保留字段并保留 Num 供诊断。
func readBlueprintBuildingV1WithPrefix(reader *blueprintBinaryReader, num int32) (BlueprintBuilding, error) {
	index, err := reader.readInt32()
	if err != nil {
		return BlueprintBuilding{}, err
	}
	areaIndex, err := reader.readInt8()
	if err != nil {
		return BlueprintBuilding{}, err
	}
	localOffset, err := readBlueprintPointFloat(reader)
	if err != nil {
		return BlueprintBuilding{}, err
	}
	localOffset2, err := readBlueprintPointFloat(reader)
	if err != nil {
		return BlueprintBuilding{}, err
	}
	yaw, err := reader.readFloat32()
	if err != nil {
		return BlueprintBuilding{}, err
	}
	yaw2, err := reader.readFloat32()
	if err != nil {
		return BlueprintBuilding{}, err
	}
	if _, err := reader.readInt32(); err != nil {
		return BlueprintBuilding{}, err
	}
	itemID, err := reader.readInt16()
	if err != nil {
		return BlueprintBuilding{}, err
	}
	modelIndex, err := reader.readInt16()
	if err != nil {
		return BlueprintBuilding{}, err
	}

	building := BlueprintBuilding{
		Num:          num,
		Index:        index,
		ItemID:       itemID,
		ItemName:     commonBlueprintItemNames[itemID],
		ModelIndex:   modelIndex,
		AreaIndex:    areaIndex,
		LocalOffset:  localOffset,
		Yaw:          yaw,
		LocalOffset2: &localOffset2,
		Yaw2:         &yaw2,
	}

	if err := readBlueprintBuildingCommon(reader, &building, false); err != nil {
		return BlueprintBuilding{}, err
	}
	return building, nil
}

// readBlueprintBuildingLegacy 读取没有新版负前缀的旧版建筑记录。
// 职责：兼容早期蓝图结构中以 index 直接开头的建筑记录，避免把 index=0 误判为不支持的前缀。
// 使用：readBlueprintBuilding 在 firstWord > -100 时调用。
// 边界：旧版记录没有 contentLength/content 字段，Content 保持空字符串；localOffset2/yaw2 按旧结构总是读取。
func readBlueprintBuildingLegacy(reader *blueprintBinaryReader, index int32) (BlueprintBuilding, error) {
	areaIndex, err := reader.readInt8()
	if err != nil {
		return BlueprintBuilding{}, err
	}
	localOffset, err := readBlueprintPointFloat(reader)
	if err != nil {
		return BlueprintBuilding{}, err
	}
	localOffset2, err := readBlueprintPointFloat(reader)
	if err != nil {
		return BlueprintBuilding{}, err
	}
	yaw, err := reader.readFloat32()
	if err != nil {
		return BlueprintBuilding{}, err
	}
	yaw2, err := reader.readFloat32()
	if err != nil {
		return BlueprintBuilding{}, err
	}
	itemID, err := reader.readInt16()
	if err != nil {
		return BlueprintBuilding{}, err
	}
	modelIndex, err := reader.readInt16()
	if err != nil {
		return BlueprintBuilding{}, err
	}

	building := BlueprintBuilding{
		Num:          index,
		Index:        index,
		ItemID:       itemID,
		ItemName:     commonBlueprintItemNames[itemID],
		ModelIndex:   modelIndex,
		AreaIndex:    areaIndex,
		LocalOffset:  localOffset,
		Yaw:          yaw,
		LocalOffset2: &localOffset2,
		Yaw2:         &yaw2,
	}

	if err := readBlueprintBuildingCommon(reader, &building, false); err != nil {
		return BlueprintBuilding{}, err
	}
	return building, nil
}

// readBlueprintBuildingCommon 读取建筑通用字段。
// 职责：读取连接槽位、配方、过滤器、参数数组，并按版本决定是否读取 UTF-8 内容。
// 使用：新版和旧版建筑分支在扩展字段读取完成后调用。
// 边界：旧版记录没有 contentLength/content 字段；新版限制参数和内容长度不能为负，实际大小由 reader 越界检查兜底。
func readBlueprintBuildingCommon(reader *blueprintBinaryReader, building *BlueprintBuilding, hasContent bool) error {
	outputObjIdx, err := reader.readInt32()
	if err != nil {
		return err
	}
	inputObjIdx, err := reader.readInt32()
	if err != nil {
		return err
	}
	outputToSlot, err := reader.readInt8()
	if err != nil {
		return err
	}
	inputFromSlot, err := reader.readInt8()
	if err != nil {
		return err
	}
	outputFromSlot, err := reader.readInt8()
	if err != nil {
		return err
	}
	inputToSlot, err := reader.readInt8()
	if err != nil {
		return err
	}
	outputOffset, err := reader.readInt8()
	if err != nil {
		return err
	}
	inputOffset, err := reader.readInt8()
	if err != nil {
		return err
	}
	recipeID, err := reader.readInt16()
	if err != nil {
		return err
	}
	filterID, err := reader.readInt16()
	if err != nil {
		return err
	}
	parameterCount, err := reader.readInt16()
	if err != nil {
		return err
	}
	if parameterCount < 0 {
		return fmt.Errorf("建筑参数数量无效: %d", parameterCount)
	}
	parameters := make([]int32, 0, int(parameterCount))
	for index := 0; index < int(parameterCount); index++ {
		parameter, err := reader.readInt32()
		if err != nil {
			return err
		}
		parameters = append(parameters, parameter)
	}
	content := ""
	if hasContent {
		contentLength, err := reader.readInt32()
		if err != nil {
			return err
		}
		if contentLength < 0 {
			return fmt.Errorf("建筑内容长度无效: %d", contentLength)
		}
		contentBytes, err := reader.readBytes(int(contentLength))
		if err != nil {
			return err
		}
		content = string(contentBytes)
	}

	building.OutputObjIdx = outputObjIdx
	building.InputObjIdx = inputObjIdx
	building.OutputToSlot = outputToSlot
	building.InputFromSlot = inputFromSlot
	building.OutputFromSlot = outputFromSlot
	building.InputToSlot = inputToSlot
	building.OutputOffset = outputOffset
	building.InputOffset = inputOffset
	building.RecipeID = recipeID
	building.FilterID = filterID
	building.Parameters = parameters
	building.Content = content
	return nil
}

// readBlueprintPointFloat 读取三维 float32 坐标。
// 职责：统一读取 x/y/z 三个连续小端浮点数。
// 使用：建筑首端和第二端坐标读取时调用。
// 边界：不做坐标合法性判断。
func readBlueprintPointFloat(reader *blueprintBinaryReader) (BlueprintPointFloat, error) {
	x, err := reader.readFloat32()
	if err != nil {
		return BlueprintPointFloat{}, err
	}
	y, err := reader.readFloat32()
	if err != nil {
		return BlueprintPointFloat{}, err
	}
	z, err := reader.readFloat32()
	if err != nil {
		return BlueprintPointFloat{}, err
	}
	return BlueprintPointFloat{X: x, Y: y, Z: z}, nil
}

// buildBlueprintSummary 从解析结果中构建摘要统计。
// 职责：统计区域、建筑、连接、参数、内容和物品聚合数量。
// 使用：ParseBlueprintCode 成功后调用。
// 边界：不读取外部物品表，不解释参数业务含义。
func buildBlueprintSummary(binaryPayload BlueprintBinary) BlueprintSummary {
	itemCounts := map[int16]int{}
	connectionCount := 0
	parameterizedCount := 0
	contentCount := 0
	for _, building := range binaryPayload.Buildings {
		itemCounts[building.ItemID]++
		if building.OutputObjIdx >= 0 || building.InputObjIdx >= 0 {
			connectionCount++
		}
		if len(building.Parameters) > 0 {
			parameterizedCount++
		}
		if strings.TrimSpace(building.Content) != "" {
			contentCount++
		}
	}

	itemGroups := make([]BlueprintItemGroup, 0, len(itemCounts))
	for itemID, count := range itemCounts {
		itemGroups = append(itemGroups, BlueprintItemGroup{
			ItemID:   itemID,
			ItemName: commonBlueprintItemNames[itemID],
			Count:    count,
		})
	}
	sort.Slice(itemGroups, func(i int, j int) bool {
		if itemGroups[i].Count != itemGroups[j].Count {
			return itemGroups[i].Count > itemGroups[j].Count
		}
		return itemGroups[i].ItemID < itemGroups[j].ItemID
	})

	return BlueprintSummary{
		AreaCount:          len(binaryPayload.Areas),
		BuildingCount:      len(binaryPayload.Buildings),
		ConnectionCount:    connectionCount,
		ParameterizedCount: parameterizedCount,
		ContentCount:       contentCount,
		ItemGroups:         itemGroups,
	}
}

// fieldAt 安全读取字段数组中的指定位置。
// 职责：避免蓝图外层字段缺失造成越界。
// 使用：parseBlueprintMeta 调用。
// 边界：索引不存在时返回空字符串。
func fieldAt(fields []string, index int) string {
	if index < 0 || index >= len(fields) {
		return ""
	}
	return fields[index]
}

// blueprintTicksToISO 将 .NET ticks 转为 ISO 时间。
// 职责：把游戏蓝图外层创建时间转为前端可直接格式化的字符串。
// 使用：parseBlueprintMeta 调用。
// 边界：ticks 无效或超出 time 范围时返回空字符串。
func blueprintTicksToISO(rawTicks string) string {
	rawTicks = strings.TrimSpace(rawTicks)
	if rawTicks == "" {
		return ""
	}
	ticks, err := strconv.ParseInt(rawTicks, 10, 64)
	if err != nil {
		return ""
	}
	const unixEpochTicks int64 = 621355968000000000
	unixMs := (ticks - unixEpochTicks) / 10000
	if unixMs <= 0 {
		return ""
	}
	return time.UnixMilli(unixMs).UTC().Format(time.RFC3339)
}

// blueprintBinaryReader 提供带越界检查的小端读取器。
// 职责：封装蓝图二进制读取偏移和基础类型读取方法。
// 使用：解析器内部按字段顺序读取。
// 边界：不是通用流式 reader，只处理内存中的完整 payload。
type blueprintBinaryReader struct {
	// data 职责：保存完整二进制 payload；使用：readBytes 切片读取；边界：解析期间只读。
	data []byte
	// offset 职责：保存当前读取偏移；使用：每次成功读取后推进；边界：越界时不推进。
	offset int
}

// newBlueprintBinaryReader 创建蓝图二进制读取器。
// 职责：初始化 payload 和偏移。
// 使用：ParseBlueprintCode 解压完成后调用。
// 边界：不复制 data，调用方不应在解析期间修改切片。
func newBlueprintBinaryReader(data []byte) *blueprintBinaryReader {
	return &blueprintBinaryReader{data: data}
}

// remaining 返回尚未读取的字节数。
// 职责：辅助尾部校验和错误提示。
// 使用：ParseBlueprintCode 和 readBytes 调用。
// 边界：offset 异常时返回 0 兜底。
func (reader *blueprintBinaryReader) remaining() int {
	if reader.offset >= len(reader.data) {
		return 0
	}
	return len(reader.data) - reader.offset
}

// isEnd 判断读取器是否到达 payload 末尾。
// 职责：确保字段读取未遗漏尾部字节。
// 使用：ParseBlueprintCode 完成后调用。
// 边界：只比较 offset 和长度，不判断字段语义。
func (reader *blueprintBinaryReader) isEnd() bool {
	return reader.offset == len(reader.data)
}

// readBytes 读取固定长度字节片段。
// 职责：执行统一越界检查并推进偏移。
// 使用：基础类型读取和建筑 content 读取调用。
// 边界：长度为负或超过剩余字节时返回错误。
func (reader *blueprintBinaryReader) readBytes(length int) ([]byte, error) {
	if length < 0 {
		return nil, fmt.Errorf("读取长度无效: %d", length)
	}
	if length > reader.remaining() {
		return nil, fmt.Errorf("蓝图二进制读取越界: offset=%d need=%d remaining=%d", reader.offset, length, reader.remaining())
	}
	start := reader.offset
	reader.offset += length
	return reader.data[start:reader.offset], nil
}

// readInt8 读取有符号 8 位整数。
// 职责：按蓝图字段类型读取 int8。
// 使用：区域、建筑槽位和偏移字段读取。
// 边界：越界时返回错误。
func (reader *blueprintBinaryReader) readInt8() (int8, error) {
	data, err := reader.readBytes(1)
	if err != nil {
		return 0, err
	}
	return int8(data[0]), nil
}

// readUInt8 读取无符号 8 位整数。
// 职责：按蓝图字段类型读取 uint8。
// 使用：区域数量读取。
// 边界：越界时返回错误。
func (reader *blueprintBinaryReader) readUInt8() (uint8, error) {
	data, err := reader.readBytes(1)
	if err != nil {
		return 0, err
	}
	return data[0], nil
}

// readInt16 读取小端有符号 16 位整数。
// 职责：按蓝图字段类型读取 int16。
// 使用：区域尺寸、物品 ID、配方和过滤器字段读取。
// 边界：越界时返回错误。
func (reader *blueprintBinaryReader) readInt16() (int16, error) {
	data, err := reader.readBytes(2)
	if err != nil {
		return 0, err
	}
	return int16(binary.LittleEndian.Uint16(data)), nil
}

// readInt32 读取小端有符号 32 位整数。
// 职责：按蓝图字段类型读取 int32。
// 使用：头部、建筑索引、连接索引和参数字段读取。
// 边界：越界时返回错误。
func (reader *blueprintBinaryReader) readInt32() (int32, error) {
	data, err := reader.readBytes(4)
	if err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(data)), nil
}

// readFloat32 读取小端 float32。
// 职责：按蓝图字段类型读取坐标和角度。
// 使用：建筑坐标、yaw、tilt 和 pitch 字段读取。
// 边界：越界时返回错误，不检查 NaN 或无穷值。
func (reader *blueprintBinaryReader) readFloat32() (float32, error) {
	data, err := reader.readBytes(4)
	if err != nil {
		return 0, err
	}
	return math.Float32frombits(binary.LittleEndian.Uint32(data)), nil
}
