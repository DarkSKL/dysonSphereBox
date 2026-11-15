package preview

// 文件职责：把前后可见场景转换为有界的新增、撤销与替换操作。
// 使用：单任务生成协程在实体提交或真实回退边界调用；边界：不依赖 Builder、Wails、候选选择或蓝图编码，不发送场景全量。

import "slices"

// Notifier 职责：保存当前已发布场景并输出最小对象变化；使用：单个生成任务；边界：调用方串行使用，回调不得重入同一通知器。
type Notifier struct {
	// emit 职责：传递一批渲染操作；使用：引擎进度桥接；边界：回调应快速返回，不参与算法决策。
	emit func(Frame, []Operation)
	// current 职责：保存已显示对象的值快照；使用：比较变化；边界：不引用算法内存或事件中可被修改的 Building 指针。
	current map[string]Building
	// order 职责：保存上个场景的稳定顺序；使用：确定性撤销；边界：不依赖 map 遍历顺序。
	order []string
	// frame 职责：保存已显示坐标语义；使用：坐标切换时替换保留对象；边界：区域数组独立复制。
	frame Frame
	// batches 职责：累计已交付批次；使用：完整事件核对；边界：不等同建筑数。
	batches int
}

// New 职责：创建独立任务通知器；使用：显式启用增量预览的生成入口；边界：没有发送器时不创建状态。
func New(emit func(Frame, []Operation)) *Notifier {
	if emit == nil {
		return nil
	}
	return &Notifier{emit: emit, current: map[string]Building{}}
}

// Sync 职责：比较场景并仅输出对象差量；使用：提交、平移、撤销及最终候选恢复；边界：不丢弃中间提交，不把 Frame 变化误当成同坐标姿态。
// 逻辑流：1. 构建新场景索引 => 2. 稳定撤销消失对象 => 3. 新增或替换变化对象 => 4. 分批交付，保留独立值快照。
func (n *Notifier) Sync(scene Scene) {
	if n == nil {
		return
	}
	next := make(map[string]Building, len(scene.Objects))
	order := make([]string, 0, len(scene.Objects))
	for _, object := range scene.Objects {
		if object.ID == "" {
			continue
		}
		if _, exists := next[object.ID]; !exists {
			order = append(order, object.ID)
		}
		next[object.ID] = object.Building
	}
	frameChanged := n.frame.CoordinateSystem != scene.Frame.CoordinateSystem || n.frame.PrimaryAreaIndex != scene.Frame.PrimaryAreaIndex || !slices.Equal(n.frame.Areas, scene.Frame.Areas)
	batch := make([]Operation, 0, BatchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		frame := scene.Frame
		frame.Areas = append([]Area(nil), frame.Areas...)
		n.emit(frame, batch)
		n.batches++
		batch = make([]Operation, 0, BatchSize)
	}
	appendOperation := func(operation Operation) {
		batch = append(batch, operation)
		if len(batch) == BatchSize {
			flush()
		}
	}
	for _, id := range n.order {
		if _, exists := next[id]; !exists {
			appendOperation(Operation{Type: Remove, ObjectID: id})
		}
	}
	for _, id := range order {
		building := next[id]
		previous, exists := n.current[id]
		if exists && !frameChanged && previous == building {
			continue
		}
		kind := Add
		if exists {
			kind = Replace
		}
		copy := building
		appendOperation(Operation{Type: kind, ObjectID: id, Building: &copy})
	}
	flush()
	n.current, n.order = next, order
	n.frame = scene.Frame
	n.frame.Areas = append([]Area(nil), scene.Frame.Areas...)
}

// Patch 职责：发布明确列出的新增或变更对象，保留其余场景；使用：已确认只有追加实体和局部连线变化的提交；边界：不支持删除或换坐标系，返回 false 时调用方必须完整 Sync。
func (n *Notifier) Patch(frame Frame, objects []Object) bool {
	if n == nil {
		return true
	}
	if n.current == nil || n.frame.CoordinateSystem != frame.CoordinateSystem || n.frame.PrimaryAreaIndex != frame.PrimaryAreaIndex || !slices.Equal(n.frame.Areas, frame.Areas) {
		return false
	}
	batch := make([]Operation, 0, BatchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		copyFrame := frame
		copyFrame.Areas = append([]Area(nil), frame.Areas...)
		n.emit(copyFrame, batch)
		n.batches++
		batch = make([]Operation, 0, BatchSize)
	}
	for _, object := range objects {
		if object.ID == "" {
			continue
		}
		previous, exists := n.current[object.ID]
		if exists && previous == object.Building {
			continue
		}
		kind := Replace
		if !exists {
			kind = Add
			n.order = append(n.order, object.ID)
		}
		value := object.Building
		n.current[object.ID] = value
		batch = append(batch, Operation{Type: kind, ObjectID: object.ID, Building: &value})
		if len(batch) == BatchSize {
			flush()
		}
	}
	flush()
	return true
}

// BatchCount 职责：返回实际输出批次数；使用：任务级完成标记；边界：空通知器返回零。
func (n *Notifier) BatchCount() int {
	if n == nil {
		return 0
	}
	return n.batches
}

// Discard 职责：释放已终结候选的差量比较缓存；使用：协议4候选finish之后；边界：不发撤销事件，保留批次数，正式获选快照由生成器独立持有。
func (n *Notifier) Discard() {
	if n == nil {
		return
	}
	n.current, n.order, n.frame = nil, nil, Frame{}
}
