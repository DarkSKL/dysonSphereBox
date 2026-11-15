package build

// 文件职责：在紧凑候选的固定几何上联合选择互不冲突的感应塔。
// 使用：全局补电独立候选；边界：静态合法域来自原精确索引，选择必须保护未覆盖消费者，不降低三维覆盖、塔距、净空和最终终检。

import (
	"fmt"
	"math"
	"sort"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
)

// connectedPowerDomain 职责：保存一个独立耗电中心的静态合法域及当前可用数；使用：最小剩余域选择和前向检查；边界：covered 只来自真正选中塔的三维覆盖，不能把可选塔当已供电。
type connectedPowerDomain struct {
	consumer  connectedPowerConsumer
	values    []int
	remaining int
	covered   int
}

// connectedPowerChoice 职责：保存一个静态合法塔位、共享消费者及与其它塔位的互斥关系；使用：可逆域更新；边界：缓存只依赖固定几何，blocked 随试探撤回而精确恢复。
type connectedPowerChoice struct {
	point     vector
	world     placement.Vector
	boxes     []placement.Box
	domains   []int
	covers    []int
	conflicts []int
	prepared  bool
	blocked   int
}

// connectedPowerSolver 职责：协调空间索引、共享覆盖、前向检查与有限回溯；使用：一次独立候选；边界：搜索成功前不向 builder 添加实体，失败或预算耗尽不留下部分电塔。
type connectedPowerSolver struct {
	index          *connectedPowerIndex
	domains        []connectedPowerDomain
	choices        []connectedPowerChoice
	consumerCells  map[[2]int][]int
	choiceCells    map[[2]int][]int
	conflictRadius float64
	minimumSpacing float64
	selected       []int
	losses         []int
	lossEpoch      []int
	coverEpoch     []int
	epoch          int
	nodes          int
	evaluations    int
	forwardRejects int
	backtracks     int
}

// solveConnectedPower 职责：建立完整有限合法域并提交通过前向约束的共享塔组；使用：全局紧凑候选；边界：已有塔可覆盖的消费者不再造塔，求解后仍用原索引逐塔复核并登记。
func (b *builder) solveConnectedPower(index *connectedPowerIndex, consumers []connectedPowerConsumer, offsets []vector) error {
	solver, err := newConnectedPowerSolver(index, consumers, offsets)
	if err != nil {
		return err
	}
	complete, err := solver.search()
	if err != nil {
		return err
	}
	if !complete {
		return &blueprint.Error{Code: "GEN_E_POWER_COVERAGE", Message: fmt.Sprintf("全局供电有限约束求解无解：%d个独立耗电中心、%d个静态合法塔位、%d次分支、%d次前向拒绝、%d次回溯；原覆盖、电塔间距与全局净空均保持", len(solver.domains), len(solver.choices), solver.nodes, solver.forwardRejects, solver.backtracks)}
	}
	for _, number := range solver.selected {
		if err := b.checkGeneration("直连约束电塔提交"); err != nil {
			return err
		}
		point := solver.choices[number].point
		if point.z != 0 {
			return &blueprint.Error{Code: "GEN_E_POWER_COVERAGE", Message: "全局供电求解包含非地面感应塔候选"}
		}
		available, err := index.available(point)
		if err != nil {
			return err
		}
		if !available {
			return &blueprint.Error{Code: "GEN_E_POWER_COVERAGE", Message: "全局供电约束塔组未通过原精确占位索引的提交复核"}
		}
		pole := b.make(2201, point.x, point.y, point.z)
		if err := index.add(pole); err != nil {
			return err
		}
	}
	if b.logistics != nil {
		b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "connected-global-power-constraints", Status: "passed", Message: fmt.Sprintf("%d个独立耗电中心在%d个静态合法塔位中联合选择%d根感应塔；%d次前向拒绝、%d次回溯，保持三维覆盖、原电塔间距和全局净空，最终球面规则另行完整审计", len(solver.domains), len(solver.choices), len(solver.selected), solver.forwardRejects, solver.backtracks)})
	}
	return nil
}

