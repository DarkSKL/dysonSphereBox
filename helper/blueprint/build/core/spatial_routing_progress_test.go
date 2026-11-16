package build

// 文件职责：验证压缩与公共布线的真实阶段边界；使用：小型内存布局定向回归；边界：不启动完整生成、不修改候选预算或预览通知器。

import (
	"context"
	"testing"

	"DysonSphereBox/helper/blueprint"
)

// TestSpatialRoutingProgressCompact 职责：验证落位后的压缩入口、轮廓与逐轮单元计数；使用：双单元微型布局；边界：检查实际坐标处理，不假定移动次数或完整三轮。
func TestSpatialRoutingProgressCompact(t *testing.T) {
	b, err := newBuilder(Setting{BeltLv: 1})
	if err != nil {
		t.Fatal(err)
	}
	a, z := b.make(2001, 0, 0, 0), b.make(2001, 4, 0, 0)
	units := []denseSpatialUnit{{entities: []*placed{a}}, {entities: []*placed{z}}}
	var events []Progress
	ctx := WithProgress(context.Background(), func(event Progress) { events = append(events, event) })
	b.generationCtx = ctx
	if err := b.compactDenseSpatialUnits(ctx, units, 20); err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 || events[0].Stage != "dense-compact-prepare" || events[0].Current != 0 || events[0].Total != len(units) {
		t.Fatalf("缺少压缩入口事实：%+v", events)
	}
	prepared, passes, current := 0, 0, 0
	for _, event := range events {
		switch event.Stage {
		case "dense-compact-prepare":
			if event.Current != prepared || event.Total != len(units) {
				t.Fatalf("轮廓准备计数不连续：%+v", event)
			}
			prepared++
		case "dense-compact":
			if event.Current == 0 {
				if passes > 0 && current != len(units) {
					t.Fatal("上一轮未处理完就启动下一轮")
				}
				passes++
				current = 0
			} else {
				current++
				if event.Current != current {
					t.Fatalf("压缩单元计数不连续：%+v", event)
				}
			}
			if event.Total != len(units) {
				t.Fatalf("压缩使用了虚构轮次总数：%+v", event)
			}
		}
	}
	if prepared != len(units)+1 || passes < 1 || passes > 3 || current != len(units) {
		t.Fatalf("真实阶段不完整：prepared=%d passes=%d current=%d", prepared, passes, current)
	}
}

// TestSpatialRoutingProgressCompactCancellation 职责：验证压缩观察不破坏取消回滚；使用：实际开始检查时取消；边界：不得发布未执行单元完成事件。
func TestSpatialRoutingProgressCompactCancellation(t *testing.T) {
	b, err := newBuilder(Setting{BeltLv: 1})
	if err != nil {
		t.Fatal(err)
	}
	a, z := b.make(2001, 0, 0, 0), b.make(2001, 4, 0, 0)
	originalA, originalZ := *a, *z
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := WithProgress(parent, func(event Progress) {
		if event.Stage == "dense-compact" {
			if event.Current != 0 {
				t.Fatal("取消后发布了未处理单元的完成计数")
			}
			cancel()
		}
	})
	b.generationCtx = ctx
	err = b.compactDenseSpatialUnits(ctx, []denseSpatialUnit{{entities: []*placed{a}}, {entities: []*placed{z}}}, 20)
	if err == nil || a.a != originalA.a || a.b != originalA.b || z.a != originalZ.a || z.b != originalZ.b {
		t.Fatalf("取消未返回错误或改变了原坐标：%v", err)
	}
}

// TestSpatialRoutingProgressRoutes 职责：验证公共入口及策略入口的布线完成计数；使用：一条实际内连线及一个无需寻路的出货头；边界：完成事件必须晚于实体接线，跳过项不能计入总数。
func TestSpatialRoutingProgressRoutes(t *testing.T) {
	for _, entry := range []string{"default", "strategy"} {
		t.Run(entry, func(t *testing.T) {
			b, err := newBuilder(Setting{BeltLv: 1})
			if err != nil {
				t.Fatal(err)
			}
			b.logistics = &LogisticsSummary{}
			a, z := b.make(2001, 0, 0, 0), b.make(2001, 4, 0, 0)
			route := &transportRoute{from: &transportRow{}, to: &transportRow{}, start: a, end: z, path: []*placed{a}}
			b.transportRoutes = []*transportRoute{{from: &transportRow{}, start: a}, route}
			var events []Progress
			ctx := WithProgress(context.Background(), func(event Progress) {
				events = append(events, event)
				if event.Stage == "routing" && event.Current > 0 {
					if len(route.path) < 2 || route.path[len(route.path)-1] != z || a.output == nil {
						t.Fatal("接线完成前发布了完成计数")
					}
				}
			})
			if entry == "default" {
				err = b.routeTransport(ctx)
			} else {
				err = b.routeTransportWithStrategy(ctx, nil, true)
			}
			if err != nil {
				t.Fatal(err)
			}
			first := "routing-prepare"
			if entry == "default" {
				first = "routing-inputs"
			}
			if len(events) == 0 || events[0].Stage != first || events[0].Current != 0 || events[0].Total != 2 {
				t.Fatalf("缺少准备入口事实：%+v", events)
			}
			current := 0
			for _, event := range events {
				if event.Stage == "routing" {
					if event.Current != current || event.Total != 1 {
						t.Fatalf("实际布线计数错误：%+v", event)
					}
					current++
				}
			}
			if current != 2 || events[len(events)-1].Stage != "routing-finalize" {
				t.Fatalf("缺少实际完成或收尾阶段：%+v", events)
			}
		})
	}
}

// TestSpatialRoutingProgressRouteCancellation 职责：验证首条寻路耗时期间仍显示零完成；使用：真实寻路入口通知后取消；边界：保留原GEN_E_TIMEOUT，不生成假完成事件。
func TestSpatialRoutingProgressRouteCancellation(t *testing.T) {
	b, err := newBuilder(Setting{BeltLv: 1})
	if err != nil {
		t.Fatal(err)
	}
	b.logistics = &LogisticsSummary{}
	a, z := b.make(2001, 0, 0, 0), b.make(2001, 4, 0, 0)
	b.transportRoutes = []*transportRoute{{from: &transportRow{}, to: &transportRow{}, start: a, end: z, path: []*placed{a}}}
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := false
	ctx := WithProgress(parent, func(event Progress) {
		if event.Stage == "routing" {
			if event.Current != 0 || event.Total != 1 {
				t.Fatalf("首条实际寻路前计数错误：%+v", event)
			}
			started = true
			cancel()
		}
	})
	err = b.routeTransportWithStrategy(ctx, nil, true)
	failure, ok := err.(*blueprint.Error)
	if !started || !ok || failure.Code != "GEN_E_TIMEOUT" || a.output != nil {
		t.Fatalf("取消行为改变：started=%t err=%v", started, err)
	}
}
