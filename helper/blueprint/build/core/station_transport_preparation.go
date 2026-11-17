package build

// 文件职责：组织落位后的塔端口分配与真实带端几何准备；使用：共同装配在主路布线前调用。
// 边界：保持原分配、交付端、生产源与配方入口的先后次序，非普通大厂由原几何条件保持原行为。
import "context"

// prepareStationTransportEntries 职责：完成端口分配和独立实体接驳准备，使输入重分配不再承担升降带链的副作用。
// 使用：全场布局已经固定、尚未开始主路时；边界：失败原样返回，不修改规范流量、制造设备和物理阈值。
func (b *builder) prepareStationTransportEntries(ctx context.Context) error {
	if err := b.assignStationInputPorts(ctx); err != nil {
		return err
	}
	return b.raiseStationProductionEntries(ctx)
}
