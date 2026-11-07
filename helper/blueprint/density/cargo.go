package density

// 文件职责：从有向带邻接和实际世界坐标独立重建局部货物曲线采样，供密铺分拣偏移重标定。
// 使用：先为旧参考读取目标货物点，再在新物料带的允许锚点内寻找采样并重新绑定。
// 边界：支持确定主路径和侧支基础样本；不推断建造顺序歧义、未建模追加曲线或运行库存，也不执行游戏的越界自动夹取。
// 数值事实来自原版0.10.35.29151：相邻中点之间的三次曲线、约0.06米采样及0.15米货物高度；实现采用独立的De Casteljau求值。

import (
	"errors"
	"fmt"
	"math"
	"sync"

	"DysonSphereBox/helper/blueprint"
)

var (
	// ErrCargoGeometry 职责：标识坐标、原型或邻接不合法；使用：拒绝不可解释的数据；边界：不提供默认直线。
	ErrCargoGeometry = errors.New("货物采样几何或邻接无效")
	// ErrCargoBoundary 职责：标识裁切后缺失的邻居；使用：要求补齐参考边界；边界：缺失邻居不当作自然带尾。
	ErrCargoBoundary = errors.New("货物采样经过未知带边界")
	// ErrCargoMerge 职责：标识非唯一主链；使用：拒绝未经证明的合流采样；边界：不能靠任意选择一个前驱通过。
	ErrCargoMerge = errors.New("货物采样经过未支持的带合流")
	// ErrCargoRange 职责：标识路径或蓝图int8偏移越界；使用：偏移绑定；边界：不静默截断或夹到另一位置。
	ErrCargoRange = errors.New("货物采样或蓝图偏移超出范围")
	// ErrCargoDistance 职责：标识允许锚点内没有足够接近的采样；使用：保留物料所属带约束；边界：不改绑到另一物料带。
	ErrCargoDistance = errors.New("允许物料带内没有足够接近的货物采样")
)

// CargoPoint 职责：保存世界空间三维位置；使用：原图与新图之间的采样定位；边界：坐标单位为米，不是蓝图格数。
type CargoPoint [3]float64

// CargoBoundary 职责：显式描述裁切丢失的带到带边；使用：参考单元的boundaryReferences适配；边界：不得传入分拣器到带的物料引用。
type CargoBoundary struct {
	// FromIndex 职责：记录带边源编号；使用：辨认缺失前驱；边界：是Building.Index而非切片下标。
	FromIndex int32
	// ToIndex 职责：记录带边目标编号；使用：辨认缺失后继；边界：一端必须存在于当前带记录。
	ToIndex int32
}

// CargoOptions 职责：声明曲线坐标系和已知裁切边界；使用：构造采样器；边界：默认采用行星中心为原点的径向世界坐标。
type CargoOptions struct {
	// Boundaries 职责：登记记录外的带邻接；使用：避免裁切边缘被误认为带头；边界：完整蓝图可为空。
	Boundaries []CargoBoundary
	// PlanarUp 职责：指定平面候选的固定向上方向；使用：切平面几何预检；边界：nil代表球面径向，非nil必须为有限非零向量。
	PlanarUp *CargoPoint
}

// CargoBinding 职责：返回足够接近目标的实际采样及可编码引用；使用：重写分拣器带端；边界：位置只属于调用方允许的主链锚点。
type CargoBinding struct {
	// AnchorIndex 职责：保存该采样的带段锚点；使用：InputObjIdx/OutputObjIdx；边界：不沿用数组顺序编号。
	AnchorIndex int32
	// Offset 职责：保存相对锚点枢轴的有向采样偏移；使用：InputOffset/OutputOffset；边界：正值为下游，严格为int8。
	Offset int8
	// Position 职责：保存命中的货物中心位置；使用：复核世界空间误差；边界：包含0.15米货物高度。
	Position CargoPoint
	// Distance 职责：保存到调用方目标的距离；使用：约束重标定误差；边界：单位为米。
	Distance float64
}

