package forum

// 文件职责：提供社区论坛页面唯一的 Wails 业务服务入口，编排浏览、发帖、回复、举报、文件选择与游戏资源导入。
// 使用：Vue 论坛页面通过生成的 ForumService 绑定调用，每个网络方法均携带前端当前账号会话键。
// 边界：本文件仅包含 ForumService 结构体及其方法；HTTP 请求、本地校验、媒体代理和审计由 helper/forum 承接，蓝图/存档/模组落盘复用 EnvironmentManagerService。

import (
	"context"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	api "DysonSphereBox/apiDefinition/forum"
	"DysonSphereBox/core"
	"DysonSphereBox/core/request"
	forumHelper "DysonSphereBox/helper/forum"
	"DysonSphereBox/service/environment"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ForumService 暴露社区论坛相关的 Wails 业务接口。
// 职责：编排论坛分类、帖子列表/详情、回复、发帖、举报、资源可用性复核与本地环境导入。
// 使用：由 service/forum/enter.go 注册给 Wails，供社区论坛前端服务调用。
// 边界：不缓存账号令牌或远端帖子数据；鉴权、可见性与配额以应用服务器裁定为准。
type ForumService struct{}

// callerContext 返回桌面应用生命周期上下文。
// 职责：为论坛请求提供应用关闭时的取消信号。
// 使用：ForumService 内部网络调用方法复用。
// 边界：单元测试等无运行时场景下回退为 context.Background()。
func (service *ForumService) callerContext() context.Context {
	if ctx := core.GLOBAL.RuntimeContext; ctx != nil {
		return ctx
	}
	return context.Background()
}

// Categories 查询服务端当前启用的论坛分类列表。
// 职责：读取动态分类供列表筛选和发帖分类选择使用。
// 使用：论坛页面加载、刷新或账号切换后调用。
// 边界：失效会话拒绝请求；返回非 nil 切片。
func (service *ForumService) Categories(input api.SessionRequest) ([]api.Category, error) {
	items, err := forumHelper.QueryCategories(service.callerContext(), input.SessionKey, input.Force)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []api.Category{}
	}
	return items, nil
}

// Posts 分页查询可见帖子列表，支持按分类筛选。
// 职责：调用 GET /client/forum/posts 并返回规范化分页结果。
// 使用：论坛首页列表浏览、切换分类和翻页时调用。
// 边界：CategoryID 为 0 时查询全部分类；每页最多 100 条。
func (service *ForumService) Posts(input api.PostListRequest) (api.PostPage, error) {
	page, err := forumHelper.QueryPosts(service.callerContext(), input)
	if err != nil {
		return api.PostPage{}, err
	}
	if page.List == nil {
		page.List = []api.PostSummary{}
	}
	return page, nil
}

// PostDetail 查询单篇可见帖子的正文、块结构与可见资源列表。
// 职责：调用 GET /client/forum/posts/:id 返回帖子完整阅读数据。
// 使用：用户点击帖子进入详情页或刷新当前帖子时调用。
// 边界：详情在当前会话短时复用，重新向服务端读取时隐藏或不存在的帖子返回 notFound。
func (service *ForumService) PostDetail(input api.PostDetailRequest) (api.PostDetail, error) {
	detail, err := forumHelper.QueryPostDetail(service.callerContext(), input)
	if err != nil {
		return api.PostDetail{}, err
	}
	if detail.Resources == nil {
		detail.Resources = []api.Resource{}
	}
	return detail, nil
}

// Replies 分页查询指定帖子下的可见回复列表。
// 职责：调用 GET /client/forum/posts/:id/replies 返回回复分页结果。
// 使用：帖子详情页加载回复、翻页或发表回复后刷新列表时调用。
// 边界：PostID 必须大于 0；每条回复自带本回复的可见图片资源列表。
func (service *ForumService) Replies(input api.ReplyListRequest) (api.ReplyPage, error) {
	page, err := forumHelper.QueryReplies(service.callerContext(), input)
	if err != nil {
		return api.ReplyPage{}, err
	}
	if page.List == nil {
		page.List = []api.Reply{}
	}
	return page, nil
}

