package build

// 文件职责：对一个明确编码区域执行真实记录、设备事实和跨区引用前检。
// 使用：引擎regions遍历提交独立证据，再汇合后执行原完整Validate。
// 边界：只读定稿，不修复、不编码、不证明整个工厂连通，不把局部证据升级为最终编码权限。

import (
	"context"

	"DysonSphereBox/helper/blueprint"
	buildgeometry "DysonSphereBox/helper/blueprint/build/geometry"
	"DysonSphereBox/helper/transport"
)

// RegionCheckOptions 职责：控制区域报告的边界采样空间；使用：显式工具参数；边界：所有真实成员和入出边始终检查，不允许用采样减少硬校验。
type RegionCheckOptions struct {
	// BoundarySampleLimit 职责：限制报告保留的跨区引用条数；使用：零保留全部，正数保留前N条；边界：负数拒绝，未采样的引用仍完整检查。
	BoundarySampleLimit int `json:"boundarySampleLimit" yaml:"boundarySampleLimit"`
}

// RegionBoundaryReference 职责：记录一条真实跨Area输入/输出引用；使用：区域边界诊断；边界：是引用事实而非物料吞吐或整厂连通证明。
type RegionBoundaryReference struct {
	// SourceBuilding/TargetBuilding 职责：保存引用端和被引用端真实编号；使用：追踪边界；边界：不同于记录数组下标。
	SourceBuilding int32 `json:"sourceBuilding"`
	TargetBuilding int32 `json:"targetBuilding"`
	// SourceRegionID/TargetRegionID 职责：保存两端实际Area；使用：跨区连接汇合；边界：输入引用仍保留原记录方向，不伪装货物流向。
	SourceRegionID string `json:"sourceRegionId"`
	TargetRegionID string `json:"targetRegionId"`
	// ReferenceKind 职责：区分input和output字段；使用：准确解释记录；边界：不是无方向边。
	ReferenceKind string `json:"referenceKind"`
	// SourceSlot/TargetSlot 职责：保留两端实际槽位；使用：端口事实追踪；边界：动态带槽仍由最终完整规则重放。
	SourceSlot int8 `json:"sourceSlot"`
	TargetSlot int8 `json:"targetSlot"`
}

// RegionValidation 职责：封闭保存同一候选、内容指纹和真实区域的局部证据；使用：区域汇合；边界：零值无效，公开读取仅返回值副本，不能替代FinalValidation。
type RegionValidation struct {
	data *regionValidationData
}

// regionValidationData 职责：绑定不可变几何身份和独立报告；使用：核对区域证据归属；边界：不复制或持有可变装配器。
type regionValidationData struct {
	geometry      *finalizedGeometry
	fingerprint   [32]byte
	region        RegionDescriptor
	boundaries    []RegionBoundaryReference
	boundaryCount int
	bytes         int64
}

// ID 职责：返回已检查区域ID；使用：实例区域产物索引；边界：零值返回空串，不表示通过。
func (validation RegionValidation) ID() string {
	if validation.data == nil {
		return ""
	}
	return validation.data.region.ID
}

// Fingerprint 职责：返回证据绑定的几何内容指纹；使用：版本核对；边界：相同指纹仍必须校验候选身份，不能单独授权复用。
func (validation RegionValidation) Fingerprint() [32]byte {
	if validation.data == nil {
		return [32]byte{}
	}
	return validation.data.fingerprint
}

// BuildingCount 职责：返回该Area实际成员数量；使用：进度及诊断；边界：不包含跨区邻居，也不代表全图制造设备计数。
func (validation RegionValidation) BuildingCount() int {
	if validation.data == nil {
		return 0
	}
	return validation.data.region.BuildingCount
}

// EstimatedBytes 职责：返回区域报告保留空间的保守估计；使用：产物预算；边界：只读几何句柄共享内存另计，不代表进程RSS。
func (validation RegionValidation) EstimatedBytes() int64 {
	if validation.data == nil {
		return 0
	}
	return validation.data.bytes
}

