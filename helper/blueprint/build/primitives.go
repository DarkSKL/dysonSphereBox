package build

// 文件职责：保留 f64 装配坐标及建筑引用；使用：各布局策略；边界：仅在最终编码前转 f32，索引在重排后统一赋值。
// 几何与原型事实核对 dsp-calc data/config.ts、itemsData.js 和 BuleprintMaker.ts，见 ../NOTICE.md。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	_ "embed"
	"encoding/json"
	"math"
)

// modelData 职责：内置模型原型映射；使用：创建建筑；边界：纯数值事实，不含游戏美术。
//
//go:embed models.json
var modelData []byte

// vector 职责：保存装配期间 f64 坐标；使用：平移和几何；边界：不提前截断。
type vector struct{ x, y, z float64 }

// placed 职责：以指针维护建筑连接；使用：合并重排；边界：最终 index 与数组映射独立赋值。
type placed struct {
	// record 职责：字段与参数；使用：最终转换；边界：坐标由 a/b 覆盖。
	record blueprint.Building
	// a 职责：首端坐标；使用：布局；边界：f64。
	a vector
	// b 职责：次端坐标；使用：分拣器；边界：f64。
	b vector
	// input 职责：输入引用；使用：最终编号；边界：nil 表示无连接。
	input *placed
	// output 职责：输出引用；使用：最终编号；边界：nil 表示无连接。
	output *placed
	// owner 职责：产线归属；使用：bindings；边界：-1 为公共设施。
	owner int
}

// builder 职责：收集单次装配；使用：六种模式；边界：不做文件写入。
type builder struct {
	// all 职责：最终建筑顺序；使用：编码；边界：每个指针只出现一次。
	all []*placed
	// models 职责：原型模型表；使用：创建；边界：未知 ID 拒绝。
	models map[string]int16
	// owner 职责：当前装配的产线；使用：归属登记；边界：跨块不依赖 recipeId 推测。
	owner int
	// setting 职责：归一设置；使用：速率/尺寸；边界：只在请求内使用。
	setting Setting
	// warnings 职责：非阻断告警；使用：返回 UI；边界：不得静默丢产线。
	warnings []string
	// coaterLinks 职责：登记每条喷涂供剂带及剂种；使用：可选网络；边界：指针随块一起平移。
	coaterLinks map[int][]*placed
	// zones 职责：多目标分区定义；使用：结果标注；边界：总线模式为空。
	zones []Zone
	// zoneRanges 职责：分区在 all 中的连续区间；使用：收尾计算边界；边界：喷涂网络等公共设施追加在区间之外。
	zoneRanges []zoneRange
	// surfaceBlocks 职责：保留可整体移动的独立产线块；使用：有限纬带装箱；边界：块内两端与连接一起移动，不能拆开塔与接驳带。
	surfaceBlocks [][]*placed
}

// zoneRange 职责：记录分区建筑区间；使用：分区边界；边界：半开区间 [begin, end)。
type zoneRange struct {
	zone, begin, end int
}

// newBuilder 职责：创建纯内存装配器；使用：Generate；边界：模型数据损坏时返回错误。
func newBuilder(s Setting) (*builder, error) {
	b := &builder{setting: s, models: map[string]int16{}, warnings: []string{}, coaterLinks: map[int][]*placed{}}
	if err := json.Unmarshal(modelData, &b.models); err != nil {
		return nil, err
	}
	return b, nil
}

// make 职责：建立默认记录；使用：各类型建筑；边界：调用方负责合法原型，最终统一校验。
func (b *builder) make(id int, x, y, z float64) *placed {
	p := &placed{record: blueprint.Building{Num: -102, RecordLayout: "v2", ItemID: int16(id), ModelIndex: b.models[itoa(id)], Yaw: 90, Yaw2: 90, OutputObjIdx: -1, InputObjIdx: -1, InputToSlot: 1}, a: vector{x, y, z}, b: vector{x, y, z}, owner: b.owner}
	b.all = append(b.all, p)
	return p
}

// itoa 职责：将原型整数转换为模型键；使用：查模型；边界：不承担数据解析。
func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var out [20]byte
	i := len(out)
	for v > 0 {
		i--
		out[i] = byte('0' + v%10)
		v /= 10
	}
	return string(out[i:])
}

// connect 职责：连接有向带；使用：路径装配；边界：不猜测输入反向字段。
func connect(a, b *placed) { a.output = b; a.record.OutputToSlot = 1 }

// reverse 职责：反转原数组中的带方向；使用：输入/输出转换；边界：保持数组位置稳定。
func reverse(line []*placed) {
	for i, p := range line {
		p.record.Yaw = float32(math.Mod(float64(p.record.Yaw)+180, 360))
		p.record.Yaw2 = p.record.Yaw
		p.output = nil
		p.record.OutputToSlot = 0
		if i > 0 {
			p.output = line[i-1]
			p.record.OutputToSlot = 1
		}
	}
}

