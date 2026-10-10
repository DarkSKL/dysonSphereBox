package diagnostics

// 文件职责：维护不依赖数据库的结构化日志文件与采集健康状态。使用：main 在 Wails 和数据库之前初始化；边界：同步文件写入，数据库只作为可重建索引。
// 逻辑流：初始化目录与会话 => 校验脱敏 => 追加 JSONL => 通知索引 => 轮转；写入失败保留内存证据并输出 stderr。

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"DysonSphereBox/constant/logging"
)

// namespacePattern 职责：约束域.模块.事件命名；使用：统一入口校验；边界：允许事件进一步分层。
var namespacePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*(\.[A-Za-z][A-Za-z0-9_-]*){2,}$`)

// Store 职责：串行维护文件和健康状态；使用：进程级单例或隔离测试；边界：索引回调不得持有业务事务。
type Store struct {
	mu          sync.Mutex
	dir         string
	file        *os.File
	size        int64
	sequence    int
	session     string
	version     string
	lastError   string
	fallback    bool
	memory      []logging.Entry
	dropped     int64
	sink        func(logging.Entry)
	repeated    map[string]*repeatedEntry
	repeatTimer *time.Timer
	noiseCount  uint64
}

// Health 职责：暴露采集状态；使用：日志页面和诊断包；边界：错误清除前不会报告健康。
type Health struct {
	// Directory 职责：实际目录；使用：打开日志；边界：可能为后备目录。
	Directory string `json:"directory"`
	// SessionID 职责：当前会话；使用：筛选；边界：启动即生成。
	SessionID string `json:"sessionId"`
	// FileError 职责：最近写入错误；使用：降级告警；边界：不吞掉错误。
	FileError string `json:"fileError"`
	// Fallback 职责：标识后备目录；使用：路径诊断；边界：不表示文件丢失。
	Fallback bool `json:"fallback"`
	// MemoryCount 职责：未落盘条数；使用：识别证据缺失风险；边界：内存最多 500 条。
	MemoryCount int `json:"memoryCount"`
	// Dropped 职责：记录内存证据溢出数；使用：采集告警；边界：不会把被丢弃记录视为已落盘。
	Dropped int64 `json:"dropped"`
	// NoiseSuppressed 职责：本次进程过滤的噪声条数；使用：健康详情；边界：不包含历史索引清理数量。
	NoiseSuppressed uint64 `json:"noiseSuppressed"`
}

// Default 职责：保存进程日志设施；使用：基础设施及业务 helper；边界：未初始化时仅保留内存和 stderr。
var Default = &Store{session: NewID(), version: "unknown"}

// NewID 职责：生成无业务信息的记录号；使用：会话及操作关联；边界：随机源失败时仍可继续记录。
func NewID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err == nil {
		return hex.EncodeToString(b)
	}
	return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
}

// Open 职责：建立独立文件采集；使用：启动与隔离测试；边界：目录不可用时返回错误和可使用的内存采集器。
func Open(dir, version string) (*Store, error) {
	s := &Store{dir: dir, session: NewID(), version: version}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(dir, 0700); err != nil {
		s.lastError = err.Error()
		return s, err
	}
	if err := s.openFile(); err != nil {
		s.lastError = err.Error()
		return s, err
	}
	return s, nil
}

// Initialize 职责：首选 exe 同级目录并验证后备目录；使用：main 入口；边界：失败也保留内存记录，不能依赖数据库。
func Initialize(version string) error {
	exe, err := os.Executable()
	if err == nil {
		Default, err = Open(filepath.Join(filepath.Dir(exe), "logs"), version)
	}
	if err == nil {
		return nil
	}
	primaryError := err
	base, fallbackErr := os.UserConfigDir()
	if fallbackErr == nil {
		Default, fallbackErr = Open(filepath.Join(base, "DysonSphereBox", "logs"), version)
		Default.fallback = true
	}
	Default.Record(logging.Entry{Action: "初始化日志目录", Level: logging.Warn, Namespace: "system.logging.fallback", Source: "backend.logging", Reason: "首选日志目录不可写，尝试后备目录", Handler: logging.System, Data: map[string]interface{}{"error": ErrorData(primaryError), "fallbackError": fmt.Sprint(fallbackErr)}})
	return fallbackErr
}

// Validate 职责：拒绝不明确的日志契约；使用：前端提交和内部写入；边界：不填充伪造业务原因。
func Validate(e logging.Entry) error {
	if strings.TrimSpace(e.Action) == "" || strings.TrimSpace(e.Source) == "" || strings.TrimSpace(e.Reason) == "" {
		return fmt.Errorf("action、source、reason 不能为空")
	}
	if !namespacePattern.MatchString(e.Namespace) {
		return fmt.Errorf("namespace 必须是域.模块.事件")
	}
	switch e.Level {
	case logging.Debug, logging.Info, logging.Warn, logging.Error, logging.Fatal:
	default:
		return fmt.Errorf("无效日志等级: %s", e.Level)
	}
	switch e.Handler {
	case logging.User, logging.System, logging.Unknown:
	default:
		return fmt.Errorf("无效触发者: %s", e.Handler)
	}
	if e.Time != "" {
		if _, err := time.Parse(time.RFC3339Nano, e.Time); err != nil {
			return fmt.Errorf("无效日志时间: %w", err)
		}
	}
	return nil
}

// Record 职责：尽力记录且不改变业务结果；使用：业务埋点；边界：错误可通过 Health 查询并输出 stderr。
func (s *Store) Record(e logging.Entry) {
	if err := s.Write(e); err != nil {
		fmt.Fprintln(os.Stderr, "diagnostics:", CleanText(err.Error()))
	}
}

// Write 职责：写入统一日志；使用：需要感知落盘失败的入口；边界：保留失败记录于有界内存，错误和 Fatal 同步刷盘。
func (s *Store) Write(e logging.Entry) error {
	return s.write(e, true)
}

// RecordEvidence 职责：只写独立证据；使用：数据库日志回调；边界：禁止再次触发索引以避免递归故障。
func (s *Store) RecordEvidence(e logging.Entry) {
	if err := s.write(e, false); err != nil {
		fmt.Fprintln(os.Stderr, "diagnostics:", CleanText(err.Error()))
	}
}

// write 职责：执行串行落盘与可选索引通知；使用：统一入口；边界：index=false 适用于索引自身错误。
func (s *Store) write(e logging.Entry, index bool) error {
	if e.Level == "" {
		e.Level = logging.Info
	}
	if e.Handler == "" {
		e.Handler = logging.Unknown
	}
	if err := Validate(e); err != nil {
		return err
	}
	if IsNoise(e) {
		s.mu.Lock()
		s.noiseCount++
		s.mu.Unlock()
		return nil
	}
	e.Action, e.Reason, e.Source = CleanText(e.Action), CleanText(e.Reason), CleanText(e.Source)
	e.Data = CleanData(e.Data)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.coalesceLocked(e) {
		return nil
	}
	if e.ID == "" {
		e.ID = NewID()
	}
	err := s.writeLocked(e, index)
	if err == nil {
		s.rememberLocked(e, index)
	}
	return err
}

// writeLocked 职责：追加单条证据；使用：普通写入与重复摘要；边界：调用方持有 Store 锁，不能递归采集。
func (s *Store) writeLocked(e logging.Entry, index bool) error {
	if e.ID == "" {
		e.ID = NewID()
	}
	e.SchemaVersion = 2
	if e.SessionID == "" {
		e.SessionID = s.session
	}
	if e.AppVersion == "" {
		e.AppVersion = s.version
	}
	if e.Time == "" {
		e.Time = time.Now().Format(time.RFC3339Nano)
	}
	e.Action, e.Reason, e.Source = CleanText(e.Action), CleanText(e.Reason), CleanText(e.Source)
	e.Data = CleanData(e.Data)
	b, err := json.Marshal(e)
	if err == nil && len(b) > 256*1024 {
		e.Data = map[string]interface{}{"truncated": true, "originalBytes": len(b), "error": e.Data["error"], "result": e.Data["result"]}
		b, err = json.Marshal(e)
	}
	if err == nil && s.file == nil {
		err = fmt.Errorf("日志文件未打开")
	}
	if err == nil && s.file != nil && s.size+int64(len(b)+1) > 8*1024*1024 {
		err = s.rotate()
	}
	if err == nil && s.file != nil {
		var n int
		n, err = s.file.Write(append(b, '\n'))
		s.size += int64(n)
		if err == nil && (e.Level == logging.Error || e.Level == logging.Fatal) {
			err = s.file.Sync()
		}
	}
	if err != nil {
		s.lastError = CleanText(err.Error())
		s.memory = append(s.memory, e)
		if len(s.memory) > 500 {
			s.dropped += int64(len(s.memory) - 500)
			s.memory = s.memory[len(s.memory)-500:]
		}
	} else {
		s.lastError = ""
	}
	if err == nil && index && s.sink != nil {
		s.sink(e)
	}
	return err
}

// openFile 职责：打开会话分片；使用：初始化和轮转；边界：不覆盖旧文件。
func (s *Store) openFile() error {
	name := fmt.Sprintf("client-%s-%s-%03d.jsonl", time.Now().Format("20060102T150405"), s.session, s.sequence)
	f, err := os.OpenFile(filepath.Join(s.dir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	s.file, s.size = f, 0
	return s.prune()
}

// rotate 职责：关闭分片并开启下一片；使用：大小超限；边界：失败后不写入已关闭句柄。
func (s *Store) rotate() error {
	if s.file != nil {
		if err := s.file.Close(); err != nil {
			s.file = nil
			return err
		}
		s.file = nil
	}
	s.sequence++
	return s.openFile()
}

// prune 职责：控制日志保留；使用：打开分片后；边界：只清理自身 client-*.jsonl，保留最近十片且不删当前片。
func (s *Store) prune() error {
	paths, err := filepath.Glob(filepath.Join(s.dir, "client-*.jsonl"))
	if err != nil {
		return err
	}
	sort.Strings(paths)
	for i, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if path != s.file.Name() && (i < len(paths)-10 || time.Since(info.ModTime()) > 30*24*time.Hour) {
			if err := os.Remove(path); err != nil {
				return err
			}
		}
	}
	return nil
}

// SetSink 职责：挂接索引通知；使用：数据库就绪后；边界：回调必须非阻塞，不能再次调用 Store。
func (s *Store) SetSink(sink func(logging.Entry)) { s.mu.Lock(); defer s.mu.Unlock(); s.sink = sink }

// Health 职责：读取采集健康快照；使用：页面和导出；边界：不泄漏原始敏感错误。
func (s *Store) Health() Health {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Health{Directory: s.dir, SessionID: s.session, FileError: s.lastError, Fallback: s.fallback, MemoryCount: len(s.memory), Dropped: s.dropped, NoiseSuppressed: s.noiseCount}
}

// Close 职责：刷新并关闭采集；使用：主进程退出；边界：文件关闭后仍可使用内存降级。
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.repeatTimer != nil {
		s.repeatTimer.Stop()
		s.repeatTimer = nil
	}
	for _, item := range s.repeated {
		s.flushRepeatLocked(item)
	}
	s.repeated = nil
	if s.file == nil {
		return nil
	}
	err := s.file.Sync()
	closeErr := s.file.Close()
	s.file = nil
	if err != nil {
		return err
	}
	return closeErr
}
