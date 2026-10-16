# 联机诊断客户端审查与验证

日期：2026-10-13（Asia/Shanghai）。本次只接管客户端诊断半成品，不修改 Server、共享 docs、用户 UI、截图、数据库或凭证，不 commit/push。

## 交付结果

`link.trace`（1303）新增可选 `result` 和 typed `diagnostic`，保留旧 `ok`。原生 DCUtR 的 Start/Attempt 按进度上报，End/DirectDial 按真实结果上报；直连、中继拨号和逐节点观测都携带原始地址与逐地址错误。入房前观测仍允许空 `roomId`，由 Server 的控制链路 SessionID/PeerID 关联房间导出背景，不在客户端伪造会话归属。

逐节点观测在会话既有 libp2p host 上进行，每个有效观测 peer 返回 TCP、QUIC 两条主结果。没有新建探测 host，没有随机换源端口，没有自定义打洞或 identify 协议，没有改变游戏中继回退和游戏白名单。

## 双端契约

| 字段 | 含义和边界 |
| --- | --- |
| `ok` | 原有 bool，兼容旧上报点；缺失 `result` 时 Server 继续按它处理 |
| `result` | 可选 `progress` / `success` / `failure`；`progress` 的 `ok=false` 不算失败 |
| `diagnostic.event` | 原生枚举 `DirectDial`、`ProtocolError`、`StartHolePunch`、`EndHolePunch`、`HolePunchAttempt`；非原生阶段为空 |
| `transport` | 明确独立观测时为 `tcp` / `quic`；普通混合拨号及原生 DCUtR 不凭候选猜测传输 |
| `localPeerId` / `remotePeerId` | 实际 libp2p 身份；Server 校验 localPeerId 与真实会话相符 |
| `atMs` | 原生事件 UNIX 纳秒转毫秒；观测为本次开始时间，普通拨号为上报时刻；不是服务端接收时间 |
| `rttMs` | StartHolePunch 的原生协商 RTT，以浮点毫秒表示，保留亚毫秒精度 |
| `listenAddrs` | 当前 host 的真实监听地址；不是公网可达性证明 |
| `hostAddrs` | 投影时库当前宣告地址；不是完整本地 native HolePunchAddrs，也不是原生本地实际候选 |
| `remoteAddrs` | 只从原生 StartHolePunch.RemoteAddrs 复制，这是该事件的实际远端候选 |
| `targets` | 拨号输入及 peerstore 候选，失败时合并库类型化 DialError 地址；候选不等于全部实际尝试 |
| `observedAddr` | 本次具体连接的原生 identify 返回地址；允许私网/回环，不证明公网入站可达 |
| `connectedAddr` | 实际建立连接的远端 multiaddr，用来核实传输和路径 |
| `errors[{addr,error}]` | 已收集的真实逐地址错误；无错误条目不证明该候选未尝试、成功或 UDP 未尝试 |
| `truncated` | 附件发生任何裁剪时为 true；不可把未展示部分解释为没有发生 |

预算针对 **diagnostic 附件的 JSON**：最多 8192 字节；四组地址数组及 errors 各最多 32 条；地址 256 字节、错误 512 字节；event 64、transport 16、PeerID 128 字节。异步 Tracer 在入队前复制结构及所有数组，并按 UTF-8 字符边界裁剪；JSON 转义后的真实长度也纳入预算，必要时继续移除尾项并标记 truncated。顶层 addr/error/detail 沿用 256/512/256 字节限制。

`peer.addrs.net` 白名单已通知 Server：`upnp:bool`、`ipv6:bool`、`natConsistent:bool|null`、`firewallRule:bool`。NAT 一致性只比较 QUIC/UDP 观测映射，不能由 TCP 结果推断 UDP 状态，也不能由一致映射证明打洞一定成功。

## 观测实现与已修正问题

