package industryworkspace

// 文件职责：用不同充电设定及真实归档验证分工估值的替换、零发电星的施工条件和原版蓝图字节守恒。

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/planet"
	"DysonSphereBox/helper/industryplan"
)

func TestFrozenStationChargeUsesBothStationSettingsAcrossSheets(t *testing.T) {
	ils, pls, collector := sheetTestBuilding(0, 2104, 50), sheetTestBuilding(1, 2103, 49), sheetTestBuilding(2, 2105, 117)
	ils.Parameters = make([]int32, 2048)
	pls.Parameters = make([]int32, 2048)
	ils.Parameters[320], pls.Parameters[320] = 200000, 300000
	collector.Parameters = append([]int32(nil), ils.Parameters...)
	code := sheetTestCode(t, sheetTestDocument(ils, pls, collector))
	parts, err := SplitConstructionBlueprint(context.Background(), code, 2, 0)
	if err != nil || len(parts) != 2 {
		t.Fatalf("两张施工图：%v", err)
	}
	grid := industryplan.External{PlanetID: 12, Kind: "grid-power", PowerMW: 5}
	estimated := industryplan.External{PlanetID: 12, Kind: "charging-peak", PowerMW: 60}
	for _, initial := range [][]industryplan.External{{grid, estimated}, {grid}} {
		b := Bundle{Report: Report{ExportName: "测试产线", External: slices.Clone(initial)}, Codes: map[int64]string{}}
		p := PlanetSheet{ID: 12, Name: "测试星", Warnings: []string{"当前规划需要玩家接入可用电网"}}
		if err := b.addSheets(context.Background(), &p, parts, false); err != nil {
			t.Fatal(err)
		}
		want := []industryplan.External{grid, {PlanetID: 12, Kind: "charging-peak", PowerMW: 30}}
		if !slices.Equal(b.Plan.External, want) || !slices.Equal(b.Report.External, want) || len(p.Warnings) != 1 || !strings.Contains(p.Warnings[0], "先接入可用电网") {
			t.Fatalf("冻结蓝图条件不符：%+v %+v", b.Report.External, p.Warnings)
		}
		b.recordStationCharging(&p, 0)
		if !slices.Equal(b.Report.External, []industryplan.External{grid}) || len(p.Warnings) != 0 {
			t.Fatal("无塔星球残留预估充电条件")
		}
	}
	ils.Parameters[320] = 0
	pls.Parameters = nil
	peak, err := planet.StationChargingMW([]blueprint.Building{ils, pls})
	if err != nil || math.Abs(peak-72) > 1e-6 {
		t.Fatalf("缺省参数应采用两座原版物流塔的初始功率，实际 %g %v", peak, err)
	}
}

// TestReplayInstalledStationCharging 从原 ZIP 原样使用每张 TXT，重算充电说明并发布新包，以原版读取的逐星充电上限和文件哈希独立核对。
func TestReplayInstalledStationCharging(t *testing.T) {
	source, output, native := os.Getenv("DSP_CHARGE_REPLAY_ZIP"), os.Getenv("DSP_CHARGE_REPLAY_OUTPUT"), os.Getenv("DSP_CHARGE_REPLAY_NATIVE")
	if source == "" || output == "" || native == "" {
		t.Skip("需要原始 ZIP、独立输出目录和原版逐图读数")
	}
	archive, err := zip.OpenReader(source)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	entries := make(map[string]*zip.File)
	for _, f := range archive.File {
		entries[f.Name] = f
	}
	var b Bundle
	if err := json.Unmarshal(bookZipEntry(t, entries["规划记录.json"]), &b); err != nil {
		t.Fatal(err)
	}
	b.Codes = make(map[int64]string)
	reads, err := os.Open(native)
	if err != nil {
		t.Fatal(err)
	}
	defer reads.Close()
	reader := csv.NewReader(reads)
	reader.Comma = '\t'
	table, err := reader.ReadAll()
	if err != nil || len(table) < 2 {
		t.Fatalf("缺少原版读取记录：%v", err)
	}
	col := slices.Index(table[0], "stationChargeMW")
	if col < 0 {
		t.Fatal("原版读数缺少 stationChargeMW")
	}
	confirmed := map[int64]float64{}
	for _, row := range table[1:] {
		id, err := strconv.ParseInt(row[0], 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		mw, err := strconv.ParseFloat(row[col], 64)
		if err != nil {
			t.Fatal(err)
		}
		confirmed[id] += mw
	}
	hashes := map[string][sha256.Size]byte{}
	var total float64
	for i := range b.Report.Planets {
		p := &b.Report.Planets[i]
		for j, warning := range p.Warnings {
			if prefix, _, ok := strings.Cut(warning, "；塔充电峰值可达 "); ok {
				p.Warnings[j] = prefix + "。启动时需接电并配置物流机队。"
			}
		}
		var peak float64
		for _, s := range p.Sheets {
			f, ok := entries[s.File]
			if !ok {
				t.Fatal(s.File)
			}
			raw := bookZipEntry(t, f)
			hashes[s.File] = sha256.Sum256(raw)
			b.Codes[s.ID] = string(raw)
			doc, err := blueprint.Decode(string(raw), true)
			if err != nil {
				t.Fatal(err)
			}
			mw, err := planet.StationChargingMW(doc.Buildings)
			if err != nil {
				t.Fatal(err)
			}
			peak += mw
		}
		want, ok := confirmed[p.ID]
		if !ok || math.Abs(want-peak) > 1e-6 {
			t.Fatalf("星球 %d，原版 %.3f MW，产品 %.3f MW", p.ID, want, peak)
		}
		total += peak
		b.recordStationCharging(p, peak)
	}
	if len(confirmed) != len(b.Report.Planets) || len(hashes) != b.Report.BlueprintCount || !slices.Equal(b.Report.External, b.Plan.External) {
		t.Fatal("施工图、原版读数或计划与报告条件不符")
	}
	if err := os.MkdirAll(output, 0755); err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(output, b.Report.ExportName+b.ExportExtension())
	if err := b.Export(name); err != nil {
		t.Fatal(err)
	}
	written, err := zip.OpenReader(name)
	if err != nil {
		t.Fatal(err)
	}
	defer written.Close()
	n := 0
	digests := []string{"file\toriginalSHA256\tcorrectedSHA256"}
	for _, f := range written.File {
		if h, ok := hashes[f.Name]; ok {
			current := sha256.Sum256(bookZipEntry(t, f))
			if current != h {
				t.Fatalf("蓝图字节变化：%s", f.Name)
			}
			digests = append(digests, fmt.Sprintf("%s\t%x\t%x", f.Name, h, current))
			n++
		}
	}
	if n != len(hashes) {
		t.Fatal("新包缺少原图")
	}
	if err := os.WriteFile(filepath.Join(output, "sheet-sha256.tsv"), []byte(strings.Join(digests, "\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	record, err := json.MarshalIndent(b.Report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(output, "matrix-2000.json"), record, 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d 星、%d 图 SHA256 逐张一致，报告与计划的实际配置同时充电上限 %.3f MW，新包 %s", len(confirmed), n, total, name)
}
