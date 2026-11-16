package build

import (
	"context"
	"math"
	"testing"
)

func TestSurfaceBufferPathResampling(t *testing.T) {
	b, err := newBuilder(Setting{})
	if err != nil {
		t.Fatal(err)
	}
	a := b.make(2001, 0, 0, 0)
	z := b.make(2001, 3, 0, 0)
	connect(a, z)
	buffer := &productionBuffer{source: a, feed: &transportRoute{path: []*placed{a, z}}, main: []*placed{a, z}, intake: []*placed{a, z}, outlet: []*placed{z, a}, storeIntake: []*placed{a, z}, storeOutlet: []*placed{z, a}, entities: []*placed{a, z}}
	b.productionBuffers = []*productionBuffer{buffer}
	b.polarFrame = &densePolarFrame{start: 0, height: 20, segments: 200, latitudeStep: float32(math.Pi / 500), longitudeStep: float32(math.Pi / 500)}
	units := []denseSpatialUnit{{entities: []*placed{a, z}, buffer: buffer}}
	if err = b.resamplePolarLocalBelts(context.Background(), units); err != nil {
		t.Fatal(err)
	}
	if len(b.all) <= 2 {
		t.Fatal("expected actual belt subdivision")
	}
	for _, path := range [][]*placed{buffer.feed.path, buffer.main, buffer.intake, buffer.outlet, buffer.storeIntake, buffer.storeOutlet, buffer.entities, units[0].entities} {
		if len(path) != len(b.all) {
			t.Fatalf("path evidence lost subdivided belts: %d vs %d", len(path), len(b.all))
		}
	}
	for _, path := range [][]*placed{buffer.feed.path, buffer.main, buffer.intake, buffer.storeIntake} {
		for i := 1; i < len(path); i++ {
			if path[i-1].output != path[i] {
				t.Fatal("forward evidence disconnected")
			}
		}
	}
	for _, path := range [][]*placed{buffer.outlet, buffer.storeOutlet} {
		for i := 1; i < len(path); i++ {
			if path[i].output != path[i-1] {
				t.Fatal("reverse evidence disconnected")
			}
		}
	}
	count := len(b.all)
	if err = b.resamplePolarLocalBelts(context.Background(), units); err != nil {
		t.Fatal(err)
	}
	if len(b.all) != count {
		t.Fatal("duplicate resampling")
	}
}
