package sphereblueprint

// 文件职责：解码层内稀疏对象池和涂装；使用：单层及多层结构共用；边界：检查全部引用，不构造缺失的边或面。

import (
	"encoding/base64"
	"math"
)

// maxSlots 职责：约束整份预览的对象总量；使用：合计节点、框架和壳面的已用及空槽；边界：容量字段不参与内存分配。
const maxSlots = 256 * 1024

// pool 职责：遍历游标内的有效槽、核对回收表；使用：三类对象共用；边界：只按实际记录读，不按声明容量分配。
func (r *reader) pool(label string, read func(int)) {
	capacity, cursor, recycle := r.integer(), r.integer(), r.integer()
	if capacity < 1 || capacity > 1<<20 || cursor < 1 || cursor > capacity || recycle < 0 || recycle >= cursor ||
		cursor-1 > maxSlots-r.slots || cursor-1+recycle > len(r.data)/4 {
		r.fail(label + "对象池游标、容量或总槽数无效")
		return
	}
	r.slots += cursor - 1
	holes := make(map[int]bool)
	for id := 1; id < cursor && r.err == nil; id++ {
		switch r.integer() {
		case 0:
			holes[id] = true
		case id:
			read(id)
		default:
			r.fail(label + "对象 ID 与槽位不符")
		}
	}
	if len(holes) != recycle {
		r.fail(label + "回收表数量与空槽不符")
		return
	}
	for i := 0; i < recycle && r.err == nil; i++ {
		id := r.integer()
		if !holes[id] {
			r.fail(label + "回收槽重复或指向有效对象")
		}
		delete(holes, id)
	}
}

// layer 职责：读取三类对象并校验拓扑；使用：保留稀疏 ID 和显式环；边界：只将通过校验的整份蓝图交给调用者。
func (r *reader) layer(l *Layer) {
	version := r.version("球层", 1)
	l.Nodes, l.Frames, l.Shells = []Node{}, []Frame{}, []Shell{}
	nodes := make(map[int]bool)
	edges := make(map[[2]int]bool)
	r.pool("节点", func(id int) {
		n := r.node(id)
		nodes[id] = true
		l.Nodes = append(l.Nodes, n)
	})
	r.pool("框架", func(id int) {
		f := r.frame(id)
		key := [2]int{min(f.A, f.B), max(f.A, f.B)}
		if f.A == f.B || !nodes[f.A] || !nodes[f.B] || edges[key] {
			r.fail("框架端点缺失、重合或边重复")
		}
		edges[key] = true
		l.Frames = append(l.Frames, f)
	})
	r.pool("壳面", func(id int) {
		s := r.shell(id)
		for i, a := range s.Nodes {
			b := s.Nodes[(i+1)%len(s.Nodes)]
			if !edges[[2]int{min(a, b), max(a, b)}] {
				r.fail("壳面边界缺少对应框架或节点")
				break
			}
		}
		l.Shells = append(l.Shells, s)
	})
	if version == 1 && r.err == nil {
		l.GridMode = r.integer()
		if r.boolean() {
			count := r.count("逐格涂装", maxPayloadBytes/4, 4)
			l.CellColors = base64.StdEncoding.EncodeToString(r.take(count * 4))
		}
	}
}

// objectID 职责：校验对象内部 ID；使用：与外层池索引对照；边界：层中三类对象独立编号。
func (r *reader) objectID(slot int) int {
	if r.integer() != slot {
		r.fail("球壳对象内部 ID 与槽位不符")
	}
	return slot
}

// positionRadius 职责：从坐标估算原始层半径；使用：单层及节点方向校验；边界：输入为已读三维单精度向量。
func positionRadius(p []float64) float64 {
	return math.Sqrt(p[0]*p[0] + p[1]*p[1] + p[2]*p[2])
}

// node 职责：读取各版本节点；使用：保存局部位置和着色；边界：施工字段只消费字节，零方向无法重建。
func (r *reader) node(slot int) Node {
	version := r.version("节点", 5)
	n := Node{ID: r.objectID(slot), Proto: r.integer()}
	r.boolean() // 使用标志
	r.boolean() // 保留标志
	n.Pos = r.vector(3)
	if positionRadius(n.Pos) == 0 {
		r.fail("球壳节点的方向为零")
	}
	r.integer() // spMax
	if version >= 2 {
		r.integer() // rid
	}
	r.integer() // frameTurn
	if version >= 1 {
		r.integer() // shellTurn
	}
	r.integer() // spReq
	if version >= 4 {
		r.integer() // cpReq
	}
	if version >= 5 {
		n.Color = r.rgba()
	}
	return n
}

// frame 职责：读取框架端点与插值标志；使用：交给本层匹配节点；边界：版本零没有颜色。
func (r *reader) frame(slot int) Frame {
	version := r.version("框架", 1)
	f := Frame{ID: r.objectID(slot), Proto: r.integer()}
	r.boolean()
	f.A, f.B = r.integer(), r.integer()
	f.Euler = r.boolean()
	r.integer()
	if version == 1 {
		f.Color = r.rgba()
	}
	return f
}

// shell 职责：保留壳面种子、颜色和边界；使用：按原顺序连接框架；边界：不接受少于三个点、重复节点或超长环。
func (r *reader) shell(slot int) Shell {
	version := r.version("壳面", 2)
	s := Shell{ID: r.objectID(slot), Proto: r.integer(), Seed: r.integer()}
	if version == 2 {
		s.Color = r.rgba()
	}
	n := r.count("单个壳面边界节点", 4096, 4)
	if n < 3 {
		r.fail("壳面边界至少需要三个节点")
		return s
	}
	seen := make(map[int]bool, n)
	s.Nodes = make([]int, n)
	for i := range s.Nodes {
		id := r.integer()
		if seen[id] {
			r.fail("壳面边界重复引用节点")
			break
		}
		seen[id] = true
		s.Nodes[i] = id
	}
	return s
}
