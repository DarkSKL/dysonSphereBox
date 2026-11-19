package factory

// 文件职责：暴露球壳预览绑定；使用：主页导入或恢复作品时调用；边界：不读写数据库或玩家文件。

import "DysonSphereBox/helper/sphereblueprint"

// PreviewSphereBlueprint 职责：校验并解析一份 DYBP 设计；使用：主页预览和恢复；边界：返回解析错误，保存由主页负责。
func (s *Service) PreviewSphereBlueprint(code string) (*sphereblueprint.Blueprint, error) {
	return sphereblueprint.Parse(code)
}
