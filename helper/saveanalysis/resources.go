package saveanalysis

// 文件职责：提供存档星球资源的只读概要与空间数据。
// 使用：产业链规划与球面布局；边界：各字段标明来源或缺失情况。
import (
	"DysonSphereBox/helper/saveanalysis/galaxy"
	"context"
	"fmt"
	"math"
	"slices"
	"sort"
)

// PlanetResourceView 职责：返回全星系资源概要；使用：跨球选址；边界：包含数据未知的星球。
type PlanetResourceView struct {
	SHA256      string                  `json:"sha256"`
	Tick        int64                   `json:"tick"`
	GameVersion string                  `json:"gameVersion"`
	Planets     []PlanetResourceSummary `json:"planets"`
}

// PlanetResourceSummary 职责：返回单球资源概要；使用：按物品规划；边界：空清单须结合来源和完整性解释。
type PlanetResourceSummary struct {
	PlanetID  int64            `json:"planetId"`
	StarID    int64            `json:"starId"`
	Source    string           `json:"source"`
	Gas       bool             `json:"gas"`
	Resources []PlanetResource `json:"resources"`
}

// PlanetResource 职责：保存一个物品的资源指标；使用：供给评价；边界：数量和开采速率采用独立字段。
type PlanetResource struct {
	ItemID         int64    `json:"itemId"`
	Kind           string   `json:"kind"`
	Unit           string   `json:"unit"`
	Source         string   `json:"source"`
	Amount         *int64   `json:"amount"`
	Count          *int64   `json:"count"`
	Rate           *float64 `json:"rate"`
	OccupiedCount  *int64   `json:"occupiedCount"`
	OccupiedAmount *int64   `json:"occupiedAmount"`
	OccupiedRate   *float64 `json:"occupiedRate"`
	// AvailableCount、AvailableAmount、AvailableRate 职责：扣除占用及确定不可采的深埋点；使用：粗选址；边界：nil 表示没有逐点可用性记录。
	AvailableCount  *int64   `json:"availableCount"`
	AvailableAmount *int64   `json:"availableAmount"`
	AvailableRate   *float64 `json:"availableRate"`
	Infinite        bool     `json:"infinite"`
}

// PlanetResourceDetail 职责：定位矿点和存档占用；使用：单球布局；边界：Position 为星球局部 Unity 坐标，单位为游戏米。
type PlanetResourceDetail struct {
	SHA256 string                `json:"sha256"`
	Tick   int64                 `json:"tick"`
	Planet PlanetResourceSummary `json:"planet"`
	// VeinSource 职责：区分现存矿点与种子初始矿点；使用：空间布局与可用性复核；边界：种子位置未包含工厂局部化或后续改造。
	VeinSource     string            `json:"veinSource"`
	BuildingSource string            `json:"buildingSource"`
	Veins          []PlanetVein      `json:"veins"`
	Groups         []PlanetVeinGroup `json:"groups"`
	Buildings      []PlanetBuilding  `json:"buildings"`
	RelayZones     []RelayZone       `json:"relayZones"`
	Terrain        PlanetTerrain     `json:"terrain"`
}

// PlanetVein 职责：保存矿点状态；使用：采矿设备定位；边界：ID、组和关联实体 ID 属于当前星球。
type PlanetVein struct {
	ID             int64         `json:"id"`
	GroupID        int64         `json:"groupId"`
	ItemID         int64         `json:"itemId"`
	VeinType       int64         `json:"veinType"`
	Position       galaxy.Vector `json:"position"`
	Amount         int64         `json:"amount"`
	Rate           *float64      `json:"rate"`
	MinerEntityIDs []int64       `json:"minerEntityIds"`
	// PredictedBurial 标明已知基地落地时可能埋藏的矿点，保留原始坐标供复核。
	PredictedBurial bool `json:"predictedBurial"`
}

// IsSurfaceMiningBlocked 排除深埋矿点及已知基地落地避让区内的矿点。
func (v PlanetVein) IsSurfaceMiningBlocked() bool {
	return v.IsDeepBuried() || v.PredictedBurial
}

