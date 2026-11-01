package build

// 文件职责：提供配方产物到机器端独立出货带的稳定映射。
// 使用：直连、配送、塔和总线接驳按物品读取；边界：兼容旧单带测试数据，不为新多产物行回退到混带。

// outputRows 职责：返回配方顺序的实际出货带；使用：统一遍历反向、标记及端点；边界：仅旧未填outputs的行保留单带兼容。
func (l *line) outputRows() [][]*placed {
	if len(l.outputs) > 0 {
		return l.outputs
	}
	if len(l.output) > 0 {
		return [][]*placed{l.output}
	}
	return nil
}

// outputForItem 职责：按物品精确取得本行出货带；使用：逐规范边绑定；边界：没有对应产物返回nil，旧未填outputs的合成行只保留原output兼容。
func (l *line) outputForItem(item int) []*placed {
	for index, result := range l.recipe.Results {
		if result != item {
			continue
		}
		if len(l.outputs) > 0 {
			if index < len(l.outputs) {
				return l.outputs[index]
			}
			return nil
		}
		return l.output
	}
	if len(l.recipe.Results) == 0 && len(l.outputs) == 0 {
		return l.output
	}
	return nil
}
