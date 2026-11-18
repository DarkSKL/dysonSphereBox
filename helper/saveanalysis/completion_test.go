package saveanalysis

// 文件职责：覆盖本轮发现的加载语义、世界副本与球体边界。
// 使用：普通 Go 测试；边界：受控数据不代表真实非空球体验收。
import (
	"DysonSphereBox/helper/saveanalysis/galaxy"
	"bytes"
	"encoding/binary"
	"math"
	"strconv"
	"testing"
)

// TestProductImportSignedTotals 职责：验证加载时历史取绝对值；使用：与游戏 Import 语义一致；边界：溢出拒绝。
func TestProductImportSignedTotals(t *testing.T) {
	for _, n := range []int64{-42, 0, 42, math.MinInt64} {
		t.Run(strconv.FormatInt(n, 10), func(t *testing.T) {
			var b bytes.Buffer
			for _, v := range []int32{1, 0, 0, 1} {
				binary.Write(&b, binary.LittleEndian, v)
			}
			binary.Write(&b, binary.LittleEndian, n)
			binary.Write(&b, binary.LittleEndian, int32(1001))
			r := &reader{data: b.Bytes()}
			if n == math.MinInt64 {
				defer func() {
					if _, ok := recover().(decodeError); !ok {
						t.Fatal("溢出未拒绝")
					}
				}()
			}
			v := r.readProductStat()
			want := n
			if want < 0 {
				want = -want
			}
			if v.total[0] != want {
				t.Fatal("历史件数错误")
			}
		})
	}
}

// TestWorldCopiesAndEmptyInventory 职责：验证完整世界切片隔离及未建厂空库存；使用：查询回归；边界：没有库存不代表统计为零。
func TestWorldCopiesAndEmptyInventory(t *testing.T) {
	s := &Snapshot{worldView: WorldView{Stars: []StarLink{{StarID: 1}}, Planets: []PlanetLink{{PlanetID: 101, StarID: 1, Position: galaxy.Vector{X: 1}}}, Missing: []string{}}, items: map[int64][]planetRecord{}}
	v := s.World()
	v.Planets[0].Position.X = 99
	v.Stars[0].Name = "changed"
	if s.World().Planets[0].Position.X != 1 || s.World().Stars[0].Name != "" {
		t.Fatal("查询污染世界")
	}
	inv, e := s.Inventory(101, 0)
	if e != nil || len(inv.Entries) != 0 {
		t.Fatal("未建厂星球不能查询空库存")
	}
	if _, e = s.Inventory(102, 0); e == nil {
		t.Fatal("未知星球被接受")
	}
}

// TestSphereProgressStates 职责：验证无设计、未建、部分与完成及非法数值；使用：受控转换；边界：不混合建设单位。
func TestSphereProgressStates(t *testing.T) {
	r := &reader{}
	for _, p := range [][2]int64{{0, 0}, {0, 10}, {3, 10}, {10, 10}} {
		got := r.progress(p[0], p[1])
		if got.Built != p[0] || got.Required != p[1] || got.Unit != "structure_point" {
			t.Fatal("建设状态错误")
		}
	}
	for _, p := range [][2]int64{{-1, 10}, {11, 10}, {0, -1}} {
		func() {
			defer func() {
				if _, ok := recover().(decodeError); !ok {
					t.Fatal("无效进度未拒绝")
				}
			}()
			r.progress(p[0], p[1])
		}()
	}
	for _, v := range []float64{math.NaN(), math.Inf(1)} {
		func() {
			defer func() {
				if _, ok := recover().(decodeError); !ok {
					t.Fatal("无效几何未拒绝")
				}
			}()
			r.finite(v)
		}()
	}
}