// IsDeepBuried 职责：识别确定超出表面矿机近邻范围的矿点；使用：粗选址和采集落位；边界：普通地形高差和浅水不按埋矿处理。
func (v PlanetVein) IsDeepBuried() bool {
	// 原版粘贴检查以球面落点做三维近邻搜索，大小矿机及油井的初筛半径至多 18 米。
	// 原版固态星球名义半径为 200 米，取低于 180 米作保守判别，涵盖原版 50/60 米埋矿。
	const minimumSurfaceRadius = 180
	x, y, z := v.Position.X, v.Position.Y, v.Position.Z
	squared := x*x + y*y + z*z
	return squared > 0 && squared < minimumSurfaceRadius*minimumSurfaceRadius
}

// PlanetVeinGroup 职责：保存扫描矿群；使用：没有矿点详情时作概览；边界：坐标为游戏记录的组方向。
type PlanetVeinGroup struct {
	ID       int64         `json:"id"`
	VeinType int64         `json:"veinType"`
	Position galaxy.Vector `json:"position"`
	Count    int64         `json:"count"`
	Amount   int64         `json:"amount"`
}

// PlanetBuilding 职责：保存建筑两端位姿；使用：避让和连接；边界：预建造和已建实体各有独立 ID 域。
type PlanetBuilding struct {
	ID          int64         `json:"id"`
	ItemID      int64         `json:"itemId"`
	Prebuild    bool          `json:"prebuild"`
	Position    galaxy.Vector `json:"position"`
	Rotation    [4]float64    `json:"rotation"`
	EndPosition galaxy.Vector `json:"endPosition"`
	EndRotation [4]float64    `json:"endRotation"`
}

// PlanetTerrain 职责：保留存档地形改造和观测信息；使用：地表布局；边界：压缩阵列遵循存档原格式。
type PlanetTerrain struct {
	ModificationSource string   `json:"modificationSource"`
	WaterSource        string   `json:"waterSource"`
	WaterItemID        *int64   `json:"waterItemId"`
	WaterHeight        *float64 `json:"waterHeight"`
	HeightSource       string   `json:"heightSource"`
	HeightPrecision    int      `json:"heightPrecision"`
	HeightCentimeters  []uint16 `json:"heightCentimeters"`
	ModData            []byte   `json:"modData"`
	ReformData         []byte   `json:"reformData"`
	ReformOffsets      []int64  `json:"reformOffsets"`
	LandPercent        *float64 `json:"landPercent"`
}

// HeightAt 职责：查询包含存档改造的地表高度；使用：地基和水面布局；边界：返回相对于星球半径的游戏米。
func (t PlanetTerrain) HeightAt(position galaxy.Vector) (float64, bool) {
	if t.HeightSource != "generated" {
		return 0, false
	}
	height, ok := galaxy.TerrainHeightAt(galaxy.HeightGrid{Precision: t.HeightPrecision, Heights: t.HeightCentimeters}, t.ModData, position)
	if !ok {
		return 0, false
	}
	radius := 200.0
	if t.HeightPrecision == 64 {
		radius = 80
	}
	return height - radius, true
}

// planetSurface 职责：保留厂区空间数据；使用：解码和详情查询；边界：快照建成后只读。
type planetSurface struct {
	veins     []PlanetVein
	buildings []PlanetBuilding
	terrain   PlanetTerrain
}

// scannedPlanet 职责：保留扫描数据；使用：概要查询；边界：开采后的实时矿量优先取厂区。
type scannedPlanet struct {
	landPercent float64
	groups      []PlanetVeinGroup
}

// readSurfaceBytes 职责：保留地形字节；使用：已启用空间记录的快照；边界：独立于存档输入缓冲。
func (r *reader) readSurfaceBytes(n int64) []byte {
	start := r.offset
	r.skip(n)
	if r.surface == nil {
		return nil
	}
	return slices.Clone(r.data[start:r.offset])
}

// vector 职责：解码局部坐标；使用：空间快照；边界：保留原始单精度值。
func (v *layoutVector3) vector() galaxy.Vector {
	return galaxy.Vector{X: float64(floatValue(v.x)), Y: float64(floatValue(v.y)), Z: float64(floatValue(v.z))}
}

// rotation 职责：解码局部旋转；使用：空间快照；边界：四元数分量顺序为 XYZW。
func (v *layoutQuaternion) rotation() [4]float64 {
	return [4]float64{float64(floatValue(v.x)), float64(floatValue(v.y)), float64(floatValue(v.z)), float64(floatValue(v.w))}
}

