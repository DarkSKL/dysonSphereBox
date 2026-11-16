package build

// 文件职责：验证最佳旁路的作用域撤销、幂等、排序及取消事务；使用：定向测试和race；边界：真实规则通过的样本用于生命周期，构造评分仅测试比较器。

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/preview"
)

// observationBestFixture 职责：执行真实输入、装配与完整规则链得到样本；使用：生命周期回归；边界：复用现有独立规则测试助手，不合成校验证明。
func observationBestFixture(t *testing.T) VerifiedSnapshot {
	t.Helper()
	ctx := context.Background()
	input := connectedPreparedTestInput(t, ctx, 1201, 600, densityDense)
	for _, span := range []float64{12, 18, 24, 32} {
		for _, limit := range []float64{16, 20, 25, 30, 60} {
			v, err := connectedVerifiedTestCandidate(ctx, input, ConnectedCandidateSpec{Limit: limit, DenseSpan: span})
			if err != nil {
				t.Logf("span=%g: %v", span, err)
				continue
			}
			if LayoutShapeAccepted(v.Metrics()) {
				return v
			}
			t.Logf("span=%g aspect=%g", span, v.Metrics().EnvelopeAspectRatio)
		}
	}
	t.Fatal("no shape-accepted genuine fixture")
	return VerifiedSnapshot{}
}

