package build

// 文件职责：为密铺接驳保存真实原生带口的有限连接豁免。
// 使用：分流器回收门及物流塔首末带；边界：不能豁免任意生产机器、未连接邻居或超出三跳的内部节点。

// denseNativeBeltPort 职责：列出已有真实带槽规则的建筑；使用：端口向外方向及连接豁免；边界：普通制造设备必须通过分拣器，不在此列。
func denseNativeBeltPort(item int16) bool {
	return item == 2020 || item == 2103 || item == 2104 || item == 2307
}

// linkedPortBuildings 职责：沿端点现有带链寻找至多三跳的原生带口；使用：新增前后缀许可；边界：仅按实际input/output指针及唯一已有前驱追溯，不按空间邻近猜测连接。
func (index *denseRoutePortIndex) linkedPortBuildings(endpoint *placed, source bool) map[*placed]int {
	result := map[*placed]int{}
	current := endpoint
	for hops := 1; hops <= 3 && current != nil; hops++ {
		next := current.output
		if source {
			next = current.input
			if next == nil {
				previous := index.previous[current]
				if len(previous) != 1 {
					break
				}
				next = previous[0]
			}
		}
		if next == nil {
			break
		}
		if !surfaceIsBelt(next.record.ItemID) {
			if denseNativeBeltPort(next.record.ItemID) {
				result[next] = hops
			}
			break
		}
		current = next
	}
	return result
}

// densePortalLinkPermissions 职责：把原端点到建筑的跳数与新增路径深度相加；使用：每条真实接驳边；边界：第一个不再有豁免的节点仍须完整检查其节点球。
func densePortalLinkPermissions(linked map[*placed]int, firstDepth, secondDepth int) map[*placed][2]bool {
	permissions := map[*placed][2]bool{}
	for owner, hops := range linked {
		allowed := [2]bool{hops+firstDepth <= 3, hops+secondDepth <= 3}
		if allowed[0] || allowed[1] {
			permissions[owner] = allowed
		}
	}
	return permissions
}
