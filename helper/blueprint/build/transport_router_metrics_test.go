package build

// 文件职责：核对路由缓存权限、占位版本失效和确定性搜索预算计数。
// 使用：全局物流寻路回归；边界：只验证内存环境，不把缓存命中或有限展开当成游戏运行证据。

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// TestTransportRoutingBudgetAndCacheIsolation 职责：要求缓存遵守端口权限、占位版本和取消状态；使用：固定三维路线正反样本；边界：计数不依赖时间、分配地址或 map 遍历顺序。
func TestTransportRoutingBudgetAndCacheIsolation(t *testing.T) {
	source, target := &placed{}, &placed{}
	env := &transportEnvironment{blocked: map[vector]bool{}, reserved: map[vector]map[*placed]bool{{2, 0, 0}: {source: true}}, cache: map[transportPathCacheKey][]vector{}, minimum: vector{-2, -2, 0}, maximum: vector{10, 10, transportMaxHeight}}
	start, end := vector{0, 0, 0}, vector{4, 0, 0}
	first, expanded := env.findTransportPath(context.Background(), start, end, source, target, false)
	if len(first) != 5 || expanded == 0 || env.searchCalls != 1 || env.cacheHits != 0 || env.totalExpanded != expanded || env.maxExpanded != expanded {
		t.Fatalf("首次搜索计数错误：path=%v env=%+v", first, env)
	}
	second, cachedExpanded := env.findTransportPath(context.Background(), start, end, source, target, false)
	if !reflect.DeepEqual(first, second) || cachedExpanded != 0 || env.searchCalls != 2 || env.cacheHits != 1 || env.totalExpanded != expanded {
		t.Fatal("当前占位版本的同权限查询应命中缓存且不重复展开")
	}
	second[0] = vector{99, 99, 99}
	again, _ := env.findTransportPath(context.Background(), start, end, source, target, false)
	if !reflect.DeepEqual(first, again) {
		t.Fatal("缓存返回值不得共享可写路径切片")
	}
	env.version++
	if _, steps := env.findTransportPath(context.Background(), start, end, source, target, false); steps == 0 {
		t.Fatal("占位版本改变必须重新搜索")
	}
	otherSource := &placed{}
	detour, steps := env.findTransportPath(context.Background(), start, end, otherSource, target, false)
	if steps == 0 || len(detour) == 0 {
		t.Fatal("不同端口权限不得复用旧路径")
	}
	for _, point := range detour {
		if point == (vector{2, 0, 0}) {
			t.Fatal("其它端口的预留柱被旧缓存穿过")
		}
	}
	beforeCalls, beforeHits, beforeExpanded := env.searchCalls, env.cacheHits, env.totalExpanded
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if path, steps := env.findTransportPath(ctx, start, end, source, target, false); len(path) != 0 || steps != 0 {
		t.Fatal("已取消请求不得从热缓存返回成功路线")
	}
	if env.searchCalls != beforeCalls+1 || env.cacheHits != beforeHits || env.totalExpanded != beforeExpanded || env.maxExpanded > transportSearchLimit {
		t.Fatal("取消或缓存计数突破固定搜索预算")
	}
}

// TestLocalGenerationCancellation 职责：要求局部机器行、配送占位和补电遵守同一总取消上下文；使用：避免长循环发布取消后的部分结果；边界：未设置上下文的独立几何调用保持兼容。
func TestLocalGenerationCancellation(t *testing.T) {
	setting, err := normalizeSetting(Setting{BeltLv: 3, InserterLv: 3})
	if err != nil {
		t.Fatal(err)
	}
	b, err := newBuilder(setting)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b.generationCtx = ctx
	link := &placed{a: vector{0, 0, 0}}
	calls := []func() error{
		func() error { _, err := b.buildLine(quant.Production{}, quant.Recipe{}, 10000, 2, 0, true); return err },
		func() error { return b.validateFlyPathCandidate([]vector{{0, 0, 0}, {1, 0, 0}}, link, nil, nil) },
		func() error { return b.addFlyPower(vector{}, 0) },
	}
	for _, call := range calls {
		var failure *blueprint.Error
		if err := call(); !errors.As(err, &failure) || failure.Code != "GEN_E_TIMEOUT" {
			t.Fatalf("局部取消应返回统一超时错误：%v", err)
		}
	}
	if len(b.all) != 0 {
		t.Fatal("取消的局部装配不应创建任何新实体")
	}
}
