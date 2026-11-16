package generator_test

// 文件职责：独立验收case21原请求的完整生成、严格解码和真实规则。
// 使用：本次回归修复及原样重放；边界：不读取历史蓝图、不更改设备、产量、选区或喷涂。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"DysonSphereBox/helper/blueprint"
	core "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/generator"
)

// TestCase21OriginalRequestRegression 职责：原样运行生产工作流及随机验收检查；使用：独立真实回归；边界：任何请求改写、Decode失败或物理长宽比超限都失败并留证。
func TestCase21OriginalRequestRegression(t *testing.T) {
	source := "D:/Workspace/DysonSphere/DysonSphereBox/develop/.local/blueprint-core-refactor/random/seed-20261116-20261116-122055/case-21.json"
	raw, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	var original struct{ Request core.Request }
	if err = json.Unmarshal(raw, &original); err != nil {
		t.Fatal(err)
	}
	requestRaw, err := json.Marshal(original.Request)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(requestRaw)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	started := time.Now()
	result, trace, generateErr := generator.GenerateDetailed(ctx, original.Request)
	elapsed := time.Since(started)
	failures, summary := randomAcceptanceCheck(original.Request, result)
	if generateErr != nil {
		failures = append(failures, generateErr.Error())
	}
	after, err := json.Marshal(original.Request)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(requestRaw) {
		failures = append(failures, "原请求被改写")
	}
	doc, decodeErr := blueprint.Decode(result.Blueprint, true)
	if decodeErr != nil {
		failures = append(failures, decodeErr.Error())
	} else if !doc.ChecksumValid || len(doc.Buildings) != result.BuildingCount {
		failures = append(failures, "严格解码数量或校验和不符")
	}
	output := os.Getenv("CASE21_EVIDENCE_DIR")
	if output == "" {
		output = "D:/Workspace/DysonSphere/DysonSphereBox/develop/client/.local/blueprint-core-refactor/case21-fix/final"
	}
	if err = os.MkdirAll(output, 0755); err != nil {
		t.Fatal(err)
	}
	payload, err := json.MarshalIndent(map[string]any{"Source": source, "RequestSHA256": hex.EncodeToString(digest[:]), "RequestUnchanged": string(after) == string(requestRaw), "Request": original.Request, "Result": result, "Trace": trace, "Elapsed": elapsed.String(), "ChecksFailed": failures, "Summary": summary}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(output, "case-21.json"), payload, 0644); err != nil {
		t.Fatal(err)
	}
	ratio := 0.
	if result.LogisticsSummary != nil {
		ratio = result.LogisticsSummary.Metrics.EnvelopeAspectRatio
	}
	t.Logf("requestSHA256=%x unchanged=%v buildings=%d physicalRatio=%.12f elapsed=%s failures=%v", digest, string(after) == string(requestRaw), result.BuildingCount, ratio, elapsed, failures)
	if len(failures) > 0 {
		t.Fatalf("完整真实验收失败: %v", failures)
	}
}