// PublishPost 以 multipart/form-data 发布新帖子及绑定的图片、视频和游戏资源。
// 职责：校验草稿字段与本地待上传文件，首段写入 post JSON，随后流式上传各类文件。
// 使用：发帖编辑器点击发布或失败重试时调用；相同草稿重试应复用 RequestID。
// 边界：蓝图内联与文件互斥、模组市场与文件互斥；上传失败不删除本地文件，由页面保留草稿。
// 逻辑流：1. 校验标题、正文、分类与互斥资源 => 2. 逐个检查本地待上传文件 => 3. 流式 multipart 提交 => 4. 记录成功审计日志并返回帖子 ID。
func (service *ForumService) PublishPost(input api.PublishPostRequest) (api.IDResult, error) {
	requestID := strings.TrimSpace(input.RequestID)
	title := strings.TrimSpace(input.Title)
	bodyFormat := strings.TrimSpace(input.BodyFormat)
	if bodyFormat == "" {
		bodyFormat = "markdown"
	}
	if requestID == "" || input.CategoryID == 0 || title == "" || strings.TrimSpace(input.Body) == "" {
		return api.IDResult{}, forumHelper.InvalidInput("请填写分类、标题和正文后再发布")
	}
	if utf8.RuneCountInString(title) > 120 {
		return api.IDResult{}, forumHelper.InvalidInput("帖子标题不能超过 120 个字符")
	}
	if utf8.RuneCountInString(input.Body) > 10000 {
		return api.IDResult{}, forumHelper.InvalidInput("帖子正文不能超过 10,000 个字符")
	}
	if bodyFormat != "markdown" && bodyFormat != "plain" {
		return api.IDResult{}, forumHelper.InvalidInput("不支持的正文格式")
	}
	if input.Blueprint != nil && strings.TrimSpace(input.BlueprintFilePath) != "" {
		return api.IDResult{}, forumHelper.InvalidInput("不能同时提交内联蓝图编码和蓝图文件")
	}
	if input.Blueprint != nil {
		if strings.TrimSpace(input.Blueprint.Name) == "" || utf8.RuneCountInString(strings.TrimSpace(input.Blueprint.Name)) > 120 {
			return api.IDResult{}, forumHelper.InvalidInput("请填写不超过 120 字的蓝图名称")
		}
		if strings.TrimSpace(input.Blueprint.BlueprintCode) == "" || int64(len(input.Blueprint.BlueprintCode)) > forumHelper.MaxBlueprintBytes || strings.ContainsRune(input.Blueprint.BlueprintCode, 0) {
			return api.IDResult{}, forumHelper.InvalidInput("内联蓝图编码无效或超过 512 KiB 限制")
		}
	}
	if input.Mod != nil && strings.TrimSpace(input.ModFilePath) != "" {
		return api.IDResult{}, forumHelper.InvalidInput("不能同时提交模组市场引用和模组文件")
	}

	files := make([]forumHelper.MultipartFilePart, 0, len(input.Media)+3)
	if path := strings.TrimSpace(input.BlueprintFilePath); path != "" {
		inspected, err := forumHelper.InspectLocalFile("blueprint", path)
		if err != nil {
			return api.IDResult{}, err
		}
		files = append(files, forumHelper.MultipartFilePart{FieldName: "blueprintFile", File: inspected})
	}
	if path := strings.TrimSpace(input.ModFilePath); path != "" {
		inspected, err := forumHelper.InspectLocalFile("mod", path)
		if err != nil {
			return api.IDResult{}, err
		}
		files = append(files, forumHelper.MultipartFilePart{FieldName: "modFile", File: inspected})
	}
	if path := strings.TrimSpace(input.SaveFilePath); path != "" {
		inspected, err := forumHelper.InspectLocalFile("save", path)
		if err != nil {
			return api.IDResult{}, err
		}
		files = append(files, forumHelper.MultipartFilePart{FieldName: "save", File: inspected})
	}

	seenSlots := make(map[string]struct{}, len(input.Media))
	for _, slot := range input.Media {
		slotName := strings.TrimSpace(slot.Slot)
		if _, duplicated := seenSlots[slotName]; duplicated {
			return api.IDResult{}, forumHelper.InvalidInput("存在重复的媒体资源槽位")
		}
		seenSlots[slotName] = struct{}{}

		kind, err := service.mediaKindForSlot(slotName, 8, true)
		if err != nil {
			return api.IDResult{}, err
		}
		inspected, err := forumHelper.InspectLocalFile(kind, slot.Path)
		if err != nil {
			return api.IDResult{}, err
		}
		files = append(files, forumHelper.MultipartFilePart{FieldName: slotName, File: inspected})
	}

	draft := struct {
		RequestID  string                     `json:"requestId"`
		CategoryID uint                       `json:"categoryId"`
		Title      string                     `json:"title"`
		Body       string                     `json:"body"`
		BodyFormat string                     `json:"bodyFormat"`
		Blueprint  *api.PublishBlueprintInput `json:"blueprint,omitempty"`
		Mod        *api.PublishModInput       `json:"mod,omitempty"`
	}{
		RequestID:  requestID,
		CategoryID: input.CategoryID,
		Title:      title,
		Body:       input.Body,
		BodyFormat: bodyFormat,
		Blueprint:  input.Blueprint,
		Mod:        input.Mod,
	}

	call := request.Input{SessionKey: input.SessionKey, Target: strconv.FormatUint(uint64(input.CategoryID), 10)}
	result, err := forumHelper.DoMultipart[api.IDResult](service.callerContext(), api.PublishPostAPI, call, "post", draft, files)
	if err == nil {
		forumHelper.RecordAction("publishPost", "发布论坛帖子", "论坛服务确认发帖成功", map[string]interface{}{
			"postId":     result.ID,
			"categoryId": input.CategoryID,
			"fileCount":  len(files),
		})
	}
	return result, err
}