// PlanetResources 职责：获取全星球资源概要；使用：产业链选址；边界：从同一快照返回独立数据。
func (s *Snapshot) PlanetResources() PlanetResourceView {
	return s.planetResourceView(true)
}

// PlanetResourceCatalog 职责：返回全星系已知资源和候选矿种；使用：选档和初选；边界：候选不执行高度或矿点生成。
func (s *Snapshot) PlanetResourceCatalog() PlanetResourceView {
	return s.planetResourceView(false)
}

// planetResourceView 职责：统一构造星系资源视图；使用：完整及快速查询；边界：由参数选择是否逐球计算。
func (s *Snapshot) planetResourceView(resolve bool) PlanetResourceView {
	out := PlanetResourceView{s.digest, s.tick, s.gameVersion, make([]PlanetResourceSummary, 0, len(s.worldView.Planets))}
	for _, planet := range s.worldView.Planets {
		summary, _ := s.planetResourceSummary(context.Background(), planet, resolve, galaxy.HeightGrid{})
		out.Planets = append(out.Planets, summary)
	}
	return out
}

// ResolvePlanetResources 职责：按星球生成候选矿点并汇总；使用：规划预算内按需精化；边界：取消时不缓存部分结果。
func (s *Snapshot) ResolvePlanetResources(ctx context.Context, planetID int64) (PlanetResourceSummary, error) {
	if err := ctx.Err(); err != nil {
		return PlanetResourceSummary{}, err
	}
	if cached, found := s.CachedPlanetResources(planetID); found {
		return cached, nil
	}
	for _, planet := range s.worldView.Planets {
		if planet.PlanetID == planetID {
			summary, err := s.planetResourceSummary(ctx, planet, true, galaxy.HeightGrid{})
			if err != nil {
				return PlanetResourceSummary{}, err
			}
			if err := ctx.Err(); err != nil {
				return PlanetResourceSummary{}, err
			}
			s.resolvedResources.LoadOrStore(planetID, clonePlanetResourceSummary(summary))
			return summary, nil
		}
	}
	return PlanetResourceSummary{}, fmt.Errorf("存档中不存在星球 %d", planetID)
}

// CachedPlanetResources 职责：查找已完成的单球精算；使用：规划先查缓存再分配预算；边界：命中可返回未知概要，结果可由调用方独立修改。
func (s *Snapshot) CachedPlanetResources(planetID int64) (PlanetResourceSummary, bool) {
	cached, found := s.resolvedResources.Load(planetID)
	if !found {
		return PlanetResourceSummary{}, false
	}
	return clonePlanetResourceSummary(cached.(PlanetResourceSummary)), true
}

// clonePlanetResourceSummary 职责：复制概要及其可选数值；使用：隔离缓存与查询结果；边界：保留缺失数值。
func clonePlanetResourceSummary(summary PlanetResourceSummary) PlanetResourceSummary {
	summary.Resources = slices.Clone(summary.Resources)
	for i := range summary.Resources {
		row := &summary.Resources[i]
		for _, value := range []**int64{&row.Amount, &row.Count, &row.OccupiedCount, &row.OccupiedAmount, &row.AvailableCount, &row.AvailableAmount} {
			if *value != nil {
				copy := **value
				*value = &copy
			}
		}
		for _, value := range []**float64{&row.Rate, &row.OccupiedRate, &row.AvailableRate} {
			if *value != nil {
				copy := **value
				*value = &copy
			}
		}
	}
	return summary
}

// PlanetResourceDetail 职责：按真实天体 ID 获取空间详情；使用：按需查询；边界：未知星球返回错误。
func (s *Snapshot) PlanetResourceDetail(planetID int64) (PlanetResourceDetail, error) {
	return s.PlanetResourceDetailContext(context.Background(), planetID)
}