// CargoSample 职责：记录一个真实货物样本及其所属带段；使用：固定offset的物料身份和取货窗口顺序核验；边界：SampleIndex是段内索引，不是可直接写入蓝图的offset。
type CargoSample struct {
	// NodeIndex 职责：保存实际样本所属带节点；使用：跨段后的物料轨道核验；边界：不同于最初绑定锚点。
	NodeIndex int32
	// SampleIndex 职责：保存所属带段的样本下标；使用：比较与喷涂枢轴的有向先后；边界：由同一采样器产生。
	SampleIndex int
	// Position 职责：保存真实货物中心坐标；使用：生成时保真及诊断；边界：游戏取放不要求保持某个来源世界点。
	Position CargoPoint
}

// cargoNode 职责：保存只读带拓扑；使用：局部曲线重建；边界：未知邻接单独登记，不补造建筑。
type cargoNode struct {
	// tilt 职责：保存原生带的倾角；使用：没有出口时的确定性主输入选择；边界：同倾角打平不推测实体创建顺序。
	tilt float32
	// position 职责：保存带节点世界位置；使用：曲线控制点；边界：与调用方坐标切片脱离。
	position CargoPoint
	// previous 职责：记录实际输出至此节点的带；使用：唯一主链判定；边界：多于一个不得猜测主输入。
	previous []int32
	// next 职责：记录有向后继；使用：跨段；边界：负值只表示已知的非带终点。
	next int32
	// unknownPrevious/unknownNext 职责：保存未包含的带边；使用：裁切失败边界；边界：不能按自然端点延长。
	unknownPrevious, unknownNext bool
}

// cargoSegment 职责：保存单个带节点的曲线样本；使用：有向索引定位；边界：pivot在samples内。
type cargoSegment struct {
	// samples 职责：保存有序货物中心位置；使用：按offset取样；边界：包含端部保护样本，取放时另行限制。
	samples []CargoPoint
	// pivot 职责：保存该带枢轴的数组下标；使用：相对偏移；边界：不是节点的空间坐标。
	pivot int
	// extensionSamples 职责：记录侧支基础曲线之后的原生尾段样本数；使用：确定真实末端保护；边界：当前不提供该追加曲线的世界坐标。
	extensionSamples int
}

// cargoCachedSegment 职责：缓存几何或错误；使用：大量分拣器共用相邻带段；边界：错误不转为默认成功。
type cargoCachedSegment struct {
	// segment 职责：缓存成功样本；使用：重复访问；边界：错误时为空。
	segment cargoSegment
	// err 职责：缓存明确失败；使用：保持错误稳定；边界：不能当作空曲线继续。
	err error
}

// CargoSampler 职责：保存不可变的主带图和线程安全采样缓存；使用：读取原图或重标定新图；边界：构造后不随外部records或positions变更。
type CargoSampler struct {
	// nodes 职责：保存不可变的带邻接；使用：段间移动；边界：只含已识别原版带。
	nodes map[int32]cargoNode
	// paths 职责：保存同CargoPath的连接身份；使用：固定offset跨段；边界：侧向output不能当作缓冲区连续后继。
	paths map[int32]cargoPathLinks
	// up 职责：可选固定平面法向；使用：兼容平面诊断；边界：nil使用径向法向。
	up *CargoPoint
	// mu 职责：保护曲线缓存；使用：并行候选查询；边界：不保护外部可写对象。
	mu sync.Mutex
	// cache 职责：复用段计算结果；使用：同一位置多臂引用；边界：构造后记录不可变化。
	cache map[int32]cargoCachedSegment
}