// observationBestAttach 职责：为相同不可变真实几何绑定独立旁路身份；使用：作用域事件测试；边界：只复制预览元数据，文档、规则和证明不变，重新核对FreezeValidatedGeometry。
func observationBestAttach(t *testing.T, ctx context.Context, source VerifiedSnapshot, path string, ordinal int) VerifiedSnapshot {
	t.Helper()
	bound, err := WithCandidateObservation(ctx, path, ordinal)
	if err != nil {
		t.Fatal(err)
	}
	p := previewFromContext(bound)
	p.start()
	geometry := *source.geometry.data
	scene := &generationPreviewSnapshot{session: p, scene: preview.Scene{Frame: preview.Frame{CoordinateSystem: preview.AreaCoordinates, Areas: previewAreas(geometry.document.Areas), PrimaryAreaIndex: geometry.document.PrimaryAreaIndex}}}
	for _, record := range geometry.document.Buildings {
		scene.scene.Objects = append(scene.scene.Objects, preview.Object{ID: fmt.Sprint("test-", record.Index), Building: renderPreviewBuilding(record, -1, false)})
	}
	geometry.previewScene = scene
	validation := *source.validation.data
	validation.geometry = &geometry
	value, err := FreezeValidatedGeometry(ctx, GeometrySnapshot{data: &geometry}, FinalValidation{data: &validation})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

// TestObservationBestLifecycle 职责：证明真实验证、重复祖先成功、父selected纠正和rejected清除；使用：规则全链样本；边界：会话复制仅隔离旁路，不改领域结果。
func TestObservationBestLifecycle(t *testing.T) {
	source := observationBestFixture(t)
	var events []Progress
	ctx := WithProgress(context.Background(), func(e Progress) { events = append(events, e) })
	ctx, observer := BeginObservation(ctx, Request{TaskID: "lifecycle", EmitPreviewDeltas: true, PreviewProtocolVersion: 4})
	state := observer.candidates
	first := observationBestAttach(t, ctx, source, "root/surface/candidate:0/layout/candidate:0", 0)
	second := observationBestAttach(t, ctx, source, "root/surface/candidate:1/layout/candidate:0", 0)
	// 同评分晚完成的较早路径仍能接管best。
	for _, value := range []VerifiedSnapshot{second, first} {
		if err := CompleteValidatedObservation(ctx, value); err != nil {
			t.Fatal(err)
		}
	}
	if state.best == nil || state.best.snapshot.session != geometryObservationSession(first.Geometry()) {
		t.Fatal("stable path tie lost")
	}
	before := len(events)
	for range 3 {
		if err := ObserveCandidate(ctx, "succeeded", "root/surface/candidate:0", 0, &first, nil); err != nil {
			t.Fatal(err)
		}
	}
	if len(events) != before {
		t.Fatal("ancestor success repeated best/valid")
	}
	// 选区裁决可以选中已释放的第二场景，必须在撤销第一场景前恢复其完整快照。
	if err := ObserveCandidate(ctx, "selected", "root/surface/candidate:1", 1, &second, nil); err != nil {
		t.Fatal(err)
	}
	if state.best == nil || state.best.snapshot.session != geometryObservationSession(second.Geometry()) {
		t.Fatal("parent selection did not correct best")
	}
	if err := CompleteValidatedObservation(ctx, first); err != nil {
		t.Fatal(err)
	}
	if state.best.snapshot.session != geometryObservationSession(second.Geometry()) {
		t.Fatal("pruned session revived")
	}
	if err := ObserveCandidate(ctx, "rejected", "root/surface/candidate:1", 1, nil, fmt.Errorf("withdraw selected area")); err != nil {
		t.Fatal(err)
	}
	if state.best != nil {
		t.Fatal("parent rejected retained best")
	}
	best, clears := "", 0
	for _, e := range events {
		switch e.Kind {
		case "candidate-best":
			best = e.CandidateID
		case "candidate-best-cleared":
			if e.CandidateID != "" {
				t.Fatal("clear is candidate-scoped")
			}
			best = ""
			clears++
		case "candidate-finish":
			if (e.CandidateStatus == "pruned" || e.CandidateStatus == "rejected") && e.CandidateID == best {
				t.Fatal("best was withdrawn before correction")
			}
		}
	}
	if best != "" || clears != 1 {
		t.Fatal("incorrect scope clear")
	}
	observer.Finish(ctx, &Result{}, fmt.Errorf("test failure"))
	t.Logf("genuine buildings=%d aspect=%g events=%d", len(source.geometry.data.document.Buildings), source.Metrics().EnvelopeAspectRatio, len(events))
}

// TestObservationBestValidationAndRace 职责：核对缺证明拒绝、并发稳定排序和取消收口；使用：真实快照并发回归；边界：取消后不允许绑定或复活会话。
func TestObservationBestValidationAndRace(t *testing.T) {
	source := observationBestFixture(t)
	var mu sync.Mutex
	var events []Progress
	parent, cancel := context.WithCancel(context.Background())
	ctx := WithProgress(parent, func(e Progress) { mu.Lock(); events = append(events, e); mu.Unlock() })
	ctx, observer := BeginObservation(ctx, Request{TaskID: "race", EmitPreviewDeltas: true, PreviewProtocolVersion: 4})
	values := []VerifiedSnapshot{}
	for i := 0; i < 8; i++ {
		values = append(values, observationBestAttach(t, ctx, source, fmt.Sprintf("root/group/candidate:%d", i), i))
	}
	broken := values[0]
	broken.validation = FinalValidation{}
	if err := CompleteValidatedObservation(ctx, broken); err == nil {
		t.Fatal("missing proof accepted")
	}
	if observer.candidates.best != nil {
		t.Fatal("unverified best published")
	}
	var workers sync.WaitGroup
	for _, value := range values {
		workers.Add(1)
		go func(v VerifiedSnapshot) {
			defer workers.Done()
			for range 3 {
				if err := CompleteValidatedObservation(ctx, v); err != nil {
					t.Error(err)
				}
			}
		}(value)
	}
	workers.Wait()
	if observer.candidates.best.snapshot.session != geometryObservationSession(values[0].Geometry()) {
		t.Fatal("concurrent tie is completion-dependent")
	}
	cancel()
	observer.Finish(ctx, nil, nil)
	if observer.candidates.best != nil {
		t.Fatal("canceled task retained snapshot")
	}
	if _, err := WithCandidateObservation(ctx, "root/group/candidate:9", 9); err == nil {
		t.Fatal("canceled task rebound")
	}
	mu.Lock()
	defer mu.Unlock()
	clears := 0
	for _, e := range events {
		if e.Kind == "candidate-best-cleared" {
			clears++
		}
	}
	if clears != 1 {
		t.Fatalf("cancel clears=%d", clears)
	}
}

// TestObservationBestRankingAndSnapshot 职责：核对真实设施数、形状门槛、不可变复制及最终新版本；使用：独立评分样本；边界：构造值仅调用内部比较发布器，不作为完整验证证据。
func TestObservationBestRankingAndSnapshot(t *testing.T) {
	var events []Progress
	ctx := WithProgress(context.Background(), func(e Progress) {
		events = append(events, e)
		if len(e.Operations) > 0 {
			e.Operations[0].Building.ItemID = 999
		}
	})
	ctx, observer := BeginObservation(ctx, Request{EmitPreviewDeltas: true, PreviewProtocolVersion: 4})
	defer observer.Finish(ctx, &Result{}, nil)
	makeValue := func(path string, ordinal int, area, aspect float64, items ...int16) VerifiedSnapshot {
		bound, err := WithCandidateObservation(ctx, path, ordinal)
		if err != nil {
			t.Fatal(err)
		}
		p := previewFromContext(bound)
		p.start()
		g := &finalizedGeometry{previewScene: &generationPreviewSnapshot{session: p, scene: preview.Scene{Frame: preview.Frame{CoordinateSystem: preview.AreaCoordinates}}}}
		for i, item := range items {
			g.document.Buildings = append(g.document.Buildings, blueprint.Building{ItemID: item})
			g.previewScene.scene.Objects = append(g.previewScene.scene.Objects, preview.Object{ID: fmt.Sprint(i), Building: preview.Building{ItemID: item}})
		}
		return VerifiedSnapshot{geometry: GeometrySnapshot{data: g}, validation: FinalValidation{data: &finalValidationData{geometry: g, logistics: &LogisticsSummary{Metrics: LayoutMetrics{EnvelopeArea: area, EnvelopeAspectRatio: aspect}}}}}
	}
	offer := func(v VerifiedSnapshot) bool {
		state := observer.candidates
		state.lifecycle.Lock()
		defer state.lifecycle.Unlock()
		p := geometryObservationSession(v.Geometry())
		p.mu.Lock()
		defer p.mu.Unlock()
		return state.offerBestLocked(ctx, v, false)
	}
	a := makeValue("root/candidate:0", 0, 100, 1.2, 2103, 2107)
	b := makeValue("root/candidate:1", 1, 200, 1.4, 2103, 2001)
	bad := makeValue("root/candidate:2", 2, 10, 2.0, 2001)
	if !offer(a) || !offer(b) || offer(bad) {
		t.Fatal("facility/shape policy mismatch")
	}
	if better, err := BetterAdjacentCandidate(b, a); err != nil || !better {
		t.Fatal("adjacent comparison differs")
	}
	saved := observer.candidates.best.snapshot
	b.geometry.data.previewScene.scene.Objects[0].Building.ItemID = 888
	if saved.scene.Objects[0].Building.ItemID != 2103 {
		t.Fatal("best shares source or observer mutation")
	}
	p := saved.session
	p.finish("valid")
	if err := commitGenerationPreview(ctx, Result{previewScene: saved, BuildingCount: len(saved.scene.Objects)}); err != nil {
		t.Fatal(err)
	}
	if p.revision != 2 {
		t.Fatal("final reused best revision")
	}
	if !reflect.DeepEqual(saved.scene.Frame, b.geometry.data.previewScene.scene.Frame) {
		t.Fatal("frame lost")
	}
}

// TestObservationBestSelectedClear 职责：验证父selected不能恢复已拒绝会话时先clear再裁剪；使用：真实完整规则样本；边界：不选择替代算法结果，不让迟到祖先事件复活场景。
func TestObservationBestSelectedClear(t *testing.T) {
	source := observationBestFixture(t)
	var events []Progress
	ctx := WithProgress(context.Background(), func(e Progress) { events = append(events, e) })
	ctx, observer := BeginObservation(ctx, Request{EmitPreviewDeltas: true, PreviewProtocolVersion: 4})
	defer observer.Finish(ctx, &Result{}, fmt.Errorf("test failure"))
	first := observationBestAttach(t, ctx, source, "root/surface/candidate:0", 0)
	withdrawn := observationBestAttach(t, ctx, source, "root/surface/candidate:1", 1)
	if err := CompleteValidatedObservation(ctx, first); err != nil {
		t.Fatal(err)
	}
	RejectGeometryObservation(ctx, withdrawn.Geometry())
	if err := ObserveCandidate(ctx, "selected", "root/surface/candidate:1", 1, &withdrawn, nil); err != nil {
		t.Fatal(err)
	}
	if observer.candidates.best != nil {
		t.Fatal("selected restored rejected scene")
	}
	before := len(events)
	if err := CompleteValidatedObservation(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := CompleteValidatedObservation(ctx, withdrawn); err != nil {
		t.Fatal(err)
	}
	if len(events) != before {
		t.Fatal("withdrawn scene revived")
	}
	clearAt, pruneAt := -1, -1
	for i, e := range events {
		if e.Kind == "candidate-best-cleared" {
			clearAt = i
		}
		if e.Kind == "candidate-finish" && e.CandidateStatus == "pruned" {
			pruneAt = i
		}
	}
	if clearAt < 0 || pruneAt <= clearAt {
		t.Fatal("clear did not precede parent pruning")
	}
}

// TestObservationBestSelectedRestore 职责：验证父分支撤销后仅selected即可恢复已有完整验证的备用；使用：真实证明和已valid会话；边界：不依赖再次succeeded，不跨作用域覆盖原best，恢复仍服从所有原有守卫。
func TestObservationBestSelectedRestore(t *testing.T) {
	source := observationBestFixture(t)
	var events []Progress
	ctx := WithProgress(context.Background(), func(e Progress) { events = append(events, e) })
	ctx, observer := BeginObservation(ctx, Request{TaskID: "selected-restore", EmitPreviewDeltas: true, PreviewProtocolVersion: 4})
	defer observer.Finish(ctx, &Result{}, fmt.Errorf("test complete"))
	first := observationBestAttach(t, ctx, source, "root/preferred/candidate:0", 0)
	fallback := observationBestAttach(t, ctx, source, "root/reserve/candidate:0", 0)
	for _, v := range []VerifiedSnapshot{first, fallback} {
		if err := CompleteValidatedObservation(ctx, v); err != nil {
			t.Fatal(err)
		}
	}
	state := observer.candidates
	if state.best == nil || state.best.snapshot.session != geometryObservationSession(first.Geometry()) {
		t.Fatal("preferred stable candidate missing")
	}
	before := len(events)
	if err := ObserveCandidate(ctx, "selected", "root/reserve/candidate:0", 0, &fallback, nil); err != nil {
		t.Fatal(err)
	}
	if len(events) != before || state.best.snapshot.session != geometryObservationSession(first.Geometry()) {
		t.Fatal("selected overrode better best in another scope")
	}
	if err := ObserveCandidate(ctx, "rejected", "root/preferred/candidate:0", 0, nil, fmt.Errorf("parent failed")); err != nil {
		t.Fatal(err)
	}
	if state.best != nil {
		t.Fatal("parent reject did not clear")
	}
	before = len(events)
	if err := ObserveCandidate(ctx, "selected", "root/reserve/candidate:0", 0, &fallback, nil); err != nil {
		t.Fatal(err)
	}
	if state.best == nil || state.best.snapshot.session != geometryObservationSession(fallback.Geometry()) {
		t.Fatal("nil best was not restored from selected verified")
	}
	restored := events[before:]
	if len(restored) < 2 || restored[len(restored)-1].Kind != "candidate-best" {
		t.Fatal("selected did not commit full best snapshot")
	}
	count := 0
	for _, e := range restored {
		if e.Kind != "preview-snapshot" && e.Kind != "candidate-best" {
			t.Fatal("selected re-emitted valid or another lifecycle event")
		}
		if e.Kind == "preview-snapshot" {
			count += len(e.Operations)
		}
	}
	if count != len(source.geometry.data.document.Buildings) {
		t.Fatal("restored snapshot incomplete")
	}
	before = len(events)
	if err := ObserveCandidate(ctx, "selected", "root/reserve/candidate:0", 0, &fallback, nil); err != nil {
		t.Fatal(err)
	}
	if len(events) != before {
		t.Fatal("repeated selected re-emitted best")
	}
	// 清空后依次验证跨任务、缺完整证明、pruned与取消均不能走恢复路径。
	if err := ObserveCandidate(ctx, "rejected", "root/reserve/candidate:0", 0, nil, fmt.Errorf("withdraw fallback")); err != nil {
		t.Fatal(err)
	}
	foreignCtx, foreignObserver := BeginObservation(WithProgress(context.Background(), func(Progress) {}), Request{EmitPreviewDeltas: true, PreviewProtocolVersion: 4})
	foreign := observationBestAttach(t, foreignCtx, source, "foreign/candidate:0", 0)
	defer foreignObserver.Finish(foreignCtx, &Result{}, nil)
	broken := observationBestAttach(t, ctx, source, "root/invalid/candidate:0", 0)
	broken.validation = FinalValidation{}
	pruned := observationBestAttach(t, ctx, source, "root/pruned/candidate:0", 0)
	geometryObservationSession(pruned.Geometry()).finish("pruned")
	canceled := observationBestAttach(t, ctx, source, "root/canceled/candidate:0", 0)
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()
	for _, test := range []struct {
		ctx  context.Context
		path string
		v    VerifiedSnapshot
	}{
		{ctx, "foreign/candidate:0", foreign},
		{ctx, "root/invalid/candidate:0", broken},
		{ctx, "root/pruned/candidate:0", pruned},
		{canceledCtx, "root/canceled/candidate:0", canceled},
	} {
		before = len(events)
		if err := ObserveCandidate(test.ctx, "selected", test.path, 0, &test.v, nil); err != nil {
			t.Fatal(err)
		}
		if state.best != nil || len(events) != before {
			t.Fatal("invalid selected restored nil best")
		}
	}
}