// PublishReply 提交帖子回复，无图片走 JSON，带图片走 multipart/form-data。
// 职责：按契约选择 JSON 或 multipart 协议提交 Markdown/纯文本回复，并支持携带最多 4 张图片。
// 使用：帖子详情页回复编辑器提交或重试时调用。
// 边界：正文最多 2,000 字符；带图回复强制使用 markdown 格式和 UUID RequestID。
// 逻辑流：1. 校验帖子 ID、正文与格式 => 2. 无图直接 DoJSON => 3. 带图校验 image1..image4 并流式 DoMultipart => 4. 记录操作日志并返回新回复。
func (service *ForumService) PublishReply(input api.PublishReplyRequest) (api.Reply, error) {
	call, err := forumHelper.IDInput(input.SessionKey, input.PostID)
	if err != nil {
		return api.Reply{}, err
	}
	bodyFormat := strings.TrimSpace(input.BodyFormat)
	if bodyFormat == "" {
		bodyFormat = "markdown"
	}
	if strings.TrimSpace(input.Body) == "" {
		return api.Reply{}, forumHelper.InvalidInput("请填写回复内容后再发送")
	}
	if utf8.RuneCountInString(input.Body) > 2000 {
		return api.Reply{}, forumHelper.InvalidInput("回复正文不能超过 2,000 个字符")
	}
	if bodyFormat != "markdown" && bodyFormat != "plain" {
		return api.Reply{}, forumHelper.InvalidInput("不支持的回复正文格式")
	}

	requestID := strings.TrimSpace(input.RequestID)
	payload := struct {
		Body       string `json:"body"`
		BodyFormat string `json:"bodyFormat,omitempty"`
		RequestID  string `json:"requestId,omitempty"`
	}{
		Body:       input.Body,
		BodyFormat: bodyFormat,
		RequestID:  requestID,
	}

	if len(input.Images) == 0 {
		call.Payload = payload
		reply, err := forumHelper.DoJSON[api.Reply](service.callerContext(), api.PublishReplyAPI, call)
		if err == nil {
			forumHelper.RecordAction("publishReply", "发表论坛回复", "论坛服务确认回复成功", map[string]interface{}{
				"postId":  input.PostID,
				"replyId": reply.ID,
			})
		}
		return reply, err
	}

	if bodyFormat != "markdown" || requestID == "" {
		return api.Reply{}, forumHelper.InvalidInput("带图片的回复必须使用 Markdown 格式并提供请求标识")
	}
	if len(input.Images) > 4 {
		return api.Reply{}, forumHelper.InvalidInput("单条回复最多附带 4 张图片")
	}

	files := make([]forumHelper.MultipartFilePart, 0, len(input.Images))
	seenSlots := make(map[string]struct{}, len(input.Images))
	for _, slot := range input.Images {
		slotName := strings.TrimSpace(slot.Slot)
		if _, duplicated := seenSlots[slotName]; duplicated {
			return api.Reply{}, forumHelper.InvalidInput("存在重复的回复图片槽位")
		}
		seenSlots[slotName] = struct{}{}
		kind, err := service.mediaKindForSlot(slotName, 4, false)
		if err != nil {
			return api.Reply{}, err
		}
		inspected, err := forumHelper.InspectLocalFile(kind, slot.Path)
		if err != nil {
			return api.Reply{}, err
		}
		files = append(files, forumHelper.MultipartFilePart{FieldName: slotName, File: inspected})
	}

	reply, err := forumHelper.DoMultipart[api.Reply](service.callerContext(), api.PublishReplyAPI, call, "reply", payload, files)
	if err == nil {
		forumHelper.RecordAction("publishReply", "发表带图论坛回复", "论坛服务确认回复成功", map[string]interface{}{
			"postId":     input.PostID,
			"replyId":    reply.ID,
			"imageCount": len(files),
		})
	}
	return reply, err
}