1. 修复半成品 `observed, details :=` 遮蔽外层变量导致成功观测没有返回、Collect 拿不到快照的问题；观测逻辑独立到 `helper/multiplayer/p2p/observe.go`。
2. 同 host 的观测轮次通过可取消通道串行化；不同观测 peer 并行，每个 peer 先 TCP 后 QUIC。按输入 peer 顺序、TCP/QUIC 顺序返回，完成先后不影响排序。
3. 订阅先于拨号，使用库 Network.DialPeer 得到具体连接；成功 identify 必须匹配 PeerID 和 Conn.ID，同时核对真实 local/remote 地址及 observed 地址的传输。不会把 TCP 连接复用误称为 QUIC 成功。
4. 只对该专用观测 peer ClosePeer/ClearAddrs/清退避；拒绝本机、游戏白名单和指定中继身份重叠。单传输无入口时明确“未下发，未尝试”，不是虚构网络失败。
5. 本机重复轮次确实暴露旧 identify 清理 TempAddrTTL 与新候选的竞态。修复采用独立的一分钟候选 TTL，并在该观测 peer 本轮临时过滤异传输地址，防止旧 identify 写回其他传输地址造成混拨；退出即解除。此过滤不修改 allowed/revoked 或 relay 策略。
6. QUIC 成功时保留该成功 QUIC 连接。TCP 成功而 QUIC 探测失败时，恢复 TCP 原生连接，并另发 `stage=observe`、detail 含“恢复TCP观测连接”的 trace；QUIC 原始失败仍保留，两条主探测结果不被恢复结果覆盖。恢复成功时 Collect 使用恢复后的真实 TCP 地址。
7. `addrLoop` 绑定会话取消，关闭会话会取消观测和地址请求；旧会话不能写入/上报新会话诊断。原生 DCUtR 对端关联复用含 pending/leaving 的既有 peer→user 映射。
8. 普通 ConnectReport 的已知候选失败也可保留于成功拨号附件；缺少逐地址证据时改成“候选缺少逐地址结果（未尝试或尚未完成）”，不再伪称“全部无响应”。

### Native observed 的必要限制

核对 libp2p v0.47.0 源码：Swarm.DialPeer 会复用现有直接连接，并无公开“强制另一传输且保持旧连接”的接口；swarm.addConn 是私有接口。原生 observed manager 在 Disconnected 时删除旧连接对应观测，因此 TCP→QUIC 重建会移除旧 TCP native observed。

保留在 diagnostics/Collect 的 TCP 历史快照不代表当前仍有 TCP native 连接。最终成功 QUIC 连接会保留；失败时恢复 TCP 是为了重新建立真实 native 观测，而不是发送伪造事件。若观测服务自己断开连接，库仍可能清除其 native observed；一次成功历史记录不保证当前候选仍有效。`hostAddrs` 未被冒充为完整 native 本地候选。

## 真实执行的验证

工作目录为 `develop/client`，PowerShell；module 声明 Go 1.24.11，执行工具为 `go version go1.25.10 windows/amd64`，libp2p v0.47.0。

```powershell
go build ./...
go vet ./helper/multiplayer/... ./service/multiplayer/... ./apiDefinition/multiplayer/...
go test ./helper/multiplayer/link ./helper/multiplayer/p2p ./service/multiplayer -run TestTemporary -v -count=1
go test ./helper/multiplayer/p2p -run 'TestTemporaryObserveDualTransport|TestTemporaryActiveCancelAndConcurrentRounds' -count=10
git diff --check
```

上述 build、vet、专项与连续 10 轮重复/并发检查通过。后续追加的附件边界/拨号投影检查单独重跑 link/service 后通过。初次重复观测曾失败于 peerstore 竞态，修正后上述重复检查通过；初次新增拨号投影测试错误选到 `/p2p-circuit` 监听入口，修正测试选择真实 UDP listener 后通过，这不是生产拨号故障。

