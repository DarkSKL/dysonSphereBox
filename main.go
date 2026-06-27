package main

// 文件职责：维护 Wails 应用启动与服务绑定入口。
// 使用：由桌面应用启动流程调用，用于初始化后端并运行前端宿主。
// 边界：不实现具体业务流程，不定义数据库模型或前端组件。

import (
	"context"
	"embed"
	"os"

	overlayRuntime "DysonSphereBox/helper/overlay/runtime"
	"DysonSphereBox/service"
	overlayService "DysonSphereBox/service/overlay"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// assets 是打包进桌面程序的前端构建产物。
// 职责：提供 Wails asset server 读取的嵌入式前端静态资源。
// 使用：main 函数构造 assetserver.Options 时传入。
// 边界：由 Wails asset server 读取，不在运行时修改。
//
//go:embed all:frontend/dist
var assets embed.FS

// databaseTemplate 是首次启动时复制到运行目录的 SQLite 模板库。
// 职责：提供应用首次启动时创建本地数据库的嵌入式模板内容。
// 使用：App.startup 调用 core.InitDatabase 时传入。
// 边界：只用于初始化数据库文件，后续读写都通过 core.GLOBAL.DB。
//
//go:embed app.db
var databaseTemplate []byte

// main 启动 Wails 桌面应用。
// 职责：创建 App、聚合业务服务绑定并交给 Wails 运行桌面宿主。
// 使用：由 Go 程序入口自动调用。
// 边界：不执行业务初始化；数据库和恢复任务由 App.startup 生命周期处理。
func main() {
	if isOverlayMode(os.Args) {
		runOverlayApp()
		return
	}
	runMainApp()
}

// runMainApp 启动主桌面应用。
// 职责：运行完整主界面、数据库初始化和业务服务绑定。
// 使用：用户正常双击 DysonSphereBox.exe 时进入该模式。
// 边界：不直接渲染游戏悬浮层，悬浮层通过同一个 exe 的 --overlay 附进程启动。
func runMainApp() {
	app := NewApp()
	services := append([]interface{}{app}, service.RegisterServices()...)

	err := wails.Run(&options.App{
		Title:     "戴森球盒子",
		Width:     1024,
		Height:    768,
		MinWidth:  1024,
		MinHeight: 768,
		Frameless: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Windows: &windows.Options{
			IsZoomControlEnabled: false,
			ZoomFactor:           1,
			DisablePinchZoom:     true,
		},
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: true,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind:       services,
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

// OverlayApp 是 Overlay 附进程的 Wails 根绑定结构。
// 职责：保存附进程 runtime context，并启动浏览器窗口贴附同步循环。
// 使用：runOverlayApp 在 --overlay 模式下绑定给 Wails 前端。
// 边界：不初始化主程序数据库，不注册主程序业务服务。
type OverlayApp struct {
	// 职责：保存 Wails runtime 上下文。
	// 使用：domReady 中切换到 Overlay 前端壳。
	// 边界：不得传递给主进程服务使用。
	ctx context.Context
	// 职责：管理附进程内部的窗口贴附循环。
	// 使用：startup 启动，shutdown 停止。
	// 边界：只控制当前附进程窗口。
	controller *overlayRuntime.Controller
}

// NewOverlayApp 创建 Overlay 附进程根绑定对象。
// 职责：构造附进程专用控制器。
// 使用：runOverlayApp 启动前调用一次。
// 边界：不启动窗口同步循环。
func NewOverlayApp() *OverlayApp {
	return &OverlayApp{controller: overlayRuntime.NewController()}
}

// startup 初始化 Overlay 附进程运行时。
// 职责：保存 Wails context 并启动浏览器窗口贴附循环。
// 使用：Wails 在 --overlay 模式启动后自动调用。
// 边界：不打开数据库，不执行主程序恢复任务。
func (app *OverlayApp) startup(ctx context.Context) {
	app.ctx = ctx
	app.controller.Start()
}

// domReady 在 Overlay 前端加载完成后切换到独立悬浮窗路由。
// 职责：让同一份前端资源在附进程中进入 /overlay 页面。
// 使用：Wails DOM ready 生命周期自动调用。
// 边界：只设置 hash 路由，不修改主进程窗口状态。
func (app *OverlayApp) domReady(ctx context.Context) {
	wailsRuntime.WindowExecJS(ctx, `window.location.hash = "#/overlay"`)
}

// shutdown 释放 Overlay 附进程内部同步循环。
// 职责：停止窗口贴附后台 goroutine。
// 使用：Overlay 附进程退出时由 Wails 调用。
// 边界：不影响主进程管理器状态。
func (app *OverlayApp) shutdown(ctx context.Context) {
	app.controller.Stop()
}

// GetRuntimeMode 返回当前 Wails 进程的运行模式。
// 使用：前端启动时区分主应用壳与 Overlay 悬浮窗壳。
// 边界：该方法在 Overlay 附进程中始终返回 overlay。
func (app *OverlayApp) GetRuntimeMode() string {
	return "overlay"
}

// runOverlayApp 启动透明贴附的 Overlay 附进程。
// 职责：用同一个 exe 和同一份前端资源渲染独立悬浮层。
// 使用：主进程通过 DysonSphereBox.exe --overlay 启动。
// 边界：只绑定 OverlayApp，不绑定主程序业务服务，避免附进程初始化数据库和恢复任务。
func runOverlayApp() {
	app := NewOverlayApp()
	err := wails.Run(&options.App{
		Title:            "DysonSphereBoxOverlay",
		Width:            900,
		Height:           520,
		Frameless:        true,
		AlwaysOnTop:      false,
		StartHidden:      true,
		BackgroundColour: options.NewRGBA(0, 0, 0, 0),
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			DisablePinchZoom:     true,
			IsZoomControlEnabled: false,
			DisableWindowIcon:    true,
		},
		OnStartup:  app.startup,
		OnDomReady: app.domReady,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
			&overlayService.OverlayService{},
			overlayService.NewOverlayRuntimeService(app.controller),
		},
	})

	if err != nil {
		println("Overlay Error:", err.Error())
	}
}

// isOverlayMode 判断当前进程是否为 Overlay 附进程模式。
// 使用：main 根据启动参数分流主应用和悬浮窗。
// 边界：只识别精确参数 --overlay，避免误判普通参数。
func isOverlayMode(args []string) bool {
	for _, arg := range args[1:] {
		if arg == "--overlay" {
			return true
		}
	}
	return false
}
