package diagnostics

// 文件职责：合并短时间重复的基础设施故障并保留次数与时间范围。使用：Store 写入和关闭；边界：首条实时落盘，合并不影响业务执行和采集健康。

import (
	"DysonSphereBox/constant/logging"
	"fmt"
	"sort"
	"strings"
	"time"
)

// repeatWindow 职责：限制重复故障摘要延迟；使用：周期刷新；边界：关闭与恢复会提前刷新。
const repeatWindow = 30 * time.Second

// repeatedEntry 职责：保存同一故障的首条证据及后续计数；使用：Store 锁内；边界：最多保留 128 个指纹。
type repeatedEntry struct {
	entry logging.Entry
	index bool
	count int
	// total 职责：统计故障首次与重复发生的总次数；使用：摘要和恢复；边界：刷新摘要不会重置。
	total int
	first time.Time
	last  time.Time
}

// coalesceLocked 职责：识别短时间重复；使用：写入前持有 Store 锁；边界：新故障先写入成功后再注册，失败不被合并掩盖。
func (s *Store) coalesceLocked(e logging.Entry) bool {
	key := repeatKey(e)
	if key == "" || s.repeated[key] == nil {
		return false
	}
	item := s.repeated[key]
	if time.Since(item.last) >= repeatWindow {
		s.flushRepeatLocked(item)
		delete(s.repeated, key)
		return false
	}
	item.count++
	item.total++
	item.last = time.Now()
	return true
}

// rememberLocked 职责：跟踪已成功写入的自动故障；使用：持有 Store 锁；边界：指纹超过上限时刷新最旧证据再回收。
func (s *Store) rememberLocked(e logging.Entry, index bool) {
	key := repeatKey(e)
	if key == "" {
		return
	}
	if s.repeated == nil {
		s.repeated = map[string]*repeatedEntry{}
	}
	if len(s.repeated) >= 128 {
		oldestKey := ""
		for candidate, item := range s.repeated {
			if oldestKey == "" || item.first.Before(s.repeated[oldestKey].first) {
				oldestKey = candidate
			}
		}
		s.flushRepeatLocked(s.repeated[oldestKey])
		delete(s.repeated, oldestKey)
	}
	now := time.Now()
	s.repeated[key] = &repeatedEntry{entry: e, index: index, total: 1, first: now, last: now}
	if s.repeatTimer == nil {
		s.repeatTimer = time.AfterFunc(repeatWindow, s.FlushRepeats)
	}
}

// flushRepeatLocked 职责：输出重复次数与范围；使用：周期、恢复和关闭；边界：不重新进入合并，原错误与定位字段完整保留。
func (s *Store) flushRepeatLocked(item *repeatedEntry) {
	if item == nil || item.count == 0 {
		return
	}
	e := item.entry
	e.ID, e.Time = "", ""
	e.Action += fmt.Sprintf("（相同故障另发生 %d 次）", item.count)
	e.Data = make(map[string]interface{}, len(item.entry.Data)+1)
	for key, value := range item.entry.Data {
		e.Data[key] = value
	}
	data := map[string]interface{}{"additionalCount": item.count, "totalCount": item.total, "firstRecordId": item.entry.ID, "firstSeen": item.first.Format(time.RFC3339Nano), "lastSeen": item.last.Format(time.RFC3339Nano)}
	e.Data["repeat"] = data
	if item.entry.Source == "backend.http" {
		e.Reason = fmt.Sprintf("%s（相同请求另失败 %d 次）", item.entry.Reason, item.count)
	}
	// 落盘失败时 writeLocked 已将摘要放入降级内存；清空窗口计数避免下一轮重复缓存同一份证据。
	s.writeLocked(e, item.index)
	item.count = 0
}

// FlushRepeats 职责：刷新合并证据；使用：计时器和索引停止前；边界：持续故障每 30 秒一条摘要，空闲指纹到期释放。
func (s *Store) FlushRepeats() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.repeatTimer != nil {
		s.repeatTimer.Stop()
		s.repeatTimer = nil
	}
	for key, item := range s.repeated {
		s.flushRepeatLocked(item)
		if item.count == 0 && time.Since(item.last) >= repeatWindow {
			delete(s.repeated, key)
		}
	}
	if len(s.repeated) > 0 {
		s.repeatTimer = time.AfterFunc(repeatWindow, s.FlushRepeats)
	}
}

// ResolveNetwork 职责：成功响应后记录故障恢复；使用：HTTP 传输；边界：只匹配相同方法与脱敏地址的失败，不把普通成功请求记为日志。
func (s *Store) ResolveNetwork(method, address string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	previousIDs := []string{}
	failureCount := 0
	for key, item := range s.repeated {
		e := item.entry
		baseURL := strings.TrimSpace(address)
		if cut := strings.IndexAny(baseURL, "?#"); cut >= 0 {
			baseURL = baseURL[:cut]
		}
		failureURL, _ := e.Data["url"].(string)
		if cut := strings.IndexAny(failureURL, "?#"); cut >= 0 {
			failureURL = failureURL[:cut]
		}
		if e.Source == "backend.http" && e.Data["method"] == method && failureURL == CleanText(baseURL) && e.Data["result"] == "failed" {
			s.flushRepeatLocked(item)
			previousIDs = append(previousIDs, item.entry.ID)
			failureCount += item.total
			delete(s.repeated, key)
		}
	}
	if len(previousIDs) > 0 {
		sort.Strings(previousIDs)
		s.writeLocked(logging.Entry{Action: "远端请求恢复", Level: logging.Info, Source: "backend.http", Reason: "此前失败的接口再次返回成功状态", Namespace: "system.network.recovered", Handler: logging.System, Data: map[string]interface{}{"method": method, "url": CleanText(address), "result": "recovered", "previousRecordIds": previousIDs, "failureCount": failureCount}}, true)
	}
}
