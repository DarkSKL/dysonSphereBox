package main

// 文件职责：维护 Wails 应用启动与服务绑定入口。
// 使用：由桌面应用启动流程调用，用于初始化后端并运行前端宿主。
// 边界：不实现具体业务流程，不定义数据库模型或前端组件。

import (
	"DysonSphereBox/constant/logging"
	"DysonSphereBox/core"
	"DysonSphereBox/core/diagnostics"
	"embed"
	"fmt"
	"log"

	"DysonSphereBox/service"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
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
	if err := diagnostics.Initialize(core.Version); err != nil {
		fmt.Println("初始化日志失败:", err)
	}
	defer diagnostics.Default.Close()
	defer diagnostics.CapturePanic("backend.main")
	diagnostics.Default.BeginSession()
	log.SetOutput(diagnostics.StandardWriter{})
	app := NewApp()
	services := append([]interface{}{app}, service.RegisterServices()...)

	err := wails.Run(&options.App{
		Logger:    diagnostics.RuntimeLogger{},
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
		diagnostics.Default.Record(logging.Entry{Action: "启动桌面宿主失败", Level: logging.Fatal, Source: "backend.main", Reason: "Wails Run 返回错误", Namespace: "system.lifecycle.hostFailed", Handler: logging.System, Data: map[string]interface{}{"error": diagnostics.ErrorData(err), "result": "failed"}})
	}
}
