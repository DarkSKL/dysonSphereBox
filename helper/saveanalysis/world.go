package saveanalysis

// 文件职责：恢复完整恒星、星球关系、名称和保存时刻坐标。
// 使用：选档后的世界查询；边界：生成参数和命名覆盖来自存档，坐标由 Go 恢复，不使用厂区列表代替星系。
import (
	"DysonSphereBox/helper/saveanalysis/galaxy"
	"fmt"
)

// StarLink 职责：描述恒星身份；使用：星球和球体关联；边界：基础名与玩家覆盖分开。
type StarLink struct {
	// StarID 职责：保存所属恒星标识；使用：关联星球与球体；边界：从 1 开始，不是数组下标。
	StarID int64 `json:"starId"`
	// Name 职责：保存最终显示名称；使用：查询展示；边界：天体自身覆盖名优先。
	Name string `json:"name"`
	// GeneratedName 职责：保存未应用本天体覆盖的默认名称；使用：说明名称来源；边界：星球默认名可包含恒星覆盖名。
	GeneratedName string `json:"generatedName"`
	// OverrideName 职责：保留存档自定义名；使用：区分玩家命名；边界：空字符串表示未覆盖。
	OverrideName string `json:"overrideName"`
	// Position 职责：保存保存时刻的三轴位置；使用：空间关联；边界：单位和原点由世界契约给定。
	Position galaxy.Vector `json:"position"`
}

// PlanetLink 职责：描述全部星球；使用：关联厂区与卫星；边界：未建厂星球也有真实生成位置。
type PlanetLink struct {
	// PlanetID 职责：保存星球原始标识；使用：库存关联；边界：包含未建厂星球。
	PlanetID int64 `json:"planetId"`
	// StarID 职责：保存所属恒星标识；使用：关联星球与球体；边界：从 1 开始，不是数组下标。
	StarID int64 `json:"starId"`
	// ParentPlanetID 职责：保存卫星的父行星标识；使用：轨道关系；边界：零表示直接绕恒星运行。
	ParentPlanetID int64 `json:"parentPlanetId"`
	// Name 职责：保存最终显示名称；使用：查询展示；边界：天体自身覆盖名优先。
	Name string `json:"name"`
	// GeneratedName 职责：保存未应用本天体覆盖的默认名称；使用：说明名称来源；边界：星球默认名可包含恒星覆盖名。
	GeneratedName string `json:"generatedName"`
	// OverrideName 职责：保留存档自定义名；使用：区分玩家命名；边界：空字符串表示未覆盖。
	OverrideName string `json:"overrideName"`
	// HasFactory 职责：区分已建厂与未建厂；使用：判断库存覆盖；边界：不是是否存在星球的标记。
	HasFactory bool `json:"hasFactory"`
	// Position 职责：保存保存时刻的三轴位置；使用：空间关联；边界：单位和原点由世界契约给定。
	Position galaxy.Vector `json:"position"`
}

// WorldView 职责：描述固定快照的完整世界；使用：定位查询；边界：坐标为游戏尺度，不是现实天文尺度。
type WorldView struct {
	// SHA256 职责：关联固定内容快照；使用：跨查询核对；边界：不是路径或 saveId。
	SHA256 string `json:"sha256"`
	// Tick 职责：保存游戏时刻；使用：位置及统计核对；边界：不跟随游戏实时推进。
	Tick int64 `json:"tick"`
	// Seed 职责：保留存档星系种子；使用：恢复生成；边界：必须结合算法及恒星数量。
	Seed int64 `json:"seed"`
	// Algorithm 职责：保留星系算法编号；使用：限制恢复范围；边界：不是游戏发行版本。
	Algorithm int64 `json:"algorithm"`
	// StarCount 职责：保留恒星总量；使用：覆盖检查；边界：不等于厂区数量。
	StarCount int64 `json:"starCount"`
	// NameLCID 职责：保留存档星名语言标识；使用：确定名称候选及编号；边界：不读取当前游戏界面语言。
	NameLCID int64 `json:"nameLCID"`
	// CompletePlanets 职责：说明完整星球覆盖；使用：调用方判断清单；边界：只有生成与存档关联校验通过才为真。
	CompletePlanets bool `json:"completePlanets"`
	// PositionUnit 职责：说明坐标长度单位；使用：位置换算；边界：game_m 是游戏米。
	PositionUnit string `json:"positionUnit"`
	// PositionOrigin 职责：说明坐标原点；使用：空间换算；边界：galaxy_origin 是星系原点。
	PositionOrigin string `json:"positionOrigin"`
	// GameMetersPerAU 职责：保存游戏天文单位换算；使用：轨道尺度换算；边界：不是现实 AU。
	GameMetersPerAU int64 `json:"gameMetersPerAU"`
	// GameMetersPerLightYear 职责：保存游戏光年换算；使用：星际尺度换算；边界：不是现实光年。
	GameMetersPerLightYear int64 `json:"gameMetersPerLightYear"`
	// TicksPerSecond 职责：保存游戏时间换算；使用：解释 Tick；边界：不是现实帧率。
	TicksPerSecond int64 `json:"ticksPerSecond"`
	// PositionSource 职责：说明坐标恢复来源；使用：识别生成结果；边界：不是逐天体直接存储值。
	PositionSource string `json:"positionSource"`
	// NameSource 职责：说明生成与命名覆盖来源；使用：审视结果；边界：不猜测玩家名称。
	NameSource string `json:"nameSource"`
	// Stars 职责：返回全部恒星；使用：世界与球体关联；边界：按原标识排列。
	Stars []StarLink `json:"stars"`
	// Planets 职责：返回全部星球；使用：厂区及卫星关联；边界：不局限已建厂列表。
	Planets []PlanetLink `json:"planets"`
	// Missing 职责：列出缺失项；使用：解释结果覆盖；边界：成功的完整世界返回空列表。
	Missing []string `json:"missing"`
}

