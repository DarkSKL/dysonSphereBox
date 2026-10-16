package multiplayer

// 文件职责：定义 link.trace 的结构化诊断契约；使用：客户端上报、服务端验证和导出；边界：字段只表示真实观测，不携带凭证。

// TraceDiagnostic 职责：传递结构化网络证据；使用：link.trace 附件；边界：不含凭证，附件总大小最多 8192 字节。
type TraceDiagnostic struct {
	// Event 职责：原生事件名；使用：区分进度和结果；边界：非原生阶段可为空。
	Event string `json:"event,omitempty"`
	// Transport 职责：本次明确探测的传输；使用：tcp/quic；边界：空表示混合或未知。
	Transport string `json:"transport,omitempty"`
	// LocalPeerID 职责：上报节点身份；使用：对应会话；边界：不是账户凭证。
	LocalPeerID string `json:"localPeerId,omitempty"`
	// RemotePeerID 职责：对端或观测节点；使用：关联证据；边界：未知为空。
	RemotePeerID string `json:"remotePeerId,omitempty"`
	// AtMs 职责：客户端事件时间；使用：与服务端接收时间对照；边界：客户端时钟可能偏移。
	AtMs int64 `json:"atMs,omitempty"`
	// RTTMs 职责：原生协商往返毫秒；使用：StartHolePunch；边界：不是直连链路 RTT。
	RTTMs float64 `json:"rttMs,omitempty"`
	// ListenAddrs 职责：本机监听；使用：判断启用的传输；边界：不是公网可达性证明。
	ListenAddrs []string `json:"listenAddrs,omitempty"`
	// HostAddrs 职责：库当前宣告地址；使用：诊断候选来源；边界：不冒充原生实际打洞候选。
	HostAddrs []string `json:"hostAddrs,omitempty"`
	// RemoteAddrs 职责：原生 StartHolePunch 的实际远端候选；使用：判断尝试传输；边界：其他事件可能为空。
	RemoteAddrs []string `json:"remoteAddrs,omitempty"`
	// Targets 职责：本次拨号输入地址；使用：直连、中继和逐传输观测；边界：候选不等于全部实际尝试。
	Targets []string `json:"targets,omitempty"`
	// ObservedAddr 职责：identify 返回的本机地址；使用：逐传输比较；边界：可能是私网或回环，不代表公网或允许入站。
	ObservedAddr string `json:"observedAddr,omitempty"`
	// ConnectedAddr 职责：实际建立连接的地址；使用：核实传输和中继路径；边界：失败为空。
	ConnectedAddr string `json:"connectedAddr,omitempty"`
	// Errors 职责：逐地址错误；使用：拨号失败诊断；边界：每条错误有长度限制。
	Errors []TraceAddrError `json:"errors,omitempty"`
	// Truncated 职责：附件裁剪标志；使用：导出完整性提示；边界：不能认为未展示的地址未尝试。
	Truncated bool `json:"truncated,omitempty"`
}

// TraceAddrError 职责：保留具体地址与错误；使用：TraceDiagnostic；边界：地址最多256字节、错误最多512字节。
type TraceAddrError struct {
	// Addr 职责：原始 multiaddr；使用：识别 TCP/QUIC；边界：不做可达性推断。
	Addr string `json:"addr"`
	// Error 职责：库返回错误；使用：排障；边界：超长裁剪。
	Error string `json:"error"`
}