// PlanetResourceDetailContext 职责：按需生成可取消的星球详情；使用：布局与用户查询；边界：取消时不返回部分地表。
func (s *Snapshot) PlanetResourceDetailContext(ctx context.Context, planetID int64) (PlanetResourceDetail, error) {
	if err := ctx.Err(); err != nil {
		return PlanetResourceDetail{}, err
	}
	for _, planet := range s.worldView.Planets {
		if planet.PlanetID != planetID {
			continue
		}
		out := PlanetResourceDetail{
			SHA256: s.digest, Tick: s.tick,
			VeinSource: "unknown", BuildingSource: "saved", Veins: []PlanetVein{}, Groups: []PlanetVeinGroup{}, Buildings: []PlanetBuilding{},
			Terrain: PlanetTerrain{ModificationSource: "unknown", WaterSource: "unknown"},
		}
		out.RelayZones = s.GroundBaseZones(planetID)
		if surface := s.surfaces[planetID]; surface != nil {
			out.VeinSource = "saved"
			out.Buildings = append(out.Buildings, surface.buildings...)
			out.Terrain = surface.terrain
			out.Terrain.ModificationSource = "saved"
			out.Terrain.ModData = slices.Clone(out.Terrain.ModData)
			out.Terrain.ReformData = slices.Clone(out.Terrain.ReformData)
			out.Terrain.ReformOffsets = slices.Clone(out.Terrain.ReformOffsets)
			occupied := s.veinOccupancy(planetID)
			for _, vein := range surface.veins {
				vein.MinerEntityIDs = append([]int64{}, occupied[vein.ID]...)
				if vein.VeinType == 7 {
					rate := oilRate(vein.Amount)
					vein.Rate = &rate
				}
				out.Veins = append(out.Veins, vein)
			}
		}
		if scan := s.scanned[planetID]; scan != nil {
			land := scan.landPercent
			out.Terrain.LandPercent = &land
			out.Groups = append(out.Groups, scan.groups...)
		}
		if theme := s.themes[planetID]; theme.ID != 0 {
			out.Terrain.WaterSource = "generated"
			out.Terrain.WaterItemID = &theme.WaterItemID
			out.Terrain.WaterHeight = &theme.WaterHeight
			out.Terrain.HeightSource = "unknown"
			grid, ok, err := galaxy.TerrainHeightsContext(ctx, s.terrainPlanets[planetID], theme)
			if err != nil {
				return PlanetResourceDetail{}, err
			}
			if ok {
				out.Terrain.HeightSource = "generated"
				out.Terrain.HeightPrecision = grid.Precision
				out.Terrain.HeightCentimeters = grid.Heights
				if out.VeinSource == "saved" && hasPendingBurial(out.RelayZones) {
					for i := range out.Veins {
						if err := ctx.Err(); err != nil {
							return PlanetResourceDetail{}, err
						}
						vein := &out.Veins[i]
						vein.PredictedBurial = !vein.IsDeepBuried() && relayBurial(vein.Position, grid, out.Terrain.ModData, out.RelayZones)
					}
				}
				if surface := s.surfaces[planetID]; surface == nil && !s.terrainPlanets[planetID].Gas {
					generated, err := s.planetGeneratedVeins(ctx, planetID, grid)
					if err != nil {
						return PlanetResourceDetail{}, err
					}
					if generated.ok {
						out.VeinSource = "generated"
						for _, vein := range generated.veins {
							vein.MinerEntityIDs = []int64{}
							if vein.VeinType == 7 {
								rate := oilRate(vein.Amount)
								vein.Rate = &rate
							}
							out.Veins = append(out.Veins, vein)
						}
					}
				}
			}
		} else {
			out.Terrain.WaterSource = "unknown"
			out.Terrain.HeightSource = "unknown"
		}
		summary, err := s.planetResourceSummary(ctx, planet, true, galaxy.HeightGrid{
			Precision: out.Terrain.HeightPrecision, Heights: out.Terrain.HeightCentimeters,
		})
		if err != nil {
			return PlanetResourceDetail{}, err
		}
		out.Planet = summary
		return out, nil
	}
	return PlanetResourceDetail{}, fmt.Errorf("存档中不存在星球 %d", planetID)
}

