package build

// 文件职责：把 Go 网格和球面中心变换与当前游戏 DLL 直接求值的数值事实逐项对照。
// 使用：普通 Go 测试离线复核；开发时通过 PowerShell 脚本重新核对 fixture 的游戏来源。
// 边界：测试数据来自 .NET 独立调用，不替代 Unity 场景中的碰撞或粘贴验收。

import (
	"encoding/json"
	"math"
	"os"
	"testing"

	"DysonSphereBox/helper/blueprint"
)

// TestSurfaceContractMatchesNativeGameFixture 职责：全范围比较游戏 501 个纬向索引及45个球面位置；使用：坐标契约回归；边界：允许跨运行时三角函数末位一 ULP 的误差，格网结果必须精确相等。
func TestSurfaceContractMatchesNativeGameFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/game-grid-25955035.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Segment struct {
			Minimum int   `json:"minLatitudeGrid"`
			Maximum int   `json:"maxLatitudeGrid"`
			Counts  []int `json:"longitudeSegmentsByLatitudeGrid"`
		} `json:"segment200"`
		World struct {
			Samples []struct {
				Anchor   int             `json:"anchorLatitudeGrid"`
				Segments int             `json:"longitudeSegments"`
				Local    blueprint.Point `json:"localOffset"`
				World    blueprint.Point `json:"worldPosition"`
			} `json:"samples"`
		} `json:"worldPointSamples"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Segment.Counts) != 501 || len(fixture.World.Samples) != 45 {
		t.Fatalf("fixture missing native coverage: rows=%d points=%d", len(fixture.Segment.Counts), len(fixture.World.Samples))
	}
	for i, want := range fixture.Segment.Counts {
		row := i - 250
		if got := longitudeSegmentCount(row, 200); got != want {
			t.Errorf("native grid mismatch at row %d: got %d, want %d", row, got, want)
		}
	}
	for _, sample := range fixture.World.Samples {
		encoding := &surfaceEncoding{areas: []blueprint.Area{{Index: 0, ParentIndex: -1, AreaSegments: int16(sample.Segments)}}, plan: &SurfacePlan{GridSegment: 200, AnchorLatitudeGrid: sample.Anchor}}
		positions, err := encoding.surfaceWorldPositions([]blueprint.Building{{AreaIndex: 0, LocalOffset: sample.Local}})
		if err != nil {
			t.Fatal(err)
		}
		got := []float32{float32(positions[0].x), float32(positions[0].y), float32(positions[0].z)}
		want := []float32{sample.World.X, sample.World.Y, sample.World.Z}
		for dimension := range got {
			distance := int64(math.Float32bits(got[dimension])) - int64(math.Float32bits(want[dimension]))
			if distance < -1 || distance > 1 {
				t.Errorf("native world mismatch anchor=%d local=%+v axis=%d: got %.9g want %.9g (%d ULP)", sample.Anchor, sample.Local, dimension, got[dimension], want[dimension], distance)
			}
		}
	}
}
