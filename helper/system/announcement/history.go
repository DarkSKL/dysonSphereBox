package announcement

// 文件职责：维护分页历史查询、当前页详情绑定与手动查阅请求。
// 使用：AnnouncementService调用，复用同账号公告句柄与有界HTTP请求。
// 边界：过期历史可查看但不能首次查阅；翻页失败也会撤销旧页选择权。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	systemAPI "DysonSphereBox/apiDefinition/system"
	accountHelper "DysonSphereBox/helper/system/account"
)

// NoticeHistoryDTO是分页历史查询结果。
// 职责：返回server时间和分页公告列表。
// 使用：前端历史列表展示。
// 边界：列表可为空，分页元数据由helper校验。
type NoticeHistoryDTO struct {
	// SubjectID 标识当前验证账号；前端只比较会话，不可修改请求身份。
	SubjectID string `json:"subjectId"`
	// ServerTime 是服务端响应快照时刻；本地用其与单调时钟估算展示有效期。
	ServerTime string `json:"serverTime"`
	// List 仅包含当前分页且已通过协议校验的历史公告。
	List []systemAPI.ClientNoticeHistoryEntry `json:"list"`
	// Total 是服务端公布的符合历史条件的条目总数。
	Total int64 `json:"total"`
	// Page 是响应确认的当前页码。
	Page int64 `json:"page"`
	// PageSize 是响应确认的当前每页数量，范围为1至100。
	PageSize int64 `json:"pageSize"`
}

