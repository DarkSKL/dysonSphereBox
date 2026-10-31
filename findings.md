# 实施发现

- 工作区起始干净，基线提交 `9d64378` 已包含此前暂停时的实现；旧文档所述“大量未提交文件”已过时。
- `当前实现进度.md` 指出的主要缺口：配送 mode 2/5 路线交叉、mode 1 大型球面跨 Area、mode 6 真实高流量拓扑及游戏运行证据。
- `helper/quant/testdata/acceptance-fixtures.json` 已有五个真实量化案例；正式 Acceptance 测试位于 `transport_test.go`，初次检索被合并输出截断，现已确认。
- `Generate` 已统一 FlowGraph、容量和最终规则主链；现有代码测试不能替代全模式真实 fixture 验证。
- 宿主具有 Go；Wails CLI 不在 PATH。读取/编译不依赖本地 Windows 游戏副本。
- 首轮 `go test ./helper/blueprint/build ./helper/quant ./helper/transport -count=1`：流图/运输包通过；build 包 31.3 秒，配送 10 项均出现交叉，另 mode6 预期容量失败先触发带段长度错误。其余正式集成用例通过。
- 当前宿主 macOS 26 ARM64，无原有 frontend/node_modules/dist；已执行 npm install 准备实际构建。
- 真实五组 mode1 × Mk.I/II/III 共 15 项矩阵通过独立实体图/机器/边界守恒检查；综合样本 lane 数 303/174/151，机器始终 765 台。
- 综合 mode3/4 × 三等级暴露 Mk.I 多产物过滤端点被相邻产线带封锁；其它四项通过。主工程师继续修复该新增缺口。
- 前端 npm build 和画板脚本 6/6 通过。原生全仓构建因既有 Windows-only 导入在 macOS 失败；Windows amd64、CGO=0 交叉构建通过。交叉 go vet 只发现进度文档已有 JSON tag 和 unsafe.Pointer 告警。
- 本地 Steam 库没有 Dyson Sphere Program，进度文档的 Windows D 盘隔离副本不在此宿主；未运行游戏内粘贴/连续产量验收。
- Mk.I塔拥堵根因包含两部分：多产物侧支路侵占同组下一短行入口；拆塔组后电杆仍按全局奇偶/末行编号复用而欠覆盖。已加入同组端点占位、逐行多产物塔组及组内供电编号，正式五fixture×mode3/4与综合三等级共16项全部通过（70.4秒）。
- 流图/运输/factory服务的 race 回归通过；打乱真实产线数组后最终建筑与空间指标相同；六模式预先取消均快速返回空结果。
- 当前未修改任何对外DTO/方法签名，无需重新生成Wails绑定。
- 真实喷涂兼容检查暴露并修复：validateProductionCapacity错误拒绝合法增产剂额外净输入；底侧逻辑输入反转使供剂干线首尾倒置并panic；供剂带未登记局部占位；塔/配送未绑定附加供剂端。现增产剂和普通物料共用规范边，旧可选网络不重复连接已统一供给的入口。
- 真喷涂recipe92、III剂4点、加速模式1、NeedCoaterNet=true：mode1–5通过增强逐边/机器/边界/addon链断言；mode6喷涂和3层研究站探针通过，等待正式整体回归。
- 新多Area安全切面机制已在真实铁块@2700及钢材@900生产图中成功；钢材内部铁块供给跨Area，新增正式回归核验真实内部边，而非仅合成路线。
- 总线已用真实共享环路、并联过滤和原料回收闭环替代固定三圈上限，五fixture MkIII与autoTropic成功；低带等级与最终整体矩阵仍在复核。
