package main

// 文件职责：维护 Wails 应用启动与服务绑定入口。
// 使用：由桌面应用启动流程调用，用于初始化后端并运行前端宿主。
// 边界：不实现具体业务流程，不定义数据库模型或前端组件。

import (
	"context"
	"fmt"

	"DysonSphereBox/core"
	legacyMigration "DysonSphereBox/helper/modDependence/installer/migration/legacy"
	v2Installer "DysonSphereBox/helper/modDependence/installer/v2"
	systemHelper "DysonSphereBox/helper/system"
	environmentService "DysonSphereBox/service/environment"
	"DysonSphereBox/utils/dysonsphere"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App 是 Wails 应用根绑定结构。
// 职责：保存运行时上下文，并承载必须依赖 Wails runtime 的窗口级能力。
// 使用：main.go 将 App 与业务服务一起绑定给 Wails，前端只调用这里的窗口级能力。
// 边界：业务服务通过 service 包注册，App 只保留生命周期和系统对话框这类运行时胶水。
type App struct {
	// 职责：保存 Wails runtime 上下文，供目录选择器等窗口级 API 使用。
	// 使用：startup 初始化后由 App 方法读取。
	// 边界：不得在业务 helper 或 service 中直接依赖该字段。
	ctx context.Context
}

// NewApp 创建 Wails 应用根绑定结构。
// 职责：构造应用根绑定对象并保持初始上下文为空。
// 使用：main.go 启动时创建一次，并与 service.RegisterServices 一起绑定给前端。
// 边界：不初始化数据库、不读取配置、不创建业务服务实例。
func NewApp() *App {
	return &App{}
}

// startup 在 Wails 启动时初始化全局基础设施。
// 职责：保存 runtime context、初始化数据库，并执行启动恢复任务。
// 使用：Wails 框架在应用启动后自动调用一次。
// 边界：启动恢复只调用 installer 子包的遗留文件修复和中断安装恢复；具体业务流程仍放在服务层。
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	core.GLOBAL.RuntimeContext = ctx

	if err := core.InitDatabase(databaseTemplate); err != nil {
		panic(fmt.Errorf("init database: %w", err))
	}
	if err := legacyMigration.RepairScopedBepInExFiles(); err != nil {
		fmt.Printf("repair legacy scoped mod files: %v\n", err)
	}
	if err := v2Installer.RecoverInterruptedRemotePackageInstalls(); err != nil {
		fmt.Printf("recover interrupted mod installs: %v\n", err)
	}
	environmentManagerService := environmentService.EnvironmentManagerService{}
	if _, err := environmentManagerService.RecoverInterruptedOperations(); err != nil {
		fmt.Printf("recover interrupted environment operations: %v\n", err)
	}
}

// shutdown 在 Wails 退出时释放数据库等基础设施资源。
// 职责：关闭数据库连接并释放应用级基础设施。
// 使用：Wails 框架在应用退出时自动调用。
// 边界：关闭失败只记录到控制台，避免退出流程二次 panic。
func (a *App) shutdown(ctx context.Context) {
	if err := core.CloseDatabase(); err != nil {
		fmt.Printf("close database: %v\n", err)
	}
}

// SelectDysonSphereDirectory 打开系统目录选择器并保存戴森球计划游戏目录。
// 职责：通过 Wails 目录选择器读取用户选择，归一化为游戏目录后写入配置并记录日志。
// 使用：前端需要用户手动选择游戏目录时调用；选择成功后写入配置并记录操作日志。
// 边界：用户取消选择时返回空路径且不视为错误；无法归一化为游戏目录时返回原路径但不写入配置。
func (a *App) SelectDysonSphereDirectory() (string, error) {
	if a.ctx == nil {
		err := fmt.Errorf("app context is not initialized")
		systemHelper.RecordGamePathFailure("selectGamePath", "用户选择游戏目录失败", err, "")
		return "", err
	}

	dirPath, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择戴森球计划安装目录",
	})
	if err != nil || dirPath == "" {
		if err == nil {
			return dirPath, nil
		}
		systemHelper.RecordGamePathFailure("selectGamePath", "用户选择游戏目录失败", err, dirPath)
		return dirPath, err
	}

	normalizedDirPath := dysonsphere.NormalizeDir(dirPath)
	if normalizedDirPath == "" {
		return dirPath, nil
	}

	if core.GLOBAL.Configure == nil || core.GLOBAL.Configure.GameStartFilePath == nil {
		err := fmt.Errorf("configure is not initialized")
		systemHelper.RecordGamePathFailure("selectGamePath", "用户选择游戏目录失败", err, normalizedDirPath)
		return "", err
	}
	if err := core.GLOBAL.Configure.GameStartFilePath.Set(normalizedDirPath); err != nil {
		systemHelper.RecordGamePathFailure("selectGamePath", "用户选择游戏目录失败", err, normalizedDirPath)
		return "", err
	}

	systemHelper.RecordGamePathSuccess("selectGamePath", "用户已选择游戏目录", normalizedDirPath, "manual")
	return normalizedDirPath, nil
}
