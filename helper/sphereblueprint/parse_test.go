package sphereblueprint

// 文件职责：回归公开单层样本及游戏格式边界；使用：go test ./helper/sphereblueprint；边界：不依赖网络、游戏或运行中的 Wails。

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"slices"
	"strings"
	"testing"

	"DysonSphereBox/helper/blueprint"
)

// TestPublicBlueprints 职责：用已知签名、计数和颜色校验真实代码；使用：离线回归；边界：生成器样本来自 Cosmin1490/DysonSphereProgram-BlueprintGenerator 同名文件。
func TestPublicBlueprints(t *testing.T) {
	for _, sample := range []struct {
		name                          string
		nodes, frames, shells, colors int
	}{
		{"60_rotated_colored", 60, 90, 32, 32},
		{"240", 240, 360, 122, 1},
	} {
		t.Run(sample.name, func(t *testing.T) {
			input, err := os.ReadFile("testdata/" + sample.name + ".txt")
			if err != nil {
				t.Fatal(err)
			}
			got, err := Parse(string(input))
			if err != nil {
				t.Fatal(err)
			}
			if got.Type != 1 || got.GameVersion != "0.10.29.21950" || len(got.Layers) != 1 || len(got.Orbits) != 0 {
				t.Fatalf("样本头部或层数错误：%d/%s", got.Type, got.GameVersion)
			}
			l := got.Layers[0]
			colors := make(map[uint32]bool)
			for _, shell := range l.Shells {
				colors[shell.Color] = true
			}
			if len(l.Nodes) != sample.nodes || len(l.Frames) != sample.frames || len(l.Shells) != sample.shells || len(colors) != sample.colors || l.CellColors != "" {
				t.Fatalf("节点/框架/壳面/颜色：%d/%d/%d/%d", len(l.Nodes), len(l.Frames), len(l.Shells), len(colors))
			}
			if sample.name == "60_rotated_colored" {
				n, f, s := l.Nodes[0], l.Frames[0], l.Shells[0]
				if n.Pos[2] != -1.1735453605651855 || n.Color != 0x3c3c3c7f || f.A != 9 || f.B != 10 || f.Proto != 1 || f.Color != 0x1e1e1e7f || s.Seed != 1678413250 || s.Color != 3692313780 || !slices.Equal(s.Nodes, []int{1, 60, 56, 30, 26, 5}) {
					t.Fatalf("颜色、坐标或显式边界错误：%+v %+v %+v", n, f, s)
				}
			}
		})
	}
}

// fixture 职责：按 BinaryWriter 布局构造小型样本；使用：覆盖无公开输入的旧格式及无效引用；边界：测试写入器不调用被测读取逻辑。
type fixture struct{ bytes.Buffer }

// ints 职责：写游戏有符号整数字段；使用：测试固定记录；边界：仅写入内存缓冲。
func (f *fixture) ints(values ...int) {
	for _, v := range values {
		f.Write(binary.LittleEndian.AppendUint32(nil, uint32(v)))
	}
}

// floats 职责：写 IEEE 单精度位模式；使用：方向、旋转、帆色；边界：可显式写入非法浮点以测拒绝路径。
func (f *fixture) floats(values ...float32) {
	for _, v := range values {
		f.Write(binary.LittleEndian.AppendUint32(nil, math.Float32bits(v)))
	}
}

// emptyLayer 职责：写一个具有空对象池的球层；使用：以非连续 ID 区分轨道和结构顺序；边界：版本零无涂装尾段。
func (f *fixture) emptyLayer(version int) {
	f.ints(version, 8, 1, 0, 8, 1, 0, 8, 1, 0)
	if version == 1 {
		f.ints(1)
		f.WriteByte(1)
		f.ints(2)
		f.Write([]byte{1, 2, 3, 4, 5, 6, 7, 8})
	}
}

// orbitRecord 职责：独立构造新旧轨道记录；使用：重复调用覆盖空帆槽；边界：传入半径与标记用于断言关联。
func (f *fixture) orbitRecord(modern bool, id int, radius float32, enabled byte) {
	if modern {
		f.ints(0)
	}
	f.ints(id)
	f.floats(radius, 0, 1, 0, 0)
	f.WriteByte(enabled)
}

