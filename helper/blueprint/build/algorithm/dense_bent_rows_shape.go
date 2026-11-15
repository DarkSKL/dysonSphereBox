package build

// 文件职责：以真实局部轮廓有限搜索L/U接角，并原子提交各短run的刚性姿态。
// 使用：私有折行候选的单元组合；边界：不拉长分拣器、不旋转已经接好的跨单元带，失败恢复所有实体与朝向。

import (
	"context"
	"math"
)

// denseBentEntityState 职责：保存试装前完整实体值；使用：有界旋转试探与失败恢复；边界：试探不修改参数数组或实体链接。
type denseBentEntityState struct {
	entity *placed
	value  placed
}

// denseBentPlacement 职责：保存一个完整凹形候选的已验证实体姿态；使用：有限候选比较；边界：不缓存或替代之后的全场接线与球面规则。
type denseBentPlacement struct {
	states []denseBentEntityState
	turns  []int
	score  float64
}

// denseBentPacking 职责：建立只用于短run间净空的真实轮廓索引；使用：接角位置搜索；边界：使用同一dense冲突规则，不缩小原型或覆盖空矩形。
func denseBentPacking(contours []connectedBlockContour) *connectedPacking {
	packing := &connectedPacking{dense: true, cells: map[[3]int][]int{}}
	for _, contour := range contours {
		for _, shape := range contour.shapes {
			index := len(packing.shapes)
			packing.shapes = append(packing.shapes, shape)
			connectedShapeCells(shape, func(cell [3]int) bool {
				packing.cells[cell] = append(packing.cells[cell], index)
				return true
			})
		}
	}
	return packing
}

// composeDenseBentRows 职责：把两个完整run形成L、三个形成U；使用：全场布线前；边界：只提交通过真实轮廓净空的刚体姿态，三形组合失败由调用者保留两形或直线候选。
func (b *builder) composeDenseBentRows(ctx context.Context, units []denseSpatialUnit) (denseSpatialUnit, bool, error) {
	states := []denseBentEntityState{}
	for _, unit := range units {
		for _, entity := range unit.entities {
			states = append(states, denseBentEntityState{entity, *entity})
		}
	}
	restore := func(saved []denseBentEntityState) {
		for _, state := range saved {
			*state.entity = state.value
		}
	}
	defer func() { restore(states) }()
	baseLow, baseHigh := bounds(units[0].entities)
	baseTurn := 0
	if baseHigh.y-baseLow.y > baseHigh.x-baseLow.x {
		baseTurn = 1
	}
	var best *denseBentPlacement
	for _, first := range []int{1, 3} {
		for _, last := range []int{1, 3} {
			if len(units) == 2 && last != 1 {
				continue
			}
			for _, mirror := range []bool{false, true} {
				if len(units) == 3 && mirror {
					continue
				}
				restore(states)
				turns := []int{baseTurn, (baseTurn + first) % 4, (baseTurn + last) % 4}
				contours := []connectedBlockContour{}
				entities := []*placed{}
				valid := true
				footLow, footHigh := vector{}, vector{}
				for part, unit := range units {
					rotatePlacedQuarter(unit.entities, turns[part])
					low, high := bounds(unit.entities)
					shift(unit.entities, -low.x, -low.y, 0)
					low, high = bounds(unit.entities)
					// 生产分流附件已独立成单元；局部试装也仅预留本run实际拥有的端点。
					trial := denseSpatialUnit{entities: unit.entities, rows: []*transportRow{unit.row}, ports: unit.ports, bent: true}
					contour, err := b.denseSpatialContour(ctx, trial, turns[part])
					if err != nil {
						return denseSpatialUnit{}, false, err
					}
					if part == 0 {
						footLow, footHigh = low, high
					} else {
						width := high.x - low.x
						if len(units) == 3 && footHigh.x-footLow.x-width < 2 {
							valid = false
							break
						}
						x := footHigh.x - width/2
						if part == 1 && len(units) == 3 || len(units) == 2 && mirror {
							x = footLow.x - width/2
						}
						packing := denseBentPacking(contours)
						bestArea, bestDelta := math.Inf(1), vector{}
						for iy := -2; iy <= 8; iy++ {
							for ix := -4; ix <= 4; ix++ {
								if err := b.checkGeneration("折行接角真实净空搜索"); err != nil {
									return denseSpatialUnit{}, false, err
								}
								dx, dy := x+float64(ix)/2, footHigh.y+float64(iy)/2
								if dy+high.y <= footHigh.y+2 || !packing.fits(ctx, contour, dx, dy) {
									continue
								}
								lo, hi := bounds(entities)
								w := math.Max(hi.x, high.x+dx) - math.Min(lo.x, low.x+dx) + 1
								h := math.Max(hi.y, high.y+dy) - math.Min(lo.y, low.y+dy) + 1
								area := w*h + math.Abs(float64(ix))*.01 + math.Abs(float64(iy))*.01
								if area < bestArea {
									bestArea, bestDelta = area, vector{x: dx, y: dy}
								}
							}
						}
						if math.IsInf(bestArea, 1) {
							valid = false
							break
						}
						shift(unit.entities, bestDelta.x, bestDelta.y, 0)
						for i := range contour.shapes {
							contour.shapes[i] = shiftedConnectedShape(contour.shapes[i], bestDelta.x, bestDelta.y)
						}
					}
					contours = append(contours, contour)
					entities = append(entities, unit.entities...)
				}
				if !valid {
					continue
				}
				low, high := bounds(entities)
				width, height := high.x-low.x+1, high.y-low.y+1
				score := width * height * math.Max(1, math.Max(width, height)/math.Max(1, math.Min(width, height))/3)
				if best == nil || score < best.score {
					best = &denseBentPlacement{score: score, turns: append([]int{}, turns[:len(units)]...)}
					for _, entity := range entities {
						best.states = append(best.states, denseBentEntityState{entity, *entity})
					}
				}
			}
		}
	}
	if best == nil {
		return denseSpatialUnit{}, false, nil
	}
	restore(best.states)
	combined := denseSpatialUnit{bent: true}
	for part, unit := range units {
		unit.row.quarterTurns = (unit.row.quarterTurns + best.turns[part]) % 4
		combined.rows = append(combined.rows, unit.row)
		combined.entities = append(combined.entities, unit.entities...)
		combined.ports = append(combined.ports, unit.ports...)
	}
	// 已接受状态成为defer恢复的目标；失败和未接受的试探仍恢复原实体值。
	states = best.states
	return combined, true, nil
}