// planetResourceSummary 职责：汇总当前矿点或扫描记录；使用：概要和详情共用；边界：油井储量编码与固体数量分开。
func (s *Snapshot) planetResourceSummary(ctx context.Context, planet PlanetLink, resolve bool, grid galaxy.HeightGrid) (PlanetResourceSummary, error) {
	if err := ctx.Err(); err != nil {
		return PlanetResourceSummary{}, err
	}
	out := PlanetResourceSummary{PlanetID: planet.PlanetID, StarID: planet.StarID, Source: "unknown", Resources: []PlanetResource{}}
	if gas, ok := s.gas[planet.PlanetID]; ok {
		out.Gas = true
		if len(gas) > 0 {
			out.Source = "generated"
			var occupied int64
			for _, station := range s.stations {
				if station.planetId == planet.PlanetID && station.isCollector != 0 {
					occupied++
				}
			}
			if surface := s.surfaces[planet.PlanetID]; surface != nil {
				for _, building := range surface.buildings {
					if building.Prebuild && building.ItemID == 2105 {
						occupied++
					}
				}
			}
			for _, entry := range gas {
				rate := *entry.Rate
				entry.Rate = &rate
				count := occupied
				entry.OccupiedCount = &count
				entry.Infinite = true
				out.Resources = append(out.Resources, entry)
			}
			return out, nil
		}
	}
	type total struct {
		kind, amount, count, occupied, occupiedAmount int64
		available, availableAmount                    int64
	}
	totals := make(map[int64]total)
	surface := s.surfaces[planet.PlanetID]
	zones := s.VirtualRelayZones(planet.PlanetID)
	availabilityKnown := surface != nil
	if surface != nil {
		out.Source = "saved"
		baseZones := s.GroundBaseZones(planet.PlanetID)
		pending := hasPendingBurial(baseZones)
		if pending {
			availabilityKnown = grid.Precision != 0
			if resolve && !availabilityKnown {
				var err error
				grid, availabilityKnown, err = galaxy.TerrainHeightsContext(ctx, s.terrainPlanets[planet.PlanetID], s.themes[planet.PlanetID])
				if err != nil {
					return PlanetResourceSummary{}, err
				}
			}
			if !availabilityKnown {
				out.Source = "candidate"
			}
		}
		occupied := s.veinOccupancy(planet.PlanetID)
		for _, vein := range surface.veins {
			if err := ctx.Err(); err != nil {
				return PlanetResourceSummary{}, err
			}
			sum := totals[vein.ItemID]
			sum.kind, sum.amount, sum.count = vein.VeinType, sum.amount+vein.Amount, sum.count+1
			if len(occupied[vein.ID]) != 0 {
				sum.occupied++
				sum.occupiedAmount += vein.Amount
			} else if availabilityKnown && !vein.IsSurfaceMiningBlocked() &&
				(!pending || !relayBurial(vein.Position, grid, surface.terrain.ModData, baseZones)) {
				sum.available++
				sum.availableAmount += vein.Amount
			}
			totals[vein.ItemID] = sum
		}
	} else if scan := s.scanned[planet.PlanetID]; scan != nil {
		out.Source = "saved"
		for _, group := range scan.groups {
			if itemID := veinItemID(group.VeinType); itemID != 0 {
				sum := totals[itemID]
				sum.kind, sum.amount, sum.count = group.VeinType, sum.amount+group.Amount, sum.count+group.Count
				totals[itemID] = sum
			}
		}
		if len(zones) != 0 {
			out.Source = "candidate"
			var generated generatedPlanetVeins
			if resolve {
				var err error
				generated, err = s.planetGeneratedVeins(ctx, planet.PlanetID, grid)
				if err != nil {
					return PlanetResourceSummary{}, err
				}
			} else if cached, ok := s.generatedVeins.Load(planet.PlanetID); ok {
				generated = cached.(generatedPlanetVeins)
			}
			if generated.ok {
				counts, amounts := make(map[int64]int64), make(map[int64]int64)
				for _, vein := range generated.veins {
					counts[vein.ItemID]++
					amounts[vein.ItemID] += vein.Amount
				}
				availabilityKnown = len(counts) == len(totals)
				for item, sum := range totals {
					availabilityKnown = availabilityKnown && counts[item] == sum.count && amounts[item] == sum.amount
				}
				if availabilityKnown {
					out.Source = "saved"
					for _, vein := range generated.veins {
						if !vein.IsSurfaceMiningBlocked() {
							sum := totals[vein.ItemID]
							sum.available++
							sum.availableAmount += vein.Amount
							totals[vein.ItemID] = sum
						}
					}
				}
			}
		}
	} else if !out.Gas {
		var generated generatedPlanetVeins
		var err error
		if resolve {
			generated, err = s.planetGeneratedVeins(ctx, planet.PlanetID, grid)
			if err != nil {
				return PlanetResourceSummary{}, err
			}
		} else if cached, ok := s.generatedVeins.Load(planet.PlanetID); ok {
			generated = cached.(generatedPlanetVeins)
		} else {
			p := s.terrainPlanets[planet.PlanetID]
			if p.ID != 0 {
				for _, kind := range galaxy.CandidateVeinTypes(p, s.terrainStars[p.StarID-1], s.themes[planet.PlanetID]) {
					entry := PlanetResource{ItemID: veinItemID(int64(kind)), Kind: "solid", Unit: "items", Source: "candidate"}
					if kind == 7 {
						entry.Kind, entry.Unit = "oil", "oil_amount_raw"
					}
					out.Resources = append(out.Resources, entry)
				}
				if len(out.Resources) > 0 {
					out.Source = "candidate"
				}
			}
		}
		if generated.ok {
			out.Source = "generated"
			availabilityKnown = true
			for _, vein := range generated.veins {
				sum := totals[vein.ItemID]
				sum.kind, sum.amount, sum.count = vein.VeinType, sum.amount+vein.Amount, sum.count+1
				if !vein.IsSurfaceMiningBlocked() {
					sum.available++
					sum.availableAmount += vein.Amount
				}
				totals[vein.ItemID] = sum
			}
		}
	}
	for item, sum := range totals {
		amount, count, occupied, occupiedAmount := sum.amount, sum.count, sum.occupied, sum.occupiedAmount
		entry := PlanetResource{ItemID: item, Kind: "solid", Unit: "items", Source: out.Source, Amount: &amount, Count: &count, OccupiedCount: &occupied, OccupiedAmount: &occupiedAmount, Infinite: s.resourceMultiplier >= 99.5}
		if out.Source == "candidate" {
			// 存档扫描的地质数值有效，待定位后的可开采量仍未知。
			entry.Source = "saved"
		}
		if availabilityKnown {
			available, availableAmount := sum.available, sum.availableAmount
			entry.AvailableCount, entry.AvailableAmount = &available, &availableAmount
		}
		if sum.kind == 7 {
			entry.Kind, entry.Unit = "oil", "oil_amount_raw"
			rate, occupiedRate := oilRate(sum.amount), oilRate(sum.occupiedAmount)
			entry.Rate, entry.OccupiedRate = &rate, &occupiedRate
			if availabilityKnown {
				availableRate := oilRate(sum.availableAmount)
				entry.AvailableRate = &availableRate
			}
			entry.Infinite = false
		}
		out.Resources = append(out.Resources, entry)
	}
	if theme := s.themes[planet.PlanetID]; theme.WaterItemID > 0 {
		out.Resources = append(out.Resources, PlanetResource{ItemID: theme.WaterItemID, Kind: "liquid", Unit: "items_per_second", Source: "generated", Infinite: true})
	}
	sort.Slice(out.Resources, func(i, j int) bool { return out.Resources[i].ItemID < out.Resources[j].ItemID })
	return out, nil
}