// Report 提交针对帖子、回复或资源的举报。
// 职责：校验目标类型、ID 和举报理由后调用 POST /client/forum/reports。
// 使用：帖子详情页、回复项或资源卡点击举报后调用。
// 边界：TargetType 仅接受 post、reply、resource。
func (service *ForumService) Report(input api.ReportRequest) (api.IDResult, error) {
	targetType := strings.TrimSpace(input.TargetType)
	reason := strings.TrimSpace(input.Reason)
	switch targetType {
	case "post", "reply", "resource":
	default:
		return api.IDResult{}, forumHelper.InvalidInput("无效的举报目标类型")
	}
	if input.TargetID == 0 || reason == "" {
		return api.IDResult{}, forumHelper.InvalidInput("请填写举报理由")
	}
	payload := struct {
		TargetType string `json:"targetType"`
		TargetID   uint   `json:"targetId"`
		Reason     string `json:"reason"`
	}{
		TargetType: targetType,
		TargetID:   input.TargetID,
		Reason:     reason,
	}
	result, err := forumHelper.DoJSON[api.IDResult](service.callerContext(), api.ReportAPI, request.Input{
		SessionKey: input.SessionKey, Payload: payload,
	})
	if err == nil {
		forumHelper.RecordAction("report", "提交论坛举报", "论坛服务已受理举报", map[string]interface{}{
			"reportId":   result.ID,
			"targetType": targetType,
			"targetId":   input.TargetID,
		})
	}
	return result, err
}

// GetResource 重新核对并返回单个可见资源的最新元数据。
// 职责：在打开市场模组或执行获取动作前向服务端确认资源尚未下架。
// 使用：点击 Thunderstore 模组卡打开市场前调用。
// 边界：已下架资源由服务端返回 notFound。
func (service *ForumService) GetResource(input api.ResourceActionRequest) (api.Resource, error) {
	call, err := forumHelper.IDInput(input.SessionKey, input.ResourceID)
	if err != nil {
		return api.Resource{}, err
	}
	return forumHelper.DoJSON[api.Resource](service.callerContext(), api.ResourceAPI, call)
}