// GetHistory 职责：读取指定页码并绑定当前账号历史列表；使用：前端分页展示；边界：失败也撤销旧页选择权，严格校验分页元数据。
func GetHistory(ctx context.Context, sessionID string, page, pageSize int64) (*NoticeHistoryDTO, error) {
	session := sessionByID(sessionID)
	if session == nil {
		return nil, errors.New("公告会话已失效")
	}
	if page < 1 || pageSize < 1 || pageSize > 100 {
		return nil, errors.New("分页参数无效")
	}
	sequence, historyCtx := beginHistoryQuery(sessionID, ctx)
	if historyCtx == nil {
		return nil, errors.New("公告会话已失效")
	}
	requestStartedAt := time.Now()
	path := systemAPI.APIURLClientNoticeGetHistory + "?page=" + fmt.Sprintf("%d", page) + "&pageSize=" + fmt.Sprintf("%d", pageSize)
	raw, err := request[json.RawMessage](historyCtx, session, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	result, err := validateHistory(raw, session.subject, page, pageSize)
	if err != nil {
		return nil, err
	}
	if sessionByID(sessionID) == nil || !bindHistoryPage(sessionID, sequence, result.List, result.ServerTime, requestStartedAt) {
		return nil, fail("stale", "公告查询已被替换")
	}
	return result, nil
}

// beginHistoryQuery 职责：使旧分页和详情版本失效；使用：发出历史GET前；边界：不更改最新弹窗的已读绑定。
func beginHistoryQuery(id string, caller context.Context) (uint64, context.Context) {
	sessions.Lock()
	defer sessions.Unlock()
	if session := sessions.values[id]; session != nil {
		if session.historyCancel != nil {
			session.historyCancel()
		}
		session.historySequence++
		session.historyItems = nil
		session.selectedHistory = nil
		session.historyCtx, session.historyCancel = context.WithCancel(caller)
		return session.historySequence, session.historyCtx
	}
	return 0, nil
}

// bindHistoryPage 职责：仅保存当前代次最后一个通过校验的分页条目；使用：历史GET成功后；边界：迟到查询不能恢复旧版本权限。
func bindHistoryPage(id string, sequence uint64, entries []systemAPI.ClientNoticeHistoryEntry, serverTime string, requestStartedAt time.Time) bool {
	sessions.Lock()
	defer sessions.Unlock()
	session := sessions.values[id]
	if session == nil || session.historySequence != sequence {
		return false
	}
	session.historyServerTime = serverTime
	session.historyRequestStartedAt = requestStartedAt
	session.historyItems = make(map[string]systemAPI.ClientNoticeHistoryEntry, len(entries))
	for _, entry := range entries {
		session.historyItems[entry.ID+":"+fmt.Sprintf("%d", entry.Revision)] = entry
	}
	return true
}

// GetHistoryItem 职责：返回本页已校验且由用户选择的完整历史条目；使用：前端打开详情时；边界：不能用任意ID/revision构造确认绑定。
func GetHistoryItem(sessionID, id string, revision int64) (*systemAPI.ClientNoticeHistoryEntry, error) {
	sessions.Lock()
	defer sessions.Unlock()
	session := sessions.values[sessionID]
	if session == nil {
		return nil, errors.New("公告会话已失效")
	}
	g, subject, key, token, _, client, ok := accountHelper.VerifiedSession()
	if !ok || g != session.generation || subject != session.subject || key != session.serverKey || client != session.client || token == "" {
		delete(sessions.values, sessionID)
		return nil, errors.New("公告会话已失效")
	}
	itemKey := id + ":" + fmt.Sprintf("%d", revision)
	entry, ok := session.historyItems[itemKey]
	if !ok {
		return nil, errors.New("公告不属于当前历史页")
	}
	if session.historyCancel != nil {
		session.historyCancel()
	}
	session.historyCtx, session.historyCancel = context.WithCancel(session.ctx)
	validUntil := time.Time{}
	if entry.EndsAt != nil {
		end, err := parseUTC(*entry.EndsAt)
		if err != nil {
			return nil, err
		}
		now, err := parseUTC(session.historyServerTime)
		if err != nil {
			return nil, err
		}
		validUntil = session.historyRequestStartedAt.Add(end.Sub(now))
	}
	session.selectedHistory = &historySelection{id: id, revision: revision, read: entry.Read, validUntil: validUntil}
	copy := entry
	return &copy, nil
}

// MarkHistoryRead 职责：按用户明确操作确认历史公告；使用：详情正文真实可见后；边界：只接受本会话本页已选择的版本并沿用首次确认期限。
func MarkHistoryRead(ctx context.Context, sessionID, id string, revision int64) (*MarkReadDTO, error) {
	session := sessionByID(sessionID)
	if session == nil || session.selectedHistory == nil || session.selectedHistory.id != id || session.selectedHistory.revision != revision {
		return nil, errors.New("公告显示句柄与账号或版本不匹配")
	}
	selected := session.selectedHistory
	if selected.read {
		return nil, errors.New("公告已查阅")
	}
	if !selected.validUntil.IsZero() && !time.Now().Before(selected.validUntil) {
		return nil, errors.New("公告已到期，不能首次确认")
	}
	sessions.Lock()
	currentSession := sessions.values[sessionID]
	if currentSession == nil || currentSession.historySequence != session.historySequence || currentSession.selectedHistory == nil || currentSession.selectedHistory.id != id || currentSession.selectedHistory.revision != revision {
		sessions.Unlock()
		return nil, errors.New("公告历史选择已变化")
	}
	selectionCtx := currentSession.historyCtx
	sessions.Unlock()
	requestCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if selectionCtx != nil {
		stop := context.AfterFunc(selectionCtx, cancel)
		defer stop()
	}
	raw, err := request[json.RawMessage](requestCtx, session, http.MethodPost, systemAPI.APIURLClientNoticeMarkRead, systemAPI.ClientNoticeMarkReadRequest{ID: id, Revision: revision})
	if err != nil {
		return nil, err
	}
	current := sessionByID(sessionID)
	if current == nil || current.historySequence != session.historySequence || current.selectedHistory == nil || current.selectedHistory.id != id || current.selectedHistory.revision != revision {
		return nil, errors.New("公告会话已失效")
	}
	receipt, err := validateReceipt(raw, session.subject, id, revision)
	if err != nil {
		return nil, err
	}
	if !bindReadReceipt(sessionID, session, receipt, true) {
		return nil, fail("stale", "公告历史选择已变化")
	}
	return receipt, nil
}