// NewCargoSampler 职责：从记录与同序世界位置建立采样器；使用：源图与最终候选各建一份；边界：支持显式裁切，未知合流只在请求涉及该邻域时失败。
func NewCargoSampler(records []blueprint.Building, positions []CargoPoint, options ...CargoOptions) (*CargoSampler, error) {
	if len(records) != len(positions) || len(options) > 1 {
		return nil, ErrCargoGeometry
	}
	s := &CargoSampler{nodes: map[int32]cargoNode{}, cache: map[int32]cargoCachedSegment{}}
	var option CargoOptions
	if len(options) == 1 {
		option = options[0]
	}
	if option.PlanarUp != nil {
		up, ok := cargoUnit(*option.PlanarUp)
		if !ok {
			return nil, fmt.Errorf("%w：平面向上方向", ErrCargoGeometry)
		}
		s.up = &up
	}
	byIndex := make(map[int32]blueprint.Building, len(records))
	for i, record := range records {
		if _, exists := byIndex[record.Index]; exists || record.Index < 0 {
			return nil, fmt.Errorf("%w：重复或负建筑编号%d", ErrCargoGeometry, record.Index)
		}
		byIndex[record.Index] = record
		if record.ItemID < 2001 || record.ItemID > 2003 {
			continue
		}
		if record.ModelIndex != record.ItemID-1966 || !cargoFinite(positions[i]) {
			return nil, fmt.Errorf("%w：带%d的模型或坐标", ErrCargoGeometry, record.Index)
		}
		if s.up == nil {
			if _, ok := cargoUnit(positions[i]); !ok {
				return nil, fmt.Errorf("%w：带%d位于行星原点", ErrCargoGeometry, record.Index)
			}
		}
		s.nodes[record.Index] = cargoNode{position: positions[i], next: -1, tilt: record.Tilt}
	}
	for index, node := range s.nodes {
		record := byIndex[index]
		if record.OutputObjIdx >= 0 {
			if target, belt := s.nodes[record.OutputObjIdx]; belt {
				if record.OutputObjIdx == index {
					return nil, fmt.Errorf("%w：带%d自连接", ErrCargoGeometry, index)
				}
				node.next = record.OutputObjIdx
				target.previous = append(target.previous, index)
				s.nodes[record.OutputObjIdx] = target
			} else if _, exists := byIndex[record.OutputObjIdx]; !exists {
				node.unknownNext = true
			}
		}
		if record.InputObjIdx >= 0 {
			if _, exists := byIndex[record.InputObjIdx]; !exists {
				node.unknownPrevious = true
			}
		}
		// 其它边可能刚向该节点登记了前驱，不能用迭代开始时的旧切片覆盖它。
		updated := s.nodes[index]
		updated.next, updated.unknownNext, updated.unknownPrevious = node.next, node.unknownNext, node.unknownPrevious
		s.nodes[index] = updated
	}
	for _, boundary := range option.Boundaries {
		from, hasFrom := s.nodes[boundary.FromIndex]
		to, hasTo := s.nodes[boundary.ToIndex]
		if hasFrom == hasTo {
			return nil, fmt.Errorf("%w：裁切边%d→%d", ErrCargoGeometry, boundary.FromIndex, boundary.ToIndex)
		}
		if hasFrom {
			from.unknownNext = true
			s.nodes[boundary.FromIndex] = from
		}
		if hasTo {
			to.unknownPrevious = true
			s.nodes[boundary.ToIndex] = to
		}
	}
	s.resolvePathLinks()
	return s, nil
}

// PointAt 职责：按锚点与有向偏移读取实际货物样本；使用：还原参考蓝图的取放位置；边界：不把超出主链或int8的偏移夹到边缘。
func (s *CargoSampler) PointAt(anchor int32, offset int) (CargoPoint, error) {
	sample, err := s.ResolveAt(anchor, offset)
	return sample.Position, err
}

// ResolveAt 职责：读取固定编码offset的实际样本身份；使用：生成结果在不同纬度的运货核验；边界：不允许游戏InternalOffsetCorrection修改偏移，不检查与原始理想世界点的距离。
func (s *CargoSampler) ResolveAt(anchor int32, offset int) (CargoSample, error) {
	if offset < -128 || offset > 127 {
		return CargoSample{}, ErrCargoRange
	}
	return s.sampleAt(anchor, offset, true)
}