// FetchBlueprintCode 复核蓝图资源可用性并返回可复制的蓝图编码。
// 职责：兼容 inline 蓝图编码与托管 .txt 蓝图文件，托管文件下载后执行长度、SHA-256 与 UTF-8 校验。
// 使用：蓝图资源卡点击「复制编码」时调用。
// 边界：已下架蓝图拒绝获取；托管临时文件读取后立即清理。
func (service *ForumService) FetchBlueprintCode(input api.ResourceActionRequest) (api.BlueprintCodeResult, error) {
	resource, err := service.GetResource(input)
	if err != nil {
		return api.BlueprintCodeResult{}, err
	}
	if resource.Type != "blueprint" {
		return api.BlueprintCodeResult{}, forumHelper.InvalidInput("目标资源不是蓝图")
	}
	if resource.Source == "inline" {
		if strings.TrimSpace(resource.BlueprintCode) == "" {
			return api.BlueprintCodeResult{}, forumHelper.InvalidInput("蓝图编码为空")
		}
		return api.BlueprintCodeResult{Name: resource.Name, BlueprintCode: resource.BlueprintCode}, nil
	}

	download, err := forumHelper.DownloadVerifiedFile(service.callerContext(), input.SessionKey, input.ResourceID, "blueprint", ".txt")
	if err != nil {
		return api.BlueprintCodeResult{}, err
	}
	defer download.Cleanup()

	raw, err := os.ReadFile(download.FilePath)
	if err != nil || !utf8.Valid(raw) || strings.TrimSpace(string(raw)) == "" {
		return api.BlueprintCodeResult{}, forumHelper.InvalidInput("下载的蓝图文件不是有效的 UTF-8 文本")
	}
	return api.BlueprintCodeResult{Name: download.Resource.Name, BlueprintCode: string(raw)}, nil
}

// ImportBlueprint 复核蓝图资源并导入到客户端蓝图库。
// 职责：复用 EnvironmentManagerService 的 QuickImportBlueprint（内联）或 ImportBlueprintFiles（托管文件）导入并保留既有冲突保护。
// 使用：蓝图资源卡点击「导入蓝图库」时调用。
// 边界：仅导入到客户端蓝图镜像库，不直接覆盖游戏蓝图目录；同名异内容时返回冲突提示。
// 逻辑流：1. 复核资源可用性 => 2. 内联编码直接 QuickImportBlueprint / 托管文件下载校验后 ImportBlueprintFiles => 3. 清理临时目录并记录操作日志。
func (service *ForumService) ImportBlueprint(input api.ResourceActionRequest) (api.ResourceImportResult, error) {
	resource, err := service.GetResource(input)
	if err != nil {
		return api.ResourceImportResult{}, err
	}
	if resource.Type != "blueprint" {
		return api.ResourceImportResult{}, forumHelper.InvalidInput("目标资源不是蓝图")
	}

	envService := &environment.EnvironmentManagerService{}
	if resource.Source == "inline" {
		opResult, err := envService.QuickImportBlueprint("", resource.Name, resource.BlueprintCode)
		if err != nil {
			return api.ResourceImportResult{}, service.wrapImportError("蓝图导入失败", err)
		}
		forumHelper.RecordAction("importBlueprint", "导入论坛蓝图", "已将内联蓝图导入本地蓝图库", map[string]interface{}{
			"resourceId": resource.ID,
			"name":       resource.Name,
			"source":     resource.Source,
		})
		return api.ResourceImportResult{
			Message: opResult.Message,
			Name:    resource.Name,
			Type:    resource.Type,
			Source:  resource.Source,
		}, nil
	}

	download, err := forumHelper.DownloadVerifiedFile(service.callerContext(), input.SessionKey, input.ResourceID, "blueprint", ".txt")
	if err != nil {
		return api.ResourceImportResult{}, err
	}
	defer download.Cleanup()

	opResult, err := envService.ImportBlueprintFiles("", []string{download.FilePath})
	if err != nil {
		return api.ResourceImportResult{}, service.wrapImportError("蓝图导入失败", err)
	}
	forumHelper.RecordAction("importBlueprint", "导入论坛蓝图", "已将托管蓝图校验并导入本地蓝图库", map[string]interface{}{
		"resourceId": download.Resource.ID,
		"name":       download.Resource.Name,
		"source":     download.Resource.Source,
	})
	return api.ResourceImportResult{
		Message: opResult.Message,
		Name:    download.Resource.Name,
		Type:    download.Resource.Type,
		Source:  download.Resource.Source,
	}, nil
}

