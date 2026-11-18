package saveanalysis

// 文件职责：验证读取边界、空货槽和查询关联。
// 使用：go test 执行；边界：使用合成数据，不提交游戏存档。

import (
	"encoding/binary"
	"strings"
	"testing"
)

// TestReaderBoundaries 职责：覆盖字符串长度、短读和未知区段；使用：解析器回归；边界：失败必须是明确解码错误。
func TestReaderBoundaries(t *testing.T) {
	text := strings.Repeat("中", 60)
	r := &reader{data: append([]byte{0xb4, 1}, []byte(text)...)}
	if got := r.string(); got != text {
		t.Fatalf("长字符串读取错误: %q", got)
	}
	cases := map[string]func(){
		"短整数":    func() { (&reader{data: []byte{1, 2}}).number(4, true) },
		"短字符串":   func() { (&reader{data: []byte{2, 'x'}}).string() },
		"溢出字符串":  func() { (&reader{data: []byte{255, 255, 255, 255, 127}}).string() },
		"未知货槽版本": func() { (&reader{data: []byte{99, 0, 0, 0}}).readStationStore() },
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if _, ok := recover().(decodeError); !ok {
					t.Fatal("没有返回明确解码错误")
				}
			}()
			run()
		})
	}
	if _, err := Decode([]byte("VFSAVE")); err == nil {
		t.Fatal("截断档头被接受")
	}
	header := make([]byte, 54)
	copy(header, "VFSAVE")
	binary.LittleEndian.PutUint64(header[6:], uint64(len(header)))
	binary.LittleEndian.PutUint32(header[14:], 7)
	binary.LittleEndian.PutUint32(header[24:], 10)
	binary.LittleEndian.PutUint32(header[28:], 99)
	if _, err := Decode(header); err == nil || !strings.Contains(err.Error(), "不支持的游戏版本") {
		t.Fatalf("未知游戏版本: %v", err)
	}
}

// TestIndexAndWindows 职责：验证厂区关联、空槽过滤、容量解码、缺失记录和缓存隔离；使用：查询回归；边界：统计和物流来自同一合成快照。
func TestIndexAndWindows(t *testing.T) {
	totals := make([]int64, 14)
	totals[1] = 120
	totals[8] = 30
	r := &reader{planets: []int64{203, 101}, production: &layoutProductionStatistics{factoryStatPool: []*layoutFactoryProductionStat{
		{productPool: []*layoutProductStat{{itemId: 1001, numCount: 7200, numCursor: 12, total: totals}}},
		{productPool: []*layoutProductStat{}},
	}}, stations: []*layoutStationComponent{{id: 4, planetId: 101, isStellar: 1, storage: []*layoutStationStore{
		{itemId: 0, count: 0, maxWithKeepMode: 10000},
		{itemId: 1001, count: 7, maxWithKeepMode: 100010000, localLogic: 1, remoteLogic: 2},
		{itemId: 99999, count: 5, maxWithKeepMode: 20},
	}}}}
	snapshot := r.buildIndex(12000, "test")
	summary, err := snapshot.Summary(60)
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Items) != 2 || summary.Items[0].ItemID != 1001 || summary.Items[0].History.Produced != 120 || summary.Items[0].Inventory != 7 || summary.Items[0].Capacity != 10000 {
		t.Fatalf("汇总错误: %+v", summary)
	}
	detail, err := snapshot.Item(1001, 60)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Planets) != 2 || detail.Planets[0].PlanetID != 101 || detail.Planets[0].History != nil || detail.Planets[1].PlanetID != 203 || detail.Planets[1].History.Consumed != 30 {
		t.Fatalf("厂区关联错误: %+v", detail)
	}
	slot := detail.Planets[0].Slots[0]
	if slot.Index != 1 || slot.LocalLogic != 1 || slot.RemoteLogic != 2 || slot.Capacity != 10000 {
		t.Fatalf("货槽错误: %+v", slot)
	}
	detail.Planets[0].Slots[0].Inventory = 999
	again, _ := snapshot.Item(1001, 60)
	if again.Planets[0].Slots[0].Inventory != 7 {
		t.Fatal("查询修改污染缓存")
	}
	if _, err := snapshot.Item(0, 60); err == nil {
		t.Fatal("空货槽被当成物品")
	}
	if _, err := snapshot.Summary(61); err == nil {
		t.Fatal("任意窗口被接受")
	}
}
