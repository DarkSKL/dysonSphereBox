package saveanalysis

import (
	"encoding/binary"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestFreeRemoteGroup(t *testing.T) {
	if got := (&Snapshot{}).FreeRemoteGroup(); got != 0 {
		t.Fatalf("未知快照返回了分组 %#x", got)
	}
	s := &Snapshot{digest: "test", stations: []*layoutStationComponent{{remoteGroupMask: 1 | 1<<63}, {remoteGroupMask: 2}}}
	if got := s.FreeRemoteGroup(); got != 4 {
		t.Fatalf("首个空组 %#x", got)
	}
	s.stations[0].remoteGroupMask = 0xffffffff
	s.stations[1].remoteGroupMask = 0x7ffffffe00000000
	if got := s.FreeRemoteGroup(); got != 1<<32 {
		t.Fatalf("应跳过不能作为蓝图组位的符号位，得到 %#x", got)
	}
	s.stations[1].remoteGroupMask |= 1 << 32
	if got := s.FreeRemoteGroup(); got != 0 {
		t.Fatalf("已占满却返回了分组 %#x", got)
	}
}

func TestRemoteAstroRules(t *testing.T) {
	r := reader{data: []byte{}}
	write := func(value uint64, n int) {
		for i := 0; i < n; i++ {
			r.data = append(r.data, byte(value>>(8*i)))
		}
	}
	write(1, 4)
	write(0, 4)
	write(2, 4)
	write(astroRouteKey(300, 400, 1120), 8)
	write(0, 4)
	write(1, 1)
	write(0, 1)
	write(astroRouteKey(301, 401, 1101), 8)
	write(0, 4)
	write(0, 1)
	write(0, 1)
	write(1, 4)
	write(astroRouteKey(301, 401, 1120), 8)
	r.readGalacticTransport()
	s := &Snapshot{astroRoutes: r.astroRoutes, astroBans: r.astroBans}
	if routed, banned := s.RemoteAstroRules(401, 301, 1120); !routed || !banned || r.offset != int64(len(r.data)) {
		t.Fatalf("星系线路和星球禁运没有完整读回：%t %t", routed, banned)
	}
	if routed, banned := s.RemoteAstroRules(301, 401, 1101); routed || banned {
		t.Fatal("禁用航线不能参与配对")
	}
}

// TestSavedRemoteGroups 职责：对照实档预先登记的原版站段和字段；使用：本机验收；边界：样本或独立偏移表缺失时跳过。
func TestSavedRemoteGroups(t *testing.T) {
	path, indexPath := os.Getenv("DSP_SAVE_SAMPLE"), os.Getenv("DSP_REMOTE_GROUP_OFFSETS")
	if path == "" || indexPath == "" {
		t.Skip("需指定原版采样档和已有站点偏移表")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	index, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	s, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	type stationKey struct{ planet, id int64 }
	stations := map[stationKey]uint64{}
	for _, station := range s.stations {
		stations[stationKey{station.planetId, station.id}] = station.remoteGroupMask
	}
	var occupied uint64
	checked := 0
	for _, line := range strings.Split(string(index), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] != "station" {
			continue
		}
		left, a := strconv.Atoi(fields[1])
		right, b := strconv.Atoi(fields[2])
		if a != nil || b != nil || left < 0 || right-left < 80 || right > len(data) {
			t.Fatal("站点偏移无效")
		}
		group := binary.LittleEndian.Uint64(data[right-12 : right-4])
		key := stationKey{int64(binary.LittleEndian.Uint32(data[left+16 : left+20])), int64(binary.LittleEndian.Uint32(data[left+4 : left+8]))}
		actual, ok := stations[key]
		if !ok || actual != group {
			t.Fatalf("站点 %v 的原始分组 %#x，解析 %#x", key, group, actual)
		}
		occupied |= group
		checked++
	}
	if checked == 0 {
		t.Fatal("偏移表未包含站点")
	}
	t.Logf("tick=%d，对照站点 %d/%d，占用位 %#016x，空组 %#016x", s.tick, checked, len(stations), occupied, s.FreeRemoteGroup())
}
