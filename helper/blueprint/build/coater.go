package build

// 文件职责：把供剂支路按等级连接到主干；使用：needCoaterNet；边界：仅连接已登记喷涂口，外部入口仍需供给增产剂。

import (
	"DysonSphereBox/helper/blueprint"
	"context"
	"math"
	"sort"
)

// coaterNetwork 职责：按同一 y 合并支路并以分拣器分配供剂；使用：块布局完成后；边界：不同剂种采用独立高度，不把多条支路写在同一格。
func (b *builder) coaterNetwork(ctx context.Context) error {
	if len(b.coaterLinks) == 0 {
		return nil
	}
	minimum, maximum := bounds(b.all)
	// 先估算主干/支路/下降段，避免在可回滚失败之前分配巨型网络。
	estimate := len(b.all)
	for level, id := range []int{1141, 1142, 1143} {
		links := b.coaterLinks[id]
		if len(links) == 0 {
			continue
		}
		estimate += int(math.Ceil(maximum.y-minimum.y)) + 6
		rows := map[float64]float64{}
		for _, link := range links {
			rows[link.a.y] = math.Max(rows[link.a.y], link.a.x)
			estimate += 8 + level*2
		}
		for _, end := range rows {
			estimate += int(math.Ceil(end-minimum.x)) + 4
		}
	}
	if estimate > 100000 {
		return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "喷涂供给网络会使建筑超过 10 万，请拆分生成"}
	}
	owner := b.owner
	b.owner = -1
	defer func() { b.owner = owner }()
	for level, id := range []int{1141, 1142, 1143} {
		links := b.coaterLinks[id]
		if len(links) == 0 {
			continue
		}
		sort.SliceStable(links, func(i, j int) bool {
			if links[i].a.y != links[j].a.y {
				return links[i].a.y < links[j].a.y
			}
			return links[i].a.x < links[j].a.x
		})
		x := math.Floor(minimum.x) - 3
		z := float64(1 + level*2)
		startY := math.Floor(minimum.y) - 3
		endY := math.Ceil(maximum.y) + 2
		rail := b.straight(x, startY, int(endY-startY)+1, 1)
		shift(rail, 0, 0, z)
		rail[0].record.Parameters = []int32{int32(id), 0}
		for begin := 0; begin < len(links); {
			if ctx.Err() != nil {
				return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "喷涂网络生成超时"}
			}
			end := begin + 1
			for end < len(links) && links[end].a.y == links[begin].a.y {
				end++
			}
			y := links[begin].a.y - 2
			branch := b.straight(x+1, y, int(math.Round(links[end-1].a.x-x)), 2)
			shift(branch, 0, 0, z)
			if _, err := b.beltTransfer(rail[int(math.Round(y-startY))], branch[0], b.setting.InserterLv, id); err != nil {
				return err
			}
			for _, link := range links[begin:end] {
				from := branch[int(math.Round(link.a.x-(x+1)))]
				if z == 1 {
					if _, err := b.beltTransfer(from, link, b.setting.InserterLv, id); err != nil {
						return err
					}
					continue
				}
				points := []vector{}
				for h := z; h >= 1; h-- {
					points = append(points, vector{link.a.x + .002*(h-1), link.a.y - 1, h})
				}
				drop := b.byPath(points)
				connect(drop[len(drop)-1], link)
				if _, err := b.beltTransfer(from, drop[0], b.setting.InserterLv, id); err != nil {
					return err
				}
			}
			begin = end
		}
	}
	return nil
}
