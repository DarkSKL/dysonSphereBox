package workflow

// 文件职责：维护唯一权威实例、克隆边界和版本快照；使用：适配层播种、引擎事务与候选；边界：不公开领域指针及内部集合。

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

// slotID 职责：绑定键和区域；使用：内部查找；边界：不接受任意字段路径。
type slotID struct {
	def    *artifactDef
	region string
}

// slot 职责：保存不可外泄的领域值与依赖证据；使用：原子发布；边界：发布后只读。
type slot struct {
	consumed bool
	value    any
	version  uint64
	deps     map[slotID]uint64
}

// BlueprintInstance 职责：保存一次任务的有类型产物；使用：Run；边界：所有读写经克隆和固定作用域。
type BlueprintInstance struct {
	origin resultOrigin
	// scopeEpoch 职责：撤销区域重绑定前的句柄；使用：原子读取作用域代际；边界：独立于领域内容 Revision。
	scopeEpoch             uint64
	identity               *taskIdentity
	mu                     sync.Mutex
	defs                   map[*artifactDef]bool
	slots                  map[slotID]slot
	revision               uint64
	serial                 uint64
	validated              uint64
	hasValidation          bool
	regionalValidation     map[string]uint64
	validationDeps         map[slotID]uint64
	regionalValidationDeps map[string]map[slotID]uint64
	regions                []string
	running                bool
}

// resultOrigin 职责：保留已验证/选中内容的真实作用域；使用：includeCurrent 的 selected；边界：纯内部身份元数据，不保留领域图。
type resultOrigin struct {
	path    string
	ordinal int
}

// taskIdentity 职责：固定一次任务身份；使用：主实例及私有候选工作区共享；边界：候选克隆不能创建第二个任务身份。
type taskIdentity struct{ id string }

// instanceSequence 职责：生成进程内稳定任务序号；使用：NewInstance；边界：不承载领域状态。
var instanceSequence atomic.Uint64

// ID 职责：返回固定任务身份；使用：预览关联；边界：所有内部候选保持相同值。
func (i *BlueprintInstance) ID() string { return i.identity.id }

// NewInstance 职责：建立空产物目录；使用：任务开始前；边界：拒绝重复名、零键，不接收无类型领域初值。
func NewInstance(keys []Artifact) (*BlueprintInstance, error) {
	i := &BlueprintInstance{identity: &taskIdentity{id: fmt.Sprintf("workflow-%d", instanceSequence.Add(1))}, defs: map[*artifactDef]bool{}, slots: map[slotID]slot{}}
	names := map[string]bool{}
	for _, a := range keys {
		d, e := artifactDefinition(a)
		if e != nil {
			return nil, e
		}
		if names[d.name] {
			return nil, fmt.Errorf("duplicate artifact %s", d.name)
		}
		names[d.name] = true
		i.defs[d] = true
	}
	return i, nil
}

