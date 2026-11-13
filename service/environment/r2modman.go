package environment

// 文件职责：向资源页提供 r2modman Profile 检索、选择和直接管理接口。
// 使用：一键同步与 Profile 模组分区经 Wails 调用。
// 边界：只修改用户明确选择的 Profile，不向游戏目录或盒子资源库复制其模组。

import (
	environmentConstant "DysonSphereBox/constant/environment"
	"DysonSphereBox/core"
	"DysonSphereBox/helper/environment/r2modman"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gorm.io/gorm"
)

// R2ModmanService 职责：编排直接托管的校验、配置和审计；使用：资源页经生成绑定调用；边界：注册时不访问数据库，互斥选择与写入避免异步误写。
type R2ModmanService struct {
	// mu 职责：串行化托管目标切换及写入；使用：每次读取或修改目标；边界：不代替外部 R2 清单并发检测。
	mu sync.Mutex
}

// ListProfiles 职责：列出盒子已同步的 Profile；使用：工具栏下拉；边界：只读数据库，不扫描 R2 文件目录。
func (service *R2ModmanService) ListProfiles() ([]r2modman.Profile, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	return r2modman.ListCachedProfiles(core.GLOBAL.DB)
}

// GetProfile 职责：读取盒子持久化的所选 Profile 快照；使用：资源页加载和激活；边界：绝不访问 R2 目录，无快照时等待用户显式同步。
func (service *R2ModmanService) GetProfile() (r2modman.ProfileState, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	path, err := core.InitConfigUnit(environmentConstant.R2ConfigGroup, environmentConstant.R2ProfilePathKey).Get()
	if err != nil {
		return r2modman.ProfileState{}, err
	}
	if path == "" {
		return r2modman.ProfileState{Mods: []r2modman.Mod{}}, nil
	}
	state, found, err := r2modman.LoadCachedProfile(core.GLOBAL.DB, path)
	if err != nil || found {
		return state, err
	}
	return r2modman.ProfileState{Mods: []r2modman.Mod{}}, nil
}

// ManageProfile 职责：持久化下拉选择的 Profile；使用：工具栏直接切换或手动定位；边界：已同步 Profile 只读缓存，新目录先校验同步再保存选择，不复制模组。
func (service *R2ModmanService) ManageProfile(path string) (state r2modman.ProfileState, err error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	defer func() { r2modman.RecordOperation("manage", path, "", err) }()
	var found bool
	state, found, err = r2modman.LoadCachedProfile(core.GLOBAL.DB, path)
	if err != nil {
		return state, err
	}
	if !found {
		state, err = service.readAndCacheProfile(path)
	}
	if err != nil {
		return state, err
	}
	err = core.InitConfigUnit(environmentConstant.R2ConfigGroup, environmentConstant.R2ProfilePathKey).Set(state.Profile.Path)
	return state, err
}

// UnmanageProfile 职责：清除盒子的托管选择；使用：资源页解除托管；边界：不删除 Profile、不改变 R2 模组状态。
func (service *R2ModmanService) UnmanageProfile() (err error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	config := core.InitConfigUnit(environmentConstant.R2ConfigGroup, environmentConstant.R2ProfilePathKey)
	path, err := config.Get()
	if err != nil {
		return err
	}
	defer func() { r2modman.RecordOperation("unmanage", path, "", err) }()
	return config.Set("")
}

// SetModEnabled 职责：启用模组或按确认范围联动禁用；使用：资源卡片切换和依赖确认框；边界：拒绝过期 Profile、框架、确认范围变化和无明确文件归属的包。
func (service *R2ModmanService) SetModEnabled(profilePath, name string, enabled bool, confirmedNames []string) (state r2modman.ProfileState, err error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	action := "disable"
	if enabled {
		action = "enable"
	}
	defer func() { r2modman.RecordOperation(action, profilePath, name, err) }()
	path, err := service.selectedProfile(profilePath)
	if err != nil {
		return state, err
	}
	err = r2modman.SetEnabledWithPlan(path, name, enabled, confirmedNames)
	if err != nil {
		return state, err
	}
	return service.readAndCacheProfile(path)
}

// RemoveMod 职责：移除所选 Profile 的一条模组及其明确归属目录；使用：资源页确认移除；边界：保留盒子备份，有依赖者或共享文件时拒绝。
func (service *R2ModmanService) RemoveMod(profilePath, name string) (state r2modman.ProfileState, err error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	defer func() { r2modman.RecordOperation("remove", profilePath, name, err) }()
	path, err := service.selectedProfile(profilePath)
	if err != nil {
		return state, err
	}
	if err = r2modman.Remove(path, name); err != nil {
		return state, err
	}
	return service.readAndCacheProfile(path)
}

