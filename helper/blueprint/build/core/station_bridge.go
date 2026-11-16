package build

// 文件职责：为拥挤入口寻找有界高架接驳；使用：平面物流塔布线失败后；边界：逐格避免已有带及同组路线，不把高架自检冒充游戏碰撞验收。

import (
	"math"
)

// beltCell 职责：把微移带点归一到占用格；使用：同高度避碰；边界：保留层高，不利用 0.002 微移绕过真正重叠。
func beltCell(v vector) vector { return vector{math.Round(v.x), math.Round(v.y), math.Round(v.z)} }

// stationApproach 职责：给出端口外侧的接入方向；使用：入口预留与高架落点共用；边界：3/4 号端口避开西侧相邻内部带。
func stationApproach(slot int) vector {
	switch {
	case slot <= 2:
		return vector{0, 1, 0}
	case slot == 3 || slot == 4:
		return vector{1, 0, 0}
	case slot == 5:
		return vector{-1, 0, 0}
	case slot <= 8:
		return vector{0, -1, 0}
	default:
		return vector{1, 0, 0}
	}
}