// PickupWindowAt 职责：解析游戏从pivot+offset-2开始检查的五个连续样本；使用：禁止取货窗口跨物料或回到喷涂点之前；边界：只限制编码中心的int8及端部保护，窗口内的原生合法样本不再夹取或套用中心保护。
func (s *CargoSampler) PickupWindowAt(anchor int32, offset int) ([5]CargoSample, error) {
	var result [5]CargoSample
	center, err := s.ResolveAt(anchor, offset)
	if err != nil {
		return result, err
	}
	result[2] = center
	for i := range result {
		if i == 2 {
			continue
		}
		result[i], err = s.sampleAt(anchor, offset+i-2, false)
		if err != nil {
			return [5]CargoSample{}, err
		}
	}
	return result, nil
}

// sampleAt 职责：沿真实有向链定位段与样本；使用：中心和原生取货窗口共用；边界：窗口可读取中心保护区内的存在样本，但不得越过真实带端、未知合流或闭环切口。
func (s *CargoSampler) sampleAt(anchor int32, offset int, protectCenter bool) (CargoSample, error) {
	segment, err := s.segment(anchor)
	if err != nil {
		return CargoSample{}, err
	}
	index := segment.pivot + offset
	visited := map[int32]bool{anchor: true}
	for index < 0 || index >= len(segment.samples) {
		path := s.paths[anchor]
		var next int32
		backward := index < 0
		if backward {
			if path.previous < 0 {
				return CargoSample{}, fmt.Errorf("%w：超过带头%d", ErrCargoRange, anchor)
			}
			next = path.previous
		} else {
			if path.sideExit {
				return CargoSample{}, fmt.Errorf("%w：带%d的侧向追加曲线尚无坐标证明", ErrCargoBoundary, anchor)
			}
			if path.next < 0 {
				return CargoSample{}, fmt.Errorf("%w：超过带尾%d", ErrCargoRange, anchor)
			}
			index -= len(segment.samples)
			next = path.next
		}
		if visited[next] {
			return CargoSample{}, fmt.Errorf("%w：偏移穿过闭环切口", ErrCargoRange)
		}
		visited[next] = true
		anchor = next
		segment, err = s.segment(anchor)
		if err != nil {
			return CargoSample{}, err
		}
		if backward {
			index += len(segment.samples)
		}
	}
	if protectCenter {
		first, last := s.usableSamples(anchor, segment)
		if index < first || index > last {
			return CargoSample{}, fmt.Errorf("%w：带%d的端部保护区", ErrCargoRange, anchor)
		}
	}
	return CargoSample{NodeIndex: anchor, SampleIndex: index, Position: segment.samples[index]}, nil
}

// ForwardSampleDistance 职责：计算同一原生连续货物路径上两个实际样本的先后距离；使用：喷涂枢轴与最早可取样本比较；边界：其它路径、未知合流/邻域和闭环不能被当作已喷涂路径。
func (s *CargoSampler) ForwardSampleDistance(from, to CargoSample) (int, error) {
	segment, err := s.segment(from.NodeIndex)
	if err != nil {
		return 0, err
	}
	target, err := s.segment(to.NodeIndex)
	if err != nil {
		return 0, err
	}
	if from.SampleIndex < 0 || from.SampleIndex >= len(segment.samples) || to.SampleIndex < 0 || to.SampleIndex >= len(target.samples) {
		return 0, ErrCargoRange
	}
	distance, node := -from.SampleIndex, from.NodeIndex
	visited := map[int32]bool{}
	for node != to.NodeIndex {
		if node < 0 || visited[node] {
			return 0, fmt.Errorf("%w：目标样本不在下游纯链", ErrCargoRange)
		}
		visited[node] = true
		segment, err = s.segment(node)
		if err != nil {
			return 0, err
		}
		distance += len(segment.samples)
		node = s.paths[node].next
	}
	return distance + to.SampleIndex, nil
}