// buildWorld 职责：恢复世界并验证保存的关联；使用：完整解码后；边界：未知算法或矛盾不发布部分结果。
func (r *reader) buildWorld(s *Snapshot) {
	desc := r.world.gameDesc
	stars, planets, err := galaxy.Generate(int32(s.seed), int(s.algorithm), int(s.starCount), int(desc.starNameLCID), s.tick)
	if err != nil {
		r.fail(err.Error())
	}
	out := WorldView{SHA256: s.digest, Tick: s.tick, Seed: s.seed, Algorithm: s.algorithm, StarCount: s.starCount, NameLCID: desc.starNameLCID, CompletePlanets: true, PositionUnit: "game_m", PositionOrigin: "galaxy_origin", GameMetersPerAU: 40000, GameMetersPerLightYear: 2400000, TicksPerSecond: 60, PositionSource: "generated_at_save_tick", NameSource: "generated_with_saved_overrides", Stars: []StarLink{}, Planets: []PlanetLink{}, Missing: []string{}}
	known := map[int64]bool{}
	factories := map[int64]bool{}
	starNames := map[int]string{}
	for _, id := range s.planetIDs {
		factories[id] = true
	}
	for _, star := range stars {
		override := r.astroNames[int64(star.ID*100)]
		name := star.Name
		if override != "" {
			name = override
		}
		starNames[star.ID] = name
		known[int64(star.ID*100)] = true
		out.Stars = append(out.Stars, StarLink{int64(star.ID), name, star.Name, override, star.Position})
	}
	for _, p := range planets {
		id := int64(p.ID)
		known[id] = true
		// 默认星球名随所属恒星的显示名变化；星球覆盖名优先级最高。
		generated := starNames[p.StarID] + p.Name[len(stars[p.StarID-1].Name):]
		override := r.astroNames[id]
		name := generated
		if override != "" {
			name = override
		}
		out.Planets = append(out.Planets, PlanetLink{id, int64(p.StarID), int64(p.ParentPlanetID), name, generated, override, factories[id], p.Position})
	}
	for _, id := range s.planetIDs {
		if !known[id] {
			r.fail("厂区星球不存在于生成星系")
		}
	}
	seen := map[int64]bool{}
	for _, id := range r.worldPlanetIDs {
		if id%100 == 0 || !known[id] || seen[id] {
			r.fail("存档天体清单与生成星系不一致")
		}
		seen[id] = true
	}
	if len(seen) != len(planets) {
		r.fail(fmt.Sprintf("存档天体清单数量不一致：%d/%d", len(seen), len(planets)))
	}
	for id := range r.astroNames {
		if !known[id] {
			r.fail("自定义名称指向不存在的天体")
		}
	}
	s.worldView = out
}

// World 职责：返回固定快照完整世界；使用：当前选档查询；边界：所有切片为独立副本。
func (s *Snapshot) World() WorldView {
	out := s.worldView
	out.Stars = append([]StarLink{}, out.Stars...)
	out.Planets = append([]PlanetLink{}, out.Planets...)
	out.Missing = append([]string{}, out.Missing...)
	return out
}
