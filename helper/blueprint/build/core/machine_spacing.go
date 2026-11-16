package build

// 文件职责：集中声明按机器类型应用的横向安全余量；使用：机器行装配的间距计算；边界：只扩大精炼厂/化工厂基础间距，不改变用户附加配置、机器数或端口槽位。

// machineSpacing 职责：计算相邻地面机器的横向格距；使用：buildLine；边界：仅类型3/4增加一格，已归一化的用户附加配置仍直接累加，其它机器类型保持原几何；参数合法性由normalizeSetting负责。
func machineSpacing(typ int, conf geometry, configuredExtra int) float64 {
	safety := 0
	if typ == 3 || typ == 4 {
		safety = 1
	}
	return conf.Space + float64(configuredExtra+safety)
}
