package build

// 文件职责：将原生纬带归为从赤道向两极的五档，并解析位置与旧版选区。
// 使用：生成器选区和容量边界；边界：第5档保留全部极区原生纬带，不伪装成单条窄带。

import (
	"fmt"
	"strings"
)

// densityLatitudeSelection 职责：区分档位、半球和南北位置；使用：选区解释；边界：南北边缘按地理方向定义，与所在半球无关。
type densityLatitudeSelection struct {
	tier, hemisphere int
	position         string
}

// label 职责：为规范键提供可读档位、半球和位置；使用：蓝图说明；边界：不表达未经验证的放置范围。
func (selection densityLatitudeSelection) label() string {
	region := "赤道环"
	if selection.hemisphere > 0 {
		region = "北半球"
	} else if selection.hemisphere < 0 {
		region = "南半球"
	}
	position := map[string]string{"center": "中心", "north": "北边缘", "south": "南边缘"}[selection.position]
	return fmt.Sprintf("第%d档 · %s · %s", selection.tier, region, position)
}

// densityLatitudeSelectionFor 职责：解析规范键及历史键；使用：统一选区语义；边界：解析成功不表示对应档位生成器已经接入。
func densityLatitudeSelectionFor(zone string) (densityLatitudeSelection, bool) {
	aliases := map[string]string{
		"equator": "ring1-center", "equator-north": "ring1-north", "equator-south": "ring1-south",
		"north-mid": "ring2-north-north", "south-mid": "ring2-south-south",
		"north-high": "ring3-north-north", "south-high": "ring3-south-south",
	}
	if canonical, ok := aliases[zone]; ok {
		zone = canonical
	}
	parts := strings.Split(zone, "-")
	if len(parts) < 2 || len(parts[0]) != 5 || !strings.HasPrefix(parts[0], "ring") || parts[0][4] < '1' || parts[0][4] > '5' {
		return densityLatitudeSelection{}, false
	}
	selection := densityLatitudeSelection{tier: int(parts[0][4] - '0')}
	if selection.tier == 1 {
		if len(parts) != 2 {
			return selection, false
		}
		selection.position = parts[1]
	} else {
		if len(parts) != 3 {
			return selection, false
		}
		switch parts[1] {
		case "north":
			selection.hemisphere = 1
		case "south":
			selection.hemisphere = -1
		default:
			return selection, false
		}
		selection.position = parts[2]
	}
	return selection, selection.position == "center" || selection.position == "north" || selection.position == "south"
}

// densityLatitudeTierBands 职责：把连续原生纬带划为五档；使用：区间预算和分档测试；边界：第1档跨赤道，第2至4档各一条原生纬带，第5档含余下全部极区纬带。
func densityLatitudeTierBands(tier, hemisphere, segment int) []surfacePackedBand {
	if segment != defaultSurfaceGridSegment || tier < 1 || tier > 5 || (tier > 1 && hemisphere != 1 && hemisphere != -1) {
		return nil
	}
	bands := surfaceLatitudeBands(segment)
	center := -1
	for i, band := range bands {
		if band.low <= 0 && band.high >= 0 {
			center = i
			break
		}
	}
	if center < 0 {
		return nil
	}
	result := []surfacePackedBand{}
	for i, band := range bands {
		if min(5, absInt(i-center)+1) != tier {
			continue
		}
		if tier > 1 && (hemisphere > 0 && band.low <= 0 || hemisphere < 0 && band.high >= 0) {
			continue
		}
		result = append(result, band)
	}
	return result
}

// denseLatitudeAssemblyLayout 职责：让第2档南北候选共用北侧原生格距预检，再显式映射回所选半球；使用：装配和补电阶段；边界：目标环宽、请求和最终放置半球不改变，南侧仍须完整复验。
func (b *builder) denseLatitudeAssemblyLayout(layout *SurfaceLayout) *SurfaceLayout {
	// 直连模式保留左侧入口，不能使用依赖半周旋转的对称装配。普通模式不参与密铺映射。
	if b.adapter == nil || b.adapter.density != densityDense || b.adapter.mode < 2 || b.adapter.mode > 4 || layout == nil || layout.Mode == "planar" {
		return layout
	}
	selection, ok := densityLatitudeSelectionFor(layout.LatitudeZone)
	if !ok || selection.tier != 2 || selection.hemisphere >= 0 {
		return layout
	}
	working := *layout
	working.LatitudeZone = "ring2-north-center"
	return &working
}
