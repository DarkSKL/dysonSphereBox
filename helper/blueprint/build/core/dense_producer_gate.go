package build

// 文件职责：把生产树接入唯一共同三模型gate构造。
// 使用：真实树缺省使用同一profile和地面优先角色；边界：未知profile明确拒绝，不使用猜测坐标或换编号兜底。

import (
	"context"
)

// denseProducerGateModule 职责：承载共享构造器返回的真实门与有向接带；使用：独立生产树装配；边界：input按外→门、outputs按门→外排序，实体已注册到builder。
type denseProducerGateModule struct {
	gate            *placed
	entities, input []*placed
	outputs         map[int8][]*placed
	inputSlot       int8
	outputSlots     []int8
}

// buildDenseProducerGate 职责：通过共同真实门构造生产树节点；使用：明确生产树候选；边界：必须具备真实模型与角色证明，不接受测试替代工厂。
func (b *builder) buildDenseProducerGate(ctx context.Context, count int) (*denseProducerGateModule, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	spec, err := defaultSplitterGateSpec(ctx, count)
	if err != nil {
		return nil, err
	}
	module, err := b.buildSplitterGate(ctx, spec)
	if err != nil {
		return nil, err
	}
	return &denseProducerGateModule{gate: module.gate, entities: module.entities, input: module.input, outputs: module.outputs, inputSlot: module.inputSlot, outputSlots: append([]int8{}, module.outputSlots...)}, nil
}
