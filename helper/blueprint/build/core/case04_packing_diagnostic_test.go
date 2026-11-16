package build

// 文件职责：重放普通电磁涡轮大厂的真实失败规格并定位单元内带路；使用：共享装箱修复诊断；边界：不改变产量、规则或候选参数。

import (
	"context"
	"testing"
	"time"
)

// TestCase04PackingDiagnostic 职责：区分单元内部和跨单元冲突；使用：指定LARGE_CANDIDATE_REQUEST；边界：失败只输出局部证据，不把残缺装配当生成成功。
func TestCase04PackingDiagnostic(t *testing.T) {
	ctx, input := largeDiagnosticInput(t)
	ctx, cancel := context.WithTimeout(input.algorithmContext(ctx), 45*time.Second)
	defer cancel()
	spec := ConnectedCandidateSpec{SpatialRows: true, Limit: 217, GlobalPower: true, DenseSpan: 18}
	b, err := assembleConnectedCandidate(ctx, input, spec, nil)
	t.Logf("assembly error=%v", err)
	if b == nil || b.polarFrame == nil {
		return
	}
	t.Logf("all=%d lines=%d frame=%+v", len(b.all), len(b.machineLines), *b.polarFrame)
	for j := 13744; j < 13757 && j < len(b.all); j++ {
		p := b.all[j]
		q := -1
		for k, n := range b.all {
			if n == p.output {
				q = k
				break
			}
		}
		t.Logf("global%d item%d owner%d at%v output%d", j, p.record.ItemID, p.owner, p.a, q)
		for _, route := range b.transportRoutes {
			if route.start == p || route.end == p {
				t.Logf("endpoint edge=%s isStart=%v isEnd=%v", route.edge.ID, route.start==p, route.end==p)
			}
		}
	}
	t.Logf("global geometry error=%v", b.validateNativeUnitBelts(ctx, denseSpatialUnit{entities: b.all}))
	for i, line := range b.machineLines {
		if err := b.validateNativeUnitBelts(ctx, denseSpatialUnit{entities: line.all}); err != nil {
			t.Logf("line=%d size=%d err=%v", i, len(line.all), err)
			for j, p := range line.all {
				if j < 265 || j > 285 {
					continue
				}
				q := -1
				for k, n := range line.all {
					if n == p.output {
						q = k
						break
					}
				}
				t.Logf("local%d item%d at%v output%d next%v", j, p.record.ItemID, p.a, q, p.output)
			}
			break
		}
	}
}
