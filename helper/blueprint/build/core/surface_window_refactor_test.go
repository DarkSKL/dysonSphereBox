package build

import (
	"context"
	"testing"
)

func TestSurfaceWindowRequestBounds(t *testing.T) {
	for _, zone := range DensityLatitudeOptions() {
		for _, expand := range []bool{false, true} {
			input := &PreparedInput{request: Request{Mode: 1, Density: densityDense, SurfaceLayout: &SurfaceLayout{Mode: "explicitTropic", LatitudeZone: zone.Value, GridSegment: 200, ExpandAdjacent: expand}}, fingerprint: "surface-test"}
			options := DefaultSurfaceSearchOptions()
			options.ExpandAdjacent = expand
			specs, err := EnumerateSurfaceCandidates(context.Background(), input, options)
			if err != nil {
				t.Fatalf("%s expand=%t: %v", zone.Value, expand, err)
			}
			for _, spec := range specs {
				if _, err := PrepareSurfaceCandidate(context.Background(), input, spec); err != nil {
					t.Fatalf("%s expand=%t spec=%+v: %v", zone.Value, expand, spec, err)
				}
			}
			bad := SurfaceCandidateSpec{Strategy: "native-window", ExtentRows: 501, ExpandAdjacent: expand}
			if _, err := PrepareSurfaceCandidate(context.Background(), input, bad); err == nil {
				t.Fatalf("accepted impossible window for %s", zone.Value)
			}
		}
	}
}

func TestPolarWindowConfigurationRemainsBounded(t *testing.T) {
	for _, rows := range []int{0, 5, 113, 157} {
		options := DefaultSurfaceSearchOptions()
		options.PolarWindowRows = []int{rows}
		if ValidateSurfaceSearchOptions(options) == nil {
			t.Fatalf("accepted polar config %d", rows)
		}
	}
}
