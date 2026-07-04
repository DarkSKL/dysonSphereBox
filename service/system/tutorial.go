package system

// 文件职责：提供首次使用引导教程进度查询、完成、跳过和重置的 Wails 服务。
// 使用：前端 Driver.js 教程模块在页面自动启动前查询状态，并在用户完成或关闭教程时写入数据库。
// 边界：不定义前端教程步骤、不操作 DOM、不记录具体页面 selector；数据库未初始化或参数非法时返回错误。

import (
	"errors"
	"fmt"
	"strings"
	"time"

	systemConstant "DysonSphereBox/constant/system"
	"DysonSphereBox/core"
	systemModel "DysonSphereBox/model/system"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TutorialService 提供前端可调用的教程进度持久化能力。
// 职责：查询、写入和重置教程阅读状态。
// 使用：全局框架教程与各主页面教程通过 Wails 绑定调用。
// 边界：只管理数据库状态，不关心 Driver.js 实例生命周期或页面元素是否存在。
type TutorialService struct{}

// TutorialProgressDTO 是前端读取教程状态时使用的数据传输对象。
// 职责：屏蔽数据库时间字段和内部主键，只暴露页面 key、版本、状态与便捷布尔值。
// 使用：前端 usePageTutorial 根据 completed/skipped 判断是否自动展示。
// 边界：没有记录时返回空 status 且两个布尔值为 false。
type TutorialProgressDTO struct {
	// PageKey 是教程所属页面或全局框架 key。
	// 职责：帮助前端确认响应对应的教程组。
	// 使用：前端调试或未来展示教程状态列表时读取。
	// 边界：不等同于路由路径。
	PageKey string `json:"pageKey"`
	// TutorialVersion 是教程版本。
	// 职责：帮助前端确认响应对应当前 registry 版本。
	// 使用：前端调试或未来展示教程状态列表时读取。
	// 边界：不由后端推断最新版本。
	TutorialVersion int `json:"tutorialVersion"`
	// Status 是持久化状态。
	// 职责：保存 completed、skipped 或空字符串。
	// 使用：前端可展示原始状态；业务判断优先使用 Completed 与 Skipped。
	// 边界：没有记录时为空。
	Status string `json:"status"`
	// Completed 表示当前页面当前版本教程已完成。
	// 职责：提供前端自动启动判断的便捷字段。
	// 使用：为 true 时前端不再自动展示同版本教程。
	// 边界：只代表当前传入版本，不代表未来版本。
	Completed bool `json:"completed"`
	// Skipped 表示当前页面当前版本教程已跳过。
	// 职责：提供前端自动启动判断的便捷字段。
	// 使用：为 true 时前端不再自动展示同版本教程。
	// 边界：用户可手动重新查看并覆盖状态。
	Skipped bool `json:"skipped"`
}

// GetTutorialProgress 查询指定页面与版本的教程进度。
// 职责：返回教程是否已完成或跳过。
// 使用：前端页面挂载或激活后、自动启动 Driver.js 前调用。
// 边界：没有记录不视为错误，返回空状态；数据库未初始化或参数非法时返回错误。
func (service *TutorialService) GetTutorialProgress(pageKey string, tutorialVersion int) (TutorialProgressDTO, error) {
	normalizedPageKey, err := service.validateTutorialIdentity(pageKey, tutorialVersion)
	if err != nil {
		return TutorialProgressDTO{}, err
	}
	if core.GLOBAL.DB == nil {
		return TutorialProgressDTO{}, errors.New("database is not initialized")
	}

	var progress systemModel.TutorialProgress
	err = core.GLOBAL.DB.
		Where("page_key = ? AND tutorial_version = ?", normalizedPageKey, tutorialVersion).
		First(&progress).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return TutorialProgressDTO{
			PageKey:         normalizedPageKey,
			TutorialVersion: tutorialVersion,
			Status:          "",
			Completed:       false,
			Skipped:         false,
		}, nil
	}
	if err != nil {
		return TutorialProgressDTO{}, fmt.Errorf("get tutorial progress: %w", err)
	}

	return service.toTutorialProgressDTO(progress), nil
}

// MarkTutorialCompleted 标记指定页面与版本的教程已完成。
// 职责：以 upsert 语义写入 completed 状态并记录完成时间。
// 使用：Driver.js 最后一步完成按钮回调调用。
// 边界：会清空 skipped_at，表示当前版本最终状态以完成为准；参数非法或数据库未初始化时返回错误。
func (service *TutorialService) MarkTutorialCompleted(pageKey string, tutorialVersion int) (TutorialProgressDTO, error) {
	return service.upsertTutorialProgress(pageKey, tutorialVersion, systemConstant.TutorialStatusCompleted)
}