// Nearest 职责：在明确允许的带锚点所属曲线段内寻找最近样本；使用：新轨道重标定；边界：不搜索其它物料、不静默忽略未知邻域，最近点无法编码时返回范围错误。
func (s *CargoSampler) Nearest(anchors []int32, target CargoPoint, maxDistance float64) (CargoBinding, error) {
	if len(anchors) == 0 || !cargoFinite(target) || math.IsNaN(maxDistance) || math.IsInf(maxDistance, 0) || maxDistance < 0 {
		return CargoBinding{}, ErrCargoGeometry
	}
	best, bestOffset := CargoBinding{Distance: math.Inf(1)}, 0
	for _, anchor := range anchors {
		segment, err := s.segment(anchor)
		if err != nil {
			return CargoBinding{}, err
		}
		first, last := s.usableSamples(anchor, segment)
		for index, point := range segment.samples {
			if index < first || index > last {
				continue
			}
			distance := cargoDistance(point, target)
			offset := index - segment.pivot
			sameDistance := math.Abs(distance-best.Distance) <= 1e-9
			preferOffset := absCargoOffset(offset) < absCargoOffset(bestOffset) || absCargoOffset(offset) == absCargoOffset(bestOffset) && (anchor < best.AnchorIndex || anchor == best.AnchorIndex && offset < bestOffset)
			if distance < best.Distance-1e-9 || sameDistance && preferOffset {
				best, bestOffset = CargoBinding{AnchorIndex: anchor, Position: point, Distance: distance}, offset
			}
		}
	}
	if best.Distance > maxDistance {
		return CargoBinding{}, fmt.Errorf("%w：误差%.6f米，大于%.6f米", ErrCargoDistance, best.Distance, maxDistance)
	}
	if bestOffset < -128 || bestOffset > 127 {
		return CargoBinding{}, fmt.Errorf("%w：锚点%d需要偏移%d", ErrCargoRange, best.AnchorIndex, bestOffset)
	}
	best.Offset = int8(bestOffset)
	return best, nil
}

// absCargoOffset 职责：比较偏移大小；使用：同距绑定优先靠近锚点枢轴；边界：输入是有限采样下标差，不接收整数最小值。
func absCargoOffset(offset int) int {
	if offset < 0 {
		return -offset
	}
	return offset
}

