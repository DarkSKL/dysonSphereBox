package generator_test

// 文件职责：重放原始普通配送与塔请求，保留完整生成和严格形状验收证据。
// 使用：STANDARD_SHAPE_REPLAY指定原始JSON；边界：不改请求、不跳过硬规则，临时文件由主线统一清理。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	core "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/generator"
)

// TestStandardShapeRefineReplay 职责：原样生成并执行完整随机验收检查；使用：独立真实请求重放；边界：任何失败都留证并使测试失败。
func TestStandardShapeRefineReplay(t *testing.T) {
	path := os.Getenv("STANDARD_SHAPE_REPLAY")
	if path == "" {
		t.Skip("指定原始请求后执行真实重放")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var original struct{ Request core.Request }
	if err := json.Unmarshal(raw, &original); err != nil {
		t.Fatal(err)
	}
	requestRaw, err := json.Marshal(original.Request)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(requestRaw)
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	result, trace, generateErr := generator.GenerateDetailed(ctx, original.Request)
	elapsed := time.Since(started)
	failures, summary := randomAcceptanceCheck(original.Request, result)
	if generateErr != nil {
		failures = append(failures, generateErr.Error())
	}
	currentRaw, err := json.Marshal(original.Request)
	if err != nil {
		t.Fatal(err)
	}
	if string(currentRaw) != string(requestRaw) {
		failures = append(failures, "生成改写了原始请求")
	}
	output := "D:/Workspace/DysonSphere/DysonSphereBox/develop/.local/blueprint-core-refactor/standard-shape"
	if err := os.MkdirAll(output, 0755); err != nil {
		t.Fatal(err)
	}
	workflowDirectory := os.Getenv("DSBOX_BLUEPRINT_WORKFLOWS")
	evidence := map[string]any{"Source": path, "WorkflowDirectory": workflowDirectory, "RequestSHA256": hex.EncodeToString(digest[:]), "Request": original.Request, "Result": result, "Trace": trace, "Elapsed": elapsed.String(), "ChecksFailed": failures, "Summary": summary}
	name := filepath.Base(path)
	if workflowDirectory != "" {
		name = "comparison-" + name
	}
	randomAcceptanceJSON(t, filepath.Join(output, name), evidence)
	ratio := 0.
	if result.LogisticsSummary != nil {
		ratio = result.LogisticsSummary.Metrics.EnvelopeAspectRatio
	}
	t.Logf("buildings=%d actualRatio=%.9f elapsed=%s failures=%v", result.BuildingCount, ratio, elapsed, failures)
	if len(failures) > 0 {
		t.Fatalf("完整生成验收失败: %v", failures)
	}
}
