package industryworkspace

// 文件职责：把同一个已核验方案写成整星蓝图包。使用原版文本和 ZIP，提交前关闭临时文件。

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/quant"
)

// ReadChunk 按字节读取只含 ASCII 的蓝图编码；偏移越界不能返回空的成功预览。
func (b *Bundle) ReadChunk(planetID int64, offset int) (Chunk, error) {
	code, ok := b.Codes[planetID]
	if !ok || code == "" || offset < 0 || offset >= len(code) {
		return Chunk{}, fmt.Errorf("这颗星球没有可读的蓝图片段")
	}
	end := offset + min(len(code)-offset, 128*1024)
	return Chunk{Text: code[offset:end], NextOffset: end, Done: end == len(code)}, nil
}

// ExportExtension 按实际图纸数选择原版文本或分享包。
func (b *Bundle) ExportExtension() string {
	if b.Report.BlueprintCount == 1 {
		return ".txt"
	}
	return ".zip"
}

// Export 按实际张数写 TXT 或 ZIP。完整写入临时文件后才发布，保留已有同名文件。
func (b *Bundle) Export(target string) error {
	ext := b.ExportExtension()
	if !strings.EqualFold(filepath.Ext(target), ext) {
		return fmt.Errorf("该方案应保存为 %s 文件", ext)
	}
	sheets, err := b.constructionSheets()
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(target), ".dsbox-industry-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if ext == ".txt" {
		_, err = io.WriteString(temp, b.Codes[sheets[0].ID])
	} else {
		err = b.WriteZIP(temp)
	}
	closeErr := temp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Link(temp.Name(), target); err == nil {
		return nil
	} else if os.IsExist(err) {
		return fmt.Errorf("已保留同名文件，请选择新的文件名")
	}
	// 不支持硬链接的移动磁盘同样可用，独占创建保证同名文件不被覆盖。
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	reader, err := os.Open(temp.Name())
	if err == nil {
		_, err = io.Copy(out, reader)
		reader.Close()
	}
	closeErr = out.Close()
	if err != nil || closeErr != nil {
		os.Remove(target)
		if err != nil {
			return err
		}
		return closeErr
	}
	return nil
}

// constructionSheets 复核每张图和目录归属，供保存、分享与直接写入蓝图库共用。
func (b *Bundle) constructionSheets() ([]BlueprintSheet, error) {
	if !b.Report.Exportable || b.Report.ExportName == "" || filepath.Base(b.Report.ExportName) != b.Report.ExportName || !filepath.IsLocal(b.Report.ExportName) {
		return nil, fmt.Errorf("完整施工方案尚未生成")
	}
	var sheets []BlueprintSheet
	seen := map[int64]bool{}
	for _, site := range b.Report.Planets {
		if len(site.Sheets) == 0 {
			return nil, fmt.Errorf("%s 缺少施工蓝图", site.Name)
		}
		for _, sheet := range site.Sheets {
			if seen[sheet.ID] || sheet.Name == "" || !filepath.IsLocal(sheet.Name) || filepath.Base(sheet.Name) != sheet.Name || sheet.File != b.Report.ExportName+"/"+sheet.Name+".txt" || !strings.HasPrefix(b.Codes[sheet.ID], "BLUEPRINT:") {
				return nil, fmt.Errorf("%s 的施工清单与蓝图不一致", site.Name)
			}
			seen[sheet.ID] = true
			sheets = append(sheets, sheet)
		}
	}
	if len(sheets) == 0 || len(sheets) != len(b.Codes) || len(sheets) != b.Report.BlueprintCount {
		return nil, fmt.Errorf("施工图纸数量与已核验的方案不符")
	}
	return sheets, nil
}

// WriteBook 在玩家选定的蓝图库中发布可浏览的子目录，同名书自动编号。
func (b *Bundle) WriteBook(root string) (string, error) {
	sheets, err := b.constructionSheets()
	if err != nil {
		return "", err
	}
	if len(sheets) < 2 {
		return "", fmt.Errorf("单张施工图请直接保存 TXT")
	}
	stage, err := os.MkdirTemp(root, ".dsbox-industry-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	for _, s := range sheets {
		if err = os.WriteFile(filepath.Join(stage, s.Name+".txt"), []byte(b.Codes[s.ID]), 0600); err != nil {
			return "", err
		}
	}
	// 原版蓝图库只列目录和 TXT。施工配船与航线须另行设置，随书保留可离线查看的清单。
	guide := fmt.Sprintf(`<!doctype html>
<html lang="zh-CN">
<meta charset="utf-8">
<title>%s · 施工说明</title>
<pre style="white-space:pre-wrap;font:1rem/1.6 system-ui,sans-serif">%s</pre>
</html>`, html.EscapeString(b.Report.ExportName), html.EscapeString(b.instructions()))
	if err = os.WriteFile(filepath.Join(stage, "施工说明.html"), []byte(guide), 0600); err != nil {
		return "", err
	}
	for n := 1; ; n++ {
		name := b.Report.ExportName
		if n > 1 {
			name += fmt.Sprintf(" (%d)", n)
		}
		dest := filepath.Join(root, name)
		if _, statErr := os.Lstat(dest); statErr == nil {
			continue
		} else if !os.IsNotExist(statErr) {
			return "", statErr
		}
		if err = os.Rename(stage, dest); err == nil {
			return dest, nil
		}
		if _, statErr := os.Lstat(dest); statErr != nil {
			return "", err
		}
	}
}

