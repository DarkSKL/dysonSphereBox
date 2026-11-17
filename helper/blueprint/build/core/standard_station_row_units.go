package build

// 文件职责：普通塔行阵只合并制造单元，真实塔单元保持独立。
// 边界：塔间15米等物理规则仍由原球面装箱及终检执行，不以同一刚体隐藏塔间距离。
import "context"

// configureStandardStationRows 职责：复用普通行阵，同时保留每个真实塔的独立刚体身份。
// 使用：station黑盒RowPacking初始化；边界：不新增、复制、删减实体，不移动或扩大许可区域，原排列与最终规则继续有效。
func (b *builder) configureStandardStationRows(ctx context.Context) context.Context {
	ctx = b.configureStandardRows(ctx)
	rows := b.denseUnitOrder
	b.denseUnitOrder = func(ctx context.Context, units []denseSpatialUnit) ([]denseSpatialUnit, error) {
		machines, stations := []denseSpatialUnit{}, []denseSpatialUnit{}
		for _, u := range units {
			hasStation := false
			for _, p := range u.entities {
				if surfaceIsStation(p.record.ItemID) {
					hasStation = true
					break
				}
			}
			if hasStation {
				stations = append(stations, u)
			} else {
				machines = append(machines, u)
			}
		}
		if len(machines) > 0 {
			var err error
			machines, err = rows(ctx, machines)
			if err != nil {
				return nil, err
			}
		}
		return append(machines, stations...), nil
	}
	return ctx
}
