package density

// 文件职责：按原生方向事实区分连续货物主路径与侧向汇入路径。
// 使用：确定性合流邻域采样；边界：不依赖未知实体创建顺序，几何/倾角打平及侧向追加曲线的未建模部分明确拒绝。

import (
	"fmt"
	"math"
)

// cargoPathLinks 职责：保存实际同CargoPath前后继与侧向出口；使用：固定offset只在本路径内移动；边界：物理output不等同于同一货物缓冲区的后继。
type cargoPathLinks struct {
	previous, next int32
	sideExit       bool
	err            error
	exitErr        error
}

// CargoPathIdentity 职责：公开已经证明的主路径与侧向出口身份；使用：真实世界坐标下审计合流或选址偏好；边界：负值表示无该连接，不能用平面近似结果强行裁掉实际球面候选。
type CargoPathIdentity struct {
	MainInputIndex  int32
	MainOutputIndex int32
	SideOutputIndex int32
}

// PathIdentity 职责：查询原生几何确定的连续货物路径身份；使用：合流诊断及世界坐标上的候选核验；边界：歧义分类明确失败，未承诺侧支追加曲线坐标已经建模。
func (s *CargoSampler) PathIdentity(anchor int32) (CargoPathIdentity, error) {
	result := CargoPathIdentity{MainInputIndex: -1, MainOutputIndex: -1, SideOutputIndex: -1}
	node, exists := s.nodes[anchor]
	if !exists {
		return result, ErrCargoBoundary
	}
	path := s.paths[anchor]
	if path.err != nil {
		return result, path.err
	}
	if path.exitErr != nil {
		return result, path.exitErr
	}
	result.MainInputIndex, result.MainOutputIndex = path.previous, path.next
	if path.sideExit {
		result.SideOutputIndex = node.next
	}
	return result, nil
}

// resolvePathLinks 职责：先独立选择每个节点的主输入，再建立连续路径；使用：只读采样器构造；边界：错误只影响实际查询涉及的未知邻域，不阻断无关纯链。
func (s *CargoSampler) resolvePathLinks() {
	s.paths = make(map[int32]cargoPathLinks, len(s.nodes))
	for index := range s.nodes {
		previous, err := s.mainInput(index)
		s.paths[index] = cargoPathLinks{previous: previous, next: -1, err: err}
	}
	for index, node := range s.nodes {
		path := s.paths[index]
		if node.next >= 0 {
			next := s.paths[node.next]
			if next.err != nil {
				path.exitErr = fmt.Errorf("%w：带%d后邻居主路径不确定", ErrCargoMerge, index)
			} else if next.previous == index {
				path.next = node.next
			} else {
				path.sideExit = true
			}
		}
		s.paths[index] = path
	}
}

// mainInput 职责：用投影方向的背向程度和环绕顺序识别原生主输入；使用：有出口的T/Y合流及可证明的末端合流；边界：打平时不以map/建筑编号猜测建造顺序。
func (s *CargoSampler) mainInput(index int32) (int32, error) {
	node := s.nodes[index]
	inputs := node.previous
	if len(inputs) == 0 {
		return -1, nil
	}
	for _, input := range inputs {
		if input == node.next {
			return -1, fmt.Errorf("%w：带%d输入与输出反向互接", ErrCargoMerge, index)
		}
	}
	if len(inputs) == 1 {
		return inputs[0], nil
	}
	if len(inputs) > 3 {
		return -1, fmt.Errorf("%w：带%d超过原生三输入", ErrCargoMerge, index)
	}
	uncertain := func() (int32, error) {
		return -1, fmt.Errorf("%w：带%d主输入存在建造顺序歧义", ErrCargoMerge, index)
	}
	up := s.upAt(node.position)
	direction := func(other int32) CargoPoint {
		delta := cargoSub(s.nodes[other].position, node.position)
		v, _ := cargoUnit(cargoSub(delta, cargoScale(up, .8*cargoDot(delta, up))))
		return v
	}
	if node.next >= 0 {
		out := direction(node.next)
		side := CargoPoint{up[1]*out[2] - up[2]*out[1], up[2]*out[0] - up[0]*out[2], up[0]*out[1] - up[1]*out[0]}
		if len(inputs) == 2 {
			a, c := direction(inputs[0]), direction(inputs[1])
			backA, backC := cargoDot(out, a), cargoDot(out, c)
			sides := cargoDot(side, a) * cargoDot(side, c)
			if math.Abs(sides+.001) < 1e-6 || math.Abs(math.Min(backA, backC)+.5) < 1e-6 {
				return uncertain()
			}
			if sides <= -.001 && math.Min(backA, backC) > -.5 {
				return -1, nil
			}
			if math.Abs(backA-backC) < 1e-6 {
				return uncertain()
			}
			if backA < backC {
				return inputs[0], nil
			}
			return inputs[1], nil
		}
		angles := [3]float64{}
		for i, input := range inputs {
			v := direction(input)
			angles[i] = math.Atan2(cargoDot(side, v), cargoDot(out, v))
			if angles[i] < 0 {
				angles[i] += 2 * math.Pi
			}
		}
		for i := 0; i < 3; i++ {
			for j := i + 1; j < 3; j++ {
				if math.Abs(angles[i]-angles[j]) < 1e-6 {
					return uncertain()
				}
			}
		}
		for i, angle := range angles {
			less := 0
			for _, other := range angles {
				if other < angle {
					less++
				}
			}
			if less == 1 {
				return inputs[i], nil
			}
		}
		return uncertain()
	}
	directions := make([]CargoPoint, len(inputs))
	for i, input := range inputs {
		directions[i], _ = cargoUnit(cargoSub(s.nodes[input].position, node.position))
	}
	if len(inputs) == 2 {
		dot := cargoDot(directions[0], directions[1])
		if math.Abs(dot+.5) < 1e-6 {
			return uncertain()
		}
		if dot <= -.5 {
			return -1, nil
		}
		first, second := math.Abs(float64(s.nodes[inputs[0]].tilt-node.tilt)), math.Abs(float64(s.nodes[inputs[1]].tilt-node.tilt))
		if math.Abs(first-second) < 1e-6 {
			return uncertain()
		}
		if first < second {
			return inputs[0], nil
		}
		return inputs[1], nil
	}
	// 没有出口的三输入节点，以夹角最大的两支为两侧，剩下的一支为主干。
	pairs := [3][2]int{{0, 1}, {1, 2}, {0, 2}}
	best, score := -1, math.Inf(1)
	tied := false
	for i, pair := range pairs {
		dot := cargoDot(directions[pair[0]], directions[pair[1]])
		if dot < score-1e-6 {
			best, score, tied = i, dot, false
		} else if math.Abs(dot-score) < 1e-6 {
			tied = true
		}
	}
	if best < 0 || tied {
		return uncertain()
	}
	for i, input := range inputs {
		if i != pairs[best][0] && i != pairs[best][1] {
			return input, nil
		}
	}
	return uncertain()
}
