# 客户端多人联机管理兼容报告

日期：2026-10-15

## 已实现

- `session.open` 请求声明 `managementEvents=true`；服务端 `session.policy`（2002/3002）按 revision 丢弃旧事件，并允许同一 revision 的有效内容变化刷新策略。
- 管理关房（20225/20226）、管理员移出（20235）和管理员断会话（20106）均保留登录与大厅控制链路，释放房间、监听端口、P2P 游戏流、白名单和重试任务，并在前端显示服务端 `message`。
- 维护终止（20105）和暂停准入（20010）阻止自动重连/新建加入；策略消息进入连接提示，创建/加入控件按有效策略禁用，人数校验使用 `min(room.maxMembers, policy.maxMembers)`。
- 中继策略采用整体替换：停止旧预约 worker，撤回 Circuit 地址、peerstore 候选与受限连接；重新启用时按新 relays 预约。直接连接保留；中继游戏连接被策略切换中断时显示重新连接提示。
- Wails DTO/bindings、Vue 类型、联机状态 composable 和页面提示已更新；未读取客户端 YAML 作为管理策略来源。

## 验证

- `go test ./service/multiplayer ./helper/multiplayer/... ./apiDefinition/multiplayer/... -count=1`：通过。
- `go test ./...`：通过。
- `go build ./...`：通过。
- `frontend/npm run build`（在 `frontend` 目录）：通过，包含 `vue-tsc --noEmit` 和 Vite build；仅有 Sass legacy API 与 chunk size 警告。
- `service/multiplayer/management_test.go`：使用本机真实 TCP/UDP socket 与 libp2p host，验证 20225/20226/20235 清理和 20105/20106 不重连。
- `helper/multiplayer/p2p/relay_policy_test.go`：使用本机原生 Circuit Relay v2，验证 relay off 撤回旧电路/预约/peerstore，relay on 使用新 PeerID，并保留直连。
- Orca 内置页面快照检查 `#/community/multiplayer` 已加载，Wails `go` 与 runtime bridge 可用，管理提示控件存在。

## 跨仓库契约核对与风险

- 服务端当前实现与共享契约一致：policy 是 DB 唯一来源，`managementEvents` 用于旧客户端门禁，管理员事件携带 `message`，同 DB revision 的 effective 状态也可能变化。
- 同 revision 的客户端策略事件按完整 payload 判断重复；旧 revision 丢弃。WS 事件队列有序，因此不额外引入 effective generation。
- 服务端允许 `maxMembers` 最高 64，客户端 libp2p 资源硬上界已提升到 64；实际创建/加入仍受服务端策略和房间快照共同限制。
- `go test -race` 在当前 Windows Go 环境因 `CGO_ENABLED=0` 无法运行；未声称 race 验证通过。
- 未执行公网 NAT、Windows 防火墙和 WebView 原生传输验证；本地测试覆盖真实 libp2p/TCP/UDP 生命周期，不等同公网联调。
- `git diff --check` 仅报告 Wails 生成的 `frontend/wailsjs/go/models.ts` 空行尾随空白；该文件由 `wails generate module` 生成，未手工清理。
