package build

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// nativeAssemblerUnit 职责：构造指定原生纬度的独立双机单元；使用：第4档端口试验；边界：不接入整厂、外部物流或供电生成。
func nativeAssemblerUnit(t *testing.T, item, level, turn, hemisphere int, position string) (*builder, *surfaceEncoding, []blueprint.Building) {
	t.Helper()
	ctx := context.WithValue(context.Background(), denseCollisionKey{}, true)
	plan, err := quant.Solve(ctx, quant.Request{Dataset: "Vanilla", Targets: []quant.Target{{ItemID: item, RatePerMinute: 120}}})
	if err != nil {
		t.Fatal(err)
	}
	disabled := false
	adapter, err := resolveModeAdapter(1, densityDense)
	if err != nil {
		t.Fatal(err)
	}
	belt := min(level, 3)
	prepared, err := prepareGeneration(ctx, Request{Plan: plan, Mode: 1, Density: densityDense, Setting: Setting{BeltLv: belt, InserterLv: level, NeedPower: &disabled}}, adapter)
	if err != nil {
		t.Fatal(err)
	}
	b := prepared.builder
	b.denseSpatial = true
	recipe := 50
	if item == 1202 {
		recipe = 6
	}
	for owner, p := range prepared.request.Plan.Productions {
		if p.RecipeID != recipe {
			continue
		}
		b.owner = owner
		r, err := datasetRecipeForProduction(prepared.dataset, p)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = b.buildReferenceLowLine(p, r, 2, 1, 0, true); err != nil {
			t.Fatal(err)
		}
	}
	if len(b.all) == 0 {
		t.Fatal("没有创建参考单元")
	}
	rotatePlacedQuarter(b.all, turn)
	low, high := bounds(b.all)
	shift(b.all, -low.x, -low.y, 0)
	w, h := int(math.Ceil(high.x-low.x))+1, int(math.Ceil(high.y-low.y))+1
	band := densityLatitudeTierBands(4, hemisphere, 200)[0]
	anchor := bandCenter(band) - (h-1)/2
	if position == "south" {
		anchor = band.low
	} else if position == "north" {
		anchor = band.high - h + 1
	}
	if h > 25 {
		t.Fatal("局部单元超出第4档环宽")
	}
	records, _, _, err := b.records()
	if err != nil {
		t.Fatal(err)
	}
	encoding := &surfaceEncoding{areas: []blueprint.Area{{Index: 0, ParentIndex: -1, AreaSegments: 100, Width: int16(w), Height: int16(h)}}, plan: &SurfacePlan{GridSegment: 200, PrimaryAreaIndex: 0, AnchorLatitudeGrid: anchor, LayoutWidth: w, LayoutHeight: h, Bands: []SurfaceBandPlan{{AreaIndex: 0, MinLatitudeGrid: anchor, MaxLatitudeGrid: anchor + h - 1, LongitudeSegments: 100, BuildingCount: len(records)}}}}
	return b, encoding, records
}

// TestNativeFourthTierAssemblerInputs 职责：验证两类双输入制造单元在第4档三位置、两半球及四向排布下的端口与取货；使用：整厂接入前局部验收；边界：不宣称第4档整厂可用。
func TestNativeFourthTierAssemblerInputs(t *testing.T) {
	ctx := context.WithValue(context.Background(), denseCollisionKey{}, true)
	for _, item := range []int{1301, 1202} {
		for _, level := range []int{1, 2, 3, 4} {
			for _, turn := range []int{0, 1, 2, 3} {
				for _, hemisphere := range []int{1, -1} {
					for _, position := range []string{"center", "south", "north"} {
						t.Run(fmt.Sprintf("item%d/lv%d/turn%d/hemisphere%d/%s", item, level, turn, hemisphere, position), func(t *testing.T) {
							b, encoding, records := nativeAssemblerUnit(t, item, level, turn, hemisphere, position)
							before := append([]blueprint.Building(nil), records...)
							if item == 1301 && level == 1 && turn == 0 && hemisphere == 1 && position == "south" {
								disabled := false
								if err := validateRelocatedSurface(ctx, Request{Setting: Setting{NeedPower: &disabled}}, encoding, records); err == nil || !strings.Contains(err.Error(), "端点偏斜") {
									t.Fatalf("没有重现原单元的原生边缘偏角：%v", err)
								}
							}
							aligned, err := b.alignNativeThirdInputPorts(ctx, encoding, records)
							if err != nil {
								t.Fatal(err)
							}
							if !reflect.DeepEqual(records, before) || len(aligned) != len(records) {
								t.Fatal("对齐改变输入记录或建筑数量")
							}
							for i, r := range aligned {
								if b.all[i].densityThirdInput {
									if r.Yaw != before[i].Yaw || r.Yaw2 != before[i].Yaw2 || r.LocalOffset.Z != before[i].LocalOffset.Z {
										t.Fatal("对齐改变臂朝向或高度")
									}
								} else if r.RecipeID > 0 || surfaceIsBelt(r.ItemID) {
									if !reflect.DeepEqual(r, before[i]) {
										t.Fatalf("对齐移动机器或带路：%d", i)
									}
								}
							}
							again, err := b.alignNativeThirdInputPorts(ctx, encoding, aligned)
							if err != nil || !reflect.DeepEqual(again, aligned) {
								t.Fatalf("再次对齐不是幂等操作：%v", err)
							}
							if level == 1 && turn == 0 {
								exportNativeUnit(t, item, hemisphere, position, encoding, aligned, before)
							}
						})
					}
				}
			}
		}
	}
}