// BoundaryReferenceCount 职责：返回全部已核对跨区引用数量；使用：核对采样是否截断报告；边界：双向字段分别计数，不声称是独立物理带数。
func (validation RegionValidation) BoundaryReferenceCount() int {
	if validation.data == nil {
		return 0
	}
	return validation.data.boundaryCount
}

// Boundaries 职责：返回明确边界引用的独立样本副本；使用：诊断；边界：修改副本不改变证据，实际已检查数量以BoundaryReferenceCount为准。
func (validation RegionValidation) Boundaries() []RegionBoundaryReference {
	if validation.data == nil {
		return nil
	}
	return append([]RegionBoundaryReference(nil), validation.data.boundaries...)
}

// Descriptor 职责：返回已检查区域事实的独立副本；使用：区域条件或结果展示；边界：Bands切片与证据隔离。
func (validation RegionValidation) Descriptor() RegionDescriptor {
	if validation.data == nil {
		return RegionDescriptor{}
	}
	result := validation.data.region
	result.Bands = append([]RegionLatitudeBand(nil), result.Bands...)
	return result
}

// regionRecordReference 职责：描述当前记录一条编号/槽位引用；使用：统一处理输入和输出边界；边界：只在本次检查内使用。
type regionRecordReference struct {
	target                 int32
	sourceSlot, targetSlot int8
	kind                   string
}

// regionReferences 职责：显式保留输入和输出的真实字段语义；使用：区域成员及入边扫描；边界：不修改或合并反向引用。
func regionReferences(record blueprint.Building) [2]regionRecordReference {
	return [2]regionRecordReference{{record.InputObjIdx, record.InputToSlot, record.InputFromSlot, "input"}, {record.OutputObjIdx, record.OutputFromSlot, record.OutputToSlot, "output"}}
}

// regionDeviceFacts 职责：复用共同原型查询并建立当前步骤缓存；使用：成员及直接边界邻居；边界：未知物品/模型或缺少已核对几何明确失败，不使用零占位。
func regionDeviceFacts(record blueprint.Building, facts map[[2]int16]buildgeometry.DeviceGeometry) (buildgeometry.DeviceGeometry, error) {
	key := [2]int16{record.ItemID, record.ModelIndex}
	if found, ok := facts[key]; ok {
		return found, nil
	}
	found, err := buildgeometry.QueryDeviceGeometry(record.ItemID, record.ModelIndex)
	if err != nil {
		return buildgeometry.DeviceGeometry{}, regionFailure("GEN_E_REGION_FACTS", "建筑%d的物品%d/模型%d没有完整设备事实：%v", record.Index, record.ItemID, record.ModelIndex, err)
	}
	if surfaceIsStation(record.ItemID) {
		ports, err := transport.StationPortCount(int(record.ItemID))
		if err != nil {
			return buildgeometry.DeviceGeometry{}, err
		}
		if _, err := transport.StationItemKindLimit(int(record.ItemID)); err != nil {
			return buildgeometry.DeviceGeometry{}, err
		}
		if len(found.Prefab.PortPoses) != ports {
			return buildgeometry.DeviceGeometry{}, regionFailure("GEN_E_REGION_FACTS", "物流塔%d端口目录和设备事实不一致", record.Index)
		}
	}
	facts[key] = found
	return found, nil
}

// regionReferencePorts 职责：按已有端口姿态目录核对当前引用的可解释性；使用：真实区域内边和边界；边界：不复制动态连接池重放、跨区碰撞或最终流量规则。
func regionReferencePorts(source, target blueprint.Building, reference regionRecordReference, facts map[[2]int16]buildgeometry.DeviceGeometry) error {
	targetFacts, err := regionDeviceFacts(target, facts)
	if err != nil {
		return err
	}
	if regionIsInserter(source.ItemID) && !surfaceIsBelt(target.ItemID) {
		if reference.targetSlot < 0 || int(reference.targetSlot) >= len(targetFacts.Prefab.SlotPoses) {
			return regionFailure("GEN_E_REGION_FACTS", "分拣器%d连接建筑%d的槽%d没有已核对姿态", source.Index, target.Index, reference.targetSlot)
		}
	} else if len(targetFacts.Prefab.PortPoses) > 0 {
		if reference.targetSlot < 0 || int(reference.targetSlot) >= len(targetFacts.Prefab.PortPoses) {
			return regionFailure("GEN_E_REGION_FACTS", "建筑%d引用建筑%d端口%d超出真实端口目录", source.Index, target.Index, reference.targetSlot)
		}
	}
	sourceFacts, err := regionDeviceFacts(source, facts)
	if err != nil {
		return err
	}
	if len(sourceFacts.Prefab.PortPoses) > 0 && (reference.sourceSlot < 0 || int(reference.sourceSlot) >= len(sourceFacts.Prefab.PortPoses)) {
		return regionFailure("GEN_E_REGION_FACTS", "建筑%d自身端口%d超出真实端口目录", source.Index, reference.sourceSlot)
	}
	return nil
}