// TestSphereMultipleLayersAndCompletedStructures 职责：验证稀疏层、多类完成态及快照单位；使用：受控球体回归；边界：不替代实档几何验收。
func TestSphereMultipleLayersAndCompletedStructures(t *testing.T) {
	rotation := &layoutQuaternion{w: int64(math.Float32bits(1))}
	empty := &layoutDysonSphereLayer{id: 1, orbitRadius: int64(math.Float32bits(1000)), orbitRotation: rotation, currentRotation: rotation}
	layer := &layoutDysonSphereLayer{id: 3, orbitRadius: int64(math.Float32bits(2000)), orbitRotation: rotation, currentRotation: rotation}
	for i := int64(1); i <= 2; i++ {
		built := int64(0)
		if i == 1 {
			built = 5
		}
		layer.nodePool = append(layer.nodePool, &layoutExportStruct_int32__DysonNode_{index: i, data: &layoutDysonNode{id: i, layerId: 3, sp: built, spMax: 5, pos: &layoutVector3{}, color: &layoutRGBA{}}})
	}
	layer.framePool = []*layoutExportStruct_int32__DysonFrame_{{index: 1, data: &layoutDysonFrame{id: 1, layerId: 3, nodeAId: 1, nodeBId: 2, spA: 2, spB: 3, spMax: 5, color: &layoutRGBA{}}}}
	layer.shellPool = []*layoutExportStruct_int32__DysonShell_{{index: 1, data: &layoutDysonShell{id: 1, layerId: 3, nodesId: []int64{1, 2}, nodecps: []int64{2, 6, 8}, gridScale: 2, vertexCount: 1, verts: []*layoutVector3{{}}, color: &layoutRGBA{}}}}
	r := &reader{world: &layoutGameData{dysonSpheres: []*layoutExportStruct_int32__DysonSphere_{{index: 1, data: &layoutDysonSphere{layerCount: 2, layersIdBased: []*layoutExportStruct_int32__DysonSphereLayer_{{index: 1, data: empty}, {index: 0}, {index: 3, data: layer}}}}}}}
	s := &Snapshot{starCount: 1, tick: 60}
	r.buildSpheres(s)
	v, e := s.Sphere(1, 0)
	if e != nil || len(v.Layers) != 2 || v.Layers[1].ID != 3 || len(v.Layers[0].Structures) != 0 || v.Tick != 60 || v.PositionUnit != "game_m" {
		t.Fatalf("多层查询错误: %+v %v", v, e)
	}
	structures := v.Layers[1].Structures
	if len(structures) != 4 || structures[2].Progress.Built != 5 || structures[2].Progress.Required != 5 || structures[3].Progress.Built != 8 || structures[3].Progress.Required != 8 || structures[3].Progress.Unit != "cell_point" {
		t.Fatal("完成态或单位错误")
	}
	if _, e = s.Sphere(1, 2); e == nil {
		t.Fatal("被删除层未报错")
	}
}

// TestWorldCustomNamePrecedence 职责：覆盖恒星覆盖、星球覆盖和清空覆盖；使用：合成世界回归；边界：这里只验证来源和优先级，独立坐标验证使用游戏基准。
func TestWorldCustomNamePrecedence(t *testing.T) {
	_, planets, err := galaxy.Generate(1, 20200403, 32, 2052, 0)
	if err != nil {
		t.Fatal(err)
	}
	r := &reader{world: &layoutGameData{gameDesc: &layoutGameDesc{galaxySeed: 1, galaxyAlgo: 20200403, starCount: 32, starNameLCID: 2052}}, astroNames: map[int64]string{100: "新恒星✨", 102: "自定义卫星", 103: ""}}
	for _, p := range planets {
		r.worldPlanetIDs = append(r.worldPlanetIDs, int64(p.ID))
	}
	s := &Snapshot{seed: 1, algorithm: 20200403, starCount: 32}
	r.buildWorld(s)
	v := s.World()
	if v.Stars[0].Name != "新恒星✨" || v.Stars[0].GeneratedName == v.Stars[0].Name || v.Planets[0].Name != "新恒星✨ - 1号星" || v.Planets[1].Name != "自定义卫星" || v.Planets[1].GeneratedName != "新恒星✨ - 2号星" || v.Planets[2].Name != "新恒星✨ - 3号星" {
		t.Fatalf("名称来源或覆盖优先级错误: %+v", v.Planets[:3])
	}
	if v.PositionUnit != "game_m" || v.PositionOrigin != "galaxy_origin" || v.GameMetersPerAU != 40000 || v.GameMetersPerLightYear != 2400000 || v.TicksPerSecond != 60 {
		t.Fatal("位置契约与计算单位不一致")
	}
	r.astroNames[100] = ""
	r.buildWorld(s)
	if s.World().Stars[0].Name != v.Stars[0].GeneratedName || s.World().Planets[1].Name != "自定义卫星" {
		t.Fatal("清空恒星覆盖错误地影响星球覆盖")
	}
}