// newConnectedPowerSolver 职责：以固定几何一次性枚举有限合法域并合并相同耗电中心；使用：搜索前；边界：不截断稀少消费者的合法塔位，不在动态试探中污染静态拒绝缓存。
func newConnectedPowerSolver(index *connectedPowerIndex, consumers []connectedPowerConsumer, offsets []vector) (*connectedPowerSolver, error) {
	solver := &connectedPowerSolver{index: index, consumerCells: map[[2]int][]int{}, choiceCells: map[[2]int][]int{}, minimumSpacing: math.Sqrt(float64(surfacePowerDistanceLimit(2201, 2201))) + 1e-4}
	solver.conflictRadius = solver.minimumSpacing
	for _, shape := range index.poleShapes {
		// 任意切线姿态的盒都包含在该世界半径中，因此 X/Z 桶宽相位不会漏掉球面塔盒互斥。
		center := math.Sqrt(float64(shape.Position[0])*float64(shape.Position[0]) + float64(shape.Position[1])*float64(shape.Position[1]) + float64(shape.Position[2])*float64(shape.Position[2]))
		x, y, z := float64(shape.Extents[0])+index.clearance, float64(shape.Extents[1]), float64(shape.Extents[2])+index.clearance
		solver.conflictRadius = math.Max(solver.conflictRadius, 2*(center+math.Sqrt(x*x+y*y+z*z)))
	}
	centers := map[placement.Vector]bool{}
	positions := map[vector]int{}
	for _, consumer := range consumers {
		if err := index.builder.checkGeneration("直连约束静态合法域"); err != nil {
			return nil, err
		}
		if centers[consumer.world] || index.coveredWorld(consumer.world) {
			continue
		}
		centers[consumer.world] = true
		domain := connectedPowerDomain{consumer: consumer}
		// 两套格网可能重合；塔位只计一次，防止合法域剩余数和前向约束被重复项放大。
		seenValues := map[int]bool{}
		domainNumber := len(solver.domains)
		// 所有消费者共用同一半格格网，避免以各自小数中心重复生成大量相邻塔位。
		origin := vector{math.Round(consumer.center.x*2) / 2, math.Round(consumer.center.y*2) / 2, 0}
		add := func(point vector) error {
			if point.z != 0 {
				return nil
			}
			covered, err := index.withinCover(point, consumer.world)
			if err != nil {
				return err
			}
			if !covered {
				return nil
			}
			number, exists := positions[point]
			if !exists {
				available, err := index.available(point)
				if err != nil {
					return err
				}
				if !available {
					return nil
				}
				number = len(solver.choices)
				positions[point] = number
				world, boxes, err := index.poleGeometry(point)
				if err != nil {
					return err
				}
				solver.choices = append(solver.choices, connectedPowerChoice{point: point, world: world, boxes: boxes})
				key := connectedPowerCell(world[0], world[2])
				solver.choiceCells[key] = append(solver.choiceCells[key], number)
			}
			if !seenValues[number] {
				seenValues[number] = true
				domain.values = append(domain.values, number)
			}
			return nil
		}
		for offsetNumber, offset := range offsets {
			if offsetNumber%64 == 0 && index.ctx.Err() != nil {
				return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "全局供电合法域枚举已取消或超时"}
			}
			point := vector{origin.x + offset.x, origin.y + offset.y, offset.z}
			if err := add(point); err != nil {
				return nil, err
			}
		}
		if len(domain.values) < 8 && !index.encoding.planar {
			// 球面补偿后的窄缝可能位于任意小数坐标。先保留共享格网，仍稀缺时再沿真实耗电中心的相对格网采样。
			// 后者恢复原始局部几何的平移相位；所有候选继续通过同一个米制覆盖和原生净空检查。
			limit := int(math.Ceil(index.coverRadius / planarGridLength * 4))
			for phase, center := range []vector{origin, {consumer.center.x, consumer.center.y, 0}} {
				if len(domain.values) >= 8 {
					break
				}
				for x := -limit; x <= limit; x++ {
					if index.ctx.Err() != nil {
						return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "球面补电窄缝细化已取消或超时"}
					}
					for y := -limit; y <= limit; y++ {
						if phase == 0 && x%2 == 0 && y%2 == 0 {
							continue
						}
						point := vector{center.x + float64(x)/4, center.y + float64(y)/4, 0}
						if err := add(point); err != nil {
							return nil, err
						}
					}
				}
			}
		}
		domain.remaining = len(domain.values)
		solver.domains = append(solver.domains, domain)
		world := consumer.world
		key := connectedPowerCell(world[0], world[2])
		solver.consumerCells[key] = append(solver.consumerCells[key], domainNumber)
	}
	if err := solver.shareGroundChoices(); err != nil {
		return nil, err
	}
	for _, domain := range solver.domains {
		if domain.remaining == 0 {
			consumer := domain.consumer
			return nil, &blueprint.Error{Code: "GEN_E_POWER_COVERAGE", Message: fmt.Sprintf("建筑 %d（物品 %d，产线 %d）的球面耗电中心(%.2f,%.2f,%.2f)在固定单区内没有满足真实球面覆盖、碰撞和塔距的地面候选塔位", consumer.number, consumer.entity.record.ItemID, consumer.entity.owner, consumer.center.x, consumer.center.y, consumer.center.z)}
		}
	}
	solver.losses, solver.lossEpoch, solver.coverEpoch = make([]int, len(solver.domains)), make([]int, len(solver.domains)), make([]int, len(solver.domains))
	return solver, nil
}

