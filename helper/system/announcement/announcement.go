package announcement

// 文件职责：执行客户端公告最新查询与手动已读请求，绑定后端捕获会话。
// 使用：service/system/AnnouncementService由Wails调用，前端只收到不含token的DTO。
// 边界：不决定弹窗排队或HTML渲染；超时、响应大小和业务码在此处收敛。

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	api "DysonSphereBox/apiDefinition"
	systemAPI "DysonSphereBox/apiDefinition/system"
	"DysonSphereBox/core/request"
	"DysonSphereBox/core/requestcache"
	accountHelper "DysonSphereBox/helper/system/account"
)

// NoticeSession捕获一次已验证账号作用域并隐藏token。
// 职责：保证公告请求不会在账号切换后改绑新身份。
// 使用：前端先调用CaptureSession，再用句柄查询/确认。
// 边界：句柄仅内存有效，切服或会话代次变化后失效。
type NoticeSession struct {
	// scope 固定原服务器、主体和会话生命周期，只在发请求时取得当前凭据。
	scope   requestcache.Scope
	subject string
	// itemID、revision 记录最新查询授权展示的版本。
	itemID   string
	revision int64
	// validUntil 使用单调时钟估算首次确认截止时间。
	validUntil time.Time
	// querySequence、historySequence 分别隔离最新与历史查询的迟到结果。
	querySequence   uint64
	historySequence uint64
	// historyItems 只保存最近成功且已校验的一页。
	historyItems map[string]systemAPI.ClientNoticeHistoryEntry
	// historyServerTime、historyRequestStartedAt 共同估算服务端有效窗口。
	historyServerTime       string
	historyRequestStartedAt time.Time
	// historyCtx、historyCancel 在换页或换详情时取消旧确认。
	historyCtx    context.Context
	historyCancel context.CancelFunc
	// selectedHistory 仅由当前页用户选择建立，不能从任意ID构造。
	selectedHistory *historySelection
}

// historySelection 职责：记录当前页用户选择的版本；使用：详情确认；边界：过期或已读条目不得首次确认。
type historySelection struct {
	// id、revision 固定当前选择版本。
	id       string
	revision int64
	// read 在服务端回执接纳后同步，不通过本地展示推断。
	read bool
	// validUntil 使用单调时钟估算首次确认期限。
	validUntil time.Time
}

// sessions 职责：保存本进程公告句柄及版本绑定；使用：公告helper持锁访问；边界：不写磁盘，失效账号不能复用。
var sessions = struct {
	sync.Mutex
	values map[string]*NoticeSession
}{values: map[string]*NoticeSession{}}

// CaptureSessionID 职责：捕获或复用同账号代次的不透明句柄；使用：Wails查询前调用；边界：同作用域不清除已有展示绑定，不返回token。
func CaptureSessionID() (string, error) {
	s, err := CaptureSession()
	if err != nil {
		return "", err
	}
	var bytes [24]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	id := hex.EncodeToString(bytes[:])
	sessions.Lock()
	if s.scope.Context.Err() != nil {
		sessions.Unlock()
		return "", fail("stale", "账号会话已切换")
	}
	for previous, existing := range sessions.values {
		if existing.scope == s.scope {
			// 相同作用域复用完整绑定，避免历史入口捕获句柄时撤销已显示的最新公告。
			sessions.Unlock()
			return previous, nil
		}
		if existing.historyCancel != nil {
			existing.historyCancel()
		}
		delete(sessions.values, previous)
	}
	sessions.values[id] = s
	sessions.Unlock()
	return id, nil
}

// sessionByID 职责：复制有效公告句柄及其版本绑定；使用：请求前后核对；边界：失效句柄立即移除。
func sessionByID(id string) *NoticeSession {
	sessions.Lock()
	defer sessions.Unlock()
	s := sessions.values[id]
	if s == nil {
		return nil
	}
	if s.scope.Context.Err() != nil {
		delete(sessions.values, id)
		return nil
	}
	copy := *s
	if s.selectedHistory != nil {
		selection := *s.selectedHistory
		copy.selectedHistory = &selection
	}
	return &copy
}

