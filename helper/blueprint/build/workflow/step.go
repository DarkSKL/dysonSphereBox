package workflow

// 文件职责：签发步骤受限能力并原子提交；使用：引擎调用边界；边界：工具无法获得实例、注册器或调度器。

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

// StepContext 职责：绑定工作区、区域和步骤生命周期；使用：工具取数暂存；边界：任何越权都会毒化事务，过期句柄不可继续使用。
type StepContext struct {
	mu            sync.Mutex
	instance      *BlueprintInstance
	spec          ToolSpec
	region        string
	candidate     int
	path          string
	scope         string
	active        bool
	baseline      uint64
	baselineScope uint64
	reads         map[slotID]uint64
	staged        map[slotID]any
	violation     error
	diagnostics   []string
	regions       []string
	reserve       func(int64) error
	stagedBytes   int64
	stageSizes    map[slotID]int64
}

// Region 职责：返回固定区域身份；使用：工具算法输入；边界：无法切换作用域。
func (s *StepContext) Region() string { return s.region }

// Candidate 职责：返回稳定候选序号；使用：诊断；边界：根工作区为负一。
func (s *StepContext) Candidate() int { return s.candidate }

// Node 职责：返回调用路径；使用：诊断；边界：不是可跳转地址。
func (s *StepContext) Node() string { return s.path }

// InstanceID 职责：返回固定主任务身份；使用：工具诊断及预览路由；边界：候选只是内部工作区，不能切换任务。
func (s *StepContext) InstanceID() string { return s.instance.identity.id }

// Regions 职责：复制已固定目标区域；使用：根级完整性校验；边界：区域工具仍不能越权读取邻区。
func (s *StepContext) Regions() []string { return append([]string(nil), s.regions...) }

// FrozenVersion 职责：给编码器提供验证绑定版本；使用：Encode 角色；边界：其他角色或过期句柄拒绝。
func (s *StepContext) FrozenVersion() (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.check(); e != nil {
		return 0, e
	}
	if s.spec.Role != Encode {
		return 0, s.deny("frozen version requires encode role")
	}
	return s.baseline, nil
}

// Diagnostic 职责：保存步骤诊断；使用：工具进度解释；边界：不修改领域版本，过期后拒绝。
func (s *StepContext) Diagnostic(message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.check(); e != nil {
		return e
	}
	s.diagnostics = append(s.diagnostics, message)
	return nil
}

// deny 职责：记录不可忽略的权限故障；使用：受限 API；边界：工具忽略返回值也不能提交。
func (s *StepContext) deny(message string) error {
	e := fmt.Errorf("%s: %s", s.path, message)
	if s.violation == nil {
		s.violation = e
	}
	return e
}

// check 职责：验证生命周期；使用：每次能力调用；边界：已结束步骤永远不可复活。
func (s *StepContext) check() error {
	if !s.active {
		return fmt.Errorf("%s: expired step handle", s.path)
	}
	if atomic.LoadUint64(&s.instance.scopeEpoch) != s.baselineScope {
		return s.deny("expired region scope epoch")
	}
	if s.violation != nil {
		return s.violation
	}
	return nil
}

// Read 职责：读取固定作用域的类型化副本；使用：工具；边界：不返回实例原始 map/slice/指针。
func Read[T any](s *StepContext, key ArtifactKey[T]) (T, error) {
	var zero T
	if s == nil || key.def == nil {
		return zero, fmt.Errorf("invalid step or key")
	}
	region := ""
	if key.def.options.Regional {
		region = s.region
	}
	return readAt(s, key, region, false)
}

// ReadRegion 职责：读取明确目标区域副本；使用：根工具声明 AllRegions 后汇合；边界：区域工具不能越区、未知区域不能读取。
func ReadRegion[T any](s *StepContext, key ArtifactKey[T], region string) (T, error) {
	return readAt(s, key, region, true)
}

