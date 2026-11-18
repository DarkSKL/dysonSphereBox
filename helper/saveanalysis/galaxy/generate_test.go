package galaxy

// 文件职责：验证生成器的范围、可重复性和关系约束。
// 使用：普通 Go 回归；边界：内部一致性测试不代替游戏独立基准。
import (
	"math"
	"reflect"
	"testing"
)

// TestGenerationContracts 职责：覆盖合法参数、未建厂天体和卫星关系；使用：生成器回归；边界：不验证地形或矿脉。
func TestGenerationContracts(t *testing.T) {
	for _, seed := range []int32{0, 1, -1, 2147483647, -2147483648} {
		for _, count := range []int{32, 64} {
			stars, planets, e := Generate(seed, 20200403, count, 2052, 123456)
			if e != nil {
				t.Fatal(e)
			}
			if len(stars) != count || len(planets) < count {
				t.Fatal("覆盖不完整")
			}
			seen := map[int]bool{}
			for _, p := range planets {
				if p.ID/100 != p.StarID || seen[p.ID] || p.Name == "" || math.IsNaN(p.Position.X) || math.IsInf(p.Position.X, 0) {
					t.Fatalf("无效天体 %+v", p)
				}
				if p.ParentPlanetID != 0 && (!seen[p.ParentPlanetID] || p.ParentPlanetID/100 != p.StarID) {
					t.Fatal("卫星引用错误")
				}
				seen[p.ID] = true
			}
			a, b, _ := Generate(seed, 20200403, count, 2052, 123456)
			if !reflect.DeepEqual(stars, a) || !reflect.DeepEqual(planets, b) {
				t.Fatal("生成不可重复")
			}
		}
	}
	for _, args := range [][2]int{{0, 64}, {20200403, 0}, {20200403, 65}} {
		if _, _, e := Generate(1, args[0], args[1], 2052, 0); e == nil {
			t.Fatal("接受不支持的参数")
		}
	}
}

// TestGiantCatalogSuffix 职责：防止巨星目录名漏掉尾部编号；使用：原游戏方法的受控种子回归；边界：不是玩家样本。
func TestGiantCatalogSuffix(t *testing.T) {
	for _, lcid := range []int{1033, 2052} {
		s := makeStar(22, 64, 1, 7, 20, Vector{1, 2, 3})
		if got := s.starName(lcid, map[string]bool{}); got != "YSC 2205-28" {
			t.Fatalf("目录名丢失编号: %q", got)
		}
	}
}

// TestOrbitPhaseAndMoonPosition 职责：验证轨道相位的秒单位与卫星父位移；使用：受控坐标测试；边界：relativePosition 返回游戏 AU 而非游戏米。
func TestOrbitPhaseAndMoonPosition(t *testing.T) {
	s := starState{planets: []planetState{{parent: -1, radius: 1, period: 60, rotation: quaternion{w: 1}}, {parent: 0, radius: .1, period: 30, rotation: quaternion{w: 1}}}}
	p := s.relativePosition(0, 15)
	if math.Abs(p.X) > 1e-12 || math.Abs(p.Z-1) > 1e-12 {
		t.Fatal("秒与轨道相位换算错误")
	}
	moon := s.relativePosition(1, 15)
	if math.Abs(moon.X+.1) > 1e-12 || math.Abs(moon.Z-1) > 1e-12 {
		t.Fatal("卫星遗漏父行星位移")
	}
}