// beginQuery 职责：清除旧版本并分配查询次序；使用：每次GET发送前；边界：失败或null不能恢复旧版本提交权。
func beginQuery(id string) uint64 {
	sessions.Lock()
	defer sessions.Unlock()
	if s := sessions.values[id]; s != nil {
		s.querySequence++
		s.itemID, s.revision, s.validUntil = "", 0, time.Time{}
		return s.querySequence
	}
	return 0
}

// bindItem 职责：只绑定仍为最新的GET版本；使用：严格校验成功后；边界：迟到查询不能覆盖较新查询或失败状态。
func bindItem(id string, sequence uint64, itemID string, revision int64, validUntil time.Time) bool {
	sessions.Lock()
	defer sessions.Unlock()
	if s := sessions.values[id]; s != nil && s.querySequence == sequence {
		s.itemID, s.revision, s.validUntil = itemID, revision, validUntil
		return true
	}
	return false
}

// NoticeItemDTO是前端显示公告的数据副本。
// 职责：传输已校验公告字段，不暴露token。
// 使用：Wails绑定返回Vue。
// 边界：content仍是源码，由前端清洗。
type NoticeItemDTO = systemAPI.ClientNoticeItem

// NoticeLatestDTO是最新查询结果。
// 职责：返回server时间和最多一个合法item。
// 使用：前端自动/手动检查。
// 边界：无内容以nil item表达。
type NoticeLatestDTO struct {
	SubjectID  string         `json:"subjectId"`
	ServerTime string         `json:"serverTime"`
	Item       *NoticeItemDTO `json:"item"`
}

// MarkReadDTO是手动确认回执。
// 职责：返回server幂等首次时间。
// 使用：前端核对后关闭详情。
// 边界：不表示客户端停留或逐字阅读。
type MarkReadDTO = systemAPI.ClientNoticeMarkReadReceipt

// CaptureSession 职责：捕获可信账号和请求取消上下文；使用：创建公告句柄；边界：未验证或需改密时失败，不进行网络查询。
func CaptureSession() (*NoticeSession, error) {
	access, err := accountHelper.ServerRequests().Capture(systemAPI.LatestNoticeAPI, "")
	if err != nil {
		return nil, errors.New("账号尚未完成验证或需要修改密码")
	}
	return &NoticeSession{scope: access.Scope, subject: access.Subject}, nil
}

// requestNotice 按原公告句柄取得最新凭据，执行接口并交付待校验的业务数据。
func requestNotice(ctx context.Context, session *NoticeSession, desc *api.ApiDesc, query url.Values, payload any) (json.RawMessage, error) {
	adapter := accountHelper.ServerRequests()
	access, err := adapter.Capture(desc, session.scope.ID)
	if err != nil || access.Scope != session.scope {
		return nil, fail("stale", "账号会话已切换")
	}
	data, err := request.Do[json.RawMessage](ctx, adapter, desc, request.Input{Session: &access, Query: query, Payload: payload})
	if err != nil {
		return nil, noticeRequestFailure(err)
	}
	return data, nil
}

// noticeRequestFailure 保留前端可恢复错误码及明确的协议提示。
func noticeRequestFailure(err error) error {
	if detail := request.Details(err); detail != nil {
		if detail.Status == http.StatusNotFound {
			return fail("unsupported", "当前服务器尚不支持公告，请升级服务器后重试")
		}
		switch detail.Kind {
		case request.Stale:
			return fail("stale", "账号会话已切换")
		case request.Unauthorized:
			return fail("unauthorized", "登录已失效，请重新登录")
		case request.PasswordRequired:
			return fail("passwordRequired", "请先修改账号密码")
		case request.ReadFailed:
			return errors.New("公告响应超过4MiB或读取失败")
		case request.InvalidEnvelope:
			return errors.New("公告响应外壳格式无效")
		case request.BusinessFailed:
			var data struct {
				Reason string `json:"reason"`
			}
			if detail.Code == 7 && json.Unmarshal(detail.Data, &data) == nil &&
				(data.Reason == "revisionChanged" || data.Reason == "announcementUnavailable") {
				return fail(data.Reason, "公告内容已变化或失效，请重新检查最新公告")
			}
		}
	}
	return errors.New("公告请求失败，请稍后重试")
}

