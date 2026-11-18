package saveanalysis

// 文件职责：转换戴森球设计形状和实际建设数量。
// 使用：按恒星和层查询；边界：不写回、不含戴森云，不对不同建设单位混算百分比。

import (
	"fmt"
	"math"
)

// BuildProgress 职责：保留同一种建设量的分子和分母；使用：单结构或同类合计；边界：Required 为零表示不适用。
type BuildProgress struct {
	// Unit 职责：明确建设计数单位；使用：同类汇总；边界：结构点与细胞点不相加。
	Unit string `json:"unit"`
	// Built 职责：保存已完成数量；使用：进度；边界：不包含已订购但未建设的数量。
	Built int64 `json:"built"`
	// Required 职责：保存设计需求量；使用：分母；边界：不代表未来增建需求。
	Required int64 `json:"required"`
}

// SphereStructure 职责：描述节点、框架或壳面；使用：设计形状与建设状态；边界：节点用 Position，框架与壳面用 NodeIDs，壳面另有顶点。
type SphereStructure struct {
	// Kind 职责：区分 node、frame、shell；使用：解释几何字段与建设单位；边界：不同类型不合算百分比。
	Kind string `json:"kind"`
	// ID 职责：保存原结构 ID；使用：层内定位；边界：仅在同类结构中唯一。
	ID int64 `json:"id"`
	// ProtoID 职责：保存设计原型；使用：后续样式解释；边界：不依赖前端百科。
	ProtoID int64 `json:"protoId"`
	// NodeIDs 职责：保存端点或壳面边界顺序；使用：恢复结构；边界：节点本身为空。
	NodeIDs []int64 `json:"nodeIds"`
	// Position 职责：保存节点层局部位置；使用：形状显示；边界：游戏长度单位，非星系坐标。
	Position []float64 `json:"position"`
	// Vertices 职责：保存壳面局部顶点；使用：形状数据；边界：不是已生成的渲染三角网格。
	Vertices [][]float64 `json:"vertices"`
	// Color 职责：保存 RGBA 字节；使用：样式；边界：四分量范围 0 到 255。
	Color []int64 `json:"color"`
	// Euler 职责：保存框架的曲线方式；使用：恢复框架；边界：其他结构为 false。
	Euler bool `json:"euler"`
	// Radius 职责：保存壳面半径；使用：恢复形状；边界：其他结构为零，单位与层半径一致。
	Radius float64 `json:"radius"`
	// Progress 职责：保存实际建设量；使用：node/frame 为结构点、shell 为细胞点；边界：不混合单位。
	Progress BuildProgress `json:"progress"`
}

// SphereLayer 职责：返回原始球层结构；使用：指定层显示；边界：空层也保留，四元数顺序为 x、y、z、w。
type SphereLayer struct {
	// ID 职责：标识原始轨道层；使用：查询；边界：删除层留下的空洞不重排。
	ID int64 `json:"id"`
	// Radius 职责：保存轨道半径；使用：比例还原；边界：游戏长度单位，非光年。
	Radius float64 `json:"radius"`
	// OrbitRotation 职责：保存轨道朝向；使用：坐标变换；边界：不作为当前位置替代。
	OrbitRotation []float64 `json:"orbitRotation"`
	// CurrentRotation 职责：保存快照时刻旋转；使用：实际朝向；边界：不做时间推进。
	CurrentRotation []float64 `json:"currentRotation"`
	// Structures 职责：列出三类设计结构；使用：明细及同类进度合计；边界：没有综合百分比。
	Structures []SphereStructure `json:"structures"`
}