// CheckGeometryRegion 职责：前检明确一个Area的真实位姿、原型和所有入出跨区引用；使用：regions单步；边界：不选择候选、不写几何，不授予编码权限，最终全图Validate仍必需。
// 逻辑流：核对不可变内容 => 解析实际成员与入出边界 => 完整查设备/端口事实和坐标 => 仅截取报告样本 => 绑定封闭区域证据。
func CheckGeometryRegion(ctx context.Context, snapshot GeometrySnapshot, regionID string, options RegionCheckOptions) (RegionValidation, error) {
	if err := ctx.Err(); err != nil {
		return RegionValidation{}, err
	}
	if options.BoundarySampleLimit < 0 {
		return RegionValidation{}, regionFailure("GEN_E_REGION_OPTIONS", "边界报告采样上限不能为负数")
	}
	view, err := readRegionGeometry(ctx, snapshot)
	if err != nil {
		return RegionValidation{}, err
	}
	area := -1
	for i := range view.members {
		if regionAreaID(i) == regionID {
			area = i
			break
		}
	}
	if area < 0 {
		return RegionValidation{}, regionFailure("GEN_E_REGION_UNKNOWN", "未知真实区域%q", regionID)
	}
	descriptor, err := describeRegion(ctx, snapshot, view, area)
	if err != nil {
		return RegionValidation{}, err
	}
	facts := make(map[[2]int16]buildgeometry.DeviceGeometry)
	touched := make(map[int]bool, len(view.members[area]))
	for _, index := range view.members[area] {
		touched[index] = true
	}
	boundaries := []RegionBoundaryReference{}
	count := 0
	for index, record := range view.records {
		if err := ctx.Err(); err != nil {
			return RegionValidation{}, err
		}
		local := int(record.AreaIndex) == area
		for _, reference := range regionReferences(record) {
			targetIndex, exists := view.byIndex[reference.target]
			if !local && (!exists || int(view.records[targetIndex].AreaIndex) != area) {
				continue
			}
			if reference.target == -1 {
				continue
			}
			if reference.target < -1 || !exists || reference.target == record.Index {
				return RegionValidation{}, regionFailure("GEN_E_REGION_REFERENCE", "区域%s建筑%d引用%d不存在或无效", regionID, record.Index, reference.target)
			}
			target := view.records[targetIndex]
			if err := regionReferencePorts(record, target, reference, facts); err != nil {
				return RegionValidation{}, err
			}
			touched[index], touched[targetIndex] = true, true
			if record.AreaIndex != target.AreaIndex {
				count++
				if options.BoundarySampleLimit == 0 || len(boundaries) < options.BoundarySampleLimit {
					boundaries = append(boundaries, RegionBoundaryReference{SourceBuilding: record.Index, TargetBuilding: target.Index, SourceRegionID: regionAreaID(int(record.AreaIndex)), TargetRegionID: regionAreaID(int(target.AreaIndex)), ReferenceKind: reference.kind, SourceSlot: reference.sourceSlot, TargetSlot: reference.targetSlot})
				}
			}
		}
	}
	records := make([]blueprint.Building, 0, len(touched))
	for i, record := range view.records {
		if touched[i] {
			if _, err := regionDeviceFacts(record, facts); err != nil {
				return RegionValidation{}, err
			}
			records = append(records, record)
		}
	}
	if _, _, err := regionPositions(ctx, view, records); err != nil {
		return RegionValidation{}, err
	}
	// 只在独立标量副本中移除与本区域无关的邻居外边，复用原有真实带距规则检查全部区域关联边。
	distanceRecords := append([]blueprint.Building(nil), records...)
	for i := range distanceRecords {
		record := &distanceRecords[i]
		if int(record.AreaIndex) == area {
			continue
		}
		if target, ok := view.byIndex[record.InputObjIdx]; !ok || int(view.records[target].AreaIndex) != area {
			record.InputObjIdx = -1
		}
		if target, ok := view.byIndex[record.OutputObjIdx]; !ok || int(view.records[target].AreaIndex) != area {
			record.OutputObjIdx = -1
		}
	}
	// 在相同独立引用视图中重放既有动态连接槽规则；所有完整全图端口及物流证明仍由最终Validate执行。
	if err := validatePastedConnections(ctx, distanceRecords); err != nil {
		if ctx.Err() != nil {
			return RegionValidation{}, ctx.Err()
		}
		return RegionValidation{}, err
	}
	if err := view.encoding.validateBeltDistancesContext(ctx, distanceRecords); err != nil {
		if ctx.Err() != nil {
			return RegionValidation{}, ctx.Err()
		}
		return RegionValidation{}, err
	}

	if err := ctx.Err(); err != nil {
		return RegionValidation{}, err
	}
	data := &regionValidationData{geometry: snapshot.data, fingerprint: snapshot.Fingerprint(), region: descriptor, boundaries: boundaries, boundaryCount: count}
	data.bytes = int64(256 + len(regionID) + len(descriptor.CoordinateFrame) + len(descriptor.Class) + len(descriptor.Bands)*64 + len(boundaries)*160)
	return RegionValidation{data: data}, nil
}

