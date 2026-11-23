package industryworkspace

// 文件职责：从正式导出入口验证游戏蓝图书的文件、标题和重名保护；使用：小样本回归与给定 1302 旧产线的单星重放。

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/quant"
)

func TestBlueprintBookAndShareUseTheSameSheets(t *testing.T) {
	doc := sheetTestDocument(sheetTestBuilding(0, 2201, 44))
	doc.Reform = &blueprint.ReformData{Rects: []blueprint.ReformRect{{W: 6, H: 1, Data: 224}}}
	parts, err := SplitConstructionBlueprint(context.Background(), sheetTestCode(t, doc), 1, 4)
	if err != nil {
		t.Fatal(err)
	}
	b := Bundle{Report: Report{ExportName: "电磁矩阵-600每分钟", Exportable: true, SaveName: "回归样本&存档", Tick: 123}, Codes: map[int64]string{}}
	site := PlanetSheet{ID: 101, Name: "施工星", Star: "恒星", Longitude: 82, Latitude: 3, Buildings: 1, FoundationCells: 6}
	verifyBookExport(t, t.TempDir(), &b, site, parts)
}

// TestConstructionGuideDistinguishesPlayerTransfersAndStartup 职责：区分玩家运输、仓储、外排和能源开工条件；使用：核验蓝图书与 ZIP 的共用说明；边界：仓满时间只适用于实际缓冲。
func TestConstructionGuideDistinguishesPlayerTransfersAndStartup(t *testing.T) {
	off := false
	req := industryplan.Request{Quant: quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 60}}},
		CenterPlanetID: 101, DeliveryPlanetIDs: map[int]int64{1101: 101}, EnableLogistics: &off}
	world := industryplan.World{Planets: []industryplan.Planet{
		{ID: 101, StarID: 1, Buildable: true, ResourcesKnown: true},
		{ID: 102, StarID: 1, Buildable: true, ResourcesKnown: true, Position: [3]float64{50000}, Resources: []industryplan.Resource{{ItemID: 1001, Kind: "solid", Count: 200}}},
	}}
	plan, err := industryplan.Plan(t.Context(), req, world, nil, nil)
	if err != nil || len(plan.Routes) == 0 {
		t.Fatalf("手动运输样例缺少路线：%v", err)
	}
	b := Bundle{Report: Report{External: plan.External, Routes: plan.Routes}}
	for _, p := range plan.Planets {
		b.Report.Planets = append(b.Report.Planets, PlanetSheet{ID: p.PlanetID, Ports: p.Ports})
	}
	b.Report.Planets = append(b.Report.Planets, PlanetSheet{ID: 103, Name: "仓储星", Ports: []industryplan.Port{
		{Kind: "storage", ItemID: 1114, RatePerMinute: 50, StorageItems: 1000},
		{Kind: "external-export", ItemID: 1120, RatePerMinute: 10},
	}})
	b.Report.External = append(b.Report.External, industryplan.External{Kind: "battery-startup", PlanetID: 103, ItemID: 2206}, industryplan.External{Kind: "solar-conditions", PlanetID: 103})
	guide := b.instructions()
	for _, want := range []string{"玩家运输 102 → 101　铁矿 60", "持续消纳精炼油 50", "空仓约 20.0 分钟装满", "持续消纳氢 10", "投入第一批空蓄电器", "确认日照并充满跨夜蓄电器"} {
		if !strings.Contains(guide, want) {
			t.Errorf("施工说明缺少 %q", want)
		}
	}
	for _, bad := range []string{"持续消纳铁矿", "外供 103"} {
		if strings.Contains(guide, bad) {
			t.Errorf("施工说明误报 %q", bad)
		}
	}
}

// TestReplayPlanet1302Book 只使用显式指定的历史验收 ZIP 和输出目录，重放全线中 1302 星的三张图，不访问玩家蓝图库。
func TestReplayPlanet1302Book(t *testing.T) {
	oldPack, output := os.Getenv("DSP_BOOK_1302_SOURCE"), os.Getenv("DSP_BOOK_1302_OUTPUT")
	if oldPack == "" || output == "" {
		t.Skip("未指定 1302 历史包和独立输出目录")
	}
	source, err := zip.OpenReader(oldPack)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	b := Bundle{Codes: make(map[int64]string)}
	var original string
	for _, entry := range source.File {
		switch entry.Name {
		case "蓝图/1302.txt":
			original = string(bookZipEntry(t, entry))
		case "规划记录.json":
			if err := json.Unmarshal(bookZipEntry(t, entry), &b); err != nil {
				t.Fatal(err)
			}
		}
	}
	if len(b.Report.Planets) != 48 || len(b.Report.Targets) != 1 || b.Report.Targets[0] != (quant.Target{ItemID: 6006, RatePerMinute: 2000}) {
		t.Fatalf("提供的历史包不是已验收的 48 星产线")
	}
	var site PlanetSheet
	for _, p := range b.Report.Planets {
		if p.ID == 1302 {
			site = p
		}
	}
	if site.Buildings != 67096 || site.FoundationCells != 70944 {
		t.Fatal("1302 施工摘要与原样本不符")
	}
	parts, err := SplitConstructionBlueprint(context.Background(), original, 1048576, 45000)
	if err != nil || len(parts) != 3 || parts[0].FoundationCells != 45000 || parts[1].FoundationCells != 25944 || parts[2].Buildings != 67096 {
		t.Fatalf("1302 分图不符：%v", err)
	}
	data, err := quant.LoadVanillaDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	identity, err := os.ReadFile(filepath.Join(filepath.Dir(oldPack), "save-identity.json"))
	if err != nil {
		t.Fatal(err)
	}
	var save struct{ Path string }
	if err := json.Unmarshal(identity, &save); err != nil || save.Path == "" {
		t.Fatalf("无法确认原存档：%v", err)
	}
	b.Report.SaveName = filepath.Base(save.Path)
	b.Report.ExportName = productionName(data, b.Report.Targets, b.Report.AchievedTargets)
	if b.Report.ExportName != "宇宙矩阵-2000每分钟" {
		t.Fatal(b.Report.ExportName)
	}
	b.Report.BlueprintCount = 0
	b.Report.Planets = nil
	b.Report.Warnings = append(b.Report.Warnings, "本包用于重放历史全线中重光 2 号星的分图和蓝图库导出，其他 47 颗星球见历史全线报告。")
	verifyBookExport(t, output, &b, site, parts)
}