// shareGroundChoices 职责：把全局合法地面塔位纳入其实际覆盖的全部消费者域；使用：静态枚举结束、前向检查之前；边界：只共享已有且几何合法的点，不能把各中心不同采样相位遗漏的点误判为不可用。
func (solver *connectedPowerSolver) shareGroundChoices() error {
	for number := range solver.choices {
		solver.choices[number].domains = nil
	}
	limitSquared := (solver.index.coverRadius - 1e-4) * (solver.index.coverRadius - 1e-4)
	for domainNumber := range solver.domains {
		if err := solver.index.builder.checkGeneration("直连地面塔位共享覆盖"); err != nil {
			return err
		}
		domain := &solver.domains[domainNumber]
		domain.values = domain.values[:0]
		world := domain.consumer.world
		minimum := connectedPowerCell(world[0]-solver.index.coverRadius, world[2]-solver.index.coverRadius)
		maximum := connectedPowerCell(world[0]+solver.index.coverRadius, world[2]+solver.index.coverRadius)
		for x := minimum[0]; x <= maximum[0]; x++ {
			for y := minimum[1]; y <= maximum[1]; y++ {
				for _, number := range solver.choiceCells[[2]int{x, y}] {
					delta := solver.choices[number].world.Sub(world)
					if delta.Dot(delta) <= limitSquared {
						domain.values = append(domain.values, number)
						solver.choices[number].domains = append(solver.choices[number].domains, domainNumber)
					}
				}
			}
		}
		domain.remaining = len(domain.values)
	}
	return nil
}

// prepare 职责：只为实际考虑的塔位建立附近互斥和共享三维覆盖；使用：候选评分和试探；边界：空间桶只缩小查询范围，最终判定保留精确距离和实际盒重叠。
func (solver *connectedPowerSolver) prepare(number int) error {
	if solver.choices[number].prepared {
		return nil
	}
	world := solver.choices[number].world
	minimum := connectedPowerCell(world[0]-solver.conflictRadius, world[2]-solver.conflictRadius)
	maximum := connectedPowerCell(world[0]+solver.conflictRadius, world[2]+solver.conflictRadius)
	for x := minimum[0]; x <= maximum[0]; x++ {
		for y := minimum[1]; y <= maximum[1]; y++ {
			for _, other := range solver.choiceCells[[2]int{x, y}] {
				if solver.choicesConflict(number, other) {
					solver.choices[number].conflicts = append(solver.choices[number].conflicts, other)
				}
			}
		}
	}
	minimum = connectedPowerCell(world[0]-solver.index.coverRadius, world[2]-solver.index.coverRadius)
	maximum = connectedPowerCell(world[0]+solver.index.coverRadius, world[2]+solver.index.coverRadius)
	for x := minimum[0]; x <= maximum[0]; x++ {
		for y := minimum[1]; y <= maximum[1]; y++ {
			for _, consumer := range solver.consumerCells[[2]int{x, y}] {
				delta := world.Sub(solver.domains[consumer].consumer.world)
				if delta.Dot(delta) <= (solver.index.coverRadius-1e-4)*(solver.index.coverRadius-1e-4) {
					solver.choices[number].covers = append(solver.choices[number].covers, consumer)
				}
			}
		}
	}
	solver.choices[number].prepared = true
	return solver.index.builder.checkGeneration("直连约束局部互斥登记")
}

// choicesConflict 职责：按原电塔间距及预排盒判断两个塔位能否共同选中；使用：互斥图局部窄相位；边界：地面塔不降低三维距离阈值，预排净空仍作用于真实盒。
func (solver *connectedPowerSolver) choicesConflict(first, second int) bool {
	left, right := solver.choices[first], solver.choices[second]
	delta := left.world.Sub(right.world)
	if delta.Dot(delta) < solver.minimumSpacing*solver.minimumSpacing {
		return true
	}
	for _, leftBox := range left.boxes {
		for _, rightBox := range right.boxes {
			if leftBox.Overlaps(rightBox) {
				return true
			}
		}
	}
	return false
}

