package build

// 文件职责：验证球面单元预置供电的真实阶段边界；使用：小型内存单元定向回归；边界：不启动完整生成，不修改求解策略或超时预算。

import (
	"context"
	"testing"
)

// TestDenseUnitPowerProgress 职责：验证单元检查与真实补电完成后才递增；使用：一个无耗电带头和一个耗电单元；边界：只验证观察顺序与提交事实，不假定塔位选择。
func TestDenseUnitPowerProgress(t *testing.T) {
	b, err := newBuilder(Setting{BeltLv: 1})
	if err != nil {
		t.Fatal(err)
	}
	b.logistics = &LogisticsSummary{}
	belt, machine := b.make(2001, -20, 0, 0), b.make(2301, 0, 0, 0)
	machine.record.RecipeID = 1
	units := []denseSpatialUnit{{entities: []*placed{belt}}, {entities: []*placed{machine}}}
	var events []Progress
	parent := context.WithValue(context.Background(), denseUnitPowerRequestedKey{}, true)
	ctx := WithProgress(parent, func(event Progress) {
		events = append(events, event)
		if event.Current == len(units) && len(units[1].entities) <= 1 {
			t.Fatal("电塔提交前发布了单元供电完成")
		}
	})
	if err := b.reserveDenseUnitPower(ctx, units); err != nil {
		t.Fatal(err)
	}
	if len(events) != 4 {
		t.Fatalf("阶段数量不符：%+v", events)
	}
	for i, event := range events[:3] {
		if event.Stage != "dense-unit-power" || event.Current != i || event.Total != len(units) {
			t.Fatalf("单元真实计数错误：%+v", event)
		}
	}
	if event := events[3]; event.Stage != "dense-unit-power-complete" || event.Current != len(units) || event.Total != len(units) {
		t.Fatalf("缺少真实完成阶段：%+v", event)
	}
}

// TestDenseUnitPowerProgressDisabledAndCanceled 职责：验证关闭供电不报阶段且取消不伪造完成；使用：入口通知后立即取消；边界：保留原context取消结果与实体集合。
func TestDenseUnitPowerProgressDisabledAndCanceled(t *testing.T) {
	b := &builder{}
	events := 0
	disabled := WithProgress(context.Background(), func(Progress) { events++ })
	if err := b.reserveDenseUnitPower(disabled, nil); err != nil || events != 0 {
		t.Fatalf("未请求供电却进入预置阶段：events=%d err=%v", events, err)
	}
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	parent = context.WithValue(parent, denseUnitPowerRequestedKey{}, true)
	ctx := WithProgress(parent, func(event Progress) {
		events++
		if event.Stage != "dense-unit-power" || event.Current != 0 || event.Total != 1 {
			t.Fatalf("取消后报告了完成：%+v", event)
		}
		cancel()
	})
	err := b.reserveDenseUnitPower(ctx, []denseSpatialUnit{{}})
	if err != context.Canceled || events != 1 || len(b.all) != 0 {
		t.Fatalf("取消行为改变：events=%d err=%v", events, err)
	}
}