// SphereView 职责：返回指定恒星的戴森球；使用：恒星和层选择；边界：Exists 区分没有球体数据与已初始化空球体。
type SphereView struct {
	// Tick 职责：标识保存时刻；使用：与世界坐标关联；边界：不推进游戏。
	Tick int64 `json:"tick"`
	// PositionUnit、PositionOrigin 职责：说明几何空间；使用：显示换算；边界：局部顶点需应用球层旋转再叠加恒星位置。
	PositionUnit   string `json:"positionUnit"`
	PositionOrigin string `json:"positionOrigin"`
	// SHA256 职责：关联快照；使用：一致性；边界：不是文件路径。
	SHA256 string `json:"sha256"`
	// StarID 职责：标识所属恒星；使用：世界关联；边界：不是数组下标。
	StarID int64 `json:"starId"`
	// Exists 职责：标识是否有保存的球体对象；使用：无设计状态；边界：true 不保证有建设量。
	Exists bool `json:"exists"`
	// Layers 职责：返回存在的球层；使用：选择显示；边界：空列表不代表建设完成。
	Layers []SphereLayer `json:"layers"`
}

// finite 职责：排除无效浮点几何；使用：解码转换；边界：不归一化或猜测损坏值。
func (r *reader) finite(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		r.fail("球体几何包含非有限数值")
	}
	return value
}

// vector 职责：恢复局部位置；使用：球体转换；边界：不转换为世界坐标。
func (r *reader) vector(v *layoutVector3) []float64 {
	return []float64{r.finite(float64(floatValue(v.x))), r.finite(float64(floatValue(v.y))), r.finite(float64(floatValue(v.z)))}
}

// rotation 职责：恢复四元数；使用：球层方向；边界：保留保存值。
func (r *reader) rotation(v *layoutQuaternion) []float64 {
	return []float64{r.finite(float64(floatValue(v.x))), r.finite(float64(floatValue(v.y))), r.finite(float64(floatValue(v.z))), r.finite(float64(floatValue(v.w)))}
}

// rgba 职责：复制颜色通道；使用：结构样式；边界：不解释材质。
func rgba(v *layoutRGBA) []int64 { return []int64{v.v_r, v.g, v.b, v.a} }

// progress 职责：验证同单位建设量；使用：结构转换；边界：拒绝负数与超过设计量的结果。
func (r *reader) progress(built, required int64) BuildProgress {
	if built < 0 || required < 0 || built > required {
		r.fail("球体建设数量无效")
	}
	return BuildProgress{Unit: "structure_point", Built: built, Required: required}
}