// SyncProfiles 职责：显式检索 R2 Profile 并刷新盒子的持久快照；使用：用户点击一键同步；边界：只读取 R2 模组文件，失效档案保留锁定快照，不自动切换用户当前选择。
// 逻辑流：1. 读取已知 Profile 和所选目录 => 2. 检索默认与自定义数据根 => 3. 恢复盒子中断操作并扫描状态 => 4. 事务保存全部快照 => 5. 返回当前选择的快照。
func (service *R2ModmanService) SyncProfiles() (state r2modman.ProfileState, err error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	defer func() { r2modman.RecordOperation("sync", "", "", err) }()
	known, err := r2modman.ListCachedProfiles(core.GLOBAL.DB)
	if err != nil {
		return state, err
	}
	selected, err := core.InitConfigUnit(environmentConstant.R2ConfigGroup, environmentConstant.R2ProfilePathKey).Get()
	if err != nil {
		return state, err
	}
	paths := map[string]string{}
	roots := []string{}
	for _, profile := range known {
		paths[strings.ToLower(profile.Path)] = profile.Path
		roots = append(roots, filepath.Dir(filepath.Dir(filepath.Dir(profile.Path))))
	}
	if selected != "" {
		paths[strings.ToLower(selected)] = selected
		roots = append(roots, filepath.Dir(filepath.Dir(filepath.Dir(selected))))
	}
	discovered, err := r2modman.DiscoverProfiles(roots...)
	if err != nil {
		return state, err
	}
	for _, profile := range discovered {
		paths[strings.ToLower(profile.Path)] = profile.Path
	}
	snapshots := []r2modman.ProfileState{}
	for _, path := range paths {
		scanErr := r2modman.Recover(path)
		var snapshot r2modman.ProfileState
		if scanErr == nil {
			snapshot, scanErr = r2modman.ReadProfile(path)
		}
		if scanErr != nil {
			r2modman.RecordOperation("syncProfile", path, "", scanErr)
			cached, found, cacheErr := r2modman.LoadCachedProfile(core.GLOBAL.DB, path)
			if cacheErr != nil || !found {
				continue
			}
			for index := range cached.Mods {
				mod := &cached.Mods[index]
				mod.CanManage, mod.CanDisable, mod.CanRemove = false, false, false
				mod.Warning = "Profile 不可用，请重新同步或手动定位目录"
				mod.DisableReason, mod.RemoveReason = mod.Warning, mod.Warning
			}
			snapshot = cached
		}
		snapshots = append(snapshots, snapshot)
	}
	err = core.GLOBAL.DB.Transaction(func(tx *gorm.DB) error {
		for _, snapshot := range snapshots {
			if err := r2modman.SaveCachedProfile(tx, snapshot); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return state, err
	}
	if selected == "" {
		return r2modman.ProfileState{Mods: []r2modman.Mod{}}, nil
	}
	state, _, err = r2modman.LoadCachedProfile(core.GLOBAL.DB, selected)
	return state, err
}

// 职责：在显式同步或修改后保存实盘状态；使用：新 Profile 定位和模组操作完成后；边界：缓存保存失败会明确报错，页面普通读取不调用。
func (service *R2ModmanService) readAndCacheProfile(path string) (r2modman.ProfileState, error) {
	if err := r2modman.Recover(path); err != nil {
		return r2modman.ProfileState{}, err
	}
	state, err := r2modman.ReadProfile(path)
	if err != nil {
		return state, err
	}
	if err := r2modman.SaveCachedProfile(core.GLOBAL.DB, state); err != nil {
		return state, fmt.Errorf("保存 Profile 同步缓存失败，请重新同步: %w", err)
	}
	return state, nil
}

// 职责：验证请求仍对应当前选择；使用：所有 Profile 写入和打开目录；边界：前端旧请求不能影响新目标，路径重新规范化。
func (service *R2ModmanService) selectedProfile(requestPath string) (string, error) {
	selected, err := core.InitConfigUnit(environmentConstant.R2ConfigGroup, environmentConstant.R2ProfilePathKey).Get()
	if err != nil {
		return "", err
	}
	if selected == "" {
		return "", fmt.Errorf("尚未托管 r2modman Profile")
	}
	path, err := r2modman.ValidateProfile(requestPath)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(path, selected) {
		return "", fmt.Errorf("托管 Profile 已变化，请刷新列表后重试")
	}
	return path, nil
}

// OpenProfileDirectory 职责：打开所选 Profile 的真实目录；使用：资源页定位文件；边界：不接受任意命令或未托管路径。
func (service *R2ModmanService) OpenProfileDirectory(profilePath string) error {
	service.mu.Lock()
	defer service.mu.Unlock()
	path, err := service.selectedProfile(profilePath)
	if err != nil {
		return err
	}
	return exec.Command("explorer.exe", path).Start()
}

// SelectProfileDirectory 职责：提供自定义数据根的 Profile 目录选择器；使用：同步弹窗手动定位；边界：取消返回空路径，选择后仍需 ManageProfile 校验和确认。
func (service *R2ModmanService) SelectProfileDirectory() (string, error) {
	if core.GLOBAL.RuntimeContext == nil {
		return "", fmt.Errorf("桌面目录选择器尚未就绪")
	}
	return runtime.OpenDirectoryDialog(core.GLOBAL.RuntimeContext, runtime.OpenDialogOptions{Title: "选择戴森球计划的 r2modman Profile 目录"})
}
