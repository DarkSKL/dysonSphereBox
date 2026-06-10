package main

import (
	"context"
	"fmt"

	"DysonSphereBox/core"
	modDependenceHelper "DysonSphereBox/helper/modDependence"
	systemHelper "DysonSphereBox/helper/system"
	"DysonSphereBox/utils"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	core.GLOBAL.RuntimeContext = ctx

	if err := core.InitDatabase(databaseTemplate); err != nil {
		panic(fmt.Errorf("init database: %w", err))
	}
	if err := modDependenceHelper.RepairLegacyScopedBepInExFiles(); err != nil {
		fmt.Printf("repair legacy scoped mod files: %v\n", err)
	}
	if err := modDependenceHelper.RecoverInterruptedRemotePackageInstalls(); err != nil {
		fmt.Printf("recover interrupted mod installs: %v\n", err)
	}
}

func (a *App) shutdown(ctx context.Context) {
	if err := core.CloseDatabase(); err != nil {
		fmt.Printf("close database: %v\n", err)
	}
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

func (a *App) ReadBoxName(id string) (string, error) {
	if id == "" {
		return "", fmt.Errorf("id cannot be empty")
	}
	fmt.Printf("Reading box name: %s\n", id)

	return "Box " + id, nil
}

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

	normalizedDirPath := utils.NormalizeDysonSphereDir(dirPath)
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