// payloadFixture 职责：组合四种类型和两种头部布局；使用：覆盖版本分界两侧；边界：两个球层按半径顺序与 ID 顺序相反。
func payloadFixture(kind int, modern bool) []byte {
	var f fixture
	if modern {
		f.ints(0)
	}
	if kind == 3 || kind == 4 {
		f.ints(-1, 1<<3)
		for i := 1; i <= 20; i++ {
			if i == 3 {
				f.orbitRecord(modern, 3, 800, 0)
			} else {
				f.orbitRecord(modern, 0, 0, 0)
			}
		}
		if modern {
			f.ints(4)
			for range 3 {
				f.floats(0, 0, 0, 0)
			}
			f.floats(0.125, 0.25, 0.5, 1)
		}
	}
	if kind == 2 || kind == 4 {
		f.ints(-1, 1<<5, 3)
		f.WriteByte(1)
		f.orbitRecord(modern, 5, 200, 1)
		f.WriteByte(1)
		f.orbitRecord(modern, 2, 400, 1)
		f.WriteByte(0)
		f.ints(6)
		for i := 0; i < 6; i++ {
			if i == 2 || i == 5 {
				f.WriteByte(1)
				f.emptyLayer(i % 2)
			} else {
				f.WriteByte(0)
			}
		}
	}
	if kind == 1 {
		f.emptyLayer(0)
	}
	return f.Bytes()
}

// encodedFixture 职责：压缩并签署测试载荷；使用：使无效内部字段能抵达相应校验；边界：MD5F 的真实覆盖范围由公开样本另行断言。
func encodedFixture(t *testing.T, version string, kind int, raw []byte) string {
	t.Helper()
	var buf bytes.Buffer
	z := gzip.NewWriter(&buf)
	if _, err := z.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return signedFixture(version, kind, base64.StdEncoding.EncodeToString(buf.Bytes()))
}

// signedFixture 职责：签署任意编码段；使用：验证签名之后的压缩错误；边界：只构造测试文本。
func signedFixture(version string, kind int, encoded string) string {
	text := fmt.Sprintf("DYBP:0,639177631600000000,%s,%d,90\"%s", version, kind, encoded)
	return text + "\"" + blueprint.MD5F([]byte(text))
}

// TestGameVersionBranches 职责：覆盖 ImportOld 和 Import 的四种类型；使用：核对稀疏 ID、可见性、姿态、帆色和逐格字节；边界：旧版不写入载荷版本或帆色表。
func TestGameVersionBranches(t *testing.T) {
	for _, v := range []struct {
		version string
		modern  bool
	}{{"0.9.24.11286", false}, {"0.9.24.11287", true}} {
		for kind := 1; kind <= 4; kind++ {
			t.Run(fmt.Sprintf("%s/%d", v.version, kind), func(t *testing.T) {
				got, err := Parse(encodedFixture(t, v.version, kind, payloadFixture(kind, v.modern)))
				if err != nil {
					t.Fatal(err)
				}
				if got.Type != kind || got.Orbits == nil || got.Layers == nil {
					t.Fatal("类型或空集合错误")
				}
				if kind == 2 || kind == 4 {
					if len(got.Layers) != 2 {
						t.Fatal("球层数量错误")
					}
					a, b := got.Layers[0], got.Layers[1]
					if a.ID != 2 || a.Radius != 400 || a.Visible || a.CellColors != "" || b.ID != 5 || b.Radius != 200 || !b.Visible || b.GridMode != 1 || b.CellColors != "AQIDBAUGBwg=" || !slices.Equal(b.Rotation, []float64{0, 1, 0, 0}) {
						t.Fatalf("球层关联或涂装错误：%+v", got.Layers)
					}
				}
				if kind == 3 || kind == 4 {
					if len(got.Orbits) != 1 {
						t.Fatal("帆轨道数量错误")
					}
					o := got.Orbits[0]
					want := []float64{0.069444, 0.35, 1, 1}
					if v.modern {
						want = []float64{0.125, 0.25, 0.5, 1}
					}
					if o.ID != 3 || o.Enabled || !o.Visible || o.Radius != 800 || !slices.Equal(o.HSVA, want) {
						t.Fatalf("帆轨道属性错误：%+v", o)
					}
				}
			})
		}
	}
}

