package build

// 文件职责：从已记录真实请求检查大厂候选准备成本，不启动布局；使用：本轮性能诊断；边界：不改变生产注册表或增加测试钩子。
import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"
)

func largeDiagnosticValue[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}

func largeDiagnosticInput(t *testing.T) (context.Context, *PreparedInput) {
	path := os.Getenv("LARGE_CANDIDATE_REQUEST")
	if path == "" {
		t.Skip("requires recorded request")
	}
	var data struct{ Request Request }
	raw := largeDiagnosticValue(os.ReadFile(path))
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), generationWorkloadKey{}, data.Request.Plan.MachineCount)
	r := largeDiagnosticValue(NormalizeInputRequest(ctx, data.Request))
	f := largeDiagnosticValue(LoadInputDatasetFacts(ctx, r))
	s := largeDiagnosticValue(ValidateInputPlanSource(ctx, r, f))
	p := largeDiagnosticValue(NormalizeInputProductions(ctx, r, f, s))
	flow := largeDiagnosticValue(BuildInputFlowFacts(ctx, p))
	c := largeDiagnosticValue(ValidateInputProductionCapacity(ctx, p))
	tr := largeDiagnosticValue(PlanMinimumInputTransport(ctx, p, c))
	proof := largeDiagnosticValue(ValidateInputTransportCapacity(ctx, tr))
	w := largeDiagnosticValue(EstimateInputWorkload(ctx, tr))
	input := largeDiagnosticValue(ComposePreparedInput(ctx, flow, c, tr, proof, w))
	o := DefaultSurfaceSearchOptions()
	o.Strategy = "native"
	o.ExpandAdjacent = true
	o.IncludeBandFallback = false
	o.PreciseFootprint = false
	surfaces := largeDiagnosticValue(EnumerateSurfaceCandidates(ctx, input, o))
	input = largeDiagnosticValue(PrepareSurfaceCandidate(ctx, input, surfaces[0]))
	t.Logf("belt%d surface%+v", input.request.Setting.BeltLv, surfaces[0])
	return ctx, input
}

func TestLargeCandidateDiagnostic(t *testing.T) {
	ctx, input := largeDiagnosticInput(t)
	options := DefaultConnectedSearchOptions()
	options.InputGroups = []ConnectedInputGroup{ConnectedNearbyInputs}
	specs := largeDiagnosticValue(EnumerateConnectedCandidates(ctx, input, options))
	for i, spec := range specs[:min(16, len(specs))] {
		rows := largeDiagnosticValue(connectedExecutionRows(ctx, input, spec.DenseSpan, spec.CapacityFill))
		t.Logf("%d rows%d span%.0f street%v refine%v bent%v capacity%v order%d", i, rows.rowCount, spec.DenseSpan, spec.Street, spec.Refine, spec.BentRows, spec.CapacityFill, spec.DenseOrder)
	}
}

func TestLargeSingleRouteDiagnostic(t *testing.T) {
	if os.Getenv("LARGE_SINGLE_ROUTE") == "" {
		t.Skip("requires explicit diagnostic")
	}
	ctx, input := largeDiagnosticInput(t)
	ctx, cancel := context.WithTimeout(input.algorithmContext(ctx), 180*time.Second)
	defer cancel()
	options := DefaultConnectedSearchOptions()
	options.InputGroups = []ConnectedInputGroup{ConnectedNearbyInputs}
	specs := largeDiagnosticValue(EnumerateConnectedCandidates(ctx, input, options))
	started := time.Now()
	b, err := assembleConnectedCandidate(ctx, input, specs[0], nil)
	t.Logf("assembly after%s error%v", time.Since(started), err)
	if err == nil || b == nil || ctx.Err() != nil {
		return
	}
	routes := b.transportRoutingRoutes()
	var failed *transportRoute
	for _, route := range routes {
		if strings.Contains(err.Error(), "边 "+route.edge.ID+"、") {
			failed = route
			break
		}
	}
	if failed == nil {
		return
	}
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
	data := map[string]any{"error": err.Error(), "buildings": len(b.all), "source": fmt.Sprint(source.a), "target": fmt.Sprint(target.a), "sourcePortals": len(src), "targetPortals": len(dst)}
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