func bookZipEntry(t *testing.T, file *zip.File) []byte {
	t.Helper()
	r, err := file.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// verifyBookExport 检查初次写库、同名再写、分享 ZIP 的独占写入以及目录和原版标题一致性。
func verifyBookExport(t *testing.T, output string, b *Bundle, site PlanetSheet, parts []ConstructionPart) {
	t.Helper()
	if err := b.addSheets(context.Background(), &site, parts, true); err != nil {
		t.Fatal(err)
	}
	b.Report.Planets = []PlanetSheet{site}
	library := filepath.Join(output, "蓝图库")
	if err := os.MkdirAll(library, 0700); err != nil {
		t.Fatal(err)
	}
	first, err := b.WriteBook(library)
	if err != nil || filepath.Base(first) != b.Report.ExportName {
		t.Fatalf("首次写入蓝图书：%s %v", first, err)
	}
	second, err := b.WriteBook(library)
	if err != nil || second != first+" (2)" {
		t.Fatalf("重导应保留第一本并编号：%s %v", second, err)
	}
	share := filepath.Join(output, b.Report.ExportName+b.ExportExtension())
	if filepath.Ext(share) != ".zip" {
		t.Fatal("多张图未使用 ZIP 分享")
	}
	if err := b.Export(share); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(share)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Export(share); err == nil {
		t.Fatal("覆盖了已有同名 ZIP")
	}
	after, err := os.ReadFile(share)
	if err != nil || sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("第二次导出改动了原 ZIP")
	}
	pack, err := zip.NewReader(bytes.NewReader(after), int64(len(after)))
	if err != nil {
		t.Fatal(err)
	}
	directories, err := os.ReadDir(library)
	if err != nil || len(directories) != 2 || !directories[0].IsDir() || !directories[1].IsDir() {
		t.Fatal("蓝图库内应仅有两本按顺序编号的书")
	}
	for _, book := range []string{first, second} {
		guide, err := os.ReadFile(filepath.Join(book, "施工说明.html"))
		if err != nil || !strings.Contains(string(guide), html.EscapeString(b.instructions())) {
			t.Fatalf("%s 缺少可离线查看的施工说明", book)
		}
		files, err := filepath.Glob(filepath.Join(book, "*.txt"))
		if err != nil || len(files) != len(parts) {
			t.Fatalf("%s 的原版可见施工图数与实际分图不一致", book)
		}
		for i, file := range files {
			s := site.Sheets[i]
			kind := "地基"
			if parts[i].Kind == "buildings" {
				kind = "建筑"
			}
			title := fmt.Sprintf("%s-%s-%d-%02d%s", b.Report.ExportName, site.Name, site.ID, i+1, kind)
			if s.Name != title || filepath.Base(file) != title+".txt" {
				t.Fatalf("蓝图库标题或施工顺序有误：%s", file)
			}
			raw, err := os.ReadFile(file)
			if err != nil || string(raw) != b.Codes[s.ID] {
				t.Fatalf("目录中的施工图与已核验文本不一致：%s", s.Name)
			}
			doc, err := blueprint.Decode(string(raw), true)
			if err != nil || doc.Name != title || !strings.Contains(doc.Description, b.Report.SaveName) || !strings.Contains(doc.Description, fmt.Sprintf("第 %d/%d 张", i+1, len(parts))) {
				t.Fatalf("原生蓝图标题或施工说明不符：%s %v", title, err)
			}
		}
	}
	var extras []string
	for i, entry := range pack.File {
		if i < len(parts) {
			s := site.Sheets[i]
			if entry.Name != s.File || s.File != b.Report.ExportName+"/"+s.Name+".txt" || string(bookZipEntry(t, entry)) != b.Codes[s.ID] {
				t.Fatalf("ZIP 的目录、顺序或蓝图内容不符：%s", entry.Name)
			}
		} else {
			extras = append(extras, entry.Name)
			if entry.Name == "施工说明.txt" && string(bookZipEntry(t, entry)) != b.instructions() {
				t.Fatal("分享 ZIP 的施工说明与蓝图书不一致")
			}
		}
	}
	slices.Sort(extras)
	if !slices.Equal(extras, []string{"施工说明.txt", "规划记录.json"}) {
		t.Fatalf("ZIP 附件位置或文件数不符：%v", extras)
	}
	t.Logf("%s 共 %d 张；同名书 %s；分享包 %s；重导前后 SHA256 %x", first, len(parts), second, share, sha256.Sum256(after))
}
