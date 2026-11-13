package r2modman

// 文件职责：读写盒子数据库中的 Profile 快照，不访问 r2modman 文件目录。
// 使用：页面加载、缓存切换与显式同步后的保存。
// 边界：使用调用方提供的数据库或事务，不启动数据库，不把缓存当作文件操作授权。

import (
	profileModel "DysonSphereBox/model/environment/r2modman"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// 职责：对已规范路径生成稳定数据库键；使用：快照保存和读取；边界：纯字符串操作，不探测外部文件。
func profileCacheKey(path string) string { return strings.ToLower(filepath.Clean(path)) }

// SaveCachedProfile 职责：写入一次完整的同步快照；使用：同步或模组操作完成后；边界：要求有效 Profile，不执行文件读取，保留传入事务。
func SaveCachedProfile(db *gorm.DB, state ProfileState) error {
	if db == nil {
		return fmt.Errorf("database is not initialized")
	}
	if state.Profile == nil || state.Profile.Path == "" {
		return fmt.Errorf("Profile 快照缺少路径")
	}
	content, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return db.Save(&profileModel.ProfileSnapshot{PathKey: profileCacheKey(state.Profile.Path), Path: state.Profile.Path, Name: state.Profile.Name, SnapshotJSON: datatypes.JSON(content), SyncedAt: time.Now()}).Error
}

// LoadCachedProfile 职责：按路径读取持久快照；使用：进入资源页和切换已同步 Profile；边界：不存在返回 found=false，不访问 Profile 目录。
func LoadCachedProfile(db *gorm.DB, path string) (state ProfileState, found bool, err error) {
	if db == nil {
		return state, false, fmt.Errorf("database is not initialized")
	}
	var record profileModel.ProfileSnapshot
	result := db.Where("path_key = ?", profileCacheKey(path)).Limit(1).Find(&record)
	if result.Error != nil {
		return state, false, result.Error
	}
	if result.RowsAffected == 0 {
		return ProfileState{Mods: []Mod{}}, false, nil
	}
	if err := json.Unmarshal(record.SnapshotJSON, &state); err != nil {
		return state, false, fmt.Errorf("Profile 缓存损坏，请重新同步: %w", err)
	}
	if state.Profile == nil || state.Profile.Path != record.Path {
		return state, false, fmt.Errorf("Profile 缓存身份异常，请重新同步")
	}
	if state.Mods == nil {
		state.Mods = []Mod{}
	}
	// 旧版快照的单包依赖锁定在内存中升级为联动权限，无需扫描实盘或要求用户重建缓存。
	updateModPermissions(state.Mods)
	return state, true, nil
}

// ListCachedProfiles 职责：返回已同步的 Profile 选项；使用：工具栏下拉；边界：只查询盒子缓存，不自动发现或创建外部目录。
func ListCachedProfiles(db *gorm.DB) ([]Profile, error) {
	if db == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	var records []profileModel.ProfileSnapshot
	if err := db.Order("path_key").Find(&records).Error; err != nil {
		return nil, err
	}
	profiles := []Profile{}
	for _, record := range records {
		var state ProfileState
		if err := json.Unmarshal(record.SnapshotJSON, &state); err != nil || state.Profile == nil || state.Profile.Path != record.Path {
			// 列表保留可重新同步的目录身份，避免坏快照让同步入口也无法恢复。
			profiles = append(profiles, Profile{Name: record.Name, Path: record.Path})
			continue
		}
		profiles = append(profiles, *state.Profile)
	}
	return profiles, nil
}
