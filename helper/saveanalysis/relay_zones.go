package saveanalysis

import (
	"math"
	"slices"

	"DysonSphereBox/helper/saveanalysis/galaxy"
)

// RelayZone 职责：记录保存时已确认的地面基地施工影响；使用：施工避让；边界：不含后续可能的增援或未确认位置。
type RelayZone struct {
	PlanetID    int64 `json:"planetId"`    // 受影响的原版星球标识。
	HiveAstroID int64 `json:"hiveAstroId"` // 中继器所属黑雾巢，和 RelayID 配合识别。
	RelayID     int64 `json:"relayId"`     // 巢内的中继器索引。
	// BaseID 和 EnemyID 配对标识已实体化的地面基地；虚拟基地两者均为0。
	BaseID  int64 `json:"baseId,omitempty"`
	EnemyID int64 `json:"enemyId,omitempty"`
	// Realized 表示原始厂区存在地面基地；Localized 表示已执行原版地表本地化。
	Realized  bool `json:"realized"`
	Localized bool `json:"localized"`
	// Position 为 planet-local 游戏米；实体基地使用原坐标，虚拟基地投影到地表半径加 0.2 米。
	Position galaxy.Vector `json:"position"`
	Yaw      float64       `json:"yaw"`    // 虚拟基地模板的原版偏航角，单位为度；实体圆形范围不使用该值。
	Radius   float64       `json:"radius"` // 原版主基地占地半径，若待本地化则使用 BuriedVeins 近邻查询半径，单位为游戏米。
	Depth    float64       `json:"depth"`  // 待本地化的原版 BuriedVeins 下沉深度参数，单位为游戏米。
}

// GroundBaseZones 职责：提供拖入蓝图前可核对的已保存地面基地范围；使用：按星球施工；边界：不推演未来行动，返回副本。
func (s *Snapshot) GroundBaseZones(planetID int64) []RelayZone {
	if s == nil {
		return nil
	}
	return append(slices.Clone(s.groundBaseZones[planetID]), s.virtualRelayZones[planetID]...)
}

// VirtualRelayZones 职责：提供星球上已落地虚拟基地区域；使用：按星球规划与施工；边界：返回独立副本，实际已有地面基地由厂区布局判断。
func (s *Snapshot) VirtualRelayZones(planetID int64) []RelayZone {
	if s == nil {
		return nil
	}
	return slices.Clone(s.virtualRelayZones[planetID])
}

// recordVirtualRelay 职责：将保存的抵达方向投影到地表基地模板锚点；使用：中继器读取；边界：仅用于原版半径 200 米的岩质星球。
func (r *reader) recordVirtualRelay(planetID, hiveID, relayID int64, p *layoutVector3, yaw float32) {
	if planetID <= 0 || planetID%100 == 0 || hiveID <= 0 {
		return
	}
	x, y, z := float64(floatValue(p.x)), float64(floatValue(p.y)), float64(floatValue(p.z))
	length := math.Sqrt(x*x + y*y + z*z)
	if length <= 0 || math.IsNaN(length) || math.IsInf(length, 0) || math.IsNaN(float64(yaw)) || math.IsInf(float64(yaw), 0) {
		return
	}
	// 游戏以单精度计算 normalized(targetLPos) * (planet.realRadius + 0.2f)。
	projection := func(v float64) float64 { return float64(float32(v/length) * float32(200.2)) }
	if r.virtualRelayZones == nil {
		r.virtualRelayZones = map[int64][]RelayZone{}
	}
	r.virtualRelayZones[planetID] = append(r.virtualRelayZones[planetID], RelayZone{
		PlanetID: planetID, HiveAstroID: hiveID, RelayID: relayID,
		Position: galaxy.Vector{X: projection(x), Y: projection(y), Z: projection(z)},
		Yaw:      float64(yaw), Radius: 40, Depth: 60,
	})
}

// recordGroundBase 职责：保留核对了双向组件编号的地面主基地；使用：厂区解析；边界：已本地化的基地只保留原版主基地占地。
func (r *reader) recordGroundBase(enemy *layoutEnemyData, baseID, hiveID, relayID int64) {
	p := enemy.position
	x, y, z := float64(floatValue(p.x)), float64(floatValue(p.y)), float64(floatValue(p.z))
	length := math.Sqrt(x*x + y*y + z*z)
	if length <= 0 || math.IsNaN(length) || math.IsInf(length, 0) {
		return
	}
	radius, depth := 11.25, 0.0
	if !enemy.localized {
		radius, depth = 40, 60
	}
	if r.groundBaseZones == nil {
		r.groundBaseZones = map[int64][]RelayZone{}
	}
	r.groundBaseZones[enemy.planetID] = append(r.groundBaseZones[enemy.planetID], RelayZone{
		PlanetID: enemy.planetID, HiveAstroID: hiveID, RelayID: relayID, BaseID: baseID, EnemyID: enemy.id,
		Realized: true, Localized: enemy.localized, Position: galaxy.Vector{X: x, Y: y, Z: z}, Radius: radius, Depth: depth,
	})
}
