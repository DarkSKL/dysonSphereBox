package galaxy

// 文件职责：由存档生成参数恢复完整天体关系和保存时刻坐标。
// 使用：解析后构建只读世界索引；边界：不读取游戏安装，不生成地形和资源。
import (
	"fmt"
	"math"
)

// Vector 职责：保存三维坐标；使用：天体位置；边界：单位由调用契约明确。
type Vector struct {
	// X 职责：保存 X 轴分量；使用：位置运算；边界：继承所属坐标系。
	X float64 `json:"x"`
	// Y 职责：保存 Y 轴分量；使用：位置运算；边界：继承所属坐标系。
	Y float64 `json:"y"`
	// Z 职责：保存 Z 轴分量；使用：位置运算；边界：继承所属坐标系。
	Z float64 `json:"z"`
}

// Star 职责：保存恢复后的恒星；使用：完整世界查询；边界：名称不含玩家覆盖。
type Star struct {
	// ID 职责：保存生成天体标识；使用：构建世界索引；边界：不是存储池索引。
	ID int `json:"id"`
	// Name 职责：保存最终显示名称；使用：查询展示；边界：天体自身覆盖名优先。
	Name string `json:"name"`
	// Position 职责：保存保存时刻的三轴位置；使用：空间关联；边界：单位和原点由世界契约给定。
	Position Vector `json:"position"`
}

// Planet 职责：保存恢复后的星球；使用：所属关系和位置查询；边界：位置使用游戏米，原点为星系原点。
type Planet struct {
	// ID 职责：保存生成天体标识；使用：构建世界索引；边界：不是存储池索引。
	ID int `json:"id"`
	// StarID 职责：保存所属恒星标识；使用：关联星球与球体；边界：从 1 开始，不是数组下标。
	StarID int `json:"starId"`
	// ParentPlanetID 职责：保存卫星的父行星标识；使用：轨道关系；边界：零表示直接绕恒星运行。
	ParentPlanetID int `json:"parentPlanetId"`
	// Name 职责：保存最终显示名称；使用：查询展示；边界：天体自身覆盖名优先。
	Name string `json:"name"`
	// Position 职责：保存保存时刻的三轴位置；使用：空间关联；边界：单位和原点由世界契约给定。
	Position Vector `json:"position"`
	// OrbitRadius 职责：保存轨道半径；使用：位置恢复核对；边界：单位为游戏 AU。
	OrbitRadius float64 `json:"orbitRadius"`
	// OrbitInclination 职责：保存轨道倾角；使用：轨道旋转；边界：单位为度。
	OrbitInclination float64 `json:"orbitInclination"`
	// OrbitLongitude 职责：保存轨道经度；使用：轨道旋转；边界：单位为度。
	OrbitLongitude float64 `json:"orbitLongitude"`
	// OrbitPhase 职责：保存初始轨道相位；使用：保存时刻预测；边界：单位为度。
	OrbitPhase float64 `json:"orbitPhase"`
	// OrbitalPeriod 职责：保存公转周期；使用：相位计算；边界：单位为游戏秒。
	OrbitalPeriod float64 `json:"orbitalPeriod"`
}

// Generate 职责：执行受支持的确定性星系恢复；使用：传入原始生成参数和 tick；边界：未知算法返回错误，不用占位坐标。
func Generate(seed int32, algorithm, count, lcid int, tick int64) ([]Star, []Planet, error) {
	if algorithm != 20200403 || count < 32 || count > 64 || tick < 0 {
		return nil, nil, fmt.Errorf("不支持的星系生成参数")
	}
	r := newRandom(seed)
	points := positions(r.integer(), count)
	if len(points) != count {
		return nil, nil, fmt.Errorf("星系位置数量与存档不一致")
	}
	rolls := [4]float64{single(r.unit()), single(r.unit()), single(r.unit()), single(r.unit())}
	quota := func(rate, spread, roll float64) int {
		return int(math.Ceil(single(single(single(rate)*float64(count)) + single(roll*single(spread)))))
	}
	black := count - quota(.01, .3, rolls[0])
	neutron := black - quota(.01, .3, rolls[1])
	dwarf := neutron - quota(.016, .4, rolls[2])
	giants := quota(.013, 1.4, rolls[3])
	spacing := (dwarf - 1) / giants
	stars := []Star{}
	planets := []Planet{}
	names := map[string]bool{}
	for i, p := range points {
		kind, spectr := 0, 7
		if i == 3 {
			spectr = 0
		} else if i == dwarf-1 {
			spectr = 6
		}
		if i > 0 {
			if spacing > 0 && i%spacing == spacing/2 {
				kind = 1
			}
			if i >= black {
				kind = 4
			} else if i >= neutron {
				kind = 3
			} else if i >= dwarf {
				kind = 2
			}
		}
		s := makeStar(i+1, count, kind, spectr, r.integer(), p)
		s.name = s.starName(lcid, names)
		s.makePlanets()
		pos := Vector{p.X * 2400000, p.Y * 2400000, p.Z * 2400000}
		stars = append(stars, Star{s.id, s.name, pos})
		for j, v := range s.planets {
			relative := s.relativePosition(j, float64(tick)/60)
			parent := 0
			if v.parent >= 0 {
				parent = s.id*100 + v.parent + 1
			}
			name := fmt.Sprintf("%s %s", s.name, roman[j+1])
			if lcid == 2052 {
				name = fmt.Sprintf("%s - %d号星", s.name, j+1)
			}
			planets = append(planets, Planet{s.id*100 + j + 1, s.id, parent, name, Vector{pos.X + relative.X*40000, pos.Y + relative.Y*40000, pos.Z + relative.Z*40000}, v.radius, v.inclination, v.longitude, v.phase, v.period})
		}
	}
	return stars, planets, nil
}