// ImportSave 下载校验论坛托管存档并导入游戏存档目录。
// 职责：先复核资源可用性并校验长度与 SHA-256，再复用 EnvironmentManagerService.ImportSaves 完成安全复制与冲突保护。
// 使用：存档资源卡点击「下载并导入存档」时调用。
// 边界：目标存在同名异内容存档时返回 importConflict 错误并保留原文件；下载临时文件自动清理。
func (service *ForumService) ImportSave(input api.ResourceActionRequest) (api.ResourceImportResult, error) {
	download, err := forumHelper.DownloadVerifiedFile(service.callerContext(), input.SessionKey, input.ResourceID, "save", ".dsv")
	if err != nil {
		return api.ResourceImportResult{}, err
	}
	defer download.Cleanup()

	envService := &environment.EnvironmentManagerService{}
	opResult, err := envService.ImportSaves([]string{download.FilePath})
	if err != nil {
		return api.ResourceImportResult{}, service.wrapImportError("存档导入失败", err)
	}
	forumHelper.RecordAction("importSave", "导入论坛存档", "已将托管存档校验并导入本地存档目录", map[string]interface{}{
		"resourceId": download.Resource.ID,
		"name":       download.Resource.Name,
		"size":       download.Resource.Size,
	})
	return api.ResourceImportResult{
		Message: opResult.Message,
		Name:    download.Resource.Name,
		Type:    download.Resource.Type,
		Source:  download.Resource.Source,
	}, nil
}

// ImportMod 下载校验论坛托管模组压缩包并导入本地 Mod 资源库。
// 职责：先复核资源可用性并校验长度与 SHA-256，再复用 EnvironmentManagerService.ImportMod 完成安全导入。
// 使用：托管模组（upload）资源卡点击「下载并导入模组」时调用。
// 边界：不自动启用到游戏目录；冲突或格式不支持时返回明确错误并清理临时文件。
func (service *ForumService) ImportMod(input api.ResourceActionRequest) (api.ResourceImportResult, error) {
	download, err := forumHelper.DownloadVerifiedFile(service.callerContext(), input.SessionKey, input.ResourceID, "mod", ".zip")
	if err != nil {
		return api.ResourceImportResult{}, err
	}
	defer download.Cleanup()

	envService := &environment.EnvironmentManagerService{}
	opResult, err := envService.ImportMod([]string{download.FilePath})
	if err != nil {
		return api.ResourceImportResult{}, service.wrapImportError("模组导入失败", err)
	}
	forumHelper.RecordAction("importMod", "导入论坛模组", "已将托管模组校验并导入本地模组库", map[string]interface{}{
		"resourceId": download.Resource.ID,
		"name":       download.Resource.Name,
		"size":       download.Resource.Size,
	})
	return api.ResourceImportResult{
		Message: opResult.Message,
		Name:    download.Resource.Name,
		Type:    download.Resource.Type,
		Source:  download.Resource.Source,
	}, nil
}

