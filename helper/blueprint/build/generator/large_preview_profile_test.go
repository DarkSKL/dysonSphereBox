package generator

// 文件职责：单次采集真实大厂 v4 进度并流式保存严格解码记录。
// 使用：独立 headless 性能验收设置 LARGE_PREVIEW_DIR 后运行本测试。
// 边界：不修改生产请求的算法参数，不缓存全体事件，不验证桌面 IPC。

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"sync"
	"testing"
	"time"

	"DysonSphereBox/helper/blueprint"
	core "DysonSphereBox/helper/blueprint/build/core"
)

// TestLargePreviewProfileCapture 职责：仅运行一次真实 GenerateDetailed 并保存逐条事件及 Decode 分块；使用：指定独占输出目录；边界：输出已存在时拒绝覆盖，采集失败保留部分证据。
func TestLargePreviewProfileCapture(t *testing.T) {
	dir := os.Getenv("LARGE_PREVIEW_DIR")
	if dir == "" {
		t.Skip("独立验收需要 LARGE_PREVIEW_DIR")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "..", "large-station", "after-ownership-transfer", "request.json")
	raw, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	var input struct {
		Request core.Request `json:"request"`
	}
	if err = json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	req := input.Request
	req.TaskID = "large-preview-single-v4"
	if req.PreviewProtocolVersion != 4 || !req.EmitPreviewDeltas || req.Plan.MachineCount != 6007 {
		t.Fatal("输入不符合真实大厂基线")
	}
	f, err := os.OpenFile(filepath.Join(dir, "progress.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := bufio.NewWriterSize(f, 256*1024)
	encoder := json.NewEncoder(w)
	began := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	var mu sync.Mutex
	var captureErr error
	var count, captureNanos int64
	kinds := map[string]int{}
	ctx = core.WithProgress(ctx, func(event core.Progress) {
		mu.Lock()
		defer mu.Unlock()
		if captureErr != nil {
			return
		}
		now := time.Now()
		captureErr = encoder.Encode(map[string]any{"micros": now.Sub(began).Microseconds(), "event": event})
		captureNanos += time.Since(now).Nanoseconds()
		count++
		kinds[event.Kind]++
		if count%512 == 0 {
			if err := w.Flush(); err != nil {
				captureErr = err
			}
			t.Logf("events=%d elapsed=%s", count, time.Since(began))
		}
		if captureErr != nil {
			cancel()
		}
	})
	result, _, runErr := GenerateDetailed(ctx, req)
	returned := time.Since(began).Microseconds()
	if err := w.Flush(); err != nil {
		t.Error(err)
	}
	if err := f.Close(); err != nil {
		t.Error(err)
	}
	// 逻辑流：保存生成终态 => 严格 Decode => 每 512 条独立保存，浏览器只读取当前块。
	write := func(name string, value any) {
		out, e := os.Create(filepath.Join(dir, name))
		if e != nil {
			t.Fatal(e)
		}
		e = json.NewEncoder(out).Encode(value)
		closeErr := out.Close()
		if e != nil {
			t.Fatal(e)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
	}
	write("capture-result.json", map[string]any{"request": req, "result": result, "error": fmt.Sprint(runErr), "returnedMicros": returned, "captureNanos": captureNanos, "events": count, "kinds": kinds, "go": goruntime.Version(), "gomaxprocs": goruntime.GOMAXPROCS(0)})
	if captureErr != nil {
		t.Fatal(captureErr)
	}
	if runErr != nil {
		t.Fatal(runErr)
	}
	doc, err := blueprint.Decode(result.Blueprint, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Buildings) != result.BuildingCount {
		t.Fatal("严格 Decode 数量不一致")
	}
	files := []string{}
	for start := 0; start < len(doc.Buildings); start += 512 {
		name := fmt.Sprintf("decoded-%05d.json", len(files))
		end := min(start+512, len(doc.Buildings))
		write(name, doc.Buildings[start:end])
		files = append(files, name)
	}
	write("metadata.json", map[string]any{"taskId": req.TaskID, "buildingCount": result.BuildingCount, "machineCount": req.Plan.MachineCount, "winnerCandidateId": result.WinnerCandidateID, "decodedChunks": files, "areas": doc.Areas, "returnedMicros": returned, "events": count, "kinds": kinds, "captureNanos": captureNanos})
	t.Logf("buildings=%d events=%d returned=%s capture=%s", result.BuildingCount, count, time.Duration(returned)*time.Microsecond, time.Duration(captureNanos))
}
