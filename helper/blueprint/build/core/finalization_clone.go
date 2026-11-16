package build

// 文件职责：提供冻结关系图的类型化复制原语。
// 使用：显式领域复制器处理切片、参数、映射及循环指针；边界：不用反射或序列化克隆，不保留原装配器的回调。

import (
	"context"
	"unsafe"

	"DysonSphereBox/helper/blueprint"
)

// finalizationCloner 职责：维护一次复制中的指针身份；使用：图快照及校验临时视图；边界：映射不跨复制操作共享，不保留在最终产物中。
type finalizationCloner struct {
	// pointers 职责：打断关系图循环并保留共享实体身份；使用：复制指针；边界：键值为同一具体指针类型，无反射转换。
	pointers map[any]any
	// bytes 职责：累计本次复制保留的容器、值和字符串大小；使用：产物内存预算；边界：重复字符串按多次计量，映射与分配器开销保守上取，不代表RSS。
	bytes int64
}

// newFinalizationCloner 职责：创建局部身份表；使用：每次快照复制；边界：调用方不得跨候选复用。
func newFinalizationCloner() *finalizationCloner {
	return &finalizationCloner{pointers: make(map[any]any)}
}

// retainedBytes 职责：汇总当前冻结图的保守保留量；使用：所有封闭产物统一计量。
// 边界：容器计数已含分配余量；暂存、工作副本和候选复制由工作流另行预留，不能在领域层再次乘倍数。
func (c *finalizationCloner) retainedBytes(overhead int64) int64 {
	if c.bytes < 0 || overhead < 0 || c.bytes > (1<<62)-overhead {
		return 1 << 62
	}
	return c.bytes + overhead
}

// seedPlacedGraph 职责：以连续内存预登记实际实体图的复制身份；使用：冻结及规则独占视图；边界：仍复制全部可变字段，图外引用继续由通用复制器处理。
func (c *finalizationCloner) seedPlacedGraph(source []*placed) {
	unseen := make([]*placed, 0, len(source))
	unique := make(map[*placed]bool, len(source))
	for _, node := range source {
		if node == nil || unique[node] {
			continue
		}
		unique[node] = true
		if _, exists := c.pointers[node]; !exists {
			unseen = append(unseen, node)
		}
	}
	if len(unseen) == 0 {
		return
	}
	arena := make([]placed, len(unseen))
	c.bytes += int64(len(arena))*int64(unsafe.Sizeof(placed{})) + 64
	// 所有身份先登记，再复制字段，双向引用和其它证据回指不会触发重复实体分配。
	for index, node := range unseen {
		c.pointers[node] = &arena[index]
	}
	for index, node := range unseen {
		arena[index] = c.buildPlaced(*node)
	}
}

// copyFinalizationPointer 职责：先登记身份再复制内容；使用：处理循环实体图；边界：nil 保持 nil，不共享原对象。
func copyFinalizationPointer[T any](c *finalizationCloner, source *T, clone func(T) T) *T {
	if source == nil {
		return nil
	}
	if existing, ok := c.pointers[source]; ok {
		return existing.(*T)
	}
	result := new(T)
	c.bytes += int64(unsafe.Sizeof(*result)) + 64
	c.pointers[source] = result
	*result = clone(*source)
	return result
}

// copyFinalizationSlice 职责：复制集合和各元素可变字段；使用：建筑参数与证据序列；边界：保留 nil 和空切片差别。
func copyFinalizationSlice[T any](c *finalizationCloner, source []T, clone func(T) T) []T {
	if source == nil {
		return nil
	}
	result := make([]T, len(source))
	var element T
	c.bytes += int64(len(source))*int64(unsafe.Sizeof(element)) + 64
	for i, value := range source {
		result[i] = clone(value)
	}
	return result
}

// copyFinalizationMap 职责：复制映射键值并重绑指针键；使用：关系图索引；边界：空映射不与源共享存储。
func copyFinalizationMap[K comparable, V any](c *finalizationCloner, source map[K]V, cloneKey func(K) K, cloneValue func(V) V) map[K]V {
	if source == nil {
		return nil
	}
	result := make(map[K]V, len(source))
	var keySize K
	var valueSize V
	c.bytes += int64(len(source))*(2*int64(unsafe.Sizeof(keySize)+unsafe.Sizeof(valueSize))+64) + 128
	for key, value := range source {
		result[cloneKey(key)] = cloneValue(value)
	}
	return result
}

// copyString 职责：计入只读字符串保留空间；使用：显式领域复制器；边界：字符串不可变可安全共享，预算仍保守逐引用计量。
func (c *finalizationCloner) copyString(source string) string {
	c.bytes += int64(len(source)) + 16
	return source
}

// finalizationBoundaryMarker 职责：在临时校验视图中保留边界类别；使用：现有检查只判定非nil时；边界：不得作为装配入口执行。
func finalizationBoundaryMarker(context.Context, []*transportRow) ([]denseSpatialUnit, error) {
	return nil, &blueprint.Error{Code: "GEN_E_FINAL_SNAPSHOT", Message: "冻结证据不具有实体装配能力"}
}