// straight 职责：创建整数格直线；使用：机器两侧带；边界：方向 0 保留 -90 原始约定。
func (b *builder) straight(x, y float64, count, direction int) []*placed {
	result := make([]*placed, 0, count)
	for i := 0; i < count; i++ {
		px, py := x, y
		switch direction {
		case 1:
			py += float64(i)
		case 2:
			px += float64(i)
		case 3:
			py -= float64(i)
		case 4:
			px -= float64(i)
		}
		p := b.make(2000+b.setting.BeltLv, px, py, 0)
		p.record.Yaw = float32((direction - 1) * 90)
		p.record.Yaw2 = p.record.Yaw
		if i > 0 {
			connect(result[i-1], p)
		}
		result = append(result, p)
	}
	return result
}

// byPath 职责：按几何点装配带；使用：路由和桥接；边界：路径须由上层验证，不跨层猜测。
func (b *builder) byPath(points []vector) []*placed {
	result := make([]*placed, 0, len(points))
	for _, v := range points {
		p := b.make(2000+b.setting.BeltLv, v.x, v.y, v.z)
		if len(result) > 0 {
			connect(result[len(result)-1], p)
		}
		result = append(result, p)
	}
	return result
}

// shift 职责：整体平移；使用：块合并；边界：两端坐标同时移动，连接引用不变。
func shift(all []*placed, x, y, z float64) {
	for _, p := range all {
		p.a.x += x
		p.a.y += y
		p.a.z += z
		p.b.x += x
		p.b.y += y
		p.b.z += z
	}
}

// footprint 职责：返回固定朝向的原型占地矩形；使用：合并范围及路由避让；边界：数值来自参考碰撞表，不代表游戏最终碰撞判定。
func footprint(id int16) (vector, vector) {
	switch id {
	case 2103, 2104:
		return vector{-3, -3, 0}, vector{3, 3, 0}
	case 2101, 2020:
		return vector{-2, -2, 0}, vector{2, 2, 0}
	}
	typ, _ := quant.MachineKind(int(id))
	switch typ {
	case 1:
		return vector{-1, -1, 0}, vector{1, 1, 0}
	case 2, 6:
		return vector{-2, -2, 0}, vector{2, 2, 0}
	case 3:
		return vector{-2, -1, 0}, vector{3, 1, 0}
	case 4:
		return vector{-3, -1, 0}, vector{3, 2, 0}
	case 5:
		return vector{-4, -2, 0}, vector{3, 2, 0}
	}
	return vector{}, vector{}
}

// bounds 职责：计算含建筑占地的块范围；使用：间距/限宽；边界：包含两端点和原型碰撞表，不替代游戏碰撞检查。
func bounds(all []*placed) (min, max vector) {
	min = vector{math.Inf(1), math.Inf(1), math.Inf(1)}
	max = vector{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	for _, p := range all {
		lo, hi := footprint(p.record.ItemID)
		for _, v := range []vector{p.a, p.b, {p.a.x + lo.x, p.a.y + lo.y, p.a.z}, {p.a.x + hi.x, p.a.y + hi.y, p.a.z}} {
			min.x = math.Min(min.x, v.x)
			min.y = math.Min(min.y, v.y)
			min.z = math.Min(min.z, v.z)
			max.x = math.Max(max.x, v.x)
			max.y = math.Max(max.y, v.y)
			max.z = math.Max(max.z, v.z)
		}
	}
	return
}

// records 职责：统一编号和转换；使用：编码前；边界：bindings 使用数组下标，连接使用独立记录索引。
func (b *builder) records() ([]blueprint.Building, []ProductionBinding, []int, error) {
	out := make([]blueprint.Building, len(b.all))
	indices := map[*placed]int{}
	bindings := map[int][]int{}
	machines := map[int][]int{}
	shared := []int{}
	for i, p := range b.all {
		if _, ok := indices[p]; ok {
			return nil, nil, nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "建筑指针重复"}
		}
		indices[p] = i
	}
	for i, p := range b.all {
		r := p.record
		if r.ModelIndex == 0 {
			return nil, nil, nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "未知建筑模型: " + itoa(int(r.ItemID))}
		}
		r.Index = int32(i)
		r.LocalOffset = blueprint.Point{X: float32(p.a.x), Y: float32(p.a.y), Z: float32(p.a.z)}
		r.LocalOffset2 = blueprint.Point{X: float32(p.b.x), Y: float32(p.b.y), Z: float32(p.b.z)}
		if p.input != nil {
			index, ok := indices[p.input]
			if !ok {
				return nil, nil, nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "输入连接指向蓝图外部"}
			}
			r.InputObjIdx = int32(index)
		}
		if p.output != nil {
			index, ok := indices[p.output]
			if !ok {
				return nil, nil, nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "输出连接指向蓝图外部"}
			}
			r.OutputObjIdx = int32(index)
		}
		out[i] = r
		if p.owner < 0 {
			shared = append(shared, i)
		} else {
			bindings[p.owner] = append(bindings[p.owner], i)
			if r.RecipeID > 0 {
				machines[p.owner] = append(machines[p.owner], i)
			}
		}
	}
	result := []ProductionBinding{}
	for owner := 0; owner < len(b.all); owner++ {
		if indices, ok := bindings[owner]; ok {
			result = append(result, ProductionBinding{ProductionIndex: owner, BuildingIndices: indices, MachineIndices: machines[owner]})
		}
	}
	return out, result, shared, nil
}