// SelectImages 打开系统多文件对话框选择正文图片并注册受控本地预览。
// 职责：选择 JPEG/PNG/GIF/WebP 图片、校验单张上限 10 MiB 并生成编辑器预览地址。
// 使用：发帖（最多 8 张）或回复（最多 4 张）编辑器点击「插入图片」时调用。
// 边界：用户取消返回 Canceled=true；任一文件不合规时返回明确校验错误。
func (service *ForumService) SelectImages(maxCount int) (api.MediaSelectionResult, error) {
	if core.GLOBAL.RuntimeContext == nil {
		return api.MediaSelectionResult{}, forumHelper.InvalidInput("桌面运行上下文尚未就绪")
	}
	if maxCount <= 0 || maxCount > 8 {
		maxCount = 8
	}
	paths, err := runtime.OpenMultipleFilesDialog(core.GLOBAL.RuntimeContext, runtime.OpenDialogOptions{
		Title: "选择正文图片（JPEG / PNG / GIF / WebP，单张不超过 10 MiB）",
		Filters: []runtime.FileFilter{
			{DisplayName: "支持的图片 (*.png;*.jpg;*.jpeg;*.gif;*.webp)", Pattern: "*.png;*.jpg;*.jpeg;*.gif;*.webp"},
		},
	})
	if err != nil {
		return api.MediaSelectionResult{}, forumHelper.InvalidInput("打开图片选择器失败")
	}
	if len(paths) == 0 {
		return api.MediaSelectionResult{Files: []api.SelectedLocalFile{}, Canceled: true}, nil
	}
	if len(paths) > maxCount {
		return api.MediaSelectionResult{}, forumHelper.InvalidInput("当前最多还能选择 " + strconv.Itoa(maxCount) + " 张图片")
	}
	selected := make([]api.SelectedLocalFile, 0, len(paths))
	for _, path := range paths {
		inspected, err := forumHelper.InspectLocalFile("image", path)
		if err != nil {
			return api.MediaSelectionResult{}, err
		}
		token, previewURL := forumHelper.RegisterLocalPreview(inspected)
		selected = append(selected, api.SelectedLocalFile{
			Path:         inspected.Path,
			Name:         inspected.Name,
			Size:         inspected.Size,
			Ext:          inspected.Ext,
			ContentType:  inspected.ContentType,
			PreviewURL:   previewURL,
			PreviewToken: token,
		})
	}
	return api.MediaSelectionResult{Files: selected, Canceled: false}, nil
}

// SelectVideo 打开系统文件对话框选择 MP4 视频并注册受控本地预览。
// 职责：校验 MP4 格式及 128 MiB 上限并返回本地 Range 预览地址。
// 使用：发帖编辑器点击「插入视频」时调用。
// 边界：单帖仅支持一个视频；用户取消返回 Canceled=true。
func (service *ForumService) SelectVideo() (api.MediaSelectionResult, error) {
	if core.GLOBAL.RuntimeContext == nil {
		return api.MediaSelectionResult{}, forumHelper.InvalidInput("桌面运行上下文尚未就绪")
	}
	path, err := runtime.OpenFileDialog(core.GLOBAL.RuntimeContext, runtime.OpenDialogOptions{
		Title: "选择 MP4 视频（不超过 128 MiB）",
		Filters: []runtime.FileFilter{
			{DisplayName: "MP4 视频 (*.mp4)", Pattern: "*.mp4"},
		},
	})
	if err != nil {
		return api.MediaSelectionResult{}, forumHelper.InvalidInput("打开视频选择器失败")
	}
	if strings.TrimSpace(path) == "" {
		return api.MediaSelectionResult{Files: []api.SelectedLocalFile{}, Canceled: true}, nil
	}
	inspected, err := forumHelper.InspectLocalFile("video", path)
	if err != nil {
		return api.MediaSelectionResult{}, err
	}
	token, previewURL := forumHelper.RegisterLocalPreview(inspected)
	return api.MediaSelectionResult{
		Files: []api.SelectedLocalFile{{
			Path:         inspected.Path,
			Name:         inspected.Name,
			Size:         inspected.Size,
			Ext:          inspected.Ext,
			ContentType:  inspected.ContentType,
			PreviewURL:   previewURL,
			PreviewToken: token,
		}},
		Canceled: false,
	}, nil
}

