package saveanalysis

// 文件职责：验证实档顺序解码与读取边界。
// 使用：显式设置 DSP_SAVE_SAMPLE 后执行本地实档检查；边界：不提交存档或玩家数据。

import (
	"os"
	"runtime/debug"
	"testing"
	"time"
)

// TestSample 职责：验证选择的本地样本区段完整消费；使用：本地验收；边界：未提供样本时跳过。
func TestSample(t *testing.T) {
	path := os.Getenv("DSP_SAVE_SAMPLE")
	if path == "" {
		t.Skip("未指定实档")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	r := &reader{data: data}
	start := time.Now()
	defer func() {
		if e := recover(); e != nil {
			t.Fatalf("%v\n%s", e, debug.Stack())
		}
	}()
	r.readGameSave()
	if r.offset != int64(len(data)) {
		t.Fatalf("剩余字节 %d", int64(len(data))-r.offset)
	}
	t.Logf("bytes=%d elapsed=%s planets=%v stations=%d statistics=%d", len(data), time.Since(start), r.planets, len(r.stations), len(r.production.factoryStatPool))
}
