package saveanalysis

// 文件职责：验证分类库存、球体引用和实档批量读取。
// 使用：go test；边界：实档只从显式环境变量指定目录读取，不随代码提交。

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestNativeSphereRecords 职责：读取游戏自身导出的受控节点与框架；使用：显式指定本地基准目录；边界：不提交程序集或二进制资料，不代替完整球体实档。
func TestNativeSphereRecords(t *testing.T) {
	dir := os.Getenv("DSP_SAVE_REFERENCE_DIR")
	if dir == "" {
		t.Skip("未指定独立基准目录")
	}
	data, err := os.ReadFile(filepath.Join(dir, "node-native.bin"))
	if err != nil {
		t.Fatal(err)
	}
	r := &reader{data: data}
	node := r.readDysonNode()
	if r.offset != int64(len(data)) || node.id != 3 || node.layerId != 2 || node.sp != 7 || node.spMax != 30 || floatValue(node.pos.x) != 10.5 || floatValue(node.pos.y) != -2.25 {
		t.Fatalf("原生节点导出对照失败: offset=%d length=%d node=%+v pos=%+v", r.offset, len(data), node, node.pos)
	}
	data, err = os.ReadFile(filepath.Join(dir, "frame-native.bin"))
	if err != nil {
		t.Fatal(err)
	}
	r = &reader{data: data}
	frame := r.readDysonFrame()
	if r.offset != int64(len(data)) || frame.id != 4 || frame.nodeAId != 3 || frame.nodeBId != 7 || frame.spA != 3 || frame.spB != 5 || frame.spMax != 20 {
		t.Fatal("原生框架导出对照失败")
	}
}

// TestSphereReferencesAndProgress 职责：验证结构关联、部分建设和壳面总计；使用：转换回归；边界：受控结构不代表已通过真实壳面验收。
func TestSphereReferencesAndProgress(t *testing.T) {
	rotation := &layoutQuaternion{w: int64(math.Float32bits(1))}
	layer := &layoutDysonSphereLayer{id: 1, orbitRadius: int64(math.Float32bits(10000)), orbitRotation: rotation, currentRotation: rotation,
		nodePool:  []*layoutExportStruct_int32__DysonNode_{{index: 1, data: &layoutDysonNode{id: 1, layerId: 1, pos: &layoutVector3{}, sp: 1, spMax: 2, color: &layoutRGBA{}}}},
		framePool: []*layoutExportStruct_int32__DysonFrame_{}, shellPool: []*layoutExportStruct_int32__DysonShell_{{index: 1, data: &layoutDysonShell{id: 1, layerId: 1, nodesId: []int64{1}, nodecps: []int64{3, 3}, gridScale: 2, vertexCount: 1, verts: []*layoutVector3{{}}, color: &layoutRGBA{}}}}}
	r := &reader{world: &layoutGameData{dysonSpheres: []*layoutExportStruct_int32__DysonSphere_{{index: 1, data: &layoutDysonSphere{layerCount: 1, layersIdBased: []*layoutExportStruct_int32__DysonSphereLayer_{{index: 1, data: layer}}}}}}}
	s := &Snapshot{starCount: 1}
	r.buildSpheres(s)
	view, _ := s.Sphere(1, 1)
	progress := view.Layers[0].Structures[1].Progress
	if progress.Built != 3 || progress.Required != 8 {
		t.Fatalf("壳面总计重复累加: %+v", progress)
	}
	layer.shellPool[0].data.nodesId[0] = 2
	defer func() {
		if _, ok := recover().(decodeError); !ok {
			t.Fatal("未拒绝无效节点引用")
		}
	}()
	r.buildSpheres(&Snapshot{starCount: 1})
}