// VerifyRegionValidation 职责：拒绝零值、异候选、异内容或异区域证据；使用：实例提交和汇合；边界：只核对封闭局部证据，不替代完整Validate。
func VerifyRegionValidation(ctx context.Context, snapshot GeometrySnapshot, regionID string, validation RegionValidation) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := checkFinalGeometry(ctx, snapshot); err != nil {
		return err
	}
	return verifyRegionIdentity(snapshot, regionID, validation)
}

// verifyRegionIdentity 职责：在内容已由入口核对后匹配封闭证据身份；使用：单证据入口和同次汇合；边界：不单独对外授权，避免每个子区域再次序列化全图。
func verifyRegionIdentity(snapshot GeometrySnapshot, regionID string, validation RegionValidation) error {
	if validation.data == nil || validation.data.geometry != snapshot.data || validation.data.fingerprint != snapshot.Fingerprint() || validation.data.region.ID != regionID {
		return regionFailure("GEN_E_REGION_EVIDENCE", "区域%s缺少同候选同内容的局部证据", regionID)
	}
	return nil
}

// RequireGeometryRegions 职责：确认真实Area各有且仅有一个匹配的局部证据；使用：AllRegions汇合后进入最终Validate前；边界：不会签发FinalValidation或可编码快照。
func RequireGeometryRegions(ctx context.Context, snapshot GeometrySnapshot, validations []RegionValidation) error {
	view, err := readRegionGeometry(ctx, snapshot)
	if err != nil {
		return err
	}
	if len(validations) != len(view.members) {
		return regionFailure("GEN_E_REGION_EVIDENCE", "真实区域%d个与局部证据%d份不一致", len(view.members), len(validations))
	}
	seen := make(map[string]bool, len(validations))
	for _, validation := range validations {
		id := validation.ID()
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := verifyRegionIdentity(snapshot, id, validation); err != nil {
			return err
		}
		if seen[id] || validation.data.region.AreaIndex < 0 || validation.data.region.AreaIndex >= len(view.members) || id != regionAreaID(validation.data.region.AreaIndex) {
			return regionFailure("GEN_E_REGION_EVIDENCE", "区域%s的证据重复或归属无效", id)
		}
		seen[id] = true
	}
	return nil
}