// SkipTutorial 标记指定页面与版本的教程已跳过。
// 职责：以 upsert 语义写入 skipped 状态并记录跳过时间。
// 使用：Driver.js 关闭按钮回调调用。
// 边界：会清空 completed_at，表示当前版本最终状态以跳过为准；参数非法或数据库未初始化时返回错误。
func (service *TutorialService) SkipTutorial(pageKey string, tutorialVersion int) (TutorialProgressDTO, error) {
	return service.upsertTutorialProgress(pageKey, tutorialVersion, systemConstant.TutorialStatusSkipped)
}

// ResetTutorialProgress 重置教程进度。
// 职责：删除指定页面的教程记录，或在 pageKey 为空时删除全部教程记录。
// 使用：后续调试入口或设置页重置教程状态时调用。
// 边界：只删除教程进度表，不影响系统配置、日志或前端 registry；数据库未初始化时返回错误。
func (service *TutorialService) ResetTutorialProgress(pageKey string) (int64, error) {
	if core.GLOBAL.DB == nil {
		return 0, errors.New("database is not initialized")
	}

	normalizedPageKey := strings.TrimSpace(pageKey)
	query := core.GLOBAL.DB.Model(&systemModel.TutorialProgress{})
	if normalizedPageKey != "" {
		query = query.Where("page_key = ?", normalizedPageKey)
	} else {
		query = query.Session(&gorm.Session{AllowGlobalUpdate: true})
	}

	result := query.Delete(&systemModel.TutorialProgress{})
	if result.Error != nil {
		return 0, fmt.Errorf("reset tutorial progress: %w", result.Error)
	}

	return result.RowsAffected, nil
}

// validateTutorialIdentity 校验教程页面 key 与版本号。
// 职责：统一服务入口对 pageKey 和 tutorialVersion 的基础约束。
// 使用：查询和写入教程状态前调用。
// 边界：只做通用非空与正数校验，不维护前端页面 key 白名单。
func (service *TutorialService) validateTutorialIdentity(pageKey string, tutorialVersion int) (string, error) {
	normalizedPageKey := strings.TrimSpace(pageKey)
	if normalizedPageKey == "" {
		return "", errors.New("pageKey cannot be empty")
	}
	if tutorialVersion <= 0 {
		return "", errors.New("tutorialVersion must be greater than 0")
	}

	return normalizedPageKey, nil
}

// upsertTutorialProgress 写入教程进度状态。
// 职责：按 page_key 与 tutorial_version 唯一键创建或更新完成/跳过记录。
// 使用：MarkTutorialCompleted 与 SkipTutorial 共享该流程。
// 边界：只接受 completed 与 skipped；写入后重新读取当前记录并返回 DTO。
// 逻辑流：1. 校验教程身份和状态 => 2. 生成对应时间字段 => 3. 通过唯一键 upsert => 4. 重新查询并返回 DTO。
func (service *TutorialService) upsertTutorialProgress(pageKey string, tutorialVersion int, status string) (TutorialProgressDTO, error) {
	normalizedPageKey, err := service.validateTutorialIdentity(pageKey, tutorialVersion)
	if err != nil {
		return TutorialProgressDTO{}, err
	}
	if status != systemConstant.TutorialStatusCompleted && status != systemConstant.TutorialStatusSkipped {
		return TutorialProgressDTO{}, errors.New("tutorial status is invalid")
	}
	if core.GLOBAL.DB == nil {
		return TutorialProgressDTO{}, errors.New("database is not initialized")
	}

	now := time.Now()
	var completedAt *time.Time
	var skippedAt *time.Time
	if status == systemConstant.TutorialStatusCompleted {
		completedAt = &now
	} else {
		skippedAt = &now
	}

	progress := systemModel.TutorialProgress{
		PageKey:         normalizedPageKey,
		TutorialVersion: tutorialVersion,
		Status:          status,
		CompletedAt:     completedAt,
		SkippedAt:       skippedAt,
	}
	err = core.GLOBAL.DB.
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "page_key"},
				{Name: "tutorial_version"},
			},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"status":       status,
				"completed_at": completedAt,
				"skipped_at":   skippedAt,
				"updated_at":   now,
			}),
		}).
		Create(&progress).
		Error
	if err != nil {
		return TutorialProgressDTO{}, fmt.Errorf("save tutorial progress: %w", err)
	}

	return service.GetTutorialProgress(normalizedPageKey, tutorialVersion)
}

// toTutorialProgressDTO 转换教程进度数据库模型。
// 职责：把 GORM 模型转换为前端稳定 DTO。
// 使用：查询和写入后返回响应时调用。
// 边界：不修改原模型，不推断未知状态。
func (service *TutorialService) toTutorialProgressDTO(progress systemModel.TutorialProgress) TutorialProgressDTO {
	return TutorialProgressDTO{
		PageKey:         progress.PageKey,
		TutorialVersion: progress.TutorialVersion,
		Status:          progress.Status,
		Completed:       progress.Status == systemConstant.TutorialStatusCompleted,
		Skipped:         progress.Status == systemConstant.TutorialStatusSkipped,
	}
}