// TestTruncatedSample 职责：在更新档头长度后仍拒绝区段截断；使用：本机样本回归；边界：仅修改内存副本。
func TestTruncatedSample(t *testing.T) {
	path := os.Getenv("DSP_SAVE_SAMPLE")
	if path == "" {
		t.Skip("未指定实档")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, length := range []int{len(data) / 2, len(data) - 1} {
		part := append([]byte(nil), data[:length]...)
		binary.LittleEndian.PutUint64(part[6:14], uint64(length))
		if _, err := Decode(part); err == nil {
			t.Fatalf("截断 %d 未被拒绝", length)
		}
	}
}

// TestInventoryCategories 职责：验证普通容器与塔不重计、空槽和返回副本；使用：库存回归；边界：使用合成厂区。
func TestInventoryCategories(t *testing.T) {
	r := &reader{planets: []int64{104}, storages: []*layoutFactoryStorage{{storagePool: []*layoutStorageExportStruct{{storage: &layoutStorageComponent{id: 1, entityId: 8, size: 2, grids: []*layoutStorageGrid{{itemId: 1001, count: 9}, {}}}}}, tankPool: []*layoutTankComponent{{id: 1, entityId: 9, fluidId: 1000, fluidCount: 12, fluidCapacity: 10000}}}}, stations: []*layoutStationComponent{{id: 1, planetId: 104, entityId: 10, storage: []*layoutStationStore{{itemId: 1001, count: 7, maxWithKeepMode: 100010000}, {}}}}}
	s := &Snapshot{planetIDs: []int64{104}, items: map[int64][]planetRecord{}}
	r.buildInventory(s)
	all, err := s.Inventory(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if all.Totals["storage"] != 9 || all.Totals["station"] != 7 || all.Totals["tank"] != 12 || len(all.Entries) != 5 {
		t.Fatalf("分类库存错误: %+v", all)
	}
	filtered, _ := s.Inventory(104, 1001)
	if len(filtered.Entries) != 2 || filtered.Entries[0].Capacity != nil {
		t.Fatal("过滤或箱子专属容量语义错误")
	}
	*filtered.Entries[1].Capacity = 99
	again, _ := s.Inventory(104, 1001)
	if *again.Entries[1].Capacity != 10000 {
		t.Fatal("查询污染缓存")
	}
	if _, err = s.Inventory(999, 0); err == nil {
		t.Fatal("未知星球被接受")
	}
	if len(s.items[1000]) != 1 {
		t.Fatal("仅在罐中的物品未进入目录")
	}
}

// TestSphereCopyAndSelection 职责：验证无球体、层选择和深复制；使用：查询回归；边界：不替代真实几何比对。
func TestSphereCopyAndSelection(t *testing.T) {
	s := &Snapshot{spheres: []SphereView{{StarID: 1, Exists: true, Layers: []SphereLayer{{ID: 2, Structures: []SphereStructure{{Kind: "shell", NodeIDs: []int64{1, 2, 3}, Vertices: [][]float64{{1, 2, 3}}}}}}}, {StarID: 2, Layers: []SphereLayer{}}}}
	view, err := s.Sphere(1, 2)
	if err != nil {
		t.Fatal(err)
	}
	view.Layers[0].Structures[0].Vertices[0][0] = 99
	view.Layers[0].Structures[0].NodeIDs[0] = 99
	again, _ := s.Sphere(1, 2)
	if again.Layers[0].Structures[0].Vertices[0][0] != 1 || again.Layers[0].Structures[0].NodeIDs[0] != 1 {
		t.Fatal("球体查询污染缓存")
	}
	if _, err = s.Sphere(1, 1); err == nil {
		t.Fatal("不存在的层被接受")
	}
	empty, err := s.Sphere(2, 0)
	if err != nil || empty.Exists || len(empty.Layers) != 0 {
		t.Fatal("无球体状态错误")
	}
}

// TestSaveBatch 职责：批量验证同版本实档和分类合计；使用：DSP_SAVE_DIR 指定目录；边界：不根据文件名判断损坏，测试不证明来源或游戏语义。
func TestSaveBatch(t *testing.T) {
	dir := os.Getenv("DSP_SAVE_DIR")
	if dir == "" {
		t.Skip("未指定批量实档目录")
	}
	paths, err := filepath.Glob(filepath.Join(dir, "*.dsv"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("找不到样本: %v", err)
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			started := time.Now()
			s, err := Read(path)
			if err != nil {
				t.Fatal(err)
			}
			view, err := s.Inventory(0, 0)
			if err != nil {
				t.Fatal(err)
			}
			counts := map[string]int64{}
			for _, entry := range view.Entries {
				counts[entry.Kind] += entry.Count
			}
			for kind, count := range view.Totals {
				if counts[kind] != count {
					t.Fatal("明细与分类合计不一致")
				}
			}
			layers, structures := 0, 0
			for _, sphere := range s.spheres {
				layers += len(sphere.Layers)
				for _, layer := range sphere.Layers {
					structures += len(layer.Structures)
				}
			}
			if s.digest == "38b48472618c29091f7575a10bd0453e30273e287f7265e4e5c87ee739ce763c" {
				detail, _ := s.Item(1106, 60)
				found := false
				for _, planet := range detail.Planets {
					for _, slot := range planet.Slots {
						if planet.PlanetID == 104 && slot.StationID == 1 && slot.Index == 0 {
							found = true
							if slot.Inventory != 700 || slot.Capacity != 10000 || s.tick != 7174748 {
								t.Fatal("已确认游戏基准发生偏差")
							}
						}
					}
				}
				if !found {
					t.Fatal("已确认货槽丢失")
				}
			}
			t.Logf("elapsed=%s planets=%d slots=%d layers=%d structures=%d totals=%v", time.Since(started), len(s.planetIDs), len(view.Entries), layers, structures, view.Totals)
		})
	}
}

// TestNativeShellRecord 职责：核对游戏原始导出的受控非空壳面；使用：显式独立基准目录；边界：只证明记录布局和计数，不代替非空球体实档。
func TestNativeShellRecord(t *testing.T) {
	dir := os.Getenv("DSP_SAVE_REFERENCE_DIR")
	if dir == "" {
		t.Skip("未指定独立基准目录")
	}
	data, e := os.ReadFile(filepath.Join(dir, "shell-native.bin"))
	if e != nil {
		t.Fatal(e)
	}
	r := &reader{data: data}
	shell := r.readDysonShell()
	if r.offset != int64(len(data)) || shell.id != 2 || shell.layerId != 3 || shell.protoId != 4 || math.Float64frombits(uint64(shell.radius)) != 10000 || shell.gridScale != 2 || shell.vertexCount != 3 || len(shell.nodesId) != 3 || len(shell.nodecps) != 4 || shell.nodecps[3] != 12 {
		t.Fatal("原生壳面身份、计数或读取边界不一致")
	}
	for i := 0; i < 3; i++ {
		if shell.nodesId[i] != int64(i+1) {
			t.Fatal("原生壳面节点顺序错误")
		}
		v := r.vector(shell.verts[i])
		for axis, value := range v {
			want := 0.0
			if axis == i {
				want = 10000
			}
			if value != want {
				t.Fatal("原生壳面顶点错误")
			}
		}
	}
	if shell.color.v_r != 10 || shell.color.g != 20 || shell.color.b != 30 || shell.color.a != 40 {
		t.Fatal("原生壳面颜色错误")
	}
	rotation := &layoutQuaternion{w: int64(math.Float32bits(1))}
	layer := &layoutDysonSphereLayer{id: 3, orbitRadius: int64(math.Float32bits(10000)), orbitRotation: rotation, currentRotation: rotation, shellPool: []*layoutExportStruct_int32__DysonShell_{{index: 0}, {index: 2, data: shell}}}
	for id := int64(1); id <= 3; id++ {
		layer.nodePool = append(layer.nodePool, &layoutExportStruct_int32__DysonNode_{index: id, data: &layoutDysonNode{id: id, layerId: 3, pos: &layoutVector3{}, color: &layoutRGBA{}}})
	}
	r.world = &layoutGameData{dysonSpheres: []*layoutExportStruct_int32__DysonSphere_{{index: 1, data: &layoutDysonSphere{layerCount: 1, layersIdBased: []*layoutExportStruct_int32__DysonSphereLayer_{{index: 0}, {index: 0}, {index: 3, data: layer}}}}}}
	s := &Snapshot{starCount: 1}
	r.buildSpheres(s)
	v, _ := s.Sphere(1, 3)
	p := v.Layers[0].Structures[3].Progress
	if p.Built != 12 || p.Required != 24 || p.Unit != "cell_point" {
		t.Fatal("游戏原生计数与转换结果不一致")
	}
}
