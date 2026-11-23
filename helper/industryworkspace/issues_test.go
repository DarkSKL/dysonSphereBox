package industryworkspace

import (
	"strings"
	"testing"

	"DysonSphereBox/helper/quant"
)

// TestPlayerMessagesUsesCompleteIDs 验证同前缀的星球显示各自名称，并将含数字的内部诊断转成玩家提示。
func TestPlayerMessagesUsesCompleteIDs(t *testing.T) {
	sites := map[int64]Site{101: {Name: "起始星"}, 1011: {Name: "远方星"}}
	catalog := &quant.Dataset{ByID: map[int]quant.Item{1101: {Name: "铁块"}}}
	warnings, issues := playerMessages([]string{"星球 1011 向星球 101 运送物品 1101", "GEN_E_MODE1_FIT"}, sites, catalog)
	if len(warnings) != 1 || warnings[0] != "远方星 向起始星 运送铁块" || len(issues) != 1 || strings.Contains(issues[0].Cause, "GEN_") {
		t.Fatalf("玩家提示没有正确转换名称或内部代码：%v %v", warnings, issues)
	}
}

// TestReceivingSiteIssueNamesPlanetAndCargo 区分收货塔无落点与货物断带，保留玩家指定的交付地和物品。
func TestReceivingSiteIssueNamesPlanetAndCargo(t *testing.T) {
	sites := map[int64]Site{201: {Name: "麒麟座κ - 1号星"}}
	catalog := &quant.Dataset{ByID: map[int]quant.Item{6001: {Name: "电磁矩阵"}}}
	_, issues := playerMessages([]string{`第 1 次布局　星球 201：PLANET_PORT_SITE 星球 201 物品 6001 模块 ""：缺少星际收货塔落点，还需 1 座`}, sites, catalog)
	if len(issues) != 1 || issues[0].Cause != "麒麟座κ - 1号星 没有找到可新建电磁矩阵收货塔的位置" || !strings.Contains(issues[0].Action, "交付星") || strings.Contains(issues[0].Action, "分拣器") {
		t.Fatalf("收货塔落点错误的玩家说明不符：%+v", issues)
	}
}