// WriteZIP 将原版可浏览的子目录与包外层的说明分开，保留玩家复现方案所需的参数。
func (b *Bundle) WriteZIP(output io.Writer) error {
	sheets, err := b.constructionSheets()
	if err != nil {
		return err
	}
	writer := zip.NewWriter(output)
	for _, sheet := range sheets {
		entry, err := writer.CreateHeader(&zip.FileHeader{Name: sheet.File, Method: zip.Store})
		if err != nil {
			return err
		}
		if _, err = io.WriteString(entry, b.Codes[sheet.ID]); err != nil {
			return err
		}
	}
	record, err := writer.Create("规划记录.json")
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(record)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(struct {
		Request Request `json:"request"`
		Report  Report  `json:"report"`
		Plan    any     `json:"plan"`
	}{b.Request, b.Report, b.Plan}); err != nil {
		return err
	}
	guide, err := writer.Create("施工说明.txt")
	if err != nil {
		return err
	}
	if _, err := io.WriteString(guide, b.instructions()); err != nil {
		return err
	}
	return writer.Close()
}

func (b *Bundle) instructions() string {
	var out strings.Builder
	fmt.Fprintf(&out, "跨星球产业线施工说明\n游戏版本 %s　存档时刻 %d tick　规划中心 %d\n", b.Report.GameVersion, b.Report.Tick, b.Report.CenterPlanetID)
	fmt.Fprintf(&out, "在游戏蓝图库打开“%s”。若收到分享 ZIP，先解压并把同名文件夹放入蓝图库。每颗星按编号从小到大在同一锚点粘贴。存档 %s。\n", b.Report.ExportName, b.Report.SaveName)
	out.WriteString("请按本包所选存档的星球、锚点和朝向粘贴。先接入列出的外供、电网和运输船，再检验稳态净交付量。打开较旧存档或改变建筑、矿点后，请重新规划。\n")
	if slices.ContainsFunc(b.Report.External, func(e industryplan.External) bool { return e.Kind == "charging-peak" }) {
		out.WriteString("逐星充电上限取自施工图中每座物流塔的设定。开工时先接通电网，再分批为物流塔充电、备货和装入机队；运行后核对实际发运量和平均用电。\n")
	}
	if b.Report.RemoteGroup > 0 {
		fmt.Fprintf(&out, "星际物流使用存档中空闲的站组 %d，远程塔已设置为指定航线。请在游戏中按运输清单设置星球间航线。同一货物发往多地时，逐个核对收货星和每分钟到货量。\n", b.Report.RemoteGroup)
	}
	if len(b.Report.Routes) > 0 {
		out.WriteString("发货塔按满载发船。低流量货物从空仓启动时需先攒够一船，可预存一船启动物资。\n")
	}
	data, _ := quant.LoadVanillaDataset("Vanilla")
	item := func(id int) string {
		if data != nil {
			if value, ok := data.ByID[id]; ok {
				return value.Name
			}
		}
		return fmt.Sprintf("物品 %d", id)
	}
	roles := map[string]string{
		"import": "星际输入", "export": "星际输出", "delivery": "净交付", "storage": "仓储",
		"external": "外部供应", "external-export": "外部接收", "power-fuel": "发电燃料", "mining": "采集",
		"supply": "本地供应", "demand": "本地需求",
	}
	names := make(map[int64]string, len(b.Report.Planets))
	for _, site := range b.Report.Planets {
		names[site.ID] = fmt.Sprintf("%s（%d）", site.Name, site.ID)
	}
	planetName := func(id int64) string {
		if name := names[id]; name != "" {
			return name
		}
		return fmt.Sprintf("星球 %d", id)
	}
	for _, target := range b.Report.Targets {
		fmt.Fprintf(&out, "交付目标　%s　%s 件/分钟\n", item(target.ItemID), constructionRate(target.RatePerMinute))
	}
	for _, site := range b.Report.Planets {
		for _, port := range site.Ports {
			if port.Kind == "delivery" {
				fmt.Fprintf(&out, "实际交付　%s / %s　%s %s 件/分钟\n", site.Star, site.Name, item(port.ItemID), constructionRate(port.RatePerMinute))
			}
			if (port.Kind == "storage" || port.Kind == "external-export" && port.PeerPlanetID == 0) && port.RatePerMinute > 0 {
				fmt.Fprintf(&out, "稳产需要在%s持续消纳%s %.3f 件/分钟。", planetName(site.ID), item(port.ItemID), port.RatePerMinute)
				if port.StorageItems > 0 {
					fmt.Fprintf(&out, "按规划容量，空仓约 %.1f 分钟装满，之后相关产线会堵塞。", float64(port.StorageItems)/port.RatePerMinute)
				}
				out.WriteByte('\n')
			}
		}
	}
	for _, p := range b.Report.Planets {
		fmt.Fprintf(&out, "\n%s / %s（%d）　%s\n经度 %.5f°　纬度 %.5f°　朝向 %.2f°\n建筑 %d　地基 %d 格　物流塔 %d　运输船 %d　生产设施用电约 %.3f MW\n", p.Star, p.Name, p.ID, p.File, p.Longitude, p.Latitude, p.Direction, p.Buildings, p.FoundationCells, p.Stations, p.Vessels, p.PowerMW)
		if p.FoundationCells > 0 {
			if p.SoilEstimateKnown {
				fmt.Fprintf(&out, "  地基沙土需求区间　%d 至 %d\n", p.SoilLowerBound, p.SoilUpperBound)
			} else {
				out.WriteString("  沙土需求待地形核实\n")
			}
		}
		for i, sheet := range p.Sheets {
			fmt.Fprintf(&out, "  %d. %s　%d 座建筑、%d 格地基\n", i+1, sheet.Name, sheet.Buildings, sheet.FoundationCells)
		}
		if len(p.Fleet) > 0 {
			out.WriteString("  新塔已关闭自动补充运输船。只给下列发货塔装入所列数量，其余新塔留空，再按运输线路设置去向。同塔各货物共用这些船。\n")
		}
		if p.OutboundVesselMW > 0 {
			fmt.Fprintf(&out, "  本星发货按满载、回程空载估算，运输船持续用电 %.3f MW，已纳入本星的供电核验。\n", p.OutboundVesselMW)
		}
		for _, station := range p.Fleet {
			cargo := make([]string, 0, len(station.Cargo))
			for _, load := range station.Cargo {
				cargo = append(cargo, item(load.ItemID))
			}
			fmt.Fprintf(&out, "    经度 %.5f°，纬度 %.5f°　装入 %d 艘　远程供应 %s\n", station.Longitude, station.Latitude, station.Vessels, strings.Join(cargo, "、"))
		}
		for _, material := range p.Materials {
			fmt.Fprintf(&out, "  建造物资　%s × %d\n", item(material.ItemID), material.Count)
		}
		if len(p.ExistingSlots) > 0 {
			out.WriteString("  旧塔的本地运输机或存档中的天体航线可能与新塔配对。开工前请核对以下货槽和对应航线，按需调整或改选星球。\n")
		}
		for _, slot := range p.ExistingSlots {
			role, network := "供应", "本地"
			if slot.Logic == 2 {
				role = "需求"
			}
			if slot.Remote {
				network = "已有天体航线"
			}
			fmt.Fprintf(&out, "    星球 %d　物流站 %d　第 %d 槽　%s　%s%s\n", slot.PlanetID, slot.StationID, slot.Slot, item(slot.ItemID), network, role)
		}
		if p.Power != nil {
			fmt.Fprintf(&out, "  实装持续负荷 %.3f MW，本图持续供电能力 %.3f MW\n", p.Power.RequiredMW, p.Power.PowerMW)
			if p.Power.ExternalGridMW > 0 {
				fmt.Fprintf(&out, "  接入的已有电网可用余量 %.3f MW\n", p.Power.ExternalGridMW)
			}
			sources := p.Power.Sources
			if len(sources) == 0 && p.Power.FacilityID > 0 {
				sources = []industryplan.PowerSource{{FacilityID: p.Power.FacilityID, Count: p.Power.Count, PowerMW: p.Power.PowerMW, FuelItemID: p.Power.FuelItemID, FuelRatePerMinute: p.Power.FuelRatePerMinute}}
			}
			for _, source := range sources {
				fmt.Fprintf(&out, "    %s × %d", item(source.FacilityID), source.Count)
				switch source.Mode {
				case "charge":
					out.WriteString("（充电）")
				case "discharge":
					out.WriteString("（放电）")
				}
				if source.PowerMW > 0 {
					fmt.Fprintf(&out, "　%.3f MW", source.PowerMW)
				}
				if source.StoredMJ > 0 {
					fmt.Fprintf(&out, "　储能 %.3f MJ", source.StoredMJ)
				}
				if source.FuelItemID > 0 {
					fmt.Fprintf(&out, "　%s %.3f 件/分钟", item(source.FuelItemID), source.FuelRatePerMinute)
				}
				out.WriteByte('\n')
			}
		}
		if p.Photons != nil {
			fmt.Fprintf(&out, "  光子接收站 × %d，临界光子 %.3f 件/分钟，占用戴森球 %.3f MW。接收效率 %.1f%%，持续接收率 %.1f%%。\n", p.Photons.Count, p.Photons.RatePerMinute, p.Photons.SpherePowerMW, 100*p.Photons.Efficiency, 100*p.Photons.Uptime)
		}
		for _, m := range p.MinePlacements {
			fmt.Fprintf(&out, "  采集设施 #%d　%s　经度 %.5f° 纬度 %.5f°　覆盖 %d 个矿点　额定 %.3f 件/分钟\n", m.BuildingIndex, item(m.FacilityID), m.Longitude, m.Latitude, len(m.VeinIDs), m.RatePerMinute)
		}
		for _, mine := range p.Mines {
			if mine.CoveredItems != nil && mine.AvailableMinutes != nil {
				fmt.Fprintf(&out, "  %s　覆盖余量 %d 件，按目标 %.4g 件/分及 100%% 耗矿折合 %.1f 分钟。单个矿点枯竭后请复核采速。\n", item(mine.ItemID), *mine.CoveredItems, mine.RatePerMinute, *mine.AvailableMinutes)
			}
		}
		for _, port := range p.Ports {
			role := roles[port.Kind]
			if role == "" {
				role = port.Kind
			}
			fmt.Fprintf(&out, "  %s　%s　%.4g 件/分钟", role, item(port.ItemID), port.RatePerMinute)
			if port.PeerPlanetID != 0 {
				fmt.Fprintf(&out, "　对端 %d", port.PeerPlanetID)
			}
			if port.StorageItems > 0 && port.RatePerMinute > 0 {
				fmt.Fprintf(&out, "　容量 %d 件，约 %.1f 分钟装满", port.StorageItems, float64(port.StorageItems)/port.RatePerMinute)
			}
			out.WriteByte('\n')
		}
		for _, msg := range p.Warnings {
			fmt.Fprintf(&out, "  %s\n", msg)
		}
	}
	for _, r := range b.Report.Routes {
		fmt.Fprintf(&out, "\n运输 %s → %s　%s %.4g 件/分钟　预计往返 %.1f 秒　翘曲器 %.3g 个/分钟", planetName(r.FromPlanetID), planetName(r.ToPlanetID), item(r.ItemID), r.RatePerMinute, r.RoundTripSeconds, r.WarperRatePerMinute)
		if r.VesselPowerMW > 0 {
			fmt.Fprintf(&out, "　派船一次 %.3f MJ，发货端平均 %.3f MW", r.VesselEnergyMJ, r.VesselPowerMW)
		}
	}
	for _, e := range b.Report.External {
		if e.Kind == "battery-startup" {
			fmt.Fprintf(&out, "\n启动物资　%s　投入第一批空蓄电器。", planetName(e.PlanetID))
			continue
		}
		if e.Kind == "solar-conditions" {
			fmt.Fprintf(&out, "\n启动供电　%s　确认日照并充满跨夜蓄电器。", planetName(e.PlanetID))
			continue
		}
		if e.Kind == "local-drones" {
			fmt.Fprintf(&out, "\n本地物流 %d　为 %s 的本地供需配置物流运输机，按所选科技与实际运距配足。", e.PlanetID, item(e.ItemID))
			continue
		}
		if e.Kind == "grid-power" {
			fmt.Fprintf(&out, "\n外接电网 %d　%.3f MW", e.PlanetID, e.PowerMW)
			continue
		}
		if e.Kind == "charging-peak" {
			fmt.Fprintf(&out, "\n物流塔同时按设定充电的上限 %d　%.3f MW", e.PlanetID, e.PowerMW)
			continue
		}
		if e.Kind == "surplus-removal" {
			fmt.Fprintf(&out, "\n持续移出 %d　%s %.4g 件/分钟", e.PlanetID, item(e.ItemID), e.RatePerMinute)
			continue
		}
		if e.Kind == "transport" {
			fmt.Fprintf(&out, "\n玩家运输 %d → %d　%s %.4g 件/分钟", e.PlanetID, e.PeerPlanetID, item(e.ItemID), e.RatePerMinute)
			continue
		}
		fmt.Fprintf(&out, "\n外供 %d　%s %.4g 件/分钟", e.PlanetID, item(e.ItemID), e.RatePerMinute)
	}
	for _, msg := range b.Report.Warnings {
		fmt.Fprintf(&out, "\n%s", msg)
	}
	out.WriteByte('\n')
	return out.String()
}