// SelectResourceFile 打开系统文件对话框选择单个蓝图、模组或存档托管文件。
// 职责：按 kind 限制文件筛选器并校验大小与格式上限。
// 使用：发帖编辑器选择托管蓝图（.txt）、托管模组（.zip）或存档（.dsv）时调用。
// 边界：kind 仅支持 blueprint、mod、save；不注册本地媒体预览。
func (service *ForumService) SelectResourceFile(kind string) (api.MediaSelectionResult, error) {
	if core.GLOBAL.RuntimeContext == nil {
		return api.MediaSelectionResult{}, forumHelper.InvalidInput("桌面运行上下文尚未就绪")
	}
	var title string
	var filters []runtime.FileFilter
	switch kind {
	case "blueprint":
		title = "选择蓝图文本文件（UTF-8 .txt，不超过 512 KiB）"
		filters = []runtime.FileFilter{{DisplayName: "蓝图文件 (*.txt)", Pattern: "*.txt"}}
	case "mod":
		title = "选择模组压缩包（.zip，不超过 256 MiB）"
		filters = []runtime.FileFilter{{DisplayName: "模组压缩包 (*.zip)", Pattern: "*.zip"}}
	case "save":
		title = "选择戴森球主存档（.dsv，不超过 512 MiB）"
		filters = []runtime.FileFilter{{DisplayName: "戴森球存档 (*.dsv)", Pattern: "*.dsv"}}
	default:
		return api.MediaSelectionResult{}, forumHelper.InvalidInput("不支持的资源文件类型")
	}

	path, err := runtime.OpenFileDialog(core.GLOBAL.RuntimeContext, runtime.OpenDialogOptions{
		Title:   title,
		Filters: filters,
	})
	if err != nil {
		return api.MediaSelectionResult{}, forumHelper.InvalidInput("打开文件选择器失败")
	}
	if strings.TrimSpace(path) == "" {
		return api.MediaSelectionResult{Files: []api.SelectedLocalFile{}, Canceled: true}, nil
	}
	inspected, err := forumHelper.InspectLocalFile(kind, path)
	if err != nil {
		return api.MediaSelectionResult{}, err
	}
	return api.MediaSelectionResult{
		Files: []api.SelectedLocalFile{{
			Path:        inspected.Path,
			Name:        inspected.Name,
			Size:        inspected.Size,
			Ext:         inspected.Ext,
			ContentType: inspected.ContentType,
		}},
		Canceled: false,
	}, nil
}

// ReleaseLocalPreviews 释放编辑器不再需要的本地媒体预览令牌。
// 职责：转发清理请求到 helper/forum 的受控预览注册表。
// 使用：编辑器移除图片/视频、发布成功或离开时调用。
// 边界：仅释放内存映射，不修改本地文件。
func (service *ForumService) ReleaseLocalPreviews(tokens []string) error {
	forumHelper.RevokeLocalPreviews(tokens)
	return nil
}

// mediaKindForSlot 校验媒体槽位名称并返回对应文件类型。
// 职责：约束发帖支持 image1..image8 和 video，回复支持 image1..image4。
// 使用：PublishPost 和 PublishReply 校验上传槽位时调用。
// 边界：非法槽位名返回 invalidInput 错误。
func (service *ForumService) mediaKindForSlot(slot string, maxImages int, allowVideo bool) (string, error) {
	if allowVideo && slot == "video" {
		return "video", nil
	}
	if strings.HasPrefix(slot, "image") {
		index, err := strconv.Atoi(strings.TrimPrefix(slot, "image"))
		if err == nil && index >= 1 && index <= maxImages {
			return "image", nil
		}
	}
	return "", forumHelper.InvalidInput("包含不支持的媒体资源槽位：" + slot)
}

// wrapImportError 将环境管理服务的导入错误转换为带原因码的论坛结构化错误。
// 职责：识别同名异内容冲突并向前端保留清晰说明。
// 使用：ImportBlueprint、ImportSave 和 ImportMod 失败分支调用。
// 边界：冲突场景返回 importConflict，其余导入失败返回 importFailed。
func (service *ForumService) wrapImportError(prefix string, err error) error {
	message := strings.TrimSpace(err.Error())
	if strings.Contains(message, "冲突") || strings.Contains(message, "已存在") || strings.Contains(message, "不同") {
		return forumHelper.ImportFailure("importConflict", message)
	}
	if message == "" {
		return forumHelper.ImportFailure("importFailed", prefix)
	}
	return forumHelper.ImportFailure("importFailed", prefix+"："+message)
}