// generatedPlanetVeins 职责：缓存初始矿点生成的结果；使用：概要与详情；边界：缺失态也独立保留。
type generatedPlanetVeins struct {
	veins []PlanetVein
	ok    bool
}

// planetGeneratedVeins 职责：按原版规则获取初始矿点；使用：未建厂星球；边界：缓存仅保留稀疏矿点，避免复制全星系地形。
func (s *Snapshot) planetGeneratedVeins(ctx context.Context, planetID int64, grid galaxy.HeightGrid) (generatedPlanetVeins, error) {
	if err := ctx.Err(); err != nil {
		return generatedPlanetVeins{}, err
	}
	if cached, ok := s.generatedVeins.Load(planetID); ok {
		return cached.(generatedPlanetVeins), nil
	}
	var out generatedPlanetVeins
	planet, exists := s.terrainPlanets[planetID]
	theme := s.themes[planetID]
	if !exists || theme.ID == 0 || planet.Gas {
		return out, nil
	}
	if grid.Precision == 0 {
		var err error
		grid, _, err = galaxy.TerrainHeightsContext(ctx, planet, theme)
		if err != nil {
			return out, err
		}
	}
	nodes, ok, err := galaxy.VeinsContext(ctx, planet, s.terrainStars[planet.StarID-1], theme, grid, s.resourceMultiplier)
	if err != nil {
		return out, err
	}
	out.ok = ok
	zones := s.VirtualRelayZones(planetID)
	for i, node := range nodes {
		if err := ctx.Err(); err != nil {
			return generatedPlanetVeins{}, err
		}
		vein := PlanetVein{ID: int64(i + 1), GroupID: int64(node.Group), ItemID: veinItemID(int64(node.Kind)), VeinType: int64(node.Kind), Amount: node.Amount, Position: node.Position}
		vein.PredictedBurial = relayBurial(node.Position, grid, nil, zones)
		out.veins = append(out.veins, vein)
	}
	if err := ctx.Err(); err != nil {
		return generatedPlanetVeins{}, err
	}
	actual, _ := s.generatedVeins.LoadOrStore(planetID, out)
	return actual.(generatedPlanetVeins), nil
}