// buildSpheres 职责：关联恒星并校验结构引用；使用：完整解码后；边界：仅发布校验成功的球体。
func (r *reader) buildSpheres(s *Snapshot) {
	if r.world == nil || s.starCount <= 0 || int64(len(r.world.dysonSpheres)) != s.starCount {
		r.fail("球体与恒星数量不一致")
	}
	for _, id := range s.planetIDs {
		if id/100 < 1 || id/100 > s.starCount || id%100 == 0 {
			r.fail("厂区星球归属无效")
		}
	}
	for index, raw := range r.world.dysonSpheres {
		if raw.index != 0 && raw.index != 1 {
			r.fail("球体存在标记无效")
		}
		sphere := SphereView{SHA256: s.digest, Tick: s.tick, PositionUnit: "game_m", PositionOrigin: "layer_local", StarID: int64(index + 1), Exists: raw.data != nil, Layers: []SphereLayer{}}
		if raw.data != nil {
			for layerIndex, entry := range raw.data.layersIdBased {
				v := entry.data
				if v == nil {
					continue
				}
				if v.id != int64(layerIndex+1) || entry.index != v.id {
					r.fail("球层身份不一致")
				}
				layer := SphereLayer{ID: v.id, Radius: r.finite(float64(floatValue(v.orbitRadius))), OrbitRotation: r.rotation(v.orbitRotation), CurrentRotation: r.rotation(v.currentRotation), Structures: []SphereStructure{}}
				if layer.Radius <= 0 {
					r.fail("球层半径无效")
				}
				nodes := make(map[int64]bool)
				for i, entry := range v.nodePool {
					n := entry.data
					if n == nil {
						continue
					}
					if n.id != int64(i+1) || entry.index != n.id || n.layerId != v.id {
						r.fail("节点身份不一致")
					}
					nodes[n.id] = true
					layer.Structures = append(layer.Structures, SphereStructure{Kind: "node", ID: n.id, ProtoID: n.protoId, Position: r.vector(n.pos), Color: rgba(n.color), Progress: r.progress(n.sp, n.spMax)})
				}
				for i, entry := range v.framePool {
					f := entry.data
					if f == nil {
						continue
					}
					if f.id != int64(i+1) || entry.index != f.id || f.layerId != v.id || !nodes[f.nodeAId] || !nodes[f.nodeBId] {
						r.fail("框架身份或节点引用无效")
					}
					layer.Structures = append(layer.Structures, SphereStructure{Kind: "frame", ID: f.id, ProtoID: f.protoId, NodeIDs: []int64{f.nodeAId, f.nodeBId}, Euler: f.euler != 0, Color: rgba(f.color), Progress: r.progress(f.spA+f.spB, f.spMax)})
				}
				for i, entry := range v.shellPool {
					c := entry.data
					if c == nil {
						continue
					}
					if c.id != int64(i+1) || entry.index != c.id || c.layerId != v.id || len(c.nodecps) != len(c.nodesId)+1 || c.gridScale <= 0 {
						r.fail("壳面身份或建设数组无效")
					}
					for _, id := range c.nodesId {
						if !nodes[id] {
							r.fail("壳面节点引用无效")
						}
					}
					var built int64
					for _, count := range c.nodecps[:len(c.nodecps)-1] {
						if count < 0 {
							r.fail("壳面节点建设量无效")
						}
						built += count
					}
					if built != c.nodecps[len(c.nodecps)-1] {
						r.fail("壳面建设合计不一致")
					}
					shape := SphereStructure{Kind: "shell", ID: c.id, ProtoID: c.protoId, NodeIDs: append([]int64(nil), c.nodesId...), Color: rgba(c.color), Radius: r.finite(math.Float64frombits(uint64(c.radius))), Progress: r.progress(built, c.vertexCount*c.gridScale*c.gridScale*2), Vertices: [][]float64{}}
					shape.Progress.Unit = "cell_point"
					for _, vertex := range c.verts {
						shape.Vertices = append(shape.Vertices, r.vector(vertex))
					}
					layer.Structures = append(layer.Structures, shape)
				}
				sphere.Layers = append(sphere.Layers, layer)
			}
			if int64(len(sphere.Layers)) != raw.data.layerCount {
				r.fail("球层数量不一致")
			}
		}
		s.spheres = append(s.spheres, sphere)
	}
}

// Sphere 职责：返回恒星全部或单层球体数据；使用：layerID 为零表示全部层；边界：深复制查询结果，不改变快照。
func (s *Snapshot) Sphere(starID, layerID int64) (SphereView, error) {
	if starID <= 0 || starID > int64(len(s.spheres)) || layerID < 0 {
		return SphereView{}, fmt.Errorf("恒星或球层标识无效")
	}
	raw := s.spheres[starID-1]
	out := raw
	out.Layers = []SphereLayer{}
	for _, layer := range raw.Layers {
		if layerID != 0 && layer.ID != layerID {
			continue
		}
		layer.OrbitRotation = append([]float64(nil), layer.OrbitRotation...)
		layer.CurrentRotation = append([]float64(nil), layer.CurrentRotation...)
		structures := make([]SphereStructure, len(layer.Structures))
		for i, shape := range layer.Structures {
			shape.NodeIDs = append([]int64(nil), shape.NodeIDs...)
			shape.Position = append([]float64(nil), shape.Position...)
			shape.Color = append([]int64(nil), shape.Color...)
			vertices := make([][]float64, len(shape.Vertices))
			for j, v := range shape.Vertices {
				vertices[j] = append([]float64(nil), v...)
			}
			shape.Vertices = vertices
			structures[i] = shape
		}
		layer.Structures = structures
		out.Layers = append(out.Layers, layer)
	}
	if layerID != 0 && len(out.Layers) == 0 {
		return SphereView{}, fmt.Errorf("指定球层不存在")
	}
	return out, nil
}
