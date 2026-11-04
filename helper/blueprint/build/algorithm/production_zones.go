package build

// 文件职责：从实际产线归属重建最终逻辑分区，支持空间交错后不再连续的实体序列。
// 使用：统一结果收尾；边界：分区只描述归属范围，不恢复矩形独占或移动实体，共用设施保持共享身份。

// finalProductionZones 职责：按owner与ProductionIndices保留多目标和共用产线边界；使用：最终布局后；边界：没有自有实体的区不凭空占地。
func (b *builder) finalProductionZones() []Zone {
	byOwner := map[int][]*placed{}
	for _, entity := range b.all {
		if entity.owner >= 0 {
			byOwner[entity.owner] = append(byOwner[entity.owner], entity)
		}
	}
	zones := make([]Zone, 0, len(b.zones))
	for _, source := range b.zones {
		entities := []*placed{}
		for _, owner := range source.ProductionIndices {
			entities = append(entities, byOwner[owner]...)
		}
		if len(entities) == 0 {
			continue
		}
		low, high := bounds(entities)
		zone := source
		zone.MinX, zone.MinY, zone.MaxX, zone.MaxY = low.x, low.y, high.x, high.y
		zones = append(zones, zone)
	}
	return zones
}
