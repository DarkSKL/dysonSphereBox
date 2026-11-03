// Package build 职责：提供蓝图生成的稳定公开入口；使用：服务层调用 Generate；边界：算法实现集中在 algorithm 子目录。
package build

import (
	"context"

	algorithm "DysonSphereBox/helper/blueprint/build/algorithm"
)

// Generate 职责：转发统一生成请求；使用：服务层传入任务配置和观察上下文；边界：不在公开入口内实现布局算法。
func Generate(ctx context.Context, request Request) (Result, error) {
	return algorithm.Generate(ctx, request)
}
