package build

// 文件职责：只重放固定case05的首个真实塔候选并定位实际端口阻塞；边界：不修改生产、不更换输入、不关闭规则，装配成功不冒充完整验收。
import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// stationCase05ActualEnvironment 职责：仅由本机测试overlay捕获实际寻路环境；边界：生产源码没有此钩子。
var stationCase05ActualEnvironment *transportEnvironment

func TestStationCase05PortDiagnostic(t *testing.T) {
	if os.Getenv("LARGE_SINGLE_ROUTE") == "" {
		t.Skip("requires explicit diagnostic")
	}
	ctx, input := largeDiagnosticInput(t)
	ctx, cancel := context.WithTimeout(input.algorithmContext(ctx), 180*time.Second)
	defer cancel()
	options := DefaultStationSearchOptions()
	options.Family = "blackbox"
	options.StandardRowPacking = os.Getenv("STATION_CASE05_ROWS") != ""
	specs := largeDiagnosticValue(EnumerateStationCandidates(ctx, input, options))
	if chosen := os.Getenv("STATION_CASE05_SPEC"); chosen != "" {
		i := largeDiagnosticValue(strconv.Atoi(chosen))
		if i < 0 || i >= len(specs) {
			t.Fatal("invalid candidate")
		}
		specs[0] = specs[i]
	}
	started := time.Now()
	spec := specs[0]
	ctx = spec.algorithmContext(ctx, input.request)
	if spec.RowPacking {
		ctx = context.WithValue(ctx, denseBalancedFrameKey{}, false)
	}
	b := largeDiagnosticValue(input.newAssembly(ctx, nil))
	var beforeIndex *denseRoutePortIndex
	var beforeAll []*placed
	lastCompleted := 0
	ctx = WithProgress(ctx, func(e Progress) {
		if e.Kind == "phase" && e.Stage == "routing" {
			lastCompleted = e.Current
		}
		if e.Stage == "routing-prepare" && beforeIndex == nil {
			beforeIndex = largeDiagnosticValue(b.newDenseRoutePortIndex(ctx))
			beforeAll = append([]*placed(nil), b.all...)
			if path := os.Getenv("STATION_CASE05_PRE_RESERVATION"); path != "" {
				lo, hi := bounds(b.all)
				pre := &transportEnvironment{densePorts: beforeIndex, blocked: map[vector]bool{}, reserved: map[vector]map[*placed]bool{}, minimum: vector{math.Max(-8, math.Floor(lo.x)-8), 0, 0}, maximum: vector{math.Ceil(hi.x) + 8, b.denseLatitudeSpan - 1, transportMaxHeight}}
				if pre.maximum.y < 0 {
					pre.maximum.y = math.Ceil(hi.y) + 8
				}
				routes := b.transportRoutingRoutes()
				pre.reserveDenseBoundaryEscapes(routes)
				rows := []map[string]any{}
				for _, r := range routes {
					if r.from != nil || r.to == nil || r.boundary == nil || !surfaceIsStation(r.boundary.record.ItemID) {
						continue
					}
					src := r.transportSource()
					owned := 0
					for _, span := range pre.reservedSpans {
						if span.owner == src {
							owned++
						}
					}
					portals := pre.densePortals(src, src, r.end, denseExternalStep(r), true, false)
					entry := map[string]any{"source": fmt.Sprint(src.a), "target": fmt.Sprint(r.end.a), "item": r.edge.ItemID, "ownedSpans": owned, "portalsAfterReservation": len(portals)}
					entry["targetPortalsAfterReservation"] = len(pre.densePortals(r.end, src, r.end, denseRotateVector(vector{x: -1}, r.to.quarterTurns), false, false))
					if len(portals) == 0 {
						clear := *pre
						clear.reserved = map[vector]map[*placed]bool{}
						clear.reservedSpans = nil
						clear.reservedSpanCells = nil
						entry["portalsWithoutReservations"] = len(clear.densePortals(src, src, r.end, denseExternalStep(r), true, false))
					}
					rows = append(rows, entry)
				}
				raw := largeDiagnosticValue(json.MarshalIndent(rows, "", "  "))
				if err := os.WriteFile(path, raw, 0644); err != nil {
					t.Fatal(err)
				}
				cancel()
			}
		}
	})
	if spec.RowPacking {
		ctx = b.configureStandardRows(ctx)
	}
	b.generationCtx = ctx
	b.beginPreview()
	b.directBoundaryExtentScale = 1.25
	err := b.assembleStationBlackbox(ctx, input.request, input.dataset, input.policy.station.Config, spec)
	t.Logf("assembly after%s error%v", time.Since(started), err)
	if err == nil || b == nil {
		raw, _ := json.MarshalIndent(map[string]any{"assemblyError": fmt.Sprint(err), "elapsed": time.Since(started).String(), "spec": specs[0]}, "", "  ")
		if writeErr := os.WriteFile(os.Getenv("LARGE_SINGLE_ROUTE"), raw, 0644); writeErr != nil {
			t.Fatal(writeErr)
		}
		return
	}
	routes := b.transportRoutingRoutes()
	var failed *transportRoute
	for _, route := range routes {
		if route.start != nil && route.end != nil && strings.Contains(err.Error(), "边 "+route.edge.ID+"、") && strings.Contains(err.Error(), fmt.Sprintf("端口 (%.0f,%.0f,%.0f)→(%.0f,%.0f,%.0f)", route.start.a.x, route.start.a.y, route.start.a.z, route.end.a.x, route.end.a.y, route.end.a.z)) {
			failed = route
			break
		}
	}
	if failed == nil && ctx.Err() != nil {
		work := []*transportRoute{}
		for _, r := range routes {
			if !(r.to == nil && r.from != nil && r.end == nil) {
				work = append(work, r)
			}
		}
		if lastCompleted < len(work) {
			failed = work[lastCompleted]
		}
		ctx = context.Background()
	}
	if failed == nil {
		return
	}
	actualEnvironment := stationCase05ActualEnvironment
	idx := largeDiagnosticValue(b.newDenseRoutePortIndex(ctx))
	idx.boundary = b.denseBoundary
	lo, hi := bounds(b.all)
	env := &transportEnvironment{densePorts: idx, blocked: map[vector]bool{}, reserved: map[vector]map[*placed]bool{}, minimum: vector{math.Floor(lo.x) - 8, 0, 0}, maximum: vector{math.Ceil(hi.x) + 8, b.denseLatitudeSpan - 1, transportMaxHeight}, preferCorridor: true, beltCorridors: map[[2]int]bool{}}
	if b.polarFrame != nil {
		env.minimum.x = math.Max(env.minimum.x, -8)
		env.maximum.x = math.Min(env.maximum.x, b.polarFrame.longitudeLimit()+8)
	}
	if b.denseBoundary != nil {
		env.minimum.x = math.Max(env.minimum.x, b.denseBoundary.minimumX)
	}
	for _, p := range b.all {
		if surfaceIsBelt(p.record.ItemID) {
			idx.blockNode(env.blocked, p.a)
			idx.addBeltSpan(p)
			env.beltCorridors[beltColumn(p.a)] = true
		}
	}
	env.reserveDenseBoundaryEscapes(routes)
	if os.Getenv("STATION_CASE05_REAL_ENV") != "" && actualEnvironment != nil {
		env = actualEnvironment
		idx = env.densePorts
	}
	source, target := failed.transportSource(), failed.end
	preferred := denseExternalStep(failed)
	if failed.from != nil {
		preferred = denseRotateVector(vector{x: -1}, failed.from.quarterTurns)
	}
	fallback := vector{x: -1}
	if failed.to != nil {
		fallback = denseRotateVector(fallback, failed.to.quarterTurns)
	}
	src := env.densePortals(source, source, target, preferred, true, false)
	dst := env.densePortals(target, source, target, fallback, false, false)
	data := map[string]any{"spec": specs[0], "sourceRecord": source.record, "targetRecord": target.record, "routeFrom": fmt.Sprint(failed.from), "sourceInput": fmt.Sprint(source.input), "sourceOutput": fmt.Sprint(source.output), "sourceStart": fmt.Sprint(failed.start.a), "boundary": fmt.Sprint(failed.boundary), "error": err.Error(), "buildings": len(b.all), "source": fmt.Sprint(source.a), "target": fmt.Sprint(target.a), "sourcePortals": len(src), "targetPortals": len(dst)}
	data["preferred"] = fmt.Sprint(preferred)
	data["completedRoutes"] = lastCompleted
	data["failedEdge"] = failed.edge.ID
	data["failedItem"] = failed.edge.ItemID
	if failed.from != nil {
		data["fromOwner"] = failed.from.owner
	}
	if failed.to != nil {
		data["toOwner"] = failed.to.owner
	}
	data["outward"] = fmt.Sprint(idx.outward(source, preferred, true))
	owned := []map[string]any{}
	for _, span := range env.reservedSpans {
		if span.owner == source {
			conflicts := []string{}
			for _, belt := range idx.belts {
				if span.shape.span.ends[0] == belt.ends[0] || span.shape.span.ends[0] == belt.ends[1] || span.shape.span.ends[1] == belt.ends[0] || span.shape.span.ends[1] == belt.ends[1] {
					continue
				}
				if surfaceSpanDistanceSquared(span.shape.span, belt) < .46*.46-1e-6 {
					conflicts = append(conflicts, fmt.Sprint(belt.ends))
				}
			}
			owned = append(owned, map[string]any{"ends": span.shape.span.ends, "intruders": conflicts})
		}
	}
	data["actualOwnedSpans"] = owned
	if beforeIndex != nil {
		pre := *env
		pre.densePorts = beforeIndex
		pre.blocked = map[vector]bool{}
		pre.reserved = map[vector]map[*placed]bool{}
		for _, p := range beforeAll {
			if surfaceIsBelt(p.record.ItemID) {
				beforeIndex.blockNode(pre.blocked, p.a)
			}
		}
		data["beforeRouting"] = map[string]any{"sourcePortals": len(pre.densePortals(source, source, target, preferred, true, false)), "targetPortals": len(pre.densePortals(target, source, target, fallback, false, false))}
	}
	checks := []map[string]any{}
	for _, delta := range []vector{{x: 1}, {x: 2}, {z: .5}, {z: 1}, {x: 1, z: 1}, {x: 1, z: 2}} {
		point := vector{source.a.x + delta.x, source.a.y + delta.y, source.a.z + delta.z}
		a, c := idx.world(source.a), idx.world(point)
		entry := map[string]any{"point": fmt.Sprint(point), "nodeClear": idx.nodeClear(point, nil), "blocked": env.blocked[beltCell(point)], "reserved": env.reservedByOther(point, source, target), "boundaryAllowed": idx.boundarySpanAllowed(source.a, point), "beltSpanClear": idx.beltSpanClear(source.a, point), "reservedSpanClear": env.reservedSpanClear(source.a, point, source, target), "spanClear": idx.spanClear(source.a, point)}
		boxes := []string{}
		for i, shape := range idx.boxes {
			if shape.box.SegmentDistanceSquared(a, c) < .23*.23-1e-6 {
				boxes = append(boxes, fmt.Sprintf("item%d at%v", idx.owners[i].record.ItemID, idx.owners[i].a))
				if len(boxes) >= 8 {
					break
				}
			}
		}
		entry["blockingBoxes"] = boxes
		nodes := []string{}
		for _, p := range b.all {
			if surfaceIsBelt(p.record.ItemID) {
				d := idx.world(p.a).Sub(c)
				if d.Dot(d) < denseRouteNodeClearance*denseRouteNodeClearance {
					nodes = append(nodes, fmt.Sprintf("at%v item%d sourceSelf%v", p.a, p.record.ItemID, p == source))
					if len(nodes) >= 8 {
						break
					}
				}
			}
		}
		entry["blockingNodes"] = nodes
		spans := []string{}
		probe := beltPackingShape(a, c)
		for _, span := range idx.belts {
			if a == span.ends[0] || a == span.ends[1] || c == span.ends[0] || c == span.ends[1] {
				continue
			}
			if surfaceSpanDistanceSquared(probe.span, span) < .46*.46-1e-6 {
				spans = append(spans, fmt.Sprint(span.ends))
				if len(spans) >= 8 {
					break
				}
			}
		}
		entry["blockingSpans"] = spans
		checks = append(checks, entry)
	}
	data["entryChecks"] = checks
	duplicates := 0
	seenSources := map[vector]bool{}
	for _, r := range routes {
		if r.routingEntry != nil {
			if seenSources[r.routingEntry.a] {
				duplicates++
			}
			seenSources[r.routingEntry.a] = true
		}
	}
	data["duplicateSourcePoints"] = duplicates
	exits := func(portals []denseRoutePortal) []string {
		out := []string{}
		for _, p := range portals {
			last := p.points[len(p.points)-1]
			free := []vector{}
			for _, d := range transportNeighborOffsets {
				n := vector{last.x + d.x, last.y + d.y, last.z + d.z}
				if !env.blocked[beltCell(n)] && !env.reservedByOther(n, source, target) && env.denseTransportStepClear(last, n) {
					free = append(free, n)
				}
			}
			out = append(out, fmt.Sprintf("%v h%.1f free%v", last, p.height, free))
		}
		return out
	}
	data["sourceExits"], data["targetExits"] = exits(src), exits(dst)
	raw := largeDiagnosticValue(json.MarshalIndent(data, "", "  "))
	if err := os.WriteFile(os.Getenv("LARGE_SINGLE_ROUTE"), raw, 0644); err != nil {
		t.Fatal(err)
	}
	t.Logf("src%d dst%d", len(src), len(dst))
}
