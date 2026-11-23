// Package build 职责：提供蓝图生成的稳定公开入口；使用：服务层调用 Generate；边界：全部任务由YAML工作流和领域工具执行。
package build

import (
	"context"

	algorithm "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/generator"
)

// Generate 职责：转发统一生成请求；使用：服务层传入任务配置和观察上下文；边界：不在公开入口内实现布局算法。
func Generate(ctx context.Context, request Request) (Result, error) {
	return generator.Generate(ctx, request)
}

// GenerateWithStationAlternative 为单星直连的实际选址返回同次生成中的另一种塔布局。
func GenerateWithStationAlternative(ctx context.Context, request Request) (Result, *algorithm.VerifiedSnapshot, error) {
	return generator.GenerateWithStationAlternative(ctx, request)
}