// exportNativeUnit 职责：导出已校验的局部端口试样供图片或游戏单元复验；使用：显式测试导出；边界：没有外部供电和完整产业物流。
func exportNativeUnit(t *testing.T, item, hemisphere int, position string, encoding *surfaceEncoding, records, before []blueprint.Building) {
	t.Helper()
	directory := os.Getenv("DSBOX_BLUEPRINT_TEST_OUTPUT")
	if directory == "" {
		return
	}
	if !filepath.IsAbs(directory) {
		t.Fatal("导出目录必须为绝对路径")
	}
	if err := os.MkdirAll(directory, 0755); err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("native-unit-%d-%d-%s", item, hemisphere, position)
	document := blueprint.Document{EnvelopeVersion: 1, BinaryVersion: 1, Name: name, Description: fmt.Sprintf("第4档双机端口试样；光标纬度%.2f度；仅验证局部几何和取料接口，需要外接原料与供电，未包含整厂物流", float64(encoding.plan.AnchorLatitudeGrid)*.36), GameVersion: "0.10.35.29151", Areas: encoding.areas, Buildings: records}
	encoded, err := blueprint.Encode(document, 1)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := blueprint.Decode(encoded.Blueprint, true)
	if err != nil || !decoded.ChecksumValid {
		t.Fatalf("试样严格解码失败：%v", err)
	}
	disabled := false
	if err := validateRelocatedSurface(context.WithValue(context.Background(), denseCollisionKey{}, true), Request{Setting: Setting{NeedPower: &disabled}}, encoding, decoded.Buildings); err != nil {
		t.Fatalf("导出试样原生几何复验失败：%v", err)
	}
	if _, err := validateFinalPlanetRules(context.WithValue(context.Background(), denseCollisionKey{}, true), Request{Mode: 1, Density: densityDense, Setting: Setting{NeedPower: &disabled}}, encoding, decoded.Buildings); err != nil {
		t.Fatalf("导出试样连接槽或最终规则检查失败：%v", err)
	}
	if err := os.WriteFile(filepath.Join(directory, name+".txt"), []byte(encoded.Blueprint), 0644); err != nil {
		t.Fatal(err)
	}
	for _, sample := range []struct {
		suffix  string
		records []blueprint.Building
	}{{"after", decoded.Buildings}, {"before", before}} {
		document.Buildings = sample.records
		data, err := json.MarshalIndent(struct {
			Document    blueprint.Document `json:"document"`
			SurfacePlan *SurfacePlan       `json:"surfacePlan"`
		}{document, encoding.plan}, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, name+"-"+sample.suffix+".json"), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestNativePortAlignmentRollback 职责：保证端口或带路验证失败时不留下半次取货重绑；使用：局部适配事务回归；边界：不把失败结果作为可用单元。
func TestNativePortAlignmentRollback(t *testing.T) {
	b, encoding, records := nativeAssemblerUnit(t, 1301, 1, 0, 1, "south")
	// 故意破坏外部带头与下一段的接线距离，其余机器和参考臂保持正常。
	b.all[0].a.x += 6
	b.all[0].b.x += 6
	records[0].LocalOffset.X = float32(b.all[0].a.x)
	records[0].LocalOffset2.X = float32(b.all[0].b.x)
	snapshots := make([]placed, len(b.all))
	for i, p := range b.all {
		snapshots[i] = *p
	}
	before := append([]blueprint.Building(nil), records...)
	plan := encoding.plan
	checks := len(b.logistics.Checks)
	result, err := b.alignNativeThirdInputPorts(context.WithValue(context.Background(), denseCollisionKey{}, true), encoding, records)
	if err == nil || result != nil {
		t.Fatal("故障带路被接受为有效单元")
	}
	if encoding.plan != plan || len(b.logistics.Checks) != checks || !reflect.DeepEqual(records, before) {
		t.Fatal("失败操作污染了记录、验证状态或诊断")
	}
	for i, p := range b.all {
		if !reflect.DeepEqual(*p, snapshots[i]) {
			t.Fatalf("失败操作未还原实体%d", i)
		}
	}
}

// TestNativePortAlignmentRejectsStaleRecords 职责：拒绝复用已经过期的端点记录；使用：防止重复映射累积货物偏移；边界：只允许当前装配坐标。
func TestNativePortAlignmentRejectsStaleRecords(t *testing.T) {
	b, encoding, records := nativeAssemblerUnit(t, 1301, 1, 0, 1, "south")
	ctx := context.WithValue(context.Background(), denseCollisionKey{}, true)
	if _, err := b.alignNativeThirdInputPorts(ctx, encoding, records); err != nil {
		t.Fatal(err)
	}
	if _, err := b.alignNativeThirdInputPorts(ctx, encoding, records); err == nil {
		t.Fatal("重复使用旧坐标未被拒绝")
	}
}