| 验证 | 实际证据 |
| --- | --- |
| 两个真实 peer、每 peer 两传输 | 本机一次执行的源监听端口为 59300；两 peer 的 TCP observed 均 `/ip4/127.0.0.1/tcp/59300`，QUIC observed 均 `/ip4/127.0.0.1/udp/59300/quic-v1`；实际远端分别使用端口 59302、59303，connected 与 observed 传输匹配 |
| TCP/QUIC 保持原源端口 | 各观测以原 host/transport 拨号，四条 observed 均与其既有对应监听端口相等；没有另起测试专用生产探测 host |
| UDP 单侧失败 | 真实目标 `/ip4/127.0.0.1/udp/59305/quic-v1` 没有 UDP listener，库逐地址错误 `context deadline exceeded`；TCP 成功记录保留并恢复 |
| TCP 单侧失败 | 真实目标 `/ip4/127.0.0.1/tcp/59308` 没有 TCP listener，库错误 `connectex: No connection could be made because the target machine actively refused it.`；QUIC 独立成功 |
| TCP 恢复的 native 行为 | 非回环实际 LAN 连接 `/ip4/192.168.0.102/tcp/59310`；库原生 observedaddrs.Manager 订阅真实 host EventBus/Network 后 `Addrs(1)` 看到相同恢复地址，连接 LocalMultiaddr 也相同 |
| QUIC 成功保留连接 | 双节点专项断言每个 peer 最终保留一个实际 QUIC 连接 |
| 取消及并发 | 活动 QUIC 拨号取消一次实测 1.5391ms 返回；dial watch、临时传输过滤、轮次锁均释放；四个并发调用串行完成且传输未混淆；重复/并发连续 10 轮通过 |
| 实际 Tracer 有界附件 | 大数组/中文/JSON 转义压力的实际发送附件为 8056 字节、truncated=true；另一附件保留恰好 32 targets，中文地址/错误裁为 255/510 字节，均有效 UTF-8 |
| 数据复制 | Trace 后修改原 event、四组数组和 error 条目，不改变实际发出的附件 |
| 原生进度和时间 | 实际 Tracer 与控制器上报路径中 Start/Attempt 保留 progress，End/DirectDial success/failure 独立；Start RTT 1500µs→1.5ms，Timestamp 纳秒→UNIX毫秒，remoteAddrs 从原生事件复制 |
| 入房前及恢复 trace | 控制器测试证明 observe 允许空 roomId、QUIC 失败和 TCP 恢复分别发送、旧会话 trace 丢弃 |
| direct/relay 附件投影 | 真实 native TCP refused 错误经过 direct_dial、relay_dial 两个 trace 投影，targets/errors 与本机/对端 PeerID 完整保留；此项未模拟实际中继网络路径 |

native 恢复验证使用额外的**库原生 manager 实例**订阅真实 host，不是读取生产 host 的私有 manager，也没有伪造 identify 事件。这个验证证明恢复连接产生了库可采纳的真实 native 观测，不宣称获取了生产库所有私有候选。

本机 `CGO_ENABLED=0`，未运行 race detector。没有公网 NAT、跨公网 DCUtR 或真实中继网络环境，不能把上述回环/LAN 结果当成用户原公网故障已复现或已修复。

## 清理与范围

按用户要求，验证完成后删除本次三个临时专项测试，不保留临时测试代码、不删除历史 tracked tests：

- `helper/multiplayer/link/diagnostic_temporary_test.go`，验证源 SHA256 `20210D7C39E6454349C9FCC5F9CF99662AE68E87B02F45086EC1548E08C4C180`。
- `helper/multiplayer/p2p/diagnostic_temporary_test.go`，验证源 SHA256 `FEA72622548A88F7628D0071CC04ED95862362489065B4390DF03BDDF9B21F88`。
- `service/multiplayer/diagnostic_temporary_test.go`，验证源 SHA256 `9656A7EB57E07084B67A7250BB6587D8C6283C105AB13720B77FB8102B60C83E`。

专项命令是当时真实执行的记录；清理后仓库中没有 TestTemporary 可重跑。清理后再次执行 Go build、相关 vet 和 `git -c core.safecrlf=false diff --check` 均通过，git status 无临时测试残留或历史测试删除。

用户原有 `.gitignore`、create-dialog.vue、join-dialog.vue 改动未编辑。工作期间生成绑定出现时间/换行提示，属于外部 Wails 生成行为，未手改、未还原；本次没有 Wails 绑定方法签名变化。未修改 app.db、运行 DB、账号凭证、截图、server 或共享 docs。

`peer.addrs` 仍有原来 16 条上限；Collect 现有 relay 优先和 IPv4 `169.254.*` 链路本地地址可能占名额，未扩大本次改动去过滤。候选截断、观测地址非公网、单次 UDP 超时都不能作为“UDP 被封”结论。Tracer 仍有队列 64、每分钟 120 条、无重试、会话关闭丢弃未发送条目的既有边界，因此房间导出不能保证捕获每一条本地事件。
