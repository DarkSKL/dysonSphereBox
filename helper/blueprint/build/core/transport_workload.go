package build

// 文件职责：复用直连及普通黑盒塔的真实分行与物料分配估计候选接线工作量；使用：运输规格排序。
// 边界：只在当前请求内创建无实体探针，不输出物理可行证明，不裁剪档位、缓存动态失败或改变生产量。
import "context"

// transportCandidateRouteCount 职责：按明确带/臂档位与固定跨度统计实际供需路线数；使用：大厂排序探针。
// 边界：独立规范设置及物流摘要与同一档位一致，七端口和双排仍走原规划；错误仅表示无法估计，不删除候选。
func transportCandidateRouteCount(ctx context.Context, input *PreparedInput, spec TransportCandidateSpec, span float64) (int, error) {
	probe, err := input.newAssembly(ctx, nil)
	if err != nil {
		return 0, err
	}
	setting := input.request.Setting
	setting.BeltLv, setting.InserterLv = spec.BeltLevel, spec.InserterLevel
	setting, err = normalizeSetting(setting)
	if err != nil {
		return 0, err
	}
	probe.setting, probe.logistics = setting, newLogisticsSummary(*input.graph, setting)
	probe.denseSpatial, probe.denseMaxSpan, probe.denseCapacityFill = true, span, true
	request := input.request
	request.Setting = setting
	if _, err := probe.planTransportRows(ctx, request, input.dataset); err != nil {
		return 0, err
	}
	return len(probe.transportRoutes), nil
}