// score 职责：拒绝将任何未覆盖域清空的塔位并优先选择共享稀缺覆盖；使用：确定性分支排序；边界：只读取动态计数，试探评分不会污染当前可用域。
func (solver *connectedPowerSolver) score(number int) (int64, bool, error) {
	solver.evaluations++
	if solver.evaluations > 500000 {
		return 0, false, &blueprint.Error{Code: "GEN_E_POWER_COVERAGE", Message: "全局供电约束候选评分达到500000次有限预算，保留原规则并回退其它布局候选"}
	}
	if err := solver.prepare(number); err != nil {
		return 0, false, err
	}
	solver.epoch++
	epoch := solver.epoch
	var benefit, penalty int64
	for _, domainNumber := range solver.choices[number].covers {
		solver.coverEpoch[domainNumber] = epoch
		domain := &solver.domains[domainNumber]
		if domain.covered == 0 {
			benefit += 1000000/int64(domain.remaining+4) + 1000
		}
	}
	for _, conflict := range solver.choices[number].conflicts {
		if solver.choices[conflict].blocked != 0 {
			continue
		}
		for _, domainNumber := range solver.choices[conflict].domains {
			domain := &solver.domains[domainNumber]
			if domain.covered != 0 || solver.coverEpoch[domainNumber] == epoch {
				continue
			}
			if solver.lossEpoch[domainNumber] != epoch {
				solver.lossEpoch[domainNumber], solver.losses[domainNumber] = epoch, 0
			}
			solver.losses[domainNumber]++
			if solver.losses[domainNumber] >= domain.remaining {
				solver.forwardRejects++
				return 0, false, nil
			}
			penalty += 10000 / int64(domain.remaining+1)
		}
	}
	return benefit - penalty, true, nil
}

// apply 职责：可逆更新塔位互斥和真实覆盖计数；使用：深度优先试探及回退；边界：仅零到非零的阻塞转换改变域容量，多个已选塔共同阻塞时不能过早恢复。
func (solver *connectedPowerSolver) apply(number, direction int) {
	for _, domain := range solver.choices[number].covers {
		solver.domains[domain].covered += direction
	}
	for _, conflict := range solver.choices[number].conflicts {
		choice := &solver.choices[conflict]
		previous := choice.blocked
		choice.blocked += direction
		if previous == 0 || choice.blocked == 0 {
			for _, domain := range choice.domains {
				solver.domains[domain].remaining -= direction
			}
		}
	}
}

// search 职责：先解决最稀缺未覆盖域并在必要时有限回溯；使用：全局塔组构造；边界：最多8192个搜索节点、每层32个优先分支，失败交回独立布局候选而不放松规则。
func (solver *connectedPowerSolver) search() (bool, error) {
	if err := solver.index.builder.checkGeneration("直连约束前向搜索"); err != nil {
		return false, err
	}
	solver.nodes++
	if solver.nodes > 8192 {
		return false, &blueprint.Error{Code: "GEN_E_POWER_COVERAGE", Message: "全局供电约束搜索达到8192个有限节点预算，保留原规则并回退其它布局候选"}
	}
	target := -1
	for number := range solver.domains {
		domain := &solver.domains[number]
		if domain.covered != 0 {
			continue
		}
		if domain.remaining == 0 {
			return false, nil
		}
		if target < 0 || domain.remaining < solver.domains[target].remaining {
			target = number
		}
	}
	if target < 0 {
		return true, nil
	}
	type rankedChoice struct {
		number int
		score  int64
	}
	options := make([]rankedChoice, 0, solver.domains[target].remaining)
	for _, number := range solver.domains[target].values {
		if solver.choices[number].blocked != 0 {
			continue
		}
		score, safe, err := solver.score(number)
		if err != nil {
			return false, err
		}
		if safe {
			options = append(options, rankedChoice{number, score})
		}
	}
	sort.SliceStable(options, func(i, j int) bool { return options[i].score > options[j].score })
	if len(options) > 32 {
		options = options[:32]
	}
	for _, option := range options {
		solver.apply(option.number, 1)
		solver.selected = append(solver.selected, option.number)
		complete, err := solver.search()
		if complete {
			return true, nil
		}
		solver.selected = solver.selected[:len(solver.selected)-1]
		solver.apply(option.number, -1)
		solver.backtracks++
		if err != nil {
			return false, err
		}
	}
	return false, nil
}