// segment 职责：缓存并同步访问单个邻域的样本；使用：PointAt和Nearest；边界：缓存错误与成功结果同样稳定。
func (s *CargoSampler) segment(index int32) (cargoSegment, error) {
	if s == nil || s.nodes == nil || s.cache == nil {
		return cargoSegment{}, ErrCargoGeometry
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if cached, ok := s.cache[index]; ok {
		return cached.segment, cached.err
	}
	segment, err := s.makeSegment(index)
	s.cache[index] = cargoCachedSegment{segment: segment, err: err}
	return segment, err
}

// usableSamples 职责：排除自然带头四点与带尾五点保护区；使用：严格取放与最近点绑定；边界：缺失边界已在segment阶段拒绝，不模拟游戏自动夹取。
func (s *CargoSampler) usableSamples(index int32, segment cargoSegment) (int, int) {
	path := s.paths[index]
	first, last := 0, len(segment.samples)-1
	if path.previous < 0 {
		first = 4
	}
	if path.next < 0 && (!path.sideExit || segment.extensionSamples == 0) {
		last = len(segment.samples) - 6
	}
	return first, last
}

// makeSegment 职责：构造实际主路径基础样本并辨明侧支末端保护；使用：首次访问带段；边界：不猜测歧义主输入，不把侧支追加曲线直接接到主路径。
func (s *CargoSampler) makeSegment(index int32) (cargoSegment, error) {
	path := s.paths[index]
	if path.exitErr != nil {
		return cargoSegment{}, path.exitErr
	}
	result, err := s.makeBaseSegment(index)
	if err != nil {
		return cargoSegment{}, err
	}
	if path.sideExit {
		node := s.nodes[index]
		target, err := s.makeBaseSegment(node.next)
		if err != nil {
			return cargoSegment{}, err
		}
		pivot := target.samples[target.pivot]
		pivot = cargoSub(pivot, cargoScale(s.upAt(pivot), .15))
		middle := cargoLerp(node.position, s.nodes[node.next].position, .5)
		length := cargoDistance(pivot, middle) / .06
		if math.Abs(length-math.Floor(length)-.5) < 1e-4 {
			return cargoSegment{}, fmt.Errorf("%w：侧支追加长度接近单精度取整边界", ErrCargoGeometry)
		}
		count := min(170, int(math.RoundToEven(length)))
		if count > 0 {
			result.extensionSamples = max(1, count-5) + 5
		}
	}
	return result, nil
}

// makeBaseSegment 职责：独立计算当前节点的原生Bezier枢轴与基础样本；使用：主干、侧支前缀及追加尾长；边界：不递归选择创建顺序，也不生成侧向追加曲线的猜测坐标。
func (s *CargoSampler) makeBaseSegment(index int32) (cargoSegment, error) {
	node, exists := s.nodes[index]
	if !exists {
		return cargoSegment{}, fmt.Errorf("%w：带%d不存在", ErrCargoBoundary, index)
	}
	if node.unknownNext || node.unknownPrevious {
		return cargoSegment{}, fmt.Errorf("%w：带%d邻居缺失", ErrCargoBoundary, index)
	}
	path := s.paths[index]
	if path.err != nil {
		return cargoSegment{}, path.err
	}
	hasPrevious, hasNext := path.previous >= 0, node.next >= 0
	if !hasPrevious && !hasNext {
		point := cargoAdd(node.position, cargoScale(s.upAt(node.position), .15))
		result := cargoSegment{pivot: 4, samples: make([]CargoPoint, 10)}
		for i := range result.samples {
			result.samples[i] = point
		}
		return result, nil
	}
	center, previous, next := node.position, node.position, node.position
	if hasPrevious {
		neighbor := s.nodes[path.previous]
		if neighbor.unknownPrevious {
			return cargoSegment{}, fmt.Errorf("%w：带%d前邻居", ErrCargoBoundary, index)
		}
		previous = neighbor.position
	}
	if hasNext {
		neighbor := s.nodes[node.next]
		if neighbor.unknownPrevious {
			return cargoSegment{}, fmt.Errorf("%w：带%d后邻居", ErrCargoBoundary, index)
		}
		next = neighbor.position
	}
	up := s.upAt(center)
	if !hasPrevious {
		previous = cargoAdd(center, cargoReject(cargoSub(center, next), up))
	}
	if !hasNext {
		next = cargoAdd(center, cargoReject(cargoSub(center, previous), up))
	}
	incoming, okA := cargoUnit(cargoSub(previous, center))
	outgoing, okB := cargoUnit(cargoSub(next, center))
	if !okA || !okB {
		return cargoSegment{}, fmt.Errorf("%w：带%d存在零长边", ErrCargoGeometry, index)
	}
	angle := math.Acos(math.Max(-1, math.Min(1, cargoDot(incoming, outgoing)))) * 180 / math.Pi
	fraction := math.Max(0, math.Min(1, (angle-90)/90))
	weight := .55192 + (.66667-.55192)*fraction
	left, right := cargoLerp(previous, center, .5), cargoLerp(next, center, .5)
	controlA, controlB := cargoLerp(left, center, weight), cargoLerp(right, center, weight)
	curve := func(t float64) CargoPoint { return cargoBezier(left, controlA, controlB, right, t) }
	pivot := curve(.5)
	leftCount, rightCount := 4, 5
	if hasPrevious {
		leftCount = int(math.RoundToEven(cargoDistance(left, pivot)/.06 - .3))
	}
	if hasNext {
		rightCount = int(math.RoundToEven(cargoDistance(right, pivot)/.06 - .3))
	}
	leftCount, rightCount = min(170, leftCount), min(170, rightCount)
	if leftCount < 0 || rightCount < 0 {
		return cargoSegment{}, fmt.Errorf("%w：带%d曲线长度不足", ErrCargoGeometry, index)
	}
	result := cargoSegment{pivot: leftCount, samples: make([]CargoPoint, leftCount+rightCount+1)}
	for offset := -leftCount; offset <= rightCount; offset++ {
		t := .5
		if offset < 0 {
			t += float64(offset) * .5 / (float64(leftCount) + .5)
		}
		if offset > 0 {
			t += float64(offset) * .5 / (float64(rightCount) + .5)
		}
		point := curve(t)
		result.samples[leftCount+offset] = cargoAdd(point, cargoScale(s.upAt(point), .15))
	}
	return result, nil
}

// upAt 职责：取得指定世界点的向上方向；使用：端部延长和货物中心偏置；边界：构造时已排除行星原点。
func (s *CargoSampler) upAt(p CargoPoint) CargoPoint {
	if s.up != nil {
		return *s.up
	}
	up, _ := cargoUnit(p)
	return up
}

// cargoBezier 职责：用De Casteljau计算三次曲线；使用：任意已知相邻带段；边界：不引入与路径方向无关的节点吸附。
func cargoBezier(a, b, c, d CargoPoint, t float64) CargoPoint {
	ab, bc, cd := cargoLerp(a, b, t), cargoLerp(b, c, t), cargoLerp(c, d, t)
	return cargoLerp(cargoLerp(ab, bc, t), cargoLerp(bc, cd, t), t)
}

// cargoFinite 职责：检查三维数值有限；使用：输入校验；边界：不修正非法数。
func cargoFinite(p CargoPoint) bool {
	for _, v := range p {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}

// cargoAdd 职责：相加世界向量；使用：曲线计算；边界：不改变单位。
func cargoAdd(a, b CargoPoint) CargoPoint { return CargoPoint{a[0] + b[0], a[1] + b[1], a[2] + b[2]} }

// cargoSub 职责：计算有向差；使用：方向和距离；边界：不取绝对值。
func cargoSub(a, b CargoPoint) CargoPoint { return CargoPoint{a[0] - b[0], a[1] - b[1], a[2] - b[2]} }

// cargoScale 职责：缩放向量；使用：插值与偏置；边界：不参与物料速率缩放。
func cargoScale(p CargoPoint, f float64) CargoPoint { return CargoPoint{p[0] * f, p[1] * f, p[2] * f} }

// cargoDot 职责：计算点积；使用：夹角与投影；边界：坐标系必须一致。
func cargoDot(a, b CargoPoint) float64 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }

// cargoDistance 职责：计算世界欧氏距离；使用：误差阈值；边界：不是测地距离。
func cargoDistance(a, b CargoPoint) float64 { d := cargoSub(a, b); return math.Sqrt(cargoDot(d, d)) }

// cargoUnit 职责：安全归一化向量；使用：法向与方向；边界：零向量或溢出明确失败。
func cargoUnit(p CargoPoint) (CargoPoint, bool) {
	length := math.Sqrt(cargoDot(p, p))
	if !cargoFinite(p) || math.IsNaN(length) || math.IsInf(length, 0) || length < 1e-9 {
		return CargoPoint{}, false
	}
	return cargoScale(p, 1/length), true
}

// cargoReject 职责：去除向上分量；使用：自然带端切向延长；边界：up须已归一化。
func cargoReject(p, up CargoPoint) CargoPoint { return cargoSub(p, cargoScale(up, cargoDot(p, up))) }

// cargoLerp 职责：线性插值；使用：独立曲线求值；边界：参数由调用方限定。
func cargoLerp(a, b CargoPoint, t float64) CargoPoint {
	return cargoAdd(cargoScale(a, 1-t), cargoScale(b, t))
}
