package build

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"context"
	"math"
	"testing"
)

func TestNativeTankPortGeometry(t *testing.T) {
	for _, anchor := range []int{-170, 0, 160} {
		for _, item := range []int{2106} {
			b, err := newBuilder(Setting{})
			if err != nil {
				t.Fatal(err)
			}
			host := b.make(item, 10, 10, 0)
			host.record.Index = 0
			host.record.Yaw = 0
			host.record.Yaw2 = 0
			if item == 2106 {
				host.record.Parameters = []int32{1, 1}
			}
			enc := &surfaceEncoding{rootCoordinates: true, areas: []blueprint.Area{{Index: 0, ParentIndex: -1, AreaSegments: int16(longitudeSegmentCount(anchor, 200)), Width: 80, Height: 40}}, plan: &SurfacePlan{GridSegment: 200, PrimaryAreaIndex: 0, AnchorLatitudeGrid: anchor}}
			records := []blueprint.Building{host.record}
			records[0].LocalOffset = blueprint.Point{X: 10, Y: 10}
			records[0].LocalOffset2 = records[0].LocalOffset
			world, err := enc.surfaceWorldPositions(records)
			if err != nil {
				t.Fatal(err)
			}
			cat, err := placement.LoadCatalog()
			if err != nil {
				t.Fatal(err)
			}
			geometry, err := cat.Geometry(int16(item), host.record.ModelIndex)
			if err != nil {
				t.Fatal(err)
			}
			pose, err := enc.recordPose(world[0], 0, 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			for end, slot := range []int{1, 3} {
				port := surfaceSlotPose(pose, geometry.PortPoses[slot], false)
				target := port.position.Add(port.axes[2].Scale(.2))
				point := nativeSingleAreaPoint(enc, target, 10)
				point.z = (math.Sqrt(target.Dot(target)) - 200.2) / 1.3333333
				// 故意给短接头施加超过真实槽公差的横向扰动，修复须由物理端口证明而非只改连接编号。
				point.y += .4
				belt := b.make(2001, point.x, point.y, point.z)
				belt.record.Index = int32(end + 1)
				if end == 0 {
					belt.output = host
					belt.record.OutputObjIdx = 0
					belt.record.OutputToSlot = int8(slot)
				} else {
					belt.input = host
					belt.record.InputObjIdx = 0
					belt.record.InputFromSlot = int8(slot)
				}
				r := belt.record
				r.LocalOffset = blueprint.Point{X: float32(point.x), Y: float32(point.y), Z: float32(point.z)}
				r.LocalOffset2 = r.LocalOffset
				records = append(records, r)
			}
			if err := b.alignNativeTankPorts(context.Background(), enc, records); err != nil {
				t.Fatalf("anchor%d item%d: %v", anchor, item, err)
			}
			if item == 2106 {
				if err := enc.validateTankBufferPorts(context.Background(), records); err != nil {
					t.Fatalf("anchor%d tank: %v", anchor, err)
				}
			} else {
				if err := enc.validateSplitterClearance(context.Background(), records); err != nil {
					t.Fatalf("anchor%d splitter: %v", anchor, err)
				}
			}
			records[1].OutputToSlot = 99
			if err := b.alignNativeTankPorts(context.Background(), enc, records); err == nil {
				t.Fatal("accepted unknown port")
			}
		}
	}
}
