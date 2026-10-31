package transport

// 文件职责：锁定分拣器原型事实和零科技计时矩阵；使用：go test 容量回归；边界：纯数值检查，不模拟真实带上物品相位或 Unity 场景。

import (
	"encoding/json"
	"math"
	"testing"
)

// TestInserterVerifiedSourceAndRawSTT 职责：核对资源数值和已审核来源；使用：目录更新审查；边界：指纹或原型变化必须显式更新测试与事实说明。
func TestInserterVerifiedSourceAndRawSTT(t *testing.T) {
	profile, err := InserterCapacityProfile()
	if err != nil {
		t.Fatal(err)
	}
	if profile.SteamBuildID != "25955035" || profile.GameVersion != "0.10.35.29151" || profile.AssemblySHA256 != "ef314425dade51f529f184f07be136cd3c2660a6a81515603c7226fbe7ab1d8c" || profile.ResourcesSHA256 != "8343173562b15b06a400d70cb1510a106d7654f96342857e5925ded5fd1900d1" {
		t.Fatalf("unreviewed source %+v", profile)
	}
	var data struct {
		Inserters []struct {
			Grade int     `json:"grade"`
			STTF  float32 `json:"sttf"`
			STT   int     `json:"stt"`
		} `json:"inserters"`
	}
	if err := json.Unmarshal(inserterFactsData, &data); err != nil {
		t.Fatal(err)
	}
	want := []int{200000, 100000, 50000, 30000}
	for _, fact := range data.Inserters {
		derived := int(math.RoundToEven(float64(fact.STTF * 600000)))
		if fact.Grade < 1 || fact.Grade > 4 || fact.STT != want[fact.Grade-1] || fact.STT != derived {
			t.Fatalf("raw STT mismatch %+v derived %d", fact, derived)
		}
	}
}

// TestInserterDistanceCapacityMatrix 职责：逐等级和跨距验证单臂硬上限；使用：端口瓶颈矩阵；边界：每个值都按独立整数 tick 周期核对。
func TestInserterDistanceCapacityMatrix(t *testing.T) {
	ticks := [][]int{{40, 80, 120}, {20, 40, 60}, {10, 20, 30}, {8, 14, 20}}
	for level := 1; level <= 4; level++ {
		for distance := 1; distance <= 3; distance++ {
			got, err := InserterCapacityPerMinute(level, distance)
			if err != nil {
				t.Fatal(err)
			}
			payload := 1
			if level == 4 {
				payload = 2
			}
			want := float64(payload*3600) / float64(ticks[level-1][distance-1])
			if math.Abs(got-want) > 1e-9 {
				t.Fatalf("level=%d distance=%d capacity=%g want %g", level, distance, got, want)
			}
		}
	}
	for _, input := range [][2]int{{0, 1}, {5, 1}, {1, 0}, {3, 4}} {
		if _, err := InserterCapacityPerMinute(input[0], input[1]); err == nil {
			t.Fatalf("unverified input accepted %v", input)
		}
	}
}
