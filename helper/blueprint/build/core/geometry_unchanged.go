package build

// 文件职责：为已证明不改变内容的几何步骤共享冻结值；使用：空监测器、无运输绑定和无副产物优先关系。
// 边界：仍执行独立工作流节点和阶段核验；不共享可变工作区，下一次真实变换仍独占恢复。
import (
	"context"
	"fmt"
)

// advanceUnchangedGeometry 职责：只推进不可变快照的阶段标识；使用：调用者已核实本步骤没有可处理对象；边界：错误阶段、零值和取消拒绝。
func advanceUnchangedGeometry(ctx context.Context, draft GeometryDraft, from, to geometryPhase) (GeometryDraft, error) {
	if err := ctx.Err(); err != nil {
		return GeometryDraft{}, err
	}
	if draft.data == nil || draft.data.assembly.data == nil || draft.data.phase != from {
		return GeometryDraft{}, fmt.Errorf("几何步骤缺少正确前置阶段：需要%d", from)
	}
	next := *draft.data
	next.phase = to
	return GeometryDraft{&next}, nil
}
