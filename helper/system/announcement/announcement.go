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
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	systemAPI "DysonSphereBox/apiDefinition/system"
	"DysonSphereBox/core"
	accountHelper "DysonSphereBox/helper/system/account"
)

// responseLimit 职责：限制完整编码响应为4MiB；使用：读取limit+1判断超限；边界：不截断为成功响应。
const responseLimit = 4 << 20

// NoticeSession捕获一次已验证账号作用域并隐藏token。
// 职责：保证公告请求不会在账号切换后改绑新身份。
// 使用：前端先调用CaptureSession，再用句柄查询/确认。
// 边界：句柄仅内存有效，切服或会话代次变化后失效。
type NoticeSession struct {
	// generation 固定账号代次，切号或退出后不得接纳旧结果。
	generation uint64
	// subject、serverKey、token 固定身份与请求目标，token只在Go侧使用。
	subject, serverKey, token string
	// client 固定HTTP实例，不随GLOBAL替换而改绑。
	client *core.HTTPClient
	// ctx 在账号切换代次时取消。
	ctx context.Context
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
	g, subject, key, _, _, client, ok := accountHelper.VerifiedSession()
	if !ok || g != s.generation || subject != s.subject || key != s.serverKey || client != s.client {
		sessions.Unlock()
		return "", fail("stale", "账号会话已切换")
	}
	for previous, existing := range sessions.values {
		if existing.generation == s.generation && existing.subject == s.subject && existing.serverKey == s.serverKey && existing.client == s.client {
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

// sessionByID 职责：按当前验证身份复制句柄及最新token；使用：每次请求前后核对；边界：失效句柄立即移除，不把续期token返回前端。
func sessionByID(id string) *NoticeSession {
	sessions.Lock()
	defer sessions.Unlock()
	s := sessions.values[id]
	if s == nil {
		return nil
	}
	g, subject, key, token, _, client, ok := accountHelper.VerifiedSession()
	if !ok || g != s.generation || subject != s.subject || key != s.serverKey || client != s.client || token == "" {
		delete(sessions.values, id)
		return nil
	}
	copy := *s
	copy.token = token
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
	g, subject, key, token, ctx, client, ok := accountHelper.VerifiedSession()
	if !ok {
		return nil, errors.New("账号尚未完成验证或需要修改密码")
	}
	return &NoticeSession{generation: g, subject: subject, serverKey: key, token: token, ctx: ctx, client: client}, nil
}

// request 职责：执行有界公告HTTP并按捕获会话接纳鉴权副作用；使用：最新、历史及手动确认；边界：禁止重定向，超时或超限失败，不自动重试。
// 逻辑流：1. 固定原会话token => 2. 有界读取统一响应 => 3. 核对会话后接纳续期或鉴权错误 => 4. 返回待校验业务数据。
func request[T any](caller context.Context, session *NoticeSession, method, path string, payload any) (T, error) {
	var zero T
	if session == nil {
		return zero, errors.New("公告会话已失效")
	}
	ctx, cancel := context.WithTimeout(caller, 10*time.Second)
	defer cancel()
	stop := context.AfterFunc(session.ctx, cancel)
	defer stop()
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return zero, err
		}
		body = strings.NewReader(string(data))
	}
	req, err := session.client.NewRequest(ctx, method, strings.TrimPrefix(path, "/"), body)
	if err != nil {
		return zero, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cache-Control", "no-store")
	req.Header.Set("Referer", "")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("x-token", session.token)
	resp, err := session.client.DoNoRedirect(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		if !accountHelper.HandleAnnouncementAuthResponse(session.generation, session.token, session.client, "", true, false) {
			return zero, fail("stale", "账号会话已切换")
		}
		return zero, fail("unauthorized", "登录已失效，请重新登录")
	}
	if resp.StatusCode == http.StatusNotFound {
		return zero, fail("unsupported", "当前服务器尚不支持公告，请升级服务器后重试")
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, responseLimit+1))
	if err != nil || len(raw) > responseLimit {
		return zero, errors.New("公告响应超过4MiB或读取失败")
	}
	fields, err := requireFields(raw, "code", "data", "msg")
	if err != nil {
		return zero, err
	}
	var envelope systemAPI.ClientNoticeEnvelope
	if string(fields["code"]) == "null" || string(fields["msg"]) == "null" || json.Unmarshal(raw, &envelope) != nil {
		return zero, errors.New("公告响应外壳格式无效")
	}
	if resp.StatusCode == http.StatusConflict && envelope.Code == 10001 {
		var flags struct {
			NeedChangePassword bool `json:"needChangePassword"`
		}
		_ = json.Unmarshal(envelope.Data, &flags)
		if flags.NeedChangePassword {
			if !accountHelper.HandleAnnouncementAuthResponse(session.generation, session.token, session.client, resp.Header.Get("new-token"), false, true) {
				return zero, fail("stale", "账号会话已切换")
			}
			return zero, fail("passwordRequired", "请先修改账号密码")
		}
	}
	if !accountHelper.HandleAnnouncementAuthResponse(session.generation, session.token, session.client, resp.Header.Get("new-token"), false, false) {
		return zero, errors.New("公告会话已失效")
	}
	if resp.StatusCode != http.StatusOK || envelope.Code != 0 {
		var reason struct {
			Reason string `json:"reason"`
		}
		_ = json.Unmarshal(envelope.Data, &reason)
		if resp.StatusCode == 200 && envelope.Code == 7 && (reason.Reason == "revisionChanged" || reason.Reason == "announcementUnavailable") {
			return zero, fail(reason.Reason, "公告内容已变化或失效，请重新检查最新公告")
		}
		return zero, errors.New("公告请求失败，请稍后重试")
	}
	if err := json.Unmarshal(envelope.Data, &zero); err != nil {
		return zero, err
	}
	return zero, nil
}

// GetLatest 职责：读取并绑定唯一最新未读版本；使用：服务层检查；边界：无内容不保存可提交版本，过期不返回。
func GetLatest(ctx context.Context, sessionID string) (*NoticeLatestDTO, error) {
	session := sessionByID(sessionID)
	if session == nil {
		return nil, errors.New("公告会话已失效")
	}
	sequence := beginQuery(sessionID)
	requestStartedAt := time.Now()
	raw, err := request[json.RawMessage](ctx, session, http.MethodGet, systemAPI.APIURLClientNoticeGetLatestUnread, nil)
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
	raw, err := request[json.RawMessage](ctx, session, http.MethodPost, systemAPI.APIURLClientNoticeMarkRead, systemAPI.ClientNoticeMarkReadRequest{ID: id, Revision: revision})
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
	if current == nil || current.generation != snapshot.generation || current.client != snapshot.client {
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
