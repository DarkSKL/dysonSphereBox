package forum

// 文件职责：将论坛读取输入绑定到接口描述，注册论坛的分页和失效中间件。
// 使用：分类、列表、详情、回复及发布接口。
// 边界：缓存时限和来源由接口声明决定，写入可见性来自论坛回执。

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"

	api "DysonSphereBox/apiDefinition/forum"
	"DysonSphereBox/core/request"
	"DysonSphereBox/core/requestcache"
	"DysonSphereBox/helper/system/account"
)

// 分页边界沿用论坛接口契约。
const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// init 注册论坛扩展阶段，启动加载器据接口声明选择使用。
func init() {
	loader := account.ServerRequests().Loader
	loader.RegisterBefore(api.NormalizePage, normalizePage)
	loader.RegisterAfter(api.InvalidatePublished, invalidatePublished)
}

// QueryCategories 职责：读取论坛分类；使用：列表和发帖表单；边界：Force 跳过进程内视图快照。
func QueryCategories(ctx context.Context, sessionKey string, force bool) ([]api.Category, error) {
	return DoJSON[[]api.Category](ctx, api.CategoriesAPI, request.Input{SessionKey: sessionKey, Force: force})
}

// QueryPosts 职责：按分类查询帖子分页；使用：列表入口；边界：归一化后再生成缓存键。
func QueryPosts(ctx context.Context, input api.PostListRequest) (api.PostPage, error) {
	query := pageValues(input.Page, input.PageSize)
	target := ""
	if input.CategoryID > 0 {
		target = strconv.FormatUint(uint64(input.CategoryID), 10)
		query.Set("categoryId", target)
	}
	return DoJSON[api.PostPage](ctx, api.PostsAPI, request.Input{
		SessionKey: input.SessionKey, Query: query, Force: input.Force, Target: target,
	})
}

// QueryPostDetail 职责：读取单篇论坛详情；使用：打开详情或强制复核；边界：按帖子 ID 隔离结果。
func QueryPostDetail(ctx context.Context, input api.PostDetailRequest) (api.PostDetail, error) {
	query, err := IDInput(input.SessionKey, input.PostID)
	if err != nil {
		return api.PostDetail{}, err
	}
	query.Force = input.Force
	return DoJSON[api.PostDetail](ctx, api.PostDetailAPI, query)
}

// QueryReplies 职责：按帖子读取回复分页；使用：详情评论区；边界：只共享等价在途请求。
func QueryReplies(ctx context.Context, input api.ReplyListRequest) (api.ReplyPage, error) {
	query, err := IDInput(input.SessionKey, input.PostID)
	if err != nil {
		return api.ReplyPage{}, err
	}
	query.Query = pageValues(input.Page, input.PageSize)
	return DoJSON[api.ReplyPage](ctx, api.RepliesAPI, query)
}

// IDInput 职责：绑定正整数资源或帖子 ID；使用：论坛动态路由和失效目标；边界：零 ID 在联网前拒绝。
func IDInput(sessionKey string, id uint) (request.Input, error) {
	if id == 0 {
		return request.Input{}, InvalidInput("缺少有效的目标 ID")
	}
	return request.Input{SessionKey: sessionKey, PathParams: api.IDPathParams{ID: id}, Target: strconv.FormatUint(uint64(id), 10)}, nil
}

// pageValues 提供原始分页值，加载的请求准备阶段负责归一化。
func pageValues(page, pageSize int) url.Values {
	return url.Values{"page": {strconv.Itoa(page)}, "pageSize": {strconv.Itoa(pageSize)}}
}

// normalizePage 在构造缓存键前归一化省略、非法或越界的分页值。
func normalizePage(_ context.Context, input *request.Parameters) error {
	page, _ := strconv.Atoi(input.Query.Get("page"))
	size, _ := strconv.Atoi(input.Query.Get("pageSize"))
	if size < 1 || size > maxPageSize {
		size = defaultPageSize
	}
	input.Query.Set("page", strconv.Itoa(max(1, page)))
	input.Query.Set("pageSize", strconv.Itoa(size))
	return nil
}

// invalidatePublished 只有公开发布回执的正整数 ID 才会使相应帖子或分类失效。
func invalidatePublished(_ context.Context, result request.Success) []requestcache.Source {
	var visible api.IDResult
	if json.Unmarshal(result.Data, &visible) != nil || visible.ID == 0 {
		return nil
	}
	return []requestcache.Source{{Method: result.API, Target: result.Target}}
}
