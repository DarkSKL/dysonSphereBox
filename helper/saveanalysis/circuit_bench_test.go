package saveanalysis

import (
	"os"
	"runtime"
	"testing"
	"time"
)

// TestCircuitQueryFootprint 使用显式本地样本记录解析、查询耗时和产线索引的近似驻留开销。
func TestCircuitQueryFootprint(t *testing.T) {
	path := os.Getenv("DSP_CIRCUIT_BENCH_SAMPLE")
	if path == "" {
		t.Skip("未指定性能测量样本")
	}
	start := time.Now()
	s, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	parsed := time.Since(start)
	start = time.Now()
	report, err := s.CircuitLine(104)
	if err != nil {
		t.Fatal(err)
	}
	query := time.Since(start)
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	s.lines = nil
	runtime.GC()
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(s)
	t.Logf("parse=%v query=%v machines=%d index_heap_delta=%d", parsed, query, len(report.Machines), int64(before.HeapAlloc)-int64(after.HeapAlloc))
}