// TestHistoricalObjects 职责：逐一覆盖节点版本 0 到 5 和旧框架/壳面；使用：检查稀疏池、欧拉弧、旧版默认色和负种子；边界：构造数据按游戏对象版本独立编码。
func TestHistoricalObjects(t *testing.T) {
	for v := 0; v <= 5; v++ {
		var f fixture
		f.ints(0, 8, 5, 1, 0) // 旧单层没有载荷版本；节点池的 1 号槽已回收。
		for id := 2; id <= 4; id++ {
			f.ints(id, v, id, 9)
			f.Write([]byte{1, 0})
			f.floats(float32(id), 1, -1)
			f.ints(30)
			if v >= 2 {
				f.ints(7)
			}
			f.ints(0)
			if v >= 1 {
				f.ints(0)
			}
			f.ints(30)
			if v >= 4 {
				f.ints(0)
			}
			if v >= 5 {
				f.Write([]byte{255, 128, 0, 255})
			}
		}
		f.ints(1, 4, 4, 0)
		for i, pair := range [][2]int{{2, 3}, {3, 4}, {4, 2}} {
			id := i + 1
			f.ints(id, 0, id, 4)
			f.WriteByte(0)
			f.ints(pair[0], pair[1])
			f.WriteByte(1)
			f.ints(10)
		}
		f.ints(2, 2, 0, 1, v%2, 1, 6, -7, 3, 2, 3, 4)
		got, err := Parse(encodedFixture(t, "0.9.24.11286", 1, f.Bytes()))
		if err != nil {
			t.Fatalf("对象版本 %d：%v", v, err)
		}
		l := got.Layers[0]
		color := uint32(0)
		if v == 5 {
			color = 0xff8000ff
		}
		if len(l.Nodes) != 3 || l.Nodes[0].ID != 2 || l.Nodes[0].Color != color || !l.Frames[0].Euler || l.Frames[0].Color != 0 || l.Shells[0].Color != 0 || l.Shells[0].Seed != -7 {
			t.Fatalf("旧记录还原失败，版本 %d", v)
		}
	}
}

// TestInvalidBlueprints 职责：拒绝损坏的信封、超限及错误对象引用；使用：对真实样本进行定点修改后重新签名；边界：断言错误不泄漏半成品。
func TestInvalidBlueprints(t *testing.T) {
	sample, err := os.ReadFile("testdata/60_rotated_colored.txt")
	if err != nil {
		t.Fatal(err)
	}
	code := string(sample)
	raw, err := decompress(strings.Split(code, `"`)[1])
	if err != nil {
		t.Fatal(err)
	}
	const v = "0.10.29.21950"
	badWord := func(offset int, value uint32) string {
		b := bytes.Clone(raw)
		binary.LittleEndian.PutUint32(b[offset:], value)
		return encodedFixture(t, v, 1, b)
	}
	// 样本字段偏移按 60 条 58 字节节点、90 条 34 字节框架及池头独立计算。
	const node = 20
	const frame = node + 60*58 + 12
	const shell = frame + 90*34 + 12
	compressed, err := base64.StdEncoding.DecodeString(strings.Split(code, `"`)[1])
	if err != nil {
		t.Fatal(err)
	}
	invalid := map[string]string{
		"工厂蓝图":   strings.Replace(code, "DYBP:", "BLUEPRINT:", 1),
		"未来版本":   strings.Replace(code, v, "0.10.35.29152", 1),
		"签名":     code[:len(code)-1] + "0",
		"Base64": signedFixture(v, 1, "???"),
		"GZip尾部": signedFixture(v, 1, base64.StdEncoding.EncodeToString(append(bytes.Clone(compressed), 1))),
		"GZip截断": signedFixture(v, 1, base64.StdEncoding.EncodeToString(compressed[:len(compressed)-1])),
		"载荷超限":   encodedFixture(t, v, 1, make([]byte, maxPayloadBytes+1)),
		"文本超限":   strings.Repeat(" ", maxTextBytes+1),
		"载荷尾部":   encodedFixture(t, v, 1, append(bytes.Clone(raw), 0)),
		"载荷截断":   encodedFixture(t, v, 1, raw[:len(raw)-1]),
		"载荷版本":   badWord(0, 1),
		"层版本":    badWord(4, 2),
		"节点容量":   badWord(8, 1<<25),
		"回收游标":   badWord(16, 5),
		"节点版本":   badWord(node+4, 6),
		"槽位":     badWord(node, 2),
		"内层ID":   badWord(node+8, 2),
		"非有限坐标":  badWord(node+18, 0x7fc00000),
		"框架端点":   badWord(frame+17, 0),
		"框架版本":   badWord(frame+4, 2),
		"壳面版本":   badWord(shell+4, 3),
		"壳面环超限":  badWord(shell+24, 100000),
		"壳面缺边":   badWord(shell+28, 3),
		"壳面重复顶点": badWord(shell+28, 60),
	}
	for name, value := range invalid {
		t.Run(name, func(t *testing.T) {
			if got, err := Parse(value); err == nil || got != nil {
				t.Fatal("损坏蓝图被接受")
			}
		})
	}
}
