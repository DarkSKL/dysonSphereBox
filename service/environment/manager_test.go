package environment

// 文件职责：验证环境管理服务的关键状态写入规则。
// 使用：由 go test ./service/environment 执行，覆盖导入和扫描后资源状态不被误覆盖的回归场景。
// 边界：只使用临时数据库和临时资源目录，不访问真实游戏目录、不绑定 Wails 运行时。

import (
	"os"
	"path/filepath"
	"testing"

	"DysonSphereBox/core"
	environmentHelper "DysonSphereBox/helper/environment"
	environmentModel "DysonSphereBox/model/environment"
	systemModel "DysonSphereBox/model/system"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// TestUpsertScannedModItemsPreservesExistingState 验证导入后重新扫描不会把已有模组状态重置为默认。
// 职责：复现“导入新 Mod 后所有 Mod 回到默认”的回归条件。
// 使用：服务层修改 upsertScannedModItems 或导入流程时运行。
// 边界：只断言已有状态保持和新资源默认入库，不覆盖文件复制、启用或禁用流程。
func TestUpsertScannedModItemsPreservesExistingState(t *testing.T) {
	previousDB := core.GLOBAL.DB
	t.Cleanup(func() {
		core.GLOBAL.DB = previousDB
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := db.AutoMigrate(&environmentModel.EnvItem{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	core.GLOBAL.DB = db

	libraryRoot := t.TempDir()
	if err := writeTestModManifest(libraryRoot, "EnabledMod", "Enabled Mod"); err != nil {
		t.Fatalf("write enabled mod manifest: %v", err)
	}

	mods, err := environmentHelper.ScanMods(libraryRoot)
	if err != nil {
		t.Fatalf("scan initial mods: %v", err)
	}
	if len(mods) != 1 {
		t.Fatalf("expected 1 initial mod, got %d", len(mods))
	}
	enabledItem := modResourceToEnvItem(mods[0], environmentHelper.EnvironmentStateEnabled, "library")
	if err := db.Create(&enabledItem).Error; err != nil {
		t.Fatalf("create enabled item: %v", err)
	}

	if err := writeTestModManifest(libraryRoot, "ImportedMod", "Imported Mod"); err != nil {
		t.Fatalf("write imported mod manifest: %v", err)
	}

	service := EnvironmentManagerService{}
	if err := service.upsertScannedModItems(libraryRoot, environmentHelper.EnvironmentStateDefault, "import_dir"); err != nil {
		t.Fatalf("upsert scanned mod items: %v", err)
	}

	var preserved environmentModel.EnvItem
	if err := db.Where("id = ? AND kind = ?", enabledItem.ID, "mod").First(&preserved).Error; err != nil {
		t.Fatalf("query preserved item: %v", err)
	}
	if preserved.State != environmentHelper.EnvironmentStateEnabled {
		t.Fatalf("expected existing state %q, got %q", environmentHelper.EnvironmentStateEnabled, preserved.State)
	}

	var items []environmentModel.EnvItem
	if err := db.Where("kind = ?", "mod").Find(&items).Error; err != nil {
		t.Fatalf("query all mod items: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 mod items, got %d", len(items))
	}
	defaultCount := 0
	for _, item := range items {
		if item.ID != enabledItem.ID && item.State == environmentHelper.EnvironmentStateDefault {
			defaultCount++
		}
	}
	if defaultCount != 1 {
		t.Fatalf("expected exactly 1 newly imported default mod, got %d", defaultCount)
	}
}

// TestRenameEnabledBlueprintMigratesRealFile 验证已启用蓝图可直接重命名并同步真实蓝图目录。
// 职责：覆盖用户要求的“无需先禁用，重命名会同步真实资源目录”行为。
// 使用：修改 RenameBlueprintItem 或蓝图迁移流程时运行。
// 边界：使用测试进程旁 appData 和临时真实蓝图目录，不访问用户真实文档目录。
func TestRenameEnabledBlueprintMigratesRealFile(t *testing.T) {
	previousDB := core.GLOBAL.DB
	previousConfigure := core.GLOBAL.Configure
	t.Cleanup(func() {
		core.GLOBAL.DB = previousDB
		core.GLOBAL.Configure = previousConfigure
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := db.AutoMigrate(&environmentModel.EnvItem{}, &environmentModel.EnvSyncedFile{}, &environmentModel.EnvOperation{}, &systemModel.Config{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	core.GLOBAL.DB = db
	core.GLOBAL.Configure = core.InitConfigure()

	executablePath, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve executable path: %v", err)
	}
	appDataRoot := filepath.Join(filepath.Dir(executablePath), "appData")
	libraryRoot := filepath.Join(appDataRoot, "blueprints")
	t.Cleanup(func() {
		_ = os.RemoveAll(appDataRoot)
	})
	if err := os.RemoveAll(appDataRoot); err != nil {
		t.Fatalf("cleanup appData root: %v", err)
	}
	if err := os.MkdirAll(libraryRoot, 0o755); err != nil {
		t.Fatalf("create blueprint library root: %v", err)
	}

	realRoot := t.TempDir()
	if err := core.InitConfigUnit(environmentHelper.ConfigGroupGame, environmentHelper.ConfigKeyBlueprintDirPath).Set(realRoot); err != nil {
		t.Fatalf("set blueprint root config: %v", err)
	}

	oldLibraryPath := filepath.Join(libraryRoot, "旧蓝图.txt")
	oldRealPath := filepath.Join(realRoot, "旧蓝图.txt")
	content := []byte("BLUEPRINT:test")
	if err := os.WriteFile(oldLibraryPath, content, 0o644); err != nil {
		t.Fatalf("write old library blueprint: %v", err)
	}
	if err := os.WriteFile(oldRealPath, content, 0o644); err != nil {
		t.Fatalf("write old real blueprint: %v", err)
	}

	resources, err := environmentHelper.ScanBlueprints(libraryRoot)
	if err != nil {
		t.Fatalf("scan blueprint library: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 blueprint resource, got %d", len(resources))
	}
	oldResource := resources[0]
	sha256, size, err := environmentHelper.HashFile(oldRealPath)
	if err != nil {
		t.Fatalf("hash old real blueprint: %v", err)
	}
	oldItem := blueprintResourceToEnvItem(oldResource, environmentHelper.EnvironmentStateEnabled, "library")
	if err := db.Create(&oldItem).Error; err != nil {
		t.Fatalf("create old blueprint item: %v", err)
	}
	oldRecord := environmentModel.EnvSyncedFile{
		ItemID:         oldResource.ID,
		Kind:           "blueprint",
		SourcePath:     oldLibraryPath,
		TargetPath:     oldRealPath,
		TargetRootPath: realRoot,
		RelativePath:   "旧蓝图.txt",
		SHA256:         sha256,
		FileSize:       size,
	}
	if err := db.Create(&oldRecord).Error; err != nil {
		t.Fatalf("create old synced record: %v", err)
	}

	service := EnvironmentManagerService{}
	result, err := service.RenameBlueprintItem(oldResource.ID, "新蓝图")
	if err != nil {
		t.Fatalf("rename enabled blueprint: %v", err)
	}
	if result.BlueprintPage == nil {
		t.Fatalf("expected refreshed blueprint page")
	}

	newLibraryPath := filepath.Join(libraryRoot, "新蓝图.txt")
	newRealPath := filepath.Join(realRoot, "新蓝图.txt")
	assertFileExists(t, newLibraryPath)
	assertFileExists(t, newRealPath)
	assertFileMissing(t, oldLibraryPath)
	assertFileMissing(t, oldRealPath)

	newResources, err := environmentHelper.ScanBlueprints(libraryRoot)
	if err != nil {
		t.Fatalf("scan renamed blueprint library: %v", err)
	}
	if len(newResources) != 1 {
		t.Fatalf("expected 1 renamed blueprint resource, got %d", len(newResources))
	}
	newResource := newResources[0]
	var newItem environmentModel.EnvItem
	if err := db.Where("id = ? AND kind = ?", newResource.ID, "blueprint").First(&newItem).Error; err != nil {
		t.Fatalf("query renamed item: %v", err)
	}
	if newItem.State != environmentHelper.EnvironmentStateEnabled {
		t.Fatalf("expected renamed item state %q, got %q", environmentHelper.EnvironmentStateEnabled, newItem.State)
	}
	var oldItemCount int64
	if err := db.Model(&environmentModel.EnvItem{}).Where("id = ? AND kind = ?", oldResource.ID, "blueprint").Count(&oldItemCount).Error; err != nil {
		t.Fatalf("count old item: %v", err)
	}
	if oldItemCount != 0 {
		t.Fatalf("expected old item removed, got %d", oldItemCount)
	}
	var records []environmentModel.EnvSyncedFile
	if err := db.Where("kind = ?", "blueprint").Find(&records).Error; err != nil {
		t.Fatalf("query synced records: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 synced record, got %d", len(records))
	}
	if records[0].ItemID != newResource.ID || records[0].RelativePath != "新蓝图.txt" || !sameCleanPath(records[0].TargetPath, newRealPath) || !sameCleanPath(records[0].SourcePath, newLibraryPath) {
		t.Fatalf("synced record was not migrated correctly: %+v", records[0])
	}
}

// TestSyncExistingModsImportsMissingRealMods 验证每次同步都会把真实 Mod 目录中的缺失项纳管进客户端资源库。
// 职责：覆盖“真实目录 2 个、虚拟目录 1 个时，同步后虚拟目录补齐到 2 个”的核心语义。
// 使用：修改 RunInitialSync、syncExistingMods 或同步菜单语义时运行。
// 边界：只验证目录型 Mod 纳管、状态记录和同步证据，不覆盖前端 toast 文案。
func TestSyncExistingModsImportsMissingRealMods(t *testing.T) {
	db := setupEnvironmentServiceTest(t)
	appDataRoot, libraryRoot := prepareTestAppDataRoot(t, "plugins")

	gameRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(gameRoot, "DSPGAME.exe"), []byte("test"), 0o644); err != nil {
		t.Fatalf("write game exe marker: %v", err)
	}
	realModRoot := filepath.Join(gameRoot, "BepInEx", "plugins")
	if err := writeTestModManifest(realModRoot, "ExistingMod", "Existing Mod"); err != nil {
		t.Fatalf("write existing real mod: %v", err)
	}
	if err := writeTestModManifest(realModRoot, "MissingMod", "Missing Mod"); err != nil {
		t.Fatalf("write missing real mod: %v", err)
	}
	if err := writeTestModManifest(libraryRoot, "ExistingMod", "Existing Mod"); err != nil {
		t.Fatalf("write existing library mod: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(appDataRoot)
	})
	if err := core.GLOBAL.Configure.GameStartFilePath.Set(gameRoot); err != nil {
		t.Fatalf("set game root config: %v", err)
	}

	service := EnvironmentManagerService{}
	synced, failures := service.syncExistingMods(environmentHelper.Resolver{})
	if len(failures) != 0 {
		t.Fatalf("expected no failures, got %v", failures)
	}
	if synced != 1 {
		t.Fatalf("expected 1 newly managed mod, got %d", synced)
	}

	assertFileExists(t, filepath.Join(libraryRoot, "ExistingMod", "manifest.json"))
	assertFileExists(t, filepath.Join(libraryRoot, "MissingMod", "manifest.json"))
	var items []environmentModel.EnvItem
	if err := db.Where("kind = ?", "mod").Find(&items).Error; err != nil {
		t.Fatalf("query mod items: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 mod items, got %d", len(items))
	}
	for _, item := range items {
		if item.State != environmentHelper.EnvironmentStateEnabled {
			t.Fatalf("expected mod %s enabled, got %s", item.Name, item.State)
		}
	}
	var recordCount int64
	if err := db.Model(&environmentModel.EnvSyncedFile{}).Where("kind = ?", "mod").Count(&recordCount).Error; err != nil {
		t.Fatalf("count mod synced records: %v", err)
	}
	if recordCount == 0 {
		t.Fatalf("expected mod synced records")
	}
}

// TestSyncExistingBlueprintsImportsMissingRealBlueprints 验证每次同步都会把真实蓝图目录中的缺失文件纳管进客户端资源库。
// 职责：覆盖“真实蓝图 2 个、虚拟蓝图 1 个时，同步后虚拟目录补齐到 2 个”的核心语义。
// 使用：修改 RunInitialSync、syncExistingBlueprints 或同步菜单语义时运行。
// 边界：只验证 .txt/.bp 蓝图文件纳管、状态记录和同步证据，不覆盖文件夹移动迁移。
func TestSyncExistingBlueprintsImportsMissingRealBlueprints(t *testing.T) {
	db := setupEnvironmentServiceTest(t)
	appDataRoot, libraryRoot := prepareTestAppDataRoot(t, "blueprints")
	t.Cleanup(func() {
		_ = os.RemoveAll(appDataRoot)
	})

	realRoot := t.TempDir()
	if err := core.InitConfigUnit(environmentHelper.ConfigGroupGame, environmentHelper.ConfigKeyBlueprintDirPath).Set(realRoot); err != nil {
		t.Fatalf("set blueprint root config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(realRoot, "Existing.txt"), []byte("BLUEPRINT:existing"), 0o644); err != nil {
		t.Fatalf("write existing real blueprint: %v", err)
	}
	if err := os.WriteFile(filepath.Join(realRoot, "Missing.txt"), []byte("BLUEPRINT:missing"), 0o644); err != nil {
		t.Fatalf("write missing real blueprint: %v", err)
	}
	if err := os.WriteFile(filepath.Join(libraryRoot, "Existing.txt"), []byte("BLUEPRINT:existing"), 0o644); err != nil {
		t.Fatalf("write existing library blueprint: %v", err)
	}

	service := EnvironmentManagerService{}
	synced, failures := service.syncExistingBlueprints(environmentHelper.Resolver{})
	if len(failures) != 0 {
		t.Fatalf("expected no failures, got %v", failures)
	}
	if synced != 1 {
		t.Fatalf("expected 1 newly managed blueprint, got %d", synced)
	}

	assertFileExists(t, filepath.Join(libraryRoot, "Existing.txt"))
	assertFileExists(t, filepath.Join(libraryRoot, "Missing.txt"))
	var items []environmentModel.EnvItem
	if err := db.Where("kind = ?", "blueprint").Find(&items).Error; err != nil {
		t.Fatalf("query blueprint items: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 blueprint items, got %d", len(items))
	}
	for _, item := range items {
		if item.State != environmentHelper.EnvironmentStateEnabled {
			t.Fatalf("expected blueprint %s enabled, got %s", item.Name, item.State)
		}
	}
	var recordCount int64
	if err := db.Model(&environmentModel.EnvSyncedFile{}).Where("kind = ?", "blueprint").Count(&recordCount).Error; err != nil {
		t.Fatalf("count blueprint synced records: %v", err)
	}
	if recordCount != 2 {
		t.Fatalf("expected 2 blueprint synced records, got %d", recordCount)
	}
}

// writeTestModManifest 写入测试用目录型 Mod 清单。
// 职责：构造 ScanMods 能识别的最小 manifest.json。
// 使用：服务层回归测试创建临时 Mod 资源时调用。
// 边界：只写入测试临时目录，不模拟 Thunderstore 完整压缩包结构。
func writeTestModManifest(libraryRoot string, dirName string, displayName string) error {
	modDir := filepath.Join(libraryRoot, dirName)
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		return err
	}
	content := `{"name":"` + displayName + `","version_number":"1.0.0","author":"Test","description":"Test mod"}`
	return os.WriteFile(filepath.Join(modDir, "manifest.json"), []byte(content), 0o644)
}

// setupEnvironmentServiceTest 创建环境服务测试用数据库和配置上下文。
// 职责：隔离 core.GLOBAL.DB 与 Configure，避免服务测试读写用户真实数据库。
// 使用：需要调用依赖配置或数据库的环境服务方法时调用。
// 边界：只负责内存数据库和全局恢复，不创建 appData 资源目录。
func setupEnvironmentServiceTest(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := core.GLOBAL.DB
	previousConfigure := core.GLOBAL.Configure
	t.Cleanup(func() {
		core.GLOBAL.DB = previousDB
		core.GLOBAL.Configure = previousConfigure
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := db.AutoMigrate(&environmentModel.EnvItem{}, &environmentModel.EnvSyncedFile{}, &environmentModel.EnvOperation{}, &systemModel.Config{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	core.GLOBAL.DB = db
	core.GLOBAL.Configure = core.InitConfigure()
	return db
}

// prepareTestAppDataRoot 清理并创建测试进程旁的 appData 资源目录。
// 职责：复用 Resolver.AppDataRoot 的真实解析规则，让服务方法无需注入测试路径。
// 使用：同步纳管和蓝图迁移测试准备客户端资源库时调用。
// 边界：只清理测试进程目录旁 appData，不访问用户运行目录。
func prepareTestAppDataRoot(t *testing.T, resourcePart string) (string, string) {
	t.Helper()
	executablePath, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve executable path: %v", err)
	}
	appDataRoot := filepath.Join(filepath.Dir(executablePath), "appData")
	libraryRoot := filepath.Join(appDataRoot, resourcePart)
	if err := os.RemoveAll(appDataRoot); err != nil {
		t.Fatalf("cleanup appData root: %v", err)
	}
	if err := os.MkdirAll(libraryRoot, 0o755); err != nil {
		t.Fatalf("create library root: %v", err)
	}
	return appDataRoot, libraryRoot
}

// assertFileExists 断言路径存在且是普通文件。
// 职责：让文件迁移测试的失败原因更直接。
// 使用：蓝图重命名迁移测试调用。
// 边界：目录或缺失路径均视为失败。
func assertFileExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected file exists %s: %v", path, err)
	}
	if info.IsDir() {
		t.Fatalf("expected file but got directory: %s", path)
	}
}

// assertFileMissing 断言路径不存在。
// 职责：验证迁移后旧客户端镜像和旧真实蓝图文件已清理。
// 使用：蓝图重命名迁移测试调用。
// 边界：访问异常且不是不存在时视为失败。
func assertFileMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected file missing: %s", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat file %s: %v", path, err)
	}
}
