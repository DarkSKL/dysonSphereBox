package galaxy

// 文件职责：对照原游戏方法生成的恒星参数与名称。
// 使用：DSP_GALAXY_STAR_REFERENCE 指定本地 TSV；边界：原始基准不随代码提交。
import (
	"bufio"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
)

// TestNativeStars 职责：覆盖不同类型与语言的恒星生成；使用：独立离线回归；边界：不证明地形或资源生成。
func TestNativeStars(t *testing.T) {
	p := os.Getenv("DSP_GALAXY_STAR_REFERENCE")
	if p == "" {
		t.Skip("未指定原生恒星基准")
	}
	f, e := os.Open(p)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	scan := bufio.NewScanner(f)
	n, bad := 0, 0
	maxMass, maxScale := 0.0, 0.0
	for scan.Scan() {
		a := strings.Split(scan.Text(), "\t")
		if len(a) != 9 {
			t.Fatal("无效基准行")
		}
		iv := func(i int) int {
			v, e := strconv.Atoi(a[i])
			if e != nil {
				t.Fatal(e)
			}
			return v
		}
		fv := func(i int) float64 {
			v, e := strconv.ParseFloat(a[i], 32)
			if e != nil {
				t.Fatal(e)
			}
			return v
		}
		s := makeStar(iv(3), 64, iv(1), 7, int32(iv(0)), Vector{1, 2, 3})
		name := s.starName(iv(2), map[string]bool{})
		dm, ds := math.Abs(s.mass-fv(4)), math.Abs(s.scale-fv(5))
		maxMass = math.Max(dm, maxMass)
		maxScale = math.Max(ds, maxScale)
		if name != a[8] || s.kind != iv(6) || s.spectr != iv(7) || dm > 1e-6*math.Max(1, fv(4)) || ds > 1e-6*math.Max(1, fv(5)) {
			bad++
			if bad < 12 {
				t.Logf("seed=%s kind=%s lcid=%s mass=%g/%g scale=%g/%g type=%d/%d spectr=%d/%d name=%s/%s", a[0], a[1], a[2], s.mass, fv(4), s.scale, fv(5), s.kind, iv(6), s.spectr, iv(7), name, a[8])
			}
		}
		n++
	}
	if e = scan.Err(); e != nil {
		t.Fatal(e)
	}
	t.Logf("rows=%d bad=%d maxMass=%g maxScale=%g", n, bad, maxMass, maxScale)
	if n == 0 || bad > 0 {
		t.Fail()
	}
}