// GetLatest 职责：读取并绑定唯一最新未读版本；使用：服务层检查；边界：无内容不保存可提交版本，过期不返回。
func GetLatest(ctx context.Context, sessionID string) (*NoticeLatestDTO, error) {
	session := sessionByID(sessionID)
	if session == nil {
		return nil, errors.New("公告会话已失效")
	}
	sequence := beginQuery(sessionID)
	requestStartedAt := time.Now()
	raw, err := requestNotice(ctx, session, systemAPI.LatestNoticeAPI, nil, nil)
	if err != nil {
		return nil, err
	}
	result, err := validateLatest(raw, session.subject)
	if err != nil {
		return nil, err
	}
	if result.Item == nil {
		if sessionByID(sessionID) == nil || !bindItem(sessionID, sequence, "", 0, time.Time{}) {
			return nil, fail("stale", "公告查询已被替换")
		}
		return result, nil
	}
	deadline := time.Time{}
	if result.Item.EndsAt != nil {
		end, _ := parseUTC(*result.Item.EndsAt)
		now, _ := parseUTC(result.ServerTime)
		deadline = requestStartedAt.Add(end.Sub(now))
		if !time.Now().Before(deadline) {
			return nil, errors.New("公告已到期，请重新检查")
		}
	}
	if sessionByID(sessionID) == nil {
		return nil, errors.New("公告会话已失效")
	}
	if !bindItem(sessionID, sequence, result.Item.ID, result.Item.Revision, deadline) {
		return nil, fail("stale", "公告查询已被替换")
	}
	return result, nil
}

// MarkRead 职责：提交绑定版本的手动确认；使用：前端用户按钮；边界：同键失败由用户重试，失效版本停止提交。
func MarkRead(ctx context.Context, sessionID, id string, revision int64) (*MarkReadDTO, error) {
	session := sessionByID(sessionID)
	if session == nil || session.itemID != id || session.revision != revision || id == "" {
		return nil, errors.New("公告显示句柄与账号或版本不匹配")
	}
	if !session.validUntil.IsZero() && !time.Now().Before(session.validUntil) {
		return nil, errors.New("公告已到期")
	}
	raw, err := requestNotice(ctx, session, systemAPI.MarkNoticeReadAPI, nil, systemAPI.ClientNoticeMarkReadRequest{ID: id, Revision: revision})
	if err != nil {
		return nil, err
	}
	current := sessionByID(sessionID)
	if current == nil || current.querySequence != session.querySequence || current.itemID != id || current.revision != revision {
		return nil, errors.New("公告会话已失效")
	}
	receipt, err := validateReceipt(raw, session.subject, id, revision)
	if err != nil {
		return nil, err
	}
	if !bindReadReceipt(sessionID, session, receipt, false) {
		return nil, fail("stale", "公告查询已被替换")
	}
	return receipt, nil
}

// bindReadReceipt 职责：在同一版本绑定内接纳已校验回执并同步缓存状态；使用：手动确认成功后；边界：旧分页或旧查询不得修改新状态。
func bindReadReceipt(sessionID string, snapshot *NoticeSession, receipt *MarkReadDTO, history bool) bool {
	sessions.Lock()
	defer sessions.Unlock()
	current := sessions.values[sessionID]
	if current == nil || current.scope != snapshot.scope || current.scope.Context.Err() != nil {
		return false
	}
	if history {
		if current.historySequence != snapshot.historySequence || current.selectedHistory == nil || current.selectedHistory.id != receipt.ID || current.selectedHistory.revision != receipt.Revision {
			return false
		}
	} else if current.querySequence != snapshot.querySequence || current.itemID != receipt.ID || current.revision != receipt.Revision {
		return false
	}
	key := receipt.ID + ":" + fmt.Sprintf("%d", receipt.Revision)
	if entry, ok := current.historyItems[key]; ok {
		entry.Read, entry.ReadAt = true, &receipt.FirstReadAt
		current.historyItems[key] = entry
	}
	if current.selectedHistory != nil && current.selectedHistory.id == receipt.ID && current.selectedHistory.revision == receipt.Revision {
		current.selectedHistory.read = true
	}
	return true
}
