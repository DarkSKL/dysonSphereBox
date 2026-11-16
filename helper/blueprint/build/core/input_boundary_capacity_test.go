package build

// 文件职责：核对早期原料带下界不混用厂内层高；使用：大厂容量优化临时回归；边界：不代替最终几何和接口连通性。
import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"errors"
	"testing"
)

func TestInputBoundaryCapacityConservative(t *testing.T) {
	req := Request{Mode: 1, Density: "dense", SurfaceLayout: &SurfaceLayout{Mode: "explicitTropic", GridSegment: 200, LatitudeZone: "equator", ExpandAdjacent: false}, Plan: quant.Result{RawInputs: []quant.ItemRate{{ItemID: 1005, RatePerMinute: 194400}, {ItemID: 1001, RatePerMinute: 36000}, {ItemID: 1002, RatePerMinute: 14400}}}}
	err := inputBoundaryCapacity(req, Setting{BeltLv: 1})
	var domain *blueprint.Error
	if !errors.As(err, &domain) || domain.Code != "GEN_E_BELT_CAPACITY" {
		t.Fatalf("Mk1 must reject impossible lower bound: %v", err)
	}
	if err := inputBoundaryCapacity(req, Setting{BeltLv: 2}); err != nil {
		t.Fatal(err)
	}
	req.SurfaceLayout.ExpandAdjacent = true
	if err := inputBoundaryCapacity(req, Setting{BeltLv: 1}); err != nil {
		t.Fatal("explicit adjacent capacity ignored", err)
	}
	req.Mode = 3
	if err := inputBoundaryCapacity(req, Setting{BeltLv: 1}); err != nil {
		t.Fatal("station mode incorrectly rejected", err)
	}
	req.Mode = 1
	req.SurfaceLayout.Mode = "planar"
	if err := inputBoundaryCapacity(req, Setting{BeltLv: 1}); err != nil {
		t.Fatal("planar incorrectly rejected", err)
	}
}
