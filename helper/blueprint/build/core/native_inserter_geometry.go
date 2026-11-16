package build

// 文件职责：缓存原生端点域中逐值相同的动态几何并加速精确分离轴检查。
// 使用：冲突图构造；边界：不删除候选、不改变候选编号、碰撞阈值或回溯预算，缓存只属于一次构图。

import "DysonSphereBox/helper/blueprint/build/placement"

// nativeInserterGeometrySet 职责：关联原候选与逐值相同的盒序列；使用：同域重复几何复用；边界：不合并记录或端口，Owner 不参与原图的几何相交运算。
type nativeInserterGeometrySet struct {
	choices []int
	boxes   [][]placement.OwnedBox
}

// nativeInserterGeometryStep 职责：以完整前缀和当前盒精确标识盒序列；使用：同域几何驻留；边界：不量化浮点、不依赖有碰撞风险的摘要。
type nativeInserterGeometryStep struct {
	prefix int
	box    placement.Box
}

// nativeInserterGeometries 职责：为逐值相同的有序盒序列分配同一编号；使用：构图前；边界：保留空盒、多个盒及每个原候选的位置，NaN 自然无法命中缓存。
func nativeInserterGeometries(domain nativeInserterDomain) nativeInserterGeometrySet {
	result := nativeInserterGeometrySet{choices: make([]int, len(domain.choices))}
	steps := make(map[nativeInserterGeometryStep]int)
	groups := make(map[int]int)
	for i := range domain.choices {
		prefix := 0
		for _, box := range domain.choices[i].boxes {
			key := nativeInserterGeometryStep{prefix: prefix, box: box.Geometry}
			id, exists := steps[key]
			if !exists {
				id = len(steps) + 1
				steps[key] = id
			}
			prefix = id
		}
		group, exists := groups[prefix]
		if !exists {
			group = len(result.boxes)
			groups[prefix] = group
			result.boxes = append(result.boxes, domain.choices[i].boxes)
		}
		result.choices[i] = group
	}
	return result
}

// nativeInserterGeometryConflict 职责：检查两组候选盒的原始正体积碰撞；使用：缓存未命中的组合；边界：保持盒访问顺序，不按 Owner 豁免。
func nativeInserterGeometryConflict(left, right []placement.OwnedBox) bool {
	for i := range left {
		for j := range right {
			if placement.BoxesOverlap(&left[i].Geometry, &right[j].Geometry) {
				return true
			}
		}
	}
	return false
}