func hasPendingBurial(zones []RelayZone) bool {
	return slices.ContainsFunc(zones, func(zone RelayZone) bool { return zone.Depth > 0 })
}

// relayBurial 判断初始矿点是否位于已知基地的近邻下沉区。
func relayBurial(position galaxy.Vector, grid galaxy.HeightGrid, modData []byte, zones []RelayZone) bool {
	if len(zones) == 0 {
		return false
	}
	radius := math.Sqrt(position.X*position.X + position.Y*position.Y + position.Z*position.Z)
	if radius == 0 {
		return false
	}
	height, ok := galaxy.TerrainHeightAt(grid, modData, position)
	if !ok {
		return false
	}
	scale := (height - 0.13) / radius
	for _, zone := range zones {
		// BuriedVeins 仅在改造高度减 0.13 米的三维坐标严格小于 R+1 时下沉。
		// 初始碰撞体近邻依赖各模型 prefab。允许碰撞体命中的这圈矿点暂不计入保证供给。
		x, y, z := position.X*scale-zone.Position.X, position.Y*scale-zone.Position.Y, position.Z*scale-zone.Position.Z
		if zone.Depth > 0 && x*x+y*y+z*z < (zone.Radius+1)*(zone.Radius+1) {
			return true
		}
	}
	return false
}

// oilRate 职责：将油井存档量换算为基础产出；使用：选址与矿点详情；边界：未应用采矿速度倍率，单位为每秒。
func oilRate(amount int64) float64 {
	// 与原版 VeinData.oilSpeedMultiplier 保持相同常量。
	return float64(amount) * float64(float32(4e-5))
}

// veinOccupancy 职责：从已验证的矿机组件反查覆盖；使用：概要与详情；边界：只统计存档矿机覆盖。
func (s *Snapshot) veinOccupancy(planetID int64) map[int64][]int64 {
	occupied := make(map[int64][]int64)
	if line := s.lines[planetID]; line != nil {
		for _, device := range line.devices {
			if device.Kind == "miner" {
				for _, veinID := range device.Veins {
					entityIDs := occupied[veinID]
					if len(entityIDs) == 0 || entityIDs[len(entityIDs)-1] != device.EntityID {
						occupied[veinID] = append(entityIDs, device.EntityID)
					}
				}
			}
		}
	}
	return occupied
}

// veinItemID 职责：转换原版矿种；使用：扫描记录关联产品；边界：不推测未知矿种或模组物品。
func veinItemID(kind int64) int64 {
	items := [...]int64{0, 1001, 1002, 1003, 1004, 1005, 1006, 1007, 1011, 1012, 1013, 1117, 1014, 1015, 1016}
	if kind < 1 || kind >= int64(len(items)) {
		return 0
	}
	return items[kind]
}

// buildGasResources 职责：转换原版气体主题产出；使用：规划概要；边界：无法重建时保留气态身份和未知速率。
func buildGasResources(stars []galaxy.Star, planets []galaxy.Planet, themes map[int64]galaxy.SurfaceTheme) map[int64][]PlanetResource {
	out := make(map[int64][]PlanetResource)
	for planetID, yields := range galaxy.GasYields(stars, planets, themes) {
		out[planetID] = nil
		for _, item := range yields {
			rate := item.Rate
			out[planetID] = append(out[planetID], PlanetResource{ItemID: item.ItemID, Kind: "gas", Unit: "items_per_second", Source: "generated", Rate: &rate})
		}
	}
	return out
}
