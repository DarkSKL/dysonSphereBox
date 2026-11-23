package industryworkspace

// 文件职责：把内部诊断转换为玩家可操作说明；使用：报告和任务失败；边界：原始错误仅进入操作日志。

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"DysonSphereBox/helper/quant"
)

// Issue 职责：说明阻碍与下一步；使用：结果面板；边界：不暴露求解器代码和实体编号。
type Issue struct {
	Cause  string `json:"cause"`
	Action string `json:"action"`
}

var diagnosticCode = regexp.MustCompile(`\b(?:GEN_|PLANET_|Q_|BP_)[A-Z0-9_]+\b`)
var technicalDetail = regexp.MustCompile(`(?:设备|原型|星球|物品)\s+\d+|\b(?:external-input|external-output|power-fuel|resource-shortfall)\b`)
var namedResource = regexp.MustCompile(`(?:星球|物品)\s+\d+`)
var receivingSite = regexp.MustCompile(`\bPLANET_PORT_SITE\s+星球\s+(\d+)\s+物品\s+(\d+)`)

// PlayerIssue 职责：映射已知故障类别；使用：同一错误的 UI 出口；边界：未知内部错误要求查看日志，不伪造原因。
func PlayerIssue(message string) Issue {
	if _, cause, ok := strings.Cut(message, "Q_E_RECIPE_LOCKED: "); ok {
		return Issue{cause, "解锁该配方后重新读取存档，或调整配方和外部供料后重新规划"}
	}
	switch {
	case strings.Contains(message, "POWER_GRID"):
		return Issue{"部分设施之间没有找到可用的供电连接", "可换一颗建设星球，或指定更集中的电源后重新规划"}
	case strings.Contains(message, "GEOTHERMAL"):
		return Issue{"地热落点缺少已确认的熔岩或遗迹条件", "选择有熔岩的星球，或调整这颗星球的电源"}
	case strings.Contains(message, "POWER_SITE"), strings.Contains(message, "PLANET_POWER") && strings.Contains(message, "空地"):
		return Issue{"当前星球放不下所需电源及其物流", "选择功率更高的电厂，或更换建设星球"}
	case strings.Contains(message, "POWER"):
		return Issue{"实际建设的电源或燃料输送不足", "调整电源、燃料或分拣器等级后重新规划"}
	case strings.Contains(message, "PLANET_PORT_SITE"):
		return Issue{"没有找到可新建星际收货塔的位置", "可改选有空地的交付星，或在原星避开建筑和矿点留出满足塔间距的空位。水面需先铺设地基，保存后重新规划"}
	case strings.Contains(message, "PLANET_LOCAL_FLEET"):
		cause := "本地物流塔的无人机机位不足以完成需要的配送"
		if start := strings.Index(message, "即使全部满载并走最近塔对"); start >= 0 {
			cause = cause + "。" + strings.Split(message[start:], "，需分散工序")[0]
		}
		return Issue{cause, "分散制造工序或增加接驳塔，重新规划后再施工"}
	case strings.Contains(message, "PLANET_STATION_CHARGE"):
		cause := "当前派运方案超出单塔能核验的持续充电能力"
		if first := strings.Index(message, "号塔（经度 "); first >= 0 {
			if end := strings.Index(message[first:], "。"); end >= 0 {
				cause = "物流塔" + message[first+len("号塔"):first+end]
			}
		}
		return Issue{cause, "分流到其他发货塔，或调整制造星球或目标产量后重新规划"}
	case strings.Contains(message, "PORT_UNVERIFIED"), strings.Contains(message, "PLANET_PORT"), strings.Contains(message, "PLANET_LINK"):
		return Issue{"部分物料没有形成可验证的运输连接", "提高传送带或分拣器等级，或调整建设星球后重试"}
	case strings.Contains(message, "GLOBAL_CANDIDATE_BUDGET"):
		return Issue{"本轮布局搜索已暂停", "继续计算会保留已经找到的布局"}
	case strings.Contains(message, "FACTORY"), strings.Contains(message, "GEOMETRY"), strings.Contains(message, "COLLISION"), strings.Contains(message, "SURFACE"):
		return Issue{"当前分工尚未找到能完整落位的制造布局", "可继续尝试其他建设星球，或调整生产设备与运输等级"}
	case strings.Contains(message, "MINING_SITE"):
		return Issue{"部分矿区没有足够的矿机与出矿带落点", "换用其他矿源或调整矿机设置"}
	case strings.Contains(message, "TERRAIN"), strings.Contains(message, "SNAPSHOT"):
		return Issue{"存档中的地表资料不足以核验施工位置", "请重新选择完整存档或更换建设星球"}
	}
	if diagnosticCode.MatchString(message) || technicalDetail.MatchString(message) {
		return Issue{"这组条件尚未通过规划核验", "调整目标或建设条件后重试；详细原因已记录在操作日志"}
	}
	return Issue{Cause: message}
}

func playerMessages(messages []string, sites map[int64]Site, dataset *quant.Dataset) ([]string, []Issue) {
	var clean []string
	var issues []Issue
	seen := map[string]bool{}
	for _, raw := range messages {
		receiver := receivingSite.FindStringSubmatch(raw)
		raw = namedResource.ReplaceAllStringFunc(raw, func(match string) string {
			fields := strings.Fields(match)
			id, err := strconv.ParseInt(fields[1], 10, 64)
			if err == nil {
				if fields[0] == "星球" {
					if site, ok := sites[id]; ok {
						return site.Name
					}
				} else if item, ok := dataset.ByID[int(id)]; ok {
					return item.Name
				}
			}
			return match
		})
		issue := PlayerIssue(raw)
		if len(receiver) == 3 {
			planetID, _ := strconv.ParseInt(receiver[1], 10, 64)
			itemID, _ := strconv.Atoi(receiver[2])
			if site, ok := sites[planetID]; ok && dataset != nil {
				if item, ok := dataset.ByID[itemID]; ok {
					issue.Cause = fmt.Sprintf("%s 没有找到可新建%s收货塔的位置", site.Name, item.Name)
				}
			}
		}
		if issue.Cause == "" || seen[issue.Cause] {
			continue
		}
		seen[issue.Cause] = true
		if issue.Action != "" {
			issues = append(issues, issue)
		} else {
			clean = append(clean, issue.Cause)
		}
	}
	return clean, issues
}
