package multiplayer

// 文件职责：定义 v2 原生联机能力。使用：session.open 与前端诊断。边界：缺失能力不授权中继，不包含账号凭证。

// Capabilities 描述原生数据面的服务端授权。
// 职责：区分协议支持、中继启动与游戏回退；使用：客户端仅按明确 true 开启；边界：旧响应默认全 false。
type Capabilities struct {
	// NativeDCUtR 职责：原生 DCUtR 契约；使用：中继协商准入；边界：false 禁止中继协商。
	NativeDCUtR bool `json:"nativeDcutr"`
	// RelayV2 职责：原生中继是否可用；使用：预约及拨号；边界：必须还有有效指定入口。
	RelayV2 bool `json:"relayV2"`
	// RelayFallback 职责：允许受限连接游戏流；使用：两侧转发准入；边界：不允许时仅协商。
	RelayFallback bool `json:"relayFallback"`
}