// Seed 职责：复制可信初始领域输入；使用：Run 前由适配层调用；边界：运行期间拒绝变更，后续播种使旧验证失效。
func Seed[T any](i *BlueprintInstance, key ArtifactKey[T], region string, value T) error {
	if i == nil || key.def == nil {
		return fmt.Errorf("invalid instance or key")
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.running {
		return fmt.Errorf("instance is running")
	}
	if !i.defs[key.def] {
		return fmt.Errorf("unknown artifact %s", key.def.name)
	}
	if key.def.options.Regional != (region != "") {
		return fmt.Errorf("artifact %s scope mismatch", key.def.name)
	}
	v, e := safeClone(key.def, value)
	if e != nil {
		return e
	}
	if _, e = safeSize(key.def, v); e != nil {
		return e
	}
	i.serial++
	i.slots[slotID{key.def, region}] = slot{value: v, version: i.serial}
	if !key.def.options.Diagnostic {
		i.revision++
		i.hasValidation = false
		i.origin = resultOrigin{}
	}
	i.invalidate(nil)
	return nil
}

// invalidate 职责：按依赖版本传播失效；使用：播种及提交；边界：不删除仍有效的未变更产物。
func (i *BlueprintInstance) invalidate(explicit map[slotID]bool) {
	for id := range explicit {
		delete(i.slots, id)
	}
	for changed := true; changed; {
		changed = false
		for id, s := range i.slots {
			for dep, v := range s.deps {
				current, ok := i.slots[dep]
				if !ok || current.version != v {
					delete(i.slots, id)
					changed = true
					break
				}
			}
		}
	}
}

// Revision 职责：读取领域内容版本；使用：诊断；边界：报告发布不推进此值。
func (i *BlueprintInstance) Revision() uint64 { i.mu.Lock(); defer i.mu.Unlock(); return i.revision }

// FrozenSnapshot 职责：保存不可变、独立克隆的版本视图；使用：编码及评分；边界：读取再次克隆，不暴露集合。
type FrozenSnapshot struct {
	origin    resultOrigin
	region    string
	identity  *taskIdentity
	slots     map[slotID]slot
	revision  uint64
	validated bool
	regions   []string
}

// Region 职责：返回验证证据的固定作用域；使用：候选预览辨别区域成功；边界：空值才表示整任务验证作用域。
func (s *FrozenSnapshot) Region() string { return s.region }

// InstanceID 职责：读取冻结视图所属任务；使用：比较及预览；边界：候选视图不会改变任务身份。
func (s *FrozenSnapshot) InstanceID() string { return s.identity.id }

// Revision 职责：读取冻结版本；使用：编码证据；边界：不会随实例后续变更。
func (s *FrozenSnapshot) Revision() uint64 { return s.revision }

// Validated 职责：说明快照是否绑定成功验证；使用：输出边界；边界：普通预览快照为 false。
func (s *FrozenSnapshot) Validated() bool { return s.validated }

// Regions 职责：返回固定区域集合副本；使用：跨区读取；边界：调用方修改不影响快照。
func (s *FrozenSnapshot) Regions() []string { return append([]string(nil), s.regions...) }

// SnapshotRead 职责：类型化读取快照副本；使用：输出适配；边界：过期或缺失产物不能读取。
func SnapshotRead[T any](s *FrozenSnapshot, key ArtifactKey[T], region string) (T, error) {
	var zero T
	if s == nil || key.def == nil {
		return zero, fmt.Errorf("invalid snapshot or key")
	}
	v, ok := s.slots[slotID{key.def, region}]
	if !ok || v.consumed {
		return zero, fmt.Errorf("artifact unavailable")
	}
	x, e := safeClone(key.def, v.value)
	if e != nil {
		return zero, e
	}
	return x.(T), nil
}

// snapshotLocked 职责：完整克隆权威状态；使用：冻结和候选复制；边界：任一领域克隆失败则整个快照失败。
func (i *BlueprintInstance) snapshotLocked(validated bool) (*FrozenSnapshot, error) {
	s := &FrozenSnapshot{origin: i.origin, identity: i.identity, slots: map[slotID]slot{}, revision: i.revision, validated: validated, regions: append([]string(nil), i.regions...)}
	for id, v := range i.slots {
		if !v.consumed {
			x, e := safeClone(id.def, v.value)
			if e != nil {
				return nil, e
			}
			v.value = x
		}
		v.deps = copyDeps(v.deps)
		s.slots[id] = v
	}
	return s, nil
}

// Freeze 职责：取得与最终验证同版本的冻结快照；使用：成功编码；边界：没有验证或内容变化后一律拒绝。
func (i *BlueprintInstance) Freeze() (*FrozenSnapshot, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if !i.validationValid("") {
		return nil, fmt.Errorf("instance revision %d is not validated", i.revision)
	}
	return i.snapshotLocked(true)
}

// freezeFor 职责：核对候选固定作用域验证；使用：区域内候选评分；边界：区域通过不能授权根级编码。
func (i *BlueprintInstance) freezeFor(region string) (*FrozenSnapshot, error) {
	if region == "" {
		return i.Freeze()
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if !i.validationValid(region) {
		return nil, fmt.Errorf("region %s is not validated at current revision", region)
	}
	s, e := i.snapshotLocked(true)
	if e == nil {
		s.region = region
	}
	return s, e
}

// Snapshot 职责：产生独立预览快照；使用：失败诊断；边界：不证明可以编码。
func (i *BlueprintInstance) Snapshot() (*FrozenSnapshot, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.snapshotLocked(i.validationValid(""))
}

// copyDeps 职责：复制版本证据；使用：事务快照；边界：内部 map 不共享可变别名。
func copyDeps(src map[slotID]uint64) map[slotID]uint64 {
	out := map[slotID]uint64{}
	for k, v := range src {
		out[k] = v
	}
	return out
}

// clone 职责：创建隔离候选工作区；使用：引擎；边界：完整复制所有产物，不能仅复制部分建筑。
func (i *BlueprintInstance) clone() (*BlueprintInstance, int64, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	s, e := i.snapshotLocked(i.validationValid(""))
	if e != nil {
		return nil, 0, e
	}
	n, e := stateSize(s.slots)
	if e != nil {
		return nil, 0, e
	}
	defs := map[*artifactDef]bool{}
	for d := range i.defs {
		defs[d] = true
	}
	validation := map[string]uint64{}
	for region, version := range i.regionalValidation {
		validation[region] = version
	}
	regionDeps := map[string]map[slotID]uint64{}
	for region, deps := range i.regionalValidationDeps {
		regionDeps[region] = copyDeps(deps)
	}
	return &BlueprintInstance{origin: i.origin, scopeEpoch: atomic.LoadUint64(&i.scopeEpoch), identity: i.identity, defs: defs, slots: s.slots, revision: i.revision, serial: i.serial, validated: i.validated, hasValidation: i.hasValidation, regions: s.regions, regionalValidation: validation, validationDeps: copyDeps(i.validationDeps), regionalValidationDeps: regionDeps}, n, nil
}

// stateSize 职责：计算保守候选保留体积；使用：预算；边界：大小器故障不允许继续搜索。
func stateSize(slots map[slotID]slot) (int64, error) {
	return stateStorageSize(slots, false)
}

// stateCloneSize 职责：估算已有计量所有者存活时的额外快照空间；使用：候选工作区旁的比较快照。
// 边界：只省略显式不可变共享图，可变领域值和管理容器仍完整计量；不能用于独立存活的工作区。
func stateCloneSize(slots map[slotID]slot) (int64, error) {
	return stateStorageSize(slots, true)
}

// stateStorageSize 职责：统一计算完整保留量或依附所有者的克隆增量；使用：预算入口；边界：溢出及领域计量错误均拒绝。
func stateStorageSize(slots map[slotID]slot, cloneOnly bool) (int64, error) {
	// 固定容器和每槽管理开销纳入估算，领域大小器只负责自身保留对象。
	total := int64(128) + int64(len(slots))*128
	for id, s := range slots {
		if s.consumed || cloneOnly && id.def.sharedClone {
			continue
		}
		n, e := safeSize(id.def, s.value)
		if e != nil {
			return 0, e
		}
		if n > int64(^uint64(0)>>1)-total {
			return 0, fmt.Errorf("state size overflow")
		}
		total += n
	}
	return total, nil
}

// adopt 职责：完整转移停止执行的获选工作区；使用：确定性归约；边界：只转移同任务私有工作区，持有发布锁后再次核对取消，领域值随后只由主实例拥有。
func (i *BlueprintInstance) adopt(ctx context.Context, candidate *BlueprintInstance) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if candidate == i {
		return nil
	}
	if candidate.identity != i.identity {
		return fmt.Errorf("candidate belongs to another task")
	}
	candidate.mu.Lock()
	slots := make(map[slotID]slot, len(candidate.slots))
	for id, value := range candidate.slots {
		value.deps = copyDeps(value.deps)
		slots[id] = value
	}
	revision, serial, validated, hasValidation := candidate.revision, candidate.serial, candidate.validated, candidate.validationValid("")
	epoch := atomic.LoadUint64(&candidate.scopeEpoch)
	origin := candidate.origin
	regions := append([]string(nil), candidate.regions...)
	validation := map[string]uint64{}
	for region, version := range candidate.regionalValidation {
		validation[region] = version
	}
	deps := copyDeps(candidate.validationDeps)
	regionDeps := map[string]map[slotID]uint64{}
	for region, values := range candidate.regionalValidationDeps {
		regionDeps[region] = copyDeps(values)
	}
	candidate.mu.Unlock()
	i.mu.Lock()
	defer i.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	i.slots = slots
	i.revision = revision
	i.serial = serial
	i.hasValidation = hasValidation
	i.validated = validated
	i.regions = regions
	i.regionalValidation = validation
	i.validationDeps = deps
	i.regionalValidationDeps = regionDeps
	i.origin = origin
	atomic.StoreUint64(&i.scopeEpoch, epoch)
	return nil
}

// bindRegions 职责：在完整性检查后固定或安全重绑定目标集合；使用：显式 regions 节点；边界：旧区域槽仍有效或旧验证绑定当前内容时不能重置，不推进内容版本。
func (i *BlueprintInstance) bindRegions(regions []string, reset bool) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if sameStrings(i.regions, regions) {
		return nil
	}
	if len(i.regions) > 0 {
		if !reset {
			return fmt.Errorf("target region set changed")
		}
		for id := range i.slots {
			if id.def.options.Regional {
				return fmt.Errorf("region reset requires all previous regional artifacts invalidated: %s", id.def.name)
			}
		}
		for region, version := range i.regionalValidation {
			if version == i.revision {
				return fmt.Errorf("region reset retains current validation for %s", region)
			}
		}
	}
	i.regions = append([]string(nil), regions...)
	i.regionalValidation = map[string]uint64{}
	i.regionalValidationDeps = map[string]map[slotID]uint64{}
	i.hasValidation = false
	i.validationDeps = nil
	atomic.AddUint64(&i.scopeEpoch, 1)
	return nil
}

// validationValid 职责：核对内容版本及全部验证读取证据；使用：锁内冻结和编码入口；边界：诊断依赖变化也撤销证据，但不推进领域内容版本。
func (i *BlueprintInstance) validationValid(region string) bool {
	deps := i.validationDeps
	if region == "" {
		if !i.hasValidation || i.validated != i.revision {
			return false
		}
	} else {
		version, ok := i.regionalValidation[region]
		if !ok || version != i.revision {
			return false
		}
		deps = i.regionalValidationDeps[region]
	}
	for id, version := range deps {
		if current, ok := i.slots[id]; !ok || current.version != version {
			return false
		}
	}
	return true
}
