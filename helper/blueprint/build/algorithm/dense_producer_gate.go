package build

// 文件职责：把生产树接入共同三模型gate构造，并隔离私有测试注入接口。
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

// denseProducerGateFactoryKey 职责：隔离显式替代构造器；使用：纯图反例测试；边界：正常候选默认使用共同角色和真实构造，不由用户设置注入。
type denseProducerGateFactoryKey struct{}

// denseProducerGateFactory 职责：把共同角色picker与真实构造结果适配到生产树；使用：候选context；边界：由模型选择地面输入和优先地面输出，不放宽几何或升级带等级。
type denseProducerGateFactory func(context.Context, *builder, int) (*denseProducerGateModule, error)

// buildDenseProducerGate 职责：调用共同真实门或显式私有测试工厂；使用：生产树创建节点；边界：实际模型和角色由共同profile选择，未启用生产树时不改变原运输路径。
func (b *builder) buildDenseProducerGate(ctx context.Context, count int) (*denseProducerGateModule, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	factory, _ := ctx.Value(denseProducerGateFactoryKey{}).(denseProducerGateFactory)
	if factory == nil {
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
	return factory(ctx, b, count)
}
