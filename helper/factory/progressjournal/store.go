package progressjournal

import (
	"DysonSphereBox/helper/blueprint/build"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

const maxBytes = 64 << 20
const lifetime = 10 * time.Minute

// Page 职责：承载一页不可变过程事件；使用：断流补发；边界：日志缺失必须显式标记，不能冒充完整过程。
type Page struct {
	Events     []build.Progress `json:"events"`
	Next       uint64           `json:"next"`
	Complete   bool             `json:"complete"`
	Incomplete bool             `json:"incomplete"`
	Message    string           `json:"message"`
}

type entry struct {
	file       *os.File
	offsets    []int64
	size       int64
	complete   bool
	incomplete bool
	timer      *time.Timer
}

// Store 职责：缓存本服务最近两次任务的有界磁盘事件日志；使用：Generate 与分页读取；边界：只访问本实例创建的临时文件，零值可用。
type Store struct {
	mu      sync.Mutex
	entries map[string]*entry
	order   []string
}

// Begin 职责：登记本次过程日志；使用：已获得生成执行槽后；边界：文件失败只关闭过程恢复，不改变蓝图计算。
func (s *Store) Begin(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.entries == nil {
		s.entries = make(map[string]*entry)
	}
	if old := s.entries[taskID]; old != nil {
		s.remove(taskID)
		for index, id := range s.order {
			if id == taskID {
				s.order = append(s.order[:index], s.order[index+1:]...)
				break
			}
		}
	}
	file, err := os.CreateTemp("", "dsbox-generation-progress-*.jsonl")
	value := &entry{file: file, incomplete: err != nil}
	s.entries[taskID] = value
	s.order = append(s.order, taskID)
	for len(s.order) > 2 {
		old := s.order[0]
		s.order = s.order[1:]
		s.remove(old)
	}
	value.timer = time.AfterFunc(lifetime, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.entries[taskID] == value {
			s.remove(taskID)
		}
	})
}

func (s *Store) remove(taskID string) {
	value := s.entries[taskID]
	if value == nil {
		return
	}
	if value.timer != nil {
		value.timer.Stop()
	}
	if value.file != nil {
		name := value.file.Name()
		value.file.Close()
		os.Remove(name)
	}
	delete(s.entries, taskID)
}

// Append 职责：在实时发送前归档同序号值快照；使用：单任务观察者；边界：超出容量后显式标记不完整，不阻止正式生成。
func (s *Store) Append(event build.Progress) {
	raw, err := json.Marshal(event)
	s.mu.Lock()
	defer s.mu.Unlock()
	value := s.entries[event.TaskID]
	if value == nil || value.incomplete {
		return
	}
	if err != nil || event.Sequence != uint64(len(value.offsets)+1) || value.size+int64(len(raw)+1) > maxBytes {
		value.incomplete = true
		return
	}
	if _, err = value.file.Write(append(raw, '\n')); err != nil {
		value.incomplete = true
		return
	}
	value.offsets = append(value.offsets, value.size)
	value.size += int64(len(raw) + 1)
}

// Finish 职责：标记所有 worker 与事件汇聚已经退出；使用：Generate 收尾；边界：不等同于计算成功。
func (s *Store) Finish(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if value := s.entries[taskID]; value != nil {
		value.complete = true
	}
}

// Close 职责：释放本实例全部过程临时文件与过期计时器；使用：应用退出及验证收尾；边界：不删除未登记路径。
func (s *Store) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id := range s.entries {
		s.remove(id)
	}
	s.order = nil
}

// Read 职责：读取指定已处理序号之后的最多十六项事件；使用：前端补发；边界：拒绝任意路径，未知或过期任务明确返回不完整。
func (s *Store) Read(taskID string, after uint64) Page {
	s.mu.Lock()
	defer s.mu.Unlock()
	page := Page{Events: []build.Progress{}, Next: after}
	value := s.entries[taskID]
	if value == nil || value.file == nil {
		page.Incomplete, page.Message = true, "生成过程记录已过期或不可用"
		return page
	}
	if after > uint64(len(value.offsets)) {
		page.Incomplete, page.Message = true, "生成过程序号超出日志范围"
		return page
	}
	end := min(after+16, uint64(len(value.offsets)))
	for index := after; index < end; index++ {
		start, finish := value.offsets[index], value.size
		if index+1 < uint64(len(value.offsets)) {
			finish = value.offsets[index+1]
		}
		raw := make([]byte, finish-start)
		if _, err := value.file.ReadAt(raw, start); err != nil && err != io.EOF {
			page.Incomplete, page.Message = true, fmt.Sprintf("读取过程记录失败：%v", err)
			return page
		}
		var event build.Progress
		if err := json.Unmarshal(raw, &event); err != nil {
			page.Incomplete, page.Message = true, "过程记录校验失败"
			return page
		}
		page.Events = append(page.Events, event)
		page.Next = event.Sequence
	}
	page.Incomplete = value.incomplete
	if page.Incomplete {
		page.Message = "过程记录超出容量或写入失败，无法完整恢复"
	}
	page.Complete = value.complete && end == uint64(len(value.offsets))
	return page
}
