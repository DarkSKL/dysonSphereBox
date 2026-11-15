package build

// 文件职责：统一带节点球与缩短胶囊的实际占用形状。
// 使用：最终建筑占用检查及原生球面装箱预筛选；边界：不提供连接豁免，不改变静态原型盒或带节点合并规则。

import (
	"DysonSphereBox/helper/blueprint/build/placement"
	"math"
)

// surfaceBeltNodeRadius 职责：保存最终检查使用的节点球半径；使用：宽阶段和节点球/胶囊窄阶段；边界：装箱不得另用更大的球直接拒绝合法拐角。
const surfaceBeltNodeRadius = .23

// surfaceBeltNodeProbe 职责：保存同一带节点的球心与缩短胶囊；使用：两个形状都侵入建筑盒时才拒绝；边界：没有两个真实带邻居时胶囊退化为节点球心。
type surfaceBeltNodeProbe struct {
	// center 职责：保存带节点径向抬升后的球心；使用：宽阶段和节点球检测；边界：不代替胶囊二次检查。
	center placement.Vector
	// span 职责：保存按实际带邻居缩短的胶囊轴线；使用：节点球命中后的确认；边界：不延长到完整带段。
	span surfaceBeltSpan
}

// beltNodeProbe 职责：从真实世界节点及两侧带邻居构造原生检测形状；使用：最终检查和装箱共用；边界：through仅在两侧均为已知真实带邻居时启用，球面端点保持原径向长度。
func (encoding *surfaceEncoding) beltNodeProbe(point, before, after placement.Vector, through bool) surfaceBeltNodeProbe {
	radial := encoding.localUp(point).Scale(.2)
	first, second := point, point
	if through {
		first = point.Add(before.Sub(point).Scale(.65))
		second = point.Add(after.Sub(point).Scale(.65))
		delta := second.Sub(first).Scale(.45)
		if encoding.planar {
			first, second = first.Add(delta), second.Sub(delta)
		} else {
			length := math.Sqrt(point.Dot(point))
			first, second = surfaceUnit(first.Add(delta)).Scale(length), surfaceUnit(second.Sub(delta)).Scale(length)
		}
	}
	return surfaceBeltNodeProbe{center: point.Add(radial), span: surfaceBeltSpan{ends: [2]placement.Vector{first.Add(radial), second.Add(radial)}}}
}

// overlapsBox 职责：按正式占用检查确认节点球与缩短胶囊是否同时侵入盒体；使用：原生装箱预筛选；边界：连接及附加通道豁免仍由最终带建筑检查负责。
func (probe surfaceBeltNodeProbe) overlapsBox(box placement.Box) bool {
	limit := surfaceBeltNodeRadius*surfaceBeltNodeRadius - 1e-6
	return box.SegmentDistanceSquared(probe.center, probe.center) < limit && box.SegmentDistanceSquared(probe.span.ends[0], probe.span.ends[1]) < limit
}
