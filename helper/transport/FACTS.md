# 运输能力事实边界

`capacity.go` 是基础传送带容量的唯一来源：Mk.I / Mk.II / Mk.III 分别为 360 / 720 / 1800 件每分钟。这里不应用科技、货物堆叠或 Mod 倍率。

## 分拣器目录

`data/inserters-25955035.json` 由 `scripts/factory/extract-inserter-capacity.py` 从用户本机游戏安装目录只读提取，仅分发原始数值、资源路径和来源指纹，不分发游戏资源或反编译实现。

- Steam build：25955035；游戏版本：0.10.35.29151；Unity：2022.3.62f3c1。
- `Assembly-CSharp.dll` SHA-256：`ef314425dade51f529f184f07be136cd3c2660a6a81515603c7226fbe7ab1d8c`。
- `resources.assets` SHA-256：`8343173562b15b06a400d70cb1510a106d7654f96342857e5925ded5fd1900d1`。
- `InserterDesc` 的 `sttf` 为 1/3、1/6、1/12、0.05（资源中为 float32）；`PrefabDesc.ReadPrefab` 将其乘以 600000 后取整，得到 200000、100000、50000、30000 的单程计时单位。
- `PlanetFactory.CreateEntityLogicComponents` 将单程计时乘以蓝图分拣器参数中的跨距；当前 API 仅核对整数跨度 1、2、3，不推断其它参数或未知几何。
- `GameHistoryData.SetForNewGame` 的零科技旧分拣器批量为 1，集装臂输入批量为 2、输出堆叠为 1、双向为否；`FactorySystem.NewInserterComponent` 按等级使用这些值。资源的 `stackSize` 本身不能替代历史科技批量。
- 已核对 `InserterComponent.InternalUpdate` 和 `InternalUpdateNoAnim`：满电每 tick 增加 10000 计时；发送/返回均受单程计时约束。批量为 N、未堆叠且持续有货、接收不阻塞时，逐件抓取与放置分别增加 N-1 tick，所以周期为 `2 * (STT * distance / 10000 + N - 1)` tick。

### 零科技非堆叠额定上限（件/分钟）

| 物品 | 等级 | 一格 | 两格 | 三格 |
| --- | --- | ---: | ---: | ---: |
| 2011 | Mk.I | 90 | 45 | 30 |
| 2012 | Mk.II | 180 | 90 | 60 |
| 2013 | Mk.III | 360 | 180 | 120 |
| 2014 | 集装 | 900 | 514.285714… | 360 |

这些数值是已声明运行条件下的额定上限，不保证实际带上货物相位、端口等待、断料、缺电后的吞吐。生产输入中带上的未堆叠物品、制造机端口可用性、正确筛选和分拣器几何仍需分别验证。调用方不得用低于此上限的流量反推真实游戏一定达到目标，也不得把未知科技或输入堆叠视作无限能力。

量化 `Vanilla` 配方数据的声明版本仍为 0.10.34.28524；它与本事实目录的几何/运行规则版本不同。诊断必须同时展示两个版本，不得把它们误认为同一份游戏事实。

## 电力与物流塔数值

`data/infrastructure-25955035.json` 由 `scripts/factory/extract-infrastructure-facts.py` 只读提取，与分拣器目录拥有相同 DLL/资源指纹。脚本完整消费 `PowerDesc`、`StationDesc` 序列字段，未知布局拒绝输出；物流端口数来自 `SlotConfig.slotPoses` 引用数组，`PrefabDesc.ReadPrefab` 把该数组转换成 `portPoses`。

- 2201 电力感应塔：`node=true`、`coverRadius=10.5`、`connectDistance=22.5`。
- 2103 行星内物流运输站：`maxItemKinds=4`、`maxItemCount=5000`、`maxDroneCount=50`、`maxShipCount=0`、12 个物流带端口。注意固定当前构建为 4 槽，不能沿用历史版本的 3 槽资料。
- 2104 星际物流运输站：`maxItemKinds=5`、`maxItemCount=10000`、`maxDroneCount=100`、`maxShipCount=10`、12 个物流带端口。

这些是固定原型的基础数值，不包含科技扩容、额外科技物流槽或 Mod 修改，不说明运行时电网是否供电、无人机是否配齐或星际运输是否有效。

## 当前实现审计边界（非游戏验收）

- 取整机器的 `MachineCount` 只表示可放置台数；`quant.Solve` 的计划 `Inputs`/`Outputs` 和 FlowGraph 仍以目标速率为准，不把额定余量自动变成额外物料。生成器可拒绝“取整台数低于精确需求”的陈旧快照，但不能据此声称多余产出已被游戏内消纳。
- 单带容量、分拣器单臂吞吐和塔槽位/端口是独立瓶颈。通过带容量不代表分流器、合流器、机器端口、物流塔调度或分拣器时序一定可运行；未知设施拓扑应硬失败或标记 `unverified`，不能默认为无限能力。
- SCC 只给出稳定的循环分组和上游优先拓扑；它不证明循环路线已完成实体布线、无死锁或游戏中持续运行。布局候选的固定预算选择应称为“规则覆盖内的最优可行候选”，不得称数学全局最优。
