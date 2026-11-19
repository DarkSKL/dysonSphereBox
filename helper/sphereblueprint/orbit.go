package sphereblueprint

// 文件职责：解码帆轨道及多层轨道与壳体的关联；使用：类型 2、3、4；边界：空槽跳过，保留有效对象的原始 ID。

// orbit 职责：读取共享轨道结构；使用：新版先读对象版本，旧版直接读 ID；边界：空轨道允许零姿态。
func (r *reader) orbit(mask int) Orbit {
	if r.modern {
		r.version("轨道", 0)
	}
	o := Orbit{ID: r.integer(), Radius: r.scalar(), Rotation: r.vector(4), Enabled: r.boolean()}
	if o.ID != 0 {
		if o.ID < 0 || o.ID > 20 || o.Radius <= 0 || (o.Rotation[0] == 0 && o.Rotation[1] == 0 && o.Rotation[2] == 0 && o.Rotation[3] == 0) {
			r.fail("球壳蓝图轨道 ID、半径或旋转无效")
			return o
		}
		o.Visible = mask&(1<<o.ID) != 0
	}
	return o
}

// swarm 职责：解码固定二十个帆轨道槽；使用：整球及纯帆群；边界：颜色按原始轨道 ID 索引，旧版沿用游戏初始色。
func (r *reader) swarm() []Orbit {
	r.integer() // 编辑视图掩码不参与主页展示。
	mask := r.integer()
	orbits := []Orbit{}
	for id := 1; id <= 20 && r.err == nil; id++ {
		o := r.orbit(mask)
		if o.ID == 0 {
			continue
		}
		if o.ID != id {
			r.fail("太阳帆轨道 ID 与槽位不符")
			break
		}
		o.HSVA = []float64{0.069444, 0.35, 1, 1}
		orbits = append(orbits, o)
	}
	if r.modern {
		n := r.count("帆轨道颜色", 32, 16)
		colors := make([][]float64, n)
		for i := range colors {
			colors[i] = r.vector(4)
		}
		for i, o := range orbits {
			if o.ID < len(colors) {
				orbits[i].HSVA = colors[o.ID]
			}
		}
	}
	return orbits
}

// layers 职责：关联按半径排序的轨道与按 ID 排序的结构；使用：多层或整球载荷；边界：拒绝重复轨道和孤立结构。
func (r *reader) layers() []Layer {
	r.integer()
	mask := r.integer()
	records := r.count("球层轨道槽", 10, 1)
	pending := make(map[int]Orbit)
	for range records {
		if r.boolean() {
			o := r.orbit(mask)
			_, duplicate := pending[o.ID]
			if o.ID < 1 || o.ID > 10 || duplicate {
				r.fail("球层轨道 ID 重复或越界")
				return nil
			}
			pending[o.ID] = o
		}
	}
	result := []Layer{}
	slots := r.count("球层结构槽", 11, 1)
	for id := range slots {
		if !r.boolean() {
			continue
		}
		o, found := pending[id]
		if !found {
			r.fail("球层结构缺少对应的轨道")
			return nil
		}
		layer := Layer{ID: id, Radius: o.Radius, Rotation: o.Rotation, Visible: o.Visible}
		r.layer(&layer)
		result = append(result, layer)
		delete(pending, id)
	}
	if len(pending) != 0 {
		r.fail("球层轨道缺少对应的结构")
	}
	return result
}
