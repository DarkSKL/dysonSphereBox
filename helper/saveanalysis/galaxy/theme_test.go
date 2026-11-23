package galaxy

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
)

// TestNativeThemes 职责：与游戏导出的主题逐星球比对；使用：设置本地参考文件；边界：参考数据不随代码分发。
func TestNativeThemes(t *testing.T) {
	path := os.Getenv("DSP_GALAXY_VEIN_REFERENCE")
	if path == "" {
		t.Skip("未指定游戏主题基准")
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	enabled := make([]int64, 25)
	for i := range enabled {
		enabled[i] = int64(i + 1)
	}
	var themes map[int64]SurfaceTheme
	var seed int32
	var total, mismatches int
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "galaxy "):
			var count int
			if _, err := fmt.Sscanf(line, "galaxy seed=%d stars=%d", &seed, &count); err != nil {
				t.Fatal(err)
			}
			stars, planets, err := Generate(seed, 20200403, count, 1033, 0)
			if err != nil {
				t.Fatal(err)
			}
			themes = Themes(stars, planets, enabled)
		case strings.HasPrefix(line, "planet "):
			var planetID int64
			var algorithm, themeID int
			if _, err := fmt.Sscanf(line, "planet id=%d algo=%d theme=%d", &planetID, &algorithm, &themeID); err != nil {
				t.Fatal(err)
			}
			total++
			actual := themes[planetID]
			if actual.Algorithm != algorithm || actual.ID != themeID {
				mismatches++
				if mismatches <= 10 {
					t.Logf("seed=%d planet=%d theme=%d/%d algo=%d/%d", seed, planetID, actual.ID, themeID, actual.Algorithm, algorithm)
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if total == 0 || mismatches > 0 {
		t.Fatalf("主题基准不一致：%d/%d", mismatches, total)
	}
	t.Logf("核对 %d 颗星球的主题及算法", total)
}

// TestNativeVeinPositions 职责：逐矿点复核类型、矿量、分组与局部落位；使用：设置游戏节点参考；边界：坐标允许单精度误差。
func TestNativeVeinPositions(t *testing.T) {
	path := os.Getenv("DSP_GALAXY_VEIN_NODES")
	if path == "" {
		t.Skip("未指定游戏矿点基准")
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	enabled := make([]int64, 25)
	for i := range enabled {
		enabled[i] = int64(i + 1)
	}
	var currentSeed int32 = -1
	var planets map[int]Planet
	var themes map[int64]SurfaceTheme
	var stars []Star
	var nodes []GeneratedVein
	var currentPlanet, total, mismatches int
	var mostError float64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "node ") {
			continue
		}
		var seed int32
		var id, index, kind, group, model int
		var amount int64
		var x, y, z uint32
		if _, err := fmt.Sscanf(line, "node seed=%d planet=%d i=%d type=%d group=%d model=%d amount=%d posX=%x posY=%x posZ=%x", &seed, &id, &index, &kind, &group, &model, &amount, &x, &y, &z); err != nil {
			t.Fatal(err)
		}
		if currentSeed != seed {
			currentSeed, currentPlanet = seed, 0
			var list []Planet
			stars, list, err = Generate(seed, 20200403, 64, 1033, 0)
			if err != nil {
				t.Fatal(err)
			}
			themes = Themes(stars, list, enabled)
			planets = make(map[int]Planet)
			for _, planet := range list {
				planets[planet.ID] = planet
			}
		}
		if currentPlanet != id {
			currentPlanet = id
			planet, ok := planets[id]
			if !ok {
				t.Fatalf("找不到星球 %d", id)
			}
			theme := themes[int64(id)]
			grid, ok := TerrainHeights(planet, theme)
			if !ok {
				t.Fatalf("算法 %d 未生成地形", theme.Algorithm)
			}
			nodes, ok = Veins(planet, stars[planet.StarID-1], theme, grid, 1)
			if !ok {
				t.Fatalf("算法 %d 未生成矿点", theme.Algorithm)
			}
		}
		total++
		if index > len(nodes) {
			mismatches++
			continue
		}
		node := nodes[index-1]
		dx := node.Position.X - float64(math.Float32frombits(x))
		dy := node.Position.Y - float64(math.Float32frombits(y))
		dz := node.Position.Z - float64(math.Float32frombits(z))
		distance := math.Sqrt(dx*dx + dy*dy + dz*dz)
		mostError = math.Max(mostError, distance)
		if node.Kind != kind || node.Group != group || node.Amount != amount || distance > .005 {
			mismatches++
			if mismatches <= 8 {
				t.Logf("seed=%d planet=%d node=%d kind=%d/%d group=%d/%d amount=%d/%d distance=%.6f", seed, id, index, node.Kind, kind, node.Group, group, node.Amount, amount, distance)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if total == 0 || mismatches > 0 {
		t.Fatalf("矿点基准不一致 %d/%d，最大坐标偏差 %.6f 米", mismatches, total, mostError)
	}
	t.Logf("核对 %d 个矿点，最大坐标偏差 %.6f 米", total, mostError)
}

// TestNativeVeins 职责：用游戏导出的矿点数和矿量复核离线生成；使用：设置本地参考文件；边界：不采用 Go 生成值作预期。
func TestNativeVeins(t *testing.T) {
	path := os.Getenv("DSP_GALAXY_VEIN_REFERENCE")
	if path == "" {
		t.Skip("未指定游戏初始矿脉基准")
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	enabled := make([]int64, 25)
	for i := range enabled {
		enabled[i] = int64(i + 1)
	}
	var seed int32
	planets := map[int64]Planet{}
	var stars []Star
	var themes map[int64]SurfaceTheme
	checked := map[int]bool{}
	verifiedPlanets := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "galaxy "):
			var count int
			if _, err := fmt.Sscanf(line, "galaxy seed=%d stars=%d", &seed, &count); err != nil {
				t.Fatal(err)
			}
			var list []Planet
			stars, list, err = Generate(seed, 20200403, count, 1033, 0)
			if err != nil {
				t.Fatal(err)
			}
			themes = Themes(stars, list, enabled)
			for _, planet := range list {
				planets[int64(planet.ID)] = planet
			}
		case strings.HasPrefix(line, "planet "):
			var planetID int64
			var algorithm, themeID int
			if _, err := fmt.Sscanf(line, "planet id=%d algo=%d theme=%d", &planetID, &algorithm, &themeID); err != nil {
				t.Fatal(err)
			}
			if algorithm == 0 || themeID == 1 || themes[planetID].ID != themeID {
				continue
			}
			p := planets[planetID]
			grid, ok := TerrainHeights(p, themes[planetID])
			if !ok {
				continue
			}
			nodes, ok := Veins(p, stars[p.StarID-1], themes[planetID], grid, 1)
			if !ok {
				continue
			}
			checked[algorithm] = true
			verifiedPlanets++
			var counts, amounts, wantCounts, wantAmounts [15]int64
			for _, node := range nodes {
				counts[node.Kind]++
				amounts[node.Kind] += node.Amount
			}
			for _, word := range strings.Fields(line) {
				var dst *[15]int64
				switch {
				case strings.HasPrefix(word, "nodes="):
					dst = &wantCounts
				case strings.HasPrefix(word, "amounts="):
					dst = &wantAmounts
				default:
					continue
				}
				_, values, _ := strings.Cut(word, "=")
				for i, raw := range strings.Split(strings.TrimSuffix(values, ","), ",") {
					(*dst)[i], err = strconv.ParseInt(raw, 10, 64)
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			if counts != wantCounts || amounts != wantAmounts {
				t.Errorf("seed=%d planet=%d algo=%d\n节点 %v/%v\n矿量 %v/%v", seed, planetID, algorithm, counts, wantCounts, amounts, wantAmounts)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(checked) == 0 {
		t.Fatal("无有效矿脉基准")
	}
	t.Logf("核对 %d 颗星球的矿脉，涉及 %d 个地形算法", verifiedPlanets, len(checked))
}

// TestNativeTerrain 职责：核对各地形算法的游戏基准；使用：设置本地矿脉导出路径；边界：算法 3 用原生网格逐点比较允许厘米级误差。
func TestNativeTerrain(t *testing.T) {
	path := os.Getenv("DSP_GALAXY_VEIN_REFERENCE")
	if path == "" {
		t.Skip("未指定游戏地形基准")
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	enabled := make([]int64, 25)
	for i := range enabled {
		enabled[i] = int64(i + 1)
	}
	var seed int32
	planets := map[int64]Planet{}
	var themes map[int64]SurfaceTheme
	checked := map[int]bool{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "galaxy "):
			var count int
			if _, err := fmt.Sscanf(line, "galaxy seed=%d stars=%d", &seed, &count); err != nil {
				t.Fatal(err)
			}
			stars, list, err := Generate(seed, 20200403, count, 1033, 0)
			if err != nil {
				t.Fatal(err)
			}
			themes = Themes(stars, list, enabled)
			for _, planet := range list {
				planets[int64(planet.ID)] = planet
			}
		case strings.HasPrefix(line, "planet "):
			var planetID int64
			var algorithm, themeID int
			var modX, modY, radius, want uint64
			if _, err := fmt.Sscanf(line, "planet id=%d algo=%d theme=%d modX=%x modY=%x radius=%x height=%x", &planetID, &algorithm, &themeID, &modX, &modY, &radius, &want); err != nil {
				t.Fatal(err)
			}
			wetAlgorithm3 := seed == 10 && planetID == 603 && algorithm == 3 && os.Getenv("DSP_GALAXY_ALGORITHM3_WET_HEIGHTS") != ""
			if checked[algorithm] && !wetAlgorithm3 {
				continue
			}
			grid, ok := TerrainHeights(planets[planetID], themes[planetID])
			if !ok {
				continue
			}
			checked[algorithm] = true
			if algorithm == 3 {
				t.Run(fmt.Sprintf("算法3逐点高度_%d", planetID), func(t *testing.T) {
					variable := "DSP_GALAXY_ALGORITHM3_HEIGHTS"
					if wetAlgorithm3 {
						variable = "DSP_GALAXY_ALGORITHM3_WET_HEIGHTS"
					}
					path := os.Getenv(variable)
					if path == "" {
						t.Skip("算法 3 的平台高度不能用摘要验证厘米误差，请指定当前游戏导出的原始高度阵列")
					}
					data, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					if len(data) != len(grid.Heights)*2 {
						t.Fatalf("原始高度阵列长度为 %d，预期 %d 字节", len(data), len(grid.Heights)*2)
					}
					maxDelta, mismatch, waterFlips := 0, 0, 0
					waterLevel := 20000 + int(math.Round(themes[planetID].WaterHeight*100))
					originalGrid := HeightGrid{Precision: grid.Precision, Heights: make([]uint16, len(grid.Heights))}
					for i, height := range grid.Heights {
						original := binary.LittleEndian.Uint16(data[2*i:])
						originalGrid.Heights[i] = original
						delta := int(math.Abs(float64(height) - float64(original)))
						maxDelta = max(maxDelta, delta)
						if delta != 0 {
							mismatch++
						}
						if (int(height) <= waterLevel) != (int(original) <= waterLevel) {
							waterFlips++
						}
					}
					if maxDelta > 3 {
						t.Fatalf("seed=%d planet=%d 高度最大偏差 %d 厘米，超过 3 厘米", seed, planetID, maxDelta)
					}
					t.Logf("核对 %d 个地形点，%d 点高度存在差异，最大偏差 %d 厘米，%d 点跨越主题液面标高，液体原型 %d", len(grid.Heights), mismatch, maxDelta, waterFlips, themes[planetID].WaterItemID)
					if wetAlgorithm3 {
						water := 200 + themes[planetID].WaterHeight
						samples, sampledFlips := 0, 0
						maxSampleDelta := 0.0
						for _, vertex := range terrainVertices() {
							want, valid := TerrainHeightAt(originalGrid, nil, vertex.vector())
							got, known := TerrainHeightAt(grid, nil, vertex.vector())
							if !valid || !known {
								continue
							}
							samples++
							maxSampleDelta = math.Max(maxSampleDelta, math.Abs(got-want))
							if (got <= water) != (want <= water) {
								sampledFlips++
								if math.Abs(got-water) > .05 {
									t.Fatalf("距离液面超过 5 厘米仍误判水陆：原版 %.5f，Go %.5f", want, got)
								}
							}
						}
						t.Logf("核对 %d 个插值方向，最大偏差 %.5f 米，水陆改变 %d 个", samples, maxSampleDelta, sampledFlips)
					}
				})
				continue
			}
			hash := fnv.New64a()
			for _, height := range grid.Heights {
				buf := [2]byte{}
				binary.LittleEndian.PutUint16(buf[:], height)
				hash.Write(buf[:])
			}
			if got := hash.Sum64(); got != want {
				t.Errorf("seed=%d planet=%d algo=%d 高度摘要 %016X/%016X", seed, planetID, algorithm, got, want)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(checked) == 0 {
		t.Fatal("没有有效地形基准")
	}
	t.Logf("逐字节核对 %d 个地形算法，算法 3 单独按实际偏差核对", len(checked)-1)
}