// readAt 职责：执行权限、版本与克隆检查；使用：泛型读取入口；边界：暂存值不构成前置阶段的正式产物。
func readAt[T any](s *StepContext, key ArtifactKey[T], region string, all bool) (T, error) {
	var zero T
	if s == nil || key.def == nil {
		return zero, fmt.Errorf("invalid step or key")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.check(); e != nil {
		return zero, e
	}
	if all {
		if s.region != "" || !key.def.options.Regional || !contains(s.spec.AllRegions, key) || !hasRegion(s.regions, region) {
			return zero, s.deny("unauthorized region read")
		}
	} else if !contains(s.spec.Reads, key) || key.def.options.Regional && region == "" {
		return zero, s.deny("unauthorized artifact read " + key.def.name)
	}
	i := s.instance
	i.mu.Lock()
	defer i.mu.Unlock()
	if atomic.LoadUint64(&i.scopeEpoch) != s.baselineScope {
		return zero, s.deny("expired region scope epoch")
	}
	id := slotID{key.def, region}
	v, ok := i.slots[id]
	if !ok || v.consumed {
		return zero, s.deny("missing or invalid artifact " + key.def.name)
	}
	x, e := safeClone(key.def, v.value)
	if e != nil {
		return zero, s.deny(e.Error())
	}
	s.reads[id] = v.version
	return x.(T), nil
}

// Stage 职责：克隆领域结果到步骤暂存区；使用：工具成功前；边界：仅固定作用域，最终提交由引擎完成。
func Stage[T any](s *StepContext, key ArtifactKey[T], value T) error {
	if s == nil || key.def == nil {
		return fmt.Errorf("invalid step or key")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.check(); e != nil {
		return e
	}
	if !contains(s.spec.Writes, key) {
		return s.deny("unauthorized artifact write " + key.def.name)
	}
	region := ""
	if key.def.options.Regional {
		region = s.region
		if region == "" {
			return s.deny("root cannot stage regional artifact")
		}
	}
	id := slotID{key.def, region}
	var reserved int64
	if s.reserve != nil {
		n, e := safeSize(key.def, value)
		if e != nil {
			return s.deny(e.Error())
		}
		if n > (int64(^uint64(0)>>1)-256)/2 {
			return s.deny("staging size overflow")
		}
		reserved = 2 * (n + 128)
		if key.def.sharedClone {
			// 暂存和提交持有同一封闭图；仍为整个输出预留一份，既有输入直到原子提交才释放。
			reserved = n + 256
		}
		if e = s.reserve(reserved); e != nil {
			return s.deny(e.Error())
		}
	}
	x, e := safeClone(key.def, value)
	if e != nil {
		if s.reserve != nil {
			_ = s.reserve(-reserved)
		}
		return s.deny(e.Error())
	}
	if s.reserve != nil {
		old := s.stageSizes[id]
		_ = s.reserve(-old)
		s.stagedBytes += reserved - old
		if s.stageSizes == nil {
			s.stageSizes = map[slotID]int64{}
		}
		s.stageSizes[id] = reserved
	}
	s.staged[id] = x
	return nil
}

// hasRegion 职责：核对目标集合；使用：区域读取；边界：不接受未遍历的任意区域。
func hasRegion(regions []string, want string) bool {
	for _, r := range regions {
		if r == want {
			return true
		}
	}
	return false
}

// beginStep 职责：冻结基线并核对前置产物；使用：引擎；边界：不满足时不调用工具。
func beginStep(i *BlueprintInstance, spec ToolSpec, region string, candidate int, path string) (*StepContext, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	s := &StepContext{instance: i, spec: spec, region: region, candidate: candidate, path: path, active: true, baseline: i.revision, reads: map[slotID]uint64{}, staged: map[slotID]any{}, regions: append([]string(nil), i.regions...)}
	s.baselineScope = atomic.LoadUint64(&i.scopeEpoch)
	if spec.Role == Encode && !i.validationValid("") {
		return nil, fmt.Errorf("encoding requires validated frozen revision")
	}
	for _, a := range spec.Requires {
		d := a.artifact()
		ids := []slotID{{d, ""}}
		if d.options.Regional {
			if region != "" {
				ids = []slotID{{d, region}}
			} else {
				if !contains(spec.AllRegions, a) || len(i.regions) == 0 {
					return nil, fmt.Errorf("requires all-region capability for %s", d.name)
				}
				ids = nil
				for _, r := range i.regions {
					ids = append(ids, slotID{d, r})
				}
			}
		}
		for _, id := range ids {
			v, ok := i.slots[id]
			if !ok || v.consumed {
				return nil, fmt.Errorf("missing or invalid artifact %s region %s", d.name, id.region)
			}
			s.reads[id] = v.version
		}
	}
	return s, nil
}

// expire 职责：撤销所有步骤能力；使用：失败或调用结束；边界：保留诊断但不能再次读写。
func (s *StepContext) expire() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = false
	if s.reserve != nil && s.stagedBytes > 0 {
		_ = s.reserve(-s.stagedBytes)
		s.stagedBytes = 0
	}
}

// commit 职责：校验基线并原子发布全部产物；使用：工具成功后；边界：取消、权限错误、克隆故障及预算失败均零发布。
func (s *StepContext) commit(ctx context.Context, checkSize func(int64) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer func() { s.active = false }()
	if e := s.check(); e != nil {
		return e
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	i := s.instance
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.revision != s.baseline {
		return fmt.Errorf("content baseline changed")
	}
	if atomic.LoadUint64(&i.scopeEpoch) != s.baselineScope {
		return fmt.Errorf("region scope baseline changed")
	}
	for id, v := range s.reads {
		if current, ok := i.slots[id]; !ok || current.consumed || current.version != v {
			return fmt.Errorf("read baseline changed for %s", id.def.name)
		}
	}
	for _, a := range s.spec.Provides {
		region := ""
		if a.artifact().options.Regional {
			region = s.region
		}
		if _, ok := s.staged[slotID{a.artifact(), region}]; !ok {
			return fmt.Errorf("declared artifact %s was not staged", a.artifact().name)
		}
	}
	next := map[slotID]slot{}
	for id, v := range i.slots {
		next[id] = v
	}
	serial := i.serial
	changed := false
	for id, value := range s.staged {
		if !i.defs[id.def] {
			return fmt.Errorf("artifact not registered in instance")
		}
		x, e := safeClone(id.def, value)
		if e != nil {
			return e
		}
		serial++
		deps := copyDeps(s.reads)
		delete(deps, id)
		next[id] = slot{value: x, version: serial, deps: deps}
		if !id.def.options.Diagnostic {
			changed = true
		}
	}
	// 同一事务共同发布的结果依赖新版本，而不是共同输入的旧版本。
	for id := range s.staged {
		v := next[id]
		for dep := range v.deps {
			if _, written := s.staged[dep]; written {
				v.deps[dep] = next[dep].version
			}
		}
		next[id] = v
	}
	for _, a := range s.spec.Consumes {
		if !contains(s.spec.Requires, a) || (!contains(s.spec.Reads, a) && !contains(s.spec.AllRegions, a)) || contains(s.spec.Provides, a) || contains(s.spec.Invalidates, a) {
			return fmt.Errorf("invalid consume capability")
		}
		found := false
		for id := range s.reads {
			if id.def == a.artifact() {
				value, ok := next[id]
				if !ok || value.consumed {
					return fmt.Errorf("artifact %s already consumed", id.def.name)
				}
				value.value = nil
				value.consumed = true
				next[id] = value
				found = true
			}
		}
		if !found {
			return fmt.Errorf("consumed artifact %s was not required in scope", a.artifact().name)
		}
	}
	shadow := &BlueprintInstance{slots: next}
	explicit := map[slotID]bool{}
	for _, a := range s.spec.Invalidates {
		d := a.artifact()
		for id := range next {
			if id.def == d && (!d.options.Regional || s.region == "" || id.region == s.region) {
				if _, written := s.staged[id]; !written {
					explicit[id] = true
					if !d.options.Diagnostic {
						changed = true
					}
				}
			}
		}
	}
	shadow.invalidate(explicit)
	for _, a := range s.spec.Provides {
		region := ""
		if a.artifact().options.Regional {
			region = s.region
		}
		if value, ok := next[slotID{a.artifact(), region}]; !ok || value.consumed {
			return fmt.Errorf("provided artifact %s invalidated during commit", a.artifact().name)
		}
	}
	// 逻辑流：候选新状态先完整计算与计量，再核对取消，最后一次替换正式槽位。
	if checkSize != nil {
		n, e := stateSize(next)
		if e != nil {
			return e
		}
		if e = checkSize(n); e != nil {
			return e
		}
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	i.slots = next
	i.serial = serial
	if changed {
		i.revision++
		i.hasValidation = false
		i.origin = resultOrigin{}
	}
	if s.spec.Role == Validate {
		evidence := copyDeps(s.reads)
		for id := range evidence {
			if current, ok := next[id]; ok {
				evidence[id] = current.version
			}
		}
		if s.region == "" {
			i.validated = i.revision
			i.hasValidation = true
			i.validationDeps = evidence
			if s.candidate >= 0 && s.scope != "" {
				i.origin = resultOrigin{s.scope, s.candidate}
			} else if i.origin.path == "" {
				i.origin = resultOrigin{s.path, 0}
			}
		} else {
			if i.regionalValidation == nil {
				i.regionalValidation = map[string]uint64{}
			}
			i.regionalValidation[s.region] = i.revision
			if i.regionalValidationDeps == nil {
				i.regionalValidationDeps = map[string]map[slotID]uint64{}
			}
			i.regionalValidationDeps[s.region] = evidence
		}
	}
	return nil
}
